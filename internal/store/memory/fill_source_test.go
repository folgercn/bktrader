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
