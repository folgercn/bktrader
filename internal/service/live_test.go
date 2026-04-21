package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestDeriveLiveSignalIntentUsesNextPlannedStep(t *testing.T) {
	decision := StrategySignalDecision{
		Action: "advance-plan",
		Metadata: map[string]any{
			"signalBarDecision": map[string]any{
				"ready":      true,
				"shortReady": true,
				"ma20":       68000.0,
				"atr14":      1200.0,
			},
			"marketPrice":       67950.0,
			"marketSource":      "order_book.bestBid",
			"signalKind":        "protect-exit",
			"decisionState":     "exit-ready",
			"signalBarStateKey": "binance|BTCUSDT|trigger|4h",
			"nextPlannedSide":   "SELL",
			"nextPlannedRole":   "exit",
			"nextPlannedReason": "PT",
			"nextPlannedEvent":  time.Date(2026, 4, 7, 4, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"nextPlannedPrice":  67900.0,
		},
	}

	intent := deriveLiveSignalIntent(decision, "BTCUSDT")
	if intent == nil {
		t.Fatal("expected intent")
	}
	if got := intent.Action; got != "exit" {
		t.Fatalf("expected exit action, got %v", got)
	}
	if got := intent.Side; got != "SELL" {
		t.Fatalf("expected SELL side, got %v", got)
	}
	if got := intent.Reason; got != "PT" {
		t.Fatalf("expected PT reason, got %v", got)
	}
}

func TestEvaluateSignalBarGateRequiresLongBreakoutAlignmentWithResearch(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"ma20":  68000.0,
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  68900.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "BUY", "entry", "")
	if boolValue(gate["longStructureReady"]) != true {
		t.Fatal("expected long structure to be ready")
	}
	if boolValue(gate["longBreakoutReady"]) {
		t.Fatal("expected breakout to remain not ready before current high breaks prevHigh2")
	}
	if boolValue(gate["longReady"]) {
		t.Fatal("expected long signal to stay blocked without breakout confirmation")
	}
	if got := stringValue(gate["reason"]); got != "long-signal-not-ready" {
		t.Fatalf("expected long-signal-not-ready, got %s", got)
	}
}

func TestEvaluateSignalBarGateAllowsLongAfterBreakoutAlignmentWithResearch(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"ma20":  68000.0,
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  69010.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "BUY", "entry", "")
	if !boolValue(gate["longStructureReady"]) {
		t.Fatal("expected long structure to be ready")
	}
	if !boolValue(gate["longBreakoutReady"]) {
		t.Fatal("expected breakout to be ready after current high breaks prevHigh2")
	}
	if !boolValue(gate["longReady"]) {
		t.Fatal("expected long signal to be ready after breakout confirmation")
	}
}

func TestEvaluateSignalBarGateDoesNotRequireOppositeBreakoutForExit(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"ma20":  68000.0,
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  68900.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "SELL", "exit", "")
	if !boolValue(gate["ready"]) {
		t.Fatalf("expected exit gate to stay ready, got reason=%s", stringValue(gate["reason"]))
	}
}

func TestAlignLivePlanStepToCurrentMarketKeepsExitForVirtualPosition(t *testing.T) {
	eventTime := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	nextPlannedEvent := eventTime.Add(-48 * time.Hour)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"ma20":      68000.0,
			"atr14":     900.0,
			"current": map[string]any{
				"close": 68100.0,
				"high":  69010.0,
				"low":   67800.0,
			},
			"prevBar1": map[string]any{
				"high": 68850.0,
				"low":  67750.0,
			},
			"prevBar2": map[string]any{
				"high": 69000.0,
				"low":  67600.0,
			},
		},
	}
	currentPosition := map[string]any{
		"id":         "virtual|session-1|signal-1",
		"virtual":    true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 69000.0,
		"quantity":   0.0,
	}

	gotEvent, gotPrice, gotSide, gotRole, gotReason := alignLivePlanStepToCurrentMarket(
		signalStates,
		"1d",
		currentPosition,
		eventTime,
		nextPlannedEvent,
		68950.0,
		"SELL",
		"exit",
		"SL",
	)
	if !gotEvent.Equal(nextPlannedEvent.UTC()) {
		t.Fatalf("expected stale exit event to stay unchanged, got %s", gotEvent.Format(time.RFC3339))
	}
	if gotPrice != 68950.0 {
		t.Fatalf("expected planned price to stay unchanged, got %.2f", gotPrice)
	}
	if gotSide != "SELL" || gotRole != "exit" || gotReason != "SL" {
		t.Fatalf("expected virtual position to preserve exit step, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationUsesZeroInitialWindowAcrossTwoBars(t *testing.T) {
	barStart := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"ma20":      68000.0,
			"atr14":     900.0,
			"current": map[string]any{
				"barStart": barStart.Format(time.RFC3339),
				"close":    68100.0,
				"high":     69010.0,
				"low":      67800.0,
			},
			"prevBar1": map[string]any{
				"high": 68850.0,
				"low":  67750.0,
			},
			"prevBar2": map[string]any{
				"high": 69000.0,
				"low":  67600.0,
			},
		},
	}
	state, gotEvent, gotPrice, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		map[string]any{},
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"long_reentry_atr":  0.1,
		},
		signalStates,
		"BTCUSDT",
		"1d",
		map[string]any{},
		barStart.Add(2*time.Hour),
		barStart.Add(-48*time.Hour),
		68950.0,
		"BUY",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "Zero-Initial-Reentry" || gotSide != "BUY" {
		t.Fatalf("expected zero initial reentry override, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if gotPrice != 67840.0 {
		t.Fatalf("expected long reentry price 67840, got %.2f", gotPrice)
	}
	if gotEvent.IsZero() {
		t.Fatal("expected planned event for zero initial window")
	}
	pending := mapValue(state[livePendingZeroInitialWindowStateKey])
	if stringValue(pending["side"]) != "BUY" {
		t.Fatalf("expected pending BUY window, got %+v", pending)
	}
	timeline := metadataList(state["timeline"])
	if len(timeline) != 1 {
		t.Fatalf("expected one zero initial window timeline event, got %+v", timeline)
	}
	if got := stringValue(timeline[0]["title"]); got != "zero-initial-window-armed" {
		t.Fatalf("expected zero-initial-window-armed timeline event, got %s", got)
	}
	timelineMetadata := mapValue(timeline[0]["metadata"])
	pendingFromTimeline := mapValue(timelineMetadata[livePendingZeroInitialWindowStateKey])
	if stringValue(pendingFromTimeline["side"]) != "BUY" || stringValue(pendingFromTimeline["symbol"]) != "BTCUSDT" {
		t.Fatalf("expected pending window snapshot in timeline metadata, got %+v", pendingFromTimeline)
	}

	nextBarStart := barStart.Add(24 * time.Hour)
	nextBarStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"atr14":     900.0,
			"current": map[string]any{
				"barStart": nextBarStart.Format(time.RFC3339),
				"close":    68200.0,
				"high":     68800.0,
				"low":      67820.0,
			},
			"prevBar1": map[string]any{
				"high": 69010.0,
				"low":  67800.0,
			},
			"prevBar2": map[string]any{
				"high": 68850.0,
				"low":  67750.0,
			},
		},
	}
	state, _, _, gotSide, gotRole, gotReason = prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"long_reentry_atr":  0.1,
		},
		nextBarStates,
		"BTCUSDT",
		"1d",
		map[string]any{},
		nextBarStart.Add(2*time.Hour),
		barStart.Add(-24*time.Hour),
		68950.0,
		"BUY",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "Zero-Initial-Reentry" || gotSide != "BUY" {
		t.Fatalf("expected pending zero initial window to remain active on next bar, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if timeline := metadataList(state["timeline"]); len(timeline) != 1 {
		t.Fatalf("expected existing pending window to avoid duplicate timeline events, got %+v", timeline)
	}

	expiredState, _, _, _, _, _ := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"long_reentry_atr":  0.1,
		},
		nextBarStates,
		"BTCUSDT",
		"1d",
		map[string]any{},
		barStart.Add(49*time.Hour),
		barStart.Add(-72*time.Hour),
		68950.0,
		"BUY",
		"entry",
		"Initial",
	)
	if pending := mapValue(expiredState[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected zero initial window to expire after next bar, got %+v", pending)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationPrioritizesExitReentryOverZeroInitialWindow(t *testing.T) {
	eventTime := time.Date(2026, 4, 10, 2, 0, 0, 0, time.UTC)
	state := map[string]any{
		livePendingZeroInitialWindowStateKey: map[string]any{
			"side":            "SELL",
			"symbol":          "BTCUSDT",
			"signalTimeframe": "1d",
			"armedAt":         eventTime.Add(-time.Minute).Format(time.RFC3339),
			"signalBarStart":  eventTime.Truncate(24 * time.Hour).Format(time.RFC3339),
			"expiresAt":       eventTime.Truncate(24 * time.Hour).Add(48 * time.Hour).Format(time.RFC3339),
		},
	}
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"sma5":      75650.0,
			"atr14":     100.0,
			"current": map[string]any{
				"barStart": eventTime.Truncate(24 * time.Hour).Format(time.RFC3339),
				"close":    75600.0,
				"high":     75700.0,
				"low":      75500.0,
			},
			"prevBar1": map[string]any{
				"high": 75600.0,
				"low":  75400.0,
			},
		},
	}

	updated, gotEvent, gotPrice, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
		},
		signalStates,
		"BTCUSDT",
		"1d",
		map[string]any{},
		eventTime,
		eventTime,
		75600.0,
		"SELL",
		"entry",
		"SL-Reentry",
	)
	if gotRole != "entry" || gotReason != "SL-Reentry" || gotSide != "SELL" {
		t.Fatalf("expected SL-Reentry to outrank pending zero initial window, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if !gotEvent.Equal(eventTime) || gotPrice != 75600.0 {
		t.Fatalf("expected original SL-Reentry plan step to be preserved, got event=%s price=%v", gotEvent, gotPrice)
	}
	if pending := mapValue(updated[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected pending zero initial window to be consumed by exit reentry priority, got %+v", pending)
	}
	timeline := metadataList(updated["timeline"])
	if len(timeline) != 1 || stringValue(timeline[0]["title"]) != "zero-initial-window-consumed" {
		t.Fatalf("expected zero initial window consumed timeline event, got %+v", timeline)
	}
	if got := stringValue(mapValue(timeline[0]["metadata"])["reason"]); got != "exit-reentry-priority" {
		t.Fatalf("expected exit-reentry-priority consume reason, got %s", got)
	}
}

func TestEvaluateSignalBarGateAllowsReentryWithoutInitialBreakout(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"timeframe": "1d",
		"sma5":      68050.0,
		"atr14":     900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  68900.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "BUY", "entry", "SL-Reentry")
	if !boolValue(gate["longStructureReady"]) {
		t.Fatal("expected long structure to be ready for reentry")
	}
	if boolValue(gate["longBreakoutReady"]) {
		t.Fatal("expected initial breakout to remain not ready for reentry regression")
	}
	if !boolValue(gate["longReady"]) || !boolValue(gate["ready"]) {
		t.Fatalf("expected reentry gate to stay ready without initial breakout, got %#v", gate)
	}
}

func TestBuildLiveExecutionPlanFromMarketDataAcceptsTickExecutionSource(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}

	version, err := platform.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		t.Fatalf("resolve strategy version failed: %v", err)
	}
	parameters, err := platform.resolveLiveSessionParameters(session, version)
	if err != nil {
		t.Fatalf("resolve live session parameters failed: %v", err)
	}
	if !boolValue(parameters["dir2_zero_initial"]) {
		t.Fatal("expected live session parameters to default dir2_zero_initial=true")
	}
	if got := stringValue(parameters["zero_initial_mode"]); got != strategyZeroInitialModeReentryWindow {
		t.Fatalf("expected live session zero_initial_mode=%s, got %s", strategyZeroInitialModeReentryWindow, got)
	}
	engine, engineKey, err := platform.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		t.Fatalf("resolve strategy engine failed: %v", err)
	}

	signalCandles := make([]candleBar, 0, 32)
	minuteBars := make([]candleBar, 0, 32)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 32; i++ {
		ts := start.AddDate(0, 0, i)
		open := 100.0 + float64(i)*2
		close := open + 1
		high := close + 2
		low := open - 2
		signalCandles = append(signalCandles, candleBar{
			Time:   ts,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: 10 + float64(i),
		})
		minuteBars = append(minuteBars, candleBar{
			Time:   ts.Add(30 * time.Minute),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: 1,
		})
	}
	signalBars, err := buildStrategySignalBarsFromCandles(signalCandles)
	if err != nil {
		t.Fatalf("build signal bars failed: %v", err)
	}

	platform.liveMarketMu.Lock()
	platform.liveMarketData["BTCUSDT"] = liveMarketSnapshot{
		Symbol:     "BTCUSDT",
		MinuteBars: minuteBars,
		SignalBars: map[string][]strategySignalBar{"1d": signalBars},
		UpdatedAt:  time.Now().UTC(),
	}
	platform.liveMarketMu.Unlock()

	plan, err := platform.buildLiveExecutionPlanFromMarketData(
		session,
		version,
		engine,
		engineKey,
		parameters,
		defaultExecutionSemantics(ExecutionModeLive, parameters),
	)
	if err != nil {
		t.Fatalf("build live execution plan failed: %v", err)
	}
	if plan == nil {
		t.Fatal("expected plan slice, got nil")
	}
}

func TestResolveLiveSessionParametersDefaultsMissingStopsToTrailingForLive(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	if _, err := platform.store.UpdateStrategyParameters("strategy-bk-1d", map[string]any{}); err != nil {
		t.Fatalf("update strategy parameters failed: %v", err)
	}

	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	version, err := platform.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		t.Fatalf("resolve strategy version failed: %v", err)
	}

	parameters, err := platform.resolveLiveSessionParameters(session, version)
	if err != nil {
		t.Fatalf("resolve live session parameters failed: %v", err)
	}

	if got := parseFloatValue(parameters["trailing_stop_atr"]); got != 0.3 {
		t.Fatalf("expected trailing_stop_atr live default 0.3, got %v", got)
	}
	if got := parseFloatValue(parameters["delayed_trailing_activation_atr"]); got != 0.5 {
		t.Fatalf("expected delayed_trailing_activation_atr live default 0.5, got %v", got)
	}
	if got := parseFloatValue(parameters["stop_loss_atr"]); got != 0.3 {
		t.Fatalf("expected stop_loss_atr to inherit trailing live default 0.3, got %v", got)
	}
}

func TestRefreshLiveMarketSnapshotFailsWithoutRESTWarmData(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	originalFetch := fetchLiveCandleRange
	fetchLiveCandleRange = func(symbol, resolution string, from, to time.Time) ([]candleBar, error) {
		return nil, fmt.Errorf("upstream unavailable")
	}
	t.Cleanup(func() {
		fetchLiveCandleRange = originalFetch
	})

	err := platform.refreshLiveMarketSnapshot("BTCUSDT")
	if err == nil {
		t.Fatal("expected warm snapshot refresh to fail when REST warm data is unavailable")
	}
	if !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("expected upstream error to surface, got %v", err)
	}
}

func TestEvaluateLiveExitStateRequiresProtectionBeforePT(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
		"protected":  false,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68900.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68900.0, map[string]any{}, "PT")
	if boolValue(state["ready"]) {
		t.Fatal("expected PT exit to stay blocked before protection is armed")
	}
	if got := stringValue(state["waitReason"]); got != "profit-protection-not-armed" {
		t.Fatalf("expected profit-protection-not-armed, got %s", got)
	}
}

func TestEvaluateLiveExitStateArmsProtectionAndTriggersPTForLong(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
		"protected":  true,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68790.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68790.0, map[string]any{}, "PT")
	if !boolValue(state["ready"]) {
		t.Fatalf("expected PT exit to trigger once protected and price <= prevLow1, got waitReason=%s", stringValue(state["waitReason"]))
	}
	if got := parseFloatValue(state["targetPrice"]); got != 68800.0 {
		t.Fatalf("expected PT target price 68800, got %v", got)
	}
}

func TestEvaluateLiveExitStateTriggersSLForLong(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68940.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68940.0, map[string]any{}, "SL")
	if !boolValue(state["ready"]) {
		t.Fatalf("expected SL exit to trigger once price <= stopLoss, got waitReason=%s", stringValue(state["waitReason"]))
	}
}

func TestEvaluateLiveExitStateUsesTrailingStopDiagnosticsForLong(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr":              1.0,
		"stop_loss_atr":                   0.05,
		"stop_mode":                       "atr",
		"trailing_stop_atr":               0.3,
		"delayed_trailing_activation_atr": 0.5,
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 69600.0,
		},
		"prevBar1": map[string]any{
			"high": 69700.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69650.0,
			"low":  68700.0,
		},
	}, 69600.0, map[string]any{}, "SL")
	if boolValue(state["ready"]) {
		t.Fatal("expected trailing SL to remain untriggered while price stays above the tightened stop")
	}
	if got := stringValue(state["waitReason"]); got != "trailing-sl-not-triggered" {
		t.Fatalf("expected trailing-sl-not-triggered, got %s", got)
	}
	if got := stringValue(state["targetPriceSource"]); got != "trailing-stop" {
		t.Fatalf("expected targetPriceSource trailing-stop, got %s", got)
	}
	if !boolValue(state["trailingStopActive"]) {
		t.Fatal("expected trailingStopActive to be surfaced in exit diagnostics")
	}
}

func TestEvaluateLiveExitStateReportsMissingEntryPrice(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":    true,
		"symbol":   "BTCUSDT",
		"side":     "LONG",
		"quantity": 0.002,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68940.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68940.0, map[string]any{}, "SL")
	if boolValue(state["ready"]) {
		t.Fatal("expected exit state to stay blocked when entry price is unavailable")
	}
	if got := stringValue(state["waitReason"]); got != "position-unavailable-missing-entry-price" {
		t.Fatalf("expected position-unavailable-missing-entry-price, got %s", got)
	}
}

func TestAdjustLiveExecutionProposalForVirtualInitialWhenZeroInitialEnabled(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{}, map[string]any{
		"dir2_zero_initial": true,
		"zero_initial_mode": "position",
	}, ExecutionProposal{
		Role:   "entry",
		Reason: "Initial",
		Status: "dispatchable",
		Metadata: map[string]any{
			"signalBarStateKey": "state-1",
		},
	})
	if proposal.Status != "virtual-initial" {
		t.Fatalf("expected virtual-initial proposal, got %s", proposal.Status)
	}
	if !boolValue(proposal.Metadata["virtualPosition"]) {
		t.Fatal("expected virtualPosition marker on proposal metadata")
	}
}

func TestAdjustLiveExecutionProposalLeavesInitialDispatchableWhenZeroInitialWindowEnabled(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{}, map[string]any{
		"dir2_zero_initial": true,
		"zero_initial_mode": "reentry_window",
	}, ExecutionProposal{
		Role:   "entry",
		Reason: "Initial",
		Status: "dispatchable",
	})
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected initial proposal to remain dispatchable under reentry window mode, got %s", proposal.Status)
	}
}

func TestAdjustLiveExecutionProposalLeavesReentryDispatchableWhenZeroInitialEnabled(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{}, map[string]any{
		"dir2_zero_initial": true,
	}, ExecutionProposal{
		Role:   "entry",
		Reason: "SL-Reentry",
		Status: "dispatchable",
	})
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected reentry proposal to remain dispatchable, got %s", proposal.Status)
	}
}

func TestAdjustLiveExecutionProposalMarksVirtualExitWhenSessionTracksVirtualPosition(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{
		State: map[string]any{
			"virtualPosition": map[string]any{
				"virtual": true,
				"symbol":  "BTCUSDT",
				"side":    "LONG",
			},
		},
	}, map[string]any{
		"dir2_zero_initial": true,
	}, ExecutionProposal{
		Role:   "exit",
		Reason: "PT",
		Status: "dispatchable",
	})
	if proposal.Status != "virtual-exit" {
		t.Fatalf("expected virtual-exit proposal, got %s", proposal.Status)
	}
	if !boolValue(proposal.Metadata["virtualExit"]) {
		t.Fatal("expected virtualExit marker on proposal metadata")
	}
}

func TestResolveLiveSessionPositionSnapshotUsesVirtualPosition(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session := domain.LiveSession{
		AccountID: "live-main",
		State: map[string]any{
			"virtualPosition": map[string]any{
				"id":         "virtual|session-1|signal-1",
				"symbol":     "BTCUSDT",
				"side":       "LONG",
				"quantity":   0.0,
				"entryPrice": 69000.0,
				"virtual":    true,
			},
		},
	}
	position, found, err := platform.resolveLiveSessionPositionSnapshot(session, "BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected virtual position not to masquerade as a real found position")
	}
	if !boolValue(position["virtual"]) {
		t.Fatal("expected returned position snapshot to be marked virtual")
	}
	if !boolValue(position["hasVirtualPosition"]) {
		t.Fatal("expected returned position snapshot to expose virtual position explicitly")
	}
	if got := stringValue(position["id"]); got != "virtual|session-1|signal-1" {
		t.Fatalf("expected virtual position id to be preserved, got %s", got)
	}
}

func TestResolveLiveSessionPositionSnapshotIgnoresZeroQuantityStoredPosition(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0,
		EntryPrice:        69000.0,
		MarkPrice:         69010.0,
	}); err != nil {
		t.Fatalf("save zero-quantity position failed: %v", err)
	}

	session := domain.LiveSession{
		AccountID: "live-main",
		State: map[string]any{
			"symbol": "BTCUSDT",
		},
	}
	position, found, err := platform.resolveLiveSessionPositionSnapshot(session, "BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected zero-quantity stored position not to be treated as found")
	}
	if boolValue(position["found"]) {
		t.Fatal("expected returned snapshot to remain flat for zero-quantity stored position")
	}
	if got := parseFloatValue(position["quantity"]); got != 0 {
		t.Fatalf("expected zero quantity snapshot, got %v", got)
	}
}

func TestShouldAutoDispatchLiveIntentBlocksRepeatedVirtualInitialWithinCooldown(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                  "auto-dispatch",
			"lastDispatchedOrderStatus":     liveOrderStatusVirtualInitial,
			"lastDispatchedIntentSignature": signature,
			"lastDispatchedAt":              now.Format(time.RFC3339),
			"dispatchCooldownSeconds":       300,
		},
	}
	if shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected repeated virtual initial signal to be cooled down")
	}
}

func TestReconcileLivePlanIndexKeepsExitWhenVirtualPositionExists(t *testing.T) {
	plan := []paperPlannedOrder{
		{Role: "entry"},
		{Role: "exit"},
	}
	nextIndex, adjusted := reconcileLivePlanIndexWithPosition(plan, 1, map[string]any{
		"virtual":  true,
		"quantity": 0.0,
	}, true)
	if adjusted {
		t.Fatalf("expected no rewind when virtual position exists, got nextIndex=%d", nextIndex)
	}
}

func TestBookAwareExecutionStrategyBuildsProposalFromOrderBook(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"defaultOrderQuantity": 0.002,
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps": 8.0,
			},
		},
		Intent: SignalIntent{
			Action:        "entry",
			Role:          "entry",
			Reason:        "Initial",
			Side:          "BUY",
			Symbol:        "BTCUSDT",
			SignalKind:    "entry",
			DecisionState: "entry-ready",
			PriceHint:     68000,
			PriceSource:   "trade_tick.price",
			Quantity:      0.001,
			Metadata: map[string]any{
				"bestBid":           67999.5,
				"bestAsk":           68000.5,
				"spreadBps":         0.15,
				"signalBarStateKey": "state-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected dispatchable proposal, got %s", proposal.Status)
	}
	if proposal.PriceHint != 68000.5 {
		t.Fatalf("expected best ask price hint, got %v", proposal.PriceHint)
	}
	if proposal.Quantity != 0.002 {
		t.Fatalf("expected session default quantity to win, got %v", proposal.Quantity)
	}
}

func TestBookAwareExecutionStrategyBuildsProposalFromBalanceFraction(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"positionSizingMode":   "fixed_fraction",
				"defaultOrderFraction": 0.1,
				"defaultOrderQuantity": 0.002,
			},
		},
		Account: domain.Account{
			Metadata: map[string]any{
				"liveSyncSnapshot": map[string]any{
					"availableBalance": 1000.0,
				},
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps": 8.0,
			},
		},
		Intent: SignalIntent{
			Action:        "entry",
			Role:          "entry",
			Reason:        "SL-Reentry",
			Side:          "BUY",
			Symbol:        "BTCUSDT",
			SignalKind:    "sl-reentry",
			DecisionState: "entry-ready",
			PriceHint:     50000,
			PriceSource:   "trade_tick.price",
			Quantity:      0.001,
			Metadata: map[string]any{
				"bestBid":           49999.5,
				"bestAsk":           50000.0,
				"spreadBps":         0.1,
				"signalBarStateKey": "state-fraction",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Quantity != 0.002 {
		t.Fatalf("expected fraction-based quantity 0.002, got %v", proposal.Quantity)
	}
	if got := stringValue(proposal.Metadata["positionSizingMode"]); got != "fixed_fraction" {
		t.Fatalf("expected fixed_fraction metadata, got %s", got)
	}
	if got := stringValue(proposal.Metadata["sizingBalanceBasis"]); got != "availableBalance" {
		t.Fatalf("expected availableBalance sizing basis, got %s", got)
	}
}

func TestBookAwareExecutionStrategyUsesReduceOnlyMakerProfileForPTExit(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"defaultOrderQuantity": 0.002,
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionPTExitOrderType":                "LIMIT",
				"executionPTExitPostOnly":                 true,
				"executionPTExitTimeInForce":              "GTX",
				"executionPTExitWideSpreadMode":           "limit-maker",
				"executionPTExitTimeoutFallbackOrderType": "MARKET",
			},
		},
		Intent: SignalIntent{
			Action:        "exit",
			Role:          "exit",
			Reason:        "PT",
			Side:          "SELL",
			Symbol:        "BTCUSDT",
			SignalKind:    "protect-exit",
			DecisionState: "exit-ready",
			PriceHint:     68800,
			PriceSource:   "order_book.bestBid",
			Metadata: map[string]any{
				"bestBid":           68800.0,
				"bestAsk":           68801.0,
				"spreadBps":         0.2,
				"signalBarStateKey": "state-exit-pt",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "LIMIT" {
		t.Fatalf("expected PT exit to prefer LIMIT, got %s", proposal.Type)
	}
	if !proposal.PostOnly || proposal.TimeInForce != "GTX" {
		t.Fatalf("expected PT exit to be post-only GTX, got postOnly=%v tif=%s", proposal.PostOnly, proposal.TimeInForce)
	}
	if !proposal.ReduceOnly {
		t.Fatal("expected PT exit proposal to be reduceOnly")
	}
}

func TestBookAwareExecutionStrategyUsesAggressiveReduceOnlyProfileForSLExit(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"defaultOrderQuantity": 0.002,
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionOrderType":                "LIMIT",
				"executionMaxSpreadBps":             5.0,
				"executionWideSpreadMode":           "limit-maker",
				"executionTimeoutFallbackOrderType": "LIMIT",
			},
		},
		Intent: SignalIntent{
			Action:        "exit",
			Role:          "exit",
			Reason:        "SL",
			Side:          "SELL",
			Symbol:        "BTCUSDT",
			SignalKind:    "risk-exit",
			DecisionState: "exit-ready",
			PriceHint:     68900,
			PriceSource:   "order_book.bestBid",
			Metadata: map[string]any{
				"bestBid":           68900.0,
				"bestAsk":           68902.0,
				"spreadBps":         25.0,
				"signalBarStateKey": "state-exit-sl",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "MARKET" {
		t.Fatalf("expected SL exit to force MARKET, got %s", proposal.Type)
	}
	if proposal.PostOnly {
		t.Fatal("expected SL exit not to be post only")
	}
	if !proposal.ReduceOnly {
		t.Fatal("expected SL exit proposal to be reduceOnly")
	}
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected SL exit to stay dispatchable despite wide spread, got %s", proposal.Status)
	}
	if got := stringValue(proposal.Metadata["executionDecision"]); got != "direct-dispatch" {
		t.Fatalf("expected explicit SL direct dispatch path, got %s", got)
	}
}

func TestBookAwareExecutionStrategyWaitsWhenSpreadTooWide(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps": 5.0,
			},
		},
		Intent: SignalIntent{
			Action:      "entry",
			Role:        "entry",
			Reason:      "Initial",
			Side:        "BUY",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "trade_tick.price",
			Metadata: map[string]any{
				"bestAsk":   68010.0,
				"spreadBps": 12.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "wait" {
		t.Fatalf("expected wait proposal, got %s", proposal.Status)
	}
	if proposal.Reason != "spread-too-wide" {
		t.Fatalf("expected spread-too-wide, got %s", proposal.Reason)
	}
}

func TestBookAwareExecutionStrategyUsesMakerLimitOnWideSpreadWhenConfigured(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	eventTime := time.Date(2026, 4, 7, 8, 0, 0, 0, time.UTC)
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps":          5.0,
				"executionWideSpreadMode":        "limit-maker",
				"executionRestingTimeoutSeconds": 45,
			},
		},
		EventTime: eventTime,
		Intent: SignalIntent{
			Action:      "entry",
			Role:        "entry",
			Reason:      "Initial",
			Side:        "BUY",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "trade_tick.price",
			Metadata: map[string]any{
				"bestBid":           67999.0,
				"bestAsk":           68005.0,
				"spreadBps":         12.0,
				"signalBarStateKey": "state-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected dispatchable proposal, got %s", proposal.Status)
	}
	if proposal.Type != "LIMIT" {
		t.Fatalf("expected LIMIT proposal, got %s", proposal.Type)
	}
	if proposal.LimitPrice != 67999.0 {
		t.Fatalf("expected passive bid limit price, got %v", proposal.LimitPrice)
	}
	if proposal.TimeInForce != "GTX" || !proposal.PostOnly {
		t.Fatalf("expected GTX post-only maker order, got tif=%s postOnly=%v", proposal.TimeInForce, proposal.PostOnly)
	}
	if got := stringValue(proposal.Metadata["executionExpiresAt"]); got != eventTime.Add(45*time.Second).Format(time.RFC3339) {
		t.Fatalf("expected execution expiry to be set, got %s", got)
	}
}

func TestBookAwareExecutionStrategySetsExpiryForSLProtectionWhenConfigured(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	eventTime := time.Date(2026, 4, 7, 8, 0, 0, 0, time.UTC)
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps":                5.0,
				"executionSLExitRestingTimeoutSeconds": 15,
				"executionSLMaxSlippageBps":            20.0,
			},
		},
		EventTime: eventTime,
		Intent: SignalIntent{
			Action:      "exit",
			Role:        "exit",
			Reason:      "SL",
			Side:        "SELL",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "order_book.bestBid",
			Metadata: map[string]any{
				"bestBid":           68000.0,
				"bestAsk":           68150.0,
				"spreadBps":         22.0,
				"signalBarStateKey": "state-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := proposal.Type; got != "LIMIT" {
		t.Fatalf("expected LIMIT SL protection order, got %s", got)
	}
	if got := stringValue(proposal.Metadata["executionDecision"]); got != "sl-slippage-protected" {
		t.Fatalf("expected sl-slippage-protected, got %s", got)
	}
	if got := proposal.LimitPrice; got != 68013.85 {
		t.Fatalf("expected spread-capped SL protection price 68013.85, got %v", got)
	}
	if got := stringValue(proposal.Metadata["executionExpiresAt"]); got != eventTime.Add(15*time.Second).Format(time.RFC3339) {
		t.Fatalf("expected configured SL expiry, got %s", got)
	}
	if !boolValue(mapValue(proposal.Metadata["executionDecisionContext"])["slProtectionBranch"]) {
		t.Fatal("expected explicit SL protection branch marker")
	}
	if got := stringValue(mapValue(proposal.Metadata["executionDecisionContext"])["slProtectionDepthMode"]); got != "spread-capped-fallback" {
		t.Fatalf("expected fallback SL depth mode without qty data, got %s", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionUsesCappedPriceWhenTopBookOutsideCap(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("SELL", 0.5, 68000, 68150, 1.2, 0, 68000, 20)
	if got := decision.Price; got != 68013.85 {
		t.Fatalf("expected capped protection price 68013.85, got %v", got)
	}
	if got := decision.DepthMode; got != "top-book-outside-cap" {
		t.Fatalf("expected top-book-outside-cap mode, got %s", got)
	}
	if got := decision.TopDepthNotional; got != 81600 {
		t.Fatalf("expected top depth notional 81600, got %v", got)
	}
	if got := decision.ExpectedCoverage; got != 1 {
		t.Fatalf("expected full coverage, got %v", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionRecordsPartialCoverageWhenTopBookOutsideCap(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("SELL", 2.0, 68000, 68150, 1.0, 0, 68000, 20)
	if got := decision.DepthMode; got != "top-book-outside-cap" {
		t.Fatalf("expected top-book-outside-cap mode, got %s", got)
	}
	if got := decision.ExpectedCoverage; got != 0.5 {
		t.Fatalf("expected 50%% coverage, got %v", got)
	}
	if got := decision.Price; got != 68013.85 {
		t.Fatalf("expected capped protection price 68013.85, got %v", got)
	}
	if got := decision.QuoteGapBps; got <= 0 {
		t.Fatalf("expected positive quote gap bps, got %v", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionRecordsPartialCoverageWhenTopBookOutsideCapForBuy(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("BUY", 2.0, 68000, 68150, 0, 1.0, 68150, 20)
	if got := decision.DepthMode; got != "top-book-outside-cap" {
		t.Fatalf("expected top-book-outside-cap mode, got %s", got)
	}
	if got := decision.ExpectedCoverage; got != 0.5 {
		t.Fatalf("expected 50%% coverage, got %v", got)
	}
	if got := decision.Price; got != 68136.15 {
		t.Fatalf("expected capped protection price 68136.15, got %v", got)
	}
	if got := decision.QuoteGapBps; got <= 0 {
		t.Fatalf("expected positive quote gap bps, got %v", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionUsesTopBookWhenWithinCapForBuy(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("BUY", 0.5, 68000, 68010, 0, 1.2, 68010, 20)
	if got := decision.DepthMode; got != "top-book-cover-within-cap" {
		t.Fatalf("expected top-book-cover-within-cap mode, got %s", got)
	}
	if got := decision.Price; got != 68010 {
		t.Fatalf("expected top-book ask price 68010, got %v", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionRecordsPartialCoverageWhenWithinCapForBuy(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("BUY", 2.0, 68000, 68010, 0, 1.0, 68010, 20)
	if got := decision.DepthMode; got != "top-book-partial-within-cap" {
		t.Fatalf("expected top-book-partial-within-cap mode, got %s", got)
	}
	if got := decision.ExpectedCoverage; got != 0.5 {
		t.Fatalf("expected 50%% coverage, got %v", got)
	}
	if got := decision.Price; got != 68010 {
		t.Fatalf("expected within-cap BUY price to remain at ask 68010, got %v", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionUsesTopBookWhenWithinCap(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("SELL", 0.5, 68000, 68010, 1.2, 0, 68000, 20)
	if got := decision.DepthMode; got != "top-book-cover-within-cap" {
		t.Fatalf("expected top-book-cover-within-cap mode, got %s", got)
	}
	if got := decision.Price; got != 68000 {
		t.Fatalf("expected top-book bid price 68000, got %v", got)
	}
}

func TestResolveAggressiveSLProtectionDecisionRecordsPartialCoverageWhenWithinCap(t *testing.T) {
	decision := resolveAggressiveSLProtectionDecision("SELL", 2.0, 68000, 68010, 1.0, 0, 68000, 20)
	if got := decision.DepthMode; got != "top-book-partial-within-cap" {
		t.Fatalf("expected top-book-partial-within-cap mode, got %s", got)
	}
	if got := decision.Price; got != 68000 {
		t.Fatalf("expected capped price to remain at 68000, got %v", got)
	}
	if got := decision.ExpectedCoverage; got != 0.5 {
		t.Fatalf("expected 50%% coverage, got %v", got)
	}
}

func TestBookAwareExecutionStrategyUsesFallbackOrderAfterTimeoutMatch(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	intent := SignalIntent{
		Action:      "entry",
		Role:        "entry",
		Reason:      "Initial",
		Side:        "BUY",
		Symbol:      "BTCUSDT",
		SignalKind:  "entry",
		PriceHint:   68000,
		PriceSource: "trade_tick.price",
		Metadata: map[string]any{
			"bestBid":           67999.0,
			"bestAsk":           68005.0,
			"spreadBps":         12.0,
			"signalBarStateKey": "state-1",
		},
	}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"lastExecutionTimeoutIntentSignature": buildLiveIntentSignature(map[string]any{
					"action":            "entry",
					"side":              "BUY",
					"symbol":            "BTCUSDT",
					"signalKind":        "entry",
					"signalBarStateKey": "state-1",
				}),
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps":             5.0,
				"executionTimeoutFallbackOrderType": "MARKET",
			},
		},
		Intent: intent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "MARKET" {
		t.Fatalf("expected MARKET fallback proposal, got %s", proposal.Type)
	}
	if !boolValue(proposal.Metadata["fallbackFromTimeout"]) {
		t.Fatal("expected proposal to record fallbackFromTimeout")
	}
}

func TestBookAwareExecutionStrategyBuildsLimitProposalWhenConfigured(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionOrderType":   "LIMIT",
				"executionTimeInForce": "IOC",
				"executionPostOnly":    true,
			},
		},
		Intent: SignalIntent{
			Action:      "entry",
			Role:        "entry",
			Reason:      "Initial",
			Side:        "BUY",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "trade_tick.price",
			Metadata: map[string]any{
				"bestAsk":   68001.0,
				"spreadBps": 1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "LIMIT" {
		t.Fatalf("expected LIMIT proposal, got %s", proposal.Type)
	}
	if proposal.LimitPrice != 68001.0 {
		t.Fatalf("expected best ask as limit price, got %v", proposal.LimitPrice)
	}
	if proposal.TimeInForce != "GTX" {
		t.Fatalf("expected GTX time in force for post only limit, got %s", proposal.TimeInForce)
	}
	if !proposal.PostOnly {
		t.Fatal("expected post only proposal")
	}
}

func TestBuildLiveOrderFromExecutionProposalUsesExecutionFields(t *testing.T) {
	session := domain.LiveSession{
		ID:        "live-session-1",
		AccountID: "live-main",
		State: map[string]any{
			"dispatchMode": "auto-dispatch",
		},
	}
	proposal := ExecutionProposal{
		Side:              "BUY",
		Symbol:            "BTCUSDT",
		Type:              "LIMIT",
		Quantity:          0.002,
		LimitPrice:        68001.0,
		PriceHint:         68001.5,
		TimeInForce:       "GTX",
		PostOnly:          true,
		ReduceOnly:        true,
		SignalKind:        "entry",
		ExecutionStrategy: "book-aware-v1",
	}
	order := buildLiveOrderFromExecutionProposal(session, "strategy-version-1", proposal, executionProposalToMap(proposal))
	if order.Type != "LIMIT" {
		t.Fatalf("expected LIMIT order, got %s", order.Type)
	}
	if order.Price != 68001.0 {
		t.Fatalf("expected limit price to be used, got %v", order.Price)
	}
	if got := stringValue(order.Metadata["timeInForce"]); got != "GTX" {
		t.Fatalf("expected GTX in metadata, got %s", got)
	}
	if !boolValue(order.Metadata["postOnly"]) {
		t.Fatal("expected postOnly metadata")
	}
	if !boolValue(order.Metadata["reduceOnly"]) {
		t.Fatal("expected reduceOnly metadata")
	}
	if !order.ReduceOnly {
		t.Fatal("expected reduceOnly formal field")
	}
}

func TestBuildLiveOrderUsesProposalQuantityOverSessionDefault(t *testing.T) {
	session := domain.LiveSession{
		ID:        "live-session-1",
		AccountID: "live-main",
		State: map[string]any{
			"symbol":               "BTCUSDT",
			"defaultOrderQuantity": 0.01,
			"dispatchMode":         "manual-review",
		},
	}
	proposal := ExecutionProposal{
		Action:            "entry",
		Role:              "entry",
		Reason:            "SL-Reentry",
		Side:              "BUY",
		Symbol:            "BTCUSDT",
		Type:              "MARKET",
		Quantity:          0.002,
		PriceHint:         68000,
		ExecutionStrategy: "book-aware-v1",
		SignalKind:        "sl-reentry",
	}
	order := buildLiveOrderFromExecutionProposal(session, "strategy-version-1", proposal, executionProposalToMap(proposal))
	if order.Quantity != 0.002 {
		t.Fatalf("expected proposal quantity to win, got %v", order.Quantity)
	}
}

func TestUpdateExecutionEventStatsMarksEventAggregationSemantics(t *testing.T) {
	state := map[string]any{}
	proposal := map[string]any{
		"status":    "dispatchable",
		"spreadBps": 1.2,
		"metadata": map[string]any{
			"executionDecision": "maker-resting",
			"bookImbalance":     0.3,
		},
	}
	dispatch := map[string]any{
		"status":        "FILLED",
		"orderType":     "LIMIT",
		"reduceOnly":    true,
		"priceDriftBps": 0.8,
	}
	updateExecutionEventStats(state, proposal, dispatch)
	stats := mapValue(state["executionEventStats"])
	if got := stringValue(stats["aggregationMode"]); got != "event" {
		t.Fatalf("expected event aggregation mode, got %s", got)
	}
	if boolValue(stats["deduplicated"]) {
		t.Fatal("expected event stats to remain explicitly non-deduplicated")
	}
	if got := maxIntValue(stats["proposalCount"], 0); got != 1 {
		t.Fatalf("expected one proposal event, got %d", got)
	}
	if got := maxIntValue(stats["dispatchEventCount"], 0); got != 1 {
		t.Fatalf("expected one dispatch event, got %d", got)
	}
}

func TestDispatchLiveSessionIntentRejectsNonDispatchableProposal(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session := domain.LiveSession{
		ID:         "live-session-1",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "RUNNING",
		State: map[string]any{
			"lastExecutionProposal": map[string]any{
				"status": "wait",
				"side":   "BUY",
				"symbol": "BTCUSDT",
			},
		},
	}
	if _, err := platform.dispatchLiveSessionIntent(session); err == nil {
		t.Fatal("expected non-dispatchable proposal to be rejected")
	}
}

func TestDispatchLiveSessionIntentBackfillsMissingDecisionEventReference(t *testing.T) {
	platform, session, runtimeSessionID, _, _ := prepareLiveDecisionTelemetryFixture(t)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-decision-event-backfill"})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-decision-event-backfill",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	freshRuntimeAt := time.Now().UTC()
	if err := platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		state := cloneMetadata(runtimeSession.State)
		state["lastEventAt"] = freshRuntimeAt.Format(time.RFC3339)
		state["lastHeartbeatAt"] = freshRuntimeAt.Format(time.RFC3339)
		sourceStates := cloneMetadata(mapValue(state["sourceStates"]))
		for key, item := range sourceStates {
			sourceState := cloneMetadata(mapValue(item))
			sourceState["lastEventAt"] = freshRuntimeAt.Format(time.RFC3339)
			sourceStates[key] = sourceState
		}
		state["sourceStates"] = sourceStates
		runtimeSession.State = state
		runtimeSession.UpdatedAt = freshRuntimeAt
	}); err != nil {
		t.Fatalf("refresh runtime state failed: %v", err)
	}

	missingDecisionEventID := "strategy-decision-event-missing-live-dispatch"
	proposal := executionProposalToMap(ExecutionProposal{
		Action:            "entry",
		Role:              "entry",
		Reason:            "SL-Reentry",
		Side:              "SELL",
		Symbol:            "BTCUSDT",
		Type:              "LIMIT",
		Quantity:          0.002,
		LimitPrice:        75600.0,
		PriceHint:         75600.0,
		PriceSource:       "test-book",
		TimeInForce:       "GTC",
		SignalKind:        "sl-reentry",
		DecisionState:     "entry-ready",
		SignalBarStateKey: "binance-kline|BTCUSDT|signal|1d",
		ExecutionStrategy: "book-aware-v1",
		Status:            "dispatchable",
		Metadata: map[string]any{
			"runtimeSessionId":  runtimeSessionID,
			"executionDecision": "maker-resting",
			"executionMode":     "live",
		},
	})
	proposal = setExecutionProposalDecisionEventID(proposal, missingDecisionEventID)
	state := cloneMetadata(session.State)
	state["lastExecutionProposal"] = proposal
	state["lastStrategyIntent"] = proposal
	state["lastStrategyIntentSignature"] = buildLiveIntentSignature(proposal)
	state["lastStrategyEvaluationContext"] = map[string]any{
		"strategyVersionId":   "strategy-version-bk-1d-v010",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"symbol":              "BTCUSDT",
	}
	state["lastStrategyDecision"] = map[string]any{
		"action": "advance-plan",
		"reason": "SL-Reentry",
		"metadata": map[string]any{
			"signalKind":    "sl-reentry",
			"decisionState": "entry-ready",
		},
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	order, err := platform.dispatchLiveSessionIntent(session)
	if err != nil {
		t.Fatalf("dispatch live session intent failed: %v", err)
	}
	if got := stringValue(order.Metadata["decisionEventId"]); got != missingDecisionEventID {
		t.Fatalf("expected order decisionEventId %s, got %s", missingDecisionEventID, got)
	}
	updatedSession, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updatedSession.State["lastStrategyDecisionEventId"]); got != missingDecisionEventID {
		t.Fatalf("expected session lastStrategyDecisionEventId %s, got %s", missingDecisionEventID, got)
	}

	events, err := platform.store.ListStrategyDecisionEvents(session.ID)
	if err != nil {
		t.Fatalf("list strategy decision events failed: %v", err)
	}
	var backfilled domain.StrategyDecisionEvent
	for _, event := range events {
		if event.ID == missingDecisionEventID {
			backfilled = event
			break
		}
	}
	if backfilled.ID == "" {
		t.Fatalf("expected missing decision event %s to be backfilled", missingDecisionEventID)
	}
	if !boolValue(backfilled.DecisionMetadata["decisionEventBackfilled"]) {
		t.Fatalf("expected backfilled decision metadata, got %+v", backfilled.DecisionMetadata)
	}
	if got := stringValue(backfilled.ExecutionProposal["decisionEventId"]); got != missingDecisionEventID {
		t.Fatalf("expected backfilled execution proposal decisionEventId %s, got %s", missingDecisionEventID, got)
	}

	executionEvents, err := platform.store.ListOrderExecutionEvents(order.ID)
	if err != nil {
		t.Fatalf("list order execution events failed: %v", err)
	}
	if len(executionEvents) == 0 {
		t.Fatal("expected order execution telemetry to be persisted")
	}
	if got := executionEvents[0].DecisionEventID; got != missingDecisionEventID {
		t.Fatalf("expected order execution event decisionEventId %s, got %s", missingDecisionEventID, got)
	}
}

func TestDispatchLiveSessionIntentRejectsRecoveredPassiveCloseWithIncompleteMetadata(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	delete(state, "signalRuntimeSessionId")
	delete(state, "lastSignalRuntimeSessionId")
	delete(state, "executionDataSource")
	state["positionRecoveryStatus"] = livePositionRecoveryStatusClosingPending
	state["hasRecoveredPosition"] = true
	state["hasRecoveredRealPosition"] = true
	state["lastExecutionProposal"] = executionProposalToMap(ExecutionProposal{
		Action:            "risk-exit-fallback",
		Role:              "exit",
		Reason:            "sl-breached-fallback",
		Side:              "SELL",
		Symbol:            "BTCUSDT",
		Type:              "MARKET",
		Quantity:          0.002,
		PriceHint:         68900.0,
		PriceSource:       "fallback-watchdog",
		TimeInForce:       "GTC",
		ReduceOnly:        true,
		SignalKind:        "recovery-watchdog",
		DecisionState:     "unprotected",
		ExecutionStrategy: "book-aware-v1",
		Status:            "dispatchable",
		Metadata: map[string]any{
			"recoveryTriggered": true,
		},
	})
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	if _, err := platform.dispatchLiveSessionIntent(session); err == nil {
		t.Fatal("expected incomplete recovered passive close metadata to be rejected")
	}
}

func TestDispatchLiveSessionIntentRejectsRecoveredPassiveCloseWithoutCurrentRuntimeLink(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	delete(state, "signalRuntimeSessionId")
	delete(state, "lastSignalRuntimeSessionId")
	state["positionRecoveryStatus"] = livePositionRecoveryStatusClosingPending
	state["hasRecoveredPosition"] = true
	state["hasRecoveredRealPosition"] = true
	state["lastStrategyEvaluationContext"] = map[string]any{
		"strategyVersionId":   "strategy-version-bk-1d-v010",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"symbol":              "BTCUSDT",
	}
	state["lastExecutionProposal"] = executionProposalToMap(ExecutionProposal{
		Action:            "risk-exit-fallback",
		Role:              "exit",
		Reason:            "sl-breached-fallback",
		Side:              "SELL",
		Symbol:            "BTCUSDT",
		Type:              "MARKET",
		Quantity:          0.002,
		PriceHint:         68900.0,
		PriceSource:       "fallback-watchdog",
		TimeInForce:       "GTC",
		ReduceOnly:        true,
		SignalKind:        "recovery-watchdog",
		DecisionState:     "unprotected",
		ExecutionStrategy: "book-aware-v1",
		Status:            "dispatchable",
		Metadata: map[string]any{
			"recoveryTriggered": true,
		},
	})
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	if _, err := platform.dispatchLiveSessionIntent(session); err == nil {
		t.Fatal("expected recovered passive close without current runtime link to be rejected")
	} else if !strings.Contains(err.Error(), "runtimeSessionId") {
		t.Fatalf("expected runtimeSessionId validation error, got %v", err)
	}
}

func TestIsRecoveryTriggeredPassiveCloseProposalRequiresExplicitRecoverySignal(t *testing.T) {
	if isRecoveryTriggeredPassiveCloseProposal(map[string]any{
		"role":       "exit",
		"reduceOnly": true,
		"reason":     "sl-breached-fallback",
		"signalKind": "protect-exit",
	}) {
		t.Fatal("expected reason-only passive close not to be treated as recovery-triggered")
	}
	if !isRecoveryTriggeredPassiveCloseProposal(map[string]any{
		"role":       "exit",
		"reduceOnly": true,
		"reason":     "sl-breached-fallback",
		"signalKind": "recovery-watchdog",
	}) {
		t.Fatal("expected recovery-watchdog passive close to be treated as recovery-triggered")
	}
}

func TestNormalizeLiveSessionOverridesIncludesExecutionControls(t *testing.T) {
	overrides := normalizeLiveSessionOverrides(map[string]any{
		"executionStrategy":                   "book-aware-v1",
		"executionOrderType":                  "limit",
		"executionTimeInForce":                "ioc",
		"executionPostOnly":                   true,
		"executionMaxSpreadBps":               6.5,
		"executionWideSpreadMode":             "limit-maker",
		"executionRestingTimeoutSeconds":      25,
		"executionTimeoutFallbackOrderType":   "market",
		"executionTimeoutFallbackTimeInForce": "fok",
		"executionPTExitOrderType":            "limit",
		"executionPTExitPostOnly":             true,
		"executionPTExitTimeInForce":          "gtx",
		"executionSLExitOrderType":            "market",
		"executionSLExitMaxSpreadBps":         999.0,
	})
	if got := stringValue(overrides["executionStrategy"]); got != "book-aware-v1" {
		t.Fatalf("expected execution strategy override, got %s", got)
	}
	if got := stringValue(overrides["executionOrderType"]); got != "limit" {
		t.Fatalf("expected execution order type override, got %s", got)
	}
	if got := stringValue(overrides["executionTimeInForce"]); got != "IOC" {
		t.Fatalf("expected uppercase time in force, got %s", got)
	}
	if !boolValue(overrides["executionPostOnly"]) {
		t.Fatal("expected executionPostOnly override")
	}
	if got := parseFloatValue(overrides["executionMaxSpreadBps"]); got != 6.5 {
		t.Fatalf("expected execution max spread override, got %v", got)
	}
	if got := stringValue(overrides["executionWideSpreadMode"]); got != "limit-maker" {
		t.Fatalf("expected wide spread mode override, got %s", got)
	}
	if got := maxIntValue(overrides["executionRestingTimeoutSeconds"], 0); got != 25 {
		t.Fatalf("expected resting timeout override, got %d", got)
	}
	if got := stringValue(overrides["executionTimeoutFallbackOrderType"]); got != "MARKET" {
		t.Fatalf("expected uppercase fallback order type, got %s", got)
	}
	if got := stringValue(overrides["executionTimeoutFallbackTimeInForce"]); got != "FOK" {
		t.Fatalf("expected uppercase fallback tif, got %s", got)
	}
	if got := stringValue(overrides["executionPTExitOrderType"]); got != "LIMIT" {
		t.Fatalf("expected PT exit order type override, got %s", got)
	}
	if !boolValue(overrides["executionPTExitPostOnly"]) {
		t.Fatal("expected PT exit post only override")
	}
	if got := stringValue(overrides["executionPTExitTimeInForce"]); got != "GTX" {
		t.Fatalf("expected PT exit tif override, got %s", got)
	}
	if got := stringValue(overrides["executionSLExitOrderType"]); got != "MARKET" {
		t.Fatalf("expected SL exit order type override, got %s", got)
	}
	if got := parseFloatValue(overrides["executionSLExitMaxSpreadBps"]); got != 999.0 {
		t.Fatalf("expected SL exit max spread override, got %v", got)
	}
}

func TestNormalizeLiveSessionOverridesIncludesPositionSizing(t *testing.T) {
	overrides := normalizeLiveSessionOverrides(map[string]any{
		"positionSizingMode":   "fixed-fraction",
		"defaultOrderFraction": 0.12,
	})
	if got := stringValue(overrides["positionSizingMode"]); got != "fixed_fraction" {
		t.Fatalf("expected fixed_fraction mode, got %s", got)
	}
	if got := parseFloatValue(overrides["defaultOrderFraction"]); got != 0.12 {
		t.Fatalf("expected default order fraction 0.12, got %v", got)
	}
}

func TestNormalizeLiveSessionOverridesIncludesZeroInitialControls(t *testing.T) {
	overrides := normalizeLiveSessionOverrides(map[string]any{
		"dir2_zero_initial":               false,
		"zero_initial_mode":               "position",
		"trailing_stop_atr":               0.3,
		"delayed_trailing_activation_atr": 0.5,
	})
	if boolValue(overrides["dir2_zero_initial"]) {
		t.Fatal("expected dir2_zero_initial override to be preserved")
	}
	if got := stringValue(overrides["zero_initial_mode"]); got != "position" {
		t.Fatalf("expected zero_initial_mode=position, got %s", got)
	}
	if got := parseFloatValue(overrides["trailing_stop_atr"]); got != 0.3 {
		t.Fatalf("expected trailing_stop_atr=0.3, got %v", got)
	}
	if got := parseFloatValue(overrides["delayed_trailing_activation_atr"]); got != 0.5 {
		t.Fatalf("expected delayed_trailing_activation_atr=0.5, got %v", got)
	}
}

func TestCreateLiveSessionAppliesExecutionOverrides(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":                              "BTCUSDT",
		"executionStrategy":                   "book-aware-v1",
		"executionOrderType":                  "LIMIT",
		"executionTimeInForce":                "IOC",
		"executionPostOnly":                   true,
		"executionWideSpreadMode":             "limit-maker",
		"executionRestingTimeoutSeconds":      30,
		"executionTimeoutFallbackOrderType":   "MARKET",
		"executionTimeoutFallbackTimeInForce": "FOK",
		"executionPTExitOrderType":            "LIMIT",
		"executionPTExitPostOnly":             true,
		"executionPTExitTimeInForce":          "GTX",
		"executionSLExitOrderType":            "MARKET",
		"executionSLExitMaxSpreadBps":         999.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stringValue(session.State["executionStrategy"]); got != "book-aware-v1" {
		t.Fatalf("expected execution strategy in session state, got %s", got)
	}
	if got := stringValue(session.State["executionOrderType"]); got != "LIMIT" {
		t.Fatalf("expected execution order type in session state, got %s", got)
	}
	if got := stringValue(session.State["executionTimeInForce"]); got != "IOC" {
		t.Fatalf("expected execution tif in session state, got %s", got)
	}
	if !boolValue(session.State["executionPostOnly"]) {
		t.Fatal("expected executionPostOnly in session state")
	}
	if got := stringValue(session.State["executionWideSpreadMode"]); got != "limit-maker" {
		t.Fatalf("expected executionWideSpreadMode in session state, got %s", got)
	}
	if got := maxIntValue(session.State["executionRestingTimeoutSeconds"], 0); got != 30 {
		t.Fatalf("expected resting timeout in session state, got %d", got)
	}
	if got := stringValue(session.State["executionTimeoutFallbackOrderType"]); got != "MARKET" {
		t.Fatalf("expected fallback order type in session state, got %s", got)
	}
	if got := stringValue(session.State["executionTimeoutFallbackTimeInForce"]); got != "FOK" {
		t.Fatalf("expected fallback tif in session state, got %s", got)
	}
	if got := stringValue(session.State["executionPTExitOrderType"]); got != "LIMIT" {
		t.Fatalf("expected PT exit order type in session state, got %s", got)
	}
	if !boolValue(session.State["executionPTExitPostOnly"]) {
		t.Fatal("expected PT exit post only in session state")
	}
	if got := stringValue(session.State["executionSLExitOrderType"]); got != "MARKET" {
		t.Fatalf("expected SL exit order type in session state, got %s", got)
	}
	if got := parseFloatValue(session.State["executionSLExitMaxSpreadBps"]); got != 999.0 {
		t.Fatalf("expected SL exit max spread in session state, got %v", got)
	}
}

func TestStartSignalRuntimeSessionIncludesAllBoundTimeframes(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy 1d failed: %v", err)
	}
	session, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "4h"},
	}); err != nil {
		t.Fatalf("rebind strategy 4h failed: %v", err)
	}
	started, err := platform.StartSignalRuntimeSession(session.ID)
	if err != nil {
		t.Fatalf("start runtime session failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = platform.StopSignalRuntimeSession(session.ID)
	})
	subscriptions := metadataList(started.State["subscriptions"])
	if len(subscriptions) == 0 {
		t.Fatal("expected subscriptions after runtime start")
	}
	channels := map[string]struct{}{}
	for _, subscription := range subscriptions {
		channels[stringValue(subscription["channel"])] = struct{}{}
	}
	for _, expected := range []string{"btcusdt@kline_1d", "btcusdt@kline_4h"} {
		if _, ok := channels[expected]; !ok {
			t.Fatalf("expected subscription %s, got %#v", expected, subscriptions)
		}
	}
}

func TestBuildSignalRuntimePlanUsesStrategyBindingsWithoutAccountBindings(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "4h"},
	}); err != nil {
		t.Fatalf("bind strategy 4h failed: %v", err)
	}
	plan, err := platform.BuildSignalRuntimePlan("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	if !boolValue(plan["ready"]) {
		t.Fatalf("expected runtime plan to be ready from strategy bindings only: %#v", plan)
	}
	missing := metadataList(plan["missingBindings"])
	if len(missing) != 0 {
		t.Fatalf("expected no missing bindings, got %#v", missing)
	}
	subscriptions := metadataList(plan["subscriptions"])
	if len(subscriptions) != 1 {
		t.Fatalf("expected one subscription, got %#v", subscriptions)
	}
	if got := stringValue(subscriptions[0]["channel"]); got != "btcusdt@kline_4h" {
		t.Fatalf("expected 4h strategy binding subscription, got %s", got)
	}
	matched := metadataList(plan["matchedBindings"])
	if len(matched) != 1 {
		t.Fatalf("expected one matched binding, got %#v", matched)
	}
	if accountBinding := mapValue(matched[0]["accountBinding"]); accountBinding != nil {
		t.Fatalf("expected account binding to be nil after account-signal removal, got %#v", accountBinding)
	}
}

func TestEnsureLaunchLiveSessionCreatesDistinctSessionPerSymbolAndTimeframe(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	first, created, err := platform.ensureLaunchLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("ensure first live session failed: %v", err)
	}
	if !created {
		t.Fatal("expected first launch live session to be created")
	}

	second, created, err := platform.ensureLaunchLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "ETHUSDT",
		"signalTimeframe": "4h",
	})
	if err != nil {
		t.Fatalf("ensure second live session failed: %v", err)
	}
	if !created {
		t.Fatal("expected second launch live session to be created")
	}
	if first.ID == second.ID {
		t.Fatalf("expected distinct live sessions for different launch scopes, got %s", first.ID)
	}

	reused, created, err := platform.ensureLaunchLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("ensure reused live session failed: %v", err)
	}
	if created {
		t.Fatal("expected matching launch scope to reuse existing live session")
	}
	if reused.ID != first.ID {
		t.Fatalf("expected reused live session %s, got %s", first.ID, reused.ID)
	}
}

func TestCreateLiveSessionPersistsLaunchScopeIntoState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "5m",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if got := stringValue(session.State["symbol"]); got != "BTCUSDT" {
		t.Fatalf("expected session state symbol BTCUSDT, got %q", got)
	}
	if got := stringValue(session.State["signalTimeframe"]); got != "5m" {
		t.Fatalf("expected session state signalTimeframe 5m, got %q", got)
	}
}

func TestUpdateLiveSessionAliasClearing(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, _ := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{"symbol": "BTCUSDT"})

	// 1. Set alias
	updated, err := platform.UpdateLiveSession(session.ID, "New Alias", "", "", map[string]any{})
	if err != nil {
		t.Fatalf("set alias failed: %v", err)
	}
	if updated.Alias != "New Alias" {
		t.Fatalf("expected alias 'New Alias', got %q", updated.Alias)
	}

	// 2. Clear alias (provide empty string)
	cleared, err := platform.UpdateLiveSession(session.ID, "  ", "", "", map[string]any{})
	if err != nil {
		t.Fatalf("clear alias failed: %v", err)
	}
	if cleared.Alias != "" {
		t.Fatalf("expected cleared alias (empty string), got %q", cleared.Alias)
	}
}

func TestIsLivePlanStepStaleUsesFiveMinuteTimeframe(t *testing.T) {
	nextPlannedEvent := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	if isLivePlanStepStale(nextPlannedEvent, "5m", nextPlannedEvent.Add(4*time.Minute+59*time.Second)) {
		t.Fatal("expected 5m plan step to remain fresh before the next 5m boundary")
	}
	if !isLivePlanStepStale(nextPlannedEvent, "5m", nextPlannedEvent.Add(5*time.Minute+time.Second)) {
		t.Fatal("expected 5m plan step to be stale after the next 5m boundary")
	}
}

func TestShouldCancelLiveOrderForExecutionTimeout(t *testing.T) {
	now := time.Date(2026, 4, 7, 8, 30, 0, 0, time.UTC)
	order := domain.Order{
		Status: "NEW",
		Metadata: map[string]any{
			"executionExpiresAt": now.Add(-time.Second).Format(time.RFC3339),
		},
	}
	if !shouldCancelLiveOrderForExecutionTimeout(order, now) {
		t.Fatal("expected expired live order to be cancelled")
	}
}

func TestMaybeIncrementLiveSessionReentryCountOnlyCountsFilledReentries(t *testing.T) {
	state := map[string]any{
		"sessionReentryCount": 0.0,
	}
	proposal := map[string]any{
		"reason":            "SL-Reentry",
		"signalBarStateKey": "bar-1",
	}
	maybeIncrementLiveSessionReentryCount(state, proposal, "order-1", "NEW")
	if got := parseFloatValue(state["sessionReentryCount"]); got != 0 {
		t.Fatalf("expected no increment for NEW order, got %v", got)
	}
	maybeIncrementLiveSessionReentryCount(state, proposal, "order-1", "FILLED")
	if got := parseFloatValue(state["sessionReentryCount"]); got != 1 {
		t.Fatalf("expected increment on FILLED reentry, got %v", got)
	}
	maybeIncrementLiveSessionReentryCount(state, proposal, "order-1", "FILLED")
	if got := parseFloatValue(state["sessionReentryCount"]); got != 1 {
		t.Fatalf("expected duplicate FILLED sync to be ignored, got %v", got)
	}
}

func TestEvaluateExecutionQualityDoesNotTreatCancelsAsRejections(t *testing.T) {
	state := map[string]any{
		"executionEventStats": map[string]any{
			"filledCount":              4,
			"rejectedCount":            0,
			"cancelledCount":           4,
			"avgPriceDriftBps":         1.0,
			"avgProposalSpreadBps":     2.0,
			"slProtectedDispatchCount": 0,
		},
	}
	evaluateExecutionQuality(state)
	if got := stringValue(state["executionQuality"]); got != "degraded" {
		t.Fatalf("expected degraded quality from excessive cancels, got %s", got)
	}
	rawReasons, _ := state["executionQualityReasons"].([]string)
	gotReasons := rawReasons
	for _, reason := range gotReasons {
		if strings.HasPrefix(reason, "high-rejection:") {
			t.Fatalf("did not expect high-rejection reason for pure cancels: %v", gotReasons)
		}
	}
}

func TestShouldAutoDispatchLiveIntentAllowsRetryAfterExecutionTimeout(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                        "auto-dispatch",
			"lastDispatchedOrderStatus":           "CANCELLED",
			"lastSyncedOrderStatus":               "CANCELLED",
			"lastDispatchedIntentSignature":       signature,
			"lastExecutionTimeoutIntentSignature": signature,
			"lastDispatchedAt":                    now.Format(time.RFC3339),
			"dispatchCooldownSeconds":             300,
		},
	}
	if !shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected timeout-cancelled intent to be eligible for immediate retry")
	}
}

func TestShouldAutoDispatchLiveIntentAllowsRetryAfterMakerRejectFallback(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                        "auto-dispatch",
			"lastDispatchedOrderStatus":           "REJECTED",
			"lastSyncedOrderStatus":               "REJECTED",
			"lastDispatchedIntentSignature":       signature,
			"lastExecutionTimeoutIntentSignature": signature,
			"lastExecutionTimeoutReason":          "maker-rejected-post-only",
			"lastDispatchedAt":                    now.Format(time.RFC3339),
			"dispatchCooldownSeconds":             300,
		},
	}
	if !shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected maker-rejected intent to be eligible for immediate fallback retry")
	}
}

func TestShouldAutoDispatchLiveIntentBlocksImmediateRetryAfterDispatchError(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                  "auto-dispatch",
			"lastDispatchedOrderStatus":     "REJECTED",
			"lastDispatchedIntentSignature": signature,
			"lastDispatchedAt":              now.Format(time.RFC3339),
			"dispatchCooldownSeconds":       30,
		},
	}
	if shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected immediate retry after dispatch error to be blocked by cooldown")
	}
}

func TestShouldAutoDispatchLiveIntentBlocksOpenOrder(t *testing.T) {
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":              "auto-dispatch",
			"lastDispatchedOrderStatus": "ACCEPTED",
		},
	}
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	if shouldAutoDispatchLiveIntent(session, intent, time.Now().UTC()) {
		t.Fatal("expected open order to block auto dispatch")
	}
}

func TestShouldAutoDispatchLiveIntentAllowsTerminalOrder(t *testing.T) {
	now := time.Now().UTC()
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                  "auto-dispatch",
			"lastDispatchedOrderStatus":     "FILLED",
			"lastDispatchedIntentSignature": "entry|BUY|BTCUSDT|initial-entry|state-0",
			"lastDispatchedAt":              now.Add(-time.Minute).Format(time.RFC3339),
			"dispatchCooldownSeconds":       5,
		},
	}
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	if !shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected terminal order to allow auto dispatch for new intent")
	}
}

func TestShouldAutoDispatchLiveIntentBlocksUnresolvedRecoveryWatchdogClose(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":     "risk-exit-fallback",
		"role":       "exit",
		"reason":     "sl-breached-fallback",
		"side":       "SELL",
		"symbol":     "BTCUSDT",
		"signalKind": "recovery-watchdog",
		"reduceOnly": true,
		"status":     "dispatchable",
		"metadata": map[string]any{
			"strategyVersionId": "strategy-version-bk-1d-v010",
			"runtimeSessionId":  "runtime-1",
			"executionContext": map[string]any{
				"strategyVersionId":   "strategy-version-bk-1d-v010",
				"signalTimeframe":     "1d",
				"executionDataSource": "tick",
				"symbol":              "BTCUSDT",
			},
		},
	}
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":             "auto-dispatch",
			"positionRecoveryStatus":   "unprotected-open-position",
			"hasRecoveredPosition":     true,
			"hasRecoveredRealPosition": true,
		},
	}
	if shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected unresolved recovery watchdog close to stay manual")
	}
}

func TestShouldAdvanceLivePlanForOrderStatus(t *testing.T) {
	if shouldAdvanceLivePlanForOrderStatus("REJECTED") {
		t.Fatal("expected rejected live order to keep the current plan step actionable")
	}
	if shouldAdvanceLivePlanForOrderStatus("") {
		t.Fatal("expected empty status to keep the current plan step actionable")
	}
	if shouldAdvanceLivePlanForOrderStatus("UNKNOWN") {
		t.Fatal("expected unknown status to keep the current plan step actionable")
	}
	if !shouldAdvanceLivePlanForOrderStatus("NEW") {
		t.Fatal("expected accepted/in-flight live order to advance the plan")
	}
	if !shouldAdvanceLivePlanForOrderStatus("FILLED") {
		t.Fatal("expected filled live order to advance the plan")
	}
}

func TestLiveRecoveryActionMatrixUsesExplicitTakeoverStates(t *testing.T) {
	cases := []struct {
		state                string
		openNewPosition      bool
		closeExisting        bool
		placeProtection      bool
		autoDispatch         bool
		manualReviewRequired bool
		allowPlanProgression bool
	}{
		{state: liveRecoveryModeCloseOnlyTakeover, closeExisting: true, manualReviewRequired: true},
		{state: liveRecoveryTakeoverStateMonitoring, closeExisting: true, placeProtection: true, manualReviewRequired: true, allowPlanProgression: true},
		{state: liveRecoveryTakeoverStateUnprotected, closeExisting: true, placeProtection: true, manualReviewRequired: true},
		{state: liveRecoveryTakeoverStateStaleSync, manualReviewRequired: true},
		{state: liveRecoveryTakeoverStateConflict, manualReviewRequired: true},
		{state: liveRecoveryTakeoverStateError, manualReviewRequired: true},
	}
	for _, tc := range cases {
		matrix := liveRecoveryActionMatrixForState(tc.state)
		if matrix.OpenNewPosition != tc.openNewPosition ||
			matrix.CloseExistingPosition != tc.closeExisting ||
			matrix.PlaceProtectionOrders != tc.placeProtection ||
			matrix.AutoDispatch != tc.autoDispatch ||
			matrix.ManualReviewRequired != tc.manualReviewRequired ||
			matrix.AllowPlanProgression != tc.allowPlanProgression {
			t.Fatalf("unexpected matrix for %s: %+v", tc.state, matrix)
		}
	}
}

func TestEnsureLiveExecutionPlanBlocksCloseOnlyTakeoverFreshEntry(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	secondary, err := platform.store.CreateLiveSession("live-main", "strategy-ambiguous")
	if err != nil {
		t.Fatalf("create secondary live session failed: %v", err)
	}
	secondaryState := cloneMetadata(secondary.State)
	secondaryState["symbol"] = "BTCUSDT"
	if _, err := platform.store.UpdateLiveSessionState(secondary.ID, secondaryState); err != nil {
		t.Fatalf("update secondary live session state failed: %v", err)
	}
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	session, err = platform.enterRecoveredLiveSessionCloseOnlyMode(session, position, "missing-strategy-version", "test")
	if err != nil {
		t.Fatalf("enter close-only mode failed: %v", err)
	}

	updated, plan, err := platform.ensureLiveExecutionPlan(session)
	if err == nil {
		t.Fatal("expected close-only takeover to block fresh entry plan construction")
	}
	if len(plan) != 0 {
		t.Fatalf("expected no live plan to be returned, got %d steps", len(plan))
	}
	if got := stringValue(updated.State["recoveryTakeoverState"]); got != liveRecoveryModeCloseOnlyTakeover {
		t.Fatalf("expected takeover state %s, got %s", liveRecoveryModeCloseOnlyTakeover, got)
	}
}

func TestRecoveryMonitoringDisablesAutoDispatch(t *testing.T) {
	now := time.Now().UTC()
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":             "auto-dispatch",
			"recoveryTakeoverActive":   true,
			"positionRecoveryStatus":   "protected-open-position",
			"hasRecoveredPosition":     true,
			"hasRecoveredRealPosition": true,
		},
	}
	intent := map[string]any{
		"action": "open",
		"role":   "entry",
		"side":   "BUY",
		"symbol": "BTCUSDT",
		"status": "dispatchable",
	}
	if shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected recovery-monitoring takeover to stay manual")
	}
}

func TestReconcileLiveSessionPlanIndexBlocksRecoveryConflictProgression(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["planIndex"] = 0
	state["recoveryTakeoverActive"] = true
	state["positionRecoveryStatus"] = liveRecoveryTakeoverStateConflict
	state["positionReconcileGateStatus"] = livePositionReconcileGateStatusConflict
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	reconciled, err := platform.reconcileLiveSessionPlanIndex(session, []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}, time.Now().UTC(), "test-recovery-conflict")
	if err != nil {
		t.Fatalf("reconcile live session plan index failed: %v", err)
	}
	gotIndex, ok := toFloat64(reconciled.State["planIndex"])
	if !ok || int(gotIndex) != 0 {
		t.Fatalf("expected recovery conflict to keep planIndex at entry, got %v", reconciled.State["planIndex"])
	}
	if got := stringValue(reconciled.State["recoveryTakeoverState"]); got != liveRecoveryTakeoverStateConflict {
		t.Fatalf("expected takeover state %s, got %s", liveRecoveryTakeoverStateConflict, got)
	}
}

func TestReconcileLiveSessionPlanIndexAllowsRecoveryMonitoringClosePath(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["planIndex"] = 0
	state["recoveryTakeoverActive"] = true
	state["positionRecoveryStatus"] = "protected-open-position"
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	reconciled, err := platform.reconcileLiveSessionPlanIndex(session, []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}, time.Now().UTC(), "test-recovery-monitoring")
	if err != nil {
		t.Fatalf("reconcile live session plan index failed: %v", err)
	}
	gotIndex, ok := toFloat64(reconciled.State["planIndex"])
	if !ok || int(gotIndex) != 1 {
		t.Fatalf("expected recovery-monitoring takeover to advance into close path, got %v", reconciled.State["planIndex"])
	}
	if got := stringValue(reconciled.State["recoveryTakeoverState"]); got != liveRecoveryTakeoverStateMonitoring {
		t.Fatalf("expected takeover state %s, got %s", liveRecoveryTakeoverStateMonitoring, got)
	}
}

func TestReconcileLiveSessionPlanIndexLeavesHealthySessionUnchanged(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	reconciled, err := platform.reconcileLiveSessionPlanIndex(session, []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}, time.Now().UTC(), "test-healthy")
	if err != nil {
		t.Fatalf("reconcile live session plan index failed: %v", err)
	}
	gotIndex, ok := toFloat64(reconciled.State["planIndex"])
	if !ok || int(gotIndex) != 1 {
		t.Fatalf("expected healthy session to keep advancing to exit, got %v", reconciled.State["planIndex"])
	}
	if got := stringValue(reconciled.State["recoveryTakeoverState"]); got != "" {
		t.Fatalf("expected healthy session to avoid takeover state, got %s", got)
	}
}

func TestEnsureLiveExecutionPlanReconcilesCachedPlanIndexBackToEntryWhenPositionFlat(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
		"zero_initial_mode":   "position",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}

	updatedState := cloneMetadata(session.State)
	updatedState["planIndex"] = 1
	session, err = platform.store.UpdateLiveSessionState(session.ID, updatedState)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}
	platform.mu.Unlock()

	reconciled, plan, err := platform.ensureLiveExecutionPlan(session)
	if err != nil {
		t.Fatalf("ensure live execution plan failed: %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("expected cached live plan to be preserved, got %d steps", len(plan))
	}
	gotIndex, ok := toFloat64(reconciled.State["planIndex"])
	if !ok || int(gotIndex) != 0 {
		t.Fatalf("expected cached plan index to roll back to entry when position is flat, got %v", reconciled.State["planIndex"])
	}
	if !boolValue(reconciled.State["planIndexRecoveredFromPosition"]) {
		t.Fatal("expected cached plan reconciliation to be recorded in session state")
	}
}

func TestEnsureLiveExecutionPlanReconcilesCachedPlanIndexToExitWhenPositionExists(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
		"zero_initial_mode":   "position",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}

	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	updatedState := cloneMetadata(session.State)
	updatedState["planIndex"] = 0
	session, err = platform.store.UpdateLiveSessionState(session.ID, updatedState)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}
	platform.mu.Unlock()

	reconciled, _, err := platform.ensureLiveExecutionPlan(session)
	if err != nil {
		t.Fatalf("ensure live execution plan failed: %v", err)
	}
	gotIndex, ok := toFloat64(reconciled.State["planIndex"])
	if !ok || int(gotIndex) != 1 {
		t.Fatalf("expected cached plan index to advance to exit when position exists, got %v", reconciled.State["planIndex"])
	}
	if !boolValue(reconciled.State["hasRecoveredPosition"]) {
		t.Fatal("expected recovered position to be captured during cached plan reconciliation")
	}
}

func TestEnsureLiveExecutionPlanReconcilesExhaustedCachedPlanIndexToCompletionWhenFlat(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
		"zero_initial_mode":   "position",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}

	updatedState := cloneMetadata(session.State)
	updatedState["planIndex"] = 5
	updatedState["planLength"] = 8
	session, err = platform.store.UpdateLiveSessionState(session.ID, updatedState)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}
	platform.mu.Unlock()

	reconciled, plan, err := platform.ensureLiveExecutionPlan(session)
	if err != nil {
		t.Fatalf("ensure live execution plan failed: %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("expected cached live plan to be preserved, got %d steps", len(plan))
	}
	gotIndex, ok := toFloat64(reconciled.State["planIndex"])
	if !ok || int(gotIndex) != len(plan) {
		t.Fatalf("expected exhausted cached plan index to reconcile to completion marker, got %v", reconciled.State["planIndex"])
	}
	if got := maxIntValue(reconciled.State["planLength"], -1); got != len(plan) {
		t.Fatalf("expected cached plan length to refresh to current plan length, got %d", got)
	}
	if !boolValue(reconciled.State["planIndexRecoveredFromPosition"]) {
		t.Fatal("expected exhausted plan reconciliation to be recorded in session state")
	}
}

func TestEvaluateLiveSessionOnSignalRollsOverFlatSessionWhenPlanExhausted(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}

	updatedState := cloneMetadata(session.State)
	updatedState["planIndex"] = 7
	session, err = platform.store.UpdateLiveSessionState(session.ID, updatedState)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}
	platform.mu.Unlock()

	eventTime := time.Date(2026, 4, 18, 2, 0, 0, 0, time.UTC)
	if err := platform.evaluateLiveSessionOnSignal(session, "", map[string]any{
		"symbol": "BTCUSDT",
	}, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	stopped, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if got := stopped.Status; got != "RUNNING" {
		t.Fatalf("expected flat exhausted live session to remain running for rollover, got %s", got)
	}
	if got := stringValue(stopped.State["lastStrategyEvaluationStatus"]); got != "plan-exhausted" {
		t.Fatalf("expected lastStrategyEvaluationStatus=plan-exhausted, got %s", got)
	}
	gotIndex, ok := toFloat64(stopped.State["planIndex"])
	if !ok || int(gotIndex) != 0 {
		t.Fatalf("expected rollover to reset planIndex, got %v", stopped.State["planIndex"])
	}
	gotLength, ok := toFloat64(stopped.State["planLength"])
	if !ok || int(gotLength) != 0 {
		t.Fatalf("expected rollover to clear current planLength, got %v", stopped.State["planLength"])
	}
	if got := stringValue(stopped.State["completedAt"]); got != eventTime.UTC().Format(time.RFC3339) {
		t.Fatalf("expected completedAt to be recorded at exhaustion time, got %s", got)
	}
	if got := stringValue(stopped.State["lastPlanRolloverAt"]); got != eventTime.UTC().Format(time.RFC3339) {
		t.Fatalf("expected lastPlanRolloverAt to be recorded, got %s", got)
	}
	if got := stringValue(stopped.State["lastPlanRolloverReason"]); got != "plan-exhausted" {
		t.Fatalf("expected lastPlanRolloverReason=plan-exhausted, got %s", got)
	}
	platform.mu.Lock()
	_, cached := platform.livePlans[session.ID]
	platform.mu.Unlock()
	if cached {
		t.Fatal("expected cached live execution plan to be cleared when rollover is scheduled")
	}

	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{
		{Role: "entry", Side: "BUY", Symbol: "BTCUSDT"},
		{Role: "exit", Side: "SELL", Symbol: "BTCUSDT"},
	}
	platform.mu.Unlock()

	rolledForward, plan, err := platform.ensureLiveExecutionPlan(stopped)
	if err != nil {
		t.Fatalf("ensure live execution plan after rollover failed: %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("expected rollover to accept the fresh cached plan, got %d steps", len(plan))
	}
	gotIndex, ok = toFloat64(rolledForward.State["planIndex"])
	if !ok || int(gotIndex) != 0 {
		t.Fatalf("expected fresh cycle to start at planIndex 0, got %v", rolledForward.State["planIndex"])
	}
	gotLength, ok = toFloat64(rolledForward.State["planLength"])
	if !ok || int(gotLength) != 2 {
		t.Fatalf("expected fresh cycle to refresh planLength, got %v", rolledForward.State["planLength"])
	}
	if _, ok := rolledForward.State["completedAt"]; ok {
		t.Fatal("expected completedAt to be cleared after fresh plan is available")
	}
}

func TestParseBinanceSymbolRules(t *testing.T) {
	rules := parseBinanceSymbolRules(map[string]any{
		"symbol": "BTCUSDT",
		"filters": []any{
			map[string]any{
				"filterType": "PRICE_FILTER",
				"tickSize":   "0.10",
			},
			map[string]any{
				"filterType": "LOT_SIZE",
				"stepSize":   "0.001",
				"minQty":     "0.001",
				"maxQty":     "1000",
			},
			map[string]any{
				"filterType":  "MIN_NOTIONAL",
				"minNotional": "100",
			},
		},
	})
	if rules.TickSize != 0.1 {
		t.Fatalf("expected tick size 0.1, got %v", rules.TickSize)
	}
	if rules.StepSize != 0.001 {
		t.Fatalf("expected step size 0.001, got %v", rules.StepSize)
	}
	if rules.MinQty != 0.001 {
		t.Fatalf("expected min qty 0.001, got %v", rules.MinQty)
	}
	if rules.MinNotional != 100 {
		t.Fatalf("expected min notional 100, got %v", rules.MinNotional)
	}
}

func TestNormalizeBinancePriceAndQuantity(t *testing.T) {
	rules := binanceSymbolRules{
		TickSize: 0.1,
		StepSize: 0.001,
		MinQty:   0.001,
	}
	if got := normalizeBinancePrice(68643.67, rules); got != 68643.6 {
		t.Fatalf("expected price rounded down to tick size, got %v", got)
	}
	if got := normalizeBinanceQuantity(0.0019, rules); got != 0.001 {
		t.Fatalf("expected quantity rounded down to step size, got %v", got)
	}
	if got := normalizeBinanceQuantity(0.0004, rules); got != 0.001 {
		t.Fatalf("expected min quantity clamp, got %v", got)
	}
}

func TestFormatBinanceDecimalUsesExchangePrecision(t *testing.T) {
	if got := formatBinanceDecimal(68915.4, 0.1); got != "68915.4" {
		t.Fatalf("expected formatted price without float noise, got %s", got)
	}
	if got := formatBinanceDecimal(0.0015, 0.0001); got != "0.0015" {
		t.Fatalf("expected formatted quantity without float noise, got %s", got)
	}
}

func TestRequiredBinanceQuantityForMinNotional(t *testing.T) {
	rules := binanceSymbolRules{
		StepSize:    0.001,
		MinQty:      0.001,
		MinNotional: 100,
	}
	if got := requiredBinanceQuantityForMinNotional(0.001, 68643.6, rules); got != 0.002 {
		t.Fatalf("expected required quantity 0.002 for min notional, got %v", got)
	}
	if got := requiredBinanceQuantityForMinNotional(0.002, 68643.6, rules); got != 0.002 {
		t.Fatalf("expected existing quantity to remain unchanged, got %v", got)
	}
}

func TestNormalizeRESTOrderRecordsNormalizationTelemetry(t *testing.T) {
	adapter := binanceFuturesLiveAdapter{}
	creds := binanceRESTCredentials{BaseURL: "https://example.test"}
	cacheKey := creds.BaseURL + "|BTCUSDT"
	binanceSymbolRulesCacheMu.Lock()
	previous, existed := binanceSymbolRulesCache[cacheKey]
	binanceSymbolRulesCacheMu.Unlock()
	t.Cleanup(func() {
		binanceSymbolRulesCacheMu.Lock()
		defer binanceSymbolRulesCacheMu.Unlock()
		if existed {
			binanceSymbolRulesCache[cacheKey] = previous
		} else {
			delete(binanceSymbolRulesCache, cacheKey)
		}
	})
	binanceSymbolRulesCacheMu.Lock()
	binanceSymbolRulesCache[cacheKey] = binanceSymbolRules{
		Symbol:      "BTCUSDT",
		TickSize:    0.1,
		StepSize:    0.001,
		MinQty:      0.001,
		MaxQty:      1000,
		MinNotional: 100,
		UpdatedAt:   time.Now().UTC(),
	}
	binanceSymbolRulesCacheMu.Unlock()

	normalized, _, err := adapter.normalizeRESTOrder(domain.Order{
		Symbol:   "BTCUSDT",
		Type:     "LIMIT",
		Quantity: 0.0021,
		Price:    68643.67,
	}, creds)
	if err != nil {
		t.Fatalf("normalize REST order failed: %v", err)
	}
	norm := mapValue(normalized.Metadata["normalization"])
	if got := parseFloatValue(norm["rawQuantity"]); got != 0.0021 {
		t.Fatalf("expected raw quantity 0.0021, got %v", got)
	}
	if got := parseFloatValue(norm["normalizedQuantity"]); got != 0.002 {
		t.Fatalf("expected normalized quantity 0.002, got %v", got)
	}
	if got := parseFloatValue(norm["normalizedPrice"]); got != 68643.6 {
		t.Fatalf("expected normalized price 68643.6, got %v", got)
	}
	if got := normalized.Quantity; got != 0.002 {
		t.Fatalf("expected normalized order quantity 0.002, got %v", got)
	}
	if got := normalized.Price; got != 68643.6 {
		t.Fatalf("expected normalized order price 68643.6, got %v", got)
	}
	quantityAdjustmentCount := normalizationItemCount(norm["quantityAdjustments"])
	if quantityAdjustmentCount != 1 {
		t.Fatalf("expected 1 quantity adjustment, got %v", norm["quantityAdjustments"])
	}
	if !boolValue(norm["stepSizeAdjusted"]) || boolValue(norm["minNotionalAdjusted"]) {
		t.Fatalf("expected only step size adjustment, got %+v", norm)
	}
	if !boolValue(norm["tickSizeAdjusted"]) {
		t.Fatalf("expected tick size adjustment, got %+v", norm)
	}
}

func TestExecutionDispatchSummaryIncludesNormalizationTelemetry(t *testing.T) {
	summary := executionDispatchSummary(map[string]any{
		"type":       "LIMIT",
		"quantity":   0.0021,
		"limitPrice": 68643.6,
		"priceHint":  68643.67,
		"metadata": map[string]any{
			"executionDecision": "direct-dispatch",
		},
	}, domain.Order{
		Status:   "NEW",
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: 0.002,
		Price:    68643.6,
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"rawQuantity":        0.0021,
				"rawPriceReference":  68643.67,
				"normalizedQuantity": 0.002,
				"normalizedPrice":    68643.6,
				"normalization": map[string]any{
					"quantityAdjustments": []any{"step_size"},
					"priceAdjustments":    []any{"tick_size"},
				},
				"symbolRules": map[string]any{
					"stepSize":    0.001,
					"tickSize":    0.1,
					"minNotional": 100.0,
				},
			},
		},
	}, false)
	if got := parseFloatValue(summary["rawQuantity"]); got != 0.0021 {
		t.Fatalf("expected raw quantity in dispatch summary, got %v", got)
	}
	if got := parseFloatValue(summary["normalizedQuantity"]); got != 0.002 {
		t.Fatalf("expected normalized quantity in dispatch summary, got %v", got)
	}
	if got := parseFloatValue(summary["normalizedPrice"]); got != 68643.6 {
		t.Fatalf("expected normalized price in dispatch summary, got %v", got)
	}
	if got := parseFloatValue(summary["rawPriceReference"]); got != 68643.67 {
		t.Fatalf("expected raw price reference in dispatch summary, got %v", got)
	}
	if normalizationItemCount(mapValue(summary["normalization"])["quantityAdjustments"]) != 1 {
		t.Fatalf("expected quantity adjustment details in dispatch summary, got %+v", summary["normalization"])
	}
	if normalizationItemCount(mapValue(summary["normalization"])["priceAdjustments"]) != 1 {
		t.Fatalf("expected price adjustment details in dispatch summary, got %+v", summary["normalization"])
	}
}

func TestExecutionDispatchSummaryFallsBackToNestedNormalizedPrice(t *testing.T) {
	summary := executionDispatchSummary(map[string]any{
		"type":       "LIMIT",
		"quantity":   0.0019,
		"limitPrice": 68643.6,
		"priceHint":  68643.67,
	}, domain.Order{
		Status:   "NEW",
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Quantity: 0.002,
		Price:    68643.6,
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"normalization": map[string]any{
					"normalizedPrice":    68643.6,
					"normalizedQuantity": 0.002,
					"rawPriceReference":  68643.67,
					"rawQuantity":        0.0019,
				},
			},
		},
	}, false)
	if got := parseFloatValue(summary["normalizedPrice"]); got != 68643.6 {
		t.Fatalf("expected normalized price fallback from normalization payload, got %v", got)
	}
}

func TestExecutionTimeoutTimelineMetadataUsesOriginalSubmissionNormalization(t *testing.T) {
	order := domain.Order{
		ID: "order-1",
		Metadata: map[string]any{
			"executionExpiresAt": "2026-04-10T01:00:00Z",
			"executionProposal": map[string]any{
				"type":       "LIMIT",
				"quantity":   0.0019,
				"limitPrice": 68643.6,
				"priceHint":  68643.67,
			},
			"adapterSubmission": map[string]any{
				"normalizedPrice": 68643.6,
				"normalization": map[string]any{
					"normalizedPrice":    68643.6,
					"normalizedQuantity": 0.002,
					"rawPriceReference":  68643.67,
					"rawQuantity":        0.0019,
				},
			},
		},
	}
	cancelledOrder := domain.Order{
		ID:     "order-1",
		Status: "CANCELLED",
	}
	metadata := executionTimeoutTimelineMetadata(order, withExecutionSubmissionFallback(cancelledOrder, order))
	if got := parseFloatValue(metadata["normalizedPrice"]); got != 68643.6 {
		t.Fatalf("expected timeout metadata to preserve normalized price, got %v", got)
	}
}

func TestWithExecutionSubmissionFallbackMergesPartialSubmissionFields(t *testing.T) {
	order := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"normalizedPrice": 68643.6,
				"normalization": map[string]any{
					"normalizedPrice": 68643.6,
				},
			},
		},
	}
	fallback := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"rawQuantity":        0.0019,
				"normalizedQuantity": 0.002,
				"rawPriceReference":  68643.67,
				"normalization": map[string]any{
					"normalizedPrice":    68643.6,
					"normalizedQuantity": 0.002,
					"rawPriceReference":  68643.67,
					"rawQuantity":        0.0019,
				},
				"symbolRules": map[string]any{
					"stepSize": 0.001,
				},
			},
		},
	}
	merged := withExecutionSubmissionFallback(order, fallback)
	submission := mapValue(merged.Metadata["adapterSubmission"])
	if got := parseFloatValue(submission["normalizedPrice"]); got != 68643.6 {
		t.Fatalf("expected existing normalized price to survive merge, got %v", got)
	}
	if got := parseFloatValue(submission["normalizedQuantity"]); got != 0.002 {
		t.Fatalf("expected normalized quantity fallback, got %v", got)
	}
	if got := parseFloatValue(mapValue(submission["normalization"])["rawQuantity"]); got != 0.0019 {
		t.Fatalf("expected nested raw quantity fallback, got %v", got)
	}
	if got := parseFloatValue(mapValue(submission["symbolRules"])["stepSize"]); got != 0.001 {
		t.Fatalf("expected symbol rules fallback, got %v", got)
	}
}

func TestWithExecutionSubmissionFallbackRestoresZeroNormalizationFields(t *testing.T) {
	order := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"normalizedPrice": 0.0,
				"rawQuantity":     0.0,
				"reduceOnly":      false,
			},
		},
	}
	fallback := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"normalizedPrice": 68643.6,
				"rawQuantity":     0.0019,
				"reduceOnly":      true,
			},
		},
	}
	merged := withExecutionSubmissionFallback(order, fallback)
	submission := mapValue(merged.Metadata["adapterSubmission"])
	if got := parseFloatValue(submission["normalizedPrice"]); got != 68643.6 {
		t.Fatalf("expected zero normalized price to fall back, got %v", got)
	}
	if got := parseFloatValue(submission["rawQuantity"]); got != 0.0019 {
		t.Fatalf("expected zero raw quantity to fall back, got %v", got)
	}
	if got := boolValue(submission["reduceOnly"]); !got {
		t.Fatal("expected known execution-control false to fall back to the original reduceOnly=true")
	}
}

func TestWithExecutionSubmissionFallbackUsesFallbackForMissingExecutionControlFlags(t *testing.T) {
	order := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"normalizedPrice": 68643.6,
			},
		},
	}
	fallback := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"postOnly":      true,
				"reduceOnly":    true,
				"closePosition": true,
			},
		},
	}
	merged := withExecutionSubmissionFallback(order, fallback)
	submission := mapValue(merged.Metadata["adapterSubmission"])
	if !boolValue(submission["postOnly"]) || !boolValue(submission["reduceOnly"]) || !boolValue(submission["closePosition"]) {
		t.Fatalf("expected missing execution-control flags to fall back, got %+v", submission)
	}
}

func TestApplyLiveVirtualInitialEventUsesFallbackVirtualPositionIDWhenIntentSignatureEmpty(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	session.State = cloneMetadata(session.State)
	proposalMap := map[string]any{
		"side":   "BUY",
		"symbol": "BTCUSDT",
		"reason": "Initial",
	}
	updated, err := platform.applyLiveVirtualInitialEvent(session, proposalMap, time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("apply virtual initial event failed: %v", err)
	}
	virtualPosition := mapValue(updated.State["virtualPosition"])
	if virtualPosition == nil {
		t.Fatal("expected virtual position to be recorded")
	}
	rawSignature := buildLiveIntentSignature(proposalMap)
	fallbackSignature := buildFallbackLiveIntentSignature(proposalMap, executionProposalFromMap(proposalMap))
	if got := stringValue(updated.State["lastDispatchedIntentSignature"]); got != fallbackSignature {
		t.Fatalf("expected sparse proposal to use fallback signature %q, got %q (raw=%q)", fallbackSignature, got, rawSignature)
	}
	if got := stringValue(virtualPosition["id"]); got == "" || strings.HasSuffix(got, rawSignature) {
		t.Fatalf("expected fallback virtual position id to avoid sparse raw signature, got %q", got)
	}
}

func TestBuildFallbackLiveIntentSignatureIncludesExecutionFields(t *testing.T) {
	baseProposalMap := map[string]any{
		"reason":            "Initial",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"type":              "LIMIT",
		"signalBarStateKey": "bar-1",
		"quantity":          0.001,
		"limitPrice":        68000.0,
		"priceHint":         68000.5,
	}
	baseSignature := buildFallbackLiveIntentSignature(baseProposalMap, executionProposalFromMap(baseProposalMap))
	variantProposalMap := cloneMetadata(baseProposalMap)
	variantProposalMap["quantity"] = 0.002
	variantSignature := buildFallbackLiveIntentSignature(variantProposalMap, executionProposalFromMap(variantProposalMap))
	if baseSignature == variantSignature {
		t.Fatalf("expected quantity changes to alter fallback signature, got %q", baseSignature)
	}
}

func TestBuildFallbackLiveIntentSignaturePreservesExplicitFalseBooleans(t *testing.T) {
	proposalMap := map[string]any{
		"reason":            "Initial",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"postOnly":          false,
		"reduceOnly":        false,
		"closePosition":     false,
		"signalBarStateKey": "bar-1",
	}
	proposal := ExecutionProposal{
		PostOnly:   true,
		ReduceOnly: true,
	}
	signature := buildFallbackLiveIntentSignature(proposalMap, proposal)
	if strings.Contains(signature, "|true|true|true") {
		t.Fatalf("expected explicit false booleans to be preserved in fallback signature, got %q", signature)
	}
}

func TestWithExecutionSubmissionFallbackPreservesExplicitZeroForUnknownFields(t *testing.T) {
	order := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"queueIndex": 0.0,
				"auditFlag":  false,
			},
		},
	}
	fallback := domain.Order{
		Metadata: map[string]any{
			"adapterSubmission": map[string]any{
				"queueIndex": 7.0,
				"auditFlag":  true,
			},
		},
	}
	merged := withExecutionSubmissionFallback(order, fallback)
	submission := mapValue(merged.Metadata["adapterSubmission"])
	if got := parseFloatValue(submission["queueIndex"]); got != 0 {
		t.Fatalf("expected explicit zero queueIndex to be preserved, got %v", got)
	}
	if got := boolValue(submission["auditFlag"]); got {
		t.Fatal("expected explicit false auditFlag to be preserved")
	}
}

func normalizationItemCount(raw any) int {
	switch value := raw.(type) {
	case []string:
		return len(value)
	case []any:
		return len(value)
	default:
		return 0
	}
}

func TestShouldMarkLiveExecutionFallback(t *testing.T) {
	order := domain.Order{
		Status: "REJECTED",
		Metadata: map[string]any{
			"liveSubmitError": "binance request failed: 400 Bad Request {\"code\":-5022,\"msg\":\"Due to the order could not be executed as maker\"}",
		},
	}
	if !shouldMarkLiveExecutionFallback(order) {
		t.Fatal("expected maker-rejected post-only order to mark fallback eligibility")
	}
}

func TestEvaluateLiveSessionOnSignalRecordsVirtualInitialForZeroInitialStrategy(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
		"zero_initial_mode":   "position",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime,
		Price:     69000.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "Initial",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              68110.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["health"] = "healthy"
		state["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastEventSummary"] = cloneMetadata(summary)
		state["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 68110.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		state["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"close": 68100.0,
					"high":  69010.0,
					"low":   67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastDispatchedOrderStatus"]); got != liveOrderStatusVirtualInitial {
		t.Fatalf("expected virtual initial dispatch marker, got %s", got)
	}
	if got := stringValue(updated.State["lastVirtualSignalType"]); got != "initial" {
		t.Fatalf("expected lastVirtualSignalType=initial, got %s", got)
	}
	if !boolValue(mapValue(updated.State["virtualPosition"])["virtual"]) {
		t.Fatal("expected virtualPosition to be recorded in live session state")
	}
	if got := stringValue(mapValue(updated.State["virtualPosition"])["id"]); got == "" {
		t.Fatal("expected virtualPosition to carry a stable id")
	}
	if got := maxIntValue(updated.State["planIndex"], -1); got != 1 {
		t.Fatalf("expected planIndex to advance after virtual initial, got %d", got)
	}
}

func TestEvaluateLiveSessionOnSignalUsesZeroInitialReentryWindowInsteadOfVirtualInitial(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
		"zero_initial_mode":   "reentry_window",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime.Add(-48 * time.Hour),
		Price:     69000.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "Initial",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              67845.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["health"] = "healthy"
		state["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastEventSummary"] = cloneMetadata(summary)
		state["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 67845.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		state["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"barStart": eventTime.Truncate(24 * time.Hour).Format(time.RFC3339),
					"close":    68100.0,
					"high":     69010.0,
					"low":      67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastDispatchedOrderStatus"]); got == liveOrderStatusVirtualInitial {
		t.Fatalf("expected zero initial window mode to avoid virtual initial dispatch marker, got %s", got)
	}
	if virtualPosition := mapValue(updated.State["virtualPosition"]); len(virtualPosition) != 0 {
		t.Fatalf("expected zero initial window mode to avoid virtual positions, got %+v", virtualPosition)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	if got := stringValue(proposal["status"]); got != "dispatchable" {
		t.Fatalf("expected dispatchable reentry proposal, got %s", got)
	}
	if got := stringValue(proposal["reason"]); got != "Zero-Initial-Reentry" {
		t.Fatalf("expected Zero-Initial-Reentry proposal reason, got %s", got)
	}
	if pending := mapValue(updated.State[livePendingZeroInitialWindowStateKey]); stringValue(pending["side"]) != "BUY" {
		t.Fatalf("expected pending BUY zero initial window in session state, got %+v", pending)
	}
}

func TestEvaluateLiveSessionOnSignalRecordsVirtualExitForStaleExitStepWithVirtualPosition(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["virtualPosition"] = map[string]any{
		"id":         "virtual|session-1|signal-1",
		"virtual":    true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 69000.0,
		"stopLoss":   69050.0,
		"quantity":   0.0,
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	staleExitEvent := eventTime.Add(-48 * time.Hour)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: staleExitEvent,
		Price:     68950.0,
		Side:      "SELL",
		Role:      "exit",
		Reason:    "SL",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              68900.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		runtimeState := cloneMetadata(runtimeSession.State)
		runtimeState["health"] = "healthy"
		runtimeState["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		runtimeState["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		runtimeState["lastEventSummary"] = cloneMetadata(summary)
		runtimeState["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 68900.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		runtimeState["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"close": 68100.0,
					"high":  69010.0,
					"low":   67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = runtimeState
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastDispatchedOrderStatus"]); got != liveOrderStatusVirtualExit {
		t.Fatalf("expected virtual exit dispatch marker, got %s", got)
	}
	if got := stringValue(updated.State["lastVirtualSignalType"]); got != "exit" {
		t.Fatalf("expected lastVirtualSignalType=exit, got %s", got)
	}
	if virtualPosition := mapValue(updated.State["virtualPosition"]); len(virtualPosition) != 0 {
		t.Fatalf("expected virtualPosition to be cleared after virtual exit, got %+v", virtualPosition)
	}
	if got := maxIntValue(updated.State["planIndex"], -1); got != 1 {
		t.Fatalf("expected planIndex to advance after virtual exit, got %d", got)
	}
}

func TestEvaluateLiveSessionOnSignalKeepsReentryDispatchableWithoutInitialBreakout(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime,
		Price:     68900.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "SL-Reentry",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              69010.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["health"] = "healthy"
		state["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastEventSummary"] = cloneMetadata(summary)
		state["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 69010.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		state["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"close": 68100.0,
					"high":  68900.0,
					"low":   67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	if proposal == nil {
		t.Fatal("expected lastExecutionProposal to be recorded")
	}
	if got := stringValue(proposal["status"]); got != "dispatchable" {
		t.Fatalf("expected reentry proposal to remain dispatchable, got %s", got)
	}
	if got := stringValue(proposal["reason"]); got != "SL-Reentry" {
		t.Fatalf("expected SL-Reentry proposal, got %s", got)
	}
	if boolValue(mapValue(proposal["metadata"])["virtualPosition"]) {
		t.Fatal("expected reentry proposal to stay non-virtual")
	}
}

func TestEvaluateLiveSessionOnSignalUsesInjectedATRForVolatilitySizing(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{"availableBalance": 10000.0}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
		"positionSizingMode":  "volatility_adjusted",
		"targetRiskBps":       100.0,
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime,
		Price:     68900.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "SL-Reentry",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              69010.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["health"] = "healthy"
		state["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastEventSummary"] = cloneMetadata(summary)
		state["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 69010.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		state["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"close": 68100.0,
					"high":  69010.0,
					"low":   67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	if proposal == nil {
		t.Fatal("expected lastExecutionProposal to be recorded")
	}
	metadata := mapValue(proposal["metadata"])
	if got := parseFloatValue(metadata["sizingATR14"]); got != 900.0 {
		t.Fatalf("expected sizing ATR to be injected in same cycle, got %v", got)
	}
	if got := parseFloatValue(metadata["sizingComputedQuantity"]); got <= 0 {
		t.Fatalf("expected positive volatility-adjusted quantity, got %v", got)
	}
}

func TestEvaluateLiveSignalDecisionDoesNotMutateOriginalSessionState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         70000,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	eventTime := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"sma5":      69900.0,
			"ma20":      68600.0,
			"atr14":     900.0,
			"current": map[string]any{
				"close": 70000.0,
				"high":  70100.0,
				"low":   69500.0,
			},
			"prevBar1": map[string]any{
				"high": 69800.0,
				"low":  68800.0,
			},
			"prevBar2": map[string]any{
				"high": 69700.0,
				"low":  68700.0,
			},
		},
	}
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              70000.0,
		"event":              "trade_tick",
	}
	sourceStates := map[string]any{
		signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT"): map[string]any{
			"sourceKey":   "binance-trade-tick",
			"role":        "trigger",
			"symbol":      "BTCUSDT",
			"streamType":  "trade_tick",
			"lastEventAt": eventTime.Format(time.RFC3339),
			"summary": map[string]any{
				"price": 70000.0,
			},
		},
	}

	_, decision, _, err := platform.evaluateLiveSignalDecision(
		session,
		summary,
		sourceStates,
		signalStates,
		eventTime,
		eventTime,
		68800.0,
		"SELL",
		"exit",
		"PT",
	)
	if err != nil {
		t.Fatalf("evaluate live signal decision failed: %v", err)
	}
	if len(session.State) == 0 {
		t.Fatal("expected session state to remain initialized")
	}
	if _, ok := session.State["hwm"]; ok {
		t.Fatal("expected original session state to remain unmutated by signal evaluation")
	}
	if _, ok := session.State["watermarkPositionKey"]; ok {
		t.Fatal("expected watermark key to stay out of original session state")
	}
	livePositionState := mapValue(decision.Metadata["livePositionState"])
	if parseFloatValue(livePositionState["stopLoss"]) <= 0 {
		t.Fatal("expected evaluated live position state to still be populated")
	}
}

func testLiveRecoverySignalBarStates(symbol string, closePrice float64) map[string]any {
	return map[string]any{
		signalBindingMatchKey("binance-kline", "signal", symbol): map[string]any{
			"symbol":    symbol,
			"timeframe": "1d",
			"ma20":      68000.0,
			"atr14":     900.0,
			"current": map[string]any{
				"close": closePrice,
				"high":  closePrice + 100.0,
				"low":   closePrice - 100.0,
			},
			"prevBar1": map[string]any{
				"high": 69800.0,
				"low":  68800.0,
			},
			"prevBar2": map[string]any{
				"high": 69700.0,
				"low":  68700.0,
			},
		},
	}
}

func TestRefreshLiveSessionPositionContextRebuildsLivePositionState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"openOrders": []map[string]any{
			{
				"symbol":        "BTCUSDT",
				"origType":      "TAKE_PROFIT_MARKET",
				"reduceOnly":    true,
				"closePosition": true,
			},
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         70000,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"ma20":      68000.0,
			"atr14":     900.0,
			"current": map[string]any{
				"close": 70000.0,
				"high":  70100.0,
				"low":   69500.0,
			},
			"prevBar1": map[string]any{
				"high": 69800.0,
				"low":  68800.0,
			},
			"prevBar2": map[string]any{
				"high": 69700.0,
				"low":  68700.0,
			},
		},
	}
	state[livePendingZeroInitialWindowStateKey] = map[string]any{
		"side":            "BUY",
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
		"armedAt":         time.Date(2026, 4, 17, 1, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"signalBarStart":  time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"expiresAt":       time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Now().UTC(), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	liveState := mapValue(updated.State["livePositionState"])
	if liveState == nil {
		t.Fatal("expected livePositionState to be rebuilt")
	}
	if !boolValue(liveState["protected"]) {
		t.Fatal("expected live position to be marked protected after crossing profit threshold")
	}
	if parseFloatValue(liveState["stopLoss"]) <= 0 {
		t.Fatal("expected stopLoss to be rebuilt")
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "protected-open-position" {
		t.Fatalf("expected protected-open-position, got %s", got)
	}
	if pending := mapValue(updated.State[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected real position refresh to consume pending zero initial window, got %+v", pending)
	}
	var consumed map[string]any
	for _, item := range metadataList(updated.State["timeline"]) {
		if stringValue(item["title"]) == "zero-initial-window-consumed" {
			consumed = item
			break
		}
	}
	if consumed == nil {
		t.Fatalf("expected zero initial window consumed timeline event, got %+v", metadataList(updated.State["timeline"]))
	}
	if got := stringValue(mapValue(consumed["metadata"])["reason"]); got != "real-position-confirmed" {
		t.Fatalf("expected real-position-confirmed consume reason, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextGeneratesLongWatchdogFallbackOnStopLossBreach(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	proposal := executionProposalFromMap(mapValue(updated.State["lastExecutionProposal"]))
	if proposal.Reason != "sl-breached-fallback" {
		t.Fatalf("expected sl-breached-fallback reason, got %s", proposal.Reason)
	}
	if proposal.Side != "SELL" {
		t.Fatalf("expected SELL exit side, got %s", proposal.Side)
	}
	if proposal.Type != "MARKET" || !proposal.ReduceOnly {
		t.Fatalf("expected reduce-only MARKET fallback proposal, got type=%s reduceOnly=%t", proposal.Type, proposal.ReduceOnly)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != livePositionRecoveryStatusClosingPending {
		t.Fatalf("expected %s recovery status, got %s", livePositionRecoveryStatusClosingPending, got)
	}
}

func TestRefreshLiveSessionPositionContextGeneratesShortWatchdogFallbackOnStopLossBreach(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SHORT",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 69100.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 5, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	proposal := executionProposalFromMap(mapValue(updated.State["lastExecutionProposal"]))
	if proposal.Reason != "sl-breached-fallback" {
		t.Fatalf("expected sl-breached-fallback reason, got %s", proposal.Reason)
	}
	if proposal.Side != "BUY" {
		t.Fatalf("expected BUY exit side, got %s", proposal.Side)
	}
	if proposal.Type != "MARKET" || !proposal.ReduceOnly {
		t.Fatalf("expected reduce-only MARKET fallback proposal, got type=%s reduceOnly=%t", proposal.Type, proposal.ReduceOnly)
	}
}

func TestRefreshLiveSessionPositionContextBuildsCompleteMetadataForRecoveredWatchdogClose(t *testing.T) {
	platform, session, runtimeSessionID, _, _ := prepareLiveDecisionTelemetryFixture(t)
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err := platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	metadata := mapValue(proposal["metadata"])
	if got := stringValue(metadata["strategyVersionId"]); got == "" {
		t.Fatal("expected recovered watchdog proposal to carry strategyVersionId")
	}
	if got := stringValue(metadata["runtimeSessionId"]); got != runtimeSessionID {
		t.Fatalf("expected runtimeSessionId %s, got %s", runtimeSessionID, got)
	}
	if !boolValue(metadata["recoveryTriggered"]) {
		t.Fatal("expected recovered watchdog proposal to be marked recoveryTriggered")
	}
	if got := stringValue(metadata["positionRecoveryStatus"]); got != livePositionRecoveryStatusClosingPending {
		t.Fatalf("expected positionRecoveryStatus %s, got %s", livePositionRecoveryStatusClosingPending, got)
	}
	executionContext := mapValue(metadata["executionContext"])
	if got := stringValue(executionContext["strategyVersionId"]); got == "" {
		t.Fatal("expected recovered watchdog proposal execution context to include strategyVersionId")
	}
	if got := stringValue(executionContext["signalTimeframe"]); got != "1d" {
		t.Fatalf("expected signalTimeframe 1d, got %s", got)
	}
	if got := stringValue(executionContext["executionDataSource"]); got != "tick" {
		t.Fatalf("expected executionDataSource tick, got %s", got)
	}
	if got := stringValue(executionContext["symbol"]); got != "BTCUSDT" {
		t.Fatalf("expected execution context symbol BTCUSDT, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextDoesNotGenerateWatchdogFallbackWithoutBreach(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68980,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68980.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 10, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	if proposal := mapValue(updated.State["lastExecutionProposal"]); len(proposal) != 0 {
		t.Fatalf("expected non-breach refresh to stay proposal-free, got %+v", proposal)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "unprotected-open-position" {
		t.Fatalf("expected unprotected-open-position to remain, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextKeepsExistingWatchdogFallbackIntent(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	existingProposal := executionProposalToMap(ExecutionProposal{
		Action:            "risk-exit-fallback",
		Role:              "exit",
		Reason:            "sl-breached-fallback",
		Side:              "SELL",
		Symbol:            "BTCUSDT",
		Type:              "MARKET",
		Quantity:          0.002,
		PriceHint:         68888.0,
		PriceSource:       "fallback-watchdog",
		TimeInForce:       "GTC",
		ReduceOnly:        true,
		SignalKind:        "recovery-watchdog",
		DecisionState:     "unprotected",
		ExecutionStrategy: "book-aware-v1",
		Status:            "dispatchable",
		Metadata: map[string]any{
			"existingMarker": "keep-me",
		},
	})
	state["lastExecutionProposal"] = existingProposal
	state["lastStrategyIntent"] = cloneMetadata(existingProposal)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 15, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	if got := stringValue(mapValue(proposal["metadata"])["existingMarker"]); got != "keep-me" {
		t.Fatalf("expected existing watchdog intent to stay in place, got marker %s", got)
	}
	if got := parseFloatValue(proposal["priceHint"]); got != 68888.0 {
		t.Fatalf("expected existing watchdog intent priceHint to remain untouched, got %v", got)
	}
	if got := stringValue(updated.State["watchdogExitStatus"]); got != "intent-ready" {
		t.Fatalf("expected intent-ready watchdog status, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextDoesNotCrossProtectedBoundary(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"openOrders": []map[string]any{
			{
				"symbol":        "BTCUSDT",
				"origType":      "STOP_MARKET",
				"reduceOnly":    true,
				"closePosition": true,
				"status":        "NEW",
			},
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 20, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	if proposal := mapValue(updated.State["lastExecutionProposal"]); len(proposal) != 0 {
		t.Fatalf("expected protected-open-position to suppress watchdog fallback, got %+v", proposal)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "protected-open-position" {
		t.Fatalf("expected protected-open-position, got %s", got)
	}
}

func TestDispatchLiveSessionIntentAllowsRecoveredPassiveCloseWithCompleteMetadata(t *testing.T) {
	platform, session, runtimeSessionID, _, _ := prepareLiveDecisionTelemetryFixture(t)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-recovered-passive-close"})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-recovered-passive-close",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}
	freshRuntimeAt := time.Now().UTC()
	if err := platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		state := cloneMetadata(runtimeSession.State)
		state["lastEventAt"] = freshRuntimeAt.Format(time.RFC3339)
		state["lastHeartbeatAt"] = freshRuntimeAt.Format(time.RFC3339)
		sourceStates := cloneMetadata(mapValue(state["sourceStates"]))
		for key, item := range sourceStates {
			sourceState := cloneMetadata(mapValue(item))
			sourceState["lastEventAt"] = freshRuntimeAt.Format(time.RFC3339)
			sourceStates[key] = sourceState
		}
		runtimeSession.State = state
		runtimeSession.State["sourceStates"] = sourceStates
		runtimeSession.UpdatedAt = freshRuntimeAt
	}); err != nil {
		t.Fatalf("refresh runtime state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 2, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	order, err := platform.dispatchLiveSessionIntent(updated)
	if err != nil {
		t.Fatalf("expected recovered passive close dispatch to succeed, got %v", err)
	}
	if got := order.StrategyVersionID; got == "" {
		t.Fatal("expected dispatched recovery close order to keep strategyVersionId")
	}
	if got := stringValue(order.Metadata["runtimeSessionId"]); got != runtimeSessionID {
		t.Fatalf("expected order runtimeSessionId %s, got %s", runtimeSessionID, got)
	}
	executionContext := mapValue(order.Metadata["executionContext"])
	if got := stringValue(executionContext["executionDataSource"]); got != "tick" {
		t.Fatalf("expected order executionDataSource tick, got %s", got)
	}
	if got := stringValue(order.Metadata["executionMode"]); got != "live" {
		t.Fatalf("expected order executionMode live, got %s", got)
	}
}

func TestRecoverRunningLiveSessionAllowsVerifiedTakeoverPassiveCloseDispatch(t *testing.T) {
	platform, session, runtimeSessionID, _, _ := prepareLiveDecisionTelemetryFixture(t)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-recovery-passive-close-rest",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
			exchangePositions := []map[string]any{{
				"symbol":      "BTCUSDT",
				"positionAmt": 0.002,
				"entryPrice":  69000.0,
				"markPrice":   68900.0,
			}}
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "binance-rest-account-v3",
				"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
				"syncedAt":        time.Now().UTC().Format(time.RFC3339),
				"bindingMode":     stringValue(binding["connectionMode"]),
				"executionMode":   "rest",
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
				"positions":       exchangePositions,
				"openOrders":      []map[string]any{},
			}
			var err error
			account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
			if err != nil {
				return domain.Account{}, err
			}
			reconcileGate, err := p.reconcileLiveAccountPositions(account, exchangePositions)
			if err != nil {
				return domain.Account{}, err
			}
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["livePositionReconcileGate"] = reconcileGate
			account.Metadata["lastLivePositionSyncAt"] = time.Now().UTC().Format(time.RFC3339)
			clearLiveAccountPositionReconcileRequirement(account.Metadata)
			return p.store.UpdateAccount(account)
		},
		submitOrderFunc: func(_ domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
			return LiveOrderSubmission{
				Status:          "ACCEPTED",
				ExchangeOrderID: "recovery-exit-order-1",
				AcceptedAt:      time.Now().UTC().Format(time.RFC3339),
				Metadata: map[string]any{
					"adapterMode":   "test-recovery-passive-close",
					"executionMode": stringValue(binding["executionMode"]),
					"positionSide":  stringValue(order.Metadata["positionSide"]),
				},
			}, nil
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-recovery-passive-close-rest",
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}
	freshRuntimeAt := time.Now().UTC()
	if err := platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		state := cloneMetadata(runtimeSession.State)
		state["lastEventAt"] = freshRuntimeAt.Format(time.RFC3339)
		state["lastHeartbeatAt"] = freshRuntimeAt.Format(time.RFC3339)
		sourceStates := cloneMetadata(mapValue(state["sourceStates"]))
		for key, item := range sourceStates {
			sourceState := cloneMetadata(mapValue(item))
			sourceState["lastEventAt"] = freshRuntimeAt.Format(time.RFC3339)
			sourceStates[key] = sourceState
		}
		runtimeSession.State = state
		runtimeSession.State["sourceStates"] = sourceStates
		runtimeSession.UpdatedAt = freshRuntimeAt
	}); err != nil {
		t.Fatalf("refresh runtime state failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "RUNNING" {
		t.Fatalf("expected verified takeover session to stay RUNNING, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusVerified {
		t.Fatalf("expected verified reconcile gate status, got %s", got)
	}
	proposal := mapValue(recovered.State["lastExecutionProposal"])
	if !isLiveWatchdogFallbackProposal(proposal) {
		t.Fatalf("expected recovered watchdog passive-close proposal, got %+v", proposal)
	}

	order, err := platform.dispatchLiveSessionIntent(recovered)
	if err != nil {
		t.Fatalf("dispatch recovered passive close failed: %v", err)
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected accepted recovery close order, got %s", got)
	}
	if got := stringValue(order.Metadata["runtimeSessionId"]); got != runtimeSessionID {
		t.Fatalf("expected order runtimeSessionId %s, got %s", runtimeSessionID, got)
	}
	if got := stringValue(order.Metadata["exchangeOrderId"]); got != "recovery-exit-order-1" {
		t.Fatalf("expected exchange order id recovery-exit-order-1, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextDoesNotRetriggerWatchdogWhileExitOrderWorking(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	firstRefreshAt := time.Date(2026, 4, 17, 3, 0, 0, 0, time.UTC)
	updated, err := platform.refreshLiveSessionPositionContext(session, firstRefreshAt, "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	if !isLiveWatchdogFallbackProposal(proposal) {
		t.Fatalf("expected watchdog fallback proposal, got %+v", proposal)
	}
	if got := stringValue(updated.State["watchdogExitStatus"]); got != "intent-ready" {
		t.Fatalf("expected watchdog intent-ready status, got %s", got)
	}

	followupState := cloneMetadata(updated.State)
	delete(followupState, "lastExecutionProposal")
	delete(followupState, "lastStrategyIntent")
	followupState["lastDispatchedIntent"] = cloneMetadata(proposal)
	followupState["lastDispatchedAt"] = firstRefreshAt.Add(time.Second).Format(time.RFC3339)
	followupState["lastDispatchedOrderId"] = "watchdog-order-1"
	followupState["lastDispatchedOrderStatus"] = "NEW"
	session, err = platform.store.UpdateLiveSessionState(session.ID, followupState)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	refreshed, err := platform.refreshLiveSessionPositionContext(session, firstRefreshAt.Add(2*time.Second), "test-refresh")
	if err != nil {
		t.Fatalf("second refresh live session position context failed: %v", err)
	}
	if proposal := mapValue(refreshed.State["lastExecutionProposal"]); len(proposal) != 0 {
		t.Fatalf("expected no duplicate watchdog proposal while exit order is working, got %+v", proposal)
	}
	if got := stringValue(refreshed.State["positionRecoveryStatus"]); got != livePositionRecoveryStatusClosingPending {
		t.Fatalf("expected %s recovery status, got %s", livePositionRecoveryStatusClosingPending, got)
	}
	if got := stringValue(refreshed.State["watchdogExitOrderId"]); got != "watchdog-order-1" {
		t.Fatalf("expected watchdog order id to be preserved, got %s", got)
	}
	if got := stringValue(refreshed.State["watchdogExitOrderStatus"]); got != "NEW" {
		t.Fatalf("expected watchdog order status NEW, got %s", got)
	}
	if got := stringValue(refreshed.State["watchdogExitStatus"]); got != "order-working" {
		t.Fatalf("expected watchdog order-working status, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextTracksActiveReduceOnlyExitOrder(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"openOrders": []map[string]any{
			{
				"symbol":     "BTCUSDT",
				"origType":   "MARKET",
				"type":       "MARKET",
				"status":     "NEW",
				"reduceOnly": true,
				"orderId":    "watchdog-order-2",
			},
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 4, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	if proposal := mapValue(updated.State["lastExecutionProposal"]); len(proposal) != 0 {
		t.Fatalf("expected active reduce-only exit order to suppress duplicate watchdog proposal, got %+v", proposal)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != livePositionRecoveryStatusClosingPending {
		t.Fatalf("expected %s recovery status, got %s", livePositionRecoveryStatusClosingPending, got)
	}
	if got := stringValue(updated.State["watchdogExitOrderId"]); got != "watchdog-order-2" {
		t.Fatalf("expected recovered watchdog order id, got %s", got)
	}
	if got := stringValue(updated.State["watchdogExitOrderStatus"]); got != "NEW" {
		t.Fatalf("expected recovered watchdog order status NEW, got %s", got)
	}
	if got := stringValue(updated.State["watchdogExitStatus"]); got != "order-working" {
		t.Fatalf("expected watchdog order-working status, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextDoesNotGenerateWatchdogFallbackFromLocalSyncSnapshot(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"source":         "platform-live-reconciliation",
		"syncStatus":     "SYNCED",
		"openOrderCount": 0,
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 4, 5, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	if proposal := mapValue(updated.State["lastExecutionProposal"]); len(proposal) != 0 {
		t.Fatalf("expected local fallback snapshot to suppress watchdog proposal, got %+v", proposal)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "unprotected-open-position" {
		t.Fatalf("expected unprotected-open-position to remain while awaiting authoritative sync, got %s", got)
	}
	if boolValue(updated.State["protectionRecoveryAuthoritative"]) {
		t.Fatal("expected local fallback snapshot to be marked non-authoritative")
	}
	if got := stringValue(updated.State["protectionRecoverySource"]); got != "platform-live-reconciliation" {
		t.Fatalf("expected local fallback recovery source to be recorded, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextGeneratesWatchdogFallbackFromAuthoritativeSyncSnapshot(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"source":         "binance-rest-account-v3",
		"syncStatus":     "SYNCED",
		"openOrderCount": 0,
		"openOrders":     []map[string]any{},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         68900,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 68900.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 17, 4, 10, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	proposal := executionProposalFromMap(mapValue(updated.State["lastExecutionProposal"]))
	if proposal.Reason != "sl-breached-fallback" {
		t.Fatalf("expected sl-breached-fallback reason, got %s", proposal.Reason)
	}
	if proposal.Side != "SELL" {
		t.Fatalf("expected SELL exit side, got %s", proposal.Side)
	}
	if proposal.Type != "MARKET" || !proposal.ReduceOnly {
		t.Fatalf("expected reduce-only MARKET fallback proposal, got type=%s reduceOnly=%t", proposal.Type, proposal.ReduceOnly)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != livePositionRecoveryStatusClosingPending {
		t.Fatalf("expected %s recovery status, got %s", livePositionRecoveryStatusClosingPending, got)
	}
	if !boolValue(updated.State["protectionRecoveryAuthoritative"]) {
		t.Fatal("expected exchange sync snapshot to remain authoritative")
	}
	if got := stringValue(updated.State["protectionRecoverySource"]); got != "binance-rest-account-v3" {
		t.Fatalf("expected authoritative recovery source to be recorded, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextRebuildsVirtualWatermarksFromVirtualPosition(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["virtualPosition"] = map[string]any{
		"id":         "virtual|session-1|signal-1",
		"virtual":    true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   0.0,
	}
	state["watermarkPositionKey"] = encodeLivePositionWatermarkIdentityComponent("position-1") + "|BTCUSDT|LONG|49000.00000000"
	state["hwm"] = 53000.0
	state["lwm"] = 49000.0
	state["lastStrategyEvaluationSignalBarStates"] = map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"atr14":     900.0,
			"current": map[string]any{
				"close": 51000.0,
				"high":  51100.0,
				"low":   50500.0,
			},
			"prevBar1": map[string]any{
				"high": 50800.0,
				"low":  49800.0,
			},
			"prevBar2": map[string]any{
				"high": 50700.0,
				"low":  49700.0,
			},
		},
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	expectedKey := encodeLivePositionWatermarkIdentityComponent("virtual|session-1|signal-1") + "|BTCUSDT|LONG|50000.00000000"
	if got := stringValue(updated.State["watermarkPositionKey"]); got != expectedKey {
		t.Fatalf("expected virtual watermark key to be rebuilt, got %s", got)
	}
	if got := parseFloatValue(updated.State["hwm"]); got != 51000.0 {
		t.Fatalf("expected virtual watermark hwm to be rebuilt from virtual position context, got %v", got)
	}
	if got := parseFloatValue(updated.State["lwm"]); got != 50000.0 {
		t.Fatalf("expected virtual watermark lwm to be reset to entry/virtual context, got %v", got)
	}
	if !boolValue(updated.State["hasRecoveredVirtualPosition"]) {
		t.Fatal("expected virtual recovery flag to remain set")
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "monitoring-virtual-position" {
		t.Fatalf("expected monitoring-virtual-position, got %s", got)
	}
	liveState := mapValue(updated.State["livePositionState"])
	if got := stringValue(liveState["watermarkPositionKey"]); got != expectedKey {
		t.Fatalf("expected rebuilt livePositionState watermark key, got %s", got)
	}
}

func TestRefreshLiveSessionPositionContextClearsStaleLivePositionStateWithoutRealOrVirtualPosition(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["livePositionState"] = map[string]any{
		"found":                true,
		"symbol":               "BTCUSDT",
		"side":                 "LONG",
		"entryPrice":           50000.0,
		"watermarkPositionKey": encodeLivePositionWatermarkIdentityComponent("position-1") + "|BTCUSDT|LONG|50000.00000000",
		"hwm":                  52000.0,
		"lwm":                  50000.0,
	}
	state["watermarkPositionKey"] = encodeLivePositionWatermarkIdentityComponent("position-1") + "|BTCUSDT|LONG|50000.00000000"
	state["hwm"] = 52000.0
	state["lwm"] = 50000.0
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 12, 7, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "flat" {
		t.Fatalf("expected flat recovery status after stale position cleanup, got %s", got)
	}
	if _, ok := updated.State["watermarkPositionKey"]; ok {
		t.Fatal("expected stale watermarkPositionKey to be cleared")
	}
	if _, ok := updated.State["hwm"]; ok {
		t.Fatal("expected stale hwm to be cleared")
	}
	if _, ok := updated.State["lwm"]; ok {
		t.Fatal("expected stale lwm to be cleared")
	}
	if liveState := mapValue(updated.State["livePositionState"]); len(liveState) != 0 {
		t.Fatalf("expected stale livePositionState to be cleared, got %+v", liveState)
	}
}

func TestRefreshLiveSessionPositionContextTreatsZeroQuantityStoredPositionAsFlat(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0,
		EntryPrice:        50000.0,
		MarkPrice:         50010.0,
	}); err != nil {
		t.Fatalf("save zero-quantity position failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 12, 8, 0, 0, 0, time.UTC), "test-zero-qty")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "flat" {
		t.Fatalf("expected flat recovery status for zero-quantity stored position, got %s", got)
	}
	if boolValue(updated.State["hasRecoveredPosition"]) {
		t.Fatal("expected zero-quantity stored position not to count as recovered position")
	}
	if got := parseFloatValue(mapValue(updated.State["recoveredPosition"])["quantity"]); got != 0 {
		t.Fatalf("expected recovered position snapshot quantity 0, got %v", got)
	}
}

func TestSyncLiveAccountReturnsFallbackSnapshotWithoutReportingAdapterFailure(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key:     "test-sync-fallback",
		syncErr: errors.New("adapter sync failed"),
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-sync-fallback",
		"connectionMode": "disabled",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	synced, err := platform.SyncLiveAccount("live-main")
	if err != nil {
		t.Fatalf("expected fallback sync to succeed, got %v", err)
	}
	if stringValue(synced.Metadata["lastLiveSyncAt"]) == "" {
		t.Fatal("expected fallback sync to persist lastLiveSyncAt")
	}
	if mapValue(synced.Metadata["liveSyncSnapshot"]) == nil {
		t.Fatal("expected fallback sync snapshot to be persisted")
	}
	accountSync := mapValue(mapValue(synced.Metadata["healthSummary"])["accountSync"])
	if stringValue(accountSync["lastSuccessAt"]) == "" {
		t.Fatal("expected accountSync health to retain successful fallback state")
	}
	if got := parseFloatValue(mapValue(accountSync["today"])["syncCount"]); got != 1 {
		t.Fatalf("expected fallback sync to record one syncCount, got %v", got)
	}
	if got := stringValue(accountSync["lastError"]); got != "" {
		t.Fatalf("expected no accountSync error after successful fallback, got %s", got)
	}
}

func TestSyncLiveAccountReturnsFailureWhenLocalFallbackFails(t *testing.T) {
	baseStore := memory.NewStore()
	platform := NewPlatform(&testFailingListOrdersStore{
		Store:     baseStore,
		listError: errors.New("orders unavailable"),
	})
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key:     "test-sync-failing",
		syncErr: errors.New("adapter sync failed"),
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-sync-failing",
		"connectionMode": "disabled",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	if _, err := platform.SyncLiveAccount("live-main"); err == nil {
		t.Fatal("expected local fallback failure to be returned")
	} else if !strings.Contains(err.Error(), "orders unavailable") {
		t.Fatalf("expected local fallback error in returned message, got %v", err)
	}

	updated, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("reload account failed: %v", err)
	}
	accountSync := mapValue(mapValue(updated.Metadata["healthSummary"])["accountSync"])
	if got := maxIntValue(accountSync["consecutiveErrorCount"], 0); got != 1 {
		t.Fatalf("expected one recorded sync failure, got %d", got)
	}
	if !strings.Contains(stringValue(accountSync["lastError"]), "orders unavailable") {
		t.Fatalf("expected recorded sync failure to mention local fallback error, got %s", stringValue(accountSync["lastError"]))
	}
}

func TestSyncLiveAccountRecordsAdapterResolutionFailuresInHealthSummary(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey": "missing-adapter",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	if _, err := platform.SyncLiveAccount("live-main"); err == nil {
		t.Fatal("expected adapter resolution failure to be returned")
	} else if !strings.Contains(err.Error(), "live adapter not registered") {
		t.Fatalf("expected adapter resolution failure in returned error, got %v", err)
	}

	updated, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("reload account failed: %v", err)
	}
	accountSync := mapValue(mapValue(updated.Metadata["healthSummary"])["accountSync"])
	if got := maxIntValue(accountSync["consecutiveErrorCount"], 0); got != 1 {
		t.Fatalf("expected one recorded adapter resolution failure, got %d", got)
	}
	if stringValue(accountSync["lastAttemptAt"]) == "" {
		t.Fatal("expected adapter resolution failure to record lastAttemptAt")
	}
	if !strings.Contains(stringValue(accountSync["lastError"]), "live adapter not registered") {
		t.Fatalf("expected recorded adapter resolution failure, got %s", stringValue(accountSync["lastError"]))
	}
}

func TestSyncActiveLiveAccountsReturnsPerAccountSyncErrors(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey": "missing-adapter",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}

	err = platform.syncActiveLiveAccounts(time.Now().UTC())
	if err == nil {
		t.Fatal("expected active live account sync failure to be surfaced")
	}
	if !strings.Contains(err.Error(), "live-main") {
		t.Fatalf("expected returned error to include account id, got %v", err)
	}
	if !strings.Contains(err.Error(), "live adapter not registered") {
		t.Fatalf("expected returned error to include sync failure reason, got %v", err)
	}
}

func TestSyncActiveLiveAccountsThrottlesFailedRetriesUntilFreshnessWindow(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.runtimePolicy.LiveAccountSyncFreshnessSecs = 60

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey": "missing-adapter",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}

	firstTick := time.Now().UTC()
	if err := platform.syncActiveLiveAccounts(firstTick); err == nil {
		t.Fatal("expected first active live account sync attempt to fail")
	}

	updated, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("reload account failed: %v", err)
	}
	accountSync := mapValue(mapValue(updated.Metadata["healthSummary"])["accountSync"])
	lastAttemptAt := parseOptionalRFC3339(stringValue(accountSync["lastAttemptAt"]))
	if lastAttemptAt.IsZero() {
		t.Fatal("expected failed sync attempt to record lastAttemptAt")
	}

	if err := platform.syncActiveLiveAccounts(lastAttemptAt.Add(10 * time.Second)); err != nil {
		t.Fatalf("expected retry within freshness window to be throttled, got %v", err)
	}
	if err := platform.syncActiveLiveAccounts(lastAttemptAt.Add(61 * time.Second)); err == nil {
		t.Fatal("expected retry after freshness window to attempt sync again")
	}
}

func TestSyncLiveAccountNormalizesAdapterSuccessHealthState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.runtimePolicy.LiveAccountSyncFreshnessSecs = 60
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-sync-success",
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-sync-success",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	account.Metadata["healthSummary"] = map[string]any{
		"accountSync": map[string]any{
			"consecutiveErrorCount": 2,
			"lastError":             "stale failure",
			"lastErrorAt":           time.Date(2026, 4, 14, 23, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	synced, err := platform.SyncLiveAccount("live-main")
	if err != nil {
		t.Fatalf("expected adapter sync success, got %v", err)
	}
	if stringValue(synced.Metadata["lastLiveSyncAt"]) == "" {
		t.Fatal("expected adapter sync success to persist lastLiveSyncAt")
	}
	accountSync := mapValue(mapValue(synced.Metadata["healthSummary"])["accountSync"])
	if got := parseFloatValue(accountSync["consecutiveErrorCount"]); got != 0 {
		t.Fatalf("expected adapter sync success to clear consecutiveErrorCount, got %v", got)
	}
	if got := stringValue(accountSync["lastError"]); got != "" {
		t.Fatalf("expected adapter sync success to clear lastError, got %s", got)
	}
	if stringValue(accountSync["lastSuccessAt"]) == "" {
		t.Fatal("expected adapter sync success to record lastSuccessAt")
	}
	if got := parseFloatValue(mapValue(accountSync["today"])["syncCount"]); got != 1 {
		t.Fatalf("expected adapter sync success to record one syncCount, got %v", got)
	}
	if got := stringValue(accountSync["lastSource"]); got != "live-account-adapter" && got != "test-sync-success" {
		t.Fatalf("expected adapter sync success to set a normalized lastSource, got %s", got)
	}
	if platform.shouldRefreshLiveAccountSync(synced, time.Now().UTC().Add(10*time.Second)) {
		t.Fatal("expected freshly normalized adapter sync to stay within freshness window")
	}
}

func TestSyncLiveAccountDoesNotDoubleCountPersistedAdapterSuccessHealth(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.runtimePolicy.LiveAccountSyncFreshnessSecs = 60
	syncedAt := time.Date(2026, 4, 15, 2, 20, 0, 0, time.UTC)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key:                 "test-sync-persisted-success",
		persistsSyncSuccess: true,
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "persisted-adapter",
				"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
				"syncedAt":        syncedAt.Format(time.RFC3339),
				"bindingMode":     stringValue(binding["connectionMode"]),
				"executionMode":   stringValue(binding["executionMode"]),
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
			}
			account.Metadata["lastLiveSyncAt"] = syncedAt.Format(time.RFC3339)
			updateAccountSyncSuccessHealth(&account, syncedAt, time.Time{})
			return p.store.UpdateAccount(account)
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-sync-persisted-success",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	synced, err := platform.SyncLiveAccount("live-main")
	if err != nil {
		t.Fatalf("expected persisted adapter sync success, got %v", err)
	}
	accountSync := mapValue(mapValue(synced.Metadata["healthSummary"])["accountSync"])
	if got := parseFloatValue(mapValue(accountSync["today"])["syncCount"]); got != 1 {
		t.Fatalf("expected persisted adapter sync success to keep syncCount at one, got %v", got)
	}
	if got := stringValue(accountSync["lastSuccessAt"]); got != syncedAt.Format(time.RFC3339) {
		t.Fatalf("expected persisted adapter sync success to keep lastSuccessAt, got %s", got)
	}
	if platform.shouldRefreshLiveAccountSync(synced, syncedAt.Add(10*time.Second)) {
		t.Fatal("expected persisted adapter sync success to stay within freshness window")
	}
}

func TestResolveRecoveredLiveEntryPriceFallsBackThroughRiskAndExistingValues(t *testing.T) {
	if got := resolveRecoveredLiveEntryPrice(74500.0, 74400.0, 74300.0); got != 74500.0 {
		t.Fatalf("expected primary entry price to win, got %v", got)
	}
	if got := resolveRecoveredLiveEntryPrice(0, 74400.0, 74300.0); got != 74400.0 {
		t.Fatalf("expected secondary entry price fallback, got %v", got)
	}
	if got := resolveRecoveredLiveEntryPrice(0, 0, 74300.0); got != 74300.0 {
		t.Fatalf("expected existing entry price fallback, got %v", got)
	}
}

func TestReconcileLiveAccountPositionsFallsBackToBreakEvenPrice(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	_, err = platform.reconcileLiveAccountPositions(account, []map[string]any{
		{
			"symbol":         "BTCUSDT",
			"positionAmt":    -0.002,
			"entryPrice":     0.0,
			"breakEvenPrice": 74512.34,
			"markPrice":      74618.69,
		},
	})
	if err != nil {
		t.Fatalf("reconcile live account positions failed: %v", err)
	}

	position, found, err := platform.store.FindPosition("live-main", "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected reconciled BTCUSDT position")
	}
	if position.Side != "SHORT" {
		t.Fatalf("expected SHORT side, got %s", position.Side)
	}
	if position.EntryPrice != 74512.34 {
		t.Fatalf("expected breakEvenPrice fallback to populate entry price, got %v", position.EntryPrice)
	}
}

func TestReconcileLiveAccountPositionsPreservesExistingEntryPriceWhenSnapshotIsZero(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SHORT",
		Quantity:          0.002,
		EntryPrice:        74123.45,
		MarkPrice:         74618.69,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	_, err = platform.reconcileLiveAccountPositions(account, []map[string]any{
		{
			"symbol":      "BTCUSDT",
			"positionAmt": -0.002,
			"entryPrice":  0.0,
			"markPrice":   74618.69,
		},
	})
	if err != nil {
		t.Fatalf("reconcile live account positions failed: %v", err)
	}

	position, found, err := platform.store.FindPosition("live-main", "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected reconciled BTCUSDT position")
	}
	if position.EntryPrice != 74123.45 {
		t.Fatalf("expected existing entry price to be preserved, got %v", position.EntryPrice)
	}
}

func TestRefreshLiveSessionPositionContextBuildsRiskStateFromRecoveredBreakEvenEntryPrice(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"openOrders": []map[string]any{},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	if _, err := platform.reconcileLiveAccountPositions(account, []map[string]any{
		{
			"symbol":         "BTCUSDT",
			"positionAmt":    -0.002,
			"entryPrice":     0.0,
			"breakEvenPrice": 74512.34,
			"markPrice":      74520.0,
		},
	}); err != nil {
		t.Fatalf("reconcile live account positions failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = testLiveRecoverySignalBarStates("BTCUSDT", 74520.0)
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Date(2026, 4, 20, 2, 0, 0, 0, time.UTC), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	liveState := mapValue(updated.State["livePositionState"])
	if len(liveState) == 0 {
		t.Fatal("expected live position state to be rebuilt from recovered entry price")
	}
	if got := parseFloatValue(liveState["entryPrice"]); got != 74512.34 {
		t.Fatalf("expected recovered break-even entry price in live position state, got %v", got)
	}
	if got := stringValue(liveState["side"]); got != "SHORT" {
		t.Fatalf("expected SHORT live position side, got %s", got)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "unprotected-open-position" {
		t.Fatalf("expected unprotected-open-position without recovered protection orders, got %s", got)
	}
}

func TestRecoverRunningLiveSessionCompletesRecoveredPositionMetadata(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "RUNNING" {
		t.Fatalf("expected recovered session to stay RUNNING, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["recoveryMetadataStatus"]); got != liveRecoveryMetadataStatusComplete {
		t.Fatalf("expected recovery metadata status complete, got %s", got)
	}
	if got := stringValue(recovered.State["recoveryMode"]); got != "" {
		t.Fatalf("expected no restricted recovery mode, got %s", got)
	}
	position, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected recovered position to remain present")
	}
	if got := position.StrategyVersionID; got != "strategy-version-bk-1d-v010" {
		t.Fatalf("expected recovered strategy version to be completed, got %s", got)
	}
}

func TestRecoverRunningLiveSessionRequiresRESTVerificationBeforeRestartTakeover(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key:     "test-restart-rest-required",
		syncErr: errors.New("rest adapter unavailable"),
	})
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-restart-rest-required",
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "BLOCKED" {
		t.Fatalf("expected restart takeover to stay BLOCKED until REST verify succeeds, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusStale {
		t.Fatalf("expected stale reconcile gate while REST verify is pending, got %s", got)
	}
	if got := stringValue(recovered.State["positionReconcileGateScenario"]); got != "startup-recovery" {
		t.Fatalf("expected startup-recovery gate scenario, got %s", got)
	}
	if got := stringValue(recovered.State["recoveryTakeoverState"]); got != liveRecoveryTakeoverStateStaleSync {
		t.Fatalf("expected stale-sync takeover state while restart verify is pending, got %s", got)
	}
	account, err = platform.store.GetAccount(session.AccountID)
	if err != nil {
		t.Fatalf("reload account failed: %v", err)
	}
	if !liveAccountPositionReconcilePending(account) {
		t.Fatal("expected account reconcile requirement to remain pending after fallback-only sync")
	}
	if _, err := platform.ClosePosition(position.ID); err == nil {
		t.Fatal("expected restart takeover close to stay blocked until REST verify completes")
	}
}

func TestRecoverRunningLiveSessionAllowsTakeoverAfterVerifiedReconcileMatch(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-reconcile-match", []map[string]any{
		{
			"symbol":      "BTCUSDT",
			"positionAmt": 0.01,
			"entryPrice":  68000.0,
			"markPrice":   68100.0,
		},
	})
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "RUNNING" {
		t.Fatalf("expected recovered session to stay RUNNING, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusVerified {
		t.Fatalf("expected verified reconcile gate status, got %s", got)
	}
	if got := stringValue(recovered.State["positionReconcileGateScenario"]); got != "db-position-matches-exchange" {
		t.Fatalf("expected db-position-matches-exchange scenario, got %s", got)
	}
	if boolValue(recovered.State["positionReconcileGateBlocking"]) {
		t.Fatal("expected verified reconcile gate not to block execution")
	}
}

func TestRecoverRunningLiveSessionBlocksWhenDBPositionMissingOnExchange(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-reconcile-stale", []map[string]any{})
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "BLOCKED" {
		t.Fatalf("expected stale recovery to stay BLOCKED, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["recoveryMode"]); got != liveRecoveryModeReconcileGateBlocked {
		t.Fatalf("expected reconcile gate blocked mode, got %s", got)
	}
	if got := stringValue(recovered.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusStale {
		t.Fatalf("expected stale reconcile gate status, got %s", got)
	}
	if got := stringValue(recovered.State["positionRecoveryStatus"]); got != livePositionReconcileGateStatusStale {
		t.Fatalf("expected stale position recovery status %s, got %s", livePositionReconcileGateStatusStale, got)
	}
	if got := stringValue(recovered.State["recoveryTakeoverState"]); got != liveRecoveryTakeoverStateStaleSync {
		t.Fatalf("expected takeover state %s, got %s", liveRecoveryTakeoverStateStaleSync, got)
	}
	if got := stringValue(recovered.State["positionReconcileGateScenario"]); got != "db-position-exchange-missing" {
		t.Fatalf("expected db-position-exchange-missing scenario, got %s", got)
	}
	if !boolValue(recovered.State["positionReconcileGateBlocking"]) {
		t.Fatal("expected stale reconcile gate to block execution")
	}
	if _, err := platform.ClosePosition(position.ID); err == nil {
		t.Fatal("expected ClosePosition to be blocked by reconcile gate")
	}
}

func TestRecoverRunningLiveSessionAllowsControlledExchangeOnlyTakeover(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-reconcile-exchange-only", []map[string]any{
		{
			"symbol":      "BTCUSDT",
			"positionAmt": -0.02,
			"entryPrice":  70250.0,
			"markPrice":   70100.0,
		},
	})
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "RUNNING" {
		t.Fatalf("expected controlled exchange takeover to proceed, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusAdopted {
		t.Fatalf("expected adopted reconcile gate status, got %s", got)
	}
	if got := stringValue(recovered.State["positionReconcileGateScenario"]); got != "exchange-position-db-missing" {
		t.Fatalf("expected exchange-position-db-missing scenario, got %s", got)
	}
	position, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected exchange-backed position to be created in store")
	}
	if position.Side != "SHORT" || position.Quantity != 0.02 {
		t.Fatalf("expected short exchange-backed takeover position, got side=%s qty=%v", position.Side, position.Quantity)
	}
}

func TestRecoverRunningLiveSessionBlocksSideMismatchAndPreventsCloseExecution(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-reconcile-conflict", []map[string]any{
		{
			"symbol":      "BTCUSDT",
			"positionAmt": -0.01,
			"entryPrice":  68000.0,
			"markPrice":   67950.0,
		},
	})
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "BLOCKED" {
		t.Fatalf("expected mismatched recovery to stay BLOCKED, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusConflict {
		t.Fatalf("expected conflict reconcile gate status, got %s", got)
	}
	if got := stringValue(recovered.State["positionReconcileGateScenario"]); got != "side-mismatch" {
		t.Fatalf("expected side-mismatch scenario, got %s", got)
	}
	stored, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected local conflict position to be preserved")
	}
	if stored.Side != "LONG" || stored.Quantity != 0.01 {
		t.Fatalf("expected conflicting local position to stay unchanged, got side=%s qty=%v", stored.Side, stored.Quantity)
	}
	if _, err := platform.ClosePosition(position.ID); err == nil {
		t.Fatal("expected ClosePosition to be blocked during unresolved conflict")
	}
}

func TestUnresolvedReconcileMismatchPreventsAutoDispatch(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-reconcile-auto-dispatch", []map[string]any{
		{
			"symbol":      "BTCUSDT",
			"positionAmt": 0.02,
			"entryPrice":  68000.0,
			"markPrice":   67950.0,
		},
	})
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":                  "BTCUSDT",
		"signalTimeframe":         "1d",
		"dispatchMode":            "auto-dispatch",
		"dispatchCooldownSeconds": 0,
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if got := stringValue(recovered.State["positionReconcileGateScenario"]); got != "quantity-mismatch" {
		t.Fatalf("expected quantity-mismatch scenario, got %s", got)
	}
	intent := map[string]any{
		"action":   "exit",
		"side":     "SELL",
		"symbol":   "BTCUSDT",
		"status":   "dispatchable",
		"quantity": 0.01,
		"metadata": map[string]any{},
	}
	if shouldAutoDispatchLiveIntent(recovered, intent, time.Now().UTC()) {
		t.Fatal("expected unresolved reconcile mismatch to suppress auto-dispatch")
	}
}

func TestRecoverRunningLiveSessionFallsBackToCloseOnlyWhenMetadataIsAmbiguous(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create primary live session failed: %v", err)
	}
	secondary, err := platform.store.CreateLiveSession("live-main", "strategy-ambiguous")
	if err != nil {
		t.Fatalf("create secondary live session failed: %v", err)
	}
	secondaryState := cloneMetadata(secondary.State)
	secondaryState["symbol"] = "BTCUSDT"
	if _, err := platform.store.UpdateLiveSessionState(secondary.ID, secondaryState); err != nil {
		t.Fatalf("update secondary live session state failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "BLOCKED" {
		t.Fatalf("expected recovered session to downgrade into BLOCKED close-only mode, got %s", recovered.Status)
	}
	if got := stringValue(recovered.State["recoveryMode"]); got != liveRecoveryModeCloseOnlyTakeover {
		t.Fatalf("expected recovery mode %s, got %s", liveRecoveryModeCloseOnlyTakeover, got)
	}
	if got := stringValue(recovered.State["recoveryMetadataStatus"]); got != liveRecoveryMetadataStatusIncomplete {
		t.Fatalf("expected incomplete recovery metadata status, got %s", got)
	}
	if got := stringValue(recovered.State["lastStrategyEvaluationStatus"]); got != liveRecoveryModeCloseOnlyTakeover {
		t.Fatalf("expected strategy evaluation status %s, got %s", liveRecoveryModeCloseOnlyTakeover, got)
	}
	if got := stringValue(recovered.State["positionRecoveryStatus"]); got != liveRecoveryModeCloseOnlyTakeover {
		t.Fatalf("expected position recovery status %s, got %s", liveRecoveryModeCloseOnlyTakeover, got)
	}
	position, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected recovered position to remain present")
	}
	if got := position.StrategyVersionID; got != "" {
		t.Fatalf("expected ambiguous recovered position to remain unlinked, got %s", got)
	}
}

func TestClosePositionAllowsRecoveredCloseOnlyTakeoverWithoutRuntimeLinkage(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-close-only"})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-close-only",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create primary live session failed: %v", err)
	}
	secondary, err := platform.store.CreateLiveSession("live-main", "strategy-ambiguous")
	if err != nil {
		t.Fatalf("create secondary live session failed: %v", err)
	}
	secondaryState := cloneMetadata(secondary.State)
	secondaryState["symbol"] = "BTCUSDT"
	if _, err := platform.store.UpdateLiveSessionState(secondary.ID, secondaryState); err != nil {
		t.Fatalf("update secondary live session state failed: %v", err)
	}
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if got := stringValue(recovered.State["recoveryMode"]); got != liveRecoveryModeCloseOnlyTakeover {
		t.Fatalf("expected close-only recovery mode, got %s", got)
	}

	platform.mu.Lock()
	platform.signalSessions = map[string]domain.SignalRuntimeSession{}
	platform.mu.Unlock()

	order, err := platform.ClosePosition(position.ID)
	if err != nil {
		t.Fatalf("expected close-only takeover close to bypass missing runtime linkage, got %v", err)
	}
	if !boolValue(order.Metadata["skipRuntimeCheck"]) {
		t.Fatal("expected close-only takeover close order to skip runtime preflight")
	}
	if !boolValue(order.Metadata["recoveryCloseOnlyTakeover"]) {
		t.Fatal("expected close-only takeover marker on close order metadata")
	}
	if got := stringValue(order.Status); got == "" {
		t.Fatal("expected live close order to be persisted with a status")
	}
}

func TestRecoverRunningLiveSessionPreservesRemainingQuantityAfterPartialFillRestart(t *testing.T) {
	platform, session, _, _, _ := prepareLiveDecisionTelemetryFixture(t)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-partial-fill-restart",
		syncOrderFunc: func(_ domain.Account, order domain.Order, _ map[string]any) (LiveOrderSync, error) {
			return LiveOrderSync{
				Status:   "PARTIALLY_FILLED",
				SyncedAt: time.Now().UTC().Format(time.RFC3339),
				Fills: []LiveFillReport{{
					Quantity: 0.004,
					Price:    68950.0,
				}},
			}, nil
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-partial-fill-restart",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        69000,
		MarkPrice:         68980,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	order, err := platform.store.CreateOrder(domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Status:            "ACCEPTED",
		Quantity:          0.01,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"source":        "live-session-intent",
			"liveSessionId": session.ID,
			"executionProposal": map[string]any{
				"role":       "exit",
				"reason":     "SL",
				"side":       "SELL",
				"symbol":     "BTCUSDT",
				"quantity":   0.01,
				"reduceOnly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["lastDispatchedOrderId"] = order.ID
	state["lastDispatchedOrderStatus"] = "ACCEPTED"
	state["lastDispatchedIntent"] = map[string]any{
		"role":       "exit",
		"reason":     "SL",
		"side":       "SELL",
		"symbol":     "BTCUSDT",
		"quantity":   0.01,
		"reduceOnly": true,
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "RUNNING" {
		t.Fatalf("expected recovered session to stay RUNNING, got %s", recovered.Status)
	}

	syncedOrder, err := platform.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("get synced order failed: %v", err)
	}
	if got := syncedOrder.Status; got != "PARTIALLY_FILLED" {
		t.Fatalf("expected partial fill order status, got %s", got)
	}
	if got := parseFloatValue(syncedOrder.Metadata["filledQuantity"]); got != 0.004 {
		t.Fatalf("expected filled quantity 0.004, got %v", got)
	}
	if got := parseFloatValue(syncedOrder.Metadata["remainingQuantity"]); got != 0.006 {
		t.Fatalf("expected remaining quantity 0.006, got %v", got)
	}
	position, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected remaining position after partial fill restart")
	}
	if got := position.Quantity; got != 0.006 {
		t.Fatalf("expected remaining position quantity 0.006, got %v", got)
	}
	if got := stringValue(recovered.State["lastSyncedOrderStatus"]); got != "PARTIALLY_FILLED" {
		t.Fatalf("expected last synced order status PARTIALLY_FILLED, got %s", got)
	}
}

func TestCompleteRecoveredLiveSessionMetadataClearsCloseOnlyTakeoverWhenPositionFlat(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["recoveryMode"] = liveRecoveryModeCloseOnlyTakeover
	state["recoveryCloseOnlyReason"] = "missing-strategy-version"
	state["recoveryCloseOnlyDetail"] = "test"
	state["recoveryCloseOnlyAt"] = time.Now().UTC().Format(time.RFC3339)
	state["runtimeMode"] = liveRecoveryModeCloseOnlyTakeover
	state["signalRuntimeMode"] = liveRecoveryModeCloseOnlyTakeover
	state["signalRuntimeRequired"] = false
	state["signalRuntimeReady"] = false
	state["lastStrategyEvaluationStatus"] = liveRecoveryModeCloseOnlyTakeover
	state["positionRecoveryStatus"] = liveRecoveryModeCloseOnlyTakeover
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, _, incomplete, err := platform.completeRecoveredLiveSessionMetadata(session)
	if err != nil {
		t.Fatalf("complete recovered live session metadata failed: %v", err)
	}
	if incomplete {
		t.Fatal("expected flat session not to remain incomplete")
	}
	if got := stringValue(updated.State["recoveryMode"]); got != "" {
		t.Fatalf("expected recoveryMode to be cleared, got %s", got)
	}
	if got := stringValue(updated.State["recoveryCloseOnlyReason"]); got != "" {
		t.Fatalf("expected recoveryCloseOnlyReason to be cleared, got %s", got)
	}
	if got := stringValue(updated.State["lastStrategyEvaluationStatus"]); got != "" {
		t.Fatalf("expected lastStrategyEvaluationStatus takeover marker to be cleared, got %s", got)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "flat" {
		t.Fatalf("expected positionRecoveryStatus to reset to flat, got %s", got)
	}
}

func TestSyncLiveSessionsForAccountSnapshotDoesNotResumeOtherBlockedSession(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeRequired"] = false
	state["signalRuntimeReady"] = false
	state["lastStrategyEvaluationStatus"] = "operator-paused"
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}
	session, err = platform.store.UpdateLiveSessionStatus(session.ID, "BLOCKED")
	if err != nil {
		t.Fatalf("update live session status failed: %v", err)
	}
	account, err := platform.store.GetAccount(session.AccountID)
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	platform.syncLiveSessionsForAccountSnapshot(account)

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := updated.Status; got != "BLOCKED" {
		t.Fatalf("expected unrelated blocked session to stay BLOCKED, got %s", got)
	}
}

func TestEnsureLiveExecutionPlanPreservesSignalRuntimeRequirementWhenFlat(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	for _, payload := range []map[string]any{
		{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "BTCUSDT",
			"options":   map[string]any{"timeframe": "1d"},
		},
		{
			"sourceKey": "binance-trade-tick",
			"role":      "trigger",
			"symbol":    "BTCUSDT",
		},
	} {
		if _, err := platform.BindStrategySignalSource("strategy-bk-1d", payload); err != nil {
			t.Fatalf("bind strategy signal source failed: %v", err)
		}
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if !boolValue(session.State["signalRuntimeRequired"]) {
		t.Fatalf("expected linked live session to require runtime before plan evaluation, got %#v", session.State)
	}

	eventTime := time.Date(2026, 4, 20, 8, 36, 10, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime,
		Price:     70000.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "cached-plan",
	}}
	platform.mu.Unlock()

	updated, plan, err := platform.ensureLiveExecutionPlan(session)
	if err != nil {
		t.Fatalf("ensure live execution plan failed: %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("expected cached plan to be preserved, got %#v", plan)
	}
	if _, ok := updated.State["signalRuntimeRequired"]; !ok {
		t.Fatalf("expected signalRuntimeRequired to remain present, got %#v", updated.State)
	}
	if !boolValue(updated.State["signalRuntimeRequired"]) {
		t.Fatalf("expected flat RUNNING session to keep signalRuntimeRequired, got %#v", updated.State)
	}
}

func TestStartLiveSessionRejectsActiveCloseOnlyTakeoverMode(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-start-close-only"})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-start-close-only",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	secondary, err := platform.store.CreateLiveSession("live-main", "strategy-ambiguous")
	if err != nil {
		t.Fatalf("create secondary live session failed: %v", err)
	}
	secondaryState := cloneMetadata(secondary.State)
	secondaryState["symbol"] = "BTCUSDT"
	if _, err := platform.store.UpdateLiveSessionState(secondary.ID, secondaryState); err != nil {
		t.Fatalf("update secondary live session state failed: %v", err)
	}

	recovered, err := platform.recoverRunningLiveSession(session)
	if err != nil {
		t.Fatalf("recover running live session failed: %v", err)
	}
	if recovered.Status != "BLOCKED" {
		t.Fatalf("expected blocked close-only recovery session, got %s", recovered.Status)
	}

	if _, err := platform.StartLiveSession(session.ID); err == nil {
		t.Fatal("expected StartLiveSession to reject active close-only takeover mode")
	} else if !strings.Contains(err.Error(), liveRecoveryModeCloseOnlyTakeover) {
		t.Fatalf("expected close-only takeover error, got %v", err)
	}
}

func TestStartLiveSessionRequiresRESTVerificationForHistoricalTakeoverActivation(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key:     "test-start-rest-required",
		syncErr: errors.New("rest adapter unavailable"),
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-start-rest-required",
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	_, err = platform.StartLiveSession(session.ID)
	if err == nil {
		t.Fatal("expected historical takeover activation to require REST verification before start")
	}
	if !strings.Contains(err.Error(), "requires authoritative reconcile before historical takeover activation") {
		t.Fatalf("expected fail-fast authoritative reconcile error, got %v", err)
	}
	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if updated.Status != "BLOCKED" {
		t.Fatalf("expected start-time takeover to stay BLOCKED, got %s", updated.Status)
	}
	if got := stringValue(updated.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusStale {
		t.Fatalf("expected stale reconcile gate on start-time takeover, got %s", got)
	}
	if got := stringValue(updated.State["positionReconcileGateScenario"]); got != "historical-takeover-activation" {
		t.Fatalf("expected historical takeover trigger scenario, got %s", got)
	}
}

func TestStartLiveSessionBackfillsFilledExitBeforeReconcileGateBlock(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	tradeTime := time.Date(2026, 4, 20, 12, 33, 23, 0, time.UTC)
	syncCalls := 0
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-start-filled-backfill",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "binance-rest-account-v3",
				"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
				"syncedAt":        time.Now().UTC().Format(time.RFC3339),
				"bindingMode":     stringValue(binding["connectionMode"]),
				"executionMode":   "rest",
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
				"positions":       []map[string]any{},
				"openOrders":      []map[string]any{},
			}
			var err error
			account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
			if err != nil {
				return domain.Account{}, err
			}
			reconcileGate, err := p.reconcileLiveAccountPositions(account, []map[string]any{})
			if err != nil {
				return domain.Account{}, err
			}
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["livePositionReconcileGate"] = reconcileGate
			account.Metadata["lastLivePositionSyncAt"] = time.Now().UTC().Format(time.RFC3339)
			clearLiveAccountPositionReconcileRequirement(account.Metadata)
			return p.store.UpdateAccount(account)
		},
		syncOrderFunc: func(_ domain.Account, order domain.Order, _ map[string]any) (LiveOrderSync, error) {
			syncCalls++
			return LiveOrderSync{
				Status:   "FILLED",
				SyncedAt: tradeTime.Format(time.RFC3339),
				Fills: []LiveFillReport{{
					Price:    67950.0,
					Quantity: order.Quantity,
					Fee:      0.01,
					Metadata: map[string]any{
						"exchangeOrderId": stringValue(order.Metadata["exchangeOrderId"]),
						"tradeId":         "trade-terminal-fill-1",
						"tradeTime":       tradeTime.Format(time.RFC3339),
					},
				}},
				Terminal:   true,
				FeeSource:  "exchange",
				FundingSrc: "exchange",
			}, nil
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-start-filled-backfill",
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67950,
	}); err != nil {
		t.Fatalf("save stale position failed: %v", err)
	}
	order, err := platform.store.CreateOrder(domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Status:            "FILLED",
		Quantity:          0.01,
		Price:             67950,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"source":          "live-session-intent",
			"liveSessionId":   session.ID,
			"exchangeOrderId": "exchange-order-terminal-fill-1",
			"executionProposal": map[string]any{
				"role":       "exit",
				"reason":     "SL",
				"side":       "SELL",
				"symbol":     "BTCUSDT",
				"quantity":   0.01,
				"reduceOnly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("create filled order failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastDispatchedOrderId"] = order.ID
	state["lastDispatchedOrderStatus"] = "FILLED"
	if _, err := platform.store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	started, err := platform.StartLiveSession(session.ID)
	if err != nil {
		t.Fatalf("expected StartLiveSession to self-heal stale filled exit, got %v", err)
	}
	if started.Status != "RUNNING" {
		t.Fatalf("expected session to start RUNNING after backfill, got %s", started.Status)
	}
	if syncCalls == 0 {
		t.Fatal("expected start-time recovery to sync the terminal filled order before gate evaluation")
	}
	if _, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected backfilled filled exit to clear stale local position before start completes")
	}
	syncedOrder, err := platform.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("get synced order failed: %v", err)
	}
	if got := parseFloatValue(syncedOrder.Metadata["filledQuantity"]); got != 0.01 {
		t.Fatalf("expected synced filledQuantity 0.01, got %v", got)
	}
	if got := stringValue(started.State["recoveryMode"]); got == liveRecoveryModeReconcileGateBlocked {
		t.Fatalf("expected reconcile gate block to clear after backfill, got %s", got)
	}
}

func TestStartLiveSessionSelfHealsStaleDBPositionViaReconcileHistory(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	syncedAt := time.Date(2026, 4, 20, 12, 33, 23, 0, time.UTC)
	configureTestLiveRESTReconcileHistoryAdapter(
		t,
		platform,
		"test-start-reconcile-history-heal",
		[]map[string]any{},
		map[string][]map[string]any{
			"BTCUSDT": {{
				"symbol":        "BTCUSDT",
				"orderId":       "9201",
				"clientOrderId": "client-9201",
				"status":        "FILLED",
				"side":          "SELL",
				"type":          "MARKET",
				"origType":      "MARKET",
				"origQty":       0.01,
				"executedQty":   0.01,
				"price":         67950.0,
				"avgPrice":      67950.0,
				"reduceOnly":    true,
				"closePosition": false,
				"time":          float64(syncedAt.Add(-2 * time.Minute).UnixMilli()),
				"updateTime":    float64(syncedAt.UnixMilli()),
			}},
		},
		map[string][]LiveFillReport{
			"BTCUSDT": {{
				Price:    67950.0,
				Quantity: 0.01,
				Fee:      0.01,
				Metadata: map[string]any{
					"exchangeOrderId": "9201",
					"tradeId":         "trade-9201",
					"tradeTime":       syncedAt.Format(time.RFC3339),
				},
			}},
		},
	)

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67950,
	}); err != nil {
		t.Fatalf("save stale position failed: %v", err)
	}

	started, err := platform.StartLiveSession(session.ID)
	if err != nil {
		t.Fatalf("expected StartLiveSession to self-heal stale db-position-exchange-missing state, got %v", err)
	}
	if started.Status != "RUNNING" {
		t.Fatalf("expected session to start RUNNING after reconcile history heal, got %s", started.Status)
	}
	if _, found, err := platform.store.FindPosition(session.AccountID, "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected reconcile history self-heal to clear stale BTCUSDT position before start completes")
	}
	if got := stringValue(started.State["recoveryMode"]); got == liveRecoveryModeReconcileGateBlocked {
		t.Fatalf("expected reconcile gate block to clear after reconcile history self-heal, got %s", got)
	}
	if got := stringValue(started.State["positionReconcileGateScenario"]); got == "db-position-exchange-missing" {
		t.Fatalf("expected stale db-position-exchange-missing scenario to clear after self-heal, got %s", got)
	}
}

func TestStartLiveSessionDowngradesIncompleteRecoveredMetadataToCloseOnly(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-start-incomplete"})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-start-incomplete",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	secondary, err := platform.store.CreateLiveSession("live-main", "strategy-ambiguous")
	if err != nil {
		t.Fatalf("create secondary live session failed: %v", err)
	}
	secondaryState := cloneMetadata(secondary.State)
	secondaryState["symbol"] = "BTCUSDT"
	if _, err := platform.store.UpdateLiveSessionState(secondary.ID, secondaryState); err != nil {
		t.Fatalf("update secondary live session state failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:  session.AccountID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.01,
		EntryPrice: 68000,
		MarkPrice:  68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	if _, err := platform.StartLiveSession(session.ID); err == nil {
		t.Fatal("expected StartLiveSession to block incomplete recovered metadata")
	} else if !strings.Contains(err.Error(), liveRecoveryModeCloseOnlyTakeover) {
		t.Fatalf("expected close-only takeover error, got %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if updated.Status != "BLOCKED" {
		t.Fatalf("expected session to downgrade to BLOCKED, got %s", updated.Status)
	}
	if got := stringValue(updated.State["recoveryMode"]); got != liveRecoveryModeCloseOnlyTakeover {
		t.Fatalf("expected recoveryMode %s, got %s", liveRecoveryModeCloseOnlyTakeover, got)
	}
	if got := stringValue(updated.State["recoveryMetadataStatus"]); got != liveRecoveryMetadataStatusIncomplete {
		t.Fatalf("expected incomplete recovery metadata status, got %s", got)
	}
}

type testLiveAccountSyncAdapter struct {
	key                 string
	syncErr             error
	persistsSyncSuccess bool
	syncSnapshotFunc    func(*Platform, domain.Account, map[string]any) (domain.Account, error)
	submitOrderFunc     func(domain.Account, domain.Order, map[string]any) (LiveOrderSubmission, error)
	syncOrderFunc       func(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error)
	cancelOrderFunc     func(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error)
}

func (a testLiveAccountSyncAdapter) Key() string {
	return a.key
}

func (a testLiveAccountSyncAdapter) Describe() map[string]any {
	return map[string]any{"key": a.key}
}

func (a testLiveAccountSyncAdapter) PersistsLiveAccountSyncSuccess() bool {
	return a.persistsSyncSuccess
}

func (a testLiveAccountSyncAdapter) ValidateAccountConfig(map[string]any) error {
	return nil
}

func (a testLiveAccountSyncAdapter) SubmitOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	if a.submitOrderFunc != nil {
		return a.submitOrderFunc(account, order, binding)
	}
	return LiveOrderSubmission{}, nil
}

func (a testLiveAccountSyncAdapter) SyncOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	if a.syncOrderFunc != nil {
		return a.syncOrderFunc(account, order, binding)
	}
	return LiveOrderSync{}, nil
}

func (a testLiveAccountSyncAdapter) CancelOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	if a.cancelOrderFunc != nil {
		return a.cancelOrderFunc(account, order, binding)
	}
	return LiveOrderSync{}, nil
}

func (a testLiveAccountSyncAdapter) SyncAccountSnapshot(platform *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
	if a.syncErr != nil {
		return domain.Account{}, a.syncErr
	}
	if a.syncSnapshotFunc != nil {
		return a.syncSnapshotFunc(platform, account, binding)
	}
	return account, nil
}

func configureTestLiveRESTReconcileAdapter(t *testing.T, platform *Platform, adapterKey string, exchangePositions []map[string]any) {
	t.Helper()
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: adapterKey,
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "binance-rest-account-v3",
				"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
				"syncedAt":        time.Now().UTC().Format(time.RFC3339),
				"bindingMode":     stringValue(binding["connectionMode"]),
				"executionMode":   "rest",
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
				"positions":       exchangePositions,
				"openOrders":      []map[string]any{},
			}
			var err error
			account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
			if err != nil {
				return domain.Account{}, err
			}
			reconcileGate, err := p.reconcileLiveAccountPositions(account, exchangePositions)
			if err != nil {
				return domain.Account{}, err
			}
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["livePositionReconcileGate"] = reconcileGate
			account.Metadata["lastLivePositionSyncAt"] = time.Now().UTC().Format(time.RFC3339)
			clearLiveAccountPositionReconcileRequirement(account.Metadata)
			return p.store.UpdateAccount(account)
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     adapterKey,
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}
}

type testFailingListOrdersStore struct {
	*memory.Store
	listError error
}

func (s *testFailingListOrdersStore) ListOrders() ([]domain.Order, error) {
	return nil, s.listError
}

func TestSyncLiveSessionRuntimePreservesConfiguredDispatchMode(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	// Prepare account and strategy to satisfy BuildSignalRuntimePlan
	acc, _ := platform.store.CreateAccount("acc-1", "LIVE", "binance-futures")
	strat, _ := platform.store.CreateStrategy("strat-1", "test", map[string]any{
		"strategyEngine": "bk-default",
	})
	stratID := strat["id"].(string)

	// Case 1: auto-dispatch is preserved
	// We create a session normally, which defaults to manual-review
	session, err := platform.CreateLiveSession("", acc.ID, stratID, nil)
	if err != nil {
		t.Fatalf("CreateLiveSession failed: %v", err)
	}

	// Manually set it to auto-dispatch and persist
	state := session.State
	state["dispatchMode"] = "auto-dispatch"
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("UpdateLiveSessionState failed: %v", err)
	}

	// Execution: Trigger sync (e.g. on restart/recovery)
	updated, err := platform.syncLiveSessionRuntime(session)
	if err != nil {
		t.Fatalf("syncLiveSessionRuntime failed: %v", err)
	}

	// Validation
	if got := stringValue(updated.State["dispatchMode"]); got != "auto-dispatch" {
		t.Errorf("expected auto-dispatch to be preserved after sync, got %s", got)
	}

	// Case 2: manual-review is preserved (default case)
	session2, _ := platform.CreateLiveSession("", acc.ID, stratID, nil)
	updated2, err := platform.syncLiveSessionRuntime(session2)
	if err != nil {
		t.Fatalf("syncLiveSessionRuntime failed: %v", err)
	}
	if got := stringValue(updated2.State["dispatchMode"]); got != "manual-review" {
		t.Errorf("expected manual-review to be preserved, got %s", got)
	}
}
