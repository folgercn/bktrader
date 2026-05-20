package service

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	bkLiveEthPretouchTimingEngineKey          = "bk-live-eth-pretouch-timing"
	bkLiveEthPretouchTimingStrategyID         = "strategy-bk-eth-pretouch-timing"
	bkLiveEthPretouchTimingStrategyVersionID  = "strategy-version-bk-eth-pretouch-timing-v010"
	defaultPretouchModelPath                  = "data/pretouch_model.json"
	defaultPretouchT3OverlayModelPath         = "data/pretouch_t3_overlay_rf_model.json"
	pretouchShadowModeTestnetCollect          = "testnet_shadow_collect"
	pretouchShadowSubmitRiskOnQuantityParam   = "pretouchShadowSubmitRiskOnQuantity"
	pretouchShadowSubmitOverlayOrderParam     = "pretouchShadowSubmitOverlayOrder"
	pretouchShadowLeadQuantityBandSizingParam = "pretouchShadowLeadQuantityBandSizing"
	pretouchShadowOverlayQualitySizingParam   = "pretouchShadowOverlayQualitySizing"
	defaultPretouchShadowCandidateID          = "lead_q020_q040_overlay_q020_q040_t3_rf_cost_20260520"
	defaultPretouchShadowLeadScale            = 1.5
	defaultPretouchShadowLeadQuantityMinQty   = 0.20
	defaultPretouchShadowLeadQuantityMaxQty   = 0.40
	defaultPretouchShadowOverlayScale         = 2.0
	defaultPretouchShadowOverlayBaseShare     = 0.40
	defaultPretouchShadowOverlaySpeedMin      = 0.35
	defaultPretouchShadowOverlayQualityMin    = 2.50
	defaultPretouchShadowOverlayQualityMax    = 5.00
	defaultPretouchShadowOverlayQualityCost   = 0.10
	defaultPretouchShadowOverlayQualityMinQty = 0.20
	defaultPretouchShadowOverlayQualityMaxQty = 0.40
	pretouchShadowMaxSubmittedQuantityParam   = "pretouchShadowMaxSubmittedQuantity"
	defaultPretouchShadowMaxSubmittedQuantity = 0.40
	maxPretouchShadowLeadScale                = defaultPretouchShadowLeadScale
	maxPretouchShadowOverlayScale             = defaultPretouchShadowOverlayScale
	maxPretouchShadowOverlayBaseShare         = defaultPretouchShadowOverlayBaseShare
	maxPretouchShadowOverlayQualityMax        = defaultPretouchShadowOverlayQualityMax
	maxPretouchShadowMaxSubmittedQuantity     = defaultPretouchShadowMaxSubmittedQuantity
	defaultPretouchShadowStrict10Pct          = 35.521555
	defaultPretouchShadowStrict15Pct          = 28.970948
	defaultPretouchShadowSevere15Pct          = 21.231073
)

// bkLiveEthPretouchTimingEngine implements the ETH pretouch timing strategy.
// It detects pretouch breakout events in real-time, uses Go-native DT3 + RF
// for timing classification and probability sizing, and produces signal intents.
// No Python dependency; single Go binary.
type bkLiveEthPretouchTimingEngine struct {
	platform   *Platform
	detector   *PretouchEventDetector
	t3Detector *PretouchEventDetector
	model      *PretouchModelBundle
	t3Model    *PretouchModelBundle
	config     PretouchDetectorConfig
}

func newBkLiveEthPretouchTimingEngine(platform *Platform) *bkLiveEthPretouchTimingEngine {
	config := DefaultPretouchDetectorConfig()
	engine := &bkLiveEthPretouchTimingEngine{
		platform:   platform,
		detector:   NewPretouchEventDetector("ETHUSDT", config),
		t3Detector: NewPretouchEventDetector("ETHUSDT", config),
		config:     config,
	}

	// Try to load model bundle from default path. Deployments can override this
	// without changing the launch template.
	modelPath := firstNonEmpty(strings.TrimSpace(os.Getenv("BK_PRETOUCH_MODEL_PATH")), defaultPretouchModelPath)
	if bundle, err := LoadModelBundle(modelPath); err == nil {
		engine.model = bundle
		platform.logger("service.pretouch_timing").Info("pretouch model loaded",
			"version", bundle.Version,
			"timing_loocv", bundle.TimingLOOCV,
			"rf_accuracy", bundle.RFAccuracy,
		)
	} else {
		platform.logger("service.pretouch_timing").Warn("pretouch model not found, events will be skipped until model is trained",
			"path", modelPath,
			"error", err,
		)
	}

	t3ModelPath := firstNonEmpty(strings.TrimSpace(os.Getenv("BK_PRETOUCH_T3_OVERLAY_MODEL_PATH")), defaultPretouchT3OverlayModelPath)
	if bundle, err := LoadModelBundle(t3ModelPath); err == nil {
		engine.t3Model = bundle
		platform.logger("service.pretouch_timing").Info("pretouch T3 overlay RF model loaded",
			"path", t3ModelPath,
			"version", bundle.Version,
			"rf_accuracy", bundle.RFAccuracy,
		)
	} else {
		platform.logger("service.pretouch_timing").Warn("pretouch T3 overlay RF model not found, T3 overlay will keep fixed sizing unless the model is available",
			"path", t3ModelPath,
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
		"name":                "BK Live ETH Pretouch Timing",
		"supportedSignalBars": []string{"1h"},
		"supportedExecutions": []string{"tick"},
		"runtimeConsistency":  "live-eth-pretouch-timing-unified",
		"symbol":              "ETHUSDT",
		"description":         "ETH pretouch timing strategy: production RF/cost sizing plus sandbox-only testnet shadow lead and T3 overlay quantity-band research candidate.",
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

	// Extract tick data from trigger summary. Open-position exits may still use
	// order-book prices when the trigger itself has no trade price.
	triggerPrice := parseFloatValue(ctx.TriggerSummary["price"])

	config := pretouchConfigFromParameters(e.config, ctx.ExecutionContext.Parameters)
	e.detector.SetConfig(config)

	closedBars, currentBar := pretouchBarsFromEvaluationContext(ctx, triggerPrice)
	e.detector.SyncBars(closedBars, currentBar)
	if e.t3Detector != nil {
		t3Config := pretouchT3OverlayConfigFromParameters(config, ctx.ExecutionContext.Parameters)
		e.t3Detector.SetConfig(t3Config)
		e.t3Detector.SyncBars(closedBars, currentBar)
	}

	orderBookStats := extractOrderBookStats(ctx.TriggerSummary, ctx.SourceStates)

	if decision, handled := pretouchEvaluateOpenPositionExit(ctx, triggerPrice, orderBookStats); handled {
		return decision, nil
	}

	if triggerPrice <= 0 {
		return StrategySignalDecision{Action: "wait", Reason: "no_trigger_price"}, nil
	}

	// Build tick from trigger
	tick := TickData{
		Time:    ctx.EventTime,
		Price:   triggerPrice,
		BestBid: orderBookStats.bestBid,
		BestAsk: orderBookStats.bestAsk,
	}

	// Process tick through detector
	result := e.detector.OnTick(tick)

	if !result.Detected {
		if decision, handled := e.evaluateT3ShadowOverlay(ctx, tick, closedBars, currentBar, orderBookStats, result.Reason); handled {
			return decision, nil
		}
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
	timingRegime := normalizePretouchTimingRegime(e.model.TimingTree.Predict(features))

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
				"pretouchEvent": result.Event.EventID,
				"timingRegime":  "skip",
				"rfProbability": rfProb,
				"modelVersion":  e.model.Version,
			},
		}, nil
	}

	// Compute final position size multiplier and live quantity.
	finalMultiplier := sizingMultiplier * result.Event.CostPenalty
	finalPositionSize := config.BaseShare * finalMultiplier
	result.Event.FinalPositionSize = finalPositionSize
	baseQuantity := pretouchBaseOrderQuantityFromParameters(ctx.ExecutionContext.Parameters)
	if baseQuantity <= 0 {
		return StrategySignalDecision{
			Action: "wait",
			Reason: "no_base_quantity",
			Metadata: map[string]any{
				"pretouchEvent": result.Event.EventID,
				"timingRegime":  timingRegime,
				"rfProbability": rfProb,
				"modelVersion":  e.model.Version,
			},
		}, nil
	}
	nextSide := nextPretouchSide(result.Event.Side)
	productionSuggestedQuantity := pretouchCapSubmittedQuantity(ctx.ExecutionContext.Parameters, baseQuantity*finalPositionSize)
	shadowSizing := pretouchShadowSizingFromParameters(
		ctx.ExecutionContext.Parameters,
		nextSide,
		productionSuggestedQuantity,
		orderBookStats,
		pretouchShadowSubmitContextFromEvaluation(ctx),
	)
	suggestedQuantity := productionSuggestedQuantity
	if shadowSizing != nil {
		if submittedQuantityAfterShadow := parseFloatValue(shadowSizing["submittedQuantityAfterShadow"]); submittedQuantityAfterShadow > 0 {
			suggestedQuantity = submittedQuantityAfterShadow
		}
	}

	logger.Info("pretouch signal: advance-plan",
		"event_id", result.Event.EventID,
		"side", result.Event.Side,
		"timing_regime", timingRegime,
		"rf_probability", rfProb,
		"sizing_multiplier", sizingMultiplier,
		"cost_penalty", result.Event.CostPenalty,
		"final_position_size", finalPositionSize,
		"production_suggested_quantity", productionSuggestedQuantity,
		"suggested_quantity", suggestedQuantity,
		"shadow_risk_on_quantity_enabled", boolValue(mapValue(shadowSizing)["submittedRiskOnQuantityEnabled"]),
		"model_version", e.model.Version,
	)

	currentSnapshot := pretouchBarSnapshot(currentBar)
	prevBar2Snapshot := map[string]any{}
	if len(closedBars) >= 2 {
		prevBar2Snapshot = pretouchBarSnapshot(&closedBars[len(closedBars)-2])
	}
	signalBarDecision := map[string]any{
		"ready":                               true,
		"longReady":                           nextSide == "BUY",
		"shortReady":                          nextSide == "SELL",
		"longBreakoutPatternReady":            nextSide == "BUY",
		"shortBreakoutPatternReady":           nextSide == "SELL",
		"breakoutPrice":                       result.Event.TouchPrice,
		"breakoutPriceSource":                 "pretouch.touch_price",
		"timeframe":                           "1h",
		"current":                             currentSnapshot,
		"prevBar2":                            prevBar2Snapshot,
		"pretouchTimingRegime":                timingRegime,
		"pretouchRFProbability":               rfProb,
		"pretouchFinalPositionSize":           finalPositionSize,
		"pretouchProductionSuggestedQuantity": productionSuggestedQuantity,
		"pretouchSuggestedQuantity":           suggestedQuantity,
	}
	if shadowSizing != nil {
		signalBarDecision["pretouchShadowSizing"] = cloneMetadata(shadowSizing)
	}
	signalBarTradeLimitKey := pretouchSignalBarTradeLimitKey(ctx.ExecutionContext.Symbol, "1h", currentBar)

	metadata := map[string]any{
		"pretouchEvent":                 result.Event,
		"timingRegime":                  timingRegime,
		"rfProbability":                 rfProb,
		"sizingMultiplier":              sizingMultiplier,
		"costPenalty":                   result.Event.CostPenalty,
		"finalPositionSize":             finalPositionSize,
		"productionSuggestedQuantity":   productionSuggestedQuantity,
		"suggestedQuantity":             suggestedQuantity,
		"modelVersion":                  e.model.Version,
		"signalSource":                  "pretouch-timing-unified",
		"signalBarDecision":             signalBarDecision,
		"signalBarStateKey":             signalBarTradeLimitKey,
		liveSignalBarTradeLimitKeyField: signalBarTradeLimitKey,
		"nextPlannedEvent":              formatOptionalRFC3339(result.Event.TouchTime),
		"nextPlannedPrice":              result.Event.TouchPrice,
		"nextPlannedSide":               nextSide,
		"nextPlannedRole":               "entry",
		"nextPlannedReason":             "Pretouch-Timing",
		"marketPrice":                   triggerPrice,
		"marketSource":                  "trade_tick.price",
		"signalKind":                    "entry",
		"decisionState":                 "ready",
		"spreadBps":                     orderBookStats.spreadBps,
		"bestBid":                       orderBookStats.bestBid,
		"bestAsk":                       orderBookStats.bestAsk,
		"bestBidQty":                    orderBookStats.bestBidQty,
		"bestAskQty":                    orderBookStats.bestAskQty,
		"bookImbalance":                 orderBookStats.imbalance,
		"liquidityBias":                 orderBookStats.bias,
		"biasActionable":                isLiquidityBiasActionable(nextSide, "entry", "Pretouch-Timing", orderBookStats.bias),
	}
	if shadowSizing != nil {
		metadata["pretouchShadowSizing"] = shadowSizing
	}

	// Produce signal decision
	return StrategySignalDecision{
		Action:   "advance-plan",
		Reason:   fmt.Sprintf("pretouch_%s_%s", timingRegime, result.Event.Side),
		Metadata: metadata,
	}, nil
}

func nextPretouchSide(eventSide string) string {
	if strings.EqualFold(eventSide, "short") {
		return "SELL"
	}
	return "BUY"
}

func (e *bkLiveEthPretouchTimingEngine) evaluateT3ShadowOverlay(ctx StrategySignalEvaluationContext, tick TickData, closedBars []HourlyBar, currentBar *HourlyBar, orderBookStats orderBookDecisionStats, leadMissReason string) (StrategySignalDecision, bool) {
	if e.t3Detector == nil || !strings.EqualFold(stringValue(ctx.ExecutionContext.Parameters["pretouchShadowMode"]), pretouchShadowModeTestnetCollect) {
		return StrategySignalDecision{}, false
	}
	if strings.EqualFold(leadMissReason, "already_touched_this_bar") {
		return StrategySignalDecision{}, false
	}
	result := e.t3Detector.OnTickT3Overlay(tick)
	if !result.Detected {
		return StrategySignalDecision{}, false
	}

	nextSide := nextPretouchSide(result.Event.Side)
	baseQuantity := pretouchBaseOrderQuantityFromParameters(ctx.ExecutionContext.Parameters)
	if baseQuantity <= 0 {
		return StrategySignalDecision{
			Action: "wait",
			Reason: "t3_overlay_no_base_quantity",
			Metadata: map[string]any{
				"pretouchEvent":      result.Event,
				"pretouchEventShape": "t3_swing",
				"detectedButSkipped": true,
			},
		}, true
	}

	overlaySizing := pretouchShadowOverlaySizingFromParameters(
		ctx.ExecutionContext.Parameters,
		nextSide,
		baseQuantity,
		orderBookStats,
		pretouchShadowSubmitContextFromEvaluation(ctx),
		e.pretouchT3OverlayQualitySizing(ctx.ExecutionContext.Parameters, result.Event),
	)
	if overlaySizing == nil {
		return StrategySignalDecision{}, false
	}
	if !boolValue(overlaySizing["submittedOverlayOrderEnabled"]) {
		return StrategySignalDecision{
			Action: "wait",
			Reason: firstNonEmpty(stringValue(overlaySizing["submittedOverlayOrderBlockReason"]), "t3_overlay_shadow_blocked"),
			Metadata: map[string]any{
				"pretouchEvent":                result.Event,
				"pretouchEventShape":           "t3_swing",
				"pretouchShadowOverlaySizing":  overlaySizing,
				"shadowOverlayDetectedButHeld": true,
				"signalSource":                 "pretouch-t3-overlay-shadow",
				"decisionState":                "blocked",
				"signalKind":                   "entry-t3-overlay-watch",
			},
		}, true
	}

	overlayQuantity := parseFloatValue(overlaySizing["submittedOverlayQuantity"])
	currentSnapshot := pretouchBarSnapshot(currentBar)
	prevBar1Snapshot := map[string]any{}
	if len(closedBars) >= 1 {
		prevBar1Snapshot = pretouchBarSnapshot(&closedBars[len(closedBars)-1])
	}
	prevBar2Snapshot := map[string]any{}
	if len(closedBars) >= 2 {
		prevBar2Snapshot = pretouchBarSnapshot(&closedBars[len(closedBars)-2])
	}
	prevBar3Snapshot := map[string]any{}
	if len(closedBars) >= 3 {
		prevBar3Snapshot = pretouchBarSnapshot(&closedBars[len(closedBars)-3])
	}
	signalBarStateKey := pretouchSignalBarTradeLimitKey(ctx.ExecutionContext.Symbol, "1h", currentBar)
	signalBarTradeLimitKey := pretouchSignalBarTradeLimitKeyForKind(ctx.ExecutionContext.Symbol, "1h", currentBar, "entry-t3-overlay")
	signalBarDecision := map[string]any{
		"ready":                       true,
		"longReady":                   nextSide == "BUY",
		"shortReady":                  nextSide == "SELL",
		"longBreakoutPatternReady":    nextSide == "BUY",
		"shortBreakoutPatternReady":   nextSide == "SELL",
		"breakoutShape":               "t3_swing",
		"breakoutPrice":               result.Event.TouchPrice,
		"breakoutPriceSource":         "pretouch.t3.touch_price",
		"timeframe":                   "1h",
		"current":                     currentSnapshot,
		"prevBar1":                    prevBar1Snapshot,
		"prevBar2":                    prevBar2Snapshot,
		"prevBar3":                    prevBar3Snapshot,
		"pretouchShadowOverlaySizing": cloneMetadata(overlaySizing),
	}

	metadata := map[string]any{
		"pretouchEvent":                 result.Event,
		"pretouchEventShape":            "t3_swing",
		"pretouchShadowOverlaySizing":   overlaySizing,
		"suggestedQuantity":             overlayQuantity,
		"productionSuggestedQuantity":   0.0,
		"signalSource":                  "pretouch-t3-overlay-shadow",
		"signalBarDecision":             signalBarDecision,
		"signalBarStateKey":             signalBarStateKey,
		liveSignalBarTradeLimitKeyField: signalBarTradeLimitKey,
		"nextPlannedEvent":              formatOptionalRFC3339(result.Event.TouchTime),
		"nextPlannedPrice":              result.Event.TouchPrice,
		"nextPlannedSide":               nextSide,
		"nextPlannedRole":               "entry",
		"nextPlannedReason":             "Pretouch-T3-Overlay",
		"marketPrice":                   tick.Price,
		"marketSource":                  "trade_tick.price",
		"signalKind":                    "entry-t3-overlay",
		"decisionState":                 "ready",
		"spreadBps":                     orderBookStats.spreadBps,
		"bestBid":                       orderBookStats.bestBid,
		"bestAsk":                       orderBookStats.bestAsk,
		"bestBidQty":                    orderBookStats.bestBidQty,
		"bestAskQty":                    orderBookStats.bestAskQty,
		"bookImbalance":                 orderBookStats.imbalance,
		"liquidityBias":                 orderBookStats.bias,
		"biasActionable":                isLiquidityBiasActionable(nextSide, "entry", "Pretouch-T3-Overlay", orderBookStats.bias),
	}

	return StrategySignalDecision{
		Action:   "advance-plan",
		Reason:   fmt.Sprintf("pretouch_t3_overlay_%s", result.Event.Side),
		Metadata: metadata,
	}, true
}

type pretouchShadowSubmitContext struct {
	liveExecution bool
	sandbox       bool
	executionMode string
}

func pretouchShadowSubmitContextFromEvaluation(ctx StrategySignalEvaluationContext) pretouchShadowSubmitContext {
	binding := cloneMetadata(ctx.LiveAccountBinding)
	sandbox := boolValue(binding["sandbox"])
	return pretouchShadowSubmitContext{
		liveExecution: ctx.ExecutionContext.Semantics.Mode == ExecutionModeLive,
		sandbox:       sandbox,
		executionMode: normalizeLiveExecutionMode(binding["executionMode"], sandbox),
	}
}

func pretouchT3OverlayConfigFromParameters(base PretouchDetectorConfig, parameters map[string]any) PretouchDetectorConfig {
	config := base
	config.MinSpeed300sATR = firstPositive(parseFloatValue(parameters["pretouchShadowOverlaySpeedThreshold"]), defaultPretouchShadowOverlaySpeedMin)
	if value := parseFloatValue(parameters["pretouchShadowOverlayMaxPreTouchSec"]); value > 0 {
		config.MaxPreTouchSeconds = value
	}
	if value := parseFloatValue(parameters["pretouchShadowOverlayMaxEff300s"]); value > 0 {
		config.MaxEff300s = value
	}
	return config
}

func pretouchShadowMaxSubmittedQuantity(parameters map[string]any) float64 {
	return cappedPositiveFloat(
		parseFloatValue(parameters[pretouchShadowMaxSubmittedQuantityParam]),
		defaultPretouchShadowMaxSubmittedQuantity,
		maxPretouchShadowMaxSubmittedQuantity,
	)
}

func pretouchBaseOrderQuantityFromParameters(parameters map[string]any) float64 {
	quantity := firstPositive(parseFloatValue(parameters["pretouchBaseOrderQuantity"]), parseFloatValue(parameters["defaultOrderQuantity"]))
	return pretouchCapSubmittedQuantity(parameters, quantity)
}

func pretouchCapSubmittedQuantity(parameters map[string]any, quantity float64) float64 {
	if !strings.EqualFold(stringValue(parameters["pretouchShadowMode"]), pretouchShadowModeTestnetCollect) {
		return quantity
	}
	return capPositiveFloat(quantity, pretouchShadowMaxSubmittedQuantity(parameters))
}

func cappedPositiveFloat(value, fallback, maxValue float64) float64 {
	resolved := firstPositive(value, fallback)
	return capPositiveFloat(resolved, maxValue)
}

func capPositiveFloat(value, maxValue float64) float64 {
	if value > 0 && maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

func pretouchShadowSizingFromParameters(parameters map[string]any, side string, productionQuantity float64, orderBookStats orderBookDecisionStats, submitContext pretouchShadowSubmitContext) map[string]any {
	if !strings.EqualFold(stringValue(parameters["pretouchShadowMode"]), pretouchShadowModeTestnetCollect) {
		return nil
	}
	leadScale := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowLeadScale"]), defaultPretouchShadowLeadScale, maxPretouchShadowLeadScale)
	overlayScale := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowOverlayScale"]), defaultPretouchShadowOverlayScale, maxPretouchShadowOverlayScale)
	maxSubmittedQuantity := pretouchShadowMaxSubmittedQuantity(parameters)
	legacyUncappedShadowLeadQuantity := productionQuantity * leadScale
	legacyShadowLeadQuantity := capPositiveFloat(legacyUncappedShadowLeadQuantity, maxSubmittedQuantity)
	uncappedShadowLeadQuantity := legacyUncappedShadowLeadQuantity
	shadowLeadQuantity := legacyShadowLeadQuantity
	leadQuantityBandRequested := pretouchShadowLeadQuantityBandSizingRequested(parameters)
	leadQuantityBandEnabled := false
	leadQuantityBandScore := 0.0
	leadQuantityBandMinQuantity, leadQuantityBandMaxQuantity := pretouchShadowLeadQuantityBandBounds(parameters)
	if leadQuantityBandRequested {
		productionMaxQuantity := pretouchShadowLeadMaxProductionQuantity(parameters)
		if productionMaxQuantity > 0 {
			leadQuantityBandScore = pretouchClampFloat(productionQuantity/productionMaxQuantity, 0.0, 1.0)
		} else if maxSubmittedQuantity > 0 {
			leadQuantityBandScore = pretouchClampFloat(legacyShadowLeadQuantity/maxSubmittedQuantity, 0.0, 1.0)
		}
		uncappedShadowLeadQuantity = leadQuantityBandMinQuantity + leadQuantityBandScore*(leadQuantityBandMaxQuantity-leadQuantityBandMinQuantity)
		shadowLeadQuantity = capPositiveFloat(uncappedShadowLeadQuantity, maxSubmittedQuantity)
		leadQuantityBandEnabled = true
	}
	topDepthQty := pretouchTopDepthQtyForSide(side, orderBookStats)
	currentCoverage := 0.0
	shadowCoverage := 0.0
	if topDepthQty > 0 && productionQuantity > 0 {
		currentCoverage = topDepthQty / productionQuantity
	}
	if topDepthQty > 0 && shadowLeadQuantity > 0 {
		shadowCoverage = topDepthQty / shadowLeadQuantity
	}
	minCoverage := firstPositive(parseFloatValue(parameters["executionEntryMinTopBookCoverage"]), 0.5)
	maxSpreadBps := firstPositive(parseFloatValue(parameters["executionEntryMaxSpreadBps"]), 8.0)
	shadowPass := shadowLeadQuantity > 0 &&
		shadowCoverage >= minCoverage &&
		(orderBookStats.spreadBps <= 0 || orderBookStats.spreadBps <= maxSpreadBps)
	shadowBlockReason := pretouchShadowBlockReason(shadowLeadQuantity, currentCoverage, shadowCoverage, minCoverage, orderBookStats.spreadBps, maxSpreadBps)
	riskOnRequested := true
	if _, ok := parameters[pretouchShadowSubmitRiskOnQuantityParam]; ok {
		riskOnRequested = boolValue(parameters[pretouchShadowSubmitRiskOnQuantityParam])
	}
	riskOnBlockReason := pretouchShadowRiskOnQuantityBlockReason(riskOnRequested, submitContext, shadowPass, shadowBlockReason)
	riskOnEnabled := riskOnBlockReason == ""
	submittedQuantityAfterShadow := productionQuantity
	submittedSizingMode := "production_intent_quantity"
	liveSizingPromotionState := "research_continue_collect_live_depth"
	liveSizingPromotionBlocker := "sample_size_lt_30_or_depth_not_calibrated"
	if riskOnEnabled {
		submittedQuantityAfterShadow = shadowLeadQuantity
		submittedSizingMode = "testnet_shadow_lead_scale_intent_quantity"
		if leadQuantityBandEnabled {
			submittedSizingMode = "testnet_shadow_lead_rf_cost_quantity_band"
		}
		liveSizingPromotionState = "testnet_shadow_risk_on_collect"
	}

	return map[string]any{
		"mode":                               pretouchShadowModeTestnetCollect,
		"candidateID":                        firstNonEmpty(stringValue(parameters["pretouchShadowCandidateID"]), defaultPretouchShadowCandidateID),
		"stage":                              "testnet_shadow_collect",
		"submittedSizingMode":                submittedSizingMode,
		"submittedQuantity":                  submittedQuantityAfterShadow,
		"submittedQuantityBeforeShadow":      productionQuantity,
		"submittedQuantityAfterShadow":       submittedQuantityAfterShadow,
		"productionSuggestedQuantity":        productionQuantity,
		"leadScale":                          leadScale,
		"overlayScale":                       overlayScale,
		"leadQuantityBandRequested":          leadQuantityBandRequested,
		"leadQuantityBandEnabled":            leadQuantityBandEnabled,
		"leadQuantityBandMinQuantity":        leadQuantityBandMinQuantity,
		"leadQuantityBandMaxQuantity":        leadQuantityBandMaxQuantity,
		"leadQuantityBandScore":              leadQuantityBandScore,
		"shadowLeadQuantity":                 shadowLeadQuantity,
		"uncappedShadowLeadQuantity":         uncappedShadowLeadQuantity,
		"legacyShadowLeadQuantity":           legacyShadowLeadQuantity,
		"legacyUncappedShadowLeadQuantity":   legacyUncappedShadowLeadQuantity,
		"shadowLeadQuantityCapped":           uncappedShadowLeadQuantity > shadowLeadQuantity,
		"maxShadowSubmittedQuantity":         maxSubmittedQuantity,
		"shadowLeadQuantityDelta":            shadowLeadQuantity - productionQuantity,
		"shadowOverlayExecution":             "testnet_shadow_t3_event_source_guarded",
		"currentTopDepthCoverage":            currentCoverage,
		"shadowTopDepthCoverage":             shadowCoverage,
		"minTopDepthCoverage":                minCoverage,
		"maxSpreadBps":                       maxSpreadBps,
		"spreadBps":                          orderBookStats.spreadBps,
		"topDepthQty":                        topDepthQty,
		"shadowPreSubmitPass":                shadowPass,
		"shadowBlockReason":                  shadowBlockReason,
		"researchStrict10CalendarPct":        firstPositive(parseFloatValue(parameters["pretouchShadowStrict10CalendarPct"]), defaultPretouchShadowStrict10Pct),
		"researchStrict15CalendarPct":        firstPositive(parseFloatValue(parameters["pretouchShadowStrict15CalendarPct"]), defaultPretouchShadowStrict15Pct),
		"researchSevere15CalendarPct":        firstPositive(parseFloatValue(parameters["pretouchShadowSevere15CalendarPct"]), defaultPretouchShadowSevere15Pct),
		"researchLeadAdverseBaseline":        firstPositive(parseFloatValue(parameters["pretouchShadowLeadAdverseBaselinePct"]), 22.971648),
		"submittedQuantityUnchanged":         !riskOnEnabled,
		"submittedRiskOnQuantityRequested":   riskOnRequested,
		"submittedRiskOnQuantityBlockReason": riskOnBlockReason,
		"liveExecutionMode":                  submitContext.executionMode,
		"liveSandbox":                        submitContext.sandbox,
		"liveSemanticsMode":                  boolValueToLiveSemanticsLabel(submitContext.liveExecution),
		"liveSizingPromotionState":           liveSizingPromotionState,
		"liveSizingPromotionBlocker":         liveSizingPromotionBlocker,
		"mainnetSizingChangePermitted":       false,
		"submittedOverlayOrderEnabled":       false,
		"submittedRiskOnQuantityEnabled":     riskOnEnabled,
	}
}

func pretouchShadowLeadQuantityBandSizingRequested(parameters map[string]any) bool {
	if _, ok := parameters[pretouchShadowLeadQuantityBandSizingParam]; !ok {
		return false
	}
	return boolValue(parameters[pretouchShadowLeadQuantityBandSizingParam])
}

func pretouchShadowLeadQuantityBandBounds(parameters map[string]any) (float64, float64) {
	minQuantity := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowLeadQuantityMinQuantity"]), defaultPretouchShadowLeadQuantityMinQty, maxPretouchShadowMaxSubmittedQuantity)
	maxQuantity := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowLeadQuantityMaxQuantity"]), defaultPretouchShadowLeadQuantityMaxQty, maxPretouchShadowMaxSubmittedQuantity)
	if minQuantity > maxQuantity {
		minQuantity = maxQuantity
	}
	return minQuantity, maxQuantity
}

func pretouchShadowLeadMaxProductionQuantity(parameters map[string]any) float64 {
	baseQuantity := pretouchBaseOrderQuantityFromParameters(parameters)
	if baseQuantity <= 0 {
		return 0
	}
	baseShare := firstPositive(parseFloatValue(parameters["pretouchBaseShare"]), DefaultPretouchDetectorConfig().BaseShare)
	return baseQuantity * baseShare * 2.0
}

func (e *bkLiveEthPretouchTimingEngine) pretouchT3OverlayQualitySizing(parameters map[string]any, event domain.PretouchEvent) map[string]any {
	if !pretouchShadowOverlayQualitySizingRequested(parameters) {
		return nil
	}
	minMultiplier, maxMultiplier := pretouchShadowOverlayQualityMultiplierBounds(parameters)
	costThresholdATR := firstPositive(
		pretouchFiniteFloat(parseFloatValue(parameters["pretouchShadowOverlayQualityCostThresholdATR"]), 0),
		defaultPretouchShadowOverlayQualityCost,
	)
	minQuantity, maxQuantity := pretouchShadowOverlayQualityQuantityBounds(parameters)
	quality := map[string]any{
		"requested":           true,
		"enabled":             false,
		"status":              "model_missing",
		"multiplier":          1.0,
		"minMultiplier":       minMultiplier,
		"maxMultiplier":       maxMultiplier,
		"quantityBandEnabled": true,
		"minQuantity":         minQuantity,
		"maxQuantity":         maxQuantity,
		"costThresholdATR":    costThresholdATR,
		"costPenalty":         1.0,
		"mainnetPermitted":    false,
		"modelArtifactPath":   defaultPretouchT3OverlayModelPath,
		"modelArtifactEnvVar": "BK_PRETOUCH_T3_OVERLAY_MODEL_PATH",
	}
	if e == nil || e.t3Model == nil || e.t3Model.RFModel == nil {
		return quality
	}
	quality["modelVersion"] = e.t3Model.Version
	features, featureMeta, ok := e.buildPretouchT3OverlayQualityFeatures(event, e.t3Model.FeatureNames)
	quality["features"] = featureMeta
	if !ok {
		quality["status"] = "feature_build_failed"
		return quality
	}
	probability := pretouchFiniteFloat(e.t3Model.RFModel.PredictProba(features), 0.5)
	rawMultiplier := minMultiplier + probability*(maxMultiplier-minMultiplier)
	linearMultiplier := pretouchClampFloat(rawMultiplier, minMultiplier, maxMultiplier)
	costPenalty := pretouchT3OverlayQualityCostPenalty(event, costThresholdATR)
	multiplier := pretouchClampFloat(linearMultiplier*costPenalty, minMultiplier, maxMultiplier)
	quality["enabled"] = true
	quality["status"] = "applied"
	quality["probability"] = probability
	quality["rawMultiplier"] = rawMultiplier
	quality["linearMultiplier"] = linearMultiplier
	quality["costPenalty"] = costPenalty
	quality["multiplier"] = multiplier
	return quality
}

func (e *bkLiveEthPretouchTimingEngine) buildPretouchT3OverlayQualityFeatures(event domain.PretouchEvent, featureNames []string) ([]float64, map[string]any, bool) {
	features := make([]float64, len(featureNames))
	meta := make(map[string]any, len(featureNames)+3)
	leadProbability := 0.0
	leadProbabilityOK := false
	if pretouchFeatureNamesContain(featureNames, "rf_probability") {
		leadProbability, leadProbabilityOK = e.predictPretouchLeadRFProbability(event.Features)
		meta["leadRFProbability"] = leadProbability
		meta["leadRFProbabilityAvailable"] = leadProbabilityOK
		if e != nil && e.model != nil {
			meta["leadModelVersion"] = e.model.Version
		}
		if !leadProbabilityOK {
			meta["missingFeature"] = "rf_probability"
			return features, meta, false
		}
	}
	for i, name := range featureNames {
		value, ok := pretouchT3OverlayFeatureValue(event, name, leadProbability)
		if !ok {
			if e != nil && e.t3Model != nil && i < len(e.t3Model.Medians) {
				value = e.t3Model.Medians[i]
				ok = true
				meta["imputed_"+name] = true
			}
		}
		if !ok {
			meta["missingFeature"] = name
			return features, meta, false
		}
		value = float64(float32(pretouchFiniteFloat(value, 0)))
		features[i] = value
		meta[name] = value
	}
	return features, meta, true
}

func (e *bkLiveEthPretouchTimingEngine) predictPretouchLeadRFProbability(eventFeatures map[string]float64) (float64, bool) {
	if e == nil || e.model == nil || e.model.RFModel == nil || len(e.model.FeatureNames) == 0 {
		return 0, false
	}
	features := make([]float64, len(e.model.FeatureNames))
	for i, name := range e.model.FeatureNames {
		value, ok := eventFeatures[name]
		if !ok {
			if i >= len(e.model.Medians) {
				return 0, false
			}
			value = e.model.Medians[i]
		}
		features[i] = pretouchFiniteFloat(value, 0)
	}
	return pretouchFiniteFloat(e.model.RFModel.PredictProba(features), 0.5), true
}

func pretouchT3OverlayFeatureValue(event domain.PretouchEvent, name string, leadProbability float64) (float64, bool) {
	switch name {
	case "rf_probability":
		return leadProbability, true
	case "speed_300s_abs":
		return math.Abs(event.Speed300sATR), true
	case "touch_extension_abs":
		return math.Abs(event.TouchExtensionATR), true
	case "side_is_short":
		if strings.EqualFold(event.Side, "short") {
			return 1.0, true
		}
		return 0.0, true
	case "speed_300s_atr":
		return event.Speed300sATR, true
	case "touch_extension_atr":
		return event.TouchExtensionATR, true
	case "eff_300s":
		return event.Eff300s, true
	case "pre_touch_seconds":
		return event.PreTouchSeconds, true
	case "roundtrip_cost_atr":
		return event.RoundtripCostATR, true
	default:
		if value, ok := event.Features[name]; ok {
			return value, true
		}
		return 0, false
	}
}

func pretouchFeatureNamesContain(featureNames []string, target string) bool {
	for _, name := range featureNames {
		if name == target {
			return true
		}
	}
	return false
}

func pretouchShadowOverlayQualitySizingRequested(parameters map[string]any) bool {
	if _, ok := parameters[pretouchShadowOverlayQualitySizingParam]; !ok {
		return false
	}
	return boolValue(parameters[pretouchShadowOverlayQualitySizingParam])
}

func pretouchShadowOverlayQualityMultiplierBounds(parameters map[string]any) (float64, float64) {
	minMultiplier := firstPositive(parseFloatValue(parameters["pretouchShadowOverlayQualityMinMultiplier"]), defaultPretouchShadowOverlayQualityMin)
	maxMultiplier := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowOverlayQualityMaxMultiplier"]), defaultPretouchShadowOverlayQualityMax, maxPretouchShadowOverlayQualityMax)
	if minMultiplier > maxMultiplier {
		minMultiplier = maxMultiplier
	}
	return minMultiplier, maxMultiplier
}

func pretouchShadowOverlayQualityQuantityBounds(parameters map[string]any) (float64, float64) {
	minQuantity := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowOverlayQualityMinQuantity"]), defaultPretouchShadowOverlayQualityMinQty, maxPretouchShadowMaxSubmittedQuantity)
	maxQuantity := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowOverlayQualityMaxQuantity"]), defaultPretouchShadowOverlayQualityMaxQty, maxPretouchShadowMaxSubmittedQuantity)
	if minQuantity > maxQuantity {
		minQuantity = maxQuantity
	}
	return minQuantity, maxQuantity
}

func pretouchT3OverlayQualityCostPenalty(event domain.PretouchEvent, thresholdATR float64) float64 {
	costPenalty := firstPositive(pretouchFiniteFloat(event.CostPenalty, 0), 1.0)
	costPenalty = pretouchClampFloat(costPenalty, 0.0, 1.0)
	cost := pretouchFiniteFloat(event.RoundtripCostATR, 0)
	computed := 1.0
	if cost > 0 && thresholdATR > 0 {
		computed = pretouchClampFloat(thresholdATR/cost, 0.25, 1.0)
	}
	if computed < costPenalty {
		return computed
	}
	return costPenalty
}

func pretouchClampFloat(value, minValue, maxValue float64) float64 {
	value = pretouchFiniteFloat(value, minValue)
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func pretouchFiniteFloat(value, fallback float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
}

func pretouchShadowOverlaySizingFromParameters(parameters map[string]any, side string, baseQuantity float64, orderBookStats orderBookDecisionStats, submitContext pretouchShadowSubmitContext, qualitySizing map[string]any) map[string]any {
	if !strings.EqualFold(stringValue(parameters["pretouchShadowMode"]), pretouchShadowModeTestnetCollect) {
		return nil
	}
	overlayScale := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowOverlayScale"]), defaultPretouchShadowOverlayScale, maxPretouchShadowOverlayScale)
	overlayBaseShare := cappedPositiveFloat(parseFloatValue(parameters["pretouchShadowOverlayBaseShare"]), defaultPretouchShadowOverlayBaseShare, maxPretouchShadowOverlayBaseShare)
	maxSubmittedQuantity := pretouchShadowMaxSubmittedQuantity(parameters)
	baseOverlayQuantity := baseQuantity * overlayBaseShare
	shadowOverlayQuantityBeforeQuality := baseOverlayQuantity * overlayScale
	overlayQualityMultiplier := 1.0
	overlayQualityStatus := "disabled"
	if qualitySizing != nil {
		overlayQualityStatus = firstNonEmpty(stringValue(qualitySizing["status"]), "unknown")
		if boolValue(qualitySizing["enabled"]) {
			if boolValue(qualitySizing["quantityBandEnabled"]) && shadowOverlayQuantityBeforeQuality > 0 {
				minQuantity := firstPositive(parseFloatValue(qualitySizing["minQuantity"]), defaultPretouchShadowOverlayQualityMinQty)
				maxQuantity := firstPositive(parseFloatValue(qualitySizing["maxQuantity"]), defaultPretouchShadowOverlayQualityMaxQty)
				if minQuantity > maxQuantity {
					minQuantity = maxQuantity
				}
				probability := pretouchClampFloat(parseFloatValue(qualitySizing["probability"]), 0.0, 1.0)
				costPenalty := firstPositive(parseFloatValue(qualitySizing["costPenalty"]), 1.0)
				rawQualityQuantity := minQuantity + probability*(maxQuantity-minQuantity)
				linearQualityQuantity := pretouchClampFloat(rawQualityQuantity, minQuantity, maxQuantity)
				qualityQuantity := pretouchClampFloat(linearQualityQuantity*costPenalty, minQuantity, maxQuantity)
				overlayQualityMultiplier = qualityQuantity / shadowOverlayQuantityBeforeQuality
				qualitySizing["rawQualityQuantity"] = rawQualityQuantity
				qualitySizing["linearQualityQuantity"] = linearQualityQuantity
				qualitySizing["qualityQuantity"] = qualityQuantity
				qualitySizing["multiplier"] = overlayQualityMultiplier
			} else {
				overlayQualityMultiplier = firstPositive(parseFloatValue(qualitySizing["multiplier"]), 1.0)
			}
		}
	}
	uncappedShadowOverlayQuantity := shadowOverlayQuantityBeforeQuality * overlayQualityMultiplier
	shadowOverlayQuantity := capPositiveFloat(uncappedShadowOverlayQuantity, maxSubmittedQuantity)
	topDepthQty := pretouchTopDepthQtyForSide(side, orderBookStats)
	overlayCoverage := 0.0
	if topDepthQty > 0 && shadowOverlayQuantity > 0 {
		overlayCoverage = topDepthQty / shadowOverlayQuantity
	}
	minCoverage := firstPositive(parseFloatValue(parameters["executionEntryMinTopBookCoverage"]), 0.5)
	maxSpreadBps := firstPositive(parseFloatValue(parameters["executionEntryMaxSpreadBps"]), 8.0)
	overlayPass := shadowOverlayQuantity > 0 &&
		overlayCoverage >= minCoverage &&
		(orderBookStats.spreadBps <= 0 || orderBookStats.spreadBps <= maxSpreadBps)
	overlayBlockReason := pretouchShadowBlockReason(shadowOverlayQuantity, overlayCoverage, overlayCoverage, minCoverage, orderBookStats.spreadBps, maxSpreadBps)
	overlayRequested := true
	if _, ok := parameters[pretouchShadowSubmitOverlayOrderParam]; ok {
		overlayRequested = boolValue(parameters[pretouchShadowSubmitOverlayOrderParam])
	}
	overlayOrderBlockReason := pretouchShadowOverlayOrderBlockReason(overlayRequested, submitContext, overlayPass, overlayBlockReason)
	overlayEnabled := overlayOrderBlockReason == ""
	submittedOverlayQuantity := 0.0
	if overlayEnabled {
		submittedOverlayQuantity = shadowOverlayQuantity
	}
	overlaySizingMode := "testnet_shadow_t3_overlay_scale_intent_quantity"
	if qualitySizing != nil && boolValue(qualitySizing["enabled"]) {
		overlaySizingMode = "testnet_shadow_t3_overlay_rf_cost_quality_quantity"
	}
	return map[string]any{
		"mode":                               pretouchShadowModeTestnetCollect,
		"candidateID":                        firstNonEmpty(stringValue(parameters["pretouchShadowCandidateID"]), defaultPretouchShadowCandidateID),
		"stage":                              "testnet_shadow_collect",
		"overlayEventSource":                 "t3_swing",
		"overlaySizingMode":                  overlaySizingMode,
		"overlayBaseShare":                   overlayBaseShare,
		"overlayBaseQuantity":                baseOverlayQuantity,
		"overlayScale":                       overlayScale,
		"overlayQualityStatus":               overlayQualityStatus,
		"overlayQualityMultiplier":           overlayQualityMultiplier,
		"shadowOverlayQuantityBeforeQuality": shadowOverlayQuantityBeforeQuality,
		"shadowOverlayQuantity":              shadowOverlayQuantity,
		"uncappedShadowOverlayQuantity":      uncappedShadowOverlayQuantity,
		"shadowOverlayQuantityCapped":        uncappedShadowOverlayQuantity > shadowOverlayQuantity,
		"maxShadowSubmittedQuantity":         maxSubmittedQuantity,
		"submittedOverlayQuantity":           submittedOverlayQuantity,
		"submittedOverlayOrderRequested":     overlayRequested,
		"submittedOverlayOrderEnabled":       overlayEnabled,
		"submittedOverlayOrderBlockReason":   overlayOrderBlockReason,
		"overlayTopDepthCoverage":            overlayCoverage,
		"minTopDepthCoverage":                minCoverage,
		"maxSpreadBps":                       maxSpreadBps,
		"spreadBps":                          orderBookStats.spreadBps,
		"topDepthQty":                        topDepthQty,
		"shadowPreSubmitPass":                overlayPass,
		"shadowBlockReason":                  overlayBlockReason,
		"liveExecutionMode":                  submitContext.executionMode,
		"liveSandbox":                        submitContext.sandbox,
		"liveSemanticsMode":                  boolValueToLiveSemanticsLabel(submitContext.liveExecution),
		"mainnetOverlayOrderPermitted":       false,
		"researchStrict10CalendarPct":        firstPositive(parseFloatValue(parameters["pretouchShadowStrict10CalendarPct"]), defaultPretouchShadowStrict10Pct),
		"researchStrict15CalendarPct":        firstPositive(parseFloatValue(parameters["pretouchShadowStrict15CalendarPct"]), defaultPretouchShadowStrict15Pct),
		"researchSevere15CalendarPct":        firstPositive(parseFloatValue(parameters["pretouchShadowSevere15CalendarPct"]), defaultPretouchShadowSevere15Pct),
		"researchLeadAdverseBaselinePct":     firstPositive(parseFloatValue(parameters["pretouchShadowLeadAdverseBaselinePct"]), 22.971648),
		"liveSizingPromotionState":           "testnet_shadow_overlay_collect",
		"liveSizingPromotionBlocker":         "sample_size_lt_30_or_depth_not_calibrated",
		"pretouchShadowOverlayQualitySizing": cloneMetadata(qualitySizing),
	}
}

func pretouchShadowRiskOnQuantityBlockReason(requested bool, submitContext pretouchShadowSubmitContext, shadowPass bool, shadowBlockReason string) string {
	switch {
	case !requested:
		return "risk_on_quantity_not_requested"
	case !submitContext.liveExecution:
		return "live_execution_required"
	case !submitContext.sandbox:
		return "sandbox_required"
	case !strings.EqualFold(submitContext.executionMode, "rest"):
		return "rest_execution_required"
	case !shadowPass:
		return firstNonEmpty(shadowBlockReason, "shadow_pre_submit_guard_failed")
	default:
		return ""
	}
}

func pretouchShadowOverlayOrderBlockReason(requested bool, submitContext pretouchShadowSubmitContext, shadowPass bool, shadowBlockReason string) string {
	switch {
	case !requested:
		return "overlay_order_not_requested"
	case !submitContext.liveExecution:
		return "live_execution_required"
	case !submitContext.sandbox:
		return "sandbox_required"
	case !strings.EqualFold(submitContext.executionMode, "rest"):
		return "rest_execution_required"
	case !shadowPass:
		return firstNonEmpty(shadowBlockReason, "shadow_pre_submit_guard_failed")
	default:
		return ""
	}
}

func boolValueToLiveSemanticsLabel(live bool) string {
	if live {
		return string(ExecutionModeLive)
	}
	return "non-live"
}

func pretouchTopDepthQtyForSide(side string, orderBookStats orderBookDecisionStats) float64 {
	if strings.EqualFold(side, "BUY") {
		return orderBookStats.bestAskQty
	}
	if strings.EqualFold(side, "SELL") {
		return orderBookStats.bestBidQty
	}
	return 0
}

func pretouchShadowBlockReason(shadowQuantity, currentCoverage, shadowCoverage, minCoverage, spreadBps, maxSpreadBps float64) string {
	switch {
	case shadowQuantity <= 0:
		return "no_shadow_quantity"
	case currentCoverage <= 0:
		return "missing_top_depth_coverage"
	case shadowCoverage < minCoverage:
		return "shadow_top_depth_coverage_below_min"
	case spreadBps > 0 && spreadBps > maxSpreadBps:
		return "spread_above_shadow_guard"
	default:
		return ""
	}
}

func pretouchEvaluateOpenPositionExit(ctx StrategySignalEvaluationContext, triggerPrice float64, orderBookStats orderBookDecisionStats) (StrategySignalDecision, bool) {
	currentPosition := cloneMetadata(ctx.CurrentPosition)
	if !hasActiveLivePositionSnapshot(currentPosition) {
		return StrategySignalDecision{}, false
	}

	symbol := NormalizeSymbol(ctx.ExecutionContext.Symbol)
	timeframe := normalizeSignalBarInterval(ctx.ExecutionContext.SignalTimeframe)
	signalBarState, signalBarStateKey := pretouchSignalBarStateFromEvaluationContext(ctx, symbol, timeframe)
	exitSide := normalizeLiveExitIntentSide("", currentPosition)
	marketPrice, marketSource := pickDecisionMarketPrice(ctx.TriggerSummary, ctx.SourceStates, exitSide)
	if marketPrice <= 0 && triggerPrice > 0 {
		marketPrice = triggerPrice
		marketSource = "trade_tick.price"
	}

	positionPnLBps := computePositionPnLBps(currentPosition, marketPrice)
	signalBarDecision := map[string]any{
		"ready":     false,
		"side":      exitSide,
		"role":      "exit",
		"reason":    "SL",
		"timeframe": timeframe,
	}
	livePositionState := map[string]any{}
	nextPlannedPrice := 0.0
	action := "wait"
	reason := "missing-signal-bars"
	if signalBarState != nil {
		signalBarDecision["current"] = cloneMetadata(mapValue(signalBarState["current"]))
		signalBarDecision["prevBar1"] = cloneMetadata(mapValue(signalBarState["prevBar1"]))
		signalBarDecision["prevBar2"] = cloneMetadata(mapValue(signalBarState["prevBar2"]))
		signalBarDecision["prevBar3"] = cloneMetadata(mapValue(signalBarState["prevBar3"]))
		signalBarDecision["atr14"] = parseFloatValue(signalBarState["atr14"])
		signalBarDecision["atrPercentile"] = parseFloatValue(signalBarState["atrPercentile"])
		livePositionState = evaluateLiveExitState(ctx.ExecutionContext.Parameters, currentPosition, signalBarState, marketPrice, cloneMetadata(ctx.SessionState), "SL")
		if len(livePositionState) > 0 {
			signalBarDecision["livePositionState"] = cloneMetadata(livePositionState)
			nextPlannedPrice = parseFloatValue(livePositionState["targetPrice"])
			if boolValue(livePositionState["ready"]) {
				action = "advance-plan"
				reason = "pretouch_sl_exit"
				signalBarDecision["ready"] = true
			} else {
				reason = firstNonEmpty(stringValue(livePositionState["waitReason"]), "exit-signal-not-ready")
			}
		}
	}
	if nextPlannedPrice <= 0 {
		nextPlannedPrice = marketPrice
	}

	decisionState := classifyStrategyDecisionState(action, reason, "exit")
	exitProximityBps := computePriceProximityBps(nextPlannedPrice, marketPrice)
	signalKind := classifyStrategySignalKind(action, reason, "exit", "SL", currentPosition, positionPnLBps, 0, exitProximityBps, orderBookStats.bias)
	suggestedQuantity := math.Abs(parseFloatValue(currentPosition["quantity"]))
	if signalBarStateKey == "" && signalBarState != nil {
		signalBarStateKey = resolveSignalBarTradeLimitKey(signalBarState, symbol, timeframe)
	}

	return StrategySignalDecision{
		Action: action,
		Reason: reason,
		Metadata: map[string]any{
			"signalSource":                  "pretouch-timing-unified",
			"signalBarDecision":             signalBarDecision,
			"signalBarState":                cloneMetadata(signalBarState),
			"signalBarStateKey":             signalBarStateKey,
			liveSignalBarTradeLimitKeyField: signalBarStateKey,
			"currentPosition":               currentPosition,
			"livePositionState":             cloneMetadata(livePositionState),
			"nextPlannedEvent":              formatOptionalRFC3339(ctx.EventTime),
			"nextPlannedPrice":              nextPlannedPrice,
			"nextPlannedSide":               exitSide,
			"nextPlannedRole":               "exit",
			"nextPlannedReason":             "SL",
			"marketPrice":                   marketPrice,
			"marketSource":                  marketSource,
			"signalKind":                    signalKind,
			"decisionState":                 decisionState,
			"suggestedQuantity":             suggestedQuantity,
			"positionPnLBps":                positionPnLBps,
			"exitProximityBps":              exitProximityBps,
			"spreadBps":                     orderBookStats.spreadBps,
			"bestBid":                       orderBookStats.bestBid,
			"bestAsk":                       orderBookStats.bestAsk,
			"bestBidQty":                    orderBookStats.bestBidQty,
			"bestAskQty":                    orderBookStats.bestAskQty,
			"bookImbalance":                 orderBookStats.imbalance,
			"liquidityBias":                 orderBookStats.bias,
			"biasActionable":                isLiquidityBiasActionable(exitSide, "exit", "SL", orderBookStats.bias),
		},
	}, true
}

func pretouchSignalBarStateFromEvaluationContext(ctx StrategySignalEvaluationContext, symbol, timeframe string) (map[string]any, string) {
	if state, key := pickSignalBarState(ctx.SignalBarStates, symbol, timeframe); state != nil {
		return state, key
	}
	derived := deriveSignalBarStates(ctx.SourceStates)
	return pickSignalBarState(derived, symbol, timeframe)
}

func pretouchConfigFromParameters(base PretouchDetectorConfig, parameters map[string]any) PretouchDetectorConfig {
	config := base
	if value := parseFloatValue(parameters["pretouchMaxPreTouchSec"]); value > 0 {
		config.MaxPreTouchSeconds = value
	}
	if value := parseFloatValue(parameters["pretouchMaxEff300s"]); value > 0 {
		config.MaxEff300s = value
	}
	if value := parseFloatValue(parameters["pretouchSpeedThreshold"]); value > 0 {
		config.MinSpeed300sATR = value
	}
	if value := parseFloatValue(parameters["pretouchCostQ50Threshold"]); value > 0 {
		config.CostQ50Threshold = value
	}
	if value := parseFloatValue(parameters["pretouchCostQ50Penalty"]); value > 0 {
		config.CostQ50Penalty = value
	}
	if value := parseFloatValue(parameters["pretouchBaseShare"]); value > 0 {
		config.BaseShare = value
	}
	if raw, ok := parameters["breakout_shape_tolerance_bps"]; ok {
		value := parseFloatValue(raw)
		if value < 0 {
			value = defaultT2BreakoutShapeToleranceBps
		}
		config.StructureToleranceBps = value
	}
	return config
}

func normalizePretouchTimingRegime(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fast", "slow":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "skip"
	}
}

func pretouchBarsFromEvaluationContext(ctx StrategySignalEvaluationContext, triggerPrice float64) ([]HourlyBar, *HourlyBar) {
	symbol := NormalizeSymbol(ctx.ExecutionContext.Symbol)
	timeframe := normalizeSignalBarInterval(ctx.ExecutionContext.SignalTimeframe)
	closed, current := pretouchBarsFromSourceStates(ctx.SourceStates, symbol, timeframe)
	if len(closed) == 0 && current == nil {
		closed, current = pretouchBarsFromSignalBarStates(ctx.SignalBarStates, symbol, timeframe)
	}
	if current == nil && len(closed) > 0 && triggerPrice > 0 {
		currentStart := ctx.EventTime.UTC().Truncate(time.Hour)
		lastClosed := closed[len(closed)-1]
		if lastClosed.OpenTime.Before(currentStart) {
			current = &HourlyBar{
				OpenTime: currentStart,
				Open:     triggerPrice,
				High:     triggerPrice,
				Low:      triggerPrice,
				Close:    triggerPrice,
			}
		}
	}
	return closed, current
}

func pretouchBarsFromSourceStates(sourceStates map[string]any, symbol, timeframe string) ([]HourlyBar, *HourlyBar) {
	for _, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil || !strings.EqualFold(stringValue(entry["streamType"]), "signal_bar") {
			continue
		}
		if symbol != "" && NormalizeSymbol(stringValue(entry["symbol"])) != symbol {
			continue
		}
		if timeframe != "" && normalizeSignalBarInterval(stringValue(entry["timeframe"])) != timeframe {
			continue
		}
		return splitPretouchSignalBars(normalizeSignalBarEntries(entry["bars"]))
	}
	return nil, nil
}

func pretouchBarsFromSignalBarStates(signalBarStates map[string]any, symbol, timeframe string) ([]HourlyBar, *HourlyBar) {
	state, _ := pickSignalBarState(signalBarStates, symbol, timeframe)
	if state == nil {
		return nil, nil
	}
	items := make([]map[string]any, 0, 4)
	if prevBar3 := mapValue(state["prevBar3"]); len(prevBar3) > 0 {
		items = append(items, prevBar3)
	}
	if prevBar2 := mapValue(state["prevBar2"]); len(prevBar2) > 0 {
		items = append(items, prevBar2)
	}
	if prevBar1 := mapValue(state["prevBar1"]); len(prevBar1) > 0 {
		items = append(items, prevBar1)
	}
	closed, _ := splitPretouchSignalBars(items)
	currentMap := mapValue(state["current"])
	if len(currentMap) == 0 {
		return closed, nil
	}
	current, ok := pretouchHourlyBarFromMetadata(currentMap)
	if !ok {
		return closed, nil
	}
	if boolValue(currentMap["isClosed"]) {
		closed = append(closed, current)
		return closed, nil
	}
	return closed, &current
}

func splitPretouchSignalBars(items []map[string]any) ([]HourlyBar, *HourlyBar) {
	closed := make([]HourlyBar, 0, len(items))
	var current *HourlyBar
	for _, item := range items {
		bar, ok := pretouchHourlyBarFromMetadata(item)
		if !ok {
			continue
		}
		if boolValue(item["isClosed"]) {
			closed = append(closed, bar)
			continue
		}
		barCopy := bar
		current = &barCopy
	}
	if len(closed) > 24 {
		closed = closed[len(closed)-24:]
	}
	return closed, current
}

func pretouchHourlyBarFromMetadata(item map[string]any) (HourlyBar, bool) {
	if item == nil {
		return HourlyBar{}, false
	}
	millis, ok := signalBarTimestampMillis(item["barStart"])
	if !ok {
		return HourlyBar{}, false
	}
	bar := HourlyBar{
		OpenTime: time.UnixMilli(millis).UTC(),
		Open:     parseFloatValue(item["open"]),
		High:     parseFloatValue(item["high"]),
		Low:      parseFloatValue(item["low"]),
		Close:    parseFloatValue(item["close"]),
	}
	return bar, validHourlyBar(bar)
}

func pretouchBarSnapshot(bar *HourlyBar) map[string]any {
	if bar == nil {
		return map[string]any{}
	}
	return map[string]any{
		"barStart": bar.OpenTime.UTC().Format(time.RFC3339),
		"open":     bar.Open,
		"high":     bar.High,
		"low":      bar.Low,
		"close":    bar.Close,
	}
}

func pretouchSignalBarTradeLimitKey(symbol, timeframe string, current *HourlyBar) string {
	if current == nil || current.OpenTime.IsZero() {
		return ""
	}
	return strings.Join([]string{
		NormalizeSymbol(symbol),
		strings.ToLower(strings.TrimSpace(timeframe)),
		current.OpenTime.UTC().Format(time.RFC3339),
	}, "|")
}

func pretouchSignalBarTradeLimitKeyForKind(symbol, timeframe string, current *HourlyBar, signalKind string) string {
	base := pretouchSignalBarTradeLimitKey(symbol, timeframe, current)
	kind := strings.ToLower(strings.TrimSpace(signalKind))
	if base == "" || kind == "" || kind == "entry" {
		return base
	}
	return base + "|" + kind
}
