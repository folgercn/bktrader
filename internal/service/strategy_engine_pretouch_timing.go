package service

import (
	"fmt"
	"strings"
	"time"
)

const bkLiveEthPretouchTimingEngineKey = "bk-live-eth-pretouch-timing"

// bkLiveEthPretouchTimingEngine implements the ETH pretouch timing strategy.
// It detects pretouch breakout events in real-time, uses Go-native DT3 + RF
// for timing classification and probability sizing, and produces signal intents.
// No Python dependency — single Go binary.
type bkLiveEthPretouchTimingEngine struct {
	platform *Platform
	detector *PretouchEventDetector
	model    *PretouchModelBundle
	config   PretouchDetectorConfig
}

func newBkLiveEthPretouchTimingEngine(platform *Platform) *bkLiveEthPretouchTimingEngine {
	config := DefaultPretouchDetectorConfig()
	engine := &bkLiveEthPretouchTimingEngine{
		platform: platform,
		detector: NewPretouchEventDetector("ETHUSDT", config),
		config:   config,
	}

	// Try to load model bundle from default path
	modelPath := "data/pretouch_model.json"
	if bundle, err := LoadModelBundle(modelPath); err == nil {
		engine.model = bundle
		platform.logger("service.pretouch_timing").Info("pretouch model loaded",
			"version", bundle.Version,
			"timing_loocv", bundle.TimingLOOCV,
			"rf_auc", bundle.RFAUC,
		)
	} else {
		platform.logger("service.pretouch_timing").Warn("pretouch model not found, events will be skipped until model is trained",
			"path", modelPath,
			"error", err,
		)
	}

	return engine
}

func (e *bkLiveEthPretouchTimingEngine) Key() string {
	return bkLiveEthPretouchTimingEngineKey
}

func (e *bkLiveEthPretouchTimingEngine) Describe() map[string]any {
	return map[string]any{
		"key":                 e.Key(),
		"name":               "BK Live ETH Pretouch Timing",
		"supportedSignalBars": []string{"1h"},
		"supportedExecutions": []string{"tick"},
		"runtimeConsistency":  "live-eth-pretouch-timing-unified",
		"symbol":             "ETHUSDT",
		"description":        "ETH pretouch timing strategy: timing classification × RF probability × cost_q50_cut050. Research-validated 10bps kill stress: 23.29%, 0 neg SM.",
	}
}

func (e *bkLiveEthPretouchTimingEngine) Run(ctx StrategyExecutionContext) (map[string]any, error) {
	return nil, fmt.Errorf("pretouch timing engine does not support batch replay; use EvaluateSignal for live")
}

// EvaluateSignal is called on each tick/bar event by the signal runtime.
// It checks for pretouch events and produces signal decisions.
func (e *bkLiveEthPretouchTimingEngine) EvaluateSignal(ctx StrategySignalEvaluationContext) (StrategySignalDecision, error) {
	logger := e.platform.logger("service.pretouch_timing",
		"symbol", ctx.ExecutionContext.Symbol,
	)

	// Only process ETHUSDT
	if !strings.EqualFold(ctx.ExecutionContext.Symbol, "ETHUSDT") {
		return StrategySignalDecision{Action: "wait", Reason: "symbol_not_eth"}, nil
	}

	// Extract tick data from trigger summary
	triggerPrice := parseFloatValue(ctx.TriggerSummary["price"])
	if triggerPrice <= 0 {
		return StrategySignalDecision{Action: "wait", Reason: "no_trigger_price"}, nil
	}

	// Build tick from trigger
	tick := TickData{
		Time:  ctx.EventTime,
		Price: triggerPrice,
	}

	// Check for 1h bar close event (from signal bar state)
	if barClosed := ctx.TriggerSummary["barClosed"]; barClosed != nil {
		if closed, ok := barClosed.(bool); ok && closed {
			barOpen := parseFloatValue(ctx.TriggerSummary["barOpen"])
			barHigh := parseFloatValue(ctx.TriggerSummary["barHigh"])
			barLow := parseFloatValue(ctx.TriggerSummary["barLow"])
			barClose := parseFloatValue(ctx.TriggerSummary["barClose"])
			barOpenTime := ctx.EventTime.Truncate(time.Hour)

			if barOpen > 0 && barHigh > 0 && barLow > 0 && barClose > 0 {
				e.detector.OnHourlyBarClose(HourlyBar{
					OpenTime: barOpenTime.Add(-time.Hour),
					Open:     barOpen,
					High:     barHigh,
					Low:      barLow,
					Close:    barClose,
				})
				// Open new bar
				e.detector.OnNewHourlyBarOpen(barOpenTime, triggerPrice)
			}
		}
	}

	// Process tick through detector
	result := e.detector.OnTick(tick)

	if !result.Detected {
		return StrategySignalDecision{Action: "wait", Reason: result.Reason}, nil
	}

	// --- Pretouch event detected! Do Go-native ML inference ---
	logger.Info("pretouch event detected",
		"event_id", result.Event.EventID,
		"side", result.Event.Side,
		"touch_price", result.Event.TouchPrice,
		"speed_300s", result.Event.Speed300sATR,
		"pre_touch_sec", result.Event.PreTouchSeconds,
	)

	// Check if model is loaded
	if e.model == nil {
		logger.Warn("no model loaded, skipping event",
			"event_id", result.Event.EventID,
		)
		return StrategySignalDecision{
			Action: "wait",
			Reason: "no_model_loaded",
			Metadata: map[string]any{
				"pretouchEvent":      result.Event.EventID,
				"detectedButSkipped": true,
			},
		}, nil
	}

	// Build feature vector in the order expected by the model
	features := make([]float64, len(e.model.FeatureNames))
	for i, name := range e.model.FeatureNames {
		if val, ok := result.Event.Features[name]; ok {
			features[i] = val
		} else if i < len(e.model.Medians) {
			features[i] = e.model.Medians[i] // impute with train median
		}
	}

	// Timing classification (DT3)
	timingRegime := e.model.TimingTree.Predict(features)

	// RF probability
	rfProb := e.model.RFModel.PredictProba(features)

	// Sizing multiplier = clip(prob × 2, 0, 2)
	sizingMultiplier := rfProb * 2
	if sizingMultiplier > 2 {
		sizingMultiplier = 2
	}
	if sizingMultiplier < 0 {
		sizingMultiplier = 0
	}

	// Apply results
	result.Event.TimingRegime = timingRegime
	result.Event.RFProbability = rfProb
	result.Event.SizingMultiplier = sizingMultiplier

	// Timing skip → do not enter
	if timingRegime == "skip" {
		logger.Info("timing classifier: skip",
			"event_id", result.Event.EventID,
			"probability", rfProb,
		)
		return StrategySignalDecision{
			Action: "wait",
			Reason: "timing_skip",
			Metadata: map[string]any{
				"pretouchEvent":    result.Event.EventID,
				"timingRegime":     "skip",
				"rfProbability":    rfProb,
				"modelVersion":     e.model.Version,
			},
		}, nil
	}

	// Compute final position size
	finalMultiplier := sizingMultiplier * result.Event.CostPenalty
	finalPositionSize := e.config.BaseShare * finalMultiplier
	result.Event.FinalPositionSize = finalPositionSize

	logger.Info("pretouch signal: advance-plan",
		"event_id", result.Event.EventID,
		"side", result.Event.Side,
		"timing_regime", timingRegime,
		"rf_probability", rfProb,
		"sizing_multiplier", sizingMultiplier,
		"cost_penalty", result.Event.CostPenalty,
		"final_position_size", finalPositionSize,
		"model_version", e.model.Version,
	)

	// Produce signal decision
	return StrategySignalDecision{
		Action: "advance-plan",
		Reason: fmt.Sprintf("pretouch_%s_%s", timingRegime, result.Event.Side),
		Metadata: map[string]any{
			"pretouchEvent":      result.Event,
			"timingRegime":       timingRegime,
			"rfProbability":      rfProb,
			"sizingMultiplier":   sizingMultiplier,
			"costPenalty":        result.Event.CostPenalty,
			"finalPositionSize":  finalPositionSize,
			"modelVersion":       e.model.Version,
			"signalSource":       "pretouch-timing-unified",
		},
	}, nil
}
