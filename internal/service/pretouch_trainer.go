package service

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PretouchTrainerConfig holds training configuration.
type PretouchTrainerConfig struct {
	EventsCSVPath       string // path to canonical events CSV
	TimingLabelsCSVPath string // optional research ledger with event_id/timing_prediction/speed_gate_pass
	ModelOutPath        string // output path for model JSON
	ForwardStart        string // "2025-11-01"
	TrainRatio          float64
	MaxDepthDT          int // 3
	NEstimatorsRF       int // 200
	RandomSeed          int64
}

// DefaultPretouchTrainerConfig returns the default training config.
func DefaultPretouchTrainerConfig() PretouchTrainerConfig {
	return PretouchTrainerConfig{
		EventsCSVPath:       "research/tick_flow_event_sources/20260514_pretouch_full_window/feature_filtered_seed_events/robust_quality/pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv",
		TimingLabelsCSVPath: "research/entry_redesign/scripts/output/timing_probability_unified/unified_trades.csv",
		ModelOutPath:        "data/pretouch_model.json",
		ForwardStart:        "2025-11-01",
		TrainRatio:          1.0,
		MaxDepthDT:          3,
		NEstimatorsRF:       200,
		RandomSeed:          42,
	}
}

// Feature names used for training (subset available in CSV without NaN)
var pretouchTrainFeatures = []string{
	"roundtrip_cost_atr",
	"prev1_range_atr",
	"prev1_close_pos_side",
	"level_to_signal_open_atr",
	"touch_extension_atr",
	"speed_300s_atr",
	"eff_300s",
	"pre_touch_seconds",
}

// TrainPretouchModel trains DT3 timing classifier + RF probability model
// from the canonical events CSV and saves the model bundle as JSON.
func TrainPretouchModel(config PretouchTrainerConfig) error {
	// Load CSV
	records, err := loadCSV(config.EventsCSVPath)
	if err != nil {
		return fmt.Errorf("load CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV has no data rows")
	}

	header := records[0]
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[col] = i
	}

	// Filter ETH only
	symbolIdx, ok := colIdx["symbol"]
	if !ok {
		return fmt.Errorf("CSV missing 'symbol' column")
	}

	var ethRows [][]string
	for _, row := range records[1:] {
		if strings.EqualFold(row[symbolIdx], "ETHUSDT") {
			ethRows = append(ethRows, row)
		}
	}

	if len(ethRows) == 0 {
		return fmt.Errorf("no ETHUSDT events in CSV")
	}

	// Sort by touch_time and split
	touchTimeIdx := colIdx["touch_time"]
	// Already sorted in CSV, but let's trust it

	// Forward split
	forwardStart, _ := time.Parse("2006-01-02", config.ForwardStart)
	var fullWindowRows [][]string
	for _, row := range ethRows {
		tt, err := time.Parse(time.RFC3339, row[touchTimeIdx])
		if err != nil {
			// Try other formats
			tt, err = time.Parse("2006-01-02 15:04:05+00:00", row[touchTimeIdx])
			if err != nil {
				continue
			}
		}
		if tt.Before(forwardStart) {
			fullWindowRows = append(fullWindowRows, row)
		}
	}

	if len(fullWindowRows) < 10 {
		return fmt.Errorf("insufficient full-window events: %d", len(fullWindowRows))
	}

	// Train/test split
	splitIdx := int(float64(len(fullWindowRows)) * config.TrainRatio)
	trainRows := fullWindowRows[:splitIdx]
	testRows := fullWindowRows[splitIdx:]

	labelOverrides, err := loadPretouchTimingLabelOverrides(config.TimingLabelsCSVPath)
	if err != nil {
		return fmt.Errorf("load timing labels: %w", err)
	}

	// Extract features and labels
	trainX, trainTimingY, trainRFY, trainMedians := extractTrainingData(trainRows, colIdx, pretouchTrainFeatures, labelOverrides)
	testX, _, testRFY, _ := extractTrainingData(testRows, colIdx, pretouchTrainFeatures, labelOverrides)

	// Impute test with train medians
	for i := range testX {
		for j := range testX[i] {
			if math.IsNaN(testX[i][j]) {
				testX[i][j] = trainMedians[j]
			}
		}
	}

	// Train DT3 timing classifier
	rng := rand.New(rand.NewSource(config.RandomSeed))
	timingTree := TrainDecisionTree(trainX, trainTimingY, config.MaxDepthDT, rng)

	// Compute timing LOOCV accuracy with an independent RNG so the metric does
	// not depend on random draws consumed by the final model fit.
	loocvRng := rand.New(rand.NewSource(config.RandomSeed + 1))
	loocvAccuracy := computeLOOCVAccuracy(trainX, trainTimingY, config.MaxDepthDT, loocvRng)

	// Train RF probability model
	rfRng := rand.New(rand.NewSource(config.RandomSeed))
	rfModel := TrainRandomForest(trainX, trainRFY, config.NEstimatorsRF, 5, rfRng)

	rfAccuracy := computeRFAccuracy(rfModel, testX, testRFY)

	// Build model bundle
	bundle := &PretouchModelBundle{
		TimingTree:     timingTree,
		RFModel:        rfModel,
		FeatureNames:   pretouchTrainFeatures,
		Medians:        trainMedians,
		Version:        time.Now().UTC().Format("20060102") + "_v1",
		TrainedAt:      time.Now().UTC().Format(time.RFC3339),
		TimingLOOCV:    loocvAccuracy,
		RFAccuracy:     rfAccuracy,
		ArtifactKind:   "pretouch_lead_timing_slow_aware",
		TrainingSource: config.EventsCSVPath,
	}

	// Save
	if err := SaveModelBundleAtomic(bundle, config.ModelOutPath); err != nil {
		return fmt.Errorf("save model: %w", err)
	}

	return nil
}

func loadCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	return reader.ReadAll()
}

type pretouchTimingLabelOverride struct {
	TimingLabel string
	RFLabel     string
}

func loadPretouchTimingLabelOverrides(path string) (map[string]pretouchTimingLabelOverride, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	records, err := loadCSV(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}
	header := records[0]
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[col] = i
	}
	eventIDIdx, ok := colIdx["event_id"]
	if !ok {
		return nil, fmt.Errorf("timing label CSV missing event_id")
	}
	timingIdx, ok := colIdx["timing_prediction"]
	if !ok {
		return nil, fmt.Errorf("timing label CSV missing timing_prediction")
	}
	speedGateIdx, hasSpeedGate := colIdx["speed_gate_pass"]
	delayPnlIdx, hasDelayPnl := colIdx["delay_pnl_pct"]
	out := make(map[string]pretouchTimingLabelOverride, len(records)-1)
	for _, row := range records[1:] {
		if eventIDIdx >= len(row) || timingIdx >= len(row) {
			continue
		}
		eventID := strings.TrimSpace(row[eventIDIdx])
		if eventID == "" {
			continue
		}
		label := normalizePretouchTrainingTimingLabel(row[timingIdx])
		if hasSpeedGate && speedGateIdx < len(row) && !parseCSVBool(row[speedGateIdx]) {
			label = "skip"
		}
		if label == "" {
			continue
		}
		rfLabel := "0"
		if label != "skip" && hasDelayPnl && delayPnlIdx < len(row) {
			pnl, _ := strconv.ParseFloat(strings.TrimSpace(row[delayPnlIdx]), 64)
			if pnl > 0 {
				rfLabel = "1"
			}
		}
		out[eventID] = pretouchTimingLabelOverride{
			TimingLabel: label,
			RFLabel:     rfLabel,
		}
	}
	return out, nil
}

func normalizePretouchTrainingTimingLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fast", "slow", "skip":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func parseCSVBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}

func extractTrainingData(rows [][]string, colIdx map[string]int, featureNames []string, labelOverrides map[string]pretouchTimingLabelOverride) ([][]float64, []string, []string, []float64) {
	X := make([][]float64, len(rows))
	timingY := make([]string, len(rows))
	rfY := make([]string, len(rows))

	// Collect values for median computation
	featureVals := make([][]float64, len(featureNames))
	for i := range featureVals {
		featureVals[i] = make([]float64, 0, len(rows))
	}

	execReturnIdx, hasExecReturn := colIdx["execution_return_pct"]
	execWinIdx, hasExecWin := colIdx["execution_win"]
	eventIDIdx, hasEventID := colIdx["event_id"]

	for i, row := range rows {
		features := make([]float64, len(featureNames))
		for j, name := range featureNames {
			idx, ok := colIdx[name]
			if !ok {
				features[j] = math.NaN()
				continue
			}
			val, err := strconv.ParseFloat(row[idx], 64)
			if err != nil {
				features[j] = math.NaN()
			} else {
				features[j] = val
				featureVals[j] = append(featureVals[j], val)
			}
		}
		X[i] = features

		if hasEventID && eventIDIdx < len(row) {
			if override, ok := labelOverrides[strings.TrimSpace(row[eventIDIdx])]; ok {
				timingY[i] = override.TimingLabel
				rfY[i] = override.RFLabel
				continue
			}
		}

		// Fallback timing label for rows outside the research ledger:
		// positive → "fast", negative → "skip".
		if hasExecReturn {
			ret, err := strconv.ParseFloat(row[execReturnIdx], 64)
			if err != nil || ret < 0 {
				timingY[i] = "skip"
			} else {
				timingY[i] = "fast"
			}
		} else {
			timingY[i] = "fast" // default if no label available
		}

		// RF label: binary win/loss
		if hasExecWin {
			if strings.TrimSpace(row[execWinIdx]) == "True" || strings.TrimSpace(row[execWinIdx]) == "1" {
				rfY[i] = "1"
			} else {
				rfY[i] = "0"
			}
		} else if hasExecReturn {
			ret, _ := strconv.ParseFloat(row[execReturnIdx], 64)
			if ret > 0 {
				rfY[i] = "1"
			} else {
				rfY[i] = "0"
			}
		} else {
			rfY[i] = "0"
		}
	}

	// Compute medians
	medians := make([]float64, len(featureNames))
	for j, vals := range featureVals {
		if len(vals) == 0 {
			medians[j] = 0
		} else {
			medians[j] = median(vals)
		}
	}

	// Impute NaN with medians
	for i := range X {
		for j := range X[i] {
			if math.IsNaN(X[i][j]) {
				X[i][j] = medians[j]
			}
		}
	}

	return X, timingY, rfY, medians
}

func median(vals []float64) float64 {
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func computeLOOCVAccuracy(X [][]float64, y []string, maxDepth int, rng *rand.Rand) float64 {
	n := len(X)
	if n < 3 {
		return 0
	}

	correct := 0
	for i := 0; i < n; i++ {
		// Leave one out
		trainX := make([][]float64, 0, n-1)
		trainY := make([]string, 0, n-1)
		for j := 0; j < n; j++ {
			if j != i {
				trainX = append(trainX, X[j])
				trainY = append(trainY, y[j])
			}
		}

		tree := TrainDecisionTree(trainX, trainY, maxDepth, rng)
		pred := tree.Predict(X[i])
		if pred == y[i] {
			correct++
		}
	}

	return float64(correct) / float64(n)
}

func computeRFAccuracy(rf *RandomForest, X [][]float64, y []string) float64 {
	if len(X) == 0 {
		return 0.5
	}
	correct := 0
	for i, features := range X {
		pred := rf.Predict(features)
		if pred == y[i] {
			correct++
		}
	}
	return float64(correct) / float64(len(X))
}
