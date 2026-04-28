package memory

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestSavePositionDeletesNonPositiveExistingPosition(t *testing.T) {
	store := NewStore()

	position, err := store.SavePosition(domain.Position{
		AccountID:  "live-main",
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.25,
		EntryPrice: 68000,
		MarkPrice:  68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	position.Quantity = 0
	if _, err := store.SavePosition(position); err != nil {
		t.Fatalf("save zero position failed: %v", err)
	}

	if _, found, err := store.FindPosition("live-main", "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected non-positive position save to delete existing position")
	}
}

func TestListPositionsFiltersNonPositiveDirtyRecords(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	store.positions["position-zero"] = domain.Position{
		ID:        "position-zero",
		AccountID: "live-main",
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0,
		UpdatedAt: now,
	}
	store.positions["position-active"] = domain.Position{
		ID:        "position-active",
		AccountID: "live-main",
		Symbol:    "ETHUSDT",
		Side:      "SHORT",
		Quantity:  0.5,
		UpdatedAt: now.Add(time.Second),
	}

	positions, err := store.ListPositions()
	if err != nil {
		t.Fatalf("list positions failed: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected only active position, got %d: %+v", len(positions), positions)
	}
	if positions[0].ID != "position-active" {
		t.Fatalf("expected active position to remain visible, got %+v", positions[0])
	}
}

func TestQueryPositionsFiltersNonPositiveDirtyRecords(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	store.positions["position-zero"] = domain.Position{
		ID:        "position-zero",
		AccountID: "live-main",
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0,
		UpdatedAt: now,
	}
	store.positions["position-active"] = domain.Position{
		ID:        "position-active",
		AccountID: "live-main",
		Symbol:    "BTCUSDT",
		Side:      "SHORT",
		Quantity:  0.5,
		UpdatedAt: now.Add(time.Second),
	}

	positions, err := store.QueryPositions(domain.PositionQuery{AccountID: "live-main", Symbol: "BTCUSDT"})
	if err != nil {
		t.Fatalf("query positions failed: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected only active query result, got %d: %+v", len(positions), positions)
	}
	if positions[0].ID != "position-active" {
		t.Fatalf("expected active position to remain query-visible, got %+v", positions[0])
	}
}
