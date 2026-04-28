package postgres

import (
	"errors"
	"os"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	storepkg "github.com/wuyaocheng/bktrader/internal/store"
)

func TestWithFillSettlementTxRollsBackFillOnError(t *testing.T) {
	dsn := os.Getenv("BKTRADER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BKTRADER_TEST_POSTGRES_DSN is not set")
	}
	if err := Migrate(dsn); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	store, err := New(dsn)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	account, err := store.GetAccount("paper-default")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	errSentinel := errors.New("stop after fill create")
	err = store.WithFillSettlementTx(func(tx storepkg.FillSettlementStore) error {
		if _, err := tx.CreateFill(domain.Fill{
			OrderID:          order.ID,
			Price:            68000,
			Quantity:         1,
			DedupFingerprint: "rollback-fill-" + order.ID,
		}); err != nil {
			return err
		}
		return errSentinel
	})
	if !errors.Is(err, errSentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	fills, err := store.QueryFills(domain.FillQuery{OrderIDs: []string{order.ID}})
	if err != nil {
		t.Fatalf("QueryFills failed: %v", err)
	}
	if len(fills) != 0 {
		t.Fatalf("expected rollback to remove created fill, got %+v", fills)
	}
}
