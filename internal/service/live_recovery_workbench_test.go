package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestClassifyRecoveryMismatches(t *testing.T) {
	p := &Platform{}

	// Scenario 1: Stale Position
	dbFact := LiveRecoveryFact{
		Position: map[string]any{"side": "LONG", "quantity": 0.1},
	}
	exFact := LiveRecoveryFact{
		Position: map[string]any{"side": "BOTH", "quantity": 0.0},
	}

	mismatches := p.classifyRecoveryMismatches(dbFact, exFact)
	if len(mismatches) != 1 {
		t.Errorf("expected 1 mismatch, got %d", len(mismatches))
	} else if mismatches[0].Scenario != "exchange-flat-db-position-present" {
		t.Errorf("expected scenario exchange-flat-db-position-present, got %s", mismatches[0].Scenario)
	}

	// Scenario 2: Missing Position
	dbFact2 := LiveRecoveryFact{
		Position: map[string]any{"side": "BOTH", "quantity": 0.0},
	}
	exFact2 := LiveRecoveryFact{
		Position: map[string]any{"side": "SHORT", "quantity": 0.5},
	}
	mismatches2 := p.classifyRecoveryMismatches(dbFact2, exFact2)
	if len(mismatches2) != 1 {
		t.Errorf("expected 1 mismatch, got %d", len(mismatches2))
	} else if mismatches2[0].Scenario != "exchange-position-db-missing" {
		t.Errorf("expected scenario exchange-position-db-missing, got %s", mismatches2[0].Scenario)
	}

	// Scenario 3: Quantity Mismatch
	dbFact3 := LiveRecoveryFact{
		Position: map[string]any{"side": "LONG", "quantity": 0.1},
	}
	exFact3 := LiveRecoveryFact{
		Position: map[string]any{"side": "LONG", "quantity": 0.15},
	}
	mismatches3 := p.classifyRecoveryMismatches(dbFact3, exFact3)
	if len(mismatches3) != 1 {
		t.Errorf("expected 1 mismatch, got %d", len(mismatches3))
	} else if mismatches3[0].Scenario != "quantity-mismatch" {
		t.Errorf("expected scenario quantity-mismatch, got %s", mismatches3[0].Scenario)
	}
}

func TestScenarioClassification(t *testing.T) {
	p := &Platform{}

	// Scenario 4: Side Conflict
	dbFact4 := LiveRecoveryFact{
		Position: map[string]any{"side": "LONG", "quantity": 0.1},
	}
	exFact4 := LiveRecoveryFact{
		Position: map[string]any{"side": "SHORT", "quantity": 0.1},
	}
	mismatches4 := p.classifyRecoveryMismatches(dbFact4, exFact4)
	if len(mismatches4) != 1 {
		t.Errorf("expected 1 mismatch, got %d", len(mismatches4))
	} else if mismatches4[0].Scenario != "side-conflict" {
		t.Errorf("expected scenario side-conflict, got %s", mismatches4[0].Scenario)
	}
}

func TestDiagnoseLiveRecoveryTreatsMissingSnapshotSymbolAsFlat(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapterKey := "test-recovery-flat-snapshot"
	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: adapterKey,
		syncSnapshotFunc: func(p *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
			previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
			account.Metadata = cloneMetadata(account.Metadata)
			account.Metadata["liveSyncSnapshot"] = map[string]any{
				"source":          "binance-rest-account-v3",
				"adapterKey":      adapterKey,
				"syncedAt":        time.Now().UTC().Format(time.RFC3339),
				"bindingMode":     "api-key-ref",
				"executionMode":   "rest",
				"syncStatus":      "SYNCED",
				"accountExchange": account.Exchange,
				"positions":       []map[string]any{},
				"openOrders":      []map[string]any{},
			}
			return p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
		},
	})

	account, err := store.CreateAccount("live-main", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	account.Metadata = map[string]any{
		"liveBinding": map[string]any{
			"adapterKey":     adapterKey,
			"connectionMode": "api-key-ref",
			"executionMode":  "rest",
		},
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:  account.ID,
		Symbol:     "BTCUSDT",
		Side:       "LONG",
		Quantity:   0.1,
		EntryPrice: 68000,
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	diag, err := platform.DiagnoseLiveRecovery(context.Background(), LiveRecoveryDiagnoseOptions{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
	})
	if err != nil {
		t.Fatalf("diagnose failed: %v", err)
	}
	if !diag.Authoritative {
		t.Fatal("expected authoritative diagnosis")
	}
	if got := parseFloatValue(diag.ExchangeFact.Position["quantity"]); got != 0 {
		t.Fatalf("expected missing exchange symbol to be treated as flat, got quantity %v", got)
	}
	if len(diag.Mismatches) == 0 || diag.Mismatches[0].Scenario != "exchange-flat-db-position-present" {
		t.Fatalf("expected stale DB position mismatch, got %#v", diag.Mismatches)
	}
	if diag.Status != "warning" {
		t.Fatalf("expected warning status, got %s", diag.Status)
	}
}

func TestDiagnoseLiveRecoveryReturnsNonAuthoritativeReportOnExchangeFailure(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapterKey := "test-recovery-exchange-failure"
	platform.registerLiveAdapter(testLiveAccountReconcileAdapter{
		key: adapterKey,
		syncSnapshotFunc: func(*Platform, domain.Account, map[string]any) (domain.Account, error) {
			return domain.Account{}, errors.New("binance account sync failed: test outage")
		},
	})

	account, err := store.CreateAccount("live-main", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	account.Metadata = map[string]any{
		"liveBinding": map[string]any{
			"adapterKey":     adapterKey,
			"connectionMode": "api-key-ref",
			"executionMode":  "rest",
		},
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	diag, err := platform.DiagnoseLiveRecovery(context.Background(), LiveRecoveryDiagnoseOptions{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
	})
	if err != nil {
		t.Fatalf("diagnose should return a report instead of error: %v", err)
	}
	if diag.Authoritative {
		t.Fatal("expected non-authoritative diagnosis")
	}
	if diag.Status != "error" {
		t.Fatalf("expected error status, got %s", diag.Status)
	}
	if diag.Error == nil || diag.Error.Stage != "fetch-exchange-fact" {
		t.Fatalf("expected fetch-exchange-fact diagnostic error, got %#v", diag.Error)
	}
	if len(diag.Mismatches) == 0 || diag.Mismatches[0].Scenario != "non-authoritative" {
		t.Fatalf("expected non-authoritative mismatch, got %#v", diag.Mismatches)
	}
	if len(diag.Actions) == 0 {
		t.Fatal("expected diagnostic actions to explain blocked recovery options")
	}
	for _, action := range diag.Actions {
		if action.Allowed {
			t.Fatalf("expected non-authoritative action %s to be blocked", action.Action)
		}
		if action.BlockedBy != "non-authoritative-diagnosis" {
			t.Fatalf("expected non-authoritative block reason, got %s for %s", action.BlockedBy, action.Action)
		}
	}

	_, err = platform.ExecuteLiveRecoveryAction(context.Background(), account.ID, "reconcile", map[string]any{
		"symbol": "BTCUSDT",
	})
	if err == nil {
		t.Fatal("expected execute action to reject non-authoritative diagnosis")
	}
	if !strings.Contains(err.Error(), "non-authoritative diagnosis") {
		t.Fatalf("expected non-authoritative execution error, got %v", err)
	}
}
