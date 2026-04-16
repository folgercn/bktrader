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
