package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestReconcileLiveAccountRecoversMissingFilledOrder(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	syncedAt := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)
	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: "test-reconcile",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":      "test-reconcile",
				"syncedAt":    syncedAt.Format(time.RFC3339),
				"openOrders":  []map[string]any{{"symbol": "BTCUSDT"}},
				"positions":   []map[string]any{},
				"bindingMode": stringValue(binding["connectionMode"]),
			}
			account.Metadata["lastLiveSyncAt"] = syncedAt.Format(time.RFC3339)
			return p.store.UpdateAccount(account)
		},
		ordersBySymbol: map[string][]map[string]any{
			"BTCUSDT": {{
				"symbol":        "BTCUSDT",
				"orderId":       "9001",
				"clientOrderId": "client-9001",
				"status":        "FILLED",
				"side":          "BUY",
				"type":          "MARKET",
				"origType":      "MARKET",
				"origQty":       0.2,
				"executedQty":   0.2,
				"price":         68000.0,
				"avgPrice":      68010.0,
				"reduceOnly":    false,
				"closePosition": false,
				"time":          float64(syncedAt.Add(-2 * time.Minute).UnixMilli()),
				"updateTime":    float64(syncedAt.UnixMilli()),
			}},
		},
		tradesBySymbol: map[string][]LiveFillReport{
			"BTCUSDT": {{
				Price:    68010,
				Quantity: 0.2,
				Fee:      1.2,
				Metadata: map[string]any{
					"exchangeOrderId": "9001",
					"tradeId":         "trade-9001",
					"executionMode":   "rest",
				},
			}},
		},
	})

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-reconcile",
		"connectionMode": "rest",
		"executionMode":  "rest",
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	result, err := platform.ReconcileLiveAccount("live-main", LiveAccountReconcileOptions{LookbackHours: 24})
	if err != nil {
		t.Fatalf("reconcile live account failed: %v", err)
	}
	if result.CreatedOrderCount != 1 {
		t.Fatalf("expected one recovered order, got %d", result.CreatedOrderCount)
	}
	if result.OrderCount != 1 {
		t.Fatalf("expected one reconciled order, got %d", result.OrderCount)
	}

	orders, err := store.ListOrders()
	if err != nil {
		t.Fatalf("list orders failed: %v", err)
	}
	var recovered domain.Order
	for _, item := range orders {
		if item.AccountID == "live-main" && stringValue(item.Metadata["exchangeOrderId"]) == "9001" {
			recovered = item
			break
		}
	}
	if recovered.ID == "" {
		t.Fatal("expected recovered order to be persisted locally")
	}
	if recovered.Status != "FILLED" {
		t.Fatalf("expected recovered order to be FILLED, got %s", recovered.Status)
	}
	if !boolValue(recovered.Metadata["reconcileRecovered"]) {
		t.Fatal("expected recovered order to be tagged as reconcileRecovered")
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	fillCount := 0
	for _, item := range fills {
		if item.OrderID == recovered.ID {
			fillCount++
			if item.ExchangeTradeID != "trade-9001" {
				t.Fatalf("expected fill exchange trade id trade-9001, got %q", item.ExchangeTradeID)
			}
		}
	}
	if fillCount != 1 {
		t.Fatalf("expected one recovered fill, got %d", fillCount)
	}

	position, found, err := store.FindPosition("live-main", "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected reconcile to rebuild BTCUSDT position")
	}
	if position.Quantity != 0.2 {
		t.Fatalf("expected reconciled position quantity 0.2, got %v", position.Quantity)
	}

	updatedAccount, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get updated account failed: %v", err)
	}
	lastReconcile := mapValue(updatedAccount.Metadata["lastLiveReconcile"])
	if got := int(parseFloatValue(lastReconcile["createdOrderCount"])); got != 1 {
		t.Fatalf("expected reconcile summary createdOrderCount=1, got %d", got)
	}
}

func TestReconcileLiveAccountRefreshClearsStaleRecoveryCache(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	syncedAt := time.Date(2026, 4, 20, 11, 0, 0, 0, time.UTC)
	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: "test-reconcile-refresh",
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":      "test-reconcile-refresh",
				"syncedAt":    syncedAt.Format(time.RFC3339),
				"openOrders":  []map[string]any{},
				"positions":   []map[string]any{},
				"bindingMode": stringValue(binding["connectionMode"]),
			}
			account.Metadata["lastLiveSyncAt"] = syncedAt.Format(time.RFC3339)
			return p.store.UpdateAccount(account)
		},
		ordersBySymbol: map[string][]map[string]any{
			"BTCUSDT": {},
		},
		tradesBySymbol: map[string][]LiveFillReport{
			"BTCUSDT": {},
		},
	})

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     "test-reconcile-refresh",
		"connectionMode": "rest",
		"executionMode":  "rest",
	}
	account.Metadata["livePositionReconcileGate"] = map[string]any{
		"symbols": map[string]any{
			"BTCUSDT": map[string]any{
				"status":   livePositionReconcileGateStatusVerified,
				"blocking": false,
				"scenario": "exchange-flat",
			},
		},
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	session, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["recoveryMode"] = liveRecoveryModeReconcileGateBlocked
	state["runtimeMode"] = liveRecoveryModeReconcileGateBlocked
	state["signalRuntimeMode"] = liveRecoveryModeReconcileGateBlocked
	state["signalRuntimeRequired"] = false
	state["signalRuntimeReady"] = false
	state["positionRecoveryStatus"] = livePositionReconcileGateStatusConflict
	state["positionReconcileGateStatus"] = livePositionReconcileGateStatusConflict
	state["positionReconcileGateBlocking"] = true
	state["positionReconcileGateScenario"] = "db-position-exchange-missing"
	session, err = store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}
	session, err = store.UpdateLiveSessionStatus(session.ID, "BLOCKED")
	if err != nil {
		t.Fatalf("update live session status failed: %v", err)
	}

	if _, err := platform.ReconcileLiveAccount("live-main", LiveAccountReconcileOptions{LookbackHours: 24}); err != nil {
		t.Fatalf("reconcile live account failed: %v", err)
	}

	updated, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["recoveryMode"]); got != "" {
		t.Fatalf("expected recoveryMode cleared after reconcile refresh, got %s", got)
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "flat" {
		t.Fatalf("expected positionRecoveryStatus flat after reconcile refresh, got %s", got)
	}
	if got := stringValue(updated.State["positionReconcileGateStatus"]); got != "verified" {
		t.Fatalf("expected positionReconcileGateStatus verified, got %s", got)
	}
	if got := updated.Status; got != "BLOCKED" {
		t.Fatalf("expected reconcile refresh to clear stale cache without auto-resuming session, got %s", got)
	}
}

func configureTestLiveRESTReconcileHistoryAdapter(
	t *testing.T,
	platform *Platform,
	adapterKey string,
	exchangePositions []map[string]any,
	ordersBySymbol map[string][]map[string]any,
	tradesBySymbol map[string][]LiveFillReport,
) {
	t.Helper()
	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: adapterKey,
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
				"positions":       exchangePositions,
				"openOrders":      []map[string]any{},
			}
			var err error
			account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
			if err != nil {
				return domain.Account{}, err
			}
			reconcileGate, err := p.reconcileLiveAccountPositions(account, exchangePositions)
			if err != nil {
				return domain.Account{}, err
			}
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["livePositionReconcileGate"] = reconcileGate
			account.Metadata["lastLivePositionSyncAt"] = time.Now().UTC().Format(time.RFC3339)
			clearLiveAccountPositionReconcileRequirement(account.Metadata)
			return p.store.UpdateAccount(account)
		},
		ordersBySymbol: ordersBySymbol,
		tradesBySymbol: tradesBySymbol,
	})

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "READY"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = map[string]any{
		"adapterKey":     adapterKey,
		"connectionMode": "mock",
		"executionMode":  "rest",
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}
}

type testLiveAccountReconcileAdapter struct {
	key              string
	syncSnapshotFunc func(*Platform, domain.Account, map[string]any) (domain.Account, error)
	ordersBySymbol   map[string][]map[string]any
	tradesBySymbol   map[string][]LiveFillReport
	ordersErr        error
	tradesErr        error
}

func (a testLiveAccountReconcileAdapter) Key() string {
	return a.key
}

func (a testLiveAccountReconcileAdapter) Describe() map[string]any {
	return map[string]any{"key": a.key}
}

func (a testLiveAccountReconcileAdapter) ValidateAccountConfig(map[string]any) error {
	return nil
}

func (a testLiveAccountReconcileAdapter) SubmitOrder(domain.Account, domain.Order, map[string]any) (LiveOrderSubmission, error) {
	return LiveOrderSubmission{}, nil
}

func (a testLiveAccountReconcileAdapter) SyncOrder(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}

func (a testLiveAccountReconcileAdapter) CancelOrder(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}

func (a testLiveAccountReconcileAdapter) SyncAccountSnapshot(platform *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
	if a.syncSnapshotFunc != nil {
		return a.syncSnapshotFunc(platform, account, binding)
	}
	return account, nil
}

func (a testLiveAccountReconcileAdapter) FetchRecentOrders(_ domain.Account, _ map[string]any, symbol string, _ int) ([]map[string]any, error) {
	if a.ordersErr != nil {
		return nil, a.ordersErr
	}
	return cloneReconcileOrders(a.ordersBySymbol[symbol]), nil
}

func (a testLiveAccountReconcileAdapter) FetchRecentTrades(_ domain.Account, _ map[string]any, symbol string, _ int) ([]LiveFillReport, error) {
	if a.tradesErr != nil {
		return nil, a.tradesErr
	}
	return cloneReconcileTrades(a.tradesBySymbol[symbol]), nil
}

func cloneReconcileOrders(source []map[string]any) []map[string]any {
	cloned := make([]map[string]any, 0, len(source))
	for _, item := range source {
		cloned = append(cloned, cloneMetadata(item))
	}
	return cloned
}

func cloneReconcileTrades(source []LiveFillReport) []LiveFillReport {
	cloned := make([]LiveFillReport, 0, len(source))
	for _, item := range source {
		cloned = append(cloned, LiveFillReport{
			Price:      item.Price,
			Quantity:   item.Quantity,
			Fee:        item.Fee,
			FundingPnL: item.FundingPnL,
			Metadata:   cloneMetadata(item.Metadata),
		})
	}
	return cloned
}
