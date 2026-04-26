package memory

import (
	"math"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestSignalRuntimeSessionCRUD(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()
	session := domain.SignalRuntimeSession{
		ID:              "signal-runtime-test",
		AccountID:       "live-main",
		StrategyID:      "strategy-bk-1d",
		Status:          "READY",
		RuntimeAdapter:  "binance-futures",
		Transport:       "websocket",
		SubscriptionCnt: 1,
		State:           map[string]any{"health": "idle"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	created, err := store.CreateSignalRuntimeSession(session)
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	if created.ID != session.ID {
		t.Fatalf("expected created id %q, got %q", session.ID, created.ID)
	}

	created.Status = "RUNNING"
	created.State["health"] = "healthy"
	updated, err := store.UpdateSignalRuntimeSession(created)
	if err != nil {
		t.Fatalf("UpdateSignalRuntimeSession failed: %v", err)
	}
	if updated.Status != "RUNNING" || updated.State["health"] != "healthy" {
		t.Fatalf("expected updated runtime session, got %#v", updated)
	}

	items, err := store.ListSignalRuntimeSessions()
	if err != nil {
		t.Fatalf("ListSignalRuntimeSessions failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != session.ID {
		t.Fatalf("expected one listed session %q, got %#v", session.ID, items)
	}

	if err := store.DeleteSignalRuntimeSession(session.ID); err != nil {
		t.Fatalf("DeleteSignalRuntimeSession failed: %v", err)
	}
	if _, err := store.GetSignalRuntimeSession(session.ID); err == nil {
		t.Fatal("expected deleted runtime session to be missing")
	}
}

func TestCreateSignalRuntimeSessionUpsertsAccountStrategyIdentity(t *testing.T) {
	store := NewStore()
	first := domain.SignalRuntimeSession{
		ID:         "signal-runtime-first",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "READY",
		State:      map[string]any{"health": "idle", "plan": map[string]any{"version": "old"}},
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	second := first
	second.ID = "signal-runtime-second"
	second.Status = "RUNNING"
	second.RuntimeAdapter = "binance-market-ws"
	second.Transport = "websocket"
	second.SubscriptionCnt = 2
	second.State = map[string]any{"health": "healthy", "plan": map[string]any{"version": "new"}}
	second.UpdatedAt = first.UpdatedAt.Add(time.Minute)

	created, err := store.CreateSignalRuntimeSession(first)
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession first failed: %v", err)
	}
	reused, err := store.CreateSignalRuntimeSession(second)
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession second failed: %v", err)
	}
	if reused.ID != created.ID {
		t.Fatalf("expected duplicate account+strategy to reuse %q, got %q", created.ID, reused.ID)
	}
	if reused.Status != "RUNNING" || reused.RuntimeAdapter != "binance-market-ws" || reused.SubscriptionCnt != 2 {
		t.Fatalf("expected duplicate account+strategy to update runtime fields, got %#v", reused)
	}
	plan := reused.State["plan"].(map[string]any)
	if plan["version"] != "new" {
		t.Fatalf("expected duplicate account+strategy to update plan state, got %#v", reused.State)
	}
}

func TestCreateSignalRuntimeSessionRejectsInvalidStateJSON(t *testing.T) {
	store := NewStore()
	_, err := store.CreateSignalRuntimeSession(domain.SignalRuntimeSession{
		ID:         "signal-runtime-invalid",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "READY",
		State:      map[string]any{"bad": math.NaN()},
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected NaN state to fail JSON persistence")
	}
}
