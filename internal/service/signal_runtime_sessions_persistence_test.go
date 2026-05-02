package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	run := &signalRuntimeRun{ctx: ctx, cancelRunner: cancel}
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

func TestStopSignalRuntimeSessionCancelsRunnerAndReleasesLeaseOnce(t *testing.T) {
	store := &countingRuntimeLeaseStore{Store: memory.NewStore()}
	platform := NewPlatform(store)
	platform.setRuntimeLeaseOwnerIDForTest("runner-local")
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	if _, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   runtimeSession.ID,
		OwnerID:      "runner-local",
		TTL:          runtimeLeaseTTL,
	}); err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease failed: ok=%v err=%v", ok, err)
	}

	ctx, cancelRunner := context.WithCancel(context.Background())
	run := &signalRuntimeRun{
		ctx:          ctx,
		cancelRunner: cancelRunner,
		releaseLease: func() {
			_, _ = store.ReleaseRuntimeLease(domain.RuntimeLeaseResourceSignalRuntimeSession, runtimeSession.ID, "runner-local")
		},
	}
	platform.mu.Lock()
	platform.signalRun[runtimeSession.ID] = run
	platform.signalSessions[runtimeSession.ID] = runtimeSession
	platform.mu.Unlock()

	if _, err := platform.StopSignalRuntimeSessionWithForce(runtimeSession.ID, true); err != nil {
		t.Fatalf("StopSignalRuntimeSessionWithForce failed: %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("expected stop to cancel runner context")
	}
	if got := store.releaseCalls.Load(); got != 1 {
		t.Fatalf("expected one owner release after stop, got %d", got)
	}
	if lease, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   runtimeSession.ID,
		OwnerID:      "runner-other",
		TTL:          runtimeLeaseTTL,
	}); err != nil || !ok || lease.OwnerID != "runner-other" {
		t.Fatalf("expected lease to be available after stop release, ok=%v lease=%#v err=%v", ok, lease, err)
	}
	if err := platform.DeleteSignalRuntimeSessionWithForce(runtimeSession.ID, true); err != nil {
		t.Fatalf("DeleteSignalRuntimeSessionWithForce failed: %v", err)
	}
	if got := store.releaseCalls.Load(); got != 1 {
		t.Fatalf("expected delete after stop not to double release, got %d", got)
	}
}

func TestRestartSignalRuntimeSessionStopsThenStarts(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	runtimeSession = markSignalRuntimeSessionRunningForTest(t, platform, runtimeSession)

	restarted, err := platform.RestartSignalRuntimeSessionWithOptions(runtimeSession.ID, SignalRuntimeRestartOptions{
		Force:  true,
		Reason: "operator requested rebinding",
		Source: "test",
	})
	if err != nil {
		t.Fatalf("RestartSignalRuntimeSessionWithOptions failed: %v", err)
	}
	if restarted.Status != "RUNNING" {
		t.Fatalf("expected restarted runtime status RUNNING, got %s", restarted.Status)
	}
	if got := stringValue(restarted.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected desiredStatus RUNNING, got %s", got)
	}
	if actual := stringValue(restarted.State["actualStatus"]); actual != "STARTING" && actual != "RUNNING" {
		t.Fatalf("expected actualStatus STARTING/RUNNING, got %s", actual)
	}
	if got := boolValue(restarted.State["restartRequestedForce"]); !got {
		t.Fatal("expected restartRequestedForce true")
	}
	if got := stringValue(restarted.State["restartRequestedReason"]); got != "operator requested rebinding" {
		t.Fatalf("expected restartRequestedReason, got %s", got)
	}
	if got := stringValue(restarted.State["restartRequestedSource"]); got != "test" {
		t.Fatalf("expected restartRequestedSource test, got %s", got)
	}
	if got := stringValue(restarted.State["restartRequestedAt"]); got == "" {
		t.Fatal("expected restartRequestedAt")
	}
	if _, err := platform.StopSignalRuntimeSessionWithForce(runtimeSession.ID, true); err != nil {
		t.Fatalf("cleanup runtime failed: %v", err)
	}
}

func TestRestartSignalRuntimeSessionWithoutForceKeepsRunningWhenExposureExists(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	runtimeSession = markSignalRuntimeSessionRunningForTest(t, platform, runtimeSession)
	if _, err := platform.store.CreateOrder(domain.Order{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "LIMIT",
		Quantity:          0.002,
		Price:             70000,
	}); err != nil {
		t.Fatalf("seed order failed: %v", err)
	}

	if _, err := platform.RestartSignalRuntimeSession(runtimeSession.ID); !errors.Is(err, ErrActivePositionsOrOrders) {
		t.Fatalf("expected active exposure error, got %v", err)
	}
	stored, err := platform.GetSignalRuntimeSession(runtimeSession.ID)
	if err != nil {
		t.Fatalf("GetSignalRuntimeSession failed: %v", err)
	}
	if stored.Status != "RUNNING" {
		t.Fatalf("expected blocked restart to keep runtime RUNNING, got %s", stored.Status)
	}
	if got := stringValue(stored.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected blocked restart to keep desiredStatus RUNNING, got %s", got)
	}
	if got := stringValue(stored.State["restartRequestedAt"]); got != "" {
		t.Fatalf("expected blocked restart to avoid audit mutation, got %s", got)
	}
	if _, err := platform.StopSignalRuntimeSessionWithForce(runtimeSession.ID, true); err != nil {
		t.Fatalf("cleanup runtime failed: %v", err)
	}
}

func TestSignalRuntimeAutoRestartSuppressAndResumeAuditState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}
	now := time.Now().UTC()
	runtimeSession.State["desiredStatus"] = "RUNNING"
	runtimeSession.State["actualStatus"] = "ERROR"
	runtimeSession.State["autoRestartSuppressed"] = true
	runtimeSession.State["nextAutoRestartAt"] = now.Add(time.Minute).Format(time.RFC3339)
	runtimeSession.State["supervisorRestartBackoff"] = time.Minute.String()
	runtimeSession.State["supervisorRestartSeverity"] = "fatal"
	runtimeSession.State["lastSupervisorError"] = "auth failed"
	updated, err := platform.store.UpdateSignalRuntimeSession(runtimeSession)
	if err != nil {
		t.Fatalf("UpdateSignalRuntimeSession failed: %v", err)
	}
	platform.cacheSignalRuntimeSession(updated)

	suppressed, err := platform.SuppressSignalRuntimeAutoRestart(runtimeSession.ID, SignalRuntimeAutoRestartControlOptions{
		Reason: "operator paused runtime recovery during maintenance",
		Source: "test",
	})
	if err != nil {
		t.Fatalf("SuppressSignalRuntimeAutoRestart failed: %v", err)
	}
	if got := boolValue(suppressed.State["autoRestartSuppressed"]); !got {
		t.Fatal("expected autoRestartSuppressed true")
	}
	if got := stringValue(suppressed.State["autoRestartSuppressedReason"]); got != "operator paused runtime recovery during maintenance" {
		t.Fatalf("expected suppress reason, got %s", got)
	}
	if got := stringValue(suppressed.State["autoRestartSuppressedSource"]); got != "test" {
		t.Fatalf("expected suppress source test, got %s", got)
	}
	if got := stringValue(suppressed.State["autoRestartSuppressedAt"]); got == "" {
		t.Fatal("expected autoRestartSuppressedAt")
	}
	if got := stringValue(suppressed.State["nextAutoRestartAt"]); got != "" {
		t.Fatalf("expected suppress to clear nextAutoRestartAt, got %s", got)
	}
	if got := stringValue(suppressed.State["supervisorRestartBackoff"]); got != "" {
		t.Fatalf("expected suppress to clear supervisorRestartBackoff, got %s", got)
	}
	if got := stringValue(suppressed.State["lastSupervisorError"]); got != "auth failed" {
		t.Fatalf("expected suppress to preserve lastSupervisorError, got %s", got)
	}

	resumed, err := platform.ResumeSignalRuntimeAutoRestart(runtimeSession.ID, SignalRuntimeAutoRestartControlOptions{
		Reason: "maintenance finished and credentials rotated",
		Source: "test",
	})
	if err != nil {
		t.Fatalf("ResumeSignalRuntimeAutoRestart failed: %v", err)
	}
	if got := boolValue(resumed.State["autoRestartSuppressed"]); got {
		t.Fatal("expected autoRestartSuppressed false after resume")
	}
	if got := stringValue(resumed.State["autoRestartSuppressedAt"]); got != "" {
		t.Fatalf("expected resume to clear autoRestartSuppressedAt, got %s", got)
	}
	if got := stringValue(resumed.State["supervisorRestartSeverity"]); got != "" {
		t.Fatalf("expected resume to clear supervisorRestartSeverity, got %s", got)
	}
	if got := stringValue(resumed.State["autoRestartResumedReason"]); got != "maintenance finished and credentials rotated" {
		t.Fatalf("expected resume reason, got %s", got)
	}
	if got := stringValue(resumed.State["autoRestartResumedSource"]); got != "test" {
		t.Fatalf("expected resume source test, got %s", got)
	}
	if got := stringValue(resumed.State["autoRestartResumedAt"]); got == "" {
		t.Fatal("expected autoRestartResumedAt")
	}
	if got := stringValue(resumed.State["lastSupervisorError"]); got != "auth failed" {
		t.Fatalf("expected resume to preserve lastSupervisorError for audit, got %s", got)
	}
}

func markSignalRuntimeSessionRunningForTest(t *testing.T, platform *Platform, session domain.SignalRuntimeSession) domain.SignalRuntimeSession {
	t.Helper()
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["health"] = "healthy"
	state["desiredStatus"] = "RUNNING"
	state["actualStatus"] = "RUNNING"
	state["lastHeartbeatAt"] = now.Format(time.RFC3339)
	session.Status = "RUNNING"
	session.State = state
	session.UpdatedAt = now
	updated, err := platform.store.UpdateSignalRuntimeSession(session)
	if err != nil {
		t.Fatalf("UpdateSignalRuntimeSession failed: %v", err)
	}
	platform.cacheSignalRuntimeSession(updated)
	return updated
}

type blockingSignalRuntimeStore struct {
	*memory.Store
	entered        chan struct{}
	release        chan struct{}
	enteredOnce    sync.Once
	runningUpdates atomic.Int32
}

func (s *blockingSignalRuntimeStore) UpdateSignalRuntimeSession(session domain.SignalRuntimeSession) (domain.SignalRuntimeSession, error) {
	if session.Status == "RUNNING" &&
		stringValue(session.State["health"]) == "healthy" &&
		stringValue(session.State["actualStatus"]) == "STARTING" &&
		stringValue(mapValue(session.State["lastEventSummary"])["type"]) == "runtime_started" {
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

type countingRuntimeLeaseStore struct {
	*memory.Store
	releaseCalls atomic.Int32
}

func (s *countingRuntimeLeaseStore) ReleaseRuntimeLease(resourceType, resourceID, ownerID string) (bool, error) {
	s.releaseCalls.Add(1)
	return s.Store.ReleaseRuntimeLease(resourceType, resourceID, ownerID)
}
