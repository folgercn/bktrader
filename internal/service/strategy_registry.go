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
	maxDeviationBps := firstPositive(parseFloatValue(context.ExecutionContext.Parameters["signalDecisionMaxDeviationBps"]), 50)
	deviationBps := 0.0
	if action == "advance-plan" && context.NextPlannedPrice > 0 && marketPrice > 0 {
		deviationBps = math.Abs(marketPrice/context.NextPlannedPrice-1) * 10000
		if !isPlannedPriceActionable(context.NextPlannedSide, context.NextPlannedPrice, marketPrice, maxDeviationBps) {
			action = "wait"
			reason = "price-not-actionable"
		}
	}
	decisionState := classifyStrategyDecisionState(action, reason, context.NextPlannedRole)
	signalKind := classifyStrategySignalKind(action, reason, context.NextPlannedRole, context.NextPlannedReason)
	return StrategySignalDecision{
		Action: action,
		Reason: reason,
		Metadata: map[string]any{
			"decisionState":     decisionState,
			"signalKind":        signalKind,
			"trigger":           trigger,
			"sourceStateCount":  len(sourceStates),
			"symbol":            symbol,
			"triggerSymbol":     triggerSymbol,
			"nextPlannedEvent":  formatOptionalRFC3339(context.NextPlannedEvent),
			"nextPlannedPrice":  context.NextPlannedPrice,
			"nextPlannedSide":   context.NextPlannedSide,
			"nextPlannedRole":   context.NextPlannedRole,
			"nextPlannedReason": context.NextPlannedReason,
			"marketPrice":       marketPrice,
			"marketSource":      marketSource,
			"maxDeviationBps":   maxDeviationBps,
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
	default:
		return "waiting"
	}
}

func classifyStrategySignalKind(action, reason, nextRole, nextReason string) string {
	if action == "advance-plan" {
		switch strings.ToLower(strings.TrimSpace(nextRole)) {
		case "entry":
			return "entry"
		case "exit":
			reasonKey := strings.ToUpper(strings.TrimSpace(nextReason))
			switch reasonKey {
			case "PT":
				return "protect-exit"
			case "SL":
				return "risk-exit"
			default:
				return "exit"
			}
		default:
			return "advance"
		}
	}
	switch reason {
	case "planned-event-not-reached":
		return "hold"
	case "price-not-actionable":
		return "hold"
	case "missing-source-states":
		return "hold"
	case "non-trigger-event", "symbol-mismatch":
		return "ignore"
	default:
		return "hold"
	}
}
