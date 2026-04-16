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

func TestMergeSignalSourceStatePreservesSignalBarHistory(t *testing.T) {
	sourceStates := map[string]any{}
	first := map[string]any{
		"sourceKey":          "binance-kline",
		"role":               "signal",
		"streamType":         "signal_bar",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"timeframe":          "1d",
		"barStart":           "1712966400000",
		"barEnd":             "1713052800000",
		"open":               "68000",
		"high":               "69000",
		"low":                "67500",
		"close":              "68800",
		"volume":             "1200",
		"isClosed":           true,
	}
	second := map[string]any{
		"sourceKey":          "binance-kline",
		"role":               "signal",
		"streamType":         "signal_bar",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"timeframe":          "1d",
		"barStart":           "1713052800000",
		"barEnd":             "1713139200000",
		"open":               "68800",
		"high":               "69500",
		"low":                "68200",
		"close":              "69200",
		"volume":             "1300",
		"isClosed":           true,
	}

	sourceStates = mergeSignalSourceState(sourceStates, first, time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC))
	sourceStates = mergeSignalSourceState(sourceStates, second, time.Date(2026, 4, 15, 0, 0, 1, 0, time.UTC))

	key := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "1d"})
	entry := mapValue(sourceStates[key])
	if entry == nil {
		t.Fatalf("expected source state entry, got %#v", sourceStates)
	}
	bars := normalizeSignalBarEntries(entry["bars"])
	if len(bars) != 2 {
		t.Fatalf("expected two retained bars, got %#v", entry["bars"])
	}
	if got := stringValue(bars[0]["barStart"]); got != "1712966400000" {
		t.Fatalf("expected first bar to be retained, got %#v", bars)
	}
	if got := stringValue(bars[1]["barStart"]); got != "1713052800000" {
		t.Fatalf("expected second bar to be appended, got %#v", bars)
	}
}

func TestBootstrapSignalRuntimeSourceStatesUsesWarmMarketCache(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	signalBars := make([]strategySignalBar, 0, 4)
	base := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		signalBars = append(signalBars, strategySignalBar{
			Time:   base.Add(time.Duration(i) * 24 * time.Hour),
			Open:   68000 + float64(i)*100,
			High:   68500 + float64(i)*100,
			Low:    67500 + float64(i)*100,
			Close:  68200 + float64(i)*100,
			Volume: 1000 + float64(i)*10,
			MA5:    68050 + float64(i)*100,
			MA20:   67000 + float64(i)*100,
			ATR:    900 + float64(i)*10,
		})
	}
	platform.liveMarketData["BTCUSDT"] = liveMarketSnapshot{
		Symbol: "BTCUSDT",
		SignalBars: map[string][]strategySignalBar{
			"1d": signalBars,
		},
		UpdatedAt: time.Now().UTC(),
	}

	subscriptions := []map[string]any{{
		"sourceKey":  "binance-kline",
		"role":       "signal",
		"streamType": "signal_bar",
		"symbol":     "BTCUSDT",
		"options":    map[string]any{"timeframe": "1d"},
	}}

	sourceStates := platform.bootstrapSignalRuntimeSourceStates(subscriptions)
	key := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "1d"})
	entry := mapValue(sourceStates[key])
	if entry == nil {
		t.Fatalf("expected bootstrap source state, got %#v", sourceStates)
	}
	if got := len(normalizeSignalBarEntries(entry["bars"])); got != 4 {
		t.Fatalf("expected warm cache bars to be copied, got %#v", entry["bars"])
	}
	if got := stringValue(entry["lastEventAt"]); got != "" {
		t.Fatalf("expected bootstrap state to wait for live freshness, got %#v", entry)
	}
	signalBarStates := deriveSignalBarStates(sourceStates)
	signalState := mapValue(signalBarStates[key])
	if signalState == nil {
		t.Fatalf("expected derived signal bar state, got %#v", signalBarStates)
	}
	if mapValue(signalState["prevBar2"]) == nil {
		t.Fatalf("expected bootstrap state to include previous bars, got %#v", signalState)
	}
}
