package service

import (
	"math"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestNormalizeBacktestParametersMapsLegacyBaselineAliases(t *testing.T) {
	normalized, err := NormalizeBacktestParameters(map[string]any{
		"signalTimeframe":              "1D",
		"executionDataSource":          "1min",
		"symbol":                       "BTCUSDT",
		"maxTradesPerBar":              3,
		"reentrySizes":                 []float64{0.20, 0.10},
		"stopLossATR":                  0.05,
		"profitProtectATR":             1.0,
		"trailingStopATR":              0.3,
		"delayedTrailingActivationATR": 0.5,
		"longReentryATR":               0.1,
		"shortReentryATR":              0.0,
	})
	if err != nil {
		t.Fatalf("expected normalization to succeed, got %v", err)
	}
	if got := maxIntValue(normalized["max_trades_per_bar"], 0); got != 3 {
		t.Fatalf("expected max_trades_per_bar=3, got %d", got)
	}
	schedule := normalizeBacktestFloatSlice(normalized["reentry_size_schedule"], nil)
	if len(schedule) != 2 || schedule[0] != 0.20 || schedule[1] != 0.10 {
		t.Fatalf("expected [0.20, 0.10], got %v", schedule)
	}
	if got := parseFloatValue(normalized["trailing_stop_atr"]); got != 0.3 {
		t.Fatalf("expected trailing_stop_atr=0.3, got %v", got)
	}
	if got := parseFloatValue(normalized["delayed_trailing_activation_atr"]); got != 0.5 {
		t.Fatalf("expected delayed_trailing_activation_atr=0.5, got %v", got)
	}
	if got := parseFloatValue(normalized["long_reentry_atr"]); got != 0.1 {
		t.Fatalf("expected long_reentry_atr=0.1, got %v", got)
	}
	if got := parseFloatValue(normalized["short_reentry_atr"]); got != 0.0 {
		t.Fatalf("expected short_reentry_atr=0.0, got %v", got)
	}
}

func TestBuildStrategyReplayConfigUsesUpdatedBaselineDefaults(t *testing.T) {
	cfg := buildStrategyReplayConfig(StrategyExecutionContext{
		SignalTimeframe:     "1d",
		ExecutionDataSource: "1min",
		Symbol:              "BTCUSDT",
		Parameters:          map[string]any{},
		Semantics: StrategyExecutionSemantics{
			TradingFeeBps:        10,
			FundingIntervalHours: 8,
		},
	})
	if cfg.MaxTradesPerBar != 3 {
		t.Fatalf("expected max trades per bar 3, got %d", cfg.MaxTradesPerBar)
	}
	if len(cfg.ReentrySizeSchedule) != 2 || cfg.ReentrySizeSchedule[0] != 0.20 || cfg.ReentrySizeSchedule[1] != 0.10 {
		t.Fatalf("expected default schedule [0.20, 0.10], got %v", cfg.ReentrySizeSchedule)
	}
	if cfg.LongReentryATR != 0.1 {
		t.Fatalf("expected long reentry atr 0.1, got %v", cfg.LongReentryATR)
	}
	if cfg.ShortReentryATR != 0.0 {
		t.Fatalf("expected short reentry atr 0.0, got %v", cfg.ShortReentryATR)
	}
}

func TestResolveReplayReentrySlotCarriesInitialBudgetAcrossBars(t *testing.T) {
	size, effective, ok := resolveReplayReentrySlot(0, 3, []float64{0.20, 0.10})
	if !ok {
		t.Fatal("expected carried reentry slot to remain available")
	}
	if effective != 1 {
		t.Fatalf("expected carried slot to reuse initial-counted slot 1, got %d", effective)
	}
	if size != 0.20 {
		t.Fatalf("expected first carried reentry size 0.20, got %v", size)
	}

	size, effective, ok = resolveReplayReentrySlot(2, 3, []float64{0.20, 0.10})
	if !ok {
		t.Fatal("expected second reentry slot to remain available")
	}
	if effective != 2 || size != 0.10 {
		t.Fatalf("expected second slot size 0.10 with effective index 2, got size=%v effective=%d", size, effective)
	}

	if _, _, ok = resolveReplayReentrySlot(3, 3, []float64{0.20, 0.10}); ok {
		t.Fatal("expected slot to close once initial + two reentries are consumed")
	}
}

func TestEvaluateReplayPositionExitAppliesTrailingStopAfterActivation(t *testing.T) {
	cfg := strategyReplayConfig{
		FixedSlippage:      0,
		ProfitProtectATR:   1.0,
		TrailingStopATR:    0.3,
		DelayedTrailingATR: 0.5,
	}
	sig := strategySignalBar{
		ATR:      10,
		PrevLow1: 98,
	}
	position := &strategyPosition{
		Side:       "long",
		EntryPrice: 100,
		StopLoss:   95,
		Notional:   1,
		HWM:        100,
		LWM:        100,
	}

	reason, _, exited := evaluateReplayPositionExit(position, sig, cfg, 106, 104)
	if exited {
		t.Fatalf("expected first bar to stay open, got %s", reason)
	}
	if math.Abs(position.StopLoss-103) > 1e-9 {
		t.Fatalf("expected trailing stop to tighten to 103, got %v", position.StopLoss)
	}
	if math.Abs(position.HWM-106) > 1e-9 {
		t.Fatalf("expected hwm to update to 106, got %v", position.HWM)
	}

	reason, exitPrice, exited := evaluateReplayPositionExit(position, sig, cfg, 104, 102)
	if !exited || reason != "SL" {
		t.Fatalf("expected second bar to exit via trailing SL, got exited=%v reason=%s", exited, reason)
	}
	if math.Abs(exitPrice-103) > 1e-9 {
		t.Fatalf("expected trailing SL exit at 103, got %v", exitPrice)
	}
}

func TestEvaluateReplayPositionExitReturnsRawExitLevels(t *testing.T) {
	cfg := strategyReplayConfig{
		FixedSlippage:    0.01,
		ProfitProtectATR: 1.0,
	}
	sig := strategySignalBar{
		ATR:      10,
		PrevLow1: 98,
	}
	position := &strategyPosition{
		Side:       "long",
		EntryPrice: 100,
		StopLoss:   95,
		Notional:   1,
		HWM:        100,
		LWM:        100,
	}

	reason, exitPrice, exited := evaluateReplayPositionExit(position, sig, cfg, 101, 94)
	if !exited || reason != "SL" {
		t.Fatalf("expected SL exit, got exited=%v reason=%s", exited, reason)
	}
	if math.Abs(exitPrice-95) > 1e-9 {
		t.Fatalf("expected raw stop price 95, got %v", exitPrice)
	}
}

func TestStrategyReplayEngineTryExitAppliesSingleSlippageAdjustment(t *testing.T) {
	engine := newStrategyReplayEngine(strategyReplayConfig{
		FixedSlippage:  0.01,
		TradingFeeRate: 0,
	})
	engine.position = &strategyPosition{
		Side:       "long",
		EntryPrice: 100,
		StopLoss:   95,
		Notional:   1000,
		HWM:        100,
		LWM:        100,
	}

	engine.tryExit(executionBar{
		Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		High: 101,
		Low:  94,
	}, strategySignalBar{ATR: 10, PrevLow1: 98})

	if engine.position != nil {
		t.Fatal("expected position to be cleared after exit")
	}
	if len(engine.trades) != 1 {
		t.Fatalf("expected one exit trade, got %d", len(engine.trades))
	}
	price := parseFloatValue(engine.trades[0]["price"])
	if math.Abs(price-94.05) > 1e-9 {
		t.Fatalf("expected single-slippage exit price 94.05, got %v", price)
	}
}

func TestResolveLiveSessionParametersNormalizesLegacyStrategyParameters(t *testing.T) {
	platform := &Platform{}
	session := domain.LiveSession{ID: "live-session-1", State: map[string]any{}}
	version := domain.StrategyVersion{
		ID:                 "strategy-version-1",
		SignalTimeframe:    "1D",
		ExecutionTimeframe: "1m",
		Parameters: map[string]any{
			"maxTradesPerBar":              3,
			"reentrySizes":                 []float64{0.20, 0.10},
			"stopLossATR":                  0.05,
			"profitProtectATR":             1.0,
			"trailingStopATR":              0.3,
			"delayedTrailingActivationATR": 0.5,
		},
	}

	parameters, err := platform.resolveLiveSessionParameters(session, version)
	if err != nil {
		t.Fatalf("expected parameter normalization to succeed, got %v", err)
	}
	if got := maxIntValue(parameters["max_trades_per_bar"], 0); got != 3 {
		t.Fatalf("expected max_trades_per_bar=3, got %d", got)
	}
	schedule := normalizeBacktestFloatSlice(parameters["reentry_size_schedule"], nil)
	if len(schedule) != 2 || schedule[0] != 0.20 || schedule[1] != 0.10 {
		t.Fatalf("expected normalized schedule [0.20, 0.10], got %v", schedule)
	}
	if got := parseFloatValue(parameters["trailing_stop_atr"]); got != 0.3 {
		t.Fatalf("expected trailing_stop_atr=0.3, got %v", got)
	}
	if got := parseFloatValue(parameters["delayed_trailing_activation_atr"]); got != 0.5 {
		t.Fatalf("expected delayed_trailing_activation_atr=0.5, got %v", got)
	}
	if got := stringValue(parameters["signalTimeframe"]); got != "1d" {
		t.Fatalf("expected normalized signalTimeframe 1d, got %s", got)
	}
	if got := stringValue(parameters["executionDataSource"]); got != "1min" {
		t.Fatalf("expected normalized executionDataSource 1min, got %s", got)
	}
}

func TestResolveReplayRangeUsesUTCInputsForConfigDefaults(t *testing.T) {
	cfg := buildStrategyReplayConfig(StrategyExecutionContext{
		SignalTimeframe:     "1d",
		ExecutionDataSource: "1min",
		Symbol:              "BTCUSDT",
		From:                time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		To:                  time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Parameters:          map[string]any{},
		Semantics:           StrategyExecutionSemantics{TradingFeeBps: 10, FundingIntervalHours: 8},
	})
	if cfg.From.Location() != time.UTC || cfg.To.Location() != time.UTC {
		t.Fatal("expected replay config range to preserve UTC times")
	}
}
