package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestEvaluateLiveSessionOnSignalPersistsStrategyDecisionEvent(t *testing.T) {
	platform, session, runtimeSessionID, summary, eventTime := prepareLiveDecisionTelemetryFixture(t)

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	events, err := platform.store.ListStrategyDecisionEvents(session.ID)
	if err != nil {
		t.Fatalf("list strategy decision events failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 strategy decision event, got %d", len(events))
	}
	if got := stringValue(updated.State["lastStrategyDecisionEventId"]); got != events[0].ID {
		t.Fatalf("expected session to record latest decision event id %s, got %s", events[0].ID, got)
	}
	if events[0].RuntimeSessionID != runtimeSessionID {
		t.Fatalf("expected runtime session id %s, got %s", runtimeSessionID, events[0].RuntimeSessionID)
	}
	if events[0].Action == "" || events[0].Reason == "" {
		t.Fatalf("expected non-empty action/reason, got %+v", events[0])
	}
	dispatchedIntent := mapValue(updated.State["lastDispatchedIntent"])
	if got := stringValue(dispatchedIntent["decisionEventId"]); got != events[0].ID {
		t.Fatalf("expected dispatched intent to carry decision event id %s, got %s", events[0].ID, got)
	}
}

func TestApplyLiveSubmissionResultPersistsOrderExecutionEvent(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	order, err := platform.store.CreateOrder(domain.Order{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "LIMIT",
		Quantity:          0.002,
		Price:             68999.0,
		Metadata: map[string]any{
			"source":          "live-session-intent",
			"liveSessionId":   "live-session-main",
			"decisionEventId": "decision-1",
			"executionProposal": map[string]any{
				"symbol":            "BTCUSDT",
				"side":              "BUY",
				"type":              "LIMIT",
				"quantity":          0.002,
				"priceHint":         68999.0,
				"executionStrategy": "book-aware-v1",
				"metadata": map[string]any{
					"executionDecision": "maker-resting",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	acceptedAt := order.CreatedAt.Add(2 * time.Second)
	updated, err := platform.applyLiveSubmissionResult(
		order,
		map[string]any{"adapterKey": "binance-futures"},
		domain.SignalRuntimeSession{ID: "runtime-1"},
		map[string]any{"ready": true},
		LiveOrderSubmission{
			Status:          "ACCEPTED",
			ExchangeOrderID: "exchange-order-1",
			AcceptedAt:      acceptedAt.Format(time.RFC3339),
			Metadata: map[string]any{
				"rawQuantity":        0.0021,
				"normalizedQuantity": 0.002,
				"rawPriceReference":  68999.3,
				"normalizedPrice":    68999.0,
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("apply live submission result failed: %v", err)
	}

	events, err := platform.store.ListOrderExecutionEvents(updated.ID)
	if err != nil {
		t.Fatalf("list order execution events failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 order execution event, got %d", len(events))
	}
	if events[0].EventType != "submitted" {
		t.Fatalf("expected submitted event type, got %s", events[0].EventType)
	}
	if events[0].DecisionEventID != "decision-1" {
		t.Fatalf("expected decision event id decision-1, got %s", events[0].DecisionEventID)
	}
	if events[0].RuntimeSessionID != "runtime-1" {
		t.Fatalf("expected runtime session id runtime-1, got %s", events[0].RuntimeSessionID)
	}
	if events[0].Status != "ACCEPTED" {
		t.Fatalf("expected status ACCEPTED, got %s", events[0].Status)
	}
	if events[0].SubmitLatencyMs <= 0 {
		t.Fatalf("expected positive submit latency, got %d", events[0].SubmitLatencyMs)
	}
}

func TestRefreshLiveSessionPositionContextPersistsPositionAccountSnapshot(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"syncStatus":         "SYNCED",
		"availableBalance":   9200.0,
		"totalWalletBalance": 9500.0,
		"totalMarginBalance": 9400.0,
		"positionCount":      1,
		"openOrders":         []map[string]any{},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["symbol"] = "BTCUSDT"
	state["signalTimeframe"] = "1d"
	state["lastStrategyDecisionEventId"] = "decision-ctx-1"
	state["lastDispatchedIntentSignature"] = "intent|btc"
	state["lastStrategyEvaluationSignalBarStates"] = map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"atr14":     900.0,
			"ma20":      68000.0,
			"current": map[string]any{
				"close": 70100.0,
				"high":  70250.0,
				"low":   69500.0,
			},
			"prevBar1": map[string]any{
				"high": 69900.0,
				"low":  68800.0,
			},
			"prevBar2": map[string]any{
				"high": 69750.0,
				"low":  68750.0,
			},
		},
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.003,
		EntryPrice:        69000.0,
		MarkPrice:         70100.0,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	eventTime := time.Date(2026, 4, 14, 3, 0, 0, 0, time.UTC)
	if _, err := platform.refreshLiveSessionPositionContext(session, eventTime, "telemetry-test"); err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}

	snapshots, err := platform.store.ListPositionAccountSnapshots(session.AccountID)
	if err != nil {
		t.Fatalf("list position/account snapshots failed: %v", err)
	}
	if len(snapshots) == 0 {
		t.Fatal("expected at least one position/account snapshot")
	}
	got := snapshots[len(snapshots)-1]
	if got.LiveSessionID != session.ID {
		t.Fatalf("expected snapshot live session id %s, got %s", session.ID, got.LiveSessionID)
	}
	if got.Trigger != "telemetry-test" {
		t.Fatalf("expected trigger telemetry-test, got %s", got.Trigger)
	}
	if got.PositionQuantity <= 0 {
		t.Fatalf("expected positive position quantity, got %v", got.PositionQuantity)
	}
	if got.AvailableBalance != 9200.0 {
		t.Fatalf("expected available balance 9200, got %v", got.AvailableBalance)
	}
	if got.DecisionEventID != "decision-ctx-1" {
		t.Fatalf("expected decision event id decision-ctx-1, got %s", got.DecisionEventID)
	}
}

func prepareLiveDecisionTelemetryFixture(t *testing.T) (*Platform, domain.LiveSession, string, map[string]any, time.Time) {
	t.Helper()

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

	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
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

	return platform, session, runtimeSessionID, summary, eventTime
}
