package service

import (
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
	CurrentPosition   map[string]any
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
		"supportedSignalBars":  []string{"4h", "1d"},
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
	if action == "advance-plan" && !context.NextPlannedEvent.IsZero() && context.EventTime.Before(context.NextPlannedEvent) {
		action = "wait"
		reason = "planned-event-not-reached"
	}
	marketPrice, marketSource := pickDecisionMarketPrice(trigger, sourceStates, context.NextPlannedSide)
	orderBookStats := extractOrderBookStats(trigger, sourceStates)
	maxDeviationBps := firstPositive(parseFloatValue(context.ExecutionContext.Parameters["signalDecisionMaxDeviationBps"]), 50)
	maxSpreadBps := firstPositive(parseFloatValue(context.ExecutionContext.Parameters["signalDecisionMaxSpreadBps"]), 8)
	deviationBps := 0.0
	positionPnLBps := computePositionPnLBps(currentPosition, marketPrice)
	if action == "advance-plan" && context.NextPlannedPrice > 0 && marketPrice > 0 {
		deviationBps = math.Abs(marketPrice/context.NextPlannedPrice-1) * 10000
		if !isPlannedPriceActionable(context.NextPlannedSide, context.NextPlannedPrice, marketPrice, maxDeviationBps) {
			action = "wait"
			reason = "price-not-actionable"
		}
	}
	if action == "advance-plan" && orderBookStats.spreadBps > 0 && orderBookStats.spreadBps > maxSpreadBps {
		action = "wait"
		reason = "spread-too-wide"
	}
	decisionState := classifyStrategyDecisionState(action, reason, context.NextPlannedRole)
	entryProximityBps := computePriceProximityBps(context.NextPlannedPrice, marketPrice)
	exitProximityBps := entryProximityBps
	signalKind := classifyStrategySignalKind(action, reason, context.NextPlannedRole, context.NextPlannedReason, currentPosition, positionPnLBps, entryProximityBps, exitProximityBps)
	return StrategySignalDecision{
		Action: action,
		Reason: reason,
		Metadata: map[string]any{
			"decisionState":     decisionState,
			"signalKind":        signalKind,
			"trigger":           trigger,
			"sourceStateCount":  len(sourceStates),
			"currentPosition":   currentPosition,
			"symbol":            symbol,
			"triggerSymbol":     triggerSymbol,
			"nextPlannedEvent":  formatOptionalRFC3339(context.NextPlannedEvent),
			"nextPlannedPrice":  context.NextPlannedPrice,
			"nextPlannedSide":   context.NextPlannedSide,
			"nextPlannedRole":   context.NextPlannedRole,
			"nextPlannedReason": context.NextPlannedReason,
			"marketPrice":       marketPrice,
			"marketSource":      marketSource,
			"bestBid":           orderBookStats.bestBid,
			"bestAsk":           orderBookStats.bestAsk,
			"spreadBps":         orderBookStats.spreadBps,
			"bookImbalance":     orderBookStats.imbalance,
			"liquidityBias":     orderBookStats.bias,
			"positionPnLBps":    positionPnLBps,
			"entryProximityBps": entryProximityBps,
			"exitProximityBps":  exitProximityBps,
			"maxDeviationBps":   maxDeviationBps,
			"maxSpreadBps":      maxSpreadBps,
			"deviationBps":      deviationBps,
			"priceActionable":   isPlannedPriceActionable(context.NextPlannedSide, context.NextPlannedPrice, marketPrice, maxDeviationBps),
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
	case "planned-event-not-reached":
		return "waiting-time"
	case "price-not-actionable":
		return "waiting-price"
	case "spread-too-wide":
		return "waiting-liquidity"
	default:
		return "waiting"
	}
}

func classifyStrategySignalKind(action, reason, nextRole, nextReason string, currentPosition map[string]any, positionPnLBps float64, entryProximityBps float64, exitProximityBps float64) string {
	positionSide := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	positionQty := parseFloatValue(currentPosition["quantity"])
	hasPosition := positionQty > 0 && positionSide != ""
	reasonTag := normalizeStrategyReasonTag(nextReason)
	nearEntry := entryProximityBps > 0 && entryProximityBps <= 10
	nearExit := exitProximityBps > 0 && exitProximityBps <= 10
	if action == "advance-plan" {
		switch strings.ToLower(strings.TrimSpace(nextRole)) {
		case "entry":
			switch reasonTag {
			case "initial":
				return "initial-entry"
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
			if nearEntry {
				return "initial-entry-near"
			}
			return "initial-entry-watch"
		case "sl-reentry":
			if nearEntry {
				return "sl-reentry-near"
			}
			return "sl-reentry-watch"
		case "pt-reentry":
			if nearEntry {
				return "pt-reentry-near"
			}
			return "pt-reentry-watch"
		}
	case "exit":
		switch reasonTag {
		case "pt":
			if nearExit {
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
	case "missing-source-states":
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

type orderBookDecisionStats struct {
	bestBid   float64
	bestAsk   float64
	spreadBps float64
	imbalance float64
	bias      string
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

func normalizeStrategyReasonTag(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	return normalized
}
