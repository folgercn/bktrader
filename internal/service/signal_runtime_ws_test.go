package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestEnrichSignalRuntimeSummaryKeepsKlineEventsScopedByTimeframe(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	for _, timeframe := range []string{"5m", "1d", "4h"} {
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

	event5m := enrichSignalRuntimeSummary(session, map[string]any{
		"event":     "kline",
		"symbol":    "BTCUSDT",
		"timeframe": "5m",
		"barStart":  "1713074400000",
	})
	if got := stringValue(event5m["channel"]); got != "btcusdt@kline_5m" {
		t.Fatalf("expected 5m event to match 5m subscription, got %#v", event5m)
	}
	if got := stringValue(event5m["timeframe"]); got != "5m" {
		t.Fatalf("expected 5m event timeframe to remain 5m, got %#v", event5m)
	}

	sourceStates := map[string]any{}
	sourceStates = mergeSignalSourceState(sourceStates, event1d, time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC))
	sourceStates = mergeSignalSourceState(sourceStates, event4h, time.Date(2026, 4, 15, 0, 0, 1, 0, time.UTC))
	sourceStates = mergeSignalSourceState(sourceStates, event5m, time.Date(2026, 4, 15, 0, 0, 2, 0, time.UTC))
	if len(sourceStates) != 3 {
		t.Fatalf("expected distinct source-state entries per timeframe, got %#v", sourceStates)
	}

	for _, timeframe := range []string{"5m", "1d", "4h"} {
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

func TestDeriveSignalBarStatesUsesOpenCurrentBarWithClosedHistory(t *testing.T) {
	key := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "5m"})
	base := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	bars := make([]any, 0, 21)
	for i := 0; i < 20; i++ {
		bars = append(bars, map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "5m",
			"barStart":  base.Add(time.Duration(i) * 5 * time.Minute).Format(time.RFC3339),
			"open":      100 + float64(i),
			"high":      101 + float64(i),
			"low":       95 + float64(i),
			"close":     100 + float64(i),
			"volume":    1000 + float64(i),
			"isClosed":  true,
		})
	}
	currentStart := base.Add(20 * 5 * time.Minute).Format(time.RFC3339)
	bars = append(bars, map[string]any{
		"symbol":    "BTCUSDT",
		"timeframe": "5m",
		"barStart":  currentStart,
		"open":      119.0,
		"high":      130.0,
		"low":       118.0,
		"close":     125.0,
		"volume":    1500.0,
		"isClosed":  false,
	})

	states := deriveSignalBarStates(map[string]any{
		key: map[string]any{
			"sourceKey":  "binance-kline",
			"role":       "signal",
			"streamType": "signal_bar",
			"symbol":     "BTCUSDT",
			"timeframe":  "5m",
			"bars":       bars,
		},
	})
	state := mapValue(states[key])
	if state == nil {
		t.Fatalf("expected signal state from open current bar, got %#v", states)
	}
	if boolValue(state["currentClosed"]) {
		t.Fatalf("expected current bar to remain marked open, got %#v", state)
	}
	current := mapValue(state["current"])
	if stringValue(current["barStart"]) != currentStart {
		t.Fatalf("expected open bar to be current, got %#v", current)
	}
	prevBar1 := mapValue(state["prevBar1"])
	prevBar2 := mapValue(state["prevBar2"])
	if stringValue(prevBar1["barStart"]) != base.Add(19*5*time.Minute).Format(time.RFC3339) {
		t.Fatalf("expected prevBar1 to be latest closed bar, got %#v", prevBar1)
	}
	if stringValue(prevBar2["barStart"]) != base.Add(18*5*time.Minute).Format(time.RFC3339) {
		t.Fatalf("expected prevBar2 to be second latest closed bar, got %#v", prevBar2)
	}
	gate := evaluateSignalBarGate(state, "BUY", "entry")
	if !boolValue(gate["ready"]) || !boolValue(gate["longReady"]) {
		t.Fatalf("expected open current bar breakout to be actionable, got %#v", gate)
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

func TestHandleSignalRuntimeMessageScopesTriggerByLiveSessionSymbol(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	for _, symbol := range []string{"BTCUSDT", "ETHUSDT"} {
		if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
			"sourceKey": "binance-trade-tick",
			"role":      "trigger",
			"symbol":    symbol,
		}); err != nil {
			t.Fatalf("bind %s trigger failed: %v", symbol, err)
		}
	}

	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
	})
	if err != nil {
		t.Fatalf("create BTC live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}
	eventTime := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	if err := platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["sourceStates"] = map[string]any{}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	}); err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.handleSignalRuntimeMessage(runtimeSessionID, map[string]any{
		"role":               "signal",
		"streamType":         "signal_bar",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"timeframe":          "1d",
		"event":              "kline",
	}, eventTime); err != nil {
		t.Fatalf("handle BTC signal bar failed: %v", err)
	}
	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session after signal bar failed: %v", err)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got != "" {
		t.Fatalf("expected signal bar to skip live evaluation, got last event at %s", got)
	}

	if err := platform.handleSignalRuntimeMessage(runtimeSessionID, map[string]any{
		"role":               "trigger",
		"streamType":         "trade_tick",
		"symbol":             "ETHUSDT",
		"subscriptionSymbol": "ETHUSDT",
		"event":              "trade",
		"price":              "3000",
	}, eventTime); err != nil {
		t.Fatalf("handle ETH trigger failed: %v", err)
	}
	updated, err = platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got != "" {
		t.Fatalf("expected ETH trigger to skip BTC session, got last event at %s", got)
	}
	if timeline := metadataList(updated.State["timeline"]); len(timeline) != 0 {
		t.Fatalf("expected ETH trigger to append no BTC timeline entries, got %#v", timeline)
	}

	if err := platform.handleSignalRuntimeMessage(runtimeSessionID, map[string]any{
		"role":               "trigger",
		"streamType":         "trade_tick",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"event":              "trade",
		"price":              "69000",
	}, eventTime.Add(time.Second)); err != nil {
		t.Fatalf("handle BTC trigger failed: %v", err)
	}
	updated, err = platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session after BTC trigger failed: %v", err)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got == "" {
		t.Fatalf("expected BTC trigger to reach BTC session, got empty last event state")
	}
}
