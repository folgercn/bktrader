package service

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	bkLiveEthPretouchTimingEngineKey         = "bk-live-eth-pretouch-timing"
	bkLiveEthPretouchTimingStrategyID        = "strategy-bk-eth-pretouch-timing"
	bkLiveEthPretouchTimingStrategyVersionID = "strategy-version-bk-eth-pretouch-timing-v010"
	defaultPretouchModelPath                 = "data/pretouch_model.json"
)

// bkLiveEthPretouchTimingEngine implements the ETH pretouch timing strategy.
// It detects pretouch breakout events in real-time, uses Go-native DT3 + RF
// for timing classification and probability sizing, and produces signal intents.
// No Python dependency; single Go binary.
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
		"description":         "ETH pretouch timing strategy: timing classification × RF probability × cost_q50_cut050. Research-validated 10bps kill stress: 23.29%, 0 neg SM.",
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

	config := pretouchConfigFromParameters(e.config, ctx.ExecutionContext.Parameters)
	e.detector.SetConfig(config)

	closedBars, currentBar := pretouchBarsFromEvaluationContext(ctx, triggerPrice)
	e.detector.SyncBars(closedBars, currentBar)

	orderBookStats := extractOrderBookStats(ctx.TriggerSummary, ctx.SourceStates)

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
	baseQuantity := firstPositive(parseFloatValue(ctx.ExecutionContext.Parameters["pretouchBaseOrderQuantity"]), parseFloatValue(ctx.ExecutionContext.Parameters["defaultOrderQuantity"]))
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
	suggestedQuantity := baseQuantity * finalPositionSize

	logger.Info("pretouch signal: advance-plan",
		"event_id", result.Event.EventID,
		"side", result.Event.Side,
		"timing_regime", timingRegime,
		"rf_probability", rfProb,
		"sizing_multiplier", sizingMultiplier,
		"cost_penalty", result.Event.CostPenalty,
		"final_position_size", finalPositionSize,
		"suggested_quantity", suggestedQuantity,
		"model_version", e.model.Version,
	)

	nextSide := "BUY"
	if strings.EqualFold(result.Event.Side, "short") {
		nextSide = "SELL"
	}
	currentSnapshot := pretouchBarSnapshot(currentBar)
	prevBar2Snapshot := map[string]any{}
	if len(closedBars) >= 2 {
		prevBar2Snapshot = pretouchBarSnapshot(&closedBars[len(closedBars)-2])
	}
	signalBarDecision := map[string]any{
		"ready":                     true,
		"longReady":                 nextSide == "BUY",
		"shortReady":                nextSide == "SELL",
		"longBreakoutPatternReady":  nextSide == "BUY",
		"shortBreakoutPatternReady": nextSide == "SELL",
		"breakoutPrice":             result.Event.TouchPrice,
		"breakoutPriceSource":       "pretouch.touch_price",
		"timeframe":                 "1h",
		"current":                   currentSnapshot,
		"prevBar2":                  prevBar2Snapshot,
		"pretouchTimingRegime":      timingRegime,
		"pretouchRFProbability":     rfProb,
		"pretouchFinalPositionSize": finalPositionSize,
		"pretouchSuggestedQuantity": suggestedQuantity,
	}
	signalBarTradeLimitKey := pretouchSignalBarTradeLimitKey(ctx.ExecutionContext.Symbol, "1h", currentBar)

	// Produce signal decision
	return StrategySignalDecision{
		Action: "advance-plan",
		Reason: fmt.Sprintf("pretouch_%s_%s", timingRegime, result.Event.Side),
		Metadata: map[string]any{
			"pretouchEvent":                 result.Event,
			"timingRegime":                  timingRegime,
			"rfProbability":                 rfProb,
			"sizingMultiplier":              sizingMultiplier,
			"costPenalty":                   result.Event.CostPenalty,
			"finalPositionSize":             finalPositionSize,
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
		},
	}, nil
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
