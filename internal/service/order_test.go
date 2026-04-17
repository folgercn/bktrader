package service

import (
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
