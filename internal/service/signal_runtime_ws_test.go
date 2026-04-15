package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestEnrichSignalRuntimeSummaryKeepsKlineEventsScopedByTimeframe(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	for _, timeframe := range []string{"1d", "4h"} {
		if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "BTCUSDT",
			"options":   map[string]any{"timeframe": timeframe},
		}); err != nil {
			t.Fatalf("bind strategy %s failed: %v", timeframe, err)
		}
	}

	session, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}

	event4h := enrichSignalRuntimeSummary(session, map[string]any{
		"event":     "kline",
		"symbol":    "BTCUSDT",
		"timeframe": "4h",
		"barStart":  "1713067200000",
	})
	if got := stringValue(event4h["channel"]); got != "btcusdt@kline_4h" {
		t.Fatalf("expected 4h event to match 4h subscription, got %#v", event4h)
	}
	if got := stringValue(event4h["timeframe"]); got != "4h" {
		t.Fatalf("expected 4h event timeframe to remain 4h, got %#v", event4h)
	}

	event1d := enrichSignalRuntimeSummary(session, map[string]any{
		"event":     "kline",
		"symbol":    "BTCUSDT",
		"timeframe": "1d",
		"barStart":  "1713052800000",
	})
	if got := stringValue(event1d["channel"]); got != "btcusdt@kline_1d" {
		t.Fatalf("expected 1d event to match 1d subscription, got %#v", event1d)
	}

	sourceStates := map[string]any{}
	sourceStates = mergeSignalSourceState(sourceStates, event1d, time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC))
	sourceStates = mergeSignalSourceState(sourceStates, event4h, time.Date(2026, 4, 15, 0, 0, 1, 0, time.UTC))
	if len(sourceStates) != 2 {
		t.Fatalf("expected distinct source-state entries per timeframe, got %#v", sourceStates)
	}

	for _, timeframe := range []string{"1d", "4h"} {
		key := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": timeframe})
		entry := mapValue(sourceStates[key])
		if entry == nil {
			t.Fatalf("expected source state for %s timeframe, got %#v", timeframe, sourceStates)
		}
		if got := stringValue(entry["timeframe"]); got != timeframe {
			t.Fatalf("expected source state timeframe %s, got %#v", timeframe, entry)
		}
	}
}
