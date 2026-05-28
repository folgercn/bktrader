package service

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	bkLiveEthPretouchTimingEngineKey           = "bk-live-eth-pretouch-timing"
	bkLiveEthPretouchTimingStrategyID          = "strategy-bk-eth-pretouch-timing"
	bkLiveEthPretouchTimingStrategyVersionID   = "strategy-version-bk-eth-pretouch-timing-v010"
	defaultPretouchModelPath                   = "data/pretouch_model.json"
	defaultPretouchT3OverlayModelPath          = "data/pretouch_t3_overlay_rf_model.json"
	pretouchShadowModeTestnetCollect           = "testnet_shadow_collect"
	pretouchShadowSubmitRiskOnQuantityParam    = "pretouchShadowSubmitRiskOnQuantity"
	pretouchShadowSubmitOverlayOrderParam      = "pretouchShadowSubmitOverlayOrder"
	pretouchShadowLeadQuantityBandSizingParam  = "pretouchShadowLeadQuantityBandSizing"
	pretouchShadowOverlayQualitySizingParam    = "pretouchShadowOverlayQualitySizing"
	pretouchShadowOverlayQualityFallbackParam  = "pretouchShadowOverlayQualityFallbackSubmit"
	pretouchShadowT3StopGateEnabledParam       = "pretouchShadowT3StopGateEnabled"
	pretouchShadowT2StaticDownsizeParam        = "pretouchShadowT2StaticDownsize"
	pretouchShadowT2StaticDownsizeCandidateID  = "static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize"
	defaultPretouchShadowCandidateID           = "lead_q020_q040_overlay_q020_q040_t3_rf_cost_det_stop_gate_20260521"
	defaultPretouchShadowLeadScale             = 1.5
	defaultPretouchShadowT2StaticDownsizeScale = 0.25
	defaultPretouchShadowLeadQuantityMinQty    = 0.20
	defaultPretouchShadowLeadQuantityMaxQty    = 0.40
	defaultPretouchShadowOverlayScale          = 2.0
	defaultPretouchShadowOverlayBaseShare      = 0.40
	defaultPretouchShadowOverlaySpeedMin       = 0.35
	defaultPretouchShadowOverlayQualityMin     = 2.50
	defaultPretouchShadowOverlayQualityMax     = 5.00
	defaultPretouchShadowOverlayQualityCost    = 0.10
	defaultPretouchShadowOverlayQualityMinQty  = 0.20
	defaultPretouchShadowOverlayQualityMaxQty  = 0.40
	pretouchShadowMaxSubmittedQuantityParam    = "pretouchShadowMaxSubmittedQuantity"
	defaultPretouchShadowMaxSubmittedQuantity  = 0.40
	maxPretouchShadowLeadScale                 = defaultPretouchShadowLeadScale
	maxPretouchShadowOverlayScale              = defaultPretouchShadowOverlayScale
	maxPretouchShadowOverlayBaseShare          = defaultPretouchShadowOverlayBaseShare
	maxPretouchShadowOverlayQualityMax         = defaultPretouchShadowOverlayQualityMax
	maxPretouchShadowMaxSubmittedQuantity      = defaultPretouchShadowMaxSubmittedQuantity
	defaultPretouchShadowStrict10Pct           = 35.521555
	defaultPretouchShadowStrict15Pct           = 28.970948
	defaultPretouchShadowSevere15Pct           = 21.231073

	pretouchShadowT3StopGateMinAbsSpeed300sATRParam        = "pretouchShadowT3StopGateMinAbsSpeed300sATR"
	pretouchShadowT3StopGateMinEff300sParam                = "pretouchShadowT3StopGateMinEff300s"
	pretouchShadowT3StopGateMinPreTouchSecondsParam        = "pretouchShadowT3StopGateMinPreTouchSeconds"
	pretouchShadowT3StopGateMaxPreTouchSecondsParam        = "pretouchShadowT3StopGateMaxPreTouchSeconds"
	pretouchShadowT3StopGateMaxAbsTouchExtensionATRParam   = "pretouchShadowT3StopGateMaxAbsTouchExtensionATR"
	pretouchShadowT3StopGateHardStopATRParam               = "pretouchShadowT3StopGateHardStopATR"
	pretouchShadowT3StopGateMinHoldSecondsBeforeTrailParam = "pretouchShadowT3StopGateMinHoldSecondsBeforeTrailingSL"

	defaultPretouchShadowT3StopGateMinAbsSpeed300sATR      = 0.65
	defaultPretouchShadowT3StopGateMinEff300s              = 0.85
	defaultPretouchShadowT3StopGateMinPreTouchSeconds      = 250.0
	defaultPretouchShadowT3StopGateMaxPreTouchSeconds      = 900.0
	defaultPretouchShadowT3StopGateMaxAbsTouchExtensionATR = 0.40
	defaultPretouchShadowT3StopGateHardStopATR             = 3.0
	defaultPretouchShadowT3StopGateTrailingDelaySeconds    = 4740.0
	pretouchT3ExitProfileBaselineID                        = "pr447_lifecycle_baseline"
	pretouchT3ExitProfileDeterministicHard3Delay79ID       = "deterministic_stop_gate_hard3_delay_trailing_79m"
)

// bkLiveEthPretouchTimingEngine implements the ETH pretouch timing strategy.
// It detects pretouch breakout events in real-time, uses Go-native DT3 + RF
// for timing classification and probability sizing, and produces signal intents.
// No Python dependency; single Go binary.
type bkLiveEthPretouchTimingEngine struct {
	platform    *Platform
	detector    *PretouchEventDetector
	t3Detector  *PretouchEventDetector
	model       atomic.Pointer[PretouchModelBundle]
	t3Model     atomic.Pointer[PretouchModelBundle]
	modelPath   string
	t3ModelPath string
	config      PretouchDetectorConfig

	t3MissLogMu  sync.Mutex
	t3MissLogKey string

	leadDelayMu      sync.Mutex
	leadDelayPending *pretouchLeadDelayPending
}

type pretouchLeadDelayPending struct {
	Event          domain.PretouchEvent
	ModelVersion   string
	SignalBarKey   string
	SelectedDelay  string
	ArmedAt        time.Time
	ReadyAt        time.Time
	ExpiresAt      time.Time
	TargetATR      float64
	ReferencePrice float64
	TargetPrice    float64
}

func newBkLiveEthPretouchTimingEngine(platform *Platform) *bkLiveEthPretouchTimingEngine {
	config := DefaultPretouchDetectorConfig()
	engine := &bkLiveEthPretouchTimingEngine{
		platform:    platform,
		detector:    NewPretouchEventDetector("ETHUSDT", config),
		t3Detector:  NewPretouchEventDetector("ETHUSDT", config),
		modelPath:   firstNonEmpty(strings.TrimSpace(os.Getenv("BK_PRETOUCH_MODEL_PATH")), defaultPretouchModelPath),
		t3ModelPath: firstNonEmpty(strings.TrimSpace(os.Getenv("BK_PRETOUCH_T3_OVERLAY_MODEL_PATH")), defaultPretouchT3OverlayModelPath),
		config:      config,
	}

	// Try to load model bundle from default path. Deployments can override this
	// without changing the launch template.
	if bundle, err := LoadModelBundle(engine.modelPath); err == nil {
		engine.setLeadModel(bundle)
		platform.logger("service.pretouch_timing").Info("pretouch model loaded",
			"path", engine.modelPath,
			"version", bundle.Version,
			"timing_loocv", bundle.TimingLOOCV,
			"rf_accuracy", bundle.RFAccuracy,
		)
	} else {
		platform.logger("service.pretouch_timing").Warn("pretouch model not found, events will be skipped until model is trained",
			"path", engine.modelPath,
			"error", err,
		)
	}

	if bundle, err := LoadModelBundle(engine.t3ModelPath); err == nil {
		engine.setT3OverlayModel(bundle)
		platform.logger("service.pretouch_timing").Info("pretouch T3 overlay RF model loaded",
			"path", engine.t3ModelPath,
			"version", bundle.Version,
			"rf_accuracy", bundle.RFAccuracy,
		)
	} else {
		platform.logger("service.pretouch_timing").Warn("pretouch T3 overlay RF model not found, T3 overlay will keep fixed sizing unless the model is available",
			"path", engine.t3ModelPath,
			"error", err,
		)
	}

	return engine
}

func (e *bkLiveEthPretouchTimingEngine) leadModel() *PretouchModelBundle {
	if e == nil {
		return nil
	}
	return e.model.Load()
}

func (e *bkLiveEthPretouchTimingEngine) setLeadModel(bundle *PretouchModelBundle) {
	if e == nil {
		return
	}
	e.model.Store(bundle)
}

func (e *bkLiveEthPretouchTimingEngine) t3OverlayModel() *PretouchModelBundle {
	if e == nil {
		return nil
	}
	return e.t3Model.Load()
}

func (e *bkLiveEthPretouchTimingEngine) setT3OverlayModel(bundle *PretouchModelBundle) {
	if e == nil {
		return
	}
	e.t3Model.Store(bundle)
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

	if decision, handled := e.evaluatePretouchLeadDelayPending(ctx, tick, closedBars, currentBar, orderBookStats); handled {
		return decision, nil
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
	leadModel := e.leadModel()
	if leadModel == nil {
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
	features := make([]float64, len(leadModel.FeatureNames))
	for i, name := range leadModel.FeatureNames {
		if val, ok := result.Event.Features[name]; ok {
			features[i] = val
		} else if i < len(leadModel.Medians) {
			features[i] = leadModel.Medians[i] // impute with train median
		}
	}

	// Timing classification (DT3)
	timingRegime := normalizePretouchTimingRegime(leadModel.TimingTree.Predict(features))

	// RF probability
	rfProb := leadModel.RFModel.PredictProba(features)

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
				"modelVersion":  leadModel.Version,
			},
		}, nil
	}

	if e.shouldArmPretouchLeadDelay(ctx, result.Event, timingRegime) {
		return e.armPretouchLeadDelay(ctx, result.Event, leadModel.Version, currentBar), nil
	}

	return e.buildPretouchLeadAdvanceDecision(ctx, result.Event, leadModel.Version, closedBars, currentBar, orderBookStats, triggerPrice, nil), nil
}

func (e *bkLiveEthPretouchTimingEngine) shouldArmPretouchLeadDelay(ctx StrategySignalEvaluationContext, event domain.PretouchEvent, timingRegime string) bool {
	if e == nil || !strings.EqualFold(timingRegime, "slow") {
		return false
	}
	if !strings.EqualFold(stringValue(ctx.ExecutionContext.Parameters["pretouchShadowMode"]), pretouchShadowModeTestnetCollect) {
		return false
	}
	submitContext := pretouchShadowSubmitContextFromEvaluation(ctx)
	return submitContext.liveExecution && submitContext.sandbox && strings.EqualFold(submitContext.executionMode, "rest")
}

func (e *bkLiveEthPretouchTimingEngine) armPretouchLeadDelay(ctx StrategySignalEvaluationContext, event domain.PretouchEvent, modelVersion string, currentBar *HourlyBar) StrategySignalDecision {
	pending := pretouchLeadDelayPending{
		Event:         event,
		ModelVersion:  modelVersion,
		SignalBarKey:  pretouchSignalBarTradeLimitKey(ctx.ExecutionContext.Symbol, "1h", currentBar),
		SelectedDelay: "pullback",
		ArmedAt:       ctx.EventTime,
		ReadyAt:       event.TouchTime.Add(5 * time.Second),
		ExpiresAt:     event.TouchTime.Add(65 * time.Second),
		TargetATR:     0.05,
	}
	e.leadDelayMu.Lock()
	e.leadDelayPending = &pending
	e.leadDelayMu.Unlock()

	return StrategySignalDecision{
		Action: "wait",
		Reason: "pretouch_delay_policy_armed",
		Metadata: map[string]any{
			"pretouchEvent":       event,
			"timingRegime":        event.TimingRegime,
			"rfProbability":       event.RFProbability,
			"modelVersion":        modelVersion,
			"selectedDelay":       pending.SelectedDelay,
			"pretouchDelayPolicy": pending.metadata("armed"),
			"signalSource":        "pretouch-timing-unified",
			"decisionState":       "waiting",
			"signalKind":          "entry-delay-watch",
		},
	}
}

func (e *bkLiveEthPretouchTimingEngine) evaluatePretouchLeadDelayPending(ctx StrategySignalEvaluationContext, tick TickData, closedBars []HourlyBar, currentBar *HourlyBar, orderBookStats orderBookDecisionStats) (StrategySignalDecision, bool) {
	if e == nil {
		return StrategySignalDecision{}, false
	}
	signalBarKey := pretouchSignalBarTradeLimitKey(ctx.ExecutionContext.Symbol, "1h", currentBar)
	e.leadDelayMu.Lock()
	pending := e.leadDelayPending
	if pending == nil {
		e.leadDelayMu.Unlock()
		return StrategySignalDecision{}, false
	}
	if pending.SignalBarKey != "" && signalBarKey != "" && pending.SignalBarKey != signalBarKey {
		e.leadDelayPending = nil
		e.leadDelayMu.Unlock()
		return StrategySignalDecision{}, false
	}
	if tick.Time.Before(pending.ReadyAt) {
		meta := map[string]any{
			"pretouchEvent":       pending.Event,
			"timingRegime":        pending.Event.TimingRegime,
			"rfProbability":       pending.Event.RFProbability,
			"modelVersion":        pending.ModelVersion,
			"selectedDelay":       pending.SelectedDelay,
			"pretouchDelayPolicy": pending.metadata("waiting"),
			"signalSource":        "pretouch-timing-unified",
			"decisionState":       "waiting",
			"signalKind":          "entry-delay-watch",
		}
		e.leadDelayMu.Unlock()
		return StrategySignalDecision{Action: "wait", Reason: "pretouch_delay_policy_waiting", Metadata: meta}, true
	}
	if pending.ReferencePrice <= 0 {
		pending.ReferencePrice = tick.Price
		pullbackAmount := pending.TargetATR * pending.Event.ATR
		if strings.EqualFold(pending.Event.Side, "short") {
			pending.TargetPrice = pending.ReferencePrice + pullbackAmount
		} else {
			pending.TargetPrice = pending.ReferencePrice - pullbackAmount
		}
	}
	triggered := pending.pullbackTriggered(tick.Price)
	timedOut := !tick.Time.Before(pending.ExpiresAt)
	status := "waiting"
	if triggered {
		status = "triggered"
	}
	if timedOut && !triggered {
		status = "timeout_fallback"
	}
	if !triggered && !timedOut {
		meta := map[string]any{
			"pretouchEvent":       pending.Event,
			"timingRegime":        pending.Event.TimingRegime,
			"rfProbability":       pending.Event.RFProbability,
			"modelVersion":        pending.ModelVersion,
			"selectedDelay":       pending.SelectedDelay,
			"pretouchDelayPolicy": pending.metadata(status),
			"signalSource":        "pretouch-timing-unified",
			"decisionState":       "waiting",
			"signalKind":          "entry-delay-watch",
			"marketPrice":         tick.Price,
			"marketSource":        "trade_tick.price",
		}
		e.leadDelayMu.Unlock()
		return StrategySignalDecision{Action: "wait", Reason: "pretouch_delay_policy_waiting_pullback", Metadata: meta}, true
	}
	pendingCopy := *pending
	e.leadDelayPending = nil
	e.leadDelayMu.Unlock()

	delayMetadata := pendingCopy.metadata(status)
	return e.buildPretouchLeadAdvanceDecision(ctx, pendingCopy.Event, pendingCopy.ModelVersion, closedBars, currentBar, orderBookStats, tick.Price, delayMetadata), true
}

func (p pretouchLeadDelayPending) pullbackTriggered(price float64) bool {
	if price <= 0 || p.TargetPrice <= 0 {
		return false
	}
	if strings.EqualFold(p.Event.Side, "short") {
		return price >= p.TargetPrice
	}
	return price <= p.TargetPrice
}

func (p pretouchLeadDelayPending) metadata(status string) map[string]any {
	return map[string]any{
		"name":                   "slow_pullback_delay_policy",
		"policySource":           "research_lead_selected_delay",
		"status":                 status,
		"selectedDelay":          p.SelectedDelay,
		"armedAt":                formatOptionalRFC3339(p.ArmedAt),
		"readyAt":                formatOptionalRFC3339(p.ReadyAt),
		"expiresAt":              formatOptionalRFC3339(p.ExpiresAt),
		"pullbackStartOffsetSec": 5.0,
		"pullbackWindowSeconds":  60.0,
		"pullbackTargetATR":      p.TargetATR,
		"referencePrice":         p.ReferencePrice,
		"targetPrice":            p.TargetPrice,
		"signalBarStateKey":      p.SignalBarKey,
		"eventId":                p.Event.EventID,
	}
}

func (e *bkLiveEthPretouchTimingEngine) buildPretouchLeadAdvanceDecision(ctx StrategySignalEvaluationContext, event domain.PretouchEvent, modelVersion string, closedBars []HourlyBar, currentBar *HourlyBar, orderBookStats orderBookDecisionStats, marketPrice float64, delayMetadata map[string]any) StrategySignalDecision {
	timingRegime := normalizePretouchTimingRegime(event.TimingRegime)
	rfProb := event.RFProbability
	sizingMultiplier := event.SizingMultiplier
	config := pretouchConfigFromParameters(e.config, ctx.ExecutionContext.Parameters)
	finalMultiplier := sizingMultiplier * event.CostPenalty
	finalPositionSize := config.BaseShare * finalMultiplier
	event.FinalPositionSize = finalPositionSize
	baseQuantity := pretouchBaseOrderQuantityFromParameters(ctx.ExecutionContext.Parameters)
	if baseQuantity <= 0 {
		return StrategySignalDecision{
			Action: "wait",
			Reason: "no_base_quantity",
			Metadata: map[string]any{
				"pretouchEvent": event.EventID,
				"timingRegime":  timingRegime,
				"rfProbability": rfProb,
				"modelVersion":  modelVersion,
			},
		}
	}
	nextSide := nextPretouchSide(event.Side)
	productionSuggestedQuantity := pretouchCapSubmittedQuantity(ctx.ExecutionContext.Parameters, baseQuantity*finalPositionSize)
	shadowSizing := pretouchShadowSizingFromParameters(
		ctx.ExecutionContext.Parameters,
		nextSide,
		productionSuggestedQuantity,
		orderBookStats,
		pretouchShadowSubmitContextFromEvaluation(ctx),
	)
	t2StaticDownsize := pretouchT2StaticDownsizeCandidateFromEvent(ctx.ExecutionContext.Parameters, event, closedBars, rfProb)
	if shadowSizing != nil && t2StaticDownsize != nil {
		pretouchApplyT2StaticDownsizeShadow(shadowSizing, t2StaticDownsize)
	}
	suggestedQuantity := productionSuggestedQuantity
	if shadowSizing != nil {
		if submittedQuantityAfterShadow := parseFloatValue(shadowSizing["submittedQuantityAfterShadow"]); submittedQuantityAfterShadow > 0 {
			suggestedQuantity = submittedQuantityAfterShadow
		}
	}

	e.platform.logger("service.pretouch_timing",
		"symbol", ctx.ExecutionContext.Symbol,
	).Info("pretouch signal: advance-plan",
		"event_id", event.EventID,
		"side", event.Side,
		"timing_regime", timingRegime,
		"rf_probability", rfProb,
		"sizing_multiplier", sizingMultiplier,
		"cost_penalty", event.CostPenalty,
		"final_position_size", finalPositionSize,
		"production_suggested_quantity", productionSuggestedQuantity,
		"suggested_quantity", suggestedQuantity,
		"shadow_risk_on_quantity_enabled", boolValue(mapValue(shadowSizing)["submittedRiskOnQuantityEnabled"]),
		"model_version", modelVersion,
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
		"breakoutPrice":                       event.TouchPrice,
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
	if t2StaticDownsize != nil {
		signalBarDecision["t2StaticDownsizeCandidate"] = cloneMetadata(t2StaticDownsize)
	}
	if delayMetadata != nil {
		signalBarDecision["pretouchDelayPolicy"] = cloneMetadata(delayMetadata)
	}
	signalBarTradeLimitKey := pretouchSignalBarTradeLimitKey(ctx.ExecutionContext.Symbol, "1h", currentBar)

	metadata := map[string]any{
		"pretouchEvent":                 event,
		"timingRegime":                  timingRegime,
		"rfProbability":                 rfProb,
		"sizingMultiplier":              sizingMultiplier,
		"costPenalty":                   event.CostPenalty,
		"finalPositionSize":             finalPositionSize,
		"productionSuggestedQuantity":   productionSuggestedQuantity,
		"suggestedQuantity":             suggestedQuantity,
		"modelVersion":                  modelVersion,
		"signalSource":                  "pretouch-timing-unified",
		"signalBarDecision":             signalBarDecision,
		"signalBarStateKey":             signalBarTradeLimitKey,
		liveSignalBarTradeLimitKeyField: signalBarTradeLimitKey,
		"nextPlannedEvent":              formatOptionalRFC3339(event.TouchTime),
		"nextPlannedPrice":              event.TouchPrice,
		"nextPlannedSide":               nextSide,
		"nextPlannedRole":               "entry",
		"nextPlannedReason":             "Pretouch-Timing",
		"marketPrice":                   marketPrice,
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
	if t2StaticDownsize != nil {
		metadata["t2StaticDownsizeCandidate"] = t2StaticDownsize
	}
	if delayMetadata != nil {
		metadata["pretouchDelayPolicy"] = delayMetadata
		metadata["selectedDelay"] = stringValue(delayMetadata["selectedDelay"])
	}

	return StrategySignalDecision{
		Action:   "advance-plan",
		Reason:   fmt.Sprintf("pretouch_%s_%s", timingRegime, event.Side),
		Metadata: metadata,
	}
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
		e.logT3ShadowOverlayMiss(ctx, tick, leadMissReason, result)
		if metadata := pretouchT3OverlayRejectMetadata(leadMissReason, result); len(metadata) > 0 {
			return StrategySignalDecision{
				Action:   "wait",
				Reason:   leadMissReason,
				Metadata: metadata,
			}, true
		}
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
	stopGate := pretouchT3DeterministicStopGate(ctx.ExecutionContext.Parameters, result.Event)
	exitProfile := cloneMetadata(mapValue(stopGate["selectedExitProfile"]))
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
		"pretouchT3StopGate":          cloneMetadata(stopGate),
		"pretouchT3ExitProfile":       cloneMetadata(exitProfile),
	}

	metadata := map[string]any{
		"pretouchEvent":                 result.Event,
		"pretouchEventShape":            "t3_swing",
		"pretouchShadowOverlaySizing":   overlaySizing,
		"pretouchT3StopGate":            stopGate,
		"pretouchT3ExitProfile":         exitProfile,
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

func (e *bkLiveEthPretouchTimingEngine) logT3ShadowOverlayMiss(ctx StrategySignalEvaluationContext, tick TickData, leadMissReason string, result PretouchDetectionResult) {
	if e == nil || e.platform == nil || !pretouchT3OverlayMissLoggable(result.Reason) {
		return
	}
	diagnostics := cloneMetadata(result.Diagnostics)
	category := pretouchT3OverlayMissReasonCategory(result.Reason)
	signalBarStart := firstNonEmpty(stringValue(diagnostics["signalBarStart"]), formatOptionalRFC3339(ctx.EventTime.Truncate(time.Hour)))
	key := strings.Join([]string{
		NormalizeSymbol(ctx.ExecutionContext.Symbol),
		signalBarStart,
		category,
		stringValue(diagnostics["side"]),
	}, "|")
	if !e.shouldLogT3ShadowOverlayMiss(key) {
		return
	}
	e.platform.logger("service.pretouch_timing",
		"symbol", ctx.ExecutionContext.Symbol,
		"strategy_version_id", ctx.ExecutionContext.StrategyVersionID,
		"signal_timeframe", ctx.ExecutionContext.SignalTimeframe,
	).Info("pretouch T3 overlay rejected",
		"lead_miss_reason", leadMissReason,
		"t3_miss_reason", result.Reason,
		"t3_miss_category", category,
		"event_time", formatOptionalRFC3339(ctx.EventTime),
		"tick_price", tick.Price,
		"t3_side", stringValue(diagnostics["side"]),
		"t3_level", parseFloatValue(diagnostics["level"]),
		"t3_touch_price", parseFloatValue(diagnostics["touchPrice"]),
		"t3_signal_bar_start", stringValue(diagnostics["signalBarStart"]),
		"t3_pre_touch_seconds", parseFloatValue(diagnostics["preTouchSeconds"]),
		"t3_speed_300s_atr", parseFloatValue(diagnostics["speed300sATR"]),
		"t3_min_abs_speed_300s_atr", parseFloatValue(diagnostics["minAbsSpeed300sATR"]),
		"t3_eff_300s", parseFloatValue(diagnostics["eff300s"]),
		"t3_max_eff_300s", parseFloatValue(diagnostics["maxEff300s"]),
		"t3_touch_extension_atr", parseFloatValue(diagnostics["touchExtensionATR"]),
		"t3_roundtrip_cost_atr", parseFloatValue(diagnostics["roundtripCostATR"]),
		"diagnostics", diagnostics,
	)
}

func pretouchT3OverlayRejectMetadata(leadMissReason string, result PretouchDetectionResult) map[string]any {
	if len(result.Diagnostics) == 0 || !pretouchT3OverlayMissLoggable(result.Reason) {
		return nil
	}
	diagnostics := cloneMetadata(result.Diagnostics)
	category := pretouchT3OverlayMissReasonCategory(result.Reason)
	return map[string]any{
		"signalSource":                 "pretouch-t3-overlay-shadow",
		"signalKind":                   "entry-t3-overlay-watch",
		"decisionState":                "rejected",
		"t3OverlayRejected":            true,
		"t3OverlayLeadMissReason":      leadMissReason,
		"t3OverlayRejectReason":        result.Reason,
		"t3OverlayRejectCategory":      category,
		"t3OverlaySide":                stringValue(diagnostics["side"]),
		"t3OverlayLevel":               parseFloatValue(diagnostics["level"]),
		"t3OverlayTouchPrice":          parseFloatValue(diagnostics["touchPrice"]),
		"t3OverlaySignalBarStart":      stringValue(diagnostics["signalBarStart"]),
		"t3OverlayPreTouchSeconds":     parseFloatValue(diagnostics["preTouchSeconds"]),
		"t3OverlaySpeed300sATR":        parseFloatValue(diagnostics["speed300sATR"]),
		"t3OverlayMinAbsSpeed300sATR":  parseFloatValue(diagnostics["minAbsSpeed300sATR"]),
		"t3OverlayEff300s":             parseFloatValue(diagnostics["eff300s"]),
		"t3OverlayMaxEff300s":          parseFloatValue(diagnostics["maxEff300s"]),
		"t3OverlayTouchExtensionATR":   parseFloatValue(diagnostics["touchExtensionATR"]),
		"t3OverlayRoundtripCostATR":    parseFloatValue(diagnostics["roundtripCostATR"]),
		"t3OverlayLongStructureReady":  boolValue(diagnostics["longStructureReady"]),
		"t3OverlayShortStructureReady": boolValue(diagnostics["shortStructureReady"]),
		"t3OverlayDiagnostics":         diagnostics,
	}
}

func (e *bkLiveEthPretouchTimingEngine) shouldLogT3ShadowOverlayMiss(key string) bool {
	if e == nil {
		return false
	}
	e.t3MissLogMu.Lock()
	defer e.t3MissLogMu.Unlock()
	if key != "" && e.t3MissLogKey == key {
		return false
	}
	e.t3MissLogKey = key
	return true
}

func pretouchT3OverlayMissLoggable(reason string) bool {
	switch pretouchT3OverlayMissReasonCategory(reason) {
	case "t3_pre_touch_seconds", "t3_speed_300s", "t3_eff_300s":
		return true
	default:
		return false
	}
}

func pretouchT3OverlayMissReasonCategory(reason string) string {
	reason = strings.TrimSpace(reason)
	switch {
	case strings.HasPrefix(reason, "t3_pre_touch_seconds="):
		return "t3_pre_touch_seconds"
	case strings.HasPrefix(reason, "t3_speed_300s="):
		return "t3_speed_300s"
	case strings.HasPrefix(reason, "t3_eff_300s="):
		return "t3_eff_300s"
	default:
		return reason
	}
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

func pretouchT2StaticDownsizeCandidateFromEvent(parameters map[string]any, event domain.PretouchEvent, closedBars []HourlyBar, rfProbability float64) map[string]any {
	if !strings.EqualFold(stringValue(parameters["pretouchShadowMode"]), pretouchShadowModeTestnetCollect) {
		return nil
	}
	requested := boolValue(parameters[pretouchShadowT2StaticDownsizeParam])
	scale := firstPositive(parseFloatValue(parameters["pretouchShadowT2StaticDownsizeScale"]), defaultPretouchShadowT2StaticDownsizeScale)
	if scale <= 0 || scale > 1 {
		scale = defaultPretouchShadowT2StaticDownsizeScale
	}
	candidate := map[string]any{
		"candidateId":            pretouchShadowT2StaticDownsizeCandidateID,
		"stage":                  "testnet_shadow_collect",
		"requested":              requested,
		"enabled":                requested,
		"scale":                  scale,
		"wouldDownsize":          false,
		"applied":                false,
		"mainnetSizingPermitted": false,
	}
	context, ok := pretouchClosedBarContextFeatures(closedBars, event)
	candidate["context"] = pretouchClosedBarContextMetadata(context, event, len(closedBars), ok)
	if !ok {
		candidate["status"] = "insufficient_closed_bar_context"
		return candidate
	}
	staticOptimal := event.Eff300s >= 0.925057 && context.ctx12hSideReturnATR <= -0.282982
	docRuleA := event.TouchExtensionATR <= -0.112263 && context.ctx12hRangeATR >= 3.006207
	rangeCap := context.ctx12hRangeATR <= 3.500000
	profitProtection := context.ctx12hSideReturnATR >= 1.45 ||
		context.ctx4hRangeATR >= 2.36 ||
		context.ctx12hRangeATR >= 3.98 ||
		rfProbability >= 0.965
	wouldDownsize := requested && (staticOptimal || docRuleA) && rangeCap && !profitProtection
	status := "not_selected"
	if wouldDownsize {
		status = "selected"
	}
	candidate["status"] = status
	candidate["wouldDownsize"] = wouldDownsize
	candidate["profitProtection"] = profitProtection
	candidate["rangeCapPass"] = rangeCap
	candidate["branches"] = map[string]any{
		"staticOptimal": staticOptimal,
		"docRuleA":      docRuleA,
	}
	candidate["features"] = map[string]any{
		"rf_probability":         rfProbability,
		"eff_300s":               event.Eff300s,
		"touch_extension_atr":    event.TouchExtensionATR,
		"ctx12h_side_return_atr": context.ctx12hSideReturnATR,
		"ctx4h_range_atr":        context.ctx4hRangeATR,
		"ctx12h_range_atr":       context.ctx12hRangeATR,
		"speed_300s_atr":         event.Speed300sATR,
		"pre_touch_seconds":      event.PreTouchSeconds,
		"roundtrip_cost_atr":     event.RoundtripCostATR,
		"signal_bar_start":       event.SignalBarStart.UTC().Format(time.RFC3339),
		"event_id":               event.EventID,
	}
	candidate["thresholds"] = map[string]any{
		"eff_300s_gte":               0.925057,
		"ctx12h_side_return_atr_lte": -0.282982,
		"touch_extension_atr_lte":    -0.112263,
		"ctx12h_range_atr_gte":       3.006207,
		"ctx12h_range_atr_lte":       3.500000,
		"pp_ctx12h_side_gte":         1.45,
		"pp_ctx4h_range_gte":         2.36,
		"pp_ctx12h_range_gte":        3.98,
		"pp_rf_probability_gte":      0.965,
	}
	return candidate
}

type pretouchContextFeatures struct {
	ctx4hSideReturnATR  float64
	ctx12hSideReturnATR float64
	ctx4hRangeATR       float64
	ctx12hRangeATR      float64
	closedBarsUsed      int
	lastClosedBarStart  time.Time
}

func pretouchClosedBarContextMetadata(context pretouchContextFeatures, event domain.PretouchEvent, closedBarsAvailable int, ok bool) map[string]any {
	if !ok {
		return map[string]any{
			"closedBarsAvailable": closedBarsAvailable,
			"requiredClosedBars":  13,
			"atr":                 event.ATR,
		}
	}
	return map[string]any{
		"ctx4h_side_return_atr":  context.ctx4hSideReturnATR,
		"ctx12h_side_return_atr": context.ctx12hSideReturnATR,
		"ctx4h_range_atr":        context.ctx4hRangeATR,
		"ctx12h_range_atr":       context.ctx12hRangeATR,
		"closedBarsUsed":         context.closedBarsUsed,
		"lastClosedBarStart":     context.lastClosedBarStart.UTC().Format(time.RFC3339),
		"atr":                    event.ATR,
	}
}

func pretouchClosedBarContextFeatures(closedBars []HourlyBar, event domain.PretouchEvent) (pretouchContextFeatures, bool) {
	if event.ATR <= 0 || len(closedBars) == 0 {
		return pretouchContextFeatures{}, false
	}
	filtered := make([]HourlyBar, 0, len(closedBars))
	for _, bar := range closedBars {
		if validHourlyBar(bar) && bar.OpenTime.Before(event.SignalBarStart) {
			filtered = append(filtered, bar)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].OpenTime.Before(filtered[j].OpenTime)
	})
	last := len(filtered) - 1
	if last < 12 {
		return pretouchContextFeatures{}, false
	}
	sideSign := 1.0
	if strings.EqualFold(event.Side, "short") {
		sideSign = -1.0
	}
	ret := func(hours int) float64 {
		start := last - hours
		if start < 0 {
			return math.NaN()
		}
		return (filtered[last].Close - filtered[start].Close) / event.ATR * sideSign
	}
	rng := func(hours int) float64 {
		start := last - hours + 1
		if start < 0 {
			return math.NaN()
		}
		high := filtered[start].High
		low := filtered[start].Low
		for _, bar := range filtered[start : last+1] {
			if bar.High > high {
				high = bar.High
			}
			if bar.Low < low {
				low = bar.Low
			}
		}
		return (high - low) / event.ATR
	}
	context := pretouchContextFeatures{
		ctx4hSideReturnATR:  ret(4),
		ctx12hSideReturnATR: ret(12),
		ctx4hRangeATR:       rng(4),
		ctx12hRangeATR:      rng(12),
		closedBarsUsed:      len(filtered),
		lastClosedBarStart:  filtered[last].OpenTime,
	}
	if !pretouchIsFiniteFloat(context.ctx4hSideReturnATR) ||
		!pretouchIsFiniteFloat(context.ctx12hSideReturnATR) ||
		!pretouchIsFiniteFloat(context.ctx4hRangeATR) ||
		!pretouchIsFiniteFloat(context.ctx12hRangeATR) {
		return pretouchContextFeatures{}, false
	}
	return context, true
}

func pretouchIsFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func pretouchApplyT2StaticDownsizeShadow(shadowSizing map[string]any, candidate map[string]any) {
	if shadowSizing == nil || candidate == nil {
		return
	}
	shadowSizing["t2StaticDownsizeCandidate"] = cloneMetadata(candidate)
	if !boolValue(candidate["wouldDownsize"]) {
		return
	}
	if !boolValue(shadowSizing["submittedRiskOnQuantityEnabled"]) {
		candidate["appliedBlockReason"] = "risk_on_shadow_quantity_not_enabled"
		shadowSizing["t2StaticDownsizeCandidate"] = cloneMetadata(candidate)
		return
	}
	before := parseFloatValue(shadowSizing["submittedQuantityAfterShadow"])
	scale := parseFloatValue(candidate["scale"])
	if before <= 0 || scale <= 0 || scale > 1 {
		candidate["appliedBlockReason"] = "invalid_shadow_quantity_or_scale"
		shadowSizing["t2StaticDownsizeCandidate"] = cloneMetadata(candidate)
		return
	}
	after := before * scale
	candidate["applied"] = true
	candidate["submittedQuantityBeforeT2Downsize"] = before
	candidate["submittedQuantityAfterT2Downsize"] = after
	shadowSizing["submittedQuantityBeforeT2Downsize"] = before
	shadowSizing["submittedQuantityAfterT2Downsize"] = after
	shadowSizing["submittedQuantityAfterShadow"] = after
	shadowSizing["submittedQuantity"] = after
	shadowSizing["submittedSizingMode"] = fmt.Sprintf("%s_t2_static_downsize%.2f", stringValue(shadowSizing["submittedSizingMode"]), scale)
	shadowSizing["t2StaticDownsizeCandidate"] = cloneMetadata(candidate)
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
		"requested":             true,
		"enabled":               false,
		"status":                "model_missing",
		"multiplier":            1.0,
		"minMultiplier":         minMultiplier,
		"maxMultiplier":         maxMultiplier,
		"quantityBandEnabled":   true,
		"minQuantity":           minQuantity,
		"maxQuantity":           maxQuantity,
		"costThresholdATR":      costThresholdATR,
		"costPenalty":           1.0,
		"fallbackSubmitAllowed": pretouchShadowOverlayQualityFallbackSubmitAllowed(parameters),
		"mainnetPermitted":      false,
		"modelArtifactPath":     defaultPretouchT3OverlayModelPath,
		"modelArtifactEnvVar":   "BK_PRETOUCH_T3_OVERLAY_MODEL_PATH",
	}
	t3Model := e.t3OverlayModel()
	if t3Model == nil || t3Model.RFModel == nil {
		return quality
	}
	leadModel := e.leadModel()
	quality["modelVersion"] = t3Model.Version
	features, featureMeta, ok := e.buildPretouchT3OverlayQualityFeatures(event, t3Model, leadModel)
	quality["features"] = featureMeta
	if !ok {
		quality["status"] = "feature_build_failed"
		return quality
	}
	probability := pretouchFiniteFloat(t3Model.RFModel.PredictProba(features), 0.5)
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

func (e *bkLiveEthPretouchTimingEngine) buildPretouchT3OverlayQualityFeatures(event domain.PretouchEvent, t3Model, leadModel *PretouchModelBundle) ([]float64, map[string]any, bool) {
	var featureNames []string
	if t3Model != nil {
		featureNames = t3Model.FeatureNames
	}
	features := make([]float64, len(featureNames))
	meta := make(map[string]any, len(featureNames)+3)
	leadProbability := 0.0
	leadProbabilityOK := false
	if pretouchFeatureNamesContain(featureNames, "rf_probability") {
		leadProbability, leadProbabilityOK = e.predictPretouchLeadRFProbability(event.Features, leadModel)
		meta["leadRFProbability"] = leadProbability
		meta["leadRFProbabilityAvailable"] = leadProbabilityOK
		if leadModel != nil {
			meta["leadModelVersion"] = leadModel.Version
		}
		if !leadProbabilityOK {
			meta["missingFeature"] = "rf_probability"
			return features, meta, false
		}
	}
	for i, name := range featureNames {
		value, ok := pretouchT3OverlayFeatureValue(event, name, leadProbability)
		if !ok {
			if t3Model != nil && i < len(t3Model.Medians) {
				value = t3Model.Medians[i]
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

func (e *bkLiveEthPretouchTimingEngine) predictPretouchLeadRFProbability(eventFeatures map[string]float64, leadModel *PretouchModelBundle) (float64, bool) {
	if leadModel == nil || leadModel.RFModel == nil || len(leadModel.FeatureNames) == 0 {
		return 0, false
	}
	features := make([]float64, len(leadModel.FeatureNames))
	for i, name := range leadModel.FeatureNames {
		value, ok := eventFeatures[name]
		if !ok {
			if i >= len(leadModel.Medians) {
				return 0, false
			}
			value = leadModel.Medians[i]
		}
		features[i] = pretouchFiniteFloat(value, 0)
	}
	return pretouchFiniteFloat(leadModel.RFModel.PredictProba(features), 0.5), true
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

func pretouchShadowT3StopGateEnabled(parameters map[string]any) bool {
	if _, ok := parameters[pretouchShadowT3StopGateEnabledParam]; !ok {
		return false
	}
	return boolValue(parameters[pretouchShadowT3StopGateEnabledParam])
}

func pretouchT3DeterministicStopGate(parameters map[string]any, event domain.PretouchEvent) map[string]any {
	enabled := pretouchShadowT3StopGateEnabled(parameters)
	minAbsSpeed := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateMinAbsSpeed300sATRParam]), defaultPretouchShadowT3StopGateMinAbsSpeed300sATR)
	minEff := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateMinEff300sParam]), defaultPretouchShadowT3StopGateMinEff300s)
	minPreTouch := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateMinPreTouchSecondsParam]), defaultPretouchShadowT3StopGateMinPreTouchSeconds)
	maxPreTouch := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateMaxPreTouchSecondsParam]), defaultPretouchShadowT3StopGateMaxPreTouchSeconds)
	maxAbsExtension := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateMaxAbsTouchExtensionATRParam]), defaultPretouchShadowT3StopGateMaxAbsTouchExtensionATR)
	hardStopATR := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateHardStopATRParam]), defaultPretouchShadowT3StopGateHardStopATR)
	trailingDelaySeconds := firstPositive(parseFloatValue(parameters[pretouchShadowT3StopGateMinHoldSecondsBeforeTrailParam]), defaultPretouchShadowT3StopGateTrailingDelaySeconds)

	absSpeed := math.Abs(event.Speed300sATR)
	absExtension := math.Abs(event.TouchExtensionATR)
	failedRules := []string{}
	if !enabled {
		failedRules = append(failedRules, "gate_disabled")
	}
	if absSpeed < minAbsSpeed {
		failedRules = append(failedRules, "speed_300s_abs_below_min")
	}
	if event.Eff300s < minEff {
		failedRules = append(failedRules, "eff_300s_below_min")
	}
	if event.PreTouchSeconds < minPreTouch {
		failedRules = append(failedRules, "pre_touch_seconds_below_min")
	}
	if maxPreTouch > 0 && event.PreTouchSeconds > maxPreTouch {
		failedRules = append(failedRules, "pre_touch_seconds_above_max")
	}
	if absExtension > maxAbsExtension {
		failedRules = append(failedRules, "touch_extension_abs_above_max")
	}
	pass := enabled && len(failedRules) == 0
	profileID := pretouchT3ExitProfileBaselineID
	selected := false
	if pass {
		profileID = pretouchT3ExitProfileDeterministicHard3Delay79ID
		selected = true
	}
	exitProfile := map[string]any{
		"id":                                    profileID,
		"eventSource":                           "t3_swing",
		"selected":                              selected,
		"selector":                              "deterministic_stop_gate",
		"fallbackProfile":                       pretouchT3ExitProfileBaselineID,
		"hardStopATR":                           0.0,
		"minHoldSecondsBeforeTrailingSL":        0.0,
		"delayTrailingUpdatesBeforeHoldSeconds": 0.0,
	}
	if selected {
		exitProfile["hardStopATR"] = hardStopATR
		exitProfile["minHoldSecondsBeforeTrailingSL"] = trailingDelaySeconds
		exitProfile["delayTrailingUpdatesBeforeHoldSeconds"] = trailingDelaySeconds
	}
	return map[string]any{
		"name":        "deterministic_stop_gate",
		"enabled":     enabled,
		"pass":        pass,
		"decision":    map[bool]string{true: "selected", false: "baseline"}[pass],
		"failedRules": failedRules,
		"features": map[string]any{
			"speed_300s_atr":      event.Speed300sATR,
			"speed_300s_abs_atr":  absSpeed,
			"eff_300s":            event.Eff300s,
			"pre_touch_seconds":   event.PreTouchSeconds,
			"touch_extension_atr": event.TouchExtensionATR,
			"touch_extension_abs": absExtension,
			"roundtrip_cost_atr":  event.RoundtripCostATR,
			"side":                event.Side,
			"eventId":             event.EventID,
		},
		"thresholds": map[string]any{
			"min_abs_speed_300s_atr":      minAbsSpeed,
			"min_eff_300s":                minEff,
			"min_pre_touch_seconds":       minPreTouch,
			"max_pre_touch_seconds":       maxPreTouch,
			"max_abs_touch_extension_atr": maxAbsExtension,
		},
		"selectedExitProfile": exitProfile,
		"exitProfile":         exitProfile,
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

func pretouchShadowOverlayQualityFallbackSubmitAllowed(parameters map[string]any) bool {
	if _, ok := parameters[pretouchShadowOverlayQualityFallbackParam]; !ok {
		return false
	}
	return boolValue(parameters[pretouchShadowOverlayQualityFallbackParam])
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
	depthPass := shadowOverlayQuantity > 0 &&
		overlayCoverage >= minCoverage &&
		(orderBookStats.spreadBps <= 0 || orderBookStats.spreadBps <= maxSpreadBps)
	depthBlockReason := pretouchShadowBlockReason(shadowOverlayQuantity, overlayCoverage, overlayCoverage, minCoverage, orderBookStats.spreadBps, maxSpreadBps)
	qualityBlockReason := pretouchShadowOverlayQualityBlockReason(qualitySizing)
	overlayPass := depthPass && qualityBlockReason == ""
	overlayBlockReason := firstNonEmpty(qualityBlockReason, depthBlockReason)
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
		"shadowDepthPreSubmitPass":           depthPass,
		"shadowBlockReason":                  overlayBlockReason,
		"overlayQualityBlockReason":          qualityBlockReason,
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

func pretouchShadowOverlayQualityBlockReason(qualitySizing map[string]any) string {
	if qualitySizing == nil || !boolValue(qualitySizing["requested"]) || boolValue(qualitySizing["enabled"]) || boolValue(qualitySizing["fallbackSubmitAllowed"]) {
		return ""
	}
	switch firstNonEmpty(stringValue(qualitySizing["status"]), "unknown") {
	case "model_missing":
		return "overlay_quality_model_missing"
	case "feature_build_failed":
		return "overlay_quality_feature_build_failed"
	default:
		return "overlay_quality_not_applied"
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
