package service

import (
	"context"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestScanLiveSessionControlRequestsStopsDesiredStoppedSession(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "STOPPED"
	state["actualStatus"] = "RUNNING"
	if _, err := store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("set desired stopped failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	updated, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if updated.Status != "STOPPED" {
		t.Fatalf("expected STOPPED status, got %s", updated.Status)
	}
	if got := stringValue(updated.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED, got %s", got)
	}
}

func TestScanLiveSessionControlRequestsPreservesStopSafetyUntilForceRequested(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, false); err != nil {
		t.Fatalf("request stop failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	blocked, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if blocked.Status != "RUNNING" {
		t.Fatalf("expected session to remain RUNNING after blocked stop, got %s", blocked.Status)
	}
	if got := stringValue(blocked.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR, got %s", got)
	}

	platform.scanLiveSessionControlRequests(context.Background())
	stillBlocked, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if stillBlocked.Status != "RUNNING" {
		t.Fatalf("expected ERROR state not to auto retry, got %s", stillBlocked.Status)
	}

	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, true); err != nil {
		t.Fatalf("request forced stop failed: %v", err)
	}
	platform.scanLiveSessionControlRequests(context.Background())

	stopped, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("expected forced stop to stop session, got %s", stopped.Status)
	}
	if got := stringValue(stopped.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED after force, got %s", got)
	}
}

func TestLiveSessionControlForceStopRecoversFromPreviousError(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, false); err != nil {
		t.Fatalf("request non-force stop failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	failed, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get failed live session failed: %v", err)
	}
	if failed.Status != "RUNNING" {
		t.Fatalf("expected non-force stop to leave session RUNNING, got %s", failed.Status)
	}
	if got := stringValue(failed.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR after blocked stop, got %s", got)
	}
	if got := stringValue(failed.State["lastControlError"]); got == "" {
		t.Fatal("expected lastControlError after blocked stop")
	}

	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, true); err != nil {
		t.Fatalf("request force stop retry failed: %v", err)
	}
	retry, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get retry live session failed: %v", err)
	}
	if got := stringValue(retry.State["actualStatus"]); got == "ERROR" {
		t.Fatal("expected force stop request to clear previous ERROR actualStatus")
	}

	platform.scanLiveSessionControlRequests(context.Background())

	stopped, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get stopped live session failed: %v", err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("expected force stop retry to stop session, got %s", stopped.Status)
	}
	if got := stringValue(stopped.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED after force stop recovery, got %s", got)
	}
	if got := stringValue(stopped.State["lastControlError"]); got != "" {
		t.Fatalf("expected lastControlError cleared after recovery, got %s", got)
	}
}

func TestScanLiveSessionControlRequestsWritesErrorWithoutRetryingStart(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	if _, err := platform.RequestLiveSessionStart("live-session-main"); err != nil {
		t.Fatalf("request start failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	failed, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if failed.Status == "RUNNING" {
		t.Fatal("expected start to fail for unconfigured live account")
	}
	if got := stringValue(failed.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR, got %s", got)
	}
	if got := stringValue(failed.State["lastControlError"]); got == "" {
		t.Fatal("expected lastControlError")
	}

	platform.scanLiveSessionControlRequests(context.Background())
	stillFailed, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(stillFailed.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected ERROR not to auto retry, got %s", got)
	}
}

func TestLiveSessionControlErrorCurrentAllowsNewerRequest(t *testing.T) {
	now := time.Now().UTC()
	state := map[string]any{
		"actualStatus":       "ERROR",
		"controlRequestedAt": now.Add(-time.Minute).Format(time.RFC3339),
		"lastControlErrorAt": now.Format(time.RFC3339),
	}
	if !liveSessionControlErrorCurrent(state) {
		t.Fatal("expected current control error to block automatic retry")
	}

	state["controlRequestedAt"] = now.Add(time.Minute).Format(time.RFC3339)
	if liveSessionControlErrorCurrent(state) {
		t.Fatal("expected newer control request to wake scanner")
	}
}

func TestDeleteLiveSessionCancelsPendingDesiredControlIntent(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	if _, err := platform.RequestLiveSessionStart("live-session-main"); err != nil {
		t.Fatalf("request start failed: %v", err)
	}

	if err := platform.DeleteLiveSessionWithForce("live-session-main", true); err != nil {
		t.Fatalf("delete live session failed: %v", err)
	}
	platform.scanLiveSessionControlRequests(context.Background())

	deleted, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if deleted.Status != "DELETED" {
		t.Fatalf("expected DELETED status, got %s", deleted.Status)
	}
	if got := stringValue(deleted.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected delete to cancel desiredStatus as STOPPED, got %s", got)
	}
	if got := stringValue(deleted.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected delete to mark actualStatus STOPPED, got %s", got)
	}
	if got := stringValue(deleted.State["controlDeletedAt"]); got == "" {
		t.Fatal("expected controlDeletedAt after delete")
	}
	if _, ok := deleted.State["desiredStopForce"]; ok {
		t.Fatalf("expected desiredStopForce to be cleared, got %#v", deleted.State["desiredStopForce"])
	}

	listed, err := store.ListLiveSessions()
	if err != nil {
		t.Fatalf("list live sessions failed: %v", err)
	}
	for _, item := range listed {
		if item.ID == deleted.ID {
			t.Fatalf("expected deleted session to be hidden from scanner list")
		}
	}
}

func TestRecoverLiveTradingOnStartupSkipsLiveSessionDesiredStopped(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "STOPPED"
	if _, err := store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("set desired stopped failed: %v", err)
	}

	platform.RecoverLiveTradingOnStartup(context.Background())

	updated, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastRecoveryStatus"]); got != "skipped-live-desired-stopped" {
		t.Fatalf("expected skipped-live-desired-stopped, got %s", got)
	}
}
