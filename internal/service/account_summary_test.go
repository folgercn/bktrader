package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestListAccountSummariesFallsBackToLatestEquitySnapshotForLocalLiveSync(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Binance Testnet", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	account.Metadata = map[string]any{
		"lastLiveSyncAt": "2026-05-18T07:34:06Z",
		"liveSyncSnapshot": map[string]any{
			"source":        "platform-live-reconciliation",
			"syncStatus":    "SYNCED",
			"positionCount": 0,
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	if _, err := platform.store.CreateAccountEquitySnapshot(domain.AccountEquitySnapshot{
		AccountID:         account.ID,
		StartEquity:       5000.00,
		RealizedPnL:       -24.50,
		UnrealizedPnL:     0,
		Fees:              1.01,
		NetEquity:         4974.49,
		ExposureNotional:  0,
		OpenPositionCount: 0,
	}); err != nil {
		t.Fatalf("CreateAccountEquitySnapshot failed: %v", err)
	}

	summaries, err := platform.ListAccountSummaries()
	if err != nil {
		t.Fatalf("ListAccountSummaries failed: %v", err)
	}
	got, ok := accountSummaryByAccountIDForTest(summaries, account.ID)
	if !ok {
		t.Fatalf("expected summary for %s, got %+v", account.ID, summaries)
	}
	if got.NetEquity != 4974.49 {
		t.Fatalf("expected fallback net equity 4974.49, got %.2f", got.NetEquity)
	}
	if got.WalletBalance != 4974.49 || got.MarginBalance != 4974.49 || got.AvailableBalance != 4974.49 {
		t.Fatalf("expected flat-account balances to use snapshot net equity, got wallet=%.2f margin=%.2f available=%.2f", got.WalletBalance, got.MarginBalance, got.AvailableBalance)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected fallback summary updatedAt from latest snapshot")
	}
}

func TestListAccountSummariesUsesLiveRestBalanceSnapshot(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Binance Testnet", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	account.Metadata = map[string]any{
		"lastLiveSyncAt": "2026-05-18T07:34:06Z",
		"liveSyncSnapshot": map[string]any{
			"source":                "binance-rest-account-v3",
			"syncStatus":            "SYNCED",
			"totalWalletBalance":    5020.00,
			"totalUnrealizedProfit": 20.00,
			"totalMarginBalance":    5040.00,
			"availableBalance":      4900.00,
			"positions": []any{
				map[string]any{"notional": -100.5},
			},
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}

	summaries, err := platform.ListAccountSummaries()
	if err != nil {
		t.Fatalf("ListAccountSummaries failed: %v", err)
	}
	got, ok := accountSummaryByAccountIDForTest(summaries, account.ID)
	if !ok {
		t.Fatalf("expected summary for %s, got %+v", account.ID, summaries)
	}
	if got.NetEquity != 5040 || got.WalletBalance != 5020 || got.AvailableBalance != 4900 {
		t.Fatalf("expected live rest balances, got net=%.2f wallet=%.2f available=%.2f", got.NetEquity, got.WalletBalance, got.AvailableBalance)
	}
	if got.ExposureNotional != 100.5 || got.OpenPositionCount != 1 {
		t.Fatalf("expected live rest exposure/count, got %.2f/%d", got.ExposureNotional, got.OpenPositionCount)
	}
}

func accountSummaryByAccountIDForTest(summaries []domain.AccountSummary, accountID string) (domain.AccountSummary, bool) {
	for _, summary := range summaries {
		if summary.AccountID == accountID {
			return summary, true
		}
	}
	return domain.AccountSummary{}, false
}
