package service

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type ExecutionMode string

const (
	ExecutionModeBacktest ExecutionMode = "BACKTEST"
	ExecutionModePaper    ExecutionMode = "PAPER"
	ExecutionModeLive     ExecutionMode = "LIVE"
)

type SlippageMode string

const (
	SlippageModeSimulated SlippageMode = "SIMULATED"
	SlippageModeObserved  SlippageMode = "OBSERVED"
)

type StrategyExecutionSemantics struct {
	Mode                  ExecutionMode `json:"mode"`
	SlippageMode          SlippageMode  `json:"slippageMode"`
	SimulatedSlippageBps  float64       `json:"simulatedSlippageBps"`
	TradingFeeBps         float64       `json:"tradingFeeBps"`
	FundingRateBps        float64       `json:"fundingRateBps"`
	FundingIntervalHours  int           `json:"fundingIntervalHours"`
	UseCanonicalExecution bool          `json:"useCanonicalExecution"`
}

type StrategyExecutionContext struct {
	StrategyEngineKey   string
	StrategyVersionID   string
	SignalTimeframe     string
	ExecutionDataSource string
	Symbol              string
	From                time.Time
	To                  time.Time
	Parameters          map[string]any
	Semantics           StrategyExecutionSemantics
}

type StrategySignalEvaluationContext struct {
	ExecutionContext  StrategyExecutionContext
	PaperSessionID    string
	TriggerSummary    map[string]any
	SourceStates      map[string]any
	SignalBarStates   map[string]any
	CurrentPosition   map[string]any
	SessionState      map[string]any
	EventTime         time.Time
	NextPlannedEvent  time.Time
	NextPlannedPrice  float64
	NextPlannedSide   string
	NextPlannedRole   string
	NextPlannedReason string
}

type StrategySignalDecision struct {
	Action   string         `json:"action"`
	Reason   string         `json:"reason"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type StrategyEngine interface {
	Key() string
	Describe() map[string]any
	Run(context StrategyExecutionContext) (map[string]any, error)
}

type SignalEvaluatingStrategyEngine interface {
	StrategyEngine
	EvaluateSignal(context StrategySignalEvaluationContext) (StrategySignalDecision, error)
}

func defaultExecutionSemantics(mode ExecutionMode, parameters map[string]any) StrategyExecutionSemantics {
	semantics := StrategyExecutionSemantics{
		Mode:                  mode,
		UseCanonicalExecution: true,
		SlippageMode:          SlippageModeObserved,
		SimulatedSlippageBps:  0,
		TradingFeeBps:         firstPositive(parseFloatValue(parameters["tradingFeeBps"]), 10),
		FundingRateBps:        parseFloatValue(parameters["fundingRateBps"]),
		FundingIntervalHours:  maxIntValue(parameters["fundingIntervalHours"], 8),
	}
	if mode == ExecutionModeBacktest {
		semantics.SlippageMode = SlippageModeSimulated
		semantics.SimulatedSlippageBps = firstPositive(parseFloatValue(parameters["fixed_slippage"])*10000, 5)
	}
	return semantics
}

func normalizeStrategyEngineKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "bk-default"
	}
	return value
}

func (p *Platform) registerStrategyEngine(engine StrategyEngine) {
	if engine == nil {
		return
	}
	if p.strategyEngines == nil {
		p.strategyEngines = make(map[string]StrategyEngine)
	}
	p.strategyEngines[normalizeStrategyEngineKey(engine.Key())] = engine
}

func (p *Platform) registerBuiltInStrategyEngines() {
	p.registerStrategyEngine(bkStrategyEngine{platform: p})
}

func (p *Platform) resolveStrategyEngine(strategyVersionID string, parameters map[string]any) (StrategyEngine, string, error) {
	engineKey := normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	if engineKey == "bk-default" {
		if resolved := p.resolveStrategyEngineFromVersion(strategyVersionID); resolved != "" {
			engineKey = normalizeStrategyEngineKey(resolved)
		}
	}
	engine, ok := p.strategyEngines[engineKey]
	if !ok {
		return nil, engineKey, fmt.Errorf("strategy engine not registered: %s", engineKey)
	}
	return engine, engineKey, nil
}

func (p *Platform) resolveStrategyEngineFromVersion(strategyVersionID string) string {
	items, err := p.ListStrategies()
	if err != nil {
		return ""
	}
	for _, item := range items {
		switch currentVersion := item["currentVersion"].(type) {
		case domain.StrategyVersion:
			if currentVersion.ID == strategyVersionID {
				return stringValue(currentVersion.Parameters["strategyEngine"])
			}
		case map[string]any:
			if stringValue(currentVersion["id"]) != strategyVersionID {
				continue
			}
			if params, ok := currentVersion["parameters"].(map[string]any); ok {
				return stringValue(params["strategyEngine"])
			}
		}
	}
	return ""
}

type bkStrategyEngine struct {
	platform *Platform
}

func (e bkStrategyEngine) Key() string {
	return "bk-default"
}

func (e bkStrategyEngine) Describe() map[string]any {
	return map[string]any{
		"key":                  e.Key(),
		"name":                 "BK Default Strategy",
		"supportedSignalBars":  []string{"5m", "4h", "1d"},
		"supportedExecutions":  []string{"tick", "1min"},
		"runtimeConsistency":   "canonical-execution-shared",
		"backtestSlippageOnly": true,
	}
}

func (e bkStrategyEngine) Run(context StrategyExecutionContext) (map[string]any, error) {
	return e.platform.runStrategyReplay(context)
}

func (e bkStrategyEngine) EvaluateSignal(context StrategySignalEvaluationContext) (StrategySignalDecision, error) {
	trigger := cloneMetadata(context.TriggerSummary)
	sourceStates := cloneMetadata(context.SourceStates)
	signalBarStates := cloneMetadata(context.SignalBarStates)
	currentPosition := cloneMetadata(context.CurrentPosition)
	action := "advance-plan"
	reason := "trigger-source-ready"
	if stringValue(trigger["role"]) != "trigger" {
		action = "wait"
		reason = "non-trigger-event"
	}
	symbol := NormalizeSymbol(context.ExecutionContext.Symbol)
	triggerSymbol := NormalizeSymbol(firstNonEmpty(stringValue(trigger["subscriptionSymbol"]), stringValue(trigger["symbol"])))
	if action == "advance-plan" && symbol != "" && triggerSymbol != "" && triggerSymbol != symbol {
		action = "wait"
		reason = "symbol-mismatch"
	}
	if action == "advance-plan" && len(sourceStates) == 0 {
		action = "wait"
		reason = "missing-source-states"
	}
	signalBarState, signalBarStateKey := pickSignalBarState(signalBarStates, context.ExecutionContext.Symbol, context.ExecutionContext.SignalTimeframe)
	if action == "advance-plan" && signalBarState == nil {
		action = "wait"
		reason = "missing-signal-bars"
	}
	if action == "advance-plan" && !context.NextPlannedEvent.IsZero() && context.EventTime.Before(context.NextPlannedEvent) {
		action = "wait"
		reason = "planned-event-not-reached"
	}
	signalBarDecision := map[string]any{}
	signalFilterReady := true
	if signalBarState != nil {
		signalBarDecision = evaluateSignalBarGate(signalBarState, context.NextPlannedSide, context.NextPlannedRole, context.NextPlannedReason)
		if value, ok := signalBarDecision["ready"].(bool); ok {
			signalFilterReady = value
		}
	}
	if action == "advance-plan" && !signalFilterReady {
		action = "wait"
		reason = "signal-filter-not-ready"
	}
	marketPrice, marketSource := pickDecisionMarketPrice(trigger, sourceStates, context.NextPlannedSide)
	orderBookStats := extractOrderBookStats(trigger, sourceStates)
	maxDeviationBps := firstPositive(parseFloatValue(context.ExecutionContext.Parameters["signalDecisionMaxDeviationBps"]), 50)
	maxSpreadBps := firstPositive(parseFloatValue(context.ExecutionContext.Parameters["signalDecisionMaxSpreadBps"]), 8)
	effectivePlannedPrice := context.NextPlannedPrice
	livePositionState := map[string]any{}
	if signalBarState != nil {
		watermarks := refreshLivePositionWatermarks(context.SessionState, currentPosition, marketPrice)
		livePositionState = deriveLivePositionState(context.ExecutionContext.Parameters, currentPosition, signalBarState, marketPrice, watermarks)
		if strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") {
			livePositionState = deriveLiveExitState(context.ExecutionContext.Parameters, currentPosition, livePositionState, marketPrice, context.NextPlannedReason)
		}
		if len(livePositionState) > 0 {
			mergedPosition := cloneMetadata(currentPosition)
			for key, value := range livePositionState {
				mergedPosition[key] = value
			}
			currentPosition = mergedPosition
			signalBarDecision["livePositionState"] = cloneMetadata(livePositionState)
			if targetPrice := parseFloatValue(livePositionState["targetPrice"]); targetPrice > 0 && strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") {
				effectivePlannedPrice = targetPrice
			}
			if action == "advance-plan" && strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") && !boolValue(livePositionState["ready"]) {
				action = "wait"
				reason = firstNonEmpty(stringValue(livePositionState["waitReason"]), "exit-signal-not-ready")
			}
		}
	}
	deviationBps := 0.0
	positionPnLBps := computePositionPnLBps(currentPosition, marketPrice)
	if action == "advance-plan" && effectivePlannedPrice > 0 && marketPrice > 0 {
		deviationBps = math.Abs(marketPrice/effectivePlannedPrice-1) * 10000
		if !strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") &&
			!isPlannedPriceActionable(context.NextPlannedSide, effectivePlannedPrice, marketPrice, maxDeviationBps) {
			action = "wait"
			reason = "price-not-actionable"
		}
	}
	if action == "advance-plan" && !strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") && orderBookStats.spreadBps > 0 && orderBookStats.spreadBps > maxSpreadBps {
		action = "wait"
		reason = "spread-too-wide"
	}
	biasActionable := isLiquidityBiasActionable(context.NextPlannedSide, context.NextPlannedRole, context.NextPlannedReason, orderBookStats.bias)
	if action == "advance-plan" && !strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") && orderBookStats.bias != "" && !biasActionable {
		action = "wait"
		reason = "bias-unfavorable"
	}
	decisionState := classifyStrategyDecisionState(action, reason, context.NextPlannedRole)
	entryProximityBps := computePriceProximityBps(effectivePlannedPrice, marketPrice)
	exitProximityBps := entryProximityBps
	signalKind := classifyStrategySignalKind(action, reason, context.NextPlannedRole, context.NextPlannedReason, currentPosition, positionPnLBps, entryProximityBps, exitProximityBps, orderBookStats.bias)
	return StrategySignalDecision{
		Action: action,
		Reason: reason,
		Metadata: map[string]any{
			"decisionState":       decisionState,
			"signalKind":          signalKind,
			"trigger":             trigger,
			"sourceStateCount":    len(sourceStates),
			"signalBarStateCount": len(signalBarStates),
			"currentPosition":     currentPosition,
			"symbol":              symbol,
			"triggerSymbol":       triggerSymbol,
			"signalBarStateKey":   signalBarStateKey,
			"signalBarState":      cloneMetadata(signalBarState),
			"signalBarDecision":   signalBarDecision,
			"livePositionState":   cloneMetadata(livePositionState),
			"nextPlannedEvent":    formatOptionalRFC3339(context.NextPlannedEvent),
			"nextPlannedPrice":    effectivePlannedPrice,
			"nextPlannedSide":     context.NextPlannedSide,
			"nextPlannedRole":     context.NextPlannedRole,
			"nextPlannedReason":   context.NextPlannedReason,
			"marketPrice":         marketPrice,
			"marketSource":        marketSource,
			"bestBid":             orderBookStats.bestBid,
			"bestAsk":             orderBookStats.bestAsk,
			"bestBidQty":          orderBookStats.bestBidQty,
			"bestAskQty":          orderBookStats.bestAskQty,
			"spreadBps":           orderBookStats.spreadBps,
			"bookImbalance":       orderBookStats.imbalance,
			"liquidityBias":       orderBookStats.bias,
			"biasActionable":      biasActionable,
			"positionPnLBps":      positionPnLBps,
			"entryProximityBps":   entryProximityBps,
			"exitProximityBps":    exitProximityBps,
			"maxDeviationBps":     maxDeviationBps,
			"maxSpreadBps":        maxSpreadBps,
			"deviationBps":        deviationBps,
			"priceActionable":     strings.EqualFold(strings.TrimSpace(context.NextPlannedRole), "exit") || isPlannedPriceActionable(context.NextPlannedSide, effectivePlannedPrice, marketPrice, maxDeviationBps),
		},
	}, nil
}

func formatOptionalRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func pickDecisionMarketPrice(trigger map[string]any, sourceStates map[string]any, side string) (float64, string) {
	normalizedSide := strings.ToUpper(strings.TrimSpace(side))
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(trigger["subscriptionSymbol"]), stringValue(trigger["symbol"])))

	bestBid, bestAsk := 0.0, 0.0
	tradePrice := parseFloatValue(trigger["price"])

	for _, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		if symbol != "" && NormalizeSymbol(stringValue(entry["symbol"])) != symbol {
			continue
		}
		streamType := strings.ToLower(strings.TrimSpace(stringValue(entry["streamType"])))
		summary := mapValue(entry["summary"])
		switch streamType {
		case "order_book":
			if bestBid <= 0 {
				bestBid = parseFloatValue(summary["bestBid"])
			}
			if bestAsk <= 0 {
				bestAsk = parseFloatValue(summary["bestAsk"])
			}
		case "trade_tick":
			if tradePrice <= 0 {
				tradePrice = parseFloatValue(summary["price"])
			}
		}
	}

	switch normalizedSide {
	case "BUY":
		if bestAsk > 0 {
			return bestAsk, "order_book.bestAsk"
		}
		if tradePrice > 0 {
			return tradePrice, "trade_tick.price"
		}
		if bestBid > 0 {
			return bestBid, "order_book.bestBid"
		}
	case "SELL", "SHORT":
		if bestBid > 0 {
			return bestBid, "order_book.bestBid"
		}
		if tradePrice > 0 {
			return tradePrice, "trade_tick.price"
		}
		if bestAsk > 0 {
			return bestAsk, "order_book.bestAsk"
		}
	}
	return tradePrice, "trigger.price"
}

func isPlannedPriceActionable(side string, plannedPrice, marketPrice, maxDeviationBps float64) bool {
	if plannedPrice <= 0 || marketPrice <= 0 {
		return false
	}
	tolerance := maxDeviationBps / 10000
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "BUY":
		return marketPrice <= plannedPrice*(1+tolerance)
	case "SELL", "SHORT":
		return marketPrice >= plannedPrice*(1-tolerance)
	default:
		return math.Abs(marketPrice/plannedPrice-1) <= tolerance
	}
}

func classifyStrategyDecisionState(action, reason, nextRole string) string {
	if action == "advance-plan" {
		switch strings.ToLower(strings.TrimSpace(nextRole)) {
		case "entry":
			return "entry-ready"
		case "exit":
			return "exit-ready"
		default:
			return "advance-ready"
		}
	}
	switch reason {
	case "non-trigger-event", "symbol-mismatch":
		return "ignore-event"
	case "missing-source-states":
		return "waiting-inputs"
	case "missing-signal-bars":
		return "waiting-signal-bars"
	case "planned-event-not-reached":
		return "waiting-time"
	case "signal-filter-not-ready":
		return "waiting-signal-filter"
	case "price-not-actionable":
		return "waiting-price"
	case "spread-too-wide":
		return "waiting-liquidity"
	case "bias-unfavorable":
		return "waiting-flow"
	default:
		return "waiting"
	}
}

func classifyStrategySignalKind(action, reason, nextRole, nextReason string, currentPosition map[string]any, positionPnLBps float64, entryProximityBps float64, exitProximityBps float64, liquidityBias string) string {
	positionSide := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	positionQty := parseFloatValue(currentPosition["quantity"])
	hasPosition := positionQty > 0 && positionSide != ""
	reasonTag := normalizeStrategyReasonTag(nextReason)
	nearEntry := entryProximityBps > 0 && entryProximityBps <= 10
	nearExit := exitProximityBps > 0 && exitProximityBps <= 10
	favorableBias := isFavorableBiasForPlan(nextRole, nextReason, liquidityBias)
	if action == "advance-plan" {
		switch strings.ToLower(strings.TrimSpace(nextRole)) {
		case "entry":
			switch reasonTag {
			case "initial":
				return "initial-entry"
			case "zero-initial-reentry":
				return "zero-initial-reentry"
			case "sl-reentry":
				return "sl-reentry"
			case "pt-reentry":
				return "pt-reentry"
			default:
				return "entry"
			}
		case "exit":
			switch reasonTag {
			case "pt":
				return "protect-exit"
			case "sl":
				return "risk-exit"
			default:
				if hasPosition {
					return "exit"
				}
				return "advance"
			}
		default:
			return "advance"
		}
	}
	switch strings.ToLower(strings.TrimSpace(nextRole)) {
	case "entry":
		switch reasonTag {
		case "initial":
			if nearEntry && reason == "bias-unfavorable" {
				return "initial-entry-near-weak"
			}
			if nearEntry {
				if favorableBias {
					return "initial-entry-near-strong"
				}
				return "initial-entry-near"
			}
			return "initial-entry-watch"
		case "zero-initial-reentry":
			if nearEntry && reason == "bias-unfavorable" {
				return "zero-initial-reentry-near-weak"
			}
			if nearEntry {
				if favorableBias {
					return "zero-initial-reentry-near-strong"
				}
				return "zero-initial-reentry-near"
			}
			return "zero-initial-reentry-watch"
		case "sl-reentry":
			if nearEntry && reason == "bias-unfavorable" {
				return "sl-reentry-near-weak"
			}
			if nearEntry {
				if favorableBias {
					return "sl-reentry-near-strong"
				}
				return "sl-reentry-near"
			}
			return "sl-reentry-watch"
		case "pt-reentry":
			if nearEntry && reason == "bias-unfavorable" {
				return "pt-reentry-near-weak"
			}
			if nearEntry {
				if favorableBias {
					return "pt-reentry-near-strong"
				}
				return "pt-reentry-near"
			}
			return "pt-reentry-watch"
		}
	case "exit":
		switch reasonTag {
		case "pt":
			if nearExit {
				if favorableBias {
					return "protect-exit-near-strong"
				}
				if liquidityBias != "" && liquidityBias != "balanced" {
					return "protect-exit-near-weak"
				}
				return "protect-exit-near"
			}
			if hasPosition && positionPnLBps > 0 {
				return "protect-exit-watch"
			}
			return "protect-exit-watch"
		case "sl":
			if nearExit {
				return "risk-exit-near"
			}
			if hasPosition && positionPnLBps <= 0 {
				return "risk-exit-watch"
			}
			return "risk-exit-watch"
		}
	}
	switch reason {
	case "planned-event-not-reached":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "price-not-actionable":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "spread-too-wide":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "bias-unfavorable":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "missing-source-states":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "missing-signal-bars":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "signal-filter-not-ready":
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	case "non-trigger-event", "symbol-mismatch":
		return "ignore"
	default:
		if hasPosition {
			return "hold-" + strings.ToLower(positionSide)
		}
		return "hold"
	}
}

func pickSignalBarState(signalBarStates map[string]any, symbol, timeframe string) (map[string]any, string) {
	normalizedSymbol := NormalizeSymbol(symbol)
	normalizedTimeframe := strings.ToLower(strings.TrimSpace(timeframe))
	for key, raw := range signalBarStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		if normalizedSymbol != "" && NormalizeSymbol(stringValue(entry["symbol"])) != normalizedSymbol {
			continue
		}
		if normalizedTimeframe != "" && strings.ToLower(strings.TrimSpace(stringValue(entry["timeframe"]))) != normalizedTimeframe {
			continue
		}
		return cloneMetadata(entry), key
	}
	return nil, ""
}

func evaluateSignalBarGate(signalBarState map[string]any, nextSide, nextRole, nextReason string) map[string]any {
	role := strings.ToLower(strings.TrimSpace(nextRole))
	reasonTag := normalizeStrategyReasonTag(nextReason)
	timeframe := strings.ToLower(strings.TrimSpace(stringValue(signalBarState["timeframe"])))
	if timeframe == "" {
		timeframe = strings.ToLower(strings.TrimSpace(stringValue(mapValue(signalBarState["current"])["timeframe"])))
	}
	result := map[string]any{
		"ready":     true,
		"reason":    "signal-bar-ready",
		"side":      strings.ToUpper(strings.TrimSpace(nextSide)),
		"role":      role,
		"timeframe": timeframe,
		"sma5":      parseFloatValue(signalBarState["sma5"]),
		"ma20":      parseFloatValue(signalBarState["ma20"]),
		"atr14":     parseFloatValue(signalBarState["atr14"]),
		"current":   cloneMetadata(mapValue(signalBarState["current"])),
		"prevBar1":  cloneMetadata(mapValue(signalBarState["prevBar1"])),
		"prevBar2":  cloneMetadata(mapValue(signalBarState["prevBar2"])),
	}
	current := mapValue(signalBarState["current"])
	prevBar1 := mapValue(signalBarState["prevBar1"])
	prevBar2 := mapValue(signalBarState["prevBar2"])
	ma20 := parseFloatValue(signalBarState["ma20"])
	sma5 := parseFloatValue(signalBarState["sma5"])
	atr14 := parseFloatValue(signalBarState["atr14"])
	if current == nil || prevBar1 == nil || prevBar2 == nil {
		result["ready"] = false
		result["reason"] = "insufficient-signal-bars"
		return result
	}
	closePrice := parseFloatValue(current["close"])
	highPrice := parseFloatValue(current["high"])
	lowPrice := parseFloatValue(current["low"])
	prevHigh1 := parseFloatValue(prevBar1["high"])
	prevHigh2 := parseFloatValue(prevBar2["high"])
	prevLow1 := parseFloatValue(prevBar1["low"])
	prevLow2 := parseFloatValue(prevBar2["low"])
	longStructureReady := false
	shortStructureReady := false
	longEarlyReversalReady := false
	shortEarlyReversalReady := false
	switch timeframe {
	case "1d":
		if atr14 <= 0 {
			result["ready"] = false
			result["reason"] = "insufficient-signal-bars"
			return result
		}
		if sma5 <= 0 {
			if ma20 <= 0 {
				result["ready"] = false
				result["reason"] = "insufficient-signal-bars"
				return result
			}
			result["usedLegacyMA20Fallback"] = true
			longStructureReady = closePrice > ma20
			shortStructureReady = closePrice < ma20
			break
		}
		earlyBand := 0.06 * atr14
		longStructureReady = closePrice > sma5
		shortStructureReady = closePrice < sma5
		longEarlyReversalReady = closePrice >= (sma5-earlyBand) && prevHigh2 > prevHigh1 && prevLow1 >= prevLow2
		shortEarlyReversalReady = closePrice <= (sma5+earlyBand) && prevLow2 < prevLow1 && prevHigh1 <= prevHigh2
		longStructureReady = longStructureReady || longEarlyReversalReady
		shortStructureReady = shortStructureReady || shortEarlyReversalReady
	default:
		if ma20 <= 0 {
			result["ready"] = false
			result["reason"] = "insufficient-signal-bars"
			return result
		}
		longStructureReady = closePrice > ma20
		shortStructureReady = closePrice < ma20
	}
	longBreakoutReady := highPrice >= prevHigh2 && prevHigh2 > 0
	shortBreakoutReady := lowPrice <= prevLow2 && prevLow2 > 0
	longReady := longStructureReady && longBreakoutReady
	shortReady := shortStructureReady && shortBreakoutReady
	if role == "entry" && (reasonTag == "zero-initial-reentry" || reasonTag == "sl-reentry" || reasonTag == "pt-reentry") {
		longReady = longStructureReady
		shortReady = shortStructureReady
	}
	result["longStructureReady"] = longStructureReady
	result["shortStructureReady"] = shortStructureReady
	result["longEarlyReversalReady"] = longEarlyReversalReady
	result["shortEarlyReversalReady"] = shortEarlyReversalReady
	result["longBreakoutReady"] = longBreakoutReady
	result["shortBreakoutReady"] = shortBreakoutReady
	result["longReady"] = longReady
	result["shortReady"] = shortReady
	if role == "exit" {
		return result
	}
	switch strings.ToUpper(strings.TrimSpace(nextSide)) {
	case "BUY":
		if !longReady {
			result["ready"] = false
			result["reason"] = "long-signal-not-ready"
		}
	case "SELL", "SHORT":
		if !shortReady {
			result["ready"] = false
			result["reason"] = "short-signal-not-ready"
		}
	}
	return result
}

func isFavorableBiasForPlan(nextRole, nextReason, liquidityBias string) bool {
	liquidityBias = strings.ToLower(strings.TrimSpace(liquidityBias))
	if liquidityBias == "" || liquidityBias == "balanced" {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(nextRole))
	reasonTag := normalizeStrategyReasonTag(nextReason)
	if role == "entry" {
		switch reasonTag {
		case "initial", "zero-initial-reentry", "sl-reentry", "pt-reentry":
			return liquidityBias == "bid-heavy"
		}
	}
	if role == "exit" && reasonTag == "pt" {
		return liquidityBias == "ask-heavy"
	}
	return false
}

func computePositionPnLBps(currentPosition map[string]any, marketPrice float64) float64 {
	side := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	qty := parseFloatValue(currentPosition["quantity"])
	entryPrice := parseFloatValue(currentPosition["entryPrice"])
	if qty <= 0 || entryPrice <= 0 || marketPrice <= 0 || side == "" {
		return 0
	}
	switch side {
	case "LONG":
		return (marketPrice/entryPrice - 1) * 10000
	case "SHORT":
		return (entryPrice/marketPrice - 1) * 10000
	default:
		return 0
	}
}

func computePriceProximityBps(plannedPrice, marketPrice float64) float64 {
	if plannedPrice <= 0 || marketPrice <= 0 {
		return 0
	}
	return math.Abs(marketPrice/plannedPrice-1) * 10000
}

type livePositionWatermarks struct {
	PositionKey string
	HWM         float64
	LWM         float64
}

func hasActiveVirtualPositionSnapshot(currentPosition map[string]any) bool {
	if !boolValue(currentPosition["virtual"]) {
		return false
	}
	if boolValue(currentPosition["trackingActive"]) {
		return true
	}
	if strings.TrimSpace(stringValue(currentPosition["id"])) == "" {
		return false
	}
	if NormalizeSymbol(stringValue(currentPosition["symbol"])) == "" {
		return false
	}
	if strings.TrimSpace(stringValue(currentPosition["side"])) == "" {
		return false
	}
	return parseFloatValue(currentPosition["entryPrice"]) > 0
}

func hasActiveLivePositionSnapshot(currentPosition map[string]any) bool {
	return boolValue(currentPosition["found"]) ||
		math.Abs(parseFloatValue(currentPosition["quantity"])) > 0 ||
		hasActiveVirtualPositionSnapshot(currentPosition)
}

func buildLivePositionWatermarkBaseKey(currentPosition map[string]any) string {
	entryPrice := parseFloatValue(currentPosition["entryPrice"])
	side := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	if entryPrice <= 0 || side == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if symbol := NormalizeSymbol(stringValue(currentPosition["symbol"])); symbol != "" {
		parts = append(parts, symbol)
	}
	parts = append(parts, side, fmt.Sprintf("%.8f", entryPrice))
	return strings.Join(parts, "|")
}

func buildLegacyLivePositionWatermarkKey(currentPosition map[string]any) string {
	entryPrice := parseFloatValue(currentPosition["entryPrice"])
	side := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	if entryPrice <= 0 || side == "" {
		return ""
	}
	return strings.Join([]string{side, fmt.Sprintf("%.8f", entryPrice)}, "|")
}

func encodeLivePositionWatermarkIdentityComponent(positionID string) string {
	normalized := strings.TrimSpace(positionID)
	if normalized == "" {
		return ""
	}
	return "id:" + base64.RawURLEncoding.EncodeToString([]byte(normalized))
}

func buildLegacyPrefixedLivePositionWatermarkKey(currentPosition map[string]any) string {
	positionID := strings.TrimSpace(stringValue(currentPosition["id"]))
	baseKey := buildLivePositionWatermarkBaseKey(currentPosition)
	if positionID == "" || baseKey == "" {
		return ""
	}
	return positionID + "|" + baseKey
}

func buildLivePositionWatermarkKey(currentPosition map[string]any) string {
	baseKey := buildLivePositionWatermarkBaseKey(currentPosition)
	if baseKey == "" {
		return ""
	}
	if positionID := strings.TrimSpace(stringValue(currentPosition["id"])); positionID != "" {
		if identityComponent := encodeLivePositionWatermarkIdentityComponent(positionID); identityComponent != "" {
			return identityComponent + "|" + baseKey
		}
	}
	return baseKey
}

func isCompatibleLivePositionWatermarkMigration(lastKey string, currentPosition map[string]any, positionKey string) bool {
	if lastKey == "" || positionKey == "" {
		return false
	}
	if lastKey == positionKey {
		return true
	}
	baseKey := buildLivePositionWatermarkBaseKey(currentPosition)
	if positionID := strings.TrimSpace(stringValue(currentPosition["id"])); positionID != "" {
		if boolValue(currentPosition["virtual"]) && lastKey == positionID {
			return true
		}
		if lastKey == baseKey {
			return true
		}
		return lastKey == buildLegacyPrefixedLivePositionWatermarkKey(currentPosition)
	}
	return lastKey == baseKey
}

func clearLivePositionWatermarks(sessionState map[string]any) {
	if sessionState == nil {
		return
	}
	delete(sessionState, "watermarkPositionKey")
	delete(sessionState, "hwm")
	delete(sessionState, "lwm")
}

func resolveLivePositionWatermarks(currentPosition map[string]any, sessionState map[string]any) livePositionWatermarks {
	if !hasActiveLivePositionSnapshot(currentPosition) {
		return livePositionWatermarks{}
	}
	entryPrice := parseFloatValue(currentPosition["entryPrice"])
	side := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	if entryPrice <= 0 || side == "" {
		return livePositionWatermarks{}
	}
	positionKey := buildLivePositionWatermarkKey(currentPosition)
	if positionKey == "" {
		return livePositionWatermarks{}
	}
	hwm := parseFloatValue(sessionState["hwm"])
	if hwm == 0 {
		hwm = entryPrice
	}
	lwm := parseFloatValue(sessionState["lwm"])
	if lwm == 0 {
		lwm = entryPrice
	}
	if lastKey := stringValue(sessionState["watermarkPositionKey"]); lastKey != positionKey {
		if isCompatibleLivePositionWatermarkMigration(lastKey, currentPosition, positionKey) {
			return livePositionWatermarks{
				PositionKey: positionKey,
				HWM:         hwm,
				LWM:         lwm,
			}
		}
		hwm = entryPrice
		lwm = entryPrice
	}
	return livePositionWatermarks{
		PositionKey: positionKey,
		HWM:         hwm,
		LWM:         lwm,
	}
}

func advanceLivePositionWatermarks(watermarks livePositionWatermarks, marketPrice float64) livePositionWatermarks {
	if marketPrice <= 0 {
		return watermarks
	}
	if watermarks.HWM == 0 {
		watermarks.HWM = marketPrice
	}
	if watermarks.LWM == 0 {
		watermarks.LWM = marketPrice
	}
	if marketPrice > watermarks.HWM {
		watermarks.HWM = marketPrice
	}
	if marketPrice < watermarks.LWM {
		watermarks.LWM = marketPrice
	}
	return watermarks
}

func applyLivePositionWatermarks(sessionState map[string]any, watermarks livePositionWatermarks) {
	if sessionState == nil || watermarks.PositionKey == "" {
		return
	}
	sessionState["watermarkPositionKey"] = watermarks.PositionKey
	sessionState["hwm"] = watermarks.HWM
	sessionState["lwm"] = watermarks.LWM
}

func refreshLivePositionWatermarks(sessionState map[string]any, currentPosition map[string]any, marketPrice float64) livePositionWatermarks {
	if !hasActiveLivePositionSnapshot(currentPosition) {
		clearLivePositionWatermarks(sessionState)
		return livePositionWatermarks{}
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	watermarks = advanceLivePositionWatermarks(watermarks, marketPrice)
	applyLivePositionWatermarks(sessionState, watermarks)
	return watermarks
}

// evaluateLivePositionState derives the current live position risk state.
// When sessionState is provided, it also refreshes HWM/LWM watermarks used by
// trailing-stop logic so callers do not need to duplicate watermark handling.
func evaluateLivePositionState(parameters map[string]any, currentPosition map[string]any, signalBarState map[string]any, marketPrice float64, sessionState map[string]any) map[string]any {
	watermarks := refreshLivePositionWatermarks(sessionState, currentPosition, marketPrice)
	return deriveLivePositionState(parameters, currentPosition, signalBarState, marketPrice, watermarks)
}

func deriveLivePositionState(parameters map[string]any, currentPosition map[string]any, signalBarState map[string]any, marketPrice float64, watermarks livePositionWatermarks) map[string]any {
	if !boolValue(currentPosition["found"]) && parseFloatValue(currentPosition["quantity"]) <= 0 && !boolValue(currentPosition["virtual"]) {
		return nil
	}
	current := mapValue(signalBarState["current"])
	prevBar1 := mapValue(signalBarState["prevBar1"])
	prevBar2 := mapValue(signalBarState["prevBar2"])
	if current == nil || prevBar1 == nil || prevBar2 == nil {
		return nil
	}
	entryPrice := parseFloatValue(currentPosition["entryPrice"])
	if entryPrice <= 0 {
		return nil
	}
	side := strings.ToLower(strings.TrimSpace(stringValue(currentPosition["side"])))
	if side == "" {
		return nil
	}
	sig := strategySignalBar{
		ATR:       parseFloatValue(signalBarState["atr14"]),
		PrevHigh1: parseFloatValue(prevBar1["high"]),
		PrevLow1:  parseFloatValue(prevBar1["low"]),
		PrevHigh2: parseFloatValue(prevBar2["high"]),
		PrevLow2:  parseFloatValue(prevBar2["low"]),
	}
	stopMode := firstNonEmpty(stringValue(parameters["stop_mode"]), "atr")
	stopLossATR := parseFloatValue(parameters["stop_loss_atr"])
	if stopLossATR <= 0 {
		stopLossATR = 0.05
	}
	stopLoss := parseFloatValue(currentPosition["stopLoss"])
	if stopLoss <= 0 {
		stopLoss = resolveStopPrice(side, entryPrice, sig, stopMode, stopLossATR)
	}
	hwm := firstPositive(watermarks.HWM, entryPrice)
	lwm := firstPositive(watermarks.LWM, entryPrice)

	// Calculate Trailing Stop Loss
	if trailingStopATR := parseFloatValue(parameters["trailing_stop_atr"]); trailingStopATR > 0 {
		isActive := true
		if delayedActivation := parseFloatValue(parameters["delayed_trailing_activation_atr"]); delayedActivation > 0 {
			profitATR := 0.0
			if sig.ATR > 0 {
				if side == "long" {
					profitATR = (marketPrice - entryPrice) / sig.ATR
				} else if side == "short" {
					profitATR = (entryPrice - marketPrice) / sig.ATR
				}
			}
			if profitATR < delayedActivation {
				isActive = false
			}
		}

		if isActive && sig.ATR > 0 {
			if side == "long" {
				trailingSL := hwm - trailingStopATR*sig.ATR
				if trailingSL > stopLoss {
					stopLoss = trailingSL
				}
			} else if side == "short" {
				trailingSL := lwm + trailingStopATR*sig.ATR
				if trailingSL < stopLoss {
					stopLoss = trailingSL
				}
			}
		}
	}

	protected := boolValue(currentPosition["protected"])
	profitProtectATR := firstPositive(parseFloatValue(parameters["profit_protect_atr"]), 1.0)
	protectionPrice := 0.0
	if side == "long" {
		protectionPrice = entryPrice + profitProtectATR*sig.ATR
		if !protected && marketPrice > 0 && marketPrice >= protectionPrice {
			protected = true
		}
	} else if side == "short" {
		protectionPrice = entryPrice - profitProtectATR*sig.ATR
		if !protected && marketPrice > 0 && marketPrice <= protectionPrice {
			protected = true
		}
	}
	return map[string]any{
		"found":                true,
		"symbol":               NormalizeSymbol(stringValue(currentPosition["symbol"])),
		"side":                 strings.ToUpper(side),
		"entryPrice":           entryPrice,
		"stopLoss":             stopLoss,
		"protected":            protected,
		"protectionTrigger":    protectionPrice,
		"prevHigh1":            sig.PrevHigh1,
		"prevLow1":             sig.PrevLow1,
		"atr14":                sig.ATR,
		"profitProtectATR":     profitProtectATR,
		"hwm":                  hwm,
		"lwm":                  lwm,
		"watermarkPositionKey": watermarks.PositionKey,
	}
}

func updateLivePositionWatermarks(sessionState map[string]any, currentPosition map[string]any, marketPrice float64) (float64, float64) {
	watermarks := refreshLivePositionWatermarks(sessionState, currentPosition, marketPrice)
	return watermarks.HWM, watermarks.LWM
}

func evaluateLiveExitState(parameters map[string]any, currentPosition map[string]any, signalBarState map[string]any, marketPrice float64, sessionState map[string]any, nextReason string) map[string]any {
	watermarks := refreshLivePositionWatermarks(sessionState, currentPosition, marketPrice)
	positionState := deriveLivePositionState(parameters, currentPosition, signalBarState, marketPrice, watermarks)
	return deriveLiveExitState(parameters, currentPosition, positionState, marketPrice, nextReason)
}

func deriveLiveExitState(parameters map[string]any, currentPosition map[string]any, positionState map[string]any, marketPrice float64, nextReason string) map[string]any {
	if len(positionState) == 0 {
		return map[string]any{
			"ready":      false,
			"waitReason": "position-unavailable",
		}
	}
	side := strings.ToLower(strings.TrimSpace(stringValue(currentPosition["side"])))
	stopLoss := parseFloatValue(positionState["stopLoss"])
	protected := boolValue(positionState["protected"])
	prevHigh1 := parseFloatValue(positionState["prevHigh1"])
	prevLow1 := parseFloatValue(positionState["prevLow1"])
	reasonTag := normalizeStrategyReasonTag(nextReason)
	state := cloneMetadata(positionState)
	state["ready"] = false
	switch side {
	case "long":
		switch reasonTag {
		case "sl":
			state["targetPrice"] = stopLoss
			if marketPrice > 0 && stopLoss > 0 && marketPrice <= stopLoss {
				state["ready"] = true
			} else {
				state["waitReason"] = "sl-not-triggered"
			}
		case "pt":
			state["targetPrice"] = prevLow1
			if !protected {
				state["waitReason"] = "profit-protection-not-armed"
			} else if marketPrice > 0 && prevLow1 > 0 && marketPrice <= prevLow1 {
				state["ready"] = true
			} else {
				state["waitReason"] = "pt-not-triggered"
			}
		}
	case "short":
		switch reasonTag {
		case "sl":
			state["targetPrice"] = stopLoss
			if marketPrice > 0 && stopLoss > 0 && marketPrice >= stopLoss {
				state["ready"] = true
			} else {
				state["waitReason"] = "sl-not-triggered"
			}
		case "pt":
			state["targetPrice"] = prevHigh1
			if !protected {
				state["waitReason"] = "profit-protection-not-armed"
			} else if marketPrice > 0 && prevHigh1 > 0 && marketPrice >= prevHigh1 {
				state["ready"] = true
			} else {
				state["waitReason"] = "pt-not-triggered"
			}
		}
	}
	if strings.TrimSpace(stringValue(state["waitReason"])) == "" && !boolValue(state["ready"]) {
		state["waitReason"] = "exit-signal-not-ready"
	}
	return state
}

type orderBookDecisionStats struct {
	bestBid    float64
	bestAsk    float64
	bestBidQty float64
	bestAskQty float64
	spreadBps  float64
	imbalance  float64
	bias       string
}

func extractOrderBookStats(trigger map[string]any, sourceStates map[string]any) orderBookDecisionStats {
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(trigger["subscriptionSymbol"]), stringValue(trigger["symbol"])))
	stats := orderBookDecisionStats{}
	for _, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringValue(entry["streamType"]))) != "order_book" {
			continue
		}
		if symbol != "" && NormalizeSymbol(stringValue(entry["symbol"])) != symbol {
			continue
		}
		summary := mapValue(entry["summary"])
		stats.bestBid = parseFloatValue(summary["bestBid"])
		stats.bestAsk = parseFloatValue(summary["bestAsk"])
		bidQty := parseFloatValue(summary["bestBidQty"])
		askQty := parseFloatValue(summary["bestAskQty"])
		stats.bestBidQty = bidQty
		stats.bestAskQty = askQty
		if stats.bestBid > 0 && stats.bestAsk > 0 {
			mid := (stats.bestBid + stats.bestAsk) / 2
			if mid > 0 {
				stats.spreadBps = (stats.bestAsk - stats.bestBid) / mid * 10000
			}
		}
		totalQty := bidQty + askQty
		if totalQty > 0 {
			stats.imbalance = (bidQty - askQty) / totalQty
		}
		switch {
		case stats.imbalance > 0.15:
			stats.bias = "bid-heavy"
		case stats.imbalance < -0.15:
			stats.bias = "ask-heavy"
		default:
			stats.bias = "balanced"
		}
		return stats
	}
	return stats
}

func isLiquidityBiasActionable(nextSide, nextRole, nextReason, bias string) bool {
	role := strings.ToLower(strings.TrimSpace(nextRole))
	reasonTag := normalizeStrategyReasonTag(nextReason)
	side := strings.ToUpper(strings.TrimSpace(nextSide))
	bias = strings.ToLower(strings.TrimSpace(bias))
	if bias == "" || bias == "balanced" {
		if role == "entry" && reasonTag == "sl-reentry" {
			return false
		}
		return true
	}
	if role == "exit" && reasonTag == "sl" {
		return true
	}
	if role == "entry" && reasonTag == "sl-reentry" {
		switch side {
		case "BUY":
			return bias == "bid-heavy"
		case "SELL", "SHORT":
			return bias == "ask-heavy"
		default:
			return false
		}
	}
	switch side {
	case "BUY":
		return bias != "ask-heavy"
	case "SELL", "SHORT":
		return bias != "bid-heavy"
	default:
		return true
	}
}

func normalizeStrategyReasonTag(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	return normalized
}
