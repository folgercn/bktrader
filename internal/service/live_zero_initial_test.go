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
	if gotRole != "entry" || gotReason != "Zero-Initial-Reentry" || gotSide != "BUY" {
		t.Fatalf("expected stale intraday SL-Reentry to fall back to zero-initial semantics, got side=%s role=%s reason=%s", gotSide, gotRole, gotReason)
	}
	if gotPrice != 97.0 {
		t.Fatalf("expected stale intraday fallback price 97.0, got %.2f", gotPrice)
	}
	if gotEvent != barStart {
		t.Fatalf("expected stale intraday fallback planned event %s, got %s", barStart.Format(time.RFC3339), gotEvent.Format(time.RFC3339))
	}
	if pending := mapValue(state[livePendingZeroInitialWindowStateKey]); stringValue(pending["side"]) != "BUY" {
		t.Fatalf("expected pending BUY window after stale intraday fallback, got %+v", pending)
	}
}
