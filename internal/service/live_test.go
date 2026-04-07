package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestDeriveLiveSessionIntentUsesNextPlannedStep(t *testing.T) {
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

	intent := deriveLiveSessionIntent(decision, "BTCUSDT")
	if intent == nil {
		t.Fatal("expected intent")
	}
	if got := intent["action"]; got != "exit" {
		t.Fatalf("expected exit action, got %v", got)
	}
	if got := intent["side"]; got != "SELL" {
		t.Fatalf("expected SELL side, got %v", got)
	}
	if got := intent["reason"]; got != "PT" {
		t.Fatalf("expected PT reason, got %v", got)
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

func TestPersistRuntimeStrategySignalStoresLatestSnapshot(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 7, 5, 0, 0, 0, time.UTC)
	platform.signalSessions["runtime-1"] = domain.SignalRuntimeSession{
		ID:         "runtime-1",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "RUNNING",
		State: map[string]any{
			"strategySignalCount":    0,
			"strategySignalsByScope": map[string]any{},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := platform.persistRuntimeStrategySignal("runtime-1", now, buildRuntimeStrategySignalSnapshot(
		"live",
		"live-session-main",
		"intent-ready",
		now,
		StrategySignalDecision{Action: "advance-plan", Reason: "trigger-source-ready"},
		map[string]any{"side": "BUY", "symbol": "BTCUSDT"},
		nil,
		StrategyExecutionContext{StrategyVersionID: "strategy-version-bk-1d-v010", Symbol: "BTCUSDT", SignalTimeframe: "1d", ExecutionDataSource: "tick"},
		time.Time{},
		0,
		"BUY",
		"entry",
		"Initial",
		map[string]any{"ready": true},
	))
	if err != nil {
		t.Fatalf("persistRuntimeStrategySignal returned error: %v", err)
	}

	session, err := platform.GetSignalRuntimeSession("runtime-1")
	if err != nil {
		t.Fatalf("GetSignalRuntimeSession returned error: %v", err)
	}
	if got := maxIntValue(session.State["strategySignalCount"], 0); got != 1 {
		t.Fatalf("expected strategySignalCount=1, got %d", got)
	}
	last := mapValue(session.State["lastStrategySignal"])
	if got := stringValue(last["status"]); got != "intent-ready" {
		t.Fatalf("expected last strategy signal status intent-ready, got %q", got)
	}
	if got := stringValue(mapValue(last["decision"])["action"]); got != "advance-plan" {
		t.Fatalf("expected decision action advance-plan, got %q", got)
	}
	byScope := mapValue(session.State["strategySignalsByScope"])
	if _, ok := byScope["live|live-session-main"]; !ok {
		t.Fatalf("expected strategy signal snapshot to be indexed by scope")
	}
}

func TestDispatchLiveSessionIntentCreatesLiveOrderFromRuntimeIntent(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	_, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    "binance-futures",
		"executionMode": "mock",
		"credentialRefs": map[string]any{
			"apiKeyRef":    "env:BINANCE_KEY",
			"apiSecretRef": "env:BINANCE_SECRET",
		},
	})
	if err != nil {
		t.Fatalf("BindLiveAccount returned error: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("BindStrategySignalSource returned error: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("BindAccountSignalSource returned error: %v", err)
	}

	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"dispatchMode": "auto-dispatch",
	})
	if err != nil {
		t.Fatalf("CreateLiveSession returned error: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session")
	}
	runtimeSession, err := platform.GetSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		t.Fatalf("GetSignalRuntimeSession returned error: %v", err)
	}
	runtimeSession.Status = "RUNNING"
	runtimeSession.State = cloneMetadata(runtimeSession.State)
	runtimeSession.State["health"] = "healthy"
	runtimeSession.State["sourceStates"] = map[string]any{
		"binance-trade-tick|trigger|BTCUSDT": map[string]any{
			"sourceKey":   "binance-trade-tick",
			"role":        "trigger",
			"streamType":  "trade_tick",
			"symbol":      "BTCUSDT",
			"lastEventAt": time.Now().UTC().Format(time.RFC3339),
		},
	}
	platform.signalSessions[runtimeSessionID] = runtimeSession

	state := cloneMetadata(session.State)
	state["signalRuntimeSessionId"] = runtimeSessionID
	state["lastStrategyIntent"] = map[string]any{
		"action":         "entry",
		"role":           "entry",
		"reason":         "Initial",
		"side":           "BUY",
		"type":           "MARKET",
		"symbol":         "BTCUSDT",
		"quantity":       0.001,
		"priceHint":      68000.0,
		"signalKind":     "initial-entry",
		"plannedEventAt": time.Now().UTC().Format(time.RFC3339),
	}
	state["dispatchMode"] = "auto-dispatch"
	updatedSession, err := platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("UpdateLiveSessionState returned error: %v", err)
	}
	updatedSession, err = platform.store.UpdateLiveSessionStatus(updatedSession.ID, "RUNNING")
	if err != nil {
		t.Fatalf("UpdateLiveSessionStatus returned error: %v", err)
	}

	order, err := platform.dispatchLiveSessionIntent(updatedSession)
	if err != nil {
		t.Fatalf("dispatchLiveSessionIntent returned error: %v", err)
	}
	if got := order.AccountID; got != "live-main" {
		t.Fatalf("expected order account live-main, got %q", got)
	}
	if got := order.Side; got != "BUY" {
		t.Fatalf("expected BUY side, got %q", got)
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected ACCEPTED status, got %q", got)
	}
	latest, err := platform.store.GetLiveSession(updatedSession.ID)
	if err != nil {
		t.Fatalf("GetLiveSession returned error: %v", err)
	}
	if got := stringValue(latest.State["lastDispatchedOrderId"]); got == "" {
		t.Fatal("expected lastDispatchedOrderId to be set")
	}
	if got := stringValue(latest.State["lastDispatchedOrderStatus"]); got != "ACCEPTED" {
		t.Fatalf("expected lastDispatchedOrderStatus ACCEPTED, got %q", got)
	}
}
