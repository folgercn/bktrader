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
	platform.scanSignalRuntimeSessions(context.Background(), func(sessionID string) (domain.SignalRuntimeSession, error) {
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
	platform.scanSignalRuntimeSessions(context.Background(), func(sessionID string) (domain.SignalRuntimeSession, error) {
		started++
		return session, nil
	})
	if started != 0 {
		t.Fatalf("expected scanner to skip locally running session, got %d starts", started)
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
