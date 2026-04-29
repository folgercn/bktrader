package memory

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestCreateFillPersistsExplicitSourceMemory(t *testing.T) {
	store := NewStore()

	created, err := store.CreateFill(domain.Fill{
		OrderID:          "order-source-explicit",
		Source:           "remainder",
		Price:            68000,
		Quantity:         0.4,
		DedupFingerprint: "synthetic-remainder|order-source-explicit|0.400000000000",
	})
	if err != nil {
		t.Fatalf("CreateFill failed: %v", err)
	}
	if created.Source != "remainder" {
		t.Fatalf("expected created source remainder, got %q", created.Source)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("ListFills failed: %v", err)
	}
	if len(fills) != 1 || fills[0].Source != "remainder" {
		t.Fatalf("expected listed fill source remainder, got %+v", fills)
	}
}

func TestCreateFillInfersSourceMemory(t *testing.T) {
	store := NewStore()

	realFill, err := store.CreateFill(domain.Fill{
		OrderID:         "order-source-infer",
		ExchangeTradeID: "trade-1",
		Price:           68000,
		Quantity:        0.4,
	})
	if err != nil {
		t.Fatalf("CreateFill real failed: %v", err)
	}
	if realFill.Source != "real" {
		t.Fatalf("expected real source, got %q", realFill.Source)
	}

	syntheticFill, err := store.CreateFill(domain.Fill{
		OrderID:          "order-source-infer",
		Price:            68000,
		Quantity:         0.6,
		DedupFingerprint: "terminal-filled-order-fallback|order-source-infer",
	})
	if err != nil {
		t.Fatalf("CreateFill synthetic failed: %v", err)
	}
	if syntheticFill.Source != "synthetic" {
		t.Fatalf("expected synthetic source, got %q", syntheticFill.Source)
	}
}

func TestCreateFillUpsertKeepsRealFeeMemory(t *testing.T) {
	store := NewStore()

	if _, err := store.CreateFill(domain.Fill{
		OrderID:         "order-fee-upsert",
		ExchangeTradeID: "trade-fee-upsert",
		Source:          "real",
		Price:           68000,
		Quantity:        0.4,
		Fee:             0,
	}); err != nil {
		t.Fatalf("CreateFill initial real failed: %v", err)
	}

	updated, err := store.CreateFill(domain.Fill{
		OrderID:         "order-fee-upsert",
		ExchangeTradeID: "trade-fee-upsert",
		Source:          "real",
		Price:           68000,
		Quantity:        0.5,
		Fee:             0.1234,
	})
	if err != nil {
		t.Fatalf("CreateFill real fee upsert failed: %v", err)
	}
	if updated.Fee != 0.1234 {
		t.Fatalf("expected fee to update from later real report, got %v", updated.Fee)
	}
	if updated.Quantity != 0.4 {
		t.Fatalf("expected fee upsert to keep original quantity 0.4, got %v", updated.Quantity)
	}

	unchanged, err := store.CreateFill(domain.Fill{
		OrderID:         "order-fee-upsert",
		ExchangeTradeID: "trade-fee-upsert",
		Source:          "real",
		Price:           68000,
		Quantity:        0.4,
		Fee:             0,
	})
	if err != nil {
		t.Fatalf("CreateFill zero fee upsert failed: %v", err)
	}
	if unchanged.Fee != 0.1234 {
		t.Fatalf("expected later zero-fee retry to keep real fee, got %v", unchanged.Fee)
	}
	if unchanged.Quantity != 0.4 {
		t.Fatalf("expected zero-fee retry to keep original quantity 0.4, got %v", unchanged.Quantity)
	}
}
