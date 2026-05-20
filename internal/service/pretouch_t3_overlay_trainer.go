package service

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultPretouchT3OverlayTradesCSVPath = "research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_20260520/t3_overlay_rf_cost_base_trades.csv"

var pretouchT3OverlayTrainFeatures = []string{
	"rf_probability",
	"speed_300s_abs",
	"eff_300s",
	"touch_extension_abs",
	"pre_touch_seconds",
	"roundtrip_cost_atr",
	"side_is_short",
}

// PretouchT3OverlayTrainerConfig holds the production-native T3 overlay RF
// trainer settings. It consumes the research paired-trade CSV, collapses rows
// to one event per external_event_key, then trains the Go-native RF model.
type PretouchT3OverlayTrainerConfig struct {
	TradesCSVPath string
	ModelOutPath  string
	NEstimatorsRF int
	MaxDepthRF    int
	RandomSeed    int64
}

func DefaultPretouchT3OverlayTrainerConfig() PretouchT3OverlayTrainerConfig {
	return PretouchT3OverlayTrainerConfig{
		TradesCSVPath: defaultPretouchT3OverlayTradesCSVPath,
		ModelOutPath:  defaultPretouchT3OverlayModelPath,
		NEstimatorsRF: 240,
		MaxDepthRF:    3,
		RandomSeed:    42,
	}
}

func TrainPretouchT3OverlayModel(config PretouchT3OverlayTrainerConfig) error {
	bundle, err := BuildPretouchT3OverlayModelBundle(config)
	if err != nil {
		return err
	}
	if err := SaveModelBundleAtomic(bundle, config.ModelOutPath); err != nil {
		return fmt.Errorf("save T3 overlay model: %w", err)
	}
	return nil
}

func BuildPretouchT3OverlayModelBundle(config PretouchT3OverlayTrainerConfig) (*PretouchModelBundle, error) {
	events, err := loadPretouchT3OverlayTrainingEvents(config.TradesCSVPath)
	if err != nil {
		return nil, err
	}
	if len(events) < 12 {
		return nil, fmt.Errorf("insufficient T3 overlay events: %d", len(events))
	}
	features, labels, medians := pretouchT3OverlayTrainingMatrix(events)
	if uniqueStringCount(labels) < 2 {
		return nil, fmt.Errorf("T3 overlay labels need at least two classes")
	}
	rng := rand.New(rand.NewSource(config.RandomSeed))
	model := TrainRandomForest(features, labels, config.NEstimatorsRF, config.MaxDepthRF, rng)
	trainedAt := time.Now().UTC()
	return &PretouchModelBundle{
		TimingTree:     &TreeNode{FeatureIndex: -1, LeafValue: "fast", LeafProba: 1.0},
		RFModel:        model,
		FeatureNames:   append([]string(nil), pretouchT3OverlayTrainFeatures...),
		Medians:        medians,
		Version:        trainedAt.Format("20060102T150405Z") + "_t3_overlay_rf_cost_go_v1",
		TrainedAt:      trainedAt.Format(time.RFC3339),
		TimingLOOCV:    1.0,
		RFAccuracy:     computeRFAccuracy(model, features, labels),
		ArtifactKind:   "pretouch_t3_overlay_rf_quality_sizing",
		TrainingSource: config.TradesCSVPath,
		TrainingRows:   len(events),
		TrainingMonths: pretouchT3OverlayTrainingMonths(events),
		RandomState:    config.RandomSeed,
		SizingPolicy: map[string]any{
			"method":                      "t3_rf_cost_quantity_band",
			"min_quantity":                defaultPretouchShadowOverlayQualityMinQty,
			"max_quantity":                defaultPretouchShadowOverlayQualityMaxQty,
			"reference_fixed_overlay_qty": 0.08,
			"cost_threshold_atr":          defaultPretouchShadowOverlayQualityCost,
			"production_native_retrainer": true,
			"source_grouping_key":         "external_event_key",
			"label":                       "sum(pnl_initial_pct) > 0",
		},
		Target: "label_win = event_net_pnl_pct > 0 after strict T3 60m lifecycle pairing",
	}, nil
}

type pretouchT3OverlayTrainingEvent struct {
	Key      string
	Month    string
	Side     string
	PnLPct   float64
	Features map[string]float64
	Order    int
}

func loadPretouchT3OverlayTrainingEvents(path string) ([]pretouchT3OverlayTrainingEvent, error) {
	records, err := loadCSV(path)
	if err != nil {
		return nil, fmt.Errorf("load T3 overlay trades CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("T3 overlay trades CSV has no data rows")
	}
	header := records[0]
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}
	required := []string{
		"external_event_key",
		"month",
		"side",
		"pnl_initial_pct",
		"rf_probability",
		"speed_300s_atr",
		"eff_300s",
		"touch_extension_atr",
		"pre_touch_seconds",
		"roundtrip_cost_atr",
	}
	for _, name := range required {
		if _, ok := colIdx[name]; !ok {
			return nil, fmt.Errorf("T3 overlay trades CSV missing %q column", name)
		}
	}

	eventsByKey := make(map[string]*pretouchT3OverlayTrainingEvent)
	order := make([]string, 0)
	for _, row := range records[1:] {
		key := strings.TrimSpace(csvCell(row, colIdx, "external_event_key"))
		if key == "" {
			continue
		}
		event, exists := eventsByKey[key]
		if !exists {
			event = &pretouchT3OverlayTrainingEvent{
				Key:      key,
				Month:    strings.TrimSpace(csvCell(row, colIdx, "month")),
				Side:     strings.TrimSpace(csvCell(row, colIdx, "side")),
				Features: make(map[string]float64, len(pretouchT3OverlayTrainFeatures)),
				Order:    len(order),
			}
			event.Features["rf_probability"] = parseCSVFloat(row, colIdx, "rf_probability", math.NaN())
			event.Features["speed_300s_abs"] = math.Abs(parseCSVFloat(row, colIdx, "speed_300s_atr", math.NaN()))
			event.Features["eff_300s"] = parseCSVFloat(row, colIdx, "eff_300s", math.NaN())
			event.Features["touch_extension_abs"] = math.Abs(parseCSVFloat(row, colIdx, "touch_extension_atr", math.NaN()))
			event.Features["pre_touch_seconds"] = parseCSVFloat(row, colIdx, "pre_touch_seconds", math.NaN())
			event.Features["roundtrip_cost_atr"] = parseCSVFloat(row, colIdx, "roundtrip_cost_atr", math.NaN())
			if strings.EqualFold(event.Side, "short") {
				event.Features["side_is_short"] = 1.0
			} else {
				event.Features["side_is_short"] = 0.0
			}
			eventsByKey[key] = event
			order = append(order, key)
		}
		event.PnLPct += parseCSVFloat(row, colIdx, "pnl_initial_pct", 0)
	}
	events := make([]pretouchT3OverlayTrainingEvent, 0, len(order))
	for _, key := range order {
		if event := eventsByKey[key]; event != nil {
			events = append(events, *event)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Order < events[j].Order
	})
	return events, nil
}

func pretouchT3OverlayTrainingMatrix(events []pretouchT3OverlayTrainingEvent) ([][]float64, []string, []float64) {
	featureValues := make([][]float64, len(pretouchT3OverlayTrainFeatures))
	for i := range featureValues {
		featureValues[i] = make([]float64, 0, len(events))
	}
	for _, event := range events {
		for i, name := range pretouchT3OverlayTrainFeatures {
			value := event.Features[name]
			if !math.IsNaN(value) && !math.IsInf(value, 0) {
				featureValues[i] = append(featureValues[i], value)
			}
		}
	}
	medians := make([]float64, len(pretouchT3OverlayTrainFeatures))
	for i, values := range featureValues {
		if len(values) == 0 {
			medians[i] = 0
		} else {
			medians[i] = median(values)
		}
	}
	features := make([][]float64, len(events))
	labels := make([]string, len(events))
	for i, event := range events {
		row := make([]float64, len(pretouchT3OverlayTrainFeatures))
		for j, name := range pretouchT3OverlayTrainFeatures {
			value := event.Features[name]
			if math.IsNaN(value) || math.IsInf(value, 0) {
				value = medians[j]
			}
			row[j] = value
		}
		features[i] = row
		if event.PnLPct > 0 {
			labels[i] = "1"
		} else {
			labels[i] = "0"
		}
	}
	return features, labels, medians
}

func pretouchT3OverlayTrainingMonths(events []pretouchT3OverlayTrainingEvent) []string {
	seen := make(map[string]struct{})
	months := make([]string, 0)
	for _, event := range events {
		month := strings.TrimSpace(event.Month)
		if month == "" {
			continue
		}
		if _, ok := seen[month]; ok {
			continue
		}
		seen[month] = struct{}{}
		months = append(months, month)
	}
	sort.Strings(months)
	return months
}

func uniqueStringCount(values []string) int {
	seen := make(map[string]struct{})
	for _, value := range values {
		seen[value] = struct{}{}
	}
	return len(seen)
}

func csvCell(row []string, colIdx map[string]int, name string) string {
	idx, ok := colIdx[name]
	if !ok || idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func parseCSVFloat(row []string, colIdx map[string]int, name string, fallback float64) float64 {
	value := strings.TrimSpace(csvCell(row, colIdx, name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
