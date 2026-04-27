package memory

import "testing"

func TestDeleteLiveSessionSoftDeletesAndListHidesDeleted(t *testing.T) {
	store := NewStore()
	session, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("GetLiveSession failed: %v", err)
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

func TestListLiveSessionsKeepsNonDeletedSessionWithLegacyDeletedAtState(t *testing.T) {
	store := NewStore()
	session, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("GetLiveSession failed: %v", err)
	}
	session.Status = "READY"
	session.State = map[string]any{
		"deletedAt":    "legacy-non-soft-delete-marker",
		"dispatchMode": "manual-review",
	}
	if _, err := store.UpdateLiveSession(session); err != nil {
		t.Fatalf("UpdateLiveSession failed: %v", err)
	}

	items, err := store.ListLiveSessions()
	if err != nil {
		t.Fatalf("ListLiveSessions failed: %v", err)
	}
	for _, item := range items {
		if item.ID == session.ID {
			return
		}
	}
	t.Fatalf("expected non-DELETED live session %s with legacy deletedAt state to remain listed", session.ID)
}
