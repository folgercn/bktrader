package memory

import (
	"errors"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	storepkg "github.com/wuyaocheng/bktrader/internal/store"
)

func TestWithFillSettlementTxRollsBackFillOnError(t *testing.T) {
	store := NewStore()
	errSentinel := errors.New("stop after fill create")

	err := store.WithFillSettlementTx("", func(tx storepkg.FillSettlementStore) error {
		if _, err := tx.CreateFill(domain.Fill{
			OrderID:          "order-rollback",
			Price:            68000,
			Quantity:         1,
			DedupFingerprint: "rollback-fill",
		}); err != nil {
			return err
		}
		return errSentinel
	})
	if !errors.Is(err, errSentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("ListFills failed: %v", err)
	}
	if len(fills) != 0 {
		t.Fatalf("expected rollback to remove created fill, got %+v", fills)
	}
}
