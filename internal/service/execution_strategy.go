package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type SignalIntent struct {
	Action         string         `json:"action"`
	Role           string         `json:"role"`
	Reason         string         `json:"reason"`
	Side           string         `json:"side"`
	Symbol         string         `json:"symbol"`
	SignalKind     string         `json:"signalKind"`
	DecisionState  string         `json:"decisionState"`
	PlannedEventAt string         `json:"plannedEventAt"`
	PlannedPrice   float64        `json:"plannedPrice"`
	PriceHint      float64        `json:"priceHint"`
	PriceSource    string         `json:"priceSource"`
	Quantity       float64        `json:"quantity"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ExecutionPlanningContext struct {
	Session        domain.LiveSession
	Execution      StrategyExecutionContext
	TriggerSummary map[string]any
	SourceStates   map[string]any
	EventTime      time.Time
	Intent         SignalIntent
}

type ExecutionProposal struct {
	Action            string         `json:"action"`
	Role              string         `json:"role"`
	Reason            string         `json:"reason"`
	Side              string         `json:"side"`
	Symbol            string         `json:"symbol"`
	Type              string         `json:"type"`
	Quantity          float64        `json:"quantity"`
	LimitPrice        float64        `json:"limitPrice"`
	PriceHint         float64        `json:"priceHint"`
	PriceSource       string         `json:"priceSource"`
	TimeInForce       string         `json:"timeInForce"`
	PostOnly          bool           `json:"postOnly"`
	ReduceOnly        bool           `json:"reduceOnly"`
	SignalKind        string         `json:"signalKind"`
	DecisionState     string         `json:"decisionState"`
	SignalBarStateKey string         `json:"signalBarStateKey"`
	SpreadBps         float64        `json:"spreadBps"`
	BestBid           float64        `json:"bestBid"`
	BestAsk           float64        `json:"bestAsk"`
	ExecutionStrategy string         `json:"executionStrategy"`
	Status            string         `json:"status"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type ExecutionStrategy interface {
	Key() string
	Describe() map[string]any
	BuildProposal(ctx ExecutionPlanningContext) (ExecutionProposal, error)
}

func normalizeExecutionStrategyKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "book-aware-v1"
	}
	return value
}

func (p *Platform) registerExecutionStrategy(strategy ExecutionStrategy) {
	if strategy == nil {
		return
	}
	if p.executionStrategies == nil {
		p.executionStrategies = make(map[string]ExecutionStrategy)
	}
	p.executionStrategies[normalizeExecutionStrategyKey(strategy.Key())] = strategy
}

func (p *Platform) registerBuiltInExecutionStrategies() {
	p.registerExecutionStrategy(bookAwareExecutionStrategy{})
}

func (p *Platform) resolveExecutionStrategy(parameters map[string]any) (ExecutionStrategy, string, error) {
	key := normalizeExecutionStrategyKey(stringValue(parameters["executionStrategy"]))
	strategy, ok := p.executionStrategies[key]
	if !ok {
		return nil, key, fmt.Errorf("execution strategy not registered: %s", key)
	}
	return strategy, key, nil
}

type bookAwareExecutionStrategy struct{}

type executionProfile struct {
	OrderType             string
	TimeInForce           string
	PostOnly              bool
	MaxSpreadBps          float64
	WideSpreadMode        string
	RestingTimeoutSeconds int
	TimeoutFallbackType   string
	TimeoutFallbackTIF    string
	ReduceOnly            bool
}

func (bookAwareExecutionStrategy) Key() string {
	return "book-aware-v1"
}

func (bookAwareExecutionStrategy) Describe() map[string]any {
	return map[string]any{
		"key":  "book-aware-v1",
		"name": "Book Aware Execution",
	}
}

func (bookAwareExecutionStrategy) BuildProposal(ctx ExecutionPlanningContext) (ExecutionProposal, error) {
	intent := ctx.Intent
	meta := cloneMetadata(intent.Metadata)
	bestBid := parseFloatValue(meta["bestBid"])
	bestAsk := parseFloatValue(meta["bestAsk"])
	spreadBps := parseFloatValue(meta["spreadBps"])
	quantity := firstPositive(parseFloatValue(ctx.Session.State["defaultOrderQuantity"]), firstPositive(intent.Quantity, 0.001))
	priceHint := intent.PriceHint
	priceSource := intent.PriceSource
	signalSignature := buildExecutionSignalSignature(intent)
	profile := resolveExecutionProfile(ctx.Execution.Parameters, intent)
	maxSpreadBps := firstPositive(profile.MaxSpreadBps, firstPositive(parseFloatValue(ctx.Execution.Parameters["signalDecisionMaxSpreadBps"]), 8))
	wideSpreadMode := strings.ToLower(strings.TrimSpace(profile.WideSpreadMode))
	timeoutFallbackType := strings.ToUpper(strings.TrimSpace(profile.TimeoutFallbackType))
	timedOutSignature := stringValue(ctx.Session.State["lastExecutionTimeoutIntentSignature"])
	useTimeoutFallback := timeoutFallbackType != "" && signalSignature != "" && signalSignature == timedOutSignature

	switch strings.ToUpper(strings.TrimSpace(intent.Side)) {
	case "BUY":
		if bestAsk > 0 {
			priceHint = bestAsk
			priceSource = "order_book.bestAsk"
		}
	case "SELL", "SHORT":
		if bestBid > 0 {
			priceHint = bestBid
			priceSource = "order_book.bestBid"
		}
	}

	proposal := ExecutionProposal{
		Action:            intent.Action,
		Role:              intent.Role,
		Reason:            intent.Reason,
		Side:              intent.Side,
		Symbol:            intent.Symbol,
		Type:              strings.ToUpper(strings.TrimSpace(firstNonEmpty(profile.OrderType, "MARKET"))),
		Quantity:          quantity,
		PriceHint:         priceHint,
		PriceSource:       priceSource,
		ReduceOnly:        profile.ReduceOnly,
		SignalKind:        intent.SignalKind,
		DecisionState:     intent.DecisionState,
		SignalBarStateKey: stringValue(meta["signalBarStateKey"]),
		SpreadBps:         spreadBps,
		BestBid:           bestBid,
		BestAsk:           bestAsk,
		ExecutionStrategy: "book-aware-v1",
		Status:            "dispatchable",
		Metadata:          meta,
	}
	proposal.Metadata["executionProfile"] = describeExecutionProfile(intent)
	proposal.Metadata["executionProfileOrderType"] = profile.OrderType
	proposal.Metadata["executionProfileTimeInForce"] = profile.TimeInForce
	proposal.Metadata["executionProfilePostOnly"] = profile.PostOnly
	proposal.Metadata["executionProfileReduceOnly"] = profile.ReduceOnly
	proposal.Metadata["executionProfileWideSpreadMode"] = profile.WideSpreadMode
	proposal.Metadata["executionStrategy"] = proposal.ExecutionStrategy
	proposal.Metadata["signalSignature"] = signalSignature
	if proposal.Type == "LIMIT" {
		proposal.LimitPrice = priceHint
		proposal.TimeInForce = strings.ToUpper(strings.TrimSpace(firstNonEmpty(profile.TimeInForce, "GTC")))
		proposal.PostOnly = profile.PostOnly
		if proposal.PostOnly {
			proposal.TimeInForce = "GTX"
		}
	}
	if proposal.Quantity <= 0 {
		proposal.Status = "blocked"
		proposal.Reason = "invalid-quantity"
		return proposal, nil
	}
	if proposal.PriceHint <= 0 {
		proposal.Status = "wait"
		proposal.Reason = "market-price-unavailable"
		return proposal, nil
	}
	if spreadBps > 0 && spreadBps > maxSpreadBps {
		if useTimeoutFallback {
			proposal.Type = timeoutFallbackType
			proposal.TimeInForce = ""
			proposal.PostOnly = false
			proposal.LimitPrice = 0
			proposal.Reason = firstNonEmpty(intent.Reason, "execution-timeout-fallback")
			proposal.Metadata["fallbackFromTimeout"] = true
			proposal.Metadata["fallbackOrderType"] = timeoutFallbackType
			if proposal.Type == "LIMIT" {
				proposal.LimitPrice = resolvePassiveBookPrice(intent.Side, bestBid, bestAsk, priceHint)
				proposal.TimeInForce = strings.ToUpper(strings.TrimSpace(firstNonEmpty(profile.TimeoutFallbackTIF, "GTC")))
			}
			return proposal, nil
		}
		if wideSpreadMode == "limit-maker" {
			proposal.Type = "LIMIT"
			proposal.LimitPrice = resolvePassiveBookPrice(intent.Side, bestBid, bestAsk, priceHint)
			proposal.TimeInForce = "GTX"
			proposal.PostOnly = true
			proposal.PriceHint = proposal.LimitPrice
			proposal.PriceSource = "order_book.passive"
			proposal.Metadata["executionMode"] = "maker-resting"
			if restingTimeout := profile.RestingTimeoutSeconds; restingTimeout > 0 {
				proposal.Metadata["executionExpiresAt"] = ctx.EventTime.UTC().Add(time.Duration(restingTimeout) * time.Second).Format(time.RFC3339)
				proposal.Metadata["executionRestingTimeoutSeconds"] = restingTimeout
			}
			return proposal, nil
		}
		proposal.Status = "wait"
		proposal.Reason = "spread-too-wide"
		return proposal, nil
	}
	return proposal, nil
}

func resolveExecutionProfile(parameters map[string]any, intent SignalIntent) executionProfile {
	profile := executionProfile{
		OrderType:             strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(parameters["executionOrderType"]), "MARKET"))),
		TimeInForce:           strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(parameters["executionTimeInForce"]), "GTC"))),
		PostOnly:              boolValue(parameters["executionPostOnly"]),
		MaxSpreadBps:          parseFloatValue(parameters["executionMaxSpreadBps"]),
		WideSpreadMode:        strings.ToLower(strings.TrimSpace(stringValue(parameters["executionWideSpreadMode"]))),
		RestingTimeoutSeconds: maxIntValue(parameters["executionRestingTimeoutSeconds"], 0),
		TimeoutFallbackType:   strings.ToUpper(strings.TrimSpace(stringValue(parameters["executionTimeoutFallbackOrderType"]))),
		TimeoutFallbackTIF:    strings.ToUpper(strings.TrimSpace(stringValue(parameters["executionTimeoutFallbackTimeInForce"]))),
	}
	reasonTag := normalizeStrategyReasonTag(intent.Reason)
	role := strings.ToLower(strings.TrimSpace(intent.Role))
	switch {
	case role == "exit" && reasonTag == "sl":
		overrideExecutionProfile(&profile, parameters, "executionSLExit")
		profile.ReduceOnly = true
		profile.OrderType = strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(parameters["executionSLExitOrderType"]), "MARKET")))
		profile.TimeInForce = strings.ToUpper(strings.TrimSpace(stringValue(parameters["executionSLExitTimeInForce"])))
		profile.PostOnly = boolValue(parameters["executionSLExitPostOnly"])
		if profile.MaxSpreadBps <= 0 {
			profile.MaxSpreadBps = 100000
		}
		profile.WideSpreadMode = ""
		if profile.TimeoutFallbackType == "" {
			profile.TimeoutFallbackType = "MARKET"
		}
	case role == "exit" && reasonTag == "pt":
		overrideExecutionProfile(&profile, parameters, "executionPTExit")
		profile.ReduceOnly = true
		if profile.OrderType == "" {
			profile.OrderType = "LIMIT"
		}
		if profile.TimeInForce == "" {
			profile.TimeInForce = "GTX"
		}
		if !profile.PostOnly {
			profile.PostOnly = true
		}
		if profile.WideSpreadMode == "" {
			profile.WideSpreadMode = "limit-maker"
		}
		if profile.TimeoutFallbackType == "" {
			profile.TimeoutFallbackType = "MARKET"
		}
	default:
		overrideExecutionProfile(&profile, parameters, "executionEntry")
	}
	if profile.OrderType == "" {
		profile.OrderType = "MARKET"
	}
	return profile
}

func overrideExecutionProfile(profile *executionProfile, parameters map[string]any, prefix string) {
	if profile == nil {
		return
	}
	if value := strings.ToUpper(strings.TrimSpace(stringValue(parameters[prefix+"OrderType"]))); value != "" {
		profile.OrderType = value
	}
	if value := strings.ToUpper(strings.TrimSpace(stringValue(parameters[prefix+"TimeInForce"]))); value != "" {
		profile.TimeInForce = value
	}
	if _, ok := parameters[prefix+"PostOnly"]; ok {
		profile.PostOnly = boolValue(parameters[prefix+"PostOnly"])
	}
	if value := parseFloatValue(parameters[prefix+"MaxSpreadBps"]); value > 0 {
		profile.MaxSpreadBps = value
	}
	if value := strings.ToLower(strings.TrimSpace(stringValue(parameters[prefix+"WideSpreadMode"]))); value != "" {
		profile.WideSpreadMode = value
	}
	if value := maxIntValue(parameters[prefix+"RestingTimeoutSeconds"], 0); value > 0 {
		profile.RestingTimeoutSeconds = value
	}
	if value := strings.ToUpper(strings.TrimSpace(stringValue(parameters[prefix+"TimeoutFallbackOrderType"]))); value != "" {
		profile.TimeoutFallbackType = value
	}
	if value := strings.ToUpper(strings.TrimSpace(stringValue(parameters[prefix+"TimeoutFallbackTimeInForce"]))); value != "" {
		profile.TimeoutFallbackTIF = value
	}
}

func describeExecutionProfile(intent SignalIntent) string {
	reasonTag := normalizeStrategyReasonTag(intent.Reason)
	role := strings.ToLower(strings.TrimSpace(intent.Role))
	switch {
	case role == "exit" && reasonTag == "sl":
		return "sl-exit"
	case role == "exit" && reasonTag == "pt":
		return "pt-exit"
	case role == "exit":
		return "exit"
	default:
		return "entry"
	}
}

func buildExecutionSignalSignature(intent SignalIntent) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(intent.Action)),
		strings.ToUpper(strings.TrimSpace(intent.Side)),
		NormalizeSymbol(intent.Symbol),
		strings.TrimSpace(intent.SignalKind),
		stringValue(intent.Metadata["signalBarStateKey"]),
	}, "|")
}

func resolvePassiveBookPrice(side string, bestBid, bestAsk, fallback float64) float64 {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "BUY":
		return firstPositive(bestBid, fallback)
	case "SELL", "SHORT":
		return firstPositive(bestAsk, fallback)
	default:
		return fallback
	}
}

func signalIntentToMap(intent SignalIntent) map[string]any {
	return map[string]any{
		"action":         intent.Action,
		"role":           intent.Role,
		"reason":         intent.Reason,
		"side":           intent.Side,
		"symbol":         intent.Symbol,
		"signalKind":     intent.SignalKind,
		"decisionState":  intent.DecisionState,
		"plannedEventAt": intent.PlannedEventAt,
		"plannedPrice":   intent.PlannedPrice,
		"priceHint":      intent.PriceHint,
		"priceSource":    intent.PriceSource,
		"quantity":       intent.Quantity,
		"metadata":       cloneMetadata(intent.Metadata),
	}
}

func executionProposalToMap(proposal ExecutionProposal) map[string]any {
	return map[string]any{
		"action":            proposal.Action,
		"role":              proposal.Role,
		"reason":            proposal.Reason,
		"side":              proposal.Side,
		"symbol":            proposal.Symbol,
		"type":              proposal.Type,
		"quantity":          proposal.Quantity,
		"limitPrice":        proposal.LimitPrice,
		"priceHint":         proposal.PriceHint,
		"priceSource":       proposal.PriceSource,
		"timeInForce":       proposal.TimeInForce,
		"postOnly":          proposal.PostOnly,
		"reduceOnly":        proposal.ReduceOnly,
		"signalKind":        proposal.SignalKind,
		"decisionState":     proposal.DecisionState,
		"signalBarStateKey": proposal.SignalBarStateKey,
		"spreadBps":         proposal.SpreadBps,
		"bestBid":           proposal.BestBid,
		"bestAsk":           proposal.BestAsk,
		"executionStrategy": proposal.ExecutionStrategy,
		"status":            proposal.Status,
		"metadata":          cloneMetadata(proposal.Metadata),
	}
}

func executionProposalSummary(proposal map[string]any) map[string]any {
	metadata := cloneMetadata(mapValue(proposal["metadata"]))
	return map[string]any{
		"status":            stringValue(proposal["status"]),
		"executionStrategy": firstNonEmpty(stringValue(proposal["executionStrategy"]), stringValue(metadata["executionStrategy"])),
		"executionProfile":  firstNonEmpty(stringValue(metadata["executionProfile"]), stringValue(proposal["role"])),
		"executionMode":     stringValue(metadata["executionMode"]),
		"orderType":         stringValue(proposal["type"]),
		"timeInForce":       firstNonEmpty(stringValue(proposal["timeInForce"]), stringValue(metadata["executionProfileTimeInForce"])),
		"postOnly":          boolValue(proposal["postOnly"]) || boolValue(metadata["executionProfilePostOnly"]),
		"reduceOnly":        boolValue(proposal["reduceOnly"]) || boolValue(metadata["executionProfileReduceOnly"]),
		"fallback":          boolValue(metadata["fallbackFromTimeout"]),
		"fallbackOrderType": stringValue(metadata["fallbackOrderType"]),
		"priceSource":       stringValue(proposal["priceSource"]),
		"spreadBps":         parseFloatValue(proposal["spreadBps"]),
	}
}

func executionProposalFromMap(raw map[string]any) ExecutionProposal {
	meta := cloneMetadata(raw)
	return ExecutionProposal{
		Action:            stringValue(meta["action"]),
		Role:              stringValue(meta["role"]),
		Reason:            stringValue(meta["reason"]),
		Side:              stringValue(meta["side"]),
		Symbol:            NormalizeSymbol(stringValue(meta["symbol"])),
		Type:              strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(meta["type"]), "MARKET"))),
		Quantity:          parseFloatValue(meta["quantity"]),
		LimitPrice:        parseFloatValue(meta["limitPrice"]),
		PriceHint:         parseFloatValue(meta["priceHint"]),
		PriceSource:       stringValue(meta["priceSource"]),
		TimeInForce:       strings.ToUpper(strings.TrimSpace(stringValue(meta["timeInForce"]))),
		PostOnly:          boolValue(meta["postOnly"]),
		ReduceOnly:        boolValue(meta["reduceOnly"]),
		SignalKind:        stringValue(meta["signalKind"]),
		DecisionState:     stringValue(meta["decisionState"]),
		SignalBarStateKey: stringValue(meta["signalBarStateKey"]),
		SpreadBps:         parseFloatValue(meta["spreadBps"]),
		BestBid:           parseFloatValue(meta["bestBid"]),
		BestAsk:           parseFloatValue(meta["bestAsk"]),
		ExecutionStrategy: normalizeExecutionStrategyKey(stringValue(meta["executionStrategy"])),
		Status:            strings.ToLower(strings.TrimSpace(firstNonEmpty(stringValue(meta["status"]), "dispatchable"))),
		Metadata:          cloneMetadata(mapValue(meta["metadata"])),
	}
}
