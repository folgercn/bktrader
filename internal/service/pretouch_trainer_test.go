package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPretouchTimingLabelOverridesUsesLedgerTimingAndSpeedGate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unified_trades.csv")
	content := "event_id,timing_prediction,speed_gate_pass,delay_pnl_pct\n" +
		"evt_fast,fast,true,0.12\n" +
		"evt_slow,slow,true,-0.01\n" +
		"evt_blocked,slow,false,0.50\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	overrides, err := loadPretouchTimingLabelOverrides(path)
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}
	if got := overrides["evt_fast"]; got.TimingLabel != "fast" || got.RFLabel != "1" {
		t.Fatalf("expected positive fast override, got %#v", got)
	}
	if got := overrides["evt_slow"]; got.TimingLabel != "slow" || got.RFLabel != "0" {
		t.Fatalf("expected negative slow override, got %#v", got)
	}
	if got := overrides["evt_blocked"]; got.TimingLabel != "skip" || got.RFLabel != "0" {
		t.Fatalf("expected failed speed gate to force skip, got %#v", got)
	}
}

func TestDefaultPretouchTrainerConfigUsesFullLedgerTrainingWindow(t *testing.T) {
	config := DefaultPretouchTrainerConfig()
	if config.TrainRatio != 1.0 {
		t.Fatalf("expected full-window train ratio for live artifact reproducibility, got %v", config.TrainRatio)
	}
	if config.TimingLabelsCSVPath == "" {
		t.Fatalf("expected default timing label ledger path")
	}
}
