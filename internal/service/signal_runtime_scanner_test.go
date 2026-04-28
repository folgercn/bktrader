package service

import (
	"context"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestScanSignalRuntimeSessionsStartsDesiredRunningSessions(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 26, 18, 0, 0, 0, time.UTC)
	desired := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-desired",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "STOPPED",
		State: map[string]any{
			"desiredStatus": "RUNNING",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-stopped",
		AccountID:  "account-1",
		StrategyID: "strategy-2",
		Status:     "RUNNING",
		State: map[string]any{
			"desiredStatus": "STOPPED",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})

	started := make([]string, 0)
	platform.scanSignalRuntimeSessions(context.Background(), func(_ context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
		started = append(started, sessionID)
		return desired, nil
	})
	if len(started) != 1 || started[0] != desired.ID {
		t.Fatalf("expected only desired running session to start, got %#v", started)
	}
}

func TestScanSignalRuntimeSessionsSkipsLocalRunningSession(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 26, 18, 0, 0, 0, time.UTC)
	session := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-running",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		State:      map[string]any{},
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	platform.mu.Lock()
	platform.signalRun[session.ID] = &signalRuntimeRun{starting: true}
	platform.mu.Unlock()

	started := 0
	platform.scanSignalRuntimeSessions(context.Background(), func(_ context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
		started++
		return session, nil
	})
	if started != 0 {
		t.Fatalf("expected scanner to skip locally running session, got %d starts", started)
	}
}

func TestScanSignalRuntimeSessionsDefersErroredDesiredRunningSessionUntilBackoff(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 26, 18, 0, 0, 0, time.UTC)
	session := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-error",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "ERROR",
		State: map[string]any{
			"desiredStatus":     "RUNNING",
			"actualStatus":      "ERROR",
			"nextAutoRestartAt": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		},
		CreatedAt: now,
		UpdatedAt: now,
	})

	started := 0
	platform.scanSignalRuntimeSessions(context.Background(), func(_ context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
		started++
		return session, nil
	})
	if started != 0 {
		t.Fatalf("expected scanner to wait for supervisor backoff, got %d starts", started)
	}
}

func TestScanSignalRuntimeSessionsRestartsErroredDesiredRunningSessionAfterBackoff(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 26, 18, 0, 0, 0, time.UTC)
	session := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-error-due",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "ERROR",
		State: map[string]any{
			"desiredStatus":            "RUNNING",
			"actualStatus":             "ERROR",
			"supervisorRestartAttempt": 1,
			"nextAutoRestartAt":        time.Now().UTC().Add(-time.Second).Format(time.RFC3339),
		},
		CreatedAt: now,
		UpdatedAt: now,
	})

	started := make([]string, 0)
	platform.scanSignalRuntimeSessions(context.Background(), func(_ context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
		started = append(started, sessionID)
		return session, nil
	})
	if len(started) != 1 || started[0] != session.ID {
		t.Fatalf("expected scanner to restart errored desired-running session after backoff, got %#v", started)
	}
}

func TestSetSessionTerminalErrorSchedulesOneThenRepeatingThreeMinuteSupervisorBackoff(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Now().UTC()
	session := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-supervisor-backoff",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		State: map[string]any{
			"desiredStatus": "RUNNING",
			"actualStatus":  "RUNNING",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})

	platform.setSessionTerminalError(session.ID, context.DeadlineExceeded)
	first, err := platform.GetSignalRuntimeSession(session.ID)
	if err != nil {
		t.Fatalf("reload first terminal error failed: %v", err)
	}
	if got := RestartAttempt(first.State, "supervisorRestartAttempt"); got != 1 {
		t.Fatalf("expected first supervisor attempt 1, got %d", got)
	}
	firstNext, ok := ParseRestartTime(first.State, "nextAutoRestartAt")
	if !ok {
		t.Fatalf("expected first nextAutoRestartAt")
	}
	if delta := time.Until(firstNext); delta < 55*time.Second || delta > 65*time.Second {
		t.Fatalf("expected first backoff about 1m, got %s", delta)
	}

	platform.setSessionTerminalError(session.ID, context.DeadlineExceeded)
	second, err := platform.GetSignalRuntimeSession(session.ID)
	if err != nil {
		t.Fatalf("reload second terminal error failed: %v", err)
	}
	if got := RestartAttempt(second.State, "supervisorRestartAttempt"); got != 2 {
		t.Fatalf("expected second supervisor attempt 2, got %d", got)
	}
	secondNext, ok := ParseRestartTime(second.State, "nextAutoRestartAt")
	if !ok {
		t.Fatalf("expected second nextAutoRestartAt")
	}
	if delta := time.Until(secondNext); delta < 175*time.Second || delta > 185*time.Second {
		t.Fatalf("expected second backoff about 3m, got %s", delta)
	}

	platform.setSessionTerminalError(session.ID, context.DeadlineExceeded)
	third, err := platform.GetSignalRuntimeSession(session.ID)
	if err != nil {
		t.Fatalf("reload third terminal error failed: %v", err)
	}
	if got := RestartAttempt(third.State, "supervisorRestartAttempt"); got != 3 {
		t.Fatalf("expected third supervisor attempt 3, got %d", got)
	}
	thirdNext, ok := ParseRestartTime(third.State, "nextAutoRestartAt")
	if !ok {
		t.Fatalf("expected third nextAutoRestartAt")
	}
	if delta := time.Until(thirdNext); delta < 175*time.Second || delta > 185*time.Second {
		t.Fatalf("expected third backoff to keep repeating about 3m, got %s", delta)
	}
}

func TestScanSignalRuntimeSessionsSkipsSessionOwnedByActiveLease(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.setRuntimeLeaseOwnerIDForTest("runner-local")
	session, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	session.Status = "STOPPED"
	session.State = cloneMetadata(session.State)
	session.State["desiredStatus"] = "RUNNING"
	session.State["actualStatus"] = "STOPPED"
	updated, err := store.UpdateSignalRuntimeSession(session)
	if err != nil {
		t.Fatalf("UpdateSignalRuntimeSession failed: %v", err)
	}
	platform.cacheSignalRuntimeSession(updated)
	if _, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   updated.ID,
		OwnerID:      "runner-other",
		TTL:          time.Minute,
	}); err != nil || !ok {
		t.Fatalf("pre-acquire runtime lease failed: ok=%v err=%v", ok, err)
	}

	started := 0
	platform.scanSignalRuntimeSessions(context.Background(), func(ctx context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
		startedSession, err := platform.startSignalRuntimeSession(ctx, sessionID)
		if err == nil {
			started++
		}
		return startedSession, err
	})
	if started != 0 {
		t.Fatalf("expected scanner to skip session owned by another active runner, got %d starts", started)
	}
}

func TestScanSignalRuntimeSessionsStopsDesiredRunningRuntimeLinkedToStoppedLiveSession(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 26, 18, 0, 0, 0, time.UTC)
	runtime := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-linked-stopped-live",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "RUNNING",
		State: map[string]any{
			"desiredStatus": "RUNNING",
			"actualStatus":  "RUNNING",
			"health":        "healthy",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	liveSession, err := platform.store.UpdateLiveSessionStatus("live-session-main", "STOPPED")
	if err != nil {
		t.Fatalf("set live session stopped failed: %v", err)
	}
	liveState := cloneMetadata(liveSession.State)
	liveState["signalRuntimeSessionId"] = runtime.ID
	if _, err := platform.store.UpdateLiveSessionState(liveSession.ID, liveState); err != nil {
		t.Fatalf("link live session to runtime failed: %v", err)
	}

	started := 0
	platform.scanSignalRuntimeSessions(context.Background(), func(_ context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
		started++
		return runtime, nil
	})
	if started != 0 {
		t.Fatalf("expected scanner not to start runtime linked to stopped live session, got %d starts", started)
	}
	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected runtime STOPPED, got %s", updatedRuntime.Status)
	}
	if got := stringValue(updatedRuntime.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected desiredStatus STOPPED, got %s", got)
	}
	updatedLiveSession, err := platform.store.GetLiveSession(liveSession.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if got := stringValue(updatedLiveSession.State["signalRuntimeStatus"]); got != "STOPPED" {
		t.Fatalf("expected live signalRuntimeStatus STOPPED, got %s", got)
	}
}

func TestPersistSignalRuntimeStoppedAfterStartCancelClearsDesiredAndActual(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Date(2026, 4, 26, 18, 0, 0, 0, time.UTC)
	session := mustCreateScannerRuntimeSession(t, platform, domain.SignalRuntimeSession{
		ID:         "runtime-cancelled",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		State: map[string]any{
			"desiredStatus": "RUNNING",
			"actualStatus":  "STARTING",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})

	platform.persistSignalRuntimeStoppedAfterStartCancel(session)
	updated, err := platform.GetSignalRuntimeSession(session.ID)
	if err != nil {
		t.Fatalf("get runtime session failed: %v", err)
	}
	if updated.Status != "STOPPED" {
		t.Fatalf("expected status STOPPED after start cancel, got %s", updated.Status)
	}
	if got := stringValue(updated.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected desiredStatus STOPPED after start cancel, got %s", got)
	}
	if got := stringValue(updated.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED after start cancel, got %s", got)
	}
}

func mustCreateScannerRuntimeSession(t *testing.T, platform *Platform, session domain.SignalRuntimeSession) domain.SignalRuntimeSession {
	t.Helper()
	created, err := platform.store.CreateSignalRuntimeSession(session)
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	platform.cacheSignalRuntimeSession(created)
	return created
}
