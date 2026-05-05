package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestCreatePaperOrderNotifiesDashboardOrdersAndFills(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	account, err := platform.CreateAccount("Paper Dashboard", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	if _, err := platform.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.01,
		Price:     68000,
	}); err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	changes := collectDashboardChanges(t, platform.DashboardBroker(), 3, 200*time.Millisecond)
	requireDashboardChange(t, changes, DashboardDomainOrders, "order-created")
	requireDashboardChange(t, changes, DashboardDomainOrders, "order-filled")
	requireDashboardChange(t, changes, DashboardDomainFills, "fill-created")
}

func TestApplyLiveSyncResultWithoutFillsNotifiesDashboardOrders(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	account, err := platform.CreateAccount("Live Dashboard", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "LIMIT",
		Status:    "ACCEPTED",
		Quantity:  0.01,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	if _, err := platform.applyLiveSyncResult(account, order, LiveOrderSync{
		Status:   "CANCELLED",
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"exchangeOrderId": "exchange-order-1",
		},
	}); err != nil {
		t.Fatalf("applyLiveSyncResult failed: %v", err)
	}

	change := waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	if change.Domain != DashboardDomainOrders || change.Reason != "order-synced" {
		t.Fatalf("unexpected dashboard change: %+v", change)
	}
}

func collectDashboardChanges(t *testing.T, broker *DashboardBroker, count int, timeout time.Duration) []dashboardChange {
	t.Helper()
	changes := make([]dashboardChange, 0, count)
	deadline := time.After(timeout)
	for len(changes) < count {
		select {
		case change := <-broker.changes:
			changes = append(changes, change)
		case <-deadline:
			t.Fatalf("timed out waiting for %d dashboard changes, got %+v", count, changes)
		}
	}
	return changes
}

func requireDashboardChange(t *testing.T, changes []dashboardChange, domain DashboardDomain, reason string) {
	t.Helper()
	for _, change := range changes {
		if change.Domain == domain && change.Reason == reason {
			return
		}
	}
	t.Fatalf("expected dashboard change domain=%s reason=%s in %+v", domain, reason, changes)
}
