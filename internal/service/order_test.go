package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestBuildLiveSyncSettlementKeepsExchangeTradeIDEmptyWithoutRealTradeID(t *testing.T) {
	order := domain.Order{ID: "order-1", Symbol: "BTCUSDT"}

	fills, _, _ := buildLiveSyncSettlement(order, LiveOrderSync{
		Status: "FILLED",
		Fills: []LiveFillReport{{
			Price:    68000,
			Quantity: 0.1,
			Fee:      1.23,
			Metadata: map[string]any{
				"exchangeOrderId": "exchange-order-1",
				"tradeTime":       "2026-04-17T12:36:00Z",
			},
		}},
	})

	if len(fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(fills))
	}
	if got := fills[0].ExchangeTradeID; got != "" {
		t.Fatalf("expected missing real trade id to stay empty, got %q", got)
	}
	if fills[0].ExchangeTradeTime == nil || fills[0].ExchangeTradeTime.Format(time.RFC3339) != "2026-04-17T12:36:00Z" {
		t.Fatalf("expected exchange trade time to be captured, got %#v", fills[0].ExchangeTradeTime)
	}
}

func TestFinalizeExecutedOrderSkipsDuplicateExchangeTradeIDFills(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	order, err := store.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "MARKET",
		Quantity:          0.1,
		Price:             68000,
		Metadata: map[string]any{
			"executionMode": "live",
			"orderLifecycle": map[string]any{
				"submitted": true,
				"accepted":  true,
				"synced":    true,
				"filled":    false,
			},
			"exchangeOrderId": "exchange-order-1",
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	fill := domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: "trade-1",
		Price:           68000,
		Quantity:        0.1,
		Fee:             1.23,
	}

	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}
	if _, err := platform.finalizeExecutedOrder(account, filledOrder, []domain.Fill{fill}); err != nil {
		t.Fatalf("second finalize failed: %v", err)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	orderFillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			orderFillCount++
		}
	}
	if orderFillCount != 1 {
		t.Fatalf("expected duplicate sync to keep one fill, got %d", orderFillCount)
	}

	position, found, err := store.FindPosition(account.ID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected filled order to create a position")
	}
	if position.Quantity != 0.1 {
		t.Fatalf("expected duplicate sync to keep position quantity at 0.1, got %v", position.Quantity)
	}
}

func TestFinalizeExecutedOrderSkipsDuplicateFallbackFillsWithoutExchangeTradeID(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	tradeTime := time.Date(2026, 4, 17, 12, 36, 0, 0, time.UTC)
	fill := domain.Fill{
		OrderID:           order.ID,
		Price:             68000,
		Quantity:          0.1,
		Fee:               1.23,
		ExchangeTradeTime: &tradeTime,
	}

	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}
	if _, err := platform.finalizeExecutedOrder(account, filledOrder, []domain.Fill{fill}); err != nil {
		t.Fatalf("second finalize failed: %v", err)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	orderFillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			orderFillCount++
		}
	}
	if orderFillCount != 1 {
		t.Fatalf("expected duplicate fallback sync to keep one fill, got %d", orderFillCount)
	}
}

func TestFilledExitWithoutFillReportsDoesNotLeaveStaleShortPosition(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	entryOrder, err := store.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "LIMIT",
		Quantity:          0.002,
		Price:             75600.0,
		Metadata: map[string]any{
			"source":        "live-session-intent",
			"executionMode": "live",
		},
	})
	if err != nil {
		t.Fatalf("create entry order failed: %v", err)
	}
	if _, err := platform.finalizeExecutedOrder(account, entryOrder, []domain.Fill{{
		OrderID:         entryOrder.ID,
		ExchangeTradeID: "entry-trade-1",
		Price:           75600.0,
		Quantity:        0.002,
	}}); err != nil {
		t.Fatalf("finalize entry order failed: %v", err)
	}

	exitOrder, err := store.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "LIMIT",
		Quantity:          0.002,
		Price:             75600.1,
		ReduceOnly:        true,
		Status:            "ACCEPTED",
		Metadata: map[string]any{
			"source":          "live-session-intent",
			"executionMode":   "live",
			"exchangeOrderId": "exchange-exit-1",
			"executionProposal": map[string]any{
				"role":       "exit",
				"reason":     "SL",
				"signalKind": "risk-exit",
				"reduceOnly": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("create exit order failed: %v", err)
	}
	syncedExit, err := platform.applyLiveSyncResult(account, exitOrder, LiveOrderSync{
		Status:   "FILLED",
		SyncedAt: "2026-04-21T06:03:12Z",
		Metadata: map[string]any{
			"exchangeOrderId": "exchange-exit-1",
			"executedQty":     0.002,
			"avgPrice":        75600.1,
			"updateTime":      "2026-04-21T06:03:12Z",
		},
	})
	if err != nil {
		t.Fatalf("apply filled exit sync without fills failed: %v", err)
	}
	if got := parseFloatValue(syncedExit.Metadata["filledQuantity"]); got != 0.002 {
		t.Fatalf("expected fallback settlement to mark filled quantity 0.002, got %v", got)
	}
	if position, found, err := store.FindPosition(account.ID, "BTCUSDT"); err != nil {
		t.Fatalf("find position after exit failed: %v", err)
	} else if found {
		t.Fatalf("expected filled reduce-only exit to clear local short, got %+v", position)
	}

	if _, err := platform.finalizeExecutedOrder(account, syncedExit, []domain.Fill{{
		OrderID:         syncedExit.ID,
		ExchangeTradeID: "late-exit-trade-1",
		Price:           75600.1,
		Quantity:        0.002,
	}}); err != nil {
		t.Fatalf("late duplicate exit fill should be ignored: %v", err)
	}
	if position, found, err := store.FindPosition(account.ID, "BTCUSDT"); err != nil {
		t.Fatalf("find position after duplicate exit failed: %v", err)
	} else if found {
		t.Fatalf("expected late duplicate exit fill not to reopen/invert position, got %+v", position)
	}

	reentryOrder, err := store.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "LIMIT",
		Quantity:          0.002,
		Price:             75600.0,
		Metadata: map[string]any{
			"source":        "live-session-intent",
			"executionMode": "live",
		},
	})
	if err != nil {
		t.Fatalf("create reentry order failed: %v", err)
	}
	if _, err := platform.finalizeExecutedOrder(account, reentryOrder, []domain.Fill{{
		OrderID:         reentryOrder.ID,
		ExchangeTradeID: "reentry-trade-1",
		Price:           75600.0,
		Quantity:        0.002,
	}}); err != nil {
		t.Fatalf("finalize reentry order failed: %v", err)
	}
	position, found, err := store.FindPosition(account.ID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find final position failed: %v", err)
	}
	if !found {
		t.Fatal("expected final reentry short position")
	}
	if position.Side != "SHORT" || position.Quantity != 0.002 {
		t.Fatalf("expected final position SHORT 0.002, got %+v", position)
	}
}

func TestFinalizeExecutedOrderUsesExchangeTradeTimeForLastFilledAt(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, _ := store.GetAccount("live-main")
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	tradeTime := time.Date(2026, 4, 17, 12, 36, 0, 0, time.UTC)
	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{{
		OrderID:           order.ID,
		Price:             68000,
		Quantity:          0.1,
		Fee:               1.23,
		ExchangeTradeTime: &tradeTime,
	}})
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if got := stringValue(filledOrder.Metadata["lastFilledAt"]); got != tradeTime.Format(time.RFC3339) {
		t.Fatalf("expected lastFilledAt to use exchange trade time, got %q", got)
	}
}

func TestClosePositionAllowsLiveManualCloseWithoutRuntimeSession(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-manual-close"})

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": "test-manual-close",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}

	position, err := store.SavePosition(domain.Position{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0.25,
		MarkPrice: 68100,
	})
	if err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.ClosePosition(position.ID)
	if err != nil {
		t.Fatalf("expected live manual close to bypass runtime session checks, got %v", err)
	}
	if got := boolValue(order.Metadata["skipRuntimeCheck"]); !got {
		t.Fatal("expected live manual close order to set skipRuntimeCheck")
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected live manual close order to be accepted, got %s", got)
	}
	if got := stringValue(order.Metadata["runtimeSessionId"]); got != "" {
		t.Fatalf("expected bypassed live manual close to avoid linking a runtime session, got %s", got)
	}
}

func TestEnsureLivePositionReconcileGateAllowsExecutionSelfHealsStaleDBOnlyPosition(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	syncedAt := time.Date(2026, 4, 21, 1, 23, 45, 0, time.UTC)

	configureTestLiveRESTReconcileHistoryAdapter(
		t,
		platform,
		"test-manual-close-gate-self-heal",
		[]map[string]any{},
		map[string][]map[string]any{
			"BTCUSDT": {{
				"symbol":        "BTCUSDT",
				"orderId":       "9103",
				"clientOrderId": "client-9103",
				"status":        "FILLED",
				"side":          "SELL",
				"type":          "MARKET",
				"origType":      "MARKET",
				"origQty":       0.01,
				"executedQty":   0.01,
				"price":         67940.0,
				"avgPrice":      67940.0,
				"reduceOnly":    true,
				"closePosition": false,
				"time":          float64(syncedAt.Add(-2 * time.Minute).UnixMilli()),
				"updateTime":    float64(syncedAt.UnixMilli()),
			}},
		},
		map[string][]LiveFillReport{
			"BTCUSDT": {{
				Price:    67940.0,
				Quantity: 0.01,
				Fee:      0.01,
				Metadata: map[string]any{
					"exchangeOrderId": "9103",
					"tradeId":         "trade-9103",
					"tradeTime":       syncedAt.Format(time.RFC3339),
				},
			}},
		},
	)

	if _, err := store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67940,
	}); err != nil {
		t.Fatalf("save stale position failed: %v", err)
	}

	account, err := platform.SyncLiveAccount("live-main")
	if err != nil {
		t.Fatalf("sync live account failed: %v", err)
	}
	initialGate := resolveLivePositionReconcileGate(account, "BTCUSDT", true)
	if !boolValue(initialGate["blocking"]) || stringValue(initialGate["scenario"]) != "db-position-exchange-missing" {
		t.Fatalf("expected initial stale db-position-exchange-missing gate, got %#v", initialGate)
	}

	if err := platform.ensureLivePositionReconcileGateAllowsExecution("live-main", "BTCUSDT", true); err != nil {
		t.Fatalf("expected reconcile gate check to self-heal stale db-only position, got %v", err)
	}
	if _, found, err := store.FindPosition("live-main", "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected stale BTCUSDT position to be removed after reconcile gate self-heal")
	}

	account, err = store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get healed account failed: %v", err)
	}
	healedGate := resolveLivePositionReconcileGate(account, "BTCUSDT", true)
	if boolValue(healedGate["blocking"]) {
		t.Fatalf("expected reconcile gate to clear after self-heal, got %#v", healedGate)
	}
}

func TestClosePositionKeepsFailClosedWhenReconcileSelfHealFails(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: "test-manual-close-self-heal-fails",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "binance-rest-account-v3",
				"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
				"syncedAt":        time.Now().UTC().Format(time.RFC3339),
				"bindingMode":     stringValue(binding["connectionMode"]),
				"executionMode":   "rest",
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
				"positions":       []map[string]any{},
				"openOrders":      []map[string]any{},
			}
			var err error
			account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
			if err != nil {
				return domain.Account{}, err
			}
			return p.refreshLiveAccountPositionReconcileGate(account)
		},
		ordersErr: errors.New("reconcile fetch recent orders failed"),
	})

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":     "test-manual-close-self-heal-fails",
		"connectionMode": "mock",
		"executionMode":  "rest",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-manual-close-self-heal-fails",
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	position, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67940,
	})
	if err != nil {
		t.Fatalf("save stale position failed: %v", err)
	}

	if _, err := platform.ClosePosition(position.ID); err == nil || !strings.Contains(err.Error(), "reconcile fetch recent orders failed") {
		t.Fatalf("expected manual close to stay fail-closed when reconcile self-heal fails, got %v", err)
	}
}

func TestClosePositionKeepsFailClosedWhenSelfHealStillLeavesBlockingGate(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	syncedAt := time.Date(2026, 4, 21, 2, 0, 0, 0, time.UTC)

	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: "test-manual-close-self-heal-still-blocked",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "binance-rest-account-v3",
				"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
				"syncedAt":        syncedAt.Format(time.RFC3339),
				"bindingMode":     stringValue(binding["connectionMode"]),
				"executionMode":   "rest",
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
				"positions":       []map[string]any{},
				"openOrders":      []map[string]any{},
			}
			var err error
			account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
			if err != nil {
				return domain.Account{}, err
			}
			return p.refreshLiveAccountPositionReconcileGate(account)
		},
		ordersBySymbol: map[string][]map[string]any{
			"BTCUSDT": {{
				"symbol":        "BTCUSDT",
				"orderId":       "9201",
				"clientOrderId": "client-9201",
				"status":        "FILLED",
				"side":          "BUY",
				"type":          "MARKET",
				"origType":      "MARKET",
				"origQty":       0.02,
				"executedQty":   0.02,
				"price":         68020.0,
				"avgPrice":      68020.0,
				"reduceOnly":    false,
				"closePosition": false,
				"time":          float64(syncedAt.Add(-2 * time.Minute).UnixMilli()),
				"updateTime":    float64(syncedAt.UnixMilli()),
			}},
		},
		tradesBySymbol: map[string][]LiveFillReport{
			"BTCUSDT": {{
				Price:    68020.0,
				Quantity: 0.02,
				Fee:      0.01,
				Metadata: map[string]any{
					"exchangeOrderId": "9201",
					"tradeId":         "trade-9201",
					"tradeTime":       syncedAt.Format(time.RFC3339),
				},
			}},
		},
	})

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":     "test-manual-close-self-heal-still-blocked",
		"connectionMode": "mock",
		"executionMode":  "rest",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-manual-close-self-heal-still-blocked",
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	position, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000,
		MarkPrice:         67940,
	})
	if err != nil {
		t.Fatalf("save stale position failed: %v", err)
	}

	if _, err := platform.ClosePosition(position.ID); err == nil || !strings.Contains(err.Error(), "execution blocked by reconcile gate") {
		t.Fatalf("expected manual close to stay fail-closed when self-heal still leaves a blocking gate, got %v", err)
	}
}

func TestCreateLiveOrderImmediateFilledSubmissionSettlesReduceOnlyExit(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	tradeTime := time.Date(2026, 4, 20, 12, 33, 23, 0, time.UTC)
	syncCalls := 0
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-immediate-filled",
		submitOrderFunc: func(domain.Account, domain.Order, map[string]any) (LiveOrderSubmission, error) {
			return LiveOrderSubmission{
				Status:          "FILLED",
				ExchangeOrderID: "exchange-order-1",
				AcceptedAt:      tradeTime.Format(time.RFC3339),
				Metadata: map[string]any{
					"adapterMode":   "test",
					"executionMode": "rest",
					"executedQty":   0.002,
					"avgPrice":      75399.42,
					"updateTime":    tradeTime.Format(time.RFC3339),
				},
			}, nil
		},
		syncOrderFunc: func(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
			syncCalls++
			return LiveOrderSync{
				Status:   "FILLED",
				SyncedAt: tradeTime.Format(time.RFC3339),
				Fills: []LiveFillReport{{
					Price:    75399.42,
					Quantity: 0.002,
					Fee:      0.06,
					Metadata: map[string]any{
						"source":          "exchange-sync",
						"exchangeOrderId": "exchange-order-1",
						"tradeId":         "trade-1",
						"tradeTime":       tradeTime.Format(time.RFC3339),
					},
				}},
				Terminal:   true,
				FeeSource:  "exchange",
				FundingSrc: "exchange",
			}, nil
		},
	})

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": "test-immediate-filled",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        75405,
		MarkPrice:         75399.42,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.002,
		Price:             75399.42,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
			"executionProposal": map[string]any{
				"role":       "exit",
				"signalKind": "risk-exit",
				"reason":     "SL",
			},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	if syncCalls != 1 {
		t.Fatalf("expected immediate FILLED submission to force one sync, got %d", syncCalls)
	}
	if got := order.Status; got != "FILLED" {
		t.Fatalf("expected order to stay FILLED after settlement, got %s", got)
	}
	if got := parseFloatValue(order.Metadata["filledQuantity"]); got != 0.002 {
		t.Fatalf("expected filledQuantity 0.002, got %v", got)
	}
	if _, found, err := store.FindPosition(account.ID, "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected reduce-only FILLED exit to clear the local position")
	}
	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	orderFillCount := 0
	for _, item := range fills {
		if item.OrderID != order.ID {
			continue
		}
		orderFillCount++
		if item.Fee != 0.06 {
			t.Fatalf("expected synced fee 0.06, got %v", item.Fee)
		}
	}
	if orderFillCount != 1 {
		t.Fatalf("expected one fill for settled immediate-FILLED order, got %d", orderFillCount)
	}
}

func TestClosePositionImmediatelySettlesFilledLiveManualClose(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{
		key: "test-immediate-filled-close",
		submitResult: LiveOrderSubmission{
			Status:          "FILLED",
			ExchangeOrderID: "exchange-order-filled-1",
			AcceptedAt:      "2026-04-20T10:00:00Z",
		},
		syncResult: LiveOrderSync{
			Status:   "FILLED",
			SyncedAt: "2026-04-20T10:00:01Z",
			Fills: []LiveFillReport{{
				Price:    68100,
				Quantity: 0.25,
				Fee:      1.25,
				Metadata: map[string]any{
					"exchangeOrderId": "exchange-order-filled-1",
					"tradeId":         "trade-filled-1",
					"tradeTime":       "2026-04-20T10:00:01Z",
				},
			}},
			Terminal: true,
		},
	}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": adapter.key,
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	position, err := store.SavePosition(domain.Position{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0.25,
		MarkPrice: 68100,
	})
	if err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.ClosePosition(position.ID)
	if err != nil {
		t.Fatalf("close position failed: %v", err)
	}
	if got := order.Status; got != "FILLED" {
		t.Fatalf("expected immediate settlement to return FILLED, got %s", got)
	}
	if adapter.syncCount != 1 {
		t.Fatalf("expected immediate FILLED submission to trigger one order sync, got %d", adapter.syncCount)
	}
	if _, found, err := store.FindPosition(account.ID, "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected immediate FILLED live close to delete the position")
	}
	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	fillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			fillCount++
		}
	}
	if fillCount != 1 {
		t.Fatalf("expected one persisted fill for immediate FILLED close, got %d", fillCount)
	}
}

func TestClosePositionFilledLiveManualCloseClearsRecoverySessionState(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{
		key: "test-filled-close-session-refresh",
		submitResult: LiveOrderSubmission{
			Status:          "FILLED",
			ExchangeOrderID: "exchange-order-filled-2",
			AcceptedAt:      "2026-04-20T10:05:00Z",
		},
		syncResult: LiveOrderSync{
			Status:   "FILLED",
			SyncedAt: "2026-04-20T10:05:01Z",
			Fills: []LiveFillReport{{
				Price:    68150,
				Quantity: 0.25,
				Fee:      1.1,
				Metadata: map[string]any{
					"exchangeOrderId": "exchange-order-filled-2",
					"tradeId":         "trade-filled-2",
					"tradeTime":       "2026-04-20T10:05:01Z",
				},
			}},
			Terminal: true,
		},
	}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": adapter.key,
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	position, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		MarkPrice:         68150,
	})
	if err != nil {
		t.Fatalf("save live position failed: %v", err)
	}
	session, err := platform.CreateLiveSession("", account.ID, "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["recoveryMode"] = liveRecoveryModeCloseOnlyTakeover
	state["runtimeMode"] = liveRecoveryModeCloseOnlyTakeover
	state["signalRuntimeMode"] = liveRecoveryModeCloseOnlyTakeover
	state["signalRuntimeRequired"] = false
	state["signalRuntimeReady"] = false
	state["positionRecoveryStatus"] = liveRecoveryModeCloseOnlyTakeover
	state["lastStrategyEvaluationStatus"] = liveRecoveryModeCloseOnlyTakeover
	state["recoveredPosition"] = buildRecoveredLivePositionStateSnapshot(position)
	state["hasRecoveredPosition"] = true
	state["hasRecoveredRealPosition"] = true
	session, err = store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}
	session, err = store.UpdateLiveSessionStatus(session.ID, "BLOCKED")
	if err != nil {
		t.Fatalf("update live session status failed: %v", err)
	}

	if _, err := platform.ClosePosition(position.ID); err != nil {
		t.Fatalf("close position failed: %v", err)
	}

	updatedSession, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updatedSession.State["recoveryMode"]); got != "" {
		t.Fatalf("expected recoveryMode to clear after successful close, got %s", got)
	}
	if got := stringValue(updatedSession.State["positionRecoveryStatus"]); got != "flat" {
		t.Fatalf("expected positionRecoveryStatus flat after successful close, got %s", got)
	}
	if updatedSession.Status != "BLOCKED" {
		t.Fatalf("expected session status to stay BLOCKED until normal runtime flow resumes it, got %s", updatedSession.Status)
	}
}

func TestSettleImmediatelyFilledLiveOrderReturnsSettledOrderWhenAccountRefreshFails(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{
		key: "test-filled-close-refresh-failure",
		syncResult: LiveOrderSync{
			Status:   "FILLED",
			SyncedAt: "2026-04-20T10:10:01Z",
			Fills: []LiveFillReport{{
				Price:    68200,
				Quantity: 0.25,
				Fee:      1.15,
				Metadata: map[string]any{
					"exchangeOrderId": "exchange-order-filled-3",
					"tradeId":         "trade-filled-3",
					"tradeTime":       "2026-04-20T10:10:01Z",
				},
			}},
			Terminal: true,
		},
	}
	adapter.syncHook = func(account domain.Account, _ domain.Order) {
		account.Metadata = cloneMetadata(account.Metadata)
		delete(account.Metadata, "liveBinding")
		if _, err := store.UpdateAccount(account); err != nil {
			t.Fatalf("update account failed: %v", err)
		}
	}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": adapter.key,
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0.25,
		MarkPrice: 68200,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	order, err := store.CreateOrder(domain.Order{
		AccountID:  account.ID,
		Symbol:     "BTCUSDT",
		Side:       "SELL",
		Type:       "MARKET",
		Quantity:   0.25,
		Status:     "FILLED",
		ReduceOnly: true,
		Metadata: map[string]any{
			"exchangeOrderId": "exchange-order-filled-3",
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	settled, err := platform.settleImmediatelyFilledLiveOrder(order)
	if err == nil {
		t.Fatal("expected account refresh failure after settlement")
	}
	if got := settled.Status; got != "FILLED" {
		t.Fatalf("expected returned order to stay FILLED, got %s", got)
	}
	if _, found, findErr := store.FindPosition(account.ID, "BTCUSDT"); findErr != nil {
		t.Fatalf("find position failed: %v", findErr)
	} else if found {
		t.Fatal("expected position to be deleted even when account refresh fails")
	}
	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	fillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			fillCount++
		}
	}
	if fillCount != 1 {
		t.Fatalf("expected one fill persisted before account refresh failure, got %d", fillCount)
	}
}

func TestImmediateFilledLiveOrderRepeatedSyncKeepsRetryMarkerAndFillDedupeStable(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{
		key: "test-immediate-filled-repeat-sync",
		syncResult: LiveOrderSync{
			Status:   "FILLED",
			SyncedAt: "2026-04-20T10:20:01Z",
			Fills: []LiveFillReport{{
				Price:    68250,
				Quantity: 0.25,
				Fee:      1.2,
				Metadata: map[string]any{
					"exchangeOrderId": "exchange-order-filled-4",
					"tradeId":         "trade-filled-4",
					"tradeTime":       "2026-04-20T10:20:01Z",
				},
			}},
			Terminal: true,
		},
	}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": adapter.key,
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0.25,
		MarkPrice: 68250,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	order, err := store.CreateOrder(domain.Order{
		AccountID:  account.ID,
		Symbol:     "BTCUSDT",
		Side:       "SELL",
		Type:       "MARKET",
		Quantity:   0.25,
		Status:     "FILLED",
		ReduceOnly: true,
		Metadata: map[string]any{
			"exchangeOrderId":             "exchange-order-filled-4",
			liveSettlementSyncErrorKey:    "previous refresh failure",
			liveSettlementSyncRequiredKey: true,
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	syncedOrder, err := platform.SyncLiveOrder(order.ID)
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	syncedOrder, err = platform.SyncLiveOrder(order.ID)
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	fillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			fillCount++
		}
	}
	if fillCount != 1 {
		t.Fatalf("expected repeated sync to keep one fill, got %d", fillCount)
	}
	if got := stringValue(syncedOrder.Metadata[liveSettlementSyncErrorKey]); got != "" {
		t.Fatalf("expected retry marker error to clear after settlement, got %q", got)
	}
	if boolValue(syncedOrder.Metadata[liveSettlementSyncRequiredKey]) {
		t.Fatal("expected retry marker to clear after settlement")
	}
}

func TestImmediateFilledLiveOrderPartialSettlementKeepsRetryMarker(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.registerLiveAdapter(&recordingLiveExecutionAdapter{key: "test-partial-settlement"})
	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": "test-partial-settlement",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	order, err := store.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "MARKET",
		Quantity:          0.004,
		Price:             75600,
		Status:            "FILLED",
		Metadata: map[string]any{
			"exchangeOrderId":             "exchange-order-partial",
			liveSettlementSyncErrorKey:    "previous immediate settlement failure",
			liveSettlementSyncRequiredKey: true,
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	order.Status = "FILLED"
	if order, err = store.UpdateOrder(order); err != nil {
		t.Fatalf("mark order filled failed: %v", err)
	}

	settled, err := platform.applyLiveSyncResult(account, order, LiveOrderSync{
		Status:   "FILLED",
		SyncedAt: "2026-04-21T06:35:00Z",
		Fills: []LiveFillReport{{
			Price:    75600,
			Quantity: 0.002,
			Fee:      0.04,
			Metadata: map[string]any{
				"exchangeOrderId": "exchange-order-partial",
				"tradeId":         "trade-partial-1",
				"tradeTime":       "2026-04-21T06:35:00Z",
			},
		}},
	})
	if err != nil {
		t.Fatalf("partial settlement failed: %v", err)
	}
	if got := settled.Status; got != "PARTIALLY_FILLED" {
		t.Fatalf("expected partial settlement status PARTIALLY_FILLED, got %s", got)
	}
	if !boolValue(settled.Metadata[liveSettlementSyncRequiredKey]) {
		t.Fatal("expected partial settlement to keep retry marker")
	}
	if got := stringValue(settled.Metadata[liveSettlementSyncErrorKey]); got == "" {
		t.Fatal("expected partial settlement to keep retry error")
	}
	if !liveOrderSettlementSyncPending(settled) {
		t.Fatal("expected partial settlement to remain pending")
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryAllowsValidHedgeClose(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{key: "test-recovered-close-valid"}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    adapter.key,
		"executionMode": "mock",
		"positionMode":  "HEDGE",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.25,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
			"executionProposal": map[string]any{
				"role":       "exit",
				"side":       "SELL",
				"symbol":     "BTCUSDT",
				"signalKind": "recovery-watchdog",
				"metadata": map[string]any{
					"recoveryTriggered": true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create recovered close order failed: %v", err)
	}
	if adapter.submitCount != 1 {
		t.Fatalf("expected adapter submit to be called once, got %d", adapter.submitCount)
	}
	if got := stringValue(adapter.lastOrder.Metadata["executionBoundaryClass"]); got != liveExecutionBoundaryClassRecoveredPassiveClose {
		t.Fatalf("expected recovered-passive-close classification, got %s", got)
	}
	if got := stringValue(adapter.lastOrder.Metadata["positionSide"]); got != "LONG" {
		t.Fatalf("expected hedge recovered close to submit positionSide LONG, got %s", got)
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected recovered close order to be accepted, got %s", got)
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryBlocksMissingReduceOnly(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{key: "test-recovered-close-missing-reduce-only"}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    adapter.key,
		"executionMode": "mock",
		"positionMode":  "ONE_WAY",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.25,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
			"executionProposal": map[string]any{
				"role":       "exit",
				"side":       "SELL",
				"symbol":     "BTCUSDT",
				"signalKind": "recovery-watchdog",
				"metadata": map[string]any{
					"recoveryTriggered": true,
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected recovered close without reduceOnly to be blocked")
	}
	if adapter.submitCount != 0 {
		t.Fatalf("expected adapter submit not to be called, got %d", adapter.submitCount)
	}
	if got := order.Status; got != "REJECTED" {
		t.Fatalf("expected blocked recovered close order to be marked REJECTED, got %s", got)
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryBlocksInvalidSideAndHedgePayload(t *testing.T) {
	tests := []struct {
		name         string
		side         string
		positionSide string
		wantErr      string
	}{
		{
			name:    "wrong side",
			side:    "BUY",
			wantErr: "does not reduce LONG position",
		},
		{
			name:         "wrong hedge positionSide",
			side:         "SELL",
			positionSide: "SHORT",
			wantErr:      "does not match LONG position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memory.NewStore()
			platform := NewPlatform(store)
			adapter := &recordingLiveExecutionAdapter{key: "test-recovered-close-" + strings.ReplaceAll(tt.name, " ", "-")}
			platform.registerLiveAdapter(adapter)

			account, err := platform.BindLiveAccount("live-main", map[string]any{
				"adapterKey":    adapter.key,
				"executionMode": "mock",
				"positionMode":  "HEDGE",
			})
			if err != nil {
				t.Fatalf("bind live account failed: %v", err)
			}
			if _, err := store.SavePosition(domain.Position{
				AccountID:         account.ID,
				StrategyVersionID: "strategy-version-bk-1d-v010",
				Symbol:            "BTCUSDT",
				Side:              "LONG",
				Quantity:          0.25,
				MarkPrice:         68100,
			}); err != nil {
				t.Fatalf("save live position failed: %v", err)
			}
			binding := resolveLiveBinding(account)
			order, err := platform.prepareLiveOrderForSubmission(account, domain.Order{
				AccountID:         account.ID,
				StrategyVersionID: "strategy-version-bk-1d-v010",
				Symbol:            "BTCUSDT",
				Side:              tt.side,
				Type:              "MARKET",
				Quantity:          0.25,
				ReduceOnly:        true,
				Metadata: map[string]any{
					"skipRuntimeCheck": true,
					"positionSide":     tt.positionSide,
					"executionProposal": map[string]any{
						"role":       "exit",
						"side":       tt.side,
						"symbol":     "BTCUSDT",
						"signalKind": "recovery-watchdog",
						"metadata": map[string]any{
							"recoveryTriggered": true,
						},
					},
				},
			}, binding)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
			if got := stringValue(order.Metadata["executionBoundaryClass"]); got != liveExecutionBoundaryClassRecoveredPassiveClose {
				t.Fatalf("expected recovered-passive-close classification, got %s", got)
			}
		})
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryLeavesNormalEntryFlowIntact(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{key: "test-normal-entry"}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    adapter.key,
		"executionMode": "mock",
		"positionMode":  "ONE_WAY",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "MARKET",
		Quantity:          0.25,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
		},
	})
	if err != nil {
		t.Fatalf("create normal entry order failed: %v", err)
	}
	if adapter.submitCount != 1 {
		t.Fatalf("expected adapter submit to be called once, got %d", adapter.submitCount)
	}
	if got := stringValue(adapter.lastOrder.Metadata["executionBoundaryClass"]); got != liveExecutionBoundaryClassNormalEntry {
		t.Fatalf("expected normal-entry classification, got %s", got)
	}
	if got := stringValue(adapter.lastOrder.Metadata["positionSide"]); got != "" {
		t.Fatalf("expected normal entry not to inject positionSide, got %s", got)
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected normal entry order to be accepted, got %s", got)
	}
}

func TestShouldSendBinanceReduceOnlyFlagRespectsPositionMode(t *testing.T) {
	order := domain.Order{ReduceOnly: true}
	if !shouldSendBinanceReduceOnlyFlag(map[string]any{"positionMode": "ONE_WAY"}, order) {
		t.Fatal("expected reduceOnly flag to be sent in ONE_WAY mode")
	}
	if shouldSendBinanceReduceOnlyFlag(map[string]any{"positionMode": "HEDGE"}, order) {
		t.Fatal("expected reduceOnly flag to be omitted in HEDGE mode")
	}
}

func TestResolveBinancePositionSideForSubmissionOmitsOneWayBoth(t *testing.T) {
	order := domain.Order{Metadata: map[string]any{"positionSide": "BOTH"}}
	if got := resolveBinancePositionSideForSubmission(map[string]any{"positionMode": "ONE_WAY"}, order); got != "" {
		t.Fatalf("expected ONE_WAY BOTH to be omitted, got %s", got)
	}
	if got := resolveBinancePositionSideForSubmission(map[string]any{"positionMode": "HEDGE"}, domain.Order{
		Metadata: map[string]any{"positionSide": "LONG"},
	}); got != "LONG" {
		t.Fatalf("expected hedge positionSide LONG to be preserved, got %s", got)
	}
}

func TestSubmitRESTOrderRecoveredPassiveClosePayloadMatchesBinanceModes(t *testing.T) {
	tests := []struct {
		name                  string
		positionMode          string
		side                  string
		positionSide          string
		reduceOnly            bool
		wantPositionSide      string
		wantReduceOnlyPresent bool
	}{
		{
			name:                  "hedge long close",
			positionMode:          "HEDGE",
			side:                  "SELL",
			positionSide:          "LONG",
			reduceOnly:            true,
			wantPositionSide:      "LONG",
			wantReduceOnlyPresent: false,
		},
		{
			name:                  "one way close",
			positionMode:          "ONE_WAY",
			side:                  "SELL",
			positionSide:          "BOTH",
			reduceOnly:            true,
			wantPositionSide:      "",
			wantReduceOnlyPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedForm neturl.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/fapi/v1/exchangeInfo":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"symbols": []map[string]any{{
							"symbol": "BTCUSDT",
							"filters": []map[string]any{
								{"filterType": "PRICE_FILTER", "tickSize": "0.1"},
								{"filterType": "LOT_SIZE", "stepSize": "0.001", "minQty": "0.001", "maxQty": "1000"},
								{"filterType": "MIN_NOTIONAL", "notional": "100"},
							},
						}},
					})
				case "/fapi/v1/order":
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("read order body failed: %v", err)
					}
					capturedForm, err = neturl.ParseQuery(string(body))
					if err != nil {
						t.Fatalf("parse order form failed: %v", err)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status":        "NEW",
						"orderId":       12345,
						"clientOrderId": "order-test",
						"updateTime":    time.Now().UTC().UnixMilli(),
					})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			t.Setenv("TEST_BINANCE_KEY", "key")
			t.Setenv("TEST_BINANCE_SECRET", "secret")

			binanceSymbolRulesCacheMu.Lock()
			binanceSymbolRulesCache = map[string]binanceSymbolRules{}
			binanceSymbolRulesCacheMu.Unlock()

			adapter := binanceFuturesLiveAdapter{}
			_, err := adapter.submitRESTOrder(domain.Account{Exchange: "binance-futures"}, domain.Order{
				ID:         "order-test",
				AccountID:  "live-main",
				Symbol:     "BTCUSDT",
				Side:       tt.side,
				Type:       "MARKET",
				Quantity:   0.25,
				ReduceOnly: tt.reduceOnly,
				Metadata: map[string]any{
					"positionSide": tt.positionSide,
				},
			}, map[string]any{
				"executionMode": "rest",
				"positionMode":  tt.positionMode,
				"restBaseUrl":   server.URL,
				"recvWindowMs":  5000,
				"credentialRefs": map[string]any{
					"apiKeyRef":    "TEST_BINANCE_KEY",
					"apiSecretRef": "TEST_BINANCE_SECRET",
				},
			})
			if err != nil {
				t.Fatalf("submit REST order failed: %v", err)
			}
			if got := capturedForm.Get("side"); got != tt.side {
				t.Fatalf("expected side %s, got %s", tt.side, got)
			}
			if got := capturedForm.Get("positionSide"); got != tt.wantPositionSide {
				t.Fatalf("expected positionSide %q, got %q", tt.wantPositionSide, got)
			}
			_, hasReduceOnly := capturedForm["reduceOnly"]
			if hasReduceOnly != tt.wantReduceOnlyPresent {
				t.Fatalf("expected reduceOnly present=%t, form=%v", tt.wantReduceOnlyPresent, capturedForm)
			}
		})
	}
}

func TestFinalizeExecutedOrderFallsBackToNowWhenExchangeTradeTimeMissing(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, _ := store.GetAccount("live-main")
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	before := time.Now().UTC().Add(-time.Second)
	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{{
		OrderID:  order.ID,
		Price:    68000,
		Quantity: 0.1,
		Fee:      1.23,
	}})
	after := time.Now().UTC().Add(time.Second)
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	got := parseOptionalRFC3339(stringValue(filledOrder.Metadata["lastFilledAt"]))
	if got.IsZero() || got.Before(before) || got.After(after) {
		t.Fatalf("expected lastFilledAt to fall back to now, got %v", got)
	}
}

func TestFinalizeExecutedOrderKeepsLastFilledAtOnDuplicateSync(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, _ := store.GetAccount("live-main")
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	firstTradeTime := time.Date(2026, 4, 17, 12, 36, 0, 0, time.UTC)
	fill := domain.Fill{
		OrderID:           order.ID,
		Price:             68000,
		Quantity:          0.1,
		Fee:               1.23,
		ExchangeTradeTime: &firstTradeTime,
	}
	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}

	filledOrder, err = platform.finalizeExecutedOrder(account, filledOrder, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("second finalize failed: %v", err)
	}

	if got := stringValue(filledOrder.Metadata["lastFilledAt"]); got != firstTradeTime.Format(time.RFC3339) {
		t.Fatalf("expected duplicate sync to keep original lastFilledAt, got %q", got)
	}
}

type recordingLiveExecutionAdapter struct {
	key          string
	submitCount  int
	syncCount    int
	lastOrder    domain.Order
	submitResult LiveOrderSubmission
	syncResult   LiveOrderSync
	submitErr    error
	syncErr      error
	syncHook     func(domain.Account, domain.Order)
}

func (a *recordingLiveExecutionAdapter) Key() string {
	return a.key
}

func (a *recordingLiveExecutionAdapter) Describe() map[string]any {
	return map[string]any{"key": a.key}
}

func (a *recordingLiveExecutionAdapter) ValidateAccountConfig(map[string]any) error {
	return nil
}

func (a *recordingLiveExecutionAdapter) SubmitOrder(_ domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	a.submitCount++
	a.lastOrder = order
	if a.submitErr != nil {
		return LiveOrderSubmission{}, a.submitErr
	}
	if a.submitResult.Status != "" || a.submitResult.ExchangeOrderID != "" || a.submitResult.AcceptedAt != "" || len(a.submitResult.Metadata) > 0 {
		result := a.submitResult
		result.Metadata = cloneMetadata(result.Metadata)
		if result.Metadata == nil {
			result.Metadata = map[string]any{}
		}
		if result.Metadata["executionMode"] == nil {
			result.Metadata["executionMode"] = stringValue(binding["executionMode"])
		}
		if result.Metadata["positionMode"] == nil {
			result.Metadata["positionMode"] = stringValue(binding["positionMode"])
		}
		return result, nil
	}
	return LiveOrderSubmission{
		Status:          "ACCEPTED",
		ExchangeOrderID: "exchange-order-1",
		AcceptedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"adapterMode":   "recording",
			"executionMode": stringValue(binding["executionMode"]),
			"positionMode":  stringValue(binding["positionMode"]),
			"positionSide":  stringValue(order.Metadata["positionSide"]),
		},
	}, nil
}

func (a *recordingLiveExecutionAdapter) SyncOrder(account domain.Account, order domain.Order, _ map[string]any) (LiveOrderSync, error) {
	a.syncCount++
	if a.syncHook != nil {
		a.syncHook(account, order)
	}
	if a.syncErr != nil {
		return LiveOrderSync{}, a.syncErr
	}
	return LiveOrderSync{
		Status:     a.syncResult.Status,
		SyncedAt:   a.syncResult.SyncedAt,
		Fills:      append([]LiveFillReport(nil), a.syncResult.Fills...),
		Metadata:   cloneMetadata(a.syncResult.Metadata),
		Terminal:   a.syncResult.Terminal,
		FeeSource:  a.syncResult.FeeSource,
		FundingSrc: a.syncResult.FundingSrc,
	}, nil
}

func (a *recordingLiveExecutionAdapter) CancelOrder(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}
