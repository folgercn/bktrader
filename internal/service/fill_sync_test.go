package service

import (
	"reflect"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestMaskSensitiveData(t *testing.T) {
	input := map[string]any{
		"orderId":     "12345",
		"apiKey":      "my-secret-api-key",
		"APISecret":   "super-secret",
		"signature":   "abcdef123456",
		"clientToken": "tok_123",
		"nested": map[string]any{
			"normal": "value",
			"key":    "nested-key",
		},
		"list": []any{
			map[string]any{
				"itemKey": "val1",
			},
			"normal_string",
		},
	}

	expected := map[string]any{
		"orderId":     "12345",
		"apiKey":      "***",
		"APISecret":   "***",
		"signature":   "***",
		"clientToken": "***",
		"nested": map[string]any{
			"normal": "value",
			"key":    "***",
		},
		"list": []any{
			map[string]any{
				"itemKey": "***",
			},
			"normal_string",
		},
	}

	result := maskSensitiveData(input)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("maskSensitiveData() = %v, want %v", result, expected)
	}
}

type mockLiveAdapter struct{}

func (m *mockLiveAdapter) Key() string                                       { return "binance-futures" }
func (m *mockLiveAdapter) Describe() map[string]any                          { return nil }
func (m *mockLiveAdapter) ValidateAccountConfig(config map[string]any) error { return nil }
func (m *mockLiveAdapter) SubmitOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	return LiveOrderSubmission{}, nil
}
func (m *mockLiveAdapter) SyncOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}
func (m *mockLiveAdapter) CancelOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}

func TestRebuildOrderFills(t *testing.T) {
	store := memory.NewStore()
	p := &Platform{
		store:        store,
		liveAdapters: make(map[string]LiveExecutionAdapter),
	}
	p.liveAdapters["binance-futures"] = &mockLiveAdapter{}

	account, _ := store.CreateAccount("test-account", "LIVE", "binance-futures")
	account.Metadata = map[string]any{
		"liveBinding": map[string]any{
			"adapterKey": "binance-futures",
		},
	}
	store.UpdateAccount(account)
	order := domain.Order{
		ID:        "order-1",
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Quantity:  1.0,
		Status:    "FILLED",
		Metadata:  map[string]any{},
	}
	order, _ = store.CreateOrder(order)

	// Create one synthetic fill and one real fill (partially)
	store.CreateFill(domain.Fill{
		OrderID:  order.ID,
		Price:    50000,
		Quantity: 0.4,
		Source:   "synthetic",
	})
	store.CreateFill(domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: "trade-already-exists",
		Price:           50000,
		Quantity:        0.3,
		Source:          "real",
	})

	// Create position
	store.SavePosition(domain.Position{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Quantity:  0.7, // 0.4 synthetic + 0.3 real
	})

	// Match 3 remote trades (one duplicate, two new using different ID keys)
	matchedTrades := []LiveFillReport{
		{
			Price:    50000,
			Quantity: 0.3,
			Metadata: map[string]any{"exchangeTradeId": "trade-already-exists"},
		},
		{
			Price:    50100,
			Quantity: 0.5,
			Metadata: map[string]any{"tradeId": "trade-new-1"},
		},
		{
			Price:    50200,
			Quantity: 0.2,
			Metadata: map[string]any{"execId": "trade-new-2"},
		},
	}

	// 1. Dry Run
	resp, err := p.RebuildOrderFills(order.ID, matchedTrades, "test-dry-run", true)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}
	if resp.Changes.DeletedSyntheticCount != 1 {
		t.Errorf("Dry run: expected 1 deleted synthetic, got %d", resp.Changes.DeletedSyntheticCount)
	}
	if resp.Changes.AddedRealCount != 2 {
		t.Errorf("Dry run: expected 2 added real, got %d", resp.Changes.AddedRealCount)
	}
	if len(resp.Changes.DuplicateTradeIDs) != 1 {
		t.Errorf("Dry run: expected 1 duplicate trade, got %d", len(resp.Changes.DuplicateTradeIDs))
	}

	// 2. Real Rebuild
	resp, err = p.RebuildOrderFills(order.ID, matchedTrades, "test-real-rebuild", false)
	if err != nil {
		t.Fatalf("Real rebuild failed: %v", err)
	}

	// Check Store State
	fills, _ := store.QueryFills(domain.FillQuery{OrderIDs: []string{order.ID}})
	if len(fills) != 3 {
		t.Errorf("Expected 3 fills after rebuild, got %d", len(fills))
	}
	for _, f := range fills {
		if f.Source == "synthetic" {
			t.Errorf("Found synthetic fill after rebuild")
		}
	}

	if resp.After.FilledQuantity != 1.0 {
		t.Errorf("Expected after snapshot filled quantity 1.0, got %f", resp.After.FilledQuantity)
	}

	// Check Audit History
	updatedOrder, _ := store.GetOrderByID(order.ID)
	history, ok := updatedOrder.Metadata["manualFillSyncHistory"].([]any)
	if !ok || len(history) != 1 {
		t.Errorf("Audit history not found or incorrect")
	} else {
		entry := history[0].(map[string]any)
		if entry["result"] != "settled" {
			t.Errorf("Expected result 'settled', got %v", entry["result"])
		}
		if entry["after"] == nil {
			t.Errorf("Expected 'after' snapshot in history")
		}
		if entry["actor"] != "system" {
			t.Errorf("Expected actor 'system', got %v", entry["actor"])
		}
	}

	// Check Position (should NOT have changed as it's degraded to reconcile)
	pos, _, _ := store.FindPosition(account.ID, "BTCUSDT")
	if pos.Quantity != 0.7 {
		t.Errorf("Expected position quantity 0.7 (unchanged), got %f", pos.Quantity)
	}
}
