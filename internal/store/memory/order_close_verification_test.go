package memory

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestQueryOrderCloseVerificationsFiltersAndSortsLatest(t *testing.T) {
	store := NewStore()
	eventTime := time.Date(2026, 4, 26, 3, 36, 23, 0, time.UTC)

	items := []domain.OrderCloseVerification{
		{
			ID:                   "verification-old-closed",
			LiveSessionID:        "live-session-1",
			OrderID:              "order-old",
			AccountID:            "live-main",
			StrategyID:           "strategy-bk-1d",
			Symbol:               "btcusdt",
			VerifiedClosed:       true,
			RemainingPositionQty: 0,
			VerificationSource:   "ws-sync",
			EventTime:            eventTime,
			RecordedAt:           eventTime,
		},
		{
			ID:                   "verification-new-residual",
			LiveSessionID:        "live-session-1",
			OrderID:              "order-new",
			AccountID:            "live-main",
			StrategyID:           "strategy-bk-1d",
			Symbol:               "BTCUSDT",
			VerifiedClosed:       false,
			RemainingPositionQty: 0.0013,
			VerificationSource:   "reconcile",
			EventTime:            eventTime.Add(time.Second),
			RecordedAt:           eventTime.Add(time.Second),
		},
		{
			ID:                   "verification-other-strategy",
			LiveSessionID:        "live-session-other",
			OrderID:              "order-other",
			AccountID:            "live-main",
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
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Symbol:     "btcusdt",
		Limit:      1,
	})
	if err != nil {
		t.Fatalf("QueryOrderCloseVerifications failed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "verification-new-residual" {
		t.Fatalf("expected latest same-strategy residual verification, got %#v", got)
	}
	if got[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected stored symbol to be normalized, got %s", got[0].Symbol)
	}
}
