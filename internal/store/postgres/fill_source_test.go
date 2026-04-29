package postgres

import (
	"os"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	storepkg "github.com/wuyaocheng/bktrader/internal/store"
)

func TestCreateFillPersistsSourcePostgres(t *testing.T) {
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

	tradeID := "trade-source-" + time.Now().UTC().Format("20060102150405.000000000")
	created, err := store.CreateFill(domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: tradeID,
		Source:          "real",
		Price:           68000,
		Quantity:        0.4,
	})
	if err != nil {
		t.Fatalf("CreateFill failed: %v", err)
	}
	if created.Source != "real" {
		t.Fatalf("expected created source real, got %q", created.Source)
	}

	fills, err := store.QueryFills(domain.FillQuery{OrderIDs: []string{order.ID}})
	if err != nil {
		t.Fatalf("QueryFills failed: %v", err)
	}
	if len(fills) != 1 || fills[0].Source != "real" {
		t.Fatalf("expected queried fill source real, got %+v", fills)
	}
}

func TestCreateFillUpsertUpdatesSourcePostgres(t *testing.T) {
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

	fingerprint := "source-upsert-" + time.Now().UTC().Format("20060102150405.000000000")
	if _, err := store.CreateFill(domain.Fill{
		OrderID:          order.ID,
		Source:           "synthetic",
		Price:            68000,
		Quantity:         0.4,
		DedupFingerprint: fingerprint,
	}); err != nil {
		t.Fatalf("CreateFill synthetic failed: %v", err)
	}
	updated, err := store.CreateFill(domain.Fill{
		OrderID:          order.ID,
		Source:           "remainder",
		Price:            68000,
		Quantity:         0.4,
		DedupFingerprint: fingerprint,
	})
	if err != nil {
		t.Fatalf("CreateFill remainder upsert failed: %v", err)
	}
	if updated.Source != "remainder" {
		t.Fatalf("expected upserted fallback source remainder, got %q", updated.Source)
	}

	tradeID := "trade-upsert-" + time.Now().UTC().Format("20060102150405.000000000")
	if _, err := store.CreateFill(domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: tradeID,
		Source:          "real",
		Price:           68000,
		Quantity:        0.1,
	}); err != nil {
		t.Fatalf("CreateFill real failed: %v", err)
	}
	manual, err := store.CreateFill(domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: tradeID,
		Source:          "manual",
		Price:           68000,
		Quantity:        0.1,
	})
	if err != nil {
		t.Fatalf("CreateFill real source upsert failed: %v", err)
	}
	if manual.Source != "manual" {
		t.Fatalf("expected real upsert source manual, got %q", manual.Source)
	}
}

func TestFillSettlementTxCreateFillUpsertUpdatesSourcePostgres(t *testing.T) {
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

	fingerprint := "tx-source-upsert-" + time.Now().UTC().Format("20060102150405.000000000")
	if err := store.WithFillSettlementTx(order.ID, func(tx storepkg.FillSettlementStore) error {
		if _, err := tx.CreateFill(domain.Fill{
			OrderID:          order.ID,
			Source:           "synthetic",
			Price:            68000,
			Quantity:         0.4,
			DedupFingerprint: fingerprint,
		}); err != nil {
			return err
		}
		updated, err := tx.CreateFill(domain.Fill{
			OrderID:          order.ID,
			Source:           "remainder",
			Price:            68000,
			Quantity:         0.4,
			DedupFingerprint: fingerprint,
		})
		if err != nil {
			return err
		}
		if updated.Source != "remainder" {
			t.Fatalf("expected tx upserted source remainder, got %q", updated.Source)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithFillSettlementTx failed: %v", err)
	}

	fills, err := store.QueryFills(domain.FillQuery{OrderIDs: []string{order.ID}})
	if err != nil {
		t.Fatalf("QueryFills failed: %v", err)
	}
	if len(fills) != 1 || fills[0].Source != "remainder" {
		t.Fatalf("expected persisted tx source remainder, got %+v", fills)
	}
}
