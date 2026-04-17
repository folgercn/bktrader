package service

import (
	"errors"
	"strings"
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
