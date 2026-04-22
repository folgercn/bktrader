package service

import (
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
