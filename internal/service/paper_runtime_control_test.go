package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestPaperSessionControlOptionsAuditStartAndStop(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.CreateAccount("paper", "PAPER", "BINANCE")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	session, err := platform.store.CreatePaperSession(account.ID, "strategy-bk-1d", 1000)
	if err != nil {
		t.Fatalf("CreatePaperSession failed: %v", err)
	}
	platform.paperPlans[session.ID] = []paperPlannedOrder{}

	started, err := platform.StartPaperSessionWithOptions(session.ID, PaperSessionControlOptions{
		Reason: "maintenance finished",
		Source: "api",
	})
	if err != nil {
		t.Fatalf("StartPaperSessionWithOptions failed: %v", err)
	}
	if got := stringValue(started.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected start desiredStatus RUNNING, got %s", got)
	}
	if got := stringValue(started.State["actualStatus"]); got != "RUNNING" {
		t.Fatalf("expected start actualStatus RUNNING, got %s", got)
	}
	if got := stringValue(started.State["startRequestedReason"]); got != "maintenance finished" {
		t.Fatalf("expected startRequestedReason, got %s", got)
	}
	if got := stringValue(started.State["startRequestedSource"]); got != "api" {
		t.Fatalf("expected startRequestedSource api, got %s", got)
	}

	stopped, err := platform.StopPaperSessionWithOptions(session.ID, PaperSessionControlOptions{
		Reason: "maintenance window",
		Source: "api",
	})
	if err != nil {
		t.Fatalf("StopPaperSessionWithOptions failed: %v", err)
	}
	if got := stringValue(stopped.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected stop desiredStatus STOPPED, got %s", got)
	}
	if got := stringValue(stopped.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected stop actualStatus STOPPED, got %s", got)
	}
	if got := stringValue(stopped.State["stopRequestedReason"]); got != "maintenance window" {
		t.Fatalf("expected stopRequestedReason, got %s", got)
	}
	if got := stopped.State["startRequestedAt"]; got != nil {
		t.Fatalf("expected stop to clear startRequestedAt, got %#v", got)
	}
}

func TestPaperSessionControlStartRecordsFailure(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.CreateAccount("paper", "PAPER", "BINANCE")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	session, err := platform.store.CreatePaperSession(account.ID, "missing-strategy", 1000)
	if err != nil {
		t.Fatalf("CreatePaperSession failed: %v", err)
	}

	if _, err := platform.StartPaperSessionWithOptions(session.ID, PaperSessionControlOptions{
		Reason: "maintenance finished",
		Source: "api",
	}); err == nil {
		t.Fatal("expected start failure")
	}
	stored, err := platform.store.GetPaperSession(session.ID)
	if err != nil {
		t.Fatalf("GetPaperSession failed: %v", err)
	}
	if got := stringValue(stored.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected desiredStatus RUNNING after failed start, got %s", got)
	}
	if got := stringValue(stored.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR after failed start, got %s", got)
	}
	if got := stringValue(stored.State["lastControlError"]); got == "" {
		t.Fatal("expected lastControlError after failed start")
	}
}
