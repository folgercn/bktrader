package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestGetSignalRuntimeSessionRestoresCacheFromStore(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	platform.mu.Lock()
	platform.signalSessions = map[string]domain.SignalRuntimeSession{}
	platform.mu.Unlock()

	restored, err := platform.GetSignalRuntimeSession(runtimeSession.ID)
	if err != nil {
		t.Fatalf("GetSignalRuntimeSession failed: %v", err)
	}
	if restored.ID != runtimeSession.ID {
		t.Fatalf("expected restored runtime id %q, got %q", runtimeSession.ID, restored.ID)
	}
	platform.mu.Lock()
	_, cached := platform.signalSessions[runtimeSession.ID]
	platform.mu.Unlock()
	if !cached {
		t.Fatal("expected persisted runtime session to be cached after get")
	}
}

func TestSyncLiveSessionRuntimeReusesPersistedRuntimeAfterCacheMiss(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "30m"},
	}); err != nil {
		t.Fatalf("bind strategy signal source failed: %v", err)
	}
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	liveSession, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("GetLiveSession failed: %v", err)
	}
	state := cloneMetadata(liveSession.State)
	state["signalRuntimeSessionId"] = runtimeSession.ID
	liveSession, err = platform.store.UpdateLiveSessionState(liveSession.ID, state)
	if err != nil {
		t.Fatalf("UpdateLiveSessionState failed: %v", err)
	}

	platform.mu.Lock()
	platform.signalSessions = map[string]domain.SignalRuntimeSession{}
	platform.mu.Unlock()

	updated, err := platform.syncLiveSessionRuntime(liveSession)
	if err != nil {
		t.Fatalf("syncLiveSessionRuntime failed: %v", err)
	}
	if got := stringValue(updated.State["signalRuntimeSessionId"]); got != runtimeSession.ID {
		t.Fatalf("expected persisted runtime id %q to be reused, got %q", runtimeSession.ID, got)
	}
	if got := stringValue(updated.State["signalRuntimeStatus"]); got != runtimeSession.Status {
		t.Fatalf("expected restored runtime status %q, got %q", runtimeSession.Status, got)
	}
}

func TestStartSignalRuntimeSessionSingleFlightsConcurrentStarts(t *testing.T) {
	store := &blockingSignalRuntimeStore{
		Store:   memory.NewStore(),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	platform := NewPlatform(store)
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := platform.StartSignalRuntimeSession(runtimeSession.ID)
		errCh <- err
	}()
	<-store.entered

	if _, err := platform.StartSignalRuntimeSession(runtimeSession.ID); err != nil {
		t.Fatalf("second StartSignalRuntimeSession should return existing in-flight runtime: %v", err)
	}
	close(store.release)
	if err := <-errCh; err != nil {
		t.Fatalf("first StartSignalRuntimeSession failed: %v", err)
	}
	if got := store.runningUpdates.Load(); got != 1 {
		t.Fatalf("expected one persisted RUNNING transition, got %d", got)
	}
	_, _ = platform.StopSignalRuntimeSessionWithForce(runtimeSession.ID, true)
}

func TestDeleteSignalRuntimeSessionKeepsRunnerWhenStoreDeleteFails(t *testing.T) {
	store := &deleteFailSignalRuntimeStore{Store: memory.NewStore()}
	platform := NewPlatform(store)
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &signalRuntimeRun{ctx: ctx, cancel: cancel}
	platform.mu.Lock()
	platform.signalRun[runtimeSession.ID] = run
	platform.signalSessions[runtimeSession.ID] = runtimeSession
	platform.mu.Unlock()

	if err := platform.DeleteSignalRuntimeSessionWithForce(runtimeSession.ID, true); err == nil {
		t.Fatal("expected delete failure")
	}
	platform.mu.Lock()
	_, stillRunning := platform.signalRun[runtimeSession.ID]
	_, stillCached := platform.signalSessions[runtimeSession.ID]
	platform.mu.Unlock()
	if !stillRunning || !stillCached {
		t.Fatal("expected local runner/cache to remain when store delete fails")
	}
	if ctx.Err() != nil {
		t.Fatalf("expected runner context to remain active, got %v", ctx.Err())
	}
}

type blockingSignalRuntimeStore struct {
	*memory.Store
	entered        chan struct{}
	release        chan struct{}
	enteredOnce    sync.Once
	runningUpdates atomic.Int32
}

func (s *blockingSignalRuntimeStore) UpdateSignalRuntimeSession(session domain.SignalRuntimeSession) (domain.SignalRuntimeSession, error) {
	if session.Status == "RUNNING" {
		s.runningUpdates.Add(1)
		s.enteredOnce.Do(func() { close(s.entered) })
		<-s.release
	}
	return s.Store.UpdateSignalRuntimeSession(session)
}

type deleteFailSignalRuntimeStore struct {
	*memory.Store
}

func (s *deleteFailSignalRuntimeStore) DeleteSignalRuntimeSession(string) error {
	return errors.New("delete failed")
}
