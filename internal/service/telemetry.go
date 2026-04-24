package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) recordStrategyDecisionEvent(
	session domain.LiveSession,
	runtimeSessionID string,
	eventTime time.Time,
	triggerSummary map[string]any,
	sourceStates map[string]any,
	signalBarStates map[string]any,
	sourceGate map[string]any,
	executionContext StrategyExecutionContext,
	decision StrategySignalDecision,
	signalIntent map[string]any,
	executionProposal map[string]any,
) (domain.StrategyDecisionEvent, error) {
	intentSignature := ""
	if len(executionProposal) > 0 {
		intentSignature = buildLiveIntentSignature(executionProposal)
	}
	if intentSignature == "" && len(signalIntent) > 0 {
		intentSignature = buildLiveIntentSignature(signalIntent)
	}
	fingerprint := buildStrategyDecisionEventFingerprint(executionContext, decision, sourceGate, signalIntent, executionProposal)
	lastDecisionEventID := strings.TrimSpace(stringValue(session.State["lastStrategyDecisionEventId"]))
	if lastDecisionEventID != "" {
		existing, found, err := p.getStrategyDecisionEvent(session.ID, lastDecisionEventID)
		if err != nil {
			return domain.StrategyDecisionEvent{}, err
		}
		if found && strategyDecisionEventFingerprintFromRecorded(existing) == fingerprint && existing.IntentSignature == intentSignature {
			return existing, nil
		}
	}

	event := domain.StrategyDecisionEvent{
		LiveSessionID:     session.ID,
		RuntimeSessionID:  runtimeSessionID,
		AccountID:         session.AccountID,
		StrategyID:        session.StrategyID,
		StrategyVersionID: executionContext.StrategyVersionID,
		Symbol: firstNonEmpty(
			NormalizeSymbol(executionContext.Symbol),
			NormalizeSymbol(stringValue(executionProposal["symbol"])),
			NormalizeSymbol(stringValue(signalIntent["symbol"])),
			NormalizeSymbol(stringValue(session.State["symbol"])),
		),
		TriggerType:     firstNonEmpty(stringValue(triggerSummary["event"]), stringValue(triggerSummary["type"])),
		Action:          firstNonEmpty(decision.Action, "wait"),
		Reason:          firstNonEmpty(decision.Reason, "unspecified"),
		SignalKind:      firstNonEmpty(stringValue(decision.Metadata["signalKind"]), stringValue(executionProposal["signalKind"])),
		DecisionState:   firstNonEmpty(stringValue(decision.Metadata["decisionState"]), stringValue(executionProposal["decisionState"])),
		IntentSignature: intentSignature,
		SourceGateReady: boolValue(sourceGate["ready"]),
		MissingCount:    len(metadataList(sourceGate["missing"])),
		StaleCount:      len(metadataList(sourceGate["stale"])),
		EventTime:       eventTime.UTC(),
		TriggerSummary:  cloneMetadata(triggerSummary),
		SourceGate:      cloneMetadata(sourceGate),
		SourceStates:    cloneMetadata(sourceStates),
		SignalBarStates: cloneMetadata(signalBarStates),
		PositionSnapshot: cloneMetadata(firstNonEmptyMapValue(
			decision.Metadata["currentPosition"],
			session.State["recoveredPosition"],
			session.State["livePositionState"],
		)),
		DecisionMetadata:  cloneMetadata(decision.Metadata),
		SignalIntent:      cloneMetadata(signalIntent),
		ExecutionProposal: cloneMetadata(executionProposal),
		EvaluationContext: map[string]any{
			"strategyEngineKey":    executionContext.StrategyEngineKey,
			"strategyVersionId":    executionContext.StrategyVersionID,
			"signalTimeframe":      executionContext.SignalTimeframe,
			"executionDataSource":  executionContext.ExecutionDataSource,
			"symbol":               executionContext.Symbol,
			"executionMode":        string(executionContext.Semantics.Mode),
			"slippageMode":         string(executionContext.Semantics.SlippageMode),
			"tradingFeeBps":        executionContext.Semantics.TradingFeeBps,
			"fundingRateBps":       executionContext.Semantics.FundingRateBps,
			"fundingIntervalHours": executionContext.Semantics.FundingIntervalHours,
		},
	}
	recorded, err := p.store.CreateStrategyDecisionEvent(event)
	if err == nil {
		p.publishLogEvent(strategyDecisionToUnifiedLogEvent(recorded))
	}
	return recorded, err
}

func buildStrategyDecisionEventFingerprint(
	executionContext StrategyExecutionContext,
	decision StrategySignalDecision,
	sourceGate map[string]any,
	signalIntent map[string]any,
	executionProposal map[string]any,
) string {
	subject := executionProposal
	if len(subject) == 0 {
		subject = signalIntent
	}
	metadata := mapValue(subject["metadata"])
	return strings.Join([]string{
		firstNonEmpty(decision.Action, "wait"),
		firstNonEmpty(decision.Reason, "unspecified"),
		firstNonEmpty(stringValue(decision.Metadata["signalKind"]), stringValue(subject["signalKind"])),
		firstNonEmpty(stringValue(decision.Metadata["decisionState"]), stringValue(subject["decisionState"])),
		fmt.Sprintf("%t", boolValue(sourceGate["ready"])),
		fmt.Sprintf("%d", len(metadataList(sourceGate["missing"]))),
		fmt.Sprintf("%d", len(metadataList(sourceGate["stale"]))),
		NormalizeSymbol(firstNonEmpty(executionContext.Symbol, stringValue(subject["symbol"]))),
		firstNonEmpty(executionContext.SignalTimeframe, stringValue(mapValue(metadata["executionContext"])["signalTimeframe"])),
		firstNonEmpty(executionContext.ExecutionDataSource, stringValue(mapValue(metadata["executionContext"])["executionDataSource"])),
		buildLiveIntentSignature(subject),
		strings.ToLower(strings.TrimSpace(firstNonEmpty(stringValue(subject["status"]), "none"))),
		normalizeExecutionStrategyKey(firstNonEmpty(stringValue(subject["executionStrategy"]), stringValue(metadata["executionStrategy"]))),
		strings.ToLower(strings.TrimSpace(stringValue(metadata["executionDecision"]))),
		fmt.Sprintf("%.8f", parseFloatValue(subject["quantity"])),
		fmt.Sprintf("%t", boolValue(subject["reduceOnly"])),
		fmt.Sprintf("%t", boolValue(metadata["fallbackFromTimeout"])),
	}, "|")
}

func strategyDecisionEventFingerprintFromRecorded(event domain.StrategyDecisionEvent) string {
	return buildStrategyDecisionEventFingerprint(
		StrategyExecutionContext{
			StrategyVersionID:   stringValue(event.EvaluationContext["strategyVersionId"]),
			SignalTimeframe:     stringValue(event.EvaluationContext["signalTimeframe"]),
			ExecutionDataSource: stringValue(event.EvaluationContext["executionDataSource"]),
			Symbol:              stringValue(event.EvaluationContext["symbol"]),
		},
		StrategySignalDecision{
			Action:   event.Action,
			Reason:   event.Reason,
			Metadata: cloneMetadata(event.DecisionMetadata),
		},
		event.SourceGate,
		event.SignalIntent,
		event.ExecutionProposal,
	)
}

func (p *Platform) getStrategyDecisionEvent(liveSessionID, decisionEventID string) (domain.StrategyDecisionEvent, bool, error) {
	decisionEventID = strings.TrimSpace(decisionEventID)
	if decisionEventID == "" {
		return domain.StrategyDecisionEvent{}, false, nil
	}
	if reader, ok := p.store.(strategyDecisionEventQueryReader); ok {
		items, err := reader.QueryStrategyDecisionEvents(domain.StrategyDecisionEventQuery{
			LiveSessionID:   strings.TrimSpace(liveSessionID),
			DecisionEventID: decisionEventID,
			Limit:           1,
		})
		if err != nil {
			return domain.StrategyDecisionEvent{}, false, err
		}
		if len(items) > 0 {
			return items[0], true, nil
		}
		return domain.StrategyDecisionEvent{}, false, nil
	}
	items, err := p.store.ListStrategyDecisionEvents(strings.TrimSpace(liveSessionID))
	if err != nil {
		return domain.StrategyDecisionEvent{}, false, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == decisionEventID {
			return item, true, nil
		}
	}
	return domain.StrategyDecisionEvent{}, false, nil
}

func (p *Platform) ensureStrategyDecisionEventForExecutionProposal(session domain.LiveSession, strategyVersionID string, proposalMap map[string]any, eventTime time.Time, trigger string) (map[string]any, error) {
	normalized := cloneMetadata(proposalMap)
	if len(normalized) == 0 {
		return normalized, nil
	}
	metadata := cloneMetadata(mapValue(normalized["metadata"]))
	if metadata == nil {
		metadata = map[string]any{}
	}
	decisionEventID := firstNonEmpty(stringValue(normalized["decisionEventId"]), stringValue(metadata["decisionEventId"]))
	if decisionEventID != "" {
		exists, err := p.strategyDecisionEventExists(session.ID, decisionEventID)
		if err != nil {
			return normalized, err
		}
		if exists {
			return setExecutionProposalDecisionEventID(normalized, decisionEventID), nil
		}
	} else {
		decisionEventID = newStrategyDecisionEventID()
	}

	normalized = setExecutionProposalDecisionEventID(normalized, decisionEventID)
	event := strategyDecisionEventFromExecutionProposal(session, strategyVersionID, normalized, eventTime, trigger)
	event.ID = decisionEventID
	recorded, err := p.store.CreateStrategyDecisionEvent(event)
	if err != nil {
		return normalized, err
	}
	p.publishLogEvent(strategyDecisionToUnifiedLogEvent(recorded))
	return setExecutionProposalDecisionEventID(normalized, recorded.ID), nil
}

func (p *Platform) ensureStrategyDecisionEventForOrderExecution(order domain.Order, proposalMap map[string]any, eventTime time.Time, eventType string) error {
	metadata := mapValue(order.Metadata)
	decisionEventID := firstNonEmpty(stringValue(metadata["decisionEventId"]), stringValue(proposalMap["decisionEventId"]))
	if decisionEventID == "" {
		return nil
	}
	liveSessionID := stringValue(metadata["liveSessionId"])
	if liveSessionID == "" {
		return nil
	}
	exists, err := p.strategyDecisionEventExists(liveSessionID, decisionEventID)
	if err != nil || exists {
		return err
	}
	session, err := p.store.GetLiveSession(liveSessionID)
	if err != nil {
		return err
	}
	proposal := cloneMetadata(proposalMap)
	if len(proposal) == 0 {
		proposal = cloneMetadata(mapValue(firstNonEmptyMapValue(metadata["executionProposal"], metadata["intent"])))
	}
	if len(proposal) == 0 {
		proposal = map[string]any{
			"action":            liveOrderActionFromOrder(order),
			"role":              liveOrderRoleFromOrder(order),
			"reason":            firstNonEmpty(stringValue(metadata["reason"]), "order-execution"),
			"side":              order.Side,
			"symbol":            order.Symbol,
			"type":              order.Type,
			"quantity":          order.Quantity,
			"priceHint":         order.Price,
			"executionStrategy": stringValue(metadata["executionStrategy"]),
			"status":            "dispatchable",
		}
	}
	proposal = setExecutionProposalDecisionEventID(proposal, decisionEventID)
	_, err = p.ensureStrategyDecisionEventForExecutionProposal(
		session,
		order.StrategyVersionID,
		proposal,
		eventTime,
		orderExecutionDecisionBackfillTrigger(eventType),
	)
	return err
}

func strategyDecisionEventFromExecutionProposal(session domain.LiveSession, strategyVersionID string, proposalMap map[string]any, eventTime time.Time, trigger string) domain.StrategyDecisionEvent {
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	state := cloneMetadata(session.State)
	metadata := cloneMetadata(mapValue(proposalMap["metadata"]))
	if metadata == nil {
		metadata = map[string]any{}
	}
	lastDecision := mapValue(state["lastStrategyDecision"])
	decisionMetadata := cloneMetadata(mapValue(lastDecision["metadata"]))
	if decisionMetadata == nil {
		decisionMetadata = map[string]any{}
	}
	decisionMetadata["decisionEventBackfilled"] = true
	decisionMetadata["decisionEventBackfillTrigger"] = firstNonEmpty(trigger, "dispatch-preflight")

	sourceGate := cloneMetadata(mapValue(state["lastStrategyEvaluationSourceGate"]))
	if len(sourceGate) == 0 {
		sourceGate = map[string]any{
			"ready":      true,
			"backfilled": true,
			"trigger":    firstNonEmpty(trigger, "dispatch-preflight"),
		}
	}
	triggerSummary := cloneMetadata(mapValue(state["lastStrategyEvaluationRuntimeSummary"]))
	if len(triggerSummary) == 0 {
		triggerSummary = map[string]any{}
	}
	triggerSummary["event"] = firstNonEmpty(stringValue(triggerSummary["event"]), trigger, "dispatch-preflight")
	triggerSummary["decisionEventBackfilled"] = true

	evaluationContext := cloneMetadata(mapValue(metadata["executionContext"]))
	if len(evaluationContext) == 0 {
		evaluationContext = cloneMetadata(mapValue(state["lastStrategyEvaluationContext"]))
	}
	if evaluationContext == nil {
		evaluationContext = map[string]any{}
	}
	strategyVersionID = firstNonEmpty(strategyVersionID, stringValue(proposalMap["strategyVersionId"]), stringValue(metadata["strategyVersionId"]), stringValue(evaluationContext["strategyVersionId"]))
	if strategyVersionID != "" {
		evaluationContext["strategyVersionId"] = strategyVersionID
	}
	evaluationContext["signalTimeframe"] = firstNonEmpty(stringValue(evaluationContext["signalTimeframe"]), stringValue(state["signalTimeframe"]))
	evaluationContext["executionDataSource"] = firstNonEmpty(stringValue(evaluationContext["executionDataSource"]), stringValue(state["executionDataSource"]))
	evaluationContext["symbol"] = firstNonEmpty(NormalizeSymbol(stringValue(evaluationContext["symbol"])), NormalizeSymbol(stringValue(proposalMap["symbol"])), NormalizeSymbol(stringValue(state["symbol"])))
	evaluationContext["executionMode"] = firstNonEmpty(stringValue(evaluationContext["executionMode"]), stringValue(metadata["executionMode"]), "live")

	signalIntent := cloneMetadata(mapValue(state["lastSignalIntent"]))
	if len(signalIntent) == 0 {
		signalIntent = signalIntentFromExecutionProposalMap(proposalMap)
	}
	positionSnapshot := cloneMetadata(mapValue(state["recoveredPosition"]))
	if len(positionSnapshot) == 0 {
		positionSnapshot = cloneMetadata(mapValue(state["livePositionState"]))
	}

	return domain.StrategyDecisionEvent{
		LiveSessionID:     session.ID,
		RuntimeSessionID:  firstNonEmpty(stringValue(metadata["runtimeSessionId"]), stringValue(state["signalRuntimeSessionId"]), stringValue(state["lastSignalRuntimeSessionId"])),
		AccountID:         session.AccountID,
		StrategyID:        session.StrategyID,
		StrategyVersionID: strategyVersionID,
		Symbol: firstNonEmpty(
			NormalizeSymbol(stringValue(proposalMap["symbol"])),
			NormalizeSymbol(stringValue(evaluationContext["symbol"])),
			NormalizeSymbol(stringValue(state["symbol"])),
		),
		TriggerType:       firstNonEmpty(stringValue(triggerSummary["event"]), trigger, "dispatch-preflight"),
		Action:            firstNonEmpty(stringValue(proposalMap["action"]), stringValue(lastDecision["action"]), "dispatch"),
		Reason:            firstNonEmpty(stringValue(proposalMap["reason"]), stringValue(lastDecision["reason"]), "dispatchable-intent"),
		SignalKind:        firstNonEmpty(stringValue(proposalMap["signalKind"]), stringValue(metadata["signalKind"]), stringValue(decisionMetadata["signalKind"])),
		DecisionState:     firstNonEmpty(stringValue(proposalMap["decisionState"]), stringValue(metadata["decisionState"]), stringValue(decisionMetadata["decisionState"])),
		IntentSignature:   buildLiveIntentSignature(proposalMap),
		SourceGateReady:   boolValue(sourceGate["ready"]),
		MissingCount:      len(metadataList(sourceGate["missing"])),
		StaleCount:        len(metadataList(sourceGate["stale"])),
		EventTime:         eventTime.UTC(),
		TriggerSummary:    triggerSummary,
		SourceGate:        sourceGate,
		SourceStates:      cloneMetadata(mapValue(state["lastStrategyEvaluationSourceStates"])),
		SignalBarStates:   cloneMetadata(mapValue(state["lastStrategyEvaluationSignalBarStates"])),
		PositionSnapshot:  positionSnapshot,
		DecisionMetadata:  decisionMetadata,
		SignalIntent:      signalIntent,
		ExecutionProposal: cloneMetadata(proposalMap),
		EvaluationContext: evaluationContext,
	}
}

func (p *Platform) strategyDecisionEventExists(liveSessionID, decisionEventID string) (bool, error) {
	decisionEventID = strings.TrimSpace(decisionEventID)
	if decisionEventID == "" {
		return false, nil
	}
	if reader, ok := p.store.(strategyDecisionEventQueryReader); ok {
		items, err := reader.QueryStrategyDecisionEvents(domain.StrategyDecisionEventQuery{
			LiveSessionID:   strings.TrimSpace(liveSessionID),
			DecisionEventID: decisionEventID,
			Limit:           1,
		})
		if err != nil {
			return false, err
		}
		return len(items) > 0, nil
	}
	items, err := p.store.ListStrategyDecisionEvents(strings.TrimSpace(liveSessionID))
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) != decisionEventID {
			continue
		}
		if strings.TrimSpace(liveSessionID) == "" || strings.TrimSpace(item.LiveSessionID) == strings.TrimSpace(liveSessionID) {
			return true, nil
		}
	}
	return false, nil
}

func setExecutionProposalDecisionEventID(proposalMap map[string]any, decisionEventID string) map[string]any {
	normalized := cloneMetadata(proposalMap)
	if len(normalized) == 0 || strings.TrimSpace(decisionEventID) == "" {
		return normalized
	}
	metadata := cloneMetadata(mapValue(normalized["metadata"]))
	if metadata == nil {
		metadata = map[string]any{}
	}
	normalized["decisionEventId"] = decisionEventID
	metadata["decisionEventId"] = decisionEventID
	normalized["metadata"] = metadata
	return normalized
}

func signalIntentFromExecutionProposalMap(proposalMap map[string]any) map[string]any {
	return map[string]any{
		"action":         stringValue(proposalMap["action"]),
		"role":           stringValue(proposalMap["role"]),
		"reason":         stringValue(proposalMap["reason"]),
		"side":           stringValue(proposalMap["side"]),
		"symbol":         NormalizeSymbol(stringValue(proposalMap["symbol"])),
		"signalKind":     stringValue(proposalMap["signalKind"]),
		"decisionState":  stringValue(proposalMap["decisionState"]),
		"plannedEventAt": stringValue(proposalMap["plannedEventAt"]),
		"plannedPrice":   parseFloatValue(proposalMap["plannedPrice"]),
		"priceHint":      parseFloatValue(proposalMap["priceHint"]),
		"priceSource":    stringValue(proposalMap["priceSource"]),
		"quantity":       parseFloatValue(proposalMap["quantity"]),
		"metadata":       cloneMetadata(mapValue(proposalMap["metadata"])),
	}
}

func liveOrderActionFromOrder(order domain.Order) string {
	if order.EffectiveReduceOnly() || order.EffectiveClosePosition() {
		return "exit"
	}
	return "entry"
}

func liveOrderRoleFromOrder(order domain.Order) string {
	if order.EffectiveReduceOnly() || order.EffectiveClosePosition() {
		return "exit"
	}
	return "entry"
}

func orderExecutionDecisionBackfillTrigger(eventType string) string {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return "order-execution"
	}
	return "order-execution-" + eventType
}

func newStrategyDecisionEventID() string {
	return fmt.Sprintf("strategy-decision-event-%d", time.Now().UTC().UnixNano())
}

func (p *Platform) recordLiveOrderExecutionEvent(order domain.Order, eventType string, eventTime time.Time, failed bool, eventErr error) error {
	if !stringsEqualFoldSafe(stringValue(order.Metadata["executionMode"]), "live") && !stringsEqualFoldSafe(stringValue(order.Metadata["source"]), "live-session-intent") {
		return nil
	}

	proposalMap := cloneMetadata(mapValue(order.Metadata["executionProposal"]))
	dispatchSummary := executionDispatchSummary(proposalMap, order, failed)
	if eventTime.IsZero() {
		eventTime = firstNonZeroTime(
			parseOptionalRFC3339(stringValue(order.Metadata["lastFilledAt"])),
			parseOptionalRFC3339(stringValue(order.Metadata["lastSyncAt"])),
			parseOptionalRFC3339(stringValue(order.Metadata["acceptedAt"])),
			order.CreatedAt,
			time.Now().UTC(),
		)
	}

	if err := p.ensureStrategyDecisionEventForOrderExecution(order, proposalMap, eventTime, eventType); err != nil {
		return err
	}

	event := domain.OrderExecutionEvent{
		OrderID:           order.ID,
		ExchangeOrderID:   firstNonEmpty(stringValue(order.Metadata["exchangeOrderId"]), stringValue(dispatchSummary["exchangeOrderId"])),
		LiveSessionID:     stringValue(order.Metadata["liveSessionId"]),
		DecisionEventID:   firstNonEmpty(stringValue(order.Metadata["decisionEventId"]), stringValue(proposalMap["decisionEventId"])),
		RuntimeSessionID:  stringValue(order.Metadata["runtimeSessionId"]),
		AccountID:         order.AccountID,
		StrategyVersionID: order.StrategyVersionID,
		Symbol:            firstNonEmpty(order.Symbol, stringValue(dispatchSummary["symbol"])),
		Side:              firstNonEmpty(order.Side, stringValue(dispatchSummary["side"])),
		OrderType:         firstNonEmpty(order.Type, stringValue(dispatchSummary["orderType"])),
		EventType:         eventType,
		Status:            firstNonEmpty(order.Status, stringValue(dispatchSummary["status"])),
		ExecutionStrategy: stringValue(dispatchSummary["executionStrategy"]),
		ExecutionDecision: stringValue(dispatchSummary["executionDecision"]),
		ExecutionMode:     firstNonEmpty(stringValue(dispatchSummary["executionMode"]), stringValue(order.Metadata["executionMode"])),
		Quantity:          firstPositive(order.Quantity, parseFloatValue(dispatchSummary["quantity"])),
		Price:             firstPositive(order.Price, parseFloatValue(dispatchSummary["price"])),
		ExpectedPrice:     parseFloatValue(dispatchSummary["expectedPrice"]),
		PriceDriftBps:     parseFloatValue(dispatchSummary["priceDriftBps"]),
		RawQuantity:       parseFloatValue(dispatchSummary["rawQuantity"]),
		NormalizedQty:     parseFloatValue(dispatchSummary["normalizedQuantity"]),
		RawPriceReference: parseFloatValue(dispatchSummary["rawPriceReference"]),
		NormalizedPrice:   parseFloatValue(dispatchSummary["normalizedPrice"]),
		SpreadBps:         parseFloatValue(dispatchSummary["spreadBps"]),
		BookImbalance:     parseFloatValue(dispatchSummary["bookImbalance"]),
		SubmitLatencyMs:   latencyMillis(order.CreatedAt, parseOptionalRFC3339(stringValue(order.Metadata["acceptedAt"]))),
		SyncLatencyMs:     latencyMillis(order.CreatedAt, parseOptionalRFC3339(stringValue(order.Metadata["lastSyncAt"]))),
		FillLatencyMs:     latencyMillis(order.CreatedAt, parseOptionalRFC3339(stringValue(order.Metadata["lastFilledAt"]))),
		EventTime:         eventTime.UTC(),
		Fallback:          boolValue(dispatchSummary["fallback"]),
		PostOnly:          boolValue(dispatchSummary["postOnly"]),
		ReduceOnly:        boolValue(dispatchSummary["reduceOnly"]),
		Failed:            failed,
		Error:             errorString(eventErr),
		RuntimePreflight:  cloneMetadata(mapValue(order.Metadata["runtimePreflight"])),
		DispatchSummary:   dispatchSummary,
		AdapterSubmission: cloneMetadata(mapValue(order.Metadata["adapterSubmission"])),
		AdapterSync:       cloneMetadata(mapValue(order.Metadata["adapterSync"])),
		Normalization:     cloneMetadata(mapValue(dispatchSummary["normalization"])),
		SymbolRules:       cloneMetadata(mapValue(dispatchSummary["symbolRules"])),
		Metadata:          cloneMetadata(order.Metadata),
	}
	recorded, err := p.store.CreateOrderExecutionEvent(event)
	if err == nil {
		p.publishLogEvent(orderExecutionToUnifiedLogEvent(recorded))
	}
	return err
}

func (p *Platform) recordLivePositionAccountSnapshot(session domain.LiveSession, eventTime time.Time, trigger string, orderID string) error {
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return err
	}
	summary, _ := p.accountSummaryByID(session.AccountID)
	state := cloneMetadata(session.State)
	positionSnapshot := cloneMetadata(mapValue(state["recoveredPosition"]))
	livePositionState := cloneMetadata(mapValue(state["livePositionState"]))
	accountSnapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	symbol := NormalizeSymbol(firstNonEmpty(
		stringValue(positionSnapshot["symbol"]),
		stringValue(livePositionState["symbol"]),
		stringValue(state["symbol"]),
		stringValue(state["lastSymbol"]),
	))
	if symbol == "" {
		return nil
	}
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	if orderID == "" {
		orderID = stringValue(state["lastDispatchedOrderId"])
	}

	snapshot := domain.PositionAccountSnapshot{
		LiveSessionID:     session.ID,
		DecisionEventID:   stringValue(state["lastStrategyDecisionEventId"]),
		OrderID:           orderID,
		AccountID:         session.AccountID,
		StrategyID:        session.StrategyID,
		Symbol:            symbol,
		Trigger:           firstNonEmpty(trigger, "live-position-refresh"),
		IntentSignature:   firstNonEmpty(stringValue(state["lastDispatchedIntentSignature"]), stringValue(state["lastStrategyIntentSignature"])),
		PositionFound:     boolValue(state["hasRecoveredPosition"]),
		PositionSide:      firstNonEmpty(stringValue(positionSnapshot["side"]), stringValue(livePositionState["side"])),
		PositionQuantity:  firstPositive(parseFloatValue(positionSnapshot["quantity"]), parseFloatValue(livePositionState["quantity"])),
		EntryPrice:        firstPositive(parseFloatValue(positionSnapshot["entryPrice"]), parseFloatValue(livePositionState["entryPrice"])),
		MarkPrice:         firstPositive(parseFloatValue(positionSnapshot["markPrice"]), parseFloatValue(livePositionState["markPrice"])),
		NetEquity:         summary.NetEquity,
		AvailableBalance:  summary.AvailableBalance,
		MarginBalance:     summary.MarginBalance,
		WalletBalance:     summary.WalletBalance,
		ExposureNotional:  summary.ExposureNotional,
		OpenPositionCount: summary.OpenPositionCount,
		SyncStatus:        stringValue(accountSnapshot["syncStatus"]),
		EventTime:         eventTime.UTC(),
		PositionSnapshot:  positionSnapshot,
		LivePositionState: livePositionState,
		AccountSnapshot:   accountSnapshot,
		AccountSummary: map[string]any{
			"netEquity":         summary.NetEquity,
			"availableBalance":  summary.AvailableBalance,
			"walletBalance":     summary.WalletBalance,
			"marginBalance":     summary.MarginBalance,
			"exposureNotional":  summary.ExposureNotional,
			"openPositionCount": summary.OpenPositionCount,
			"updatedAt":         summary.UpdatedAt.Format(time.RFC3339),
		},
		Metadata: map[string]any{
			"positionRecoveryStatus":      stringValue(state["positionRecoveryStatus"]),
			"lastPositionContextSource":   stringValue(state["lastPositionContextSource"]),
			"lastPositionContextAt":       stringValue(state["lastPositionContextRefreshAt"]),
			"hasRecoveredRealPosition":    boolValue(state["hasRecoveredRealPosition"]),
			"hasRecoveredVirtualPosition": boolValue(state["hasRecoveredVirtualPosition"]),
			"protectionRecoveryStatus":    stringValue(state["protectionRecoveryStatus"]),
			"recoveredProtectionCount":    maxIntValue(state["recoveredProtectionCount"], 0),
		},
	}
	recorded, err := p.store.CreatePositionAccountSnapshot(snapshot)
	if err == nil {
		p.publishLogEvent(positionSnapshotToUnifiedLogEvent(recorded))
	}
	return err
}

func (p *Platform) accountSummaryByID(accountID string) (domain.AccountSummary, bool) {
	summaries, err := p.ListAccountSummaries()
	if err != nil {
		return domain.AccountSummary{}, false
	}
	for _, item := range summaries {
		if item.AccountID == accountID {
			return item, true
		}
	}
	return domain.AccountSummary{}, false
}

func latencyMillis(start, end time.Time) int {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return int(end.Sub(start) / time.Millisecond)
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func stringsEqualFoldSafe(left, right string) bool {
	return left != "" && right != "" && strings.EqualFold(left, right)
}
