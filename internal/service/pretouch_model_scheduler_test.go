package service

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestPretouchModelHotReloadReplacesModelAndPreservesOnInvalidFile(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	engine := platform.pretouchTimingEngine()
	if engine == nil {
		t.Fatal("expected pretouch timing engine")
	}

	path := filepath.Join(t.TempDir(), "pretouch_model.json")
	v1 := pretouchModelSchedulerTestBundle("v1", 0.61)
	if err := SaveModelBundleAtomic(v1, path); err != nil {
		t.Fatalf("save v1 model failed: %v", err)
	}

	var state pretouchModelFileState
	if ok := platform.reloadPretouchModelIfChanged("lead", path, &state, true, engine.setLeadModel); !ok {
		t.Fatal("expected initial lead model reload")
	}
	if got := engine.leadModel(); got == nil || got.Version != "v1" {
		t.Fatalf("expected v1 hot-reloaded model, got %#v", got)
	}

	if err := os.WriteFile(path, []byte(`{"broken":`), 0644); err != nil {
		t.Fatalf("write invalid model failed: %v", err)
	}
	if ok := platform.reloadPretouchModelIfChanged("lead", path, &state, true, engine.setLeadModel); ok {
		t.Fatal("expected invalid model reload to be rejected")
	}
	if got := engine.leadModel(); got == nil || got.Version != "v1" {
		t.Fatalf("expected invalid reload to preserve v1, got %#v", got)
	}

	v2 := pretouchModelSchedulerTestBundle("v2", 0.82)
	if err := SaveModelBundleAtomic(v2, path); err != nil {
		t.Fatalf("save v2 model failed: %v", err)
	}
	if ok := platform.reloadPretouchModelIfChanged("lead", path, &state, false, engine.setLeadModel); !ok {
		t.Fatal("expected changed file to hot reload")
	}
	if got := engine.leadModel(); got == nil || got.Version != "v2" {
		t.Fatalf("expected v2 hot-reloaded model, got %#v", got)
	}
}

func TestPretouchT3OverlayTrainerBuildsLoadableBundle(t *testing.T) {
	dir := t.TempDir()
	tradesPath := filepath.Join(dir, "t3_overlay_trades.csv")
	modelPath := filepath.Join(dir, "pretouch_t3_overlay_rf_model.json")
	if err := os.WriteFile(tradesPath, []byte(testPretouchT3OverlayTradesCSV()), 0644); err != nil {
		t.Fatalf("write T3 overlay trades CSV failed: %v", err)
	}

	err := TrainPretouchT3OverlayModel(PretouchT3OverlayTrainerConfig{
		TradesCSVPath: tradesPath,
		ModelOutPath:  modelPath,
		NEstimatorsRF: 8,
		MaxDepthRF:    3,
		RandomSeed:    7,
	})
	if err != nil {
		t.Fatalf("train T3 overlay model failed: %v", err)
	}

	loaded, err := LoadModelBundle(modelPath)
	if err != nil {
		t.Fatalf("load trained T3 overlay model failed: %v", err)
	}
	if loaded.ArtifactKind != "pretouch_t3_overlay_rf_quality_sizing" {
		t.Fatalf("unexpected artifact kind %s", loaded.ArtifactKind)
	}
	if got := len(loaded.FeatureNames); got != len(pretouchT3OverlayTrainFeatures) {
		t.Fatalf("expected %d features, got %d", len(pretouchT3OverlayTrainFeatures), got)
	}
	if loaded.TrainingRows != 12 {
		t.Fatalf("expected 12 grouped training events, got %d", loaded.TrainingRows)
	}
	if loaded.RFModel == nil || loaded.RFModel.NEstimators != 8 {
		t.Fatalf("expected 8-estimator RF model, got %#v", loaded.RFModel)
	}
	probability := loaded.RFModel.PredictProba([]float64{0.7, 0.4, 0.9, 0.02, 240, 0.1, 0})
	if math.IsNaN(probability) || probability < 0 || probability > 1 {
		t.Fatalf("expected bounded probability, got %v", probability)
	}
}

func pretouchModelSchedulerTestBundle(version string, probability float64) *PretouchModelBundle {
	return &PretouchModelBundle{
		TimingTree: &TreeNode{FeatureIndex: -1, LeafValue: "fast", LeafProba: 1},
		RFModel: &RandomForest{
			Trees:       []*TreeNode{{FeatureIndex: -1, LeafValue: "1", LeafProba: probability}},
			NEstimators: 1,
		},
		FeatureNames: []string{"roundtrip_cost_atr"},
		Medians:      []float64{0.1},
		Version:      version,
		TrainedAt:    "2026-05-20T00:00:00Z",
		RFAccuracy:   probability,
	}
}

func testPretouchT3OverlayTradesCSV() string {
	var b strings.Builder
	b.WriteString("external_event_key,month,side,pnl_initial_pct,rf_probability,speed_300s_atr,eff_300s,touch_extension_atr,pre_touch_seconds,roundtrip_cost_atr\n")
	for i := 0; i < 12; i++ {
		side := "long"
		if i%3 == 0 {
			side = "short"
		}
		pnl := 0.01
		if i%2 == 0 {
			pnl = -0.01
		}
		month := "2026-01"
		if i >= 6 {
			month = "2026-02"
		}
		b.WriteString(fmt.Sprintf(
			"event-%02d,%s,%s,%.6f,%.6f,%.6f,%.6f,%.6f,%.1f,%.6f\n",
			i,
			month,
			side,
			pnl,
			0.35+float64(i)*0.04,
			0.20+float64(i)*0.02,
			0.80+float64(i)*0.01,
			0.01+float64(i)*0.001,
			200+float64(i)*10,
			0.08+float64(i)*0.002,
		))
	}
	return b.String()
}
