package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestClassifyDisconnectSeverityTreatsContextErrorsAsTransient(t *testing.T) {
	for _, err := range []error{
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("read failed: context canceled"),
		errors.New("dial failed: context deadline exceeded"),
	} {
		if got := classifyDisconnectSeverity(err); got != disconnectTransient {
			t.Fatalf("expected %q to be transient, got %s", err, got.String())
		}
	}
}

func TestRunSignalRuntimeWithRecoveryResetsAttemptsAfterRecoveredRun(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	sessionID := "runtime-recovery"
	now := time.Now().UTC()
	platform.signalSessions[sessionID] = domain.SignalRuntimeSession{
		ID:        sessionID,
		Status:    "RUNNING",
		State:     map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	outcomes := []struct {
		connected bool
		err       error
	}{
		{connected: true, err: errors.New("read tcp: EOF")},
		{connected: true, err: errors.New("read tcp: EOF")},
		{connected: false, err: errors.New("dial failed: 403 forbidden")},
	}
	callCount := 0
	waits := make([]time.Duration, 0, 2)

	platform.runSignalRuntimeWithRecoveryUsing(
		context.Background(),
		sessionID,
		func(context.Context, string) (bool, error) {
			if callCount >= len(outcomes) {
				t.Fatalf("unexpected extra recovery loop call: %d", callCount)
			}
			outcome := outcomes[callCount]
			callCount++
			return outcome.connected, outcome.err
		},
		func(_ context.Context, backoff time.Duration) bool {
			waits = append(waits, backoff)
			return true
		},
	)

	session, err := platform.GetSignalRuntimeSession(sessionID)
	if err != nil {
		t.Fatalf("get runtime session failed: %v", err)
	}
	if session.Status != "ERROR" {
		t.Fatalf("expected terminal ERROR after fatal retry, got %s", session.Status)
	}
	if callCount != len(outcomes) {
		t.Fatalf("expected %d recovery loop calls, got %d", len(outcomes), callCount)
	}
	if len(waits) != 2 {
		t.Fatalf("expected two recovery waits across two disconnect cycles, got %#v", waits)
	}
	if waits[0] != 5*time.Second || waits[1] != 5*time.Second {
		t.Fatalf("expected retry budget to reset after recovered run, got wait sequence %#v", waits)
	}
}

func TestRecoveryBackoffUsesFasterReconnectBudget(t *testing.T) {
	if got := recoveryBackoff(transientReconnectPolicy, 1); got != 5*time.Second {
		t.Fatalf("expected first transient backoff to be 5s, got %s", got)
	}
	if got := recoveryBackoff(transientReconnectPolicy, 2); got != 15*time.Second {
		t.Fatalf("expected second transient backoff to be 15s, got %s", got)
	}
	if got := recoveryBackoff(transientReconnectPolicy, 3); got != 30*time.Second {
		t.Fatalf("expected third transient backoff to be 30s, got %s", got)
	}
	if got := recoveryBackoff(kickedReconnectPolicy, 1); got != 20*time.Second {
		t.Fatalf("expected first kicked backoff to be 20s, got %s", got)
	}
	if got := recoveryBackoff(kickedReconnectPolicy, 2); got != 60*time.Second {
		t.Fatalf("expected second kicked backoff to be 60s, got %s", got)
	}
}

func TestSignalRuntimeWSProfileForAttemptExpandsReadTimeout(t *testing.T) {
	if got := signalRuntimeWSProfileForAttempt(0).readTimeout; got != 20*time.Second {
		t.Fatalf("expected initial read timeout to be 20s, got %s", got)
	}
	if got := signalRuntimeWSProfileForAttempt(1).readTimeout; got != 30*time.Second {
		t.Fatalf("expected first reconnect read timeout to be 30s, got %s", got)
	}
	if got := signalRuntimeWSProfileForAttempt(2).readTimeout; got != 45*time.Second {
		t.Fatalf("expected later reconnect read timeout to be 45s, got %s", got)
	}
}

func TestRunExchangeWebsocketLoopRecordsReconnectObservability(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Now().UTC()
	session := domain.SignalRuntimeSession{
		ID:             "runtime-reconnect-observe",
		Status:         "RUNNING",
		RuntimeAdapter: "binance-market-ws",
		State: map[string]any{
			"subscriptions": []map[string]any{{
				"sourceKey":  "binance-trade-tick",
				"role":       "trigger",
				"symbol":     "BTCUSDT",
				"channel":    "btcusdt@trade",
				"adapterKey": "binance-market-ws",
			}},
			"reconnectAttempt":            2,
			"reconnectAttemptStartedAtMs": now.Add(-1500 * time.Millisecond).UnixMilli(),
			"lastDisconnectAt":            now.Add(-2 * time.Second).Format(time.RFC3339),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	platform.signalSessions[session.ID] = session

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Errorf("read subscribe payload failed: %v", err)
			return
		}
		time.Sleep(20 * time.Millisecond)
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "test done"), time.Now().Add(time.Second))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	connected, err := platform.runExchangeWebsocketLoop(context.Background(), session, wsURL, func([]map[string]any) (map[string]any, error) {
		return map[string]any{
			"method": "SUBSCRIBE",
			"params": []string{"btcusdt@trade"},
			"id":     1,
		}, nil
	})
	if !connected {
		t.Fatal("expected websocket loop to report connected before disconnect")
	}
	if err == nil {
		t.Fatal("expected websocket loop to end with disconnect error")
	}

	stored, getErr := platform.GetSignalRuntimeSession(session.ID)
	if getErr != nil {
		t.Fatalf("get runtime session failed: %v", getErr)
	}
	if got := stringValue(stored.State["lastReconnectDuration"]); got == "" {
		t.Fatalf("expected reconnect duration to be recorded, got %#v", stored.State)
	}
	reconnectMs, ok := stored.State["lastReconnectDurationMs"].(int64)
	if !ok || reconnectMs <= 0 {
		t.Fatalf("expected reconnect duration ms to be positive, got %#v", stored.State["lastReconnectDurationMs"])
	}
	subs := metadataList(stored.State["lastReconnectSubscriptions"])
	if len(subs) != 1 {
		t.Fatalf("expected reconnect subscription summary, got %#v", stored.State["lastReconnectSubscriptions"])
	}
	if got := stringValue(stored.State["lastReconnectResult"]); got != "subscribe_request_sent" {
		t.Fatalf("expected reconnect result to record subscribe request only, got %#v", got)
	}
	if stringValue(stored.State["reconnectAttemptStartedAtMs"]) != "" {
		t.Fatalf("expected reconnect attempt marker to be cleared, got %#v", stored.State)
	}
}

func TestFilterStatesBySymbolKeepsBlankEntriesForBackwardCompatibility(t *testing.T) {
	sourceStates := map[string]any{
		"btc":    map[string]any{"symbol": "BTCUSDT"},
		"legacy": map[string]any{"lastPrice": 68000.0},
		"eth":    map[string]any{"symbol": "ETHUSDT"},
	}
	filteredSourceStates := filterSourceStatesBySymbol(sourceStates, "BTCUSDT")
	if len(filteredSourceStates) != 2 {
		t.Fatalf("expected BTC + legacy source states, got %#v", filteredSourceStates)
	}
	if _, ok := filteredSourceStates["btc"]; !ok {
		t.Fatalf("expected BTC source state to remain, got %#v", filteredSourceStates)
	}
	if _, ok := filteredSourceStates["legacy"]; !ok {
		t.Fatalf("expected blank-symbol source state to remain for compatibility, got %#v", filteredSourceStates)
	}
	if _, ok := filteredSourceStates["eth"]; ok {
		t.Fatalf("expected ETH source state to be filtered out, got %#v", filteredSourceStates)
	}

	signalBarStates := map[string]any{
		"btc":    map[string]any{"symbol": "BTCUSDT"},
		"legacy": map[string]any{"current": map[string]any{"close": 68000.0}},
		"eth":    map[string]any{"symbol": "ETHUSDT"},
	}
	filteredSignalBarStates := filterSignalBarStatesBySymbol(signalBarStates, "BTCUSDT")
	if len(filteredSignalBarStates) != 2 {
		t.Fatalf("expected BTC + legacy signal bar states, got %#v", filteredSignalBarStates)
	}
	if _, ok := filteredSignalBarStates["btc"]; !ok {
		t.Fatalf("expected BTC signal bar state to remain, got %#v", filteredSignalBarStates)
	}
	if _, ok := filteredSignalBarStates["legacy"]; !ok {
		t.Fatalf("expected blank-symbol signal bar state to remain for compatibility, got %#v", filteredSignalBarStates)
	}
	if _, ok := filteredSignalBarStates["eth"]; ok {
		t.Fatalf("expected ETH signal bar state to be filtered out, got %#v", filteredSignalBarStates)
	}
}

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

func TestEnrichSignalRuntimeSummaryInfersOKXTradesChannelAsTrigger(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "okx-order-book",
		"role":      "feature",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind okx order book failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "okx-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind okx trigger failed: %v", err)
	}

	session, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}

	summary := enrichSignalRuntimeSummary(session, map[string]any{
		"event":   "message",
		"channel": "trades",
		"symbol":  "BTCUSDT",
		"price":   "68000",
	})
	if got := stringValue(summary["streamType"]); got != "trade_tick" {
		t.Fatalf("expected okx trades message to infer trade_tick, got %#v", summary)
	}
	if got := stringValue(summary["role"]); got != "trigger" {
		t.Fatalf("expected okx trades message to attach trigger role, got %#v", summary)
	}
	if !signalRuntimeSummaryShouldTriggerLiveEvaluation(summary) {
		t.Fatalf("expected okx trades message to remain trigger-actionable, got %#v", summary)
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

func TestMergeSignalSourceStateCanonicalizesScientificBarTimestamps(t *testing.T) {
	sourceStates := map[string]any{}
	first := map[string]any{
		"sourceKey":          "binance-kline",
		"role":               "signal",
		"streamType":         "signal_bar",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"timeframe":          "30m",
		"barStart":           "1776826800000",
		"barEnd":             "1776828600000",
		"open":               "77300",
		"high":               "77400",
		"low":                "77200",
		"close":              "77350",
		"volume":             "100",
		"isClosed":           true,
	}
	second := cloneMetadata(first)
	second["close"] = "77380"
	second["barStart"] = "1.7768268e+12"
	second["barEnd"] = "1.7768286e+12"

	sourceStates = mergeSignalSourceState(sourceStates, first, time.Date(2026, 4, 22, 3, 0, 0, 0, time.UTC))
	sourceStates = mergeSignalSourceState(sourceStates, second, time.Date(2026, 4, 22, 3, 0, 5, 0, time.UTC))

	key := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"})
	entry := mapValue(sourceStates[key])
	if entry == nil {
		t.Fatalf("expected source state entry, got %#v", sourceStates)
	}
	bars := normalizeSignalBarEntries(entry["bars"])
	if len(bars) != 1 {
		t.Fatalf("expected canonicalized duplicate bar to collapse into one entry, got %#v", bars)
	}
	if got := stringValue(bars[0]["barStart"]); got != "1776826800000" {
		t.Fatalf("expected canonical barStart 1776826800000, got %#v", bars[0])
	}
	if got := stringValue(bars[0]["barEnd"]); got != "1776828600000" {
		t.Fatalf("expected canonical barEnd 1776828600000, got %#v", bars[0])
	}
	if got := stringValue(bars[0]["close"]); got != "77380" {
		t.Fatalf("expected later duplicate bar to win after normalization, got %#v", bars[0])
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
	gate := evaluateSignalBarGate(state, "BUY", "entry", "", parseFloatValue(current["high"]), "signal-bar.high")
	if boolValue(gate["ready"]) || boolValue(gate["longReady"]) {
		t.Fatalf("expected open current bar to stay blocked without prev t-2 > prev t-1 breakout shape, got %#v", gate)
	}
	if boolValue(gate["longBreakoutShapeReady"]) {
		t.Fatalf("expected breakout shape to remain false for this monotonic sample, got %#v", gate)
	}
}

func TestDeriveSignalBarStatesDedupesCanonicalBarHistory(t *testing.T) {
	key := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{"timeframe": "30m"})
	states := deriveSignalBarStates(map[string]any{
		key: map[string]any{
			"sourceKey":  "binance-kline",
			"role":       "signal",
			"streamType": "signal_bar",
			"symbol":     "BTCUSDT",
			"timeframe":  "30m",
			"bars": []any{
				map[string]any{
					"symbol":    "BTCUSDT",
					"timeframe": "30m",
					"barStart":  "1776825000000",
					"close":     77250.0,
					"high":      77320.0,
					"low":       77180.0,
					"isClosed":  true,
				},
				map[string]any{
					"symbol":    "BTCUSDT",
					"timeframe": "30m",
					"barStart":  "1776826800000",
					"close":     77350.0,
					"high":      77420.0,
					"low":       77290.0,
					"isClosed":  true,
				},
				map[string]any{
					"symbol":    "BTCUSDT",
					"timeframe": "30m",
					"barStart":  "1.7768268e+12",
					"close":     77360.0,
					"high":      77430.0,
					"low":       77295.0,
					"isClosed":  true,
				},
				map[string]any{
					"symbol":    "BTCUSDT",
					"timeframe": "30m",
					"barStart":  "1776828600000",
					"close":     77410.0,
					"high":      77480.0,
					"low":       77340.0,
					"isClosed":  false,
				},
			},
		},
	})
	state := mapValue(states[key])
	if state == nil {
		t.Fatalf("expected signal bar state, got %#v", states)
	}
	if got := stringValue(mapValue(state["prevBar1"])["barStart"]); got != "1776826800000" {
		t.Fatalf("expected prevBar1 to use the deduped latest closed bar, got %#v", state["prevBar1"])
	}
	if got := stringValue(mapValue(state["prevBar2"])["barStart"]); got != "1776825000000" {
		t.Fatalf("expected prevBar2 to use the prior distinct closed bar, got %#v", state["prevBar2"])
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
	for _, indicatorKey := range []string{"sma5", "ma20", "atr14"} {
		if _, exists := signalState[indicatorKey]; exists {
			t.Fatalf("expected insufficient warm bars to omit %s, got %#v", indicatorKey, signalState[indicatorKey])
		}
	}
	if _, err := json.Marshal(signalBarStates); err != nil {
		t.Fatalf("expected derived signal bar states to remain JSON encodable, got %v", err)
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

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
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

func TestHandleSignalRuntimeMessageRepairsMissingSignalRuntimeRequired(t *testing.T) {
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
			t.Fatalf("bind strategy source failed: %v", err)
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
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeRequired"] = nil
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("corrupt live session state failed: %v", err)
	}

	eventTime := time.Date(2026, 4, 20, 10, 23, 0, 0, time.UTC)
	if err := platform.handleSignalRuntimeMessage(runtimeSessionID, map[string]any{
		"role":               "trigger",
		"streamType":         "trade_tick",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"event":              "trade",
		"price":              "75229.70",
	}, eventTime); err != nil {
		t.Fatalf("handle trigger after missing signalRuntimeRequired failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if !boolValue(updated.State["signalRuntimeRequired"]) {
		t.Fatalf("expected fanout to restore signalRuntimeRequired, got %#v", updated.State)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got == "" {
		t.Fatalf("expected repaired session to consume runtime trigger, got %#v", updated.State)
	}
}

func TestHandleSignalRuntimeMessageRecordsRuntimeNotRequiredDrop(t *testing.T) {
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
			t.Fatalf("bind strategy source failed: %v", err)
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
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeRequired"] = false
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	eventTime := time.Date(2026, 4, 20, 10, 23, 0, 0, time.UTC)
	if err := platform.handleSignalRuntimeMessage(runtimeSessionID, map[string]any{
		"role":               "trigger",
		"streamType":         "trade_tick",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"event":              "trade",
		"price":              "75229.70",
	}, eventTime); err != nil {
		t.Fatalf("handle trigger with runtime disabled failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got != "" {
		t.Fatalf("expected runtime-not-required session to skip fanout, got lastSignalRuntimeEventAt=%s", got)
	}
	if got := stringValue(updated.State["lastRuntimeFanoutDropReason"]); got != "runtime-not-required" {
		t.Fatalf("expected runtime-not-required drop breadcrumb, got %s", got)
	}
	if got := stringValue(updated.State["lastRuntimeFanoutDropAt"]); got == "" {
		t.Fatalf("expected runtime fanout drop timestamp, got %#v", updated.State)
	}
}

func runTestSignalRuntimeReconnect(platform *Platform, runtimeSessionID string) {
	outcomes := []struct {
		connected bool
		err       error
	}{
		{connected: true, err: errors.New("read tcp: EOF")},
		{connected: true, err: errors.New("read tcp: EOF")},
		{connected: false, err: errors.New("dial failed: 403 forbidden")},
	}
	callCount := 0
	platform.runSignalRuntimeWithRecoveryUsing(
		context.Background(),
		runtimeSessionID,
		func(context.Context, string) (bool, error) {
			if callCount >= len(outcomes) {
				return false, errors.New("unexpected extra reconnect loop")
			}
			outcome := outcomes[callCount]
			callCount++
			return outcome.connected, outcome.err
		},
		func(_ context.Context, _ time.Duration) bool {
			return true
		},
	)
}

func TestRunSignalRuntimeWithRecoveryReconnectTriggersAuthoritativeRESTSync(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-ws-reconnect-reconcile", []map[string]any{
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

	runtimeSession, err := platform.CreateSignalRuntimeSession(session.AccountID, session.StrategyID)
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	runTestSignalRuntimeReconnect(platform, runtimeSession.ID)

	account, err := platform.store.GetAccount(session.AccountID)
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	if got := stringValue(account.Metadata["lastLivePositionSyncAt"]); got == "" {
		t.Fatal("expected websocket reconnect to trigger a REST position sync")
	}
	if liveAccountPositionReconcilePending(account) {
		t.Fatalf("expected reconnect-triggered reconcile to clear pending gate, account=%#v", account.Metadata)
	}
	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusVerified {
		t.Fatalf("expected verified gate after healthy reconnect reconcile, got %s", got)
	}
	if boolValue(updated.State["positionReconcileGateBlocking"]) {
		t.Fatal("expected healthy reconnect reconcile not to block live session actions")
	}
}

func TestRunSignalRuntimeWithRecoveryReconnectMarksStaleSyncOnRESTMismatch(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	configureTestLiveRESTReconcileAdapter(t, platform, "test-ws-reconnect-stale", []map[string]any{})

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

	runtimeSession, err := platform.CreateSignalRuntimeSession(session.AccountID, session.StrategyID)
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	runTestSignalRuntimeReconnect(platform, runtimeSession.ID)

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["positionReconcileGateStatus"]); got != livePositionReconcileGateStatusStale {
		t.Fatalf("expected stale reconcile gate after reconnect mismatch, got %s", got)
	}
	if got := stringValue(updated.State["recoveryTakeoverState"]); got != liveRecoveryTakeoverStateStaleSync {
		t.Fatalf("expected stale-sync takeover state after reconnect mismatch, got %s", got)
	}
	if !boolValue(updated.State["positionReconcileGateBlocking"]) {
		t.Fatal("expected reconnect mismatch to block live session actions")
	}
	if _, err := platform.ClosePosition(position.ID); err == nil {
		t.Fatal("expected stale reconnect mismatch to block close execution")
	}
}
