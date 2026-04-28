package service

import (
	"context"
	"testing"

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
