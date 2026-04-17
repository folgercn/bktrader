package service

import (
	"errors"
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
	if _, err := platform.store.GetLiveSession("live-session-main"); err == nil {
		t.Fatal("expected live session to be deleted after force delete")
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
	if !boolValue(order.Metadata["reduceOnly"]) {
		t.Fatal("expected close order to be reduceOnly")
	}
	if got := stringValue(order.Metadata["positionId"]); got != position.ID {
		t.Fatalf("expected close order to reference position %s, got %s", position.ID, got)
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
