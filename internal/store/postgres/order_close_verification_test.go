package postgres

import (
	"os"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestQueryOrderCloseVerificationsFiltersAndSortsLatest(t *testing.T) {
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

	eventTime := time.Date(2026, 4, 26, 3, 36, 23, 0, time.UTC)
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	accountID := "live-main-close-verif-" + suffix
	items := []domain.OrderCloseVerification{
		{
			ID:                   "verification-old-closed-" + suffix,
			LiveSessionID:        "live-session-1-" + suffix,
			OrderID:              "order-old-" + suffix,
			AccountID:            accountID,
			StrategyID:           "strategy-bk-1d",
			Symbol:               "btcusdt",
			VerifiedClosed:       true,
			RemainingPositionQty: 0,
			VerificationSource:   "ws-sync",
			EventTime:            eventTime,
			RecordedAt:           eventTime,
		},
		{
			ID:                   "verification-new-residual-" + suffix,
			LiveSessionID:        "live-session-1-" + suffix,
			OrderID:              "order-new-" + suffix,
			AccountID:            accountID,
			StrategyID:           "strategy-bk-1d",
			Symbol:               "BTCUSDT",
			VerifiedClosed:       false,
			RemainingPositionQty: 0.0013,
			VerificationSource:   "reconcile",
			EventTime:            eventTime.Add(time.Second),
			RecordedAt:           eventTime.Add(time.Second),
		},
		{
			ID:                   "verification-other-strategy-" + suffix,
			LiveSessionID:        "live-session-other-" + suffix,
			OrderID:              "order-other-" + suffix,
			AccountID:            accountID,
			StrategyID:           "strategy-other",
			Symbol:               "BTCUSDT",
			VerifiedClosed:       true,
			RemainingPositionQty: 0,
			VerificationSource:   "ws-sync",
			EventTime:            eventTime.Add(2 * time.Second),
			RecordedAt:           eventTime.Add(2 * time.Second),
		},
	}
	for _, item := range items {
		if _, err := store.CreateOrderCloseVerification(item); err != nil {
			t.Fatalf("CreateOrderCloseVerification failed: %v", err)
		}
	}

	got, err := store.QueryOrderCloseVerifications(domain.OrderCloseVerificationQuery{
		AccountID:  accountID,
		StrategyID: "strategy-bk-1d",
		Symbol:     "btcusdt",
		Limit:      1,
	})
	if err != nil {
		t.Fatalf("QueryOrderCloseVerifications failed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "verification-new-residual-"+suffix {
		t.Fatalf("expected latest same-strategy residual verification, got %#v", got)
	}
	if got[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected stored symbol to be normalized, got %s", got[0].Symbol)
	}
}
