package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/store"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestLiveSessionStoreWritesNotifyDashboard(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", nil)
	if err != nil {
		t.Fatalf("CreateLiveSession failed: %v", err)
	}
	waitForDashboardChange(t, platform.DashboardBroker(), DashboardDomainLiveSessions, "live-session-created", 100*time.Millisecond)

	session.Alias = "Primary live"
	if _, err := platform.UpdateLiveSession(session.ID, session.Alias, "", "", nil); err != nil {
		t.Fatalf("UpdateLiveSession failed: %v", err)
	}
	waitForDashboardChange(t, platform.DashboardBroker(), DashboardDomainLiveSessions, "live-session-updated", 100*time.Millisecond)

	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("UpdateLiveSessionStatus failed: %v", err)
	}
	waitForDashboardChange(t, platform.DashboardBroker(), DashboardDomainLiveSessions, "live-session-status-updated", 100*time.Millisecond)

	state := cloneMetadata(session.State)
	state["lastEvent"] = "test"
	if _, err := platform.store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("UpdateLiveSessionState failed: %v", err)
	}
	change := waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	if change.Domain != DashboardDomainLiveSessions || change.Reason != "live-session-state-updated" {
		t.Fatalf("unexpected state dashboard change: %+v", change)
	}
}

func TestLiveSessionDeleteNotifyDashboardOnSuccessOnly(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", nil)
	if err != nil {
		t.Fatalf("CreateLiveSession failed: %v", err)
	}
	drainDashboardChanges(platform.DashboardBroker())

	if err := platform.store.DeleteLiveSession(session.ID); err != nil {
		t.Fatalf("DeleteLiveSession failed: %v", err)
	}
	change := waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	if change.Domain != DashboardDomainLiveSessions || change.Reason != "live-session-deleted" {
		t.Fatalf("unexpected delete dashboard change: %+v", change)
	}

	drainDashboardChanges(platform.DashboardBroker())
	if err := platform.store.DeleteLiveSession("missing-live-session"); err == nil {
		t.Fatal("expected missing live session delete to fail")
	}
	select {
	case change := <-platform.DashboardBroker().changes:
		t.Fatalf("unexpected dashboard change for failed delete: %+v", change)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDashboardRootRepositoryUnwrapsStoreWrappers(t *testing.T) {
	root := memory.NewStore()
	wrapped := &testStoreWrapper{Repository: root}
	if got := dashboardRootRepository(wrapped); got != root {
		t.Fatalf("expected root repository unwrap, got %#v", got)
	}
}

func TestLiveSessionControlCASNotifyDashboardOnlyWhenUpdated(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", nil)
	if err != nil {
		t.Fatalf("CreateLiveSession failed: %v", err)
	}
	_ = waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)

	state := cloneMetadata(session.State)
	state["controlRequestId"] = "request-1"
	state["controlVersion"] = int64(3)
	if _, err := platform.store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("UpdateLiveSessionState failed: %v", err)
	}
	_ = waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	drainDashboardChanges(platform.DashboardBroker())

	staleState := cloneMetadata(state)
	staleState["actualStatus"] = "STALE"
	_, ok, err := platform.updateLiveSessionControlStateIfPrevious(session.ID, liveSessionControlRequest{
		ID:      "request-older",
		Version: 3,
	}, staleState)
	if err != nil {
		t.Fatalf("stale control state update failed: %v", err)
	}
	if ok {
		t.Fatal("expected stale control request update to be skipped")
	}
	select {
	case change := <-platform.DashboardBroker().changes:
		t.Fatalf("unexpected dashboard change for stale CAS: %+v", change)
	case <-time.After(50 * time.Millisecond):
	}

	nextState := cloneMetadata(state)
	nextState["actualStatus"] = "RUNNING"
	_, ok, err = platform.updateLiveSessionControlStateIfPrevious(session.ID, liveSessionControlRequest{
		ID:      "request-1",
		Version: 3,
	}, nextState)
	if err != nil {
		t.Fatalf("current control state update failed: %v", err)
	}
	if !ok {
		t.Fatal("expected current control request update to succeed")
	}
	change := waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	if change.Domain != DashboardDomainLiveSessions || change.Reason != "live-session-control-state-updated" {
		t.Fatalf("unexpected control dashboard change: %+v", change)
	}
}

type testStoreWrapper struct {
	store.Repository
}

func (s *testStoreWrapper) UnwrapStoreRepository() store.Repository {
	return s.Repository
}

func drainDashboardChanges(broker *DashboardBroker) {
	for {
		select {
		case <-broker.changes:
		default:
			return
		}
	}
}

func waitForDashboardChange(t *testing.T, broker *DashboardBroker, domain DashboardDomain, reason string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	seen := make([]dashboardChange, 0)
	for {
		select {
		case change := <-broker.changes:
			seen = append(seen, change)
			if change.Domain == domain && change.Reason == reason {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for dashboard change domain=%s reason=%s, saw %+v", domain, reason, seen)
		}
	}
}
