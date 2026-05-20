package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTreeNodePredictAndPredictProba(t *testing.T) {
	tree := &TreeNode{
		FeatureIndex: 0,
		Threshold:    1.0,
		Left:         &TreeNode{FeatureIndex: -1, LeafValue: "fast", LeafProba: 0.8},
		Right:        &TreeNode{FeatureIndex: -1, LeafValue: "skip", LeafProba: 0.2},
	}
	if got := tree.Predict([]float64{0.5}); got != "fast" {
		t.Fatalf("expected fast, got %s", got)
	}
	if got := tree.PredictProba([]float64{2.0}); got != 0.2 {
		t.Fatalf("expected right leaf probability 0.2, got %v", got)
	}
}

func TestTreeNodeBadFeatureLengthDoesNotPanic(t *testing.T) {
	tree := &TreeNode{
		FeatureIndex: 3,
		Threshold:    1.0,
		Left:         &TreeNode{FeatureIndex: -1, LeafValue: "fast", LeafProba: 0.8},
		Right:        &TreeNode{FeatureIndex: -1, LeafValue: "skip", LeafProba: 0.2},
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Predict/PredictProba should not panic on short features: %v", recovered)
		}
	}()
	if got := tree.Predict([]float64{0.5}); got != "" {
		t.Fatalf("expected empty fallback class for invalid feature index, got %q", got)
	}
	if got := tree.PredictProba([]float64{0.5}); got != 0.5 {
		t.Fatalf("expected neutral probability for invalid feature index, got %v", got)
	}
}

func TestRandomForestHandlesEmptyAndNilTrees(t *testing.T) {
	var nilForest *RandomForest
	if got := nilForest.PredictProba([]float64{1}); got != 0.5 {
		t.Fatalf("expected nil forest probability fallback, got %v", got)
	}
	forest := &RandomForest{
		Trees: []*TreeNode{
			nil,
			{FeatureIndex: -1, LeafValue: "1", LeafProba: 0.75},
		},
	}
	if got := forest.PredictProba([]float64{1}); got != 0.75 {
		t.Fatalf("expected probability from non-nil tree only, got %v", got)
	}
	if got := forest.Predict([]float64{1}); got != "1" {
		t.Fatalf("expected class from non-nil tree only, got %s", got)
	}
}

func TestPretouchModelBundleSaveLoadAndLegacyRFAUC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "model.json")
	bundle := &PretouchModelBundle{
		TimingTree:   &TreeNode{FeatureIndex: -1, LeafValue: "fast", LeafProba: 1},
		RFModel:      &RandomForest{Trees: []*TreeNode{{FeatureIndex: -1, LeafValue: "1", LeafProba: 0.7}}, NEstimators: 1},
		FeatureNames: []string{"roundtrip_cost_atr"},
		Medians:      []float64{0.1},
		Version:      "test",
		TrainedAt:    "2026-05-15T00:00:00Z",
		RFAccuracy:   0.7,
	}
	if err := SaveModelBundle(bundle, path); err != nil {
		t.Fatalf("save model bundle failed: %v", err)
	}
	loaded, err := LoadModelBundle(path)
	if err != nil {
		t.Fatalf("load model bundle failed: %v", err)
	}
	if loaded.RFAccuracy != 0.7 {
		t.Fatalf("expected RFAccuracy 0.7, got %v", loaded.RFAccuracy)
	}

	legacy := clonePretouchBundleForTest(bundle)
	legacy.RFAccuracy = 0
	legacy.RFAUC = 0.66
	legacyPath := filepath.Join(dir, "legacy.json")
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy model failed: %v", err)
	}
	if err := os.WriteFile(legacyPath, data, 0644); err != nil {
		t.Fatalf("write legacy model failed: %v", err)
	}
	loadedLegacy, err := LoadModelBundle(legacyPath)
	if err != nil {
		t.Fatalf("load legacy model failed: %v", err)
	}
	if loadedLegacy.RFAccuracy != 0.66 {
		t.Fatalf("expected legacy rf_auc to populate RFAccuracy, got %v", loadedLegacy.RFAccuracy)
	}
}

func TestPretouchModelBundleRejectsFeatureMismatch(t *testing.T) {
	bundle := &PretouchModelBundle{
		TimingTree:   &TreeNode{FeatureIndex: 2, Threshold: 1, Left: &TreeNode{FeatureIndex: -1, LeafValue: "fast"}},
		RFModel:      &RandomForest{Trees: []*TreeNode{{FeatureIndex: -1, LeafValue: "1", LeafProba: 0.7}}},
		FeatureNames: []string{"a"},
		Medians:      []float64{0},
	}
	if err := SaveModelBundle(bundle, filepath.Join(t.TempDir(), "model.json")); err == nil {
		t.Fatalf("expected feature-index mismatch to reject model")
	}
}

func TestPretouchT3OverlayModelArtifactLoads(t *testing.T) {
	path := filepath.Join("..", "..", "data", "pretouch_t3_overlay_rf_model.json")
	bundle, err := LoadModelBundle(path)
	if err != nil {
		t.Fatalf("load T3 overlay model artifact failed: %v", err)
	}
	if bundle.Version != "20260520_t3_overlay_rf_cost_v1" {
		t.Fatalf("unexpected T3 overlay model version %s", bundle.Version)
	}
	wantFeatures := []string{
		"rf_probability",
		"speed_300s_abs",
		"eff_300s",
		"touch_extension_abs",
		"pre_touch_seconds",
		"roundtrip_cost_atr",
		"side_is_short",
	}
	if len(bundle.FeatureNames) != len(wantFeatures) {
		t.Fatalf("expected %d features, got %#v", len(wantFeatures), bundle.FeatureNames)
	}
	for i, want := range wantFeatures {
		if bundle.FeatureNames[i] != want {
			t.Fatalf("expected feature %d to be %s, got %s", i, want, bundle.FeatureNames[i])
		}
	}
	if bundle.RFModel == nil || len(bundle.RFModel.Trees) != 240 {
		t.Fatalf("expected 240-tree T3 overlay RF model, got %#v", bundle.RFModel)
	}
	probability := bundle.RFModel.PredictProba([]float64{0.56, 0.45, 0.93, 0.04, 386, 0.10, 1})
	if probability < 0 || probability > 1 {
		t.Fatalf("expected bounded RF probability, got %v", probability)
	}
}

func clonePretouchBundleForTest(bundle *PretouchModelBundle) *PretouchModelBundle {
	data, _ := json.Marshal(bundle)
	var out PretouchModelBundle
	_ = json.Unmarshal(data, &out)
	return &out
}
