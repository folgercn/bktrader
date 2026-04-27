package postgres

import (
	"os"
	"testing"
)

func TestDeleteLiveSessionSoftDeletesAndListHidesDeleted(t *testing.T) {
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

	account, err := store.CreateAccount("Live Soft Delete Test", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	strategy, err := store.CreateStrategy("live-soft-delete-test", "live soft delete test", map[string]any{
		"strategyEngine": "bk-default",
	})
	if err != nil {
		t.Fatalf("CreateStrategy failed: %v", err)
	}
	session, err := store.CreateLiveSession(account.ID, strategy["id"].(string))
	if err != nil {
		t.Fatalf("CreateLiveSession failed: %v", err)
	}

	if err := store.DeleteLiveSession(session.ID); err != nil {
		t.Fatalf("DeleteLiveSession failed: %v", err)
	}
	deleted, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("soft-deleted session should remain loadable: %v", err)
	}
	if deleted.Status != "DELETED" {
		t.Fatalf("expected status DELETED, got %s", deleted.Status)
	}
	if raw, ok := deleted.State["deletedAt"]; !ok || raw == "" {
		t.Fatalf("expected deletedAt in state, got %#v", deleted.State)
	}
	items, err := store.ListLiveSessions()
	if err != nil {
		t.Fatalf("ListLiveSessions failed: %v", err)
	}
	for _, item := range items {
		if item.ID == session.ID {
			t.Fatalf("expected deleted live session %s to be hidden from list", session.ID)
		}
	}
}
