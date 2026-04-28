package service

import (
	"strconv"
	"testing"
	"time"
)

func TestPrepareLivePlanStepForSignalEvaluationExpiresStaleExitReentryWindow(t *testing.T) {
	barStart := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"sma5":      68050.0,
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
		barStart.Add(49*time.Hour),
		69010.0,
		"trade_tick.price",
		barStart.Add(-48*time.Hour),
		75600.0,
		"SELL",
		"entry",
		"SL-Reentry",
	)
	if gotRole != "entry" || gotReason != "Zero-Initial-Reentry" || gotSide != "BUY" {
		t.Fatalf("expected stale SL-Reentry to fall back to current bar alignment, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if gotPrice != 67840.0 {
		t.Fatalf("expected stale SL-Reentry fallback price 67840, got %.2f", gotPrice)
	}
	if gotEvent.IsZero() {
		t.Fatal("expected stale SL-Reentry fallback planned event")
	}
	if pending := mapValue(state[livePendingZeroInitialWindowStateKey]); stringValue(pending["side"]) != "BUY" {
		t.Fatalf("expected pending BUY window after stale SL-Reentry fallback, got %+v", pending)
	} else if !boolValue(pending["breakoutBacked"]) || stringValue(pending["openReason"]) != liveZeroInitialWindowOpenReasonBreakoutLocked {
		t.Fatalf("expected stale fallback window to carry breakout proof, got %+v", pending)
	}
	timeline := metadataList(state["timeline"])
	if len(timeline) != 1 || stringValue(timeline[0]["title"]) != "zero-initial-window-armed" {
		t.Fatalf("expected one zero-initial-window-armed event, got %+v", timeline)
	}
	context := mapValue(mapValue(timeline[0]["metadata"])["staleExitReentryContext"])
	if stringValue(context["alignmentMode"]) != "breakout-confirmed" {
		t.Fatalf("expected breakout-confirmed stale exit reentry context, got %+v", context)
	}
	if stringValue(context["plannedReason"]) != "SL-Reentry" || stringValue(context["breakoutPriceSource"]) != "trade_tick.price" {
		t.Fatalf("expected stale exit reentry context to retain original plan and breakout source, got %+v", context)
	}
	if parseFloatValue(context["staleAgeSeconds"]) <= parseFloatValue(context["staleWindowSeconds"]) {
		t.Fatalf("expected stale age to exceed stale window in context, got %+v", context)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationUsesSignalBarBoundaryForMillisBarStart(t *testing.T) {
	barStart := time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
	eventTime := barStart.Add(12*time.Minute + 14*time.Second)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      76561.0,
			"atr14":     230.0,
			"current": map[string]any{
				"barStart": strconv.FormatInt(barStart.UnixMilli(), 10),
				"close":    76440.0,
				"high":     76483.7,
				"low":      76440.0,
			},
			"prevBar1": map[string]any{
				"high": 76564.2,
				"low":  76470.2,
			},
			"prevBar2": map[string]any{
				"high": 76565.7,
				"low":  76444.6,
			},
		},
	}

	state, gotEvent, _, _, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		map[string]any{},
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"short_reentry_atr": 0.0,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		eventTime,
		76444.5,
		"trigger.price",
		barStart.Add(-2*time.Hour),
		76444.6,
		"SELL",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "Zero-Initial-Reentry" {
		t.Fatalf("expected breakout-backed zero window entry, got role=%s reason=%s", gotRole, gotReason)
	}
	if !gotEvent.Equal(barStart) {
		t.Fatalf("expected planned event to use signal bar boundary %s, got %s", barStart, gotEvent)
	}
	pending := mapValue(state[livePendingZeroInitialWindowStateKey])
	if got := stringValue(pending["signalBarStart"]); got != barStart.Format(time.RFC3339) {
		t.Fatalf("expected pending signal bar start at bar boundary, got %s", got)
	}
	if got := stringValue(pending["expiresAt"]); got != barStart.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("expected 30m zero window to expire after current+next bar, got %s", got)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationBlocksWeakTickBreakoutWithMinATRMargin(t *testing.T) {
	barStart := time.Date(2026, 4, 28, 16, 0, 0, 0, time.UTC)
	eventTime := barStart.Add(time.Minute + 40*time.Second + 77*time.Millisecond)
	stalePlanEvent := barStart.Add(-time.Hour)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      75939.52,
			"ma20":      76361.98,
			"atr14":     191.47857143,
			"current": map[string]any{
				"barStart": barStart.Format(time.RFC3339),
				"close":    76022.50,
				"high":     76022.50,
				"low":      76022.50,
			},
			"prevBar1": map[string]any{
				"high": 76022.50,
				"low":  75879.20,
			},
			"prevBar2": map[string]any{
				"high": 76025.80,
				"low":  75833.40,
			},
			"prevBar3": map[string]any{
				"high": 75926.60,
				"low":  75726.30,
			},
		},
	}

	state, gotEvent, gotPrice, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		map[string]any{},
		map[string]any{
			"dir2_zero_initial":       true,
			"zero_initial_mode":       "reentry_window",
			"long_reentry_atr":        0.1,
			"breakout_min_atr_margin": 0.02,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		eventTime,
		76026.60,
		"trigger.price",
		stalePlanEvent,
		76025.80,
		"BUY",
		"entry",
		"Initial",
	)
	if !gotEvent.Equal(stalePlanEvent) || gotPrice != 76025.80 || gotSide != "BUY" || gotRole != "entry" || gotReason != "Initial" {
		t.Fatalf("expected weak tick breakout to leave stale plan unchanged, got event=%s price=%.2f side=%s role=%s reason=%s", gotEvent, gotPrice, gotSide, gotRole, gotReason)
	}
	if pending := mapValue(state[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected weak tick breakout to avoid arming zero initial window, got %+v", pending)
	}
	if timeline := metadataList(state["timeline"]); len(timeline) != 0 {
		t.Fatalf("expected no zero-initial timeline event for filtered weak tick, got %+v", timeline)
	}
}

func TestRefreshLiveZeroInitialWindowStateExpiresLegacyEventTimeWindowAtBarBoundary(t *testing.T) {
	barStart := time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
	eventTime := barStart.Add(time.Hour + 7*time.Second)
	state := map[string]any{
		livePendingZeroInitialWindowStateKey: map[string]any{
			"side":            "SELL",
			"symbol":          "BTCUSDT",
			"signalTimeframe": "30m",
			"armedAt":         barStart.Add(12*time.Minute + 14*time.Second).Format(time.RFC3339),
			"signalBarStart":  barStart.Add(12*time.Minute + 14*time.Second).Format(time.RFC3339),
			"expiresAt":       barStart.Add(72*time.Minute + 14*time.Second).Format(time.RFC3339),
			"breakoutBacked":  true,
			"openReason":      liveZeroInitialWindowOpenReasonBreakoutLocked,
		},
	}
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"current": map[string]any{
				"barStart": strconv.FormatInt(barStart.Add(time.Hour).UnixMilli(), 10),
				"close":    76170.4,
				"high":     76206.3,
				"low":      76170.4,
			},
		},
	}

	updated := refreshLiveZeroInitialWindowState(state, signalStates, "BTCUSDT", "30m", map[string]any{}, eventTime)
	if pending := mapValue(updated[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected legacy event-time zero window to expire at signal bar boundary, got %+v", pending)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationClearsUnprovenZeroInitialWindow(t *testing.T) {
	barStart := time.Date(2026, 4, 22, 6, 0, 0, 0, time.UTC)
	state := map[string]any{
		livePendingZeroInitialWindowStateKey: map[string]any{
			"side":            "BUY",
			"symbol":          "BTCUSDT",
			"signalTimeframe": "30m",
			"armedAt":         barStart.Add(-time.Minute).Format(time.RFC3339),
			"signalBarStart":  barStart.Format(time.RFC3339),
			"expiresAt":       barStart.Add(30 * time.Minute).Format(time.RFC3339),
		},
	}
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      77800.0,
			"atr14":     416.0,
			"current": map[string]any{
				"barStart": barStart.Format(time.RFC3339),
				"close":    77966.3,
				"high":     77978.0,
				"low":      77930.0,
			},
			"prevBar1": map[string]any{
				"high": 78335.7,
				"low":  77928.6,
			},
			"prevBar2": map[string]any{
				"high": 78447.5,
				"low":  77406.0,
			},
		},
	}

	updated, _, _, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"long_reentry_atr":  0.1,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		barStart.Add(35*time.Second),
		77974.3,
		"trade_tick.price",
		barStart,
		77970.28,
		"BUY",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "Initial" || gotSide != "BUY" {
		t.Fatalf("expected unproven zero-initial window to fall back to original plan, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if pending := mapValue(updated[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected unproven zero-initial window to be cleared, got %+v", pending)
	}
	timeline := metadataList(updated["timeline"])
	if len(timeline) != 1 || stringValue(timeline[0]["title"]) != "zero-initial-window-consumed" {
		t.Fatalf("expected unproven window clear timeline event, got %+v", timeline)
	}
	if got := stringValue(mapValue(timeline[0]["metadata"])["reason"]); got != "zero-initial-window-missing-breakout-proof" {
		t.Fatalf("expected missing breakout proof clear reason, got %s", got)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationUsesZeroInitialSemanticsForStaleIntradayReentry(t *testing.T) {
	barStart := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      100.0,
			"atr14":     20.0,
			"current": map[string]any{
				"barStart": barStart.Format(time.RFC3339),
				"close":    108.0,
				"high":     109.0,
				"low":      104.0,
			},
			"prevBar1": map[string]any{
				"high": 104.0,
				"low":  95.0,
			},
			"prevBar2": map[string]any{
				"high": 106.0,
				"low":  96.0,
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
		"30m",
		map[string]any{},
		barStart.Add(45*time.Minute),
		101.0,
		"trade_tick.price",
		barStart.Add(-2*time.Hour),
		92.0,
		"SELL",
		"entry",
		"SL-Reentry",
	)
	if gotRole != "entry" || gotReason != "Initial" || gotSide != "BUY" {
		t.Fatalf("expected stale intraday SL-Reentry without breakout to reset to initial watch, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if gotPrice != 108.0 {
		t.Fatalf("expected stale intraday bootstrap price 108.0, got %.2f", gotPrice)
	}
	if gotEvent != barStart {
		t.Fatalf("expected stale intraday fallback planned event %s, got %s", barStart.Format(time.RFC3339), gotEvent.Format(time.RFC3339))
	}
	if pending := mapValue(state[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected stale intraday fallback to avoid arming zero-initial window without breakout, got %+v", pending)
	}
	timeline := metadataList(state["timeline"])
	if len(timeline) != 0 {
		t.Fatalf("expected no zero-initial timeline event without breakout-backed window, got %+v", timeline)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationKeepsPendingWindowLatentWhilePositionActive(t *testing.T) {
	barStart := time.Date(2026, 4, 22, 6, 0, 0, 0, time.UTC)
	state := map[string]any{
		livePendingZeroInitialWindowStateKey: map[string]any{
			"side":            "BUY",
			"symbol":          "BTCUSDT",
			"signalTimeframe": "30m",
			"armedAt":         barStart.Add(-time.Minute).Format(time.RFC3339),
			"signalBarStart":  barStart.Format(time.RFC3339),
			"expiresAt":       barStart.Add(30 * time.Minute).Format(time.RFC3339),
			"breakoutBacked":  true,
			"openReason":      liveZeroInitialWindowOpenReasonBreakoutLocked,
		},
	}
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      77800.0,
			"atr14":     416.0,
			"current": map[string]any{
				"barStart": barStart.Format(time.RFC3339),
				"close":    77966.3,
				"high":     77978.0,
				"low":      77930.0,
			},
			"prevBar1": map[string]any{
				"high": 78335.7,
				"low":  77928.6,
			},
			"prevBar2": map[string]any{
				"high": 78447.5,
				"low":  77406.0,
			},
		},
	}

	updated, gotEvent, gotPrice, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"long_reentry_atr":  0.1,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{
			"id":         "position-1",
			"symbol":     "BTCUSDT",
			"side":       "LONG",
			"quantity":   0.0128,
			"entryPrice": 77974.3,
			"found":      true,
		},
		barStart.Add(5*time.Minute),
		77974.3,
		"trade_tick.price",
		barStart.Add(5*time.Minute),
		77850.0,
		"SELL",
		"exit",
		"SL",
	)
	if gotRole != "exit" || gotReason != "SL" || gotSide != "SELL" {
		t.Fatalf("expected active position to keep exit plan, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if gotPrice != 77850.0 || gotEvent != barStart.Add(5*time.Minute) {
		t.Fatalf("expected exit plan step to stay untouched while position active, got event=%s price=%v", gotEvent.Format(time.RFC3339), gotPrice)
	}
	if pending := mapValue(updated[livePendingZeroInitialWindowStateKey]); stringValue(pending["side"]) != "BUY" {
		t.Fatalf("expected pending zero initial window to remain latent while position active, got %+v", pending)
	}
	if timeline := metadataList(updated["timeline"]); len(timeline) != 0 {
		t.Fatalf("expected no consume timeline event while position is still active, got %+v", timeline)
	}

	reactivated, _, _, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		updated,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"long_reentry_atr":  0.1,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		barStart.Add(6*time.Minute),
		77974.3,
		"trade_tick.price",
		barStart,
		77970.28,
		"BUY",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "Zero-Initial-Reentry" || gotSide != "BUY" {
		t.Fatalf("expected latent pending window to reactivate once flat, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if pending := mapValue(reactivated[livePendingZeroInitialWindowStateKey]); stringValue(pending["side"]) != "BUY" {
		t.Fatalf("expected pending zero initial window to remain available until a real reentry consumes it, got %+v", pending)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationUsesRecordedSLReentryWindow(t *testing.T) {
	slBarStart := time.Date(2026, 4, 28, 11, 30, 0, 0, time.UTC)
	currentBarStart := slBarStart.Add(30 * time.Minute)
	eventTime := currentBarStart.Add(7 * time.Second)
	state := map[string]any{
		"sessionReentryCount":         2.0,
		"lastSLExitFilledAt":          slBarStart.Add(12*time.Minute + 57*time.Second).Format(time.RFC3339),
		"lastSLExitOrderId":           "order-sl",
		"lastSLExitSignalBarStateKey": "BTCUSDT|30m|" + slBarStart.Format(time.RFC3339),
		"lastSLExitReentrySide":       "SELL",
	}
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      76344.76,
			"atr14":     231.3,
			"current": map[string]any{
				"barStart": strconv.FormatInt(currentBarStart.UnixMilli(), 10),
				"close":    76170.4,
				"high":     76206.3,
				"low":      76170.4,
			},
			"prevBar1": map[string]any{
				"high": 76483.7,
				"low":  76157.7,
			},
		},
	}

	updated, gotEvent, gotPrice, gotSide, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"short_reentry_atr": 0.0,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		eventTime,
		76170.4,
		"trigger.price",
		currentBarStart.Add(-2*time.Hour),
		76444.6,
		"SELL",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "SL-Reentry" || gotSide != "SELL" {
		t.Fatalf("expected recorded SL exit to arm SL-Reentry, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if !gotEvent.Equal(currentBarStart) {
		t.Fatalf("expected current signal bar event %s, got %s", currentBarStart, gotEvent)
	}
	if gotPrice != 76483.7 {
		t.Fatalf("expected short reentry price from prev high, got %v", gotPrice)
	}
	if pending := mapValue(updated[livePendingZeroInitialWindowStateKey]); len(pending) != 0 {
		t.Fatalf("expected no pending zero window for recorded SL reentry, got %+v", pending)
	}
	if got := stringValue(updated["lastSLExitReentryConsumedOrderId"]); got != "order-sl" {
		t.Fatalf("expected recorded SL reentry to consume order-sl, got %q", got)
	}
	if got := stringValue(updated["lastSLExitReentryConsumedReason"]); got != "consumed-on-derive" {
		t.Fatalf("expected consumed-on-derive reason, got %q", got)
	}
	if got := stringValue(updated["lastSLExitReentrySide"]); got != "" {
		t.Fatalf("expected derived SL reentry to clear armed side, got %q", got)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationConsumesRecordedSLReentryWindowOnce(t *testing.T) {
	slBarStart := time.Date(2026, 4, 28, 11, 30, 0, 0, time.UTC)
	currentBarStart := slBarStart.Add(30 * time.Minute)
	eventTime, state, signalStates := recordedSLReentryWindowFixture(slBarStart, currentBarStart)

	updated, _, _, _, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"short_reentry_atr": 0.0,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		eventTime,
		76170.4,
		"trigger.price",
		currentBarStart.Add(-2*time.Hour),
		76444.6,
		"SELL",
		"entry",
		"Initial",
	)
	if gotRole != "entry" || gotReason != "SL-Reentry" {
		t.Fatalf("expected first evaluation to derive SL-Reentry, got role=%s reason=%s", gotRole, gotReason)
	}
	if got := stringValue(updated["lastSLExitReentryConsumedOrderId"]); got != "order-sl" {
		t.Fatalf("expected consumed order id order-sl after first derive, got %q", got)
	}

	second, _, _, _, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		updated,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"short_reentry_atr": 0.0,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		eventTime.Add(time.Second),
		76170.4,
		"trigger.price",
		currentBarStart,
		76444.6,
		"SELL",
		"entry",
		"Initial",
	)
	if gotReason == "SL-Reentry" {
		t.Fatalf("expected consumed SL fill not to derive SL-Reentry again, got role=%s reason=%s", gotRole, gotReason)
	}
	if got := stringValue(second["lastSLExitReentryConsumedOrderId"]); got != "order-sl" {
		t.Fatalf("expected consumed order id to remain order-sl, got %q", got)
	}
	if got := stringValue(second["lastSLExitReentrySide"]); got != "" {
		t.Fatalf("expected consumed SL reentry side to stay cleared, got %q", got)
	}
}

func TestPrepareLivePlanStepForSignalEvaluationExpiresRecordedSLReentryWindowAfterNextBar(t *testing.T) {
	slBarStart := time.Date(2026, 4, 28, 11, 30, 0, 0, time.UTC)
	currentBarStart := slBarStart.Add(time.Hour)
	eventTime, state, signalStates := recordedSLReentryWindowFixture(slBarStart, currentBarStart)

	updated, _, _, _, gotRole, gotReason := prepareLivePlanStepForSignalEvaluation(
		state,
		map[string]any{
			"dir2_zero_initial": true,
			"zero_initial_mode": "reentry_window",
			"short_reentry_atr": 0.0,
		},
		signalStates,
		"BTCUSDT",
		"30m",
		map[string]any{},
		eventTime,
		76170.4,
		"trigger.price",
		currentBarStart,
		76444.6,
		"SELL",
		"entry",
		"Initial",
	)
	if gotReason == "SL-Reentry" {
		t.Fatalf("expected third signal bar not to derive SL-Reentry, got role=%s reason=%s", gotRole, gotReason)
	}
	if got := stringValue(updated["lastSLExitReentryConsumedOrderId"]); got != "order-sl" {
		t.Fatalf("expected expired SL reentry window to consume order-sl, got %q", got)
	}
	if got := stringValue(updated["lastSLExitReentryConsumedReason"]); got != "expired" {
		t.Fatalf("expected expired consume reason, got %q", got)
	}
	if got := stringValue(updated["lastSLExitReentrySide"]); got != "" {
		t.Fatalf("expected expired SL reentry side to be cleared, got %q", got)
	}
}

func recordedSLReentryWindowFixture(slBarStart time.Time, currentBarStart time.Time) (time.Time, map[string]any, map[string]any) {
	eventTime := currentBarStart.Add(7 * time.Second)
	state := map[string]any{
		"sessionReentryCount":         2.0,
		"lastSLExitFilledAt":          slBarStart.Add(12*time.Minute + 57*time.Second).Format(time.RFC3339),
		"lastSLExitOrderId":           "order-sl",
		"lastSLExitSignalBarStateKey": "BTCUSDT|30m|" + slBarStart.Format(time.RFC3339),
		"lastSLExitReentrySide":       "SELL",
	}
	signalStates := map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"}): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "30m",
			"sma5":      76344.76,
			"atr14":     231.3,
			"current": map[string]any{
				"barStart": strconv.FormatInt(currentBarStart.UnixMilli(), 10),
				"close":    76170.4,
				"high":     76206.3,
				"low":      76170.4,
			},
			"prevBar1": map[string]any{
				"high": 76483.7,
				"low":  76157.7,
			},
		},
	}
	return eventTime, state, signalStates
}
