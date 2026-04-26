package service

import (
	"testing"
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
