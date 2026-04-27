package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestHasActivePositionsOrOrdersMatchesStrategyScopedExposure(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	active, err := platform.HasActivePositionsOrOrders("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("HasActivePositionsOrOrders returned error: %v", err)
	}
	if !active {
		t.Fatal("expected active exposure to be detected")
	}

	active, err = platform.HasActivePositionsOrOrders("live-main", "strategy-does-not-exist")
	if err != nil {
		t.Fatalf("HasActivePositionsOrOrders for unrelated strategy returned error: %v", err)
	}
	if active {
		t.Fatal("expected unrelated strategy lookup to stay inactive")
	}
}

func TestStopLiveSessionWithForceRequiresForceWhenExposureExists(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	if _, err := platform.StopLiveSessionWithForce("live-session-main", false); !errors.Is(err, ErrActivePositionsOrOrders) {
		t.Fatalf("expected active exposure error, got %v", err)
	}
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if session.Status != "READY" {
		t.Fatalf("expected blocked stop to leave session READY, got %s", session.Status)
	}

	session, err = platform.StopLiveSessionWithForce("live-session-main", true)
	if err != nil {
		t.Fatalf("force stop live session failed: %v", err)
	}
	if session.Status != "STOPPED" {
		t.Fatalf("expected STOPPED after force stop, got %s", session.Status)
	}
}

func TestDeleteLiveSessionWithForceRequiresForceWhenExposureExists(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
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

	if err := platform.DeleteLiveSessionWithForce("live-session-main", false); !errors.Is(err, ErrActivePositionsOrOrders) {
		t.Fatalf("expected active exposure error, got %v", err)
	}
	if _, err := platform.store.GetLiveSession("live-session-main"); err != nil {
		t.Fatalf("expected live session to remain after blocked delete, got %v", err)
	}

	if err := platform.DeleteLiveSessionWithForce("live-session-main", true); err != nil {
		t.Fatalf("force delete live session failed: %v", err)
	}
	deleted, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("expected soft-deleted live session to remain loadable, got %v", err)
	}
	if deleted.Status != "DELETED" {
		t.Fatalf("expected live session status DELETED after force delete, got %s", deleted.Status)
	}
	if got := stringValue(deleted.State["deletedAt"]); got == "" {
		t.Fatal("expected soft-deleted live session to record deletedAt")
	}
	items, err := platform.ListLiveSessions()
	if err != nil {
		t.Fatalf("list live sessions failed: %v", err)
	}
	for _, item := range items {
		if item.ID == deleted.ID {
			t.Fatalf("expected deleted live session %s to be hidden from default list", deleted.ID)
		}
	}
}

func TestStopLiveSessionWithForceStopsLinkedRuntimeWhenLiveAlreadyStopped(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.UpdateLiveSessionStatus("live-session-main", "STOPPED")
	if err != nil {
		t.Fatalf("set live session stopped failed: %v", err)
	}
	runtime := mustCreateLinkedLiveRuntime(t, platform, session, "RUNNING", "RUNNING")

	stopped, err := platform.StopLiveSessionWithForce(session.ID, false)
	if err != nil {
		t.Fatalf("stop already-stopped live session failed: %v", err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("expected live session to remain STOPPED, got %s", stopped.Status)
	}
	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected linked runtime STOPPED, got %s", updatedRuntime.Status)
	}
	if got := stringValue(updatedRuntime.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected linked runtime desiredStatus STOPPED, got %s", got)
	}
	updatedSession, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if got := stringValue(updatedSession.State["signalRuntimeStatus"]); got != "STOPPED" {
		t.Fatalf("expected live session signalRuntimeStatus STOPPED, got %s", got)
	}
}

func TestDeleteLiveSessionWithForceStopsLinkedRuntimeBeforeDelete(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	runtime := mustCreateLinkedLiveRuntime(t, platform, session, "RUNNING", "RUNNING")

	if err := platform.DeleteLiveSessionWithForce(session.ID, false); err != nil {
		t.Fatalf("delete live session failed: %v", err)
	}
	deleted, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("expected soft-deleted live session to remain loadable, got %v", err)
	}
	if deleted.Status != "DELETED" {
		t.Fatalf("expected live session status DELETED, got %s", deleted.Status)
	}
	if got := stringValue(deleted.State["deletedAt"]); got == "" {
		t.Fatal("expected deletedAt to be set")
	}
	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected linked runtime STOPPED before live delete, got %s", updatedRuntime.Status)
	}
	if got := stringValue(updatedRuntime.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected linked runtime desiredStatus STOPPED, got %s", got)
	}
}

func TestDeleteLiveSessionWithForceKeepsSessionWhenLinkedRuntimeStopFails(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeSessionId"] = "signal-runtime-missing"
	if _, err := platform.store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("seed linked runtime id failed: %v", err)
	}

	if err := platform.DeleteLiveSessionWithForce(session.ID, true); err == nil {
		t.Fatal("expected delete to fail when linked runtime stop fails")
	}
	if _, err := platform.store.GetLiveSession(session.ID); err != nil {
		t.Fatalf("expected live session to remain after failed linked runtime stop, got %v", err)
	}
}

func TestRecoverLiveTradingOnStartupSkipsLinkedRuntimeDesiredStopped(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	runtime := mustCreateLinkedLiveRuntime(t, platform, session, "STOPPED", "STOPPED")

	platform.RecoverLiveTradingOnStartup(context.Background())

	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected runtime to stay STOPPED, got %s", updatedRuntime.Status)
	}
	if got := stringValue(updatedRuntime.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected desiredStatus STOPPED, got %s", got)
	}
	updatedSession, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if got := stringValue(updatedSession.State["lastRecoveryError"]); got != "" {
		t.Fatalf("expected no lastRecoveryError for skipped desired-stopped runtime, got %s", got)
	}
	if got := stringValue(updatedSession.State["lastRecoveryStatus"]); got != "skipped-runtime-desired-stopped" {
		t.Fatalf("expected skipped recovery status, got %s", got)
	}
}

func TestRecoverLiveTradingOnStartupTreatsRuntimeLeaseRaceAsTransient(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.setRuntimeLeaseOwnerIDForTest("runner-local")
	session, err := platform.store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	runtime := mustCreateLinkedLiveRuntime(t, platform, session, "STOPPED", "RUNNING")
	state := cloneMetadata(session.State)
	state["signalRuntimeSessionId"] = runtime.ID
	state["lastRecoveryError"] = "old critical error"
	if _, err := platform.store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("seed live recovery state failed: %v", err)
	}
	if _, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   runtime.ID,
		OwnerID:      "runner-other",
		TTL:          time.Minute,
	}); err != nil || !ok {
		t.Fatalf("pre-acquire runtime lease failed: ok=%v err=%v", ok, err)
	}

	platform.RecoverLiveTradingOnStartup(context.Background())

	updatedSession, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if got := stringValue(updatedSession.State["lastRecoveryError"]); got != "" {
		t.Fatalf("expected transient lease race to clear lastRecoveryError, got %s", got)
	}
	if got := stringValue(updatedSession.State["lastRecoveryStatus"]); got != "lease-not-acquired" {
		t.Fatalf("expected lease-not-acquired recovery status, got %s", got)
	}
	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected runtime to remain STOPPED after lease race, got %s", updatedRuntime.Status)
	}
}

func TestLiveSessionControlRejectsConcurrentSameAccountStrategyOperations(t *testing.T) {
	store := &blockingLiveSessionStatusStore{
		Store:       memory.NewStore(),
		blockStatus: "STOPPED",
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	platform := NewPlatform(store)

	stopDone := make(chan error, 1)
	go func() {
		_, err := platform.StopLiveSessionWithForce("live-session-main", false)
		stopDone <- err
	}()
	<-store.entered

	if err := platform.DeleteLiveSessionWithForce("live-session-main", true); !errors.Is(err, ErrLiveControlOperationInProgress) {
		close(store.release)
		t.Fatalf("expected concurrent delete to return control operation error, got %v", err)
	}
	close(store.release)
	if err := <-stopDone; err != nil {
		t.Fatalf("stop operation should complete after release: %v", err)
	}
}

func TestSignalRuntimeStartReturnsConflictDuringLiveControlOperation(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	runtime := mustCreateLinkedLiveRuntime(t, platform, session, "STOPPED", "RUNNING")
	if _, err := platform.StartSignalRuntimeSession(runtime.ID); err != nil {
		t.Fatalf("start runtime failed: %v", err)
	}

	requested := liveControlOperationInfo{
		Operation:     liveControlOperationStop,
		AccountID:     session.AccountID,
		StrategyID:    session.StrategyID,
		LiveSessionID: session.ID,
	}
	release, acquired, current := platform.tryStartLiveControlOperation(requested)
	if !acquired {
		t.Fatalf("acquire live control operation failed: %v", liveControlOperationInProgressError(requested, current))
	}
	_, err = platform.StartSignalRuntimeSession(runtime.ID)
	release()
	if !errors.Is(err, ErrLiveControlOperationInProgress) {
		t.Fatalf("expected runtime start to return live control conflict, got %v", err)
	}
	if _, stopErr := platform.StopSignalRuntimeSessionWithForce(runtime.ID, true); stopErr != nil {
		t.Fatalf("cleanup runtime failed: %v", stopErr)
	}
}

func TestSignalRuntimeSessionForceActionsRespectSafetyLock(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	runtime := domain.SignalRuntimeSession{
		ID:         "signal-runtime-test",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "RUNNING",
		State:      map[string]any{},
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	platform.signalSessions[runtime.ID] = runtime
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	if _, err := platform.StopSignalRuntimeSessionWithForce(runtime.ID, false); !errors.Is(err, ErrActivePositionsOrOrders) {
		t.Fatalf("expected active exposure error for stop, got %v", err)
	}
	stopped, err := platform.StopSignalRuntimeSessionWithForce(runtime.ID, true)
	if err != nil {
		t.Fatalf("force stop signal runtime session failed: %v", err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("expected STOPPED status after force stop, got %s", stopped.Status)
	}

	platform.signalSessions[runtime.ID] = runtime
	if err := platform.DeleteSignalRuntimeSessionWithForce(runtime.ID, false); !errors.Is(err, ErrActivePositionsOrOrders) {
		t.Fatalf("expected active exposure error for delete, got %v", err)
	}
	if err := platform.DeleteSignalRuntimeSessionWithForce(runtime.ID, true); err != nil {
		t.Fatalf("force delete signal runtime session failed: %v", err)
	}
	if _, err := platform.GetSignalRuntimeSession(runtime.ID); err == nil {
		t.Fatal("expected signal runtime session to be deleted after force delete")
	}
}

func TestStopLiveFlowWithForceSucceedsWhenLiveStopAlreadyStoppedLinkedRuntime(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}

	session, err := platform.store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	runtime, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime failed: %v", err)
	}
	if _, err := platform.StartSignalRuntimeSession(runtime.ID); err != nil {
		t.Fatalf("start runtime failed: %v", err)
	}
	if _, err := platform.store.UpdateLiveSessionState(session.ID, map[string]any{
		"signalRuntimeSessionId": runtime.ID,
	}); err != nil {
		t.Fatalf("link runtime into live session state failed: %v", err)
	}

	result, err := platform.StopLiveFlowWithForce("live-main", false)
	if err != nil {
		t.Fatalf("StopLiveFlowWithForce failed: %v", err)
	}
	if len(result.StoppedLiveSessionIDs) != 1 || result.StoppedLiveSessionIDs[0] != session.ID {
		t.Fatalf("expected stopped live session %s, got %#v", session.ID, result.StoppedLiveSessionIDs)
	}
	if len(result.StoppedRuntimeSessionIDs) != 1 || result.StoppedRuntimeSessionIDs[0] != runtime.ID {
		t.Fatalf("expected stopped runtime session %s, got %#v", runtime.ID, result.StoppedRuntimeSessionIDs)
	}

	updatedSession, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("reload live session failed: %v", err)
	}
	if updatedSession.Status != "STOPPED" {
		t.Fatalf("expected live session STOPPED, got %s", updatedSession.Status)
	}
	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("reload runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected runtime STOPPED, got %s", updatedRuntime.Status)
	}
}

func mustCreateLinkedLiveRuntime(t *testing.T, platform *Platform, session domain.LiveSession, status, desiredStatus string) domain.SignalRuntimeSession {
	t.Helper()
	if _, err := platform.BindStrategySignalSource(session.StrategyID, map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	runtime, err := platform.CreateSignalRuntimeSession(session.AccountID, session.StrategyID)
	if err != nil {
		t.Fatalf("create runtime failed: %v", err)
	}
	now := time.Now().UTC()
	runtime.Status = status
	runtime.State = cloneMetadata(runtime.State)
	runtime.State["desiredStatus"] = desiredStatus
	runtime.State["actualStatus"] = status
	runtime.State["health"] = strings.ToLower(status)
	runtime.UpdatedAt = now
	updatedRuntime, err := platform.store.UpdateSignalRuntimeSession(runtime)
	if err != nil {
		t.Fatalf("update runtime failed: %v", err)
	}
	platform.cacheSignalRuntimeSession(updatedRuntime)
	state := cloneMetadata(session.State)
	state["signalRuntimeSessionId"] = updatedRuntime.ID
	state["signalRuntimeStatus"] = updatedRuntime.Status
	if _, err := platform.store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("link runtime into live session failed: %v", err)
	}
	return updatedRuntime
}

type blockingLiveSessionStatusStore struct {
	*memory.Store
	blockStatus string
	entered     chan struct{}
	release     chan struct{}
	once        sync.Once
}

func (s *blockingLiveSessionStatusStore) UpdateLiveSessionStatus(sessionID, status string) (domain.LiveSession, error) {
	if strings.EqualFold(status, s.blockStatus) {
		s.once.Do(func() { close(s.entered) })
		<-s.release
	}
	return s.Store.UpdateLiveSessionStatus(sessionID, status)
}

func TestClosePositionCreatesReduceOnlyMarketOrder(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Paper Close", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		EntryPrice:        68000,
		MarkPrice:         68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	order, err := platform.ClosePosition(position.ID)
	if err != nil {
		t.Fatalf("ClosePosition failed: %v", err)
	}
	if order.Side != "SELL" {
		t.Fatalf("expected SELL side for closing LONG position, got %s", order.Side)
	}
	if order.Type != "MARKET" {
		t.Fatalf("expected MARKET close order, got %s", order.Type)
	}
	if !order.ReduceOnly {
		t.Fatal("expected close order to set the formal ReduceOnly field")
	}
	if !boolValue(order.Metadata["reduceOnly"]) {
		t.Fatal("expected close order to be reduceOnly")
	}
	if got := stringValue(order.Metadata["positionId"]); got != position.ID {
		t.Fatalf("expected close order to reference position %s, got %s", position.ID, got)
	}
	if got := parseFloatValue(order.Metadata["priceHint"]); got != 68100 {
		t.Fatalf("expected close order to preserve priceHint 68100, got %v", got)
	}
	if got := stringValue(order.Status); got != "FILLED" {
		t.Fatalf("expected paper close order to be FILLED, got %s", got)
	}
	if _, exists, err := platform.store.FindPosition(account.ID, "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if exists {
		t.Fatal("expected close order to flatten the paper position")
	}
}

func TestBuildClosePositionOrderUsesMetadataPriceHintForMarketClose(t *testing.T) {
	order := buildClosePositionOrder(domain.Position{
		ID:                "position-test-close",
		AccountID:         "account-test-close",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		MarkPrice:         68100,
	})

	if order.Type != "MARKET" {
		t.Fatalf("expected MARKET close order, got %s", order.Type)
	}
	if order.Price != 0 {
		t.Fatalf("expected close MARKET order to leave explicit price empty, got %v", order.Price)
	}
	if got := parseFloatValue(order.Metadata["priceHint"]); got != 68100 {
		t.Fatalf("expected close MARKET order priceHint 68100, got %v", got)
	}
	if got := parseFloatValue(order.Metadata["markPrice"]); got != 68100 {
		t.Fatalf("expected close MARKET order markPrice metadata 68100, got %v", got)
	}
}

func TestResolveClosePositionTargetRefreshesLivePositionBeforeSubmitting(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		EntryPrice:        68000,
		MarkPrice:         68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-close-refresh",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			refreshed, found, err := p.findPositionByID(position.ID)
			if err != nil {
				return domain.Account{}, err
			}
			if !found {
				return domain.Account{}, errors.New("position disappeared during refresh")
			}
			refreshed.Quantity = 0.1
			refreshed.MarkPrice = 68200
			if _, err := p.store.SavePosition(refreshed); err != nil {
				return domain.Account{}, err
			}
			return account, nil
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-close-refresh",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	target, _, err := platform.resolveClosePositionTarget(position.ID)
	if err != nil {
		t.Fatalf("resolveClosePositionTarget failed: %v", err)
	}
	if target.Quantity != 0.1 {
		t.Fatalf("expected refreshed close quantity 0.1, got %v", target.Quantity)
	}
	if target.MarkPrice != 68200 {
		t.Fatalf("expected refreshed close markPrice 68200, got %v", target.MarkPrice)
	}
}

func TestResolveClosePositionTargetFailsWhenPositionDisappearsAfterRefresh(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		EntryPrice:        68000,
		MarkPrice:         68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-close-disappear",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			if err := p.store.DeletePosition(position.ID); err != nil {
				return domain.Account{}, err
			}
			return account, nil
		},
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-close-disappear",
		"connectionMode": "mock",
		"executionMode":  "mock",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	if _, _, err := platform.resolveClosePositionTarget(position.ID); err == nil || !strings.Contains(err.Error(), "position not found") {
		t.Fatalf("expected refreshed close target lookup to fail after position disappears, got %v", err)
	}
}

func TestEnsureNoActivePositionsOrOrdersKeepsStaleLiveExposureBlockedWhenReconcileOnlyFindsHistoricalExternalOrders(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	syncedAt := time.Date(2026, 4, 20, 12, 33, 23, 0, time.UTC)
	configureTestLiveRESTReconcileHistoryAdapter(
		t,
		platform,
		"test-active-exposure-self-heal",
		[]map[string]any{},
		map[string][]map[string]any{
			"BTCUSDT": {{
				"symbol":        "BTCUSDT",
				"orderId":       "9101",
				"clientOrderId": "client-9101",
				"status":        "FILLED",
				"side":          "SELL",
				"type":          "MARKET",
				"origType":      "MARKET",
				"origQty":       0.01,
				"executedQty":   0.01,
				"price":         67950.0,
				"avgPrice":      67950.0,
				"reduceOnly":    true,
				"closePosition": false,
				"time":          float64(syncedAt.Add(-2 * time.Minute).UnixMilli()),
				"updateTime":    float64(syncedAt.UnixMilli()),
			}},
		},
		map[string][]LiveFillReport{
			"BTCUSDT": {{
				Price:    67950.0,
				Quantity: 0.01,
				Fee:      0.01,
				Metadata: map[string]any{
					"exchangeOrderId": "9101",
					"tradeId":         "trade-9101",
					"tradeTime":       syncedAt.Format(time.RFC3339),
				},
			}},
		},
	)
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67950,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	if err := platform.ensureNoActivePositionsOrOrders("live-main", "strategy-bk-1d"); err == nil || !strings.Contains(err.Error(), "活动中的订单或未平仓头寸") {
		t.Fatalf("expected stale live exposure to remain and block session cleanup, got %v", err)
	}
	if _, found, err := platform.store.FindPosition("live-main", "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if !found {
		t.Fatal("expected stale BTCUSDT position to remain until manual review")
	}
	active, err := platform.HasActivePositionsOrOrders("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("HasActivePositionsOrOrders after blocked reconcile returned error: %v", err)
	}
	if !active {
		t.Fatal("expected active exposure to remain until manual review")
	}
}

func TestClosePositionKeepsStaleLivePositionBlockedWhenReconcileOnlyFindsHistoricalExternalOrders(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	syncedAt := time.Date(2026, 4, 20, 12, 33, 23, 0, time.UTC)
	configureTestLiveRESTReconcileHistoryAdapter(
		t,
		platform,
		"test-close-self-heal",
		[]map[string]any{},
		map[string][]map[string]any{
			"BTCUSDT": {{
				"symbol":        "BTCUSDT",
				"orderId":       "9102",
				"clientOrderId": "client-9102",
				"status":        "FILLED",
				"side":          "SELL",
				"type":          "MARKET",
				"origType":      "MARKET",
				"origQty":       0.01,
				"executedQty":   0.01,
				"price":         67950.0,
				"avgPrice":      67950.0,
				"reduceOnly":    true,
				"closePosition": false,
				"time":          float64(syncedAt.Add(-2 * time.Minute).UnixMilli()),
				"updateTime":    float64(syncedAt.UnixMilli()),
			}},
		},
		map[string][]LiveFillReport{
			"BTCUSDT": {{
				Price:    67950.0,
				Quantity: 0.01,
				Fee:      0.01,
				Metadata: map[string]any{
					"exchangeOrderId": "9102",
					"tradeId":         "trade-9102",
					"tradeTime":       syncedAt.Format(time.RFC3339),
				},
			}},
		},
	)
	position, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67950,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	if _, err := platform.ClosePosition(position.ID); err == nil || !strings.Contains(err.Error(), "reconcile gate") {
		t.Fatalf("expected ClosePosition to stay blocked by reconcile gate, got %v", err)
	}
	if _, found, err := platform.store.FindPosition("live-main", "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if !found {
		t.Fatal("expected stale BTCUSDT position to remain until manual review")
	}
}

func TestCreateOrderReduceOnlyFormalFieldPreventsReverseOpen(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Paper ReduceOnly", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		EntryPrice:        68000,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.1,
		ReduceOnly:        true,
	})
	if err != nil {
		t.Fatalf("CreateOrder reduce-only failed: %v", err)
	}
	if !order.ReduceOnly {
		t.Fatal("expected returned order to preserve ReduceOnly field")
	}
	if !boolValue(order.Metadata["reduceOnly"]) {
		t.Fatal("expected returned order metadata to preserve reduceOnly")
	}
	position, found, err := platform.store.FindPosition(account.ID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected partial reduce-only execution to leave a remaining position")
	}
	if position.Side != "LONG" || position.Quantity != 0.15 {
		t.Fatalf("expected remaining LONG 0.15 after partial reduce-only close, got side=%s qty=%v", position.Side, position.Quantity)
	}
}

func TestResolveReduceOnlyTargetPositionScopesSharedSymbolByStrategyVersionOrPositionID(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Paper Shared Symbol", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	fourHour, err := platform.store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-4h-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.05,
		EntryPrice:        68000,
		MarkPrice:         68100,
	})
	if err != nil {
		t.Fatalf("save 4h position failed: %v", err)
	}
	oneDay, err := platform.store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		EntryPrice:        68200,
		MarkPrice:         68300,
	})
	if err != nil {
		t.Fatalf("save 1d position failed: %v", err)
	}

	position, found, err := platform.resolveReduceOnlyTargetPosition(account.ID, domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: oneDay.StrategyVersionID,
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		ReduceOnly:        true,
	})
	if err != nil {
		t.Fatalf("resolveReduceOnlyTargetPosition by strategyVersionID failed: %v", err)
	}
	if !found || position.ID != oneDay.ID {
		t.Fatalf("expected strategy-scoped reduce-only target %s, got found=%t id=%s", oneDay.ID, found, position.ID)
	}

	position, found, err = platform.resolveReduceOnlyTargetPosition(account.ID, domain.Order{
		AccountID:  account.ID,
		Symbol:     "BTCUSDT",
		Side:       "SELL",
		ReduceOnly: true,
		Metadata: map[string]any{
			"positionId": fourHour.ID,
		},
	})
	if err != nil {
		t.Fatalf("resolveReduceOnlyTargetPosition by positionId failed: %v", err)
	}
	if !found || position.ID != fourHour.ID {
		t.Fatalf("expected position-scoped reduce-only target %s, got found=%t id=%s", fourHour.ID, found, position.ID)
	}
}

func TestResolveReduceOnlyTargetPositionRejectsAmbiguousSharedSymbolWithoutIdentity(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Paper Shared Symbol Ambiguous", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	for _, strategyVersionID := range []string{"strategy-version-bk-4h-v010", "strategy-version-bk-1d-v010"} {
		if _, err := platform.store.SavePosition(domain.Position{
			AccountID:         account.ID,
			StrategyVersionID: strategyVersionID,
			Symbol:            "BTCUSDT",
			Side:              "LONG",
			Quantity:          0.1,
			EntryPrice:        68000,
			MarkPrice:         68100,
		}); err != nil {
			t.Fatalf("seed position for %s failed: %v", strategyVersionID, err)
		}
	}

	if _, _, err := platform.resolveReduceOnlyTargetPosition(account.ID, domain.Order{
		AccountID:  account.ID,
		Symbol:     "BTCUSDT",
		Side:       "SELL",
		ReduceOnly: true,
	}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous shared-symbol reduce-only lookup to be rejected, got %v", err)
	}
}

func TestCreateOrderReduceOnlyRejectsOversizedQuantity(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Paper ReduceOnly Oversize", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		EntryPrice:        68000,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	if _, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.5,
		ReduceOnly:        true,
	}); err == nil || !strings.Contains(err.Error(), "exceeds open position quantity") {
		t.Fatalf("expected oversized reduce-only order to be rejected, got %v", err)
	}

	position, found, err := platform.store.FindPosition(account.ID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found || position.Side != "LONG" || position.Quantity != 0.25 {
		t.Fatalf("expected original LONG 0.25 position to remain untouched, got found=%t side=%s qty=%v", found, position.Side, position.Quantity)
	}
}

func TestNormalizeRESTOrderRejectsReduceOnlyQuantityExpansion(t *testing.T) {
	adapter := binanceFuturesLiveAdapter{}
	creds := binanceRESTCredentials{BaseURL: "https://example.test"}
	cacheKey := creds.BaseURL + "|BTCUSDT"
	binanceSymbolRulesCacheMu.Lock()
	previous, existed := binanceSymbolRulesCache[cacheKey]
	binanceSymbolRulesCacheMu.Unlock()
	t.Cleanup(func() {
		binanceSymbolRulesCacheMu.Lock()
		defer binanceSymbolRulesCacheMu.Unlock()
		if existed {
			binanceSymbolRulesCache[cacheKey] = previous
		} else {
			delete(binanceSymbolRulesCache, cacheKey)
		}
	})
	binanceSymbolRulesCacheMu.Lock()
	binanceSymbolRulesCache[cacheKey] = binanceSymbolRules{
		Symbol:      "BTCUSDT",
		TickSize:    0.1,
		StepSize:    0.001,
		MinQty:      0.001,
		MaxQty:      1000,
		MinNotional: 100,
		UpdatedAt:   time.Now().UTC(),
	}
	binanceSymbolRulesCacheMu.Unlock()

	if _, _, err := adapter.normalizeRESTOrder(domain.Order{
		Symbol:     "BTCUSDT",
		Type:       "MARKET",
		Quantity:   0.0005,
		ReduceOnly: true,
	}, creds); err == nil || !strings.Contains(err.Error(), "reduce-only order quantity") {
		t.Fatalf("expected reduce-only REST normalization to reject quantity expansion, got %v", err)
	}
}

func TestNormalizeRESTOrderAllowsReduceOnlyExactStepQuantity(t *testing.T) {
	adapter := binanceFuturesLiveAdapter{}
	creds := binanceRESTCredentials{BaseURL: "https://example.test"}
	cacheKey := creds.BaseURL + "|BTCUSDT"
	binanceSymbolRulesCacheMu.Lock()
	previous, existed := binanceSymbolRulesCache[cacheKey]
	binanceSymbolRulesCacheMu.Unlock()
	t.Cleanup(func() {
		binanceSymbolRulesCacheMu.Lock()
		defer binanceSymbolRulesCacheMu.Unlock()
		if existed {
			binanceSymbolRulesCache[cacheKey] = previous
		} else {
			delete(binanceSymbolRulesCache, cacheKey)
		}
	})
	binanceSymbolRulesCacheMu.Lock()
	binanceSymbolRulesCache[cacheKey] = binanceSymbolRules{
		Symbol:      "BTCUSDT",
		TickSize:    0.1,
		StepSize:    0.0001,
		MinQty:      0.0001,
		MaxQty:      1000,
		MinNotional: 100,
		UpdatedAt:   time.Now().UTC(),
	}
	binanceSymbolRulesCacheMu.Unlock()

	normalized, _, err := adapter.normalizeRESTOrder(domain.Order{
		Symbol:     "BTCUSDT",
		Type:       "MARKET",
		Quantity:   0.013,
		ReduceOnly: true,
	}, creds)
	if err != nil {
		t.Fatalf("expected exact-step reduce-only quantity to normalize cleanly, got %v", err)
	}
	if math.Abs(normalized.Quantity-0.013) > 1e-12 {
		t.Fatalf("expected normalized quantity to stay at 0.013, got %.18f", normalized.Quantity)
	}
	normalization := mapValue(normalized.Metadata["normalization"])
	if normalization == nil {
		t.Fatal("expected normalization metadata to be recorded")
	}
	if boolValue(normalization["stepSizeAdjusted"]) {
		t.Fatalf("expected exact-step reduce-only quantity to avoid phantom step-size adjustment, got %#v", normalization)
	}
	if boolValue(normalization["minQtyAdjusted"]) {
		t.Fatalf("expected exact-step reduce-only quantity to avoid minQty adjustment, got %#v", normalization)
	}
}

func TestNormalizeRESTOrderAvoidsPhantomExactTickAdjustments(t *testing.T) {
	adapter := binanceFuturesLiveAdapter{}
	creds := binanceRESTCredentials{BaseURL: "https://example.test"}
	cacheKey := creds.BaseURL + "|BTCUSDT"
	binanceSymbolRulesCacheMu.Lock()
	previous, existed := binanceSymbolRulesCache[cacheKey]
	binanceSymbolRulesCacheMu.Unlock()
	t.Cleanup(func() {
		binanceSymbolRulesCacheMu.Lock()
		defer binanceSymbolRulesCacheMu.Unlock()
		if existed {
			binanceSymbolRulesCache[cacheKey] = previous
		} else {
			delete(binanceSymbolRulesCache, cacheKey)
		}
	})
	binanceSymbolRulesCacheMu.Lock()
	binanceSymbolRulesCache[cacheKey] = binanceSymbolRules{
		Symbol:      "BTCUSDT",
		TickSize:    0.1,
		StepSize:    0.0001,
		MinQty:      0.0001,
		MaxQty:      1000,
		MinNotional: 100,
		UpdatedAt:   time.Now().UTC(),
	}
	binanceSymbolRulesCacheMu.Unlock()

	normalized, _, err := adapter.normalizeRESTOrder(domain.Order{
		Symbol:   "BTCUSDT",
		Type:     "LIMIT",
		Quantity: 0.013,
		Price:    78168.3,
	}, creds)
	if err != nil {
		t.Fatalf("expected exact-step exact-tick limit order to normalize cleanly, got %v", err)
	}
	if math.Abs(normalized.Quantity-0.013) > 1e-12 {
		t.Fatalf("expected normalized quantity to stay at 0.013, got %.18f", normalized.Quantity)
	}
	if math.Abs(normalized.Price-78168.3) > 1e-9 {
		t.Fatalf("expected normalized price to stay at 78168.3, got %.18f", normalized.Price)
	}
	normalization := mapValue(normalized.Metadata["normalization"])
	if normalization == nil {
		t.Fatal("expected normalization metadata to be recorded")
	}
	if boolValue(normalization["stepSizeAdjusted"]) || boolValue(normalization["tickSizeAdjusted"]) || boolValue(normalization["normalizationApplied"]) {
		t.Fatalf("expected exact-step exact-tick order to avoid phantom adjustments, got %#v", normalization)
	}
}

func TestRequiredBinanceQuantityForMinNotionalKeepsBoundaryWithinFloatTolerance(t *testing.T) {
	rules := binanceSymbolRules{
		Symbol:      "BTCUSDT",
		StepSize:    0.0001,
		MinQty:      0.0001,
		MinNotional: 100,
	}

	required := requiredBinanceQuantityForMinNotional(0.006, 16666.666666666664, rules)
	if math.Abs(required-0.006) > 1e-12 {
		t.Fatalf("expected exact min-notional boundary quantity to remain unchanged, got %.18f", required)
	}
}
