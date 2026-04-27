package postgres

import (
	"os"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestSignalRuntimeSessionCreateUpsertsPlanState(t *testing.T) {
	dsn := os.Getenv("BKTRADER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BKTRADER_TEST_POSTGRES_DSN is not set")
	}
	if err := Migrate(dsn); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	store, err := New(dsn)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	account, err := store.CreateAccount("Runtime Upsert Test", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	strategy, err := store.CreateStrategy("runtime-upsert-test", "runtime upsert test", map[string]any{
		"strategyEngine": "bk-default",
	})
	if err != nil {
		t.Fatalf("CreateStrategy failed: %v", err)
	}
	strategyID := strategy["id"].(string)
	now := time.Now().UTC()

	first, err := store.CreateSignalRuntimeSession(domain.SignalRuntimeSession{
		ID:              "signal-runtime-pg-first-" + now.Format("20060102150405.000000000"),
		AccountID:       account.ID,
		StrategyID:      strategyID,
		Status:          "READY",
		RuntimeAdapter:  "old-adapter",
		Transport:       "websocket",
		SubscriptionCnt: 1,
		State:           map[string]any{"plan": map[string]any{"version": "old"}},
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession first failed: %v", err)
	}

	second, err := store.CreateSignalRuntimeSession(domain.SignalRuntimeSession{
		ID:              "signal-runtime-pg-second-" + now.Format("20060102150405.000000000"),
		AccountID:       account.ID,
		StrategyID:      strategyID,
		Status:          "RUNNING",
		RuntimeAdapter:  "new-adapter",
		Transport:       "websocket",
		SubscriptionCnt: 2,
		State:           map[string]any{"plan": map[string]any{"version": "new"}},
		CreatedAt:       now.Add(time.Second),
		UpdatedAt:       now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession second failed: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected upsert to preserve runtime identity %q, got %q", first.ID, second.ID)
	}
	if second.Status != "RUNNING" || second.RuntimeAdapter != "new-adapter" || second.SubscriptionCnt != 2 {
		t.Fatalf("expected upserted runtime fields, got %#v", second)
	}
	plan := second.State["plan"].(map[string]any)
	if plan["version"] != "new" {
		t.Fatalf("expected upserted plan state, got %#v", second.State)
	}
}
