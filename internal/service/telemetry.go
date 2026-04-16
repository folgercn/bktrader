package service

import (
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
		TriggerType:       firstNonEmpty(stringValue(triggerSummary["event"]), stringValue(triggerSummary["type"])),
		Action:            firstNonEmpty(decision.Action, "wait"),
		Reason:            firstNonEmpty(decision.Reason, "unspecified"),
		SignalKind:        firstNonEmpty(stringValue(decision.Metadata["signalKind"]), stringValue(executionProposal["signalKind"])),
		DecisionState:     firstNonEmpty(stringValue(decision.Metadata["decisionState"]), stringValue(executionProposal["decisionState"])),
		IntentSignature:   intentSignature,
		SourceGateReady:   boolValue(sourceGate["ready"]),
		MissingCount:      len(metadataList(sourceGate["missing"])),
		StaleCount:        len(metadataList(sourceGate["stale"])),
		EventTime:         eventTime.UTC(),
		TriggerSummary:    cloneMetadata(triggerSummary),
		SourceGate:        cloneMetadata(sourceGate),
		SourceStates:      cloneMetadata(sourceStates),
		SignalBarStates:   cloneMetadata(signalBarStates),
		PositionSnapshot:  cloneMetadata(mapValue(session.State["recoveredPosition"])),
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
