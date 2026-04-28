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

func TestNormalizeBacktestParametersPreservesExplicitZeroReentrySlots(t *testing.T) {
	normalized, err := NormalizeBacktestParameters(map[string]any{
		"reentry_size_schedule": []any{0.20, 0.0},
	})
	if err != nil {
		t.Fatalf("expected normalization to succeed, got %v", err)
	}
	schedule := normalizeBacktestFloatSlice(normalized["reentry_size_schedule"], nil)
	if len(schedule) != 2 {
		t.Fatalf("expected two schedule slots, got %v", schedule)
	}
	if schedule[0] != 0.20 || schedule[1] != 0.0 {
		t.Fatalf("expected [0.20, 0.0], got %v", schedule)
	}
}

func TestNormalizeBacktestParametersCanonicalizesFiveMinuteSignalTimeframe(t *testing.T) {
	normalized, err := NormalizeBacktestParameters(map[string]any{
		"signalTimeframe":     "5min",
		"executionDataSource": "1min",
	})
	if err != nil {
		t.Fatalf("expected 5min signal timeframe to be supported, got %v", err)
	}
	if got := stringValue(normalized["signalTimeframe"]); got != "5m" {
		t.Fatalf("expected canonicalized signalTimeframe 5m, got %s", got)
	}
}

func TestNormalizeBacktestParametersDefaultsZeroInitialToReentryWindow(t *testing.T) {
	normalized, err := NormalizeBacktestParameters(map[string]any{})
	if err != nil {
		t.Fatalf("expected normalization to succeed, got %v", err)
	}
	if !boolValue(normalized["dir2_zero_initial"]) {
		t.Fatal("expected dir2_zero_initial to default to true")
	}
	if got := stringValue(normalized["zero_initial_mode"]); got != strategyZeroInitialModeReentryWindow {
		t.Fatalf("expected zero_initial_mode=%s, got %s", strategyZeroInitialModeReentryWindow, got)
	}
	if got := maxIntValue(normalized["max_trades_per_bar"], 0); got != domain.ResearchBaselineMaxTradesPerBar {
		t.Fatalf("expected max_trades_per_bar=%d, got %d", domain.ResearchBaselineMaxTradesPerBar, got)
	}
	schedule := normalizeBacktestFloatSlice(normalized["reentry_size_schedule"], nil)
	if len(schedule) != 2 || schedule[0] != 0.20 || schedule[1] != 0.10 {
		t.Fatalf("expected default schedule [0.20, 0.10], got %v", schedule)
	}
}

func TestBuildSignalBarsSupportsFiveMinuteAggregation(t *testing.T) {
	minuteBars := make([]candleBar, 0, 12)
	start := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		price := 100.0 + float64(i)
		minuteBars = append(minuteBars, candleBar{
			Time:   start.Add(time.Duration(i) * time.Minute),
			Open:   price,
			High:   price + 1,
			Low:    price - 1,
			Close:  price + 0.5,
			Volume: 10,
		})
	}

	signals, err := buildSignalBars(minuteBars, "5m")
	if err != nil {
		t.Fatalf("build 5m signal bars failed: %v", err)
	}
	if len(signals) != 3 {
		t.Fatalf("expected 3 aggregated 5m bars, got %d", len(signals))
	}
	if !signals[0].Time.Equal(start) {
		t.Fatalf("expected first 5m bucket at %s, got %s", start, signals[0].Time)
	}
	if signals[0].Open != 100 || signals[0].Close != 104.5 || signals[0].Volume != 50 {
		t.Fatalf("unexpected first 5m bar: %#v", signals[0])
	}
	if !signals[1].Time.Equal(start.Add(5 * time.Minute)) {
		t.Fatalf("expected second 5m bucket at %s, got %s", start.Add(5*time.Minute), signals[1].Time)
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
	if cfg.MaxTradesPerBar != domain.ResearchBaselineMaxTradesPerBar {
		t.Fatalf("expected max trades per bar %d, got %d", domain.ResearchBaselineMaxTradesPerBar, cfg.MaxTradesPerBar)
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
	if cfg.ZeroInitialMode != strategyZeroInitialModeReentryWindow {
		t.Fatalf("expected zero initial mode %s, got %s", strategyZeroInitialModeReentryWindow, cfg.ZeroInitialMode)
	}
	if !cfg.Dir2ZeroInitial {
		t.Fatal("expected dir2 zero initial to remain enabled by default")
	}
}

func TestResolveReplayInitialBreakoutAllowsT3WithSep(t *testing.T) {
	cfg := strategyReplayConfig{
		BreakoutShape:         "baseline_plus_t3",
		T3MinSMAATRSeparation: 0.25,
	}
	sig := strategySignalBar{
		MA5:       68200,
		ATR:       800,
		PrevHigh1: 68800,
		PrevHigh2: 68600,
		PrevHigh3: 69000,
		PrevLow1:  68100,
		PrevLow2:  68000,
		PrevLow3:  67900,
	}
	breakout := resolveReplayInitialBreakout(sig, "long", 69010, cfg)
	if !breakout.Ready || breakout.Level != 69000 || breakout.Shape != "t3_swing" {
		t.Fatalf("expected t3 breakout to pass sep filter, got %#v", breakout)
	}
}

func TestResolveReplayInitialBreakoutBlocksT3InsideSep(t *testing.T) {
	cfg := strategyReplayConfig{
		BreakoutShape:         "baseline_plus_t3",
		T3MinSMAATRSeparation: 0.25,
	}
	sig := strategySignalBar{
		MA5:       68850,
		ATR:       800,
		PrevHigh1: 68800,
		PrevHigh2: 68600,
		PrevHigh3: 69000,
	}
	breakout := resolveReplayInitialBreakout(sig, "long", 69010, cfg)
	if breakout.Ready {
		t.Fatalf("expected t3 breakout inside sep filter to be blocked, got %#v", breakout)
	}
}

func TestRunStrategyReplayOnMinuteBarsUsesZeroInitialReentryWindowForLongEntries(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	signals := []strategySignalBar{
		{
			Time:      start,
			Close:     110,
			MA5:       100,
			MA20:      100,
			ATR:       10,
			PrevHigh1: 100,
			PrevHigh2: 105,
			PrevLow1:  95,
			PrevLow2:  94,
		},
		{Time: start.Add(24 * time.Hour)},
	}
	minuteBars := []candleBar{
		{Time: start.Add(time.Minute), Open: 105, High: 106, Low: 104, Close: 105},
		{Time: start.Add(2 * time.Minute), Open: 106, High: 106, Low: 94, Close: 95},
	}
	cfg := strategyReplayConfig{
		SignalTimeframe:     "1d",
		ExecutionDataSource: "1min",
		InitialBalance:      100000,
		Dir2ZeroInitial:     true,
		ZeroInitialMode:     strategyZeroInitialModeReentryWindow,
		FixedSlippage:       0,
		StopLossATR:         0.05,
		MaxTradesPerBar:     4,
		ReentrySizeSchedule: []float64{0.10, 0.05, 0.025},
		LongReentryATR:      0.1,
		ShortReentryATR:     0.0,
		StopMode:            "atr",
		ProfitProtectATR:    1.0,
		TradingFeeRate:      0,
	}

	result, err := runStrategyReplayOnMinuteBars(cfg, signals, minuteBars)
	if err != nil {
		t.Fatalf("runStrategyReplayOnMinuteBars failed: %v", err)
	}
	trades, err := executionTradesFromResult(result)
	if err != nil {
		t.Fatalf("executionTradesFromResult failed: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected one execution trade, got %d", len(trades))
	}
	if got := stringValue(trades[0]["entryReason"]); got != "Zero-Initial-Reentry" {
		t.Fatalf("expected Zero-Initial-Reentry entry reason, got %s", got)
	}
	if got := parseFloatValue(trades[0]["notional"]); got <= 0 {
		t.Fatalf("expected positive notional for zero initial reentry window, got %v", got)
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
