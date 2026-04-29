package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestListLogEventsSupportsPaginationAndFilters(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	base := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)

	if _, err := store.CreateStrategyDecisionEvent(domain.StrategyDecisionEvent{
		ID:               "decision-1",
		LiveSessionID:    "live-1",
		RuntimeSessionID: "runtime-1",
		AccountID:        "account-1",
		StrategyID:       "strategy-1",
		Action:           "wait",
		Reason:           "waiting for source gate",
		SourceGateReady:  false,
		MissingCount:     1,
		EventTime:        base.Add(-3 * time.Minute),
		RecordedAt:       base.Add(-3 * time.Minute),
	}); err != nil {
		t.Fatalf("create strategy decision event: %v", err)
	}
	if _, err := store.CreateOrderExecutionEvent(domain.OrderExecutionEvent{
		ID:               "execution-1",
		OrderID:          "order-1",
		LiveSessionID:    "live-1",
		DecisionEventID:  "decision-1",
		RuntimeSessionID: "runtime-1",
		AccountID:        "account-1",
		Status:           "FAILED",
		EventType:        "submit",
		Failed:           true,
		Error:            "exchange unavailable",
		EventTime:        base.Add(-2 * time.Minute),
		RecordedAt:       base.Add(-2 * time.Minute),
		Metadata: map[string]any{
			"strategyId": "strategy-1",
		},
	}); err != nil {
		t.Fatalf("create order execution event: %v", err)
	}
	if _, err := store.CreatePositionAccountSnapshot(domain.PositionAccountSnapshot{
		ID:              "snapshot-1",
		LiveSessionID:   "live-2",
		AccountID:       "account-2",
		StrategyID:      "strategy-2",
		Trigger:         "post-sync",
		SyncStatus:      "ok",
		EventTime:       base.Add(-1 * time.Minute),
		RecordedAt:      base.Add(-1 * time.Minute),
		DecisionEventID: "decision-2",
	}); err != nil {
		t.Fatalf("create position account snapshot: %v", err)
	}

	page, err := platform.ListLogEvents(UnifiedLogEventQuery{Limit: 2})
	if err != nil {
		t.Fatalf("list unified log events: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items on first page, got %d", len(page.Items))
	}
	if page.Items[0].ID != "snapshot-1" {
		t.Fatalf("expected newest snapshot first, got %s", page.Items[0].ID)
	}
	if page.Items[1].ID != "execution-1" {
		t.Fatalf("expected order execution second, got %s", page.Items[1].ID)
	}
	if page.NextCursor == "" {
		t.Fatal("expected next cursor for truncated page")
	}

	nextPage, err := platform.ListLogEvents(UnifiedLogEventQuery{
		Limit:  2,
		Cursor: page.NextCursor,
	})
	if err != nil {
		t.Fatalf("list next page: %v", err)
	}
	if len(nextPage.Items) != 1 || nextPage.Items[0].ID != "decision-1" {
		t.Fatalf("expected older decision event on second page, got %#v", nextPage.Items)
	}

	filtered, err := platform.ListLogEvents(UnifiedLogEventQuery{
		Type:      "order-execution",
		AccountID: "account-1",
		Level:     "critical",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("filter unified log events: %v", err)
	}
	if len(filtered.Items) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(filtered.Items))
	}
	item := filtered.Items[0]
	if item.ID != "execution-1" || item.Level != "critical" || item.OrderID != "order-1" {
		t.Fatalf("unexpected filtered item: %#v", item)
	}
}

func TestLiveControlMetricsAggregatesLatencyAndErrorCodes(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	base := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)

	session, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session: %v", err)
	}
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "RUNNING"
	state["actualStatus"] = "STARTING"
	state["controlRequestId"] = "request-2"
	state["controlVersion"] = 2
	state["controlRequestedAt"] = base.Add(-5 * time.Minute).Format(time.RFC3339)
	state[liveSessionControlEventStateKey] = []any{
		map[string]any{
			"id":               "control-event-1",
			"phase":            "request_accepted",
			"eventTime":        base.Add(-30 * time.Second).Format(time.RFC3339Nano),
			"recordedAt":       base.Add(-30 * time.Second).Format(time.RFC3339Nano),
			"liveSessionId":    session.ID,
			"accountId":        session.AccountID,
			"strategyId":       session.StrategyID,
			"controlRequestId": "request-1",
			"controlVersion":   1,
			"desiredStatus":    "RUNNING",
			"actualStatus":     "STOPPED",
			"action":           "start",
		},
		map[string]any{
			"id":               "control-event-2",
			"phase":            "runner_picked_up",
			"eventTime":        base.Add(-25 * time.Second).Format(time.RFC3339Nano),
			"recordedAt":       base.Add(-25 * time.Second).Format(time.RFC3339Nano),
			"liveSessionId":    session.ID,
			"accountId":        session.AccountID,
			"strategyId":       session.StrategyID,
			"controlRequestId": "request-1",
			"controlVersion":   1,
			"desiredStatus":    "RUNNING",
			"actualStatus":     "STARTING",
			"action":           "start",
			"latencyMs":        1500,
		},
		map[string]any{
			"id":               "control-event-3",
			"phase":            "succeeded",
			"eventTime":        base.Add(-20 * time.Second).Format(time.RFC3339Nano),
			"recordedAt":       base.Add(-20 * time.Second).Format(time.RFC3339Nano),
			"liveSessionId":    session.ID,
			"accountId":        session.AccountID,
			"strategyId":       session.StrategyID,
			"controlRequestId": "request-1",
			"controlVersion":   1,
			"desiredStatus":    "RUNNING",
			"actualStatus":     "RUNNING",
			"action":           "start",
			"latencyMs":        4500,
		},
		map[string]any{
			"id":               "control-event-4",
			"phase":            "failed",
			"eventTime":        base.Add(-10 * time.Second).Format(time.RFC3339Nano),
			"recordedAt":       base.Add(-10 * time.Second).Format(time.RFC3339Nano),
			"liveSessionId":    session.ID,
			"accountId":        session.AccountID,
			"strategyId":       session.StrategyID,
			"controlRequestId": "request-2",
			"controlVersion":   2,
			"desiredStatus":    "RUNNING",
			"actualStatus":     "ERROR",
			"action":           "start",
			"errorCode":        LiveSessionControlErrorCodeConfigError,
			"latencyMs":        8000,
		},
		map[string]any{
			"id":               "control-event-5",
			"phase":            "stale_update_discarded",
			"eventTime":        base.Add(-5 * time.Second).Format(time.RFC3339Nano),
			"recordedAt":       base.Add(-5 * time.Second).Format(time.RFC3339Nano),
			"liveSessionId":    session.ID,
			"accountId":        session.AccountID,
			"strategyId":       session.StrategyID,
			"controlRequestId": "request-old",
			"controlVersion":   1,
			"desiredStatus":    "RUNNING",
			"actualStatus":     "STARTING",
			"action":           "start",
		},
	}
	if _, err := store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("update live session state: %v", err)
	}

	metrics, err := platform.LiveControlMetrics(UnifiedLogEventQuery{LiveSessionID: session.ID})
	if err != nil {
		t.Fatalf("live control metrics: %v", err)
	}
	if metrics.TotalEvents != 5 || metrics.Requests != 1 || metrics.RunnerPickups != 1 || metrics.Succeeded != 1 || metrics.Failed != 1 || metrics.StaleDiscarded != 1 {
		t.Fatalf("unexpected counters: %#v", metrics)
	}
	if metrics.CurrentPending != 1 || metrics.CurrentErrors != 0 {
		t.Fatalf("expected current pending without current error, got pending=%d errors=%d", metrics.CurrentPending, metrics.CurrentErrors)
	}
	if got := metrics.ByErrorCode[LiveSessionControlErrorCodeConfigError]; got != 1 {
		t.Fatalf("expected CONFIG_ERROR count 1, got %d", got)
	}
	if metrics.Latency.PickupMs.Count != 1 || metrics.Latency.PickupMs.Min != 1500 {
		t.Fatalf("unexpected pickup latency: %#v", metrics.Latency.PickupMs)
	}
	if metrics.Latency.TerminalMs.Count != 2 || metrics.Latency.TerminalMs.Min != 4500 || metrics.Latency.TerminalMs.Max != 8000 || metrics.Latency.TerminalMs.Average != 6250 {
		t.Fatalf("unexpected terminal latency: %#v", metrics.Latency.TerminalMs)
	}
	accountGroup := metrics.ByAccount[session.AccountID]
	if accountGroup.Total != 5 || accountGroup.ErrorCodes[LiveSessionControlErrorCodeConfigError] != 1 {
		t.Fatalf("unexpected account group: %#v", accountGroup)
	}
}
