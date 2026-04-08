package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func updateExecutionEventStats(state map[string]any, proposalMap map[string]any, dispatchSummary map[string]any) {
	if state == nil {
		return
	}
	stats := cloneMetadata(mapValue(state["executionEventStats"]))
	if len(stats) == 0 {
		stats = map[string]any{
			"aggregationMode": "event",
			"deduplicated":    false,
		}
	}
	metadata := cloneMetadata(mapValue(proposalMap["metadata"]))
	incrementInt := func(key string) {
		stats[key] = maxIntValue(stats[key], 0) + 1
	}
	accumulateFloat := func(key string, value float64) {
		if value == 0 {
			return
		}
		stats[key] = parseFloatValue(stats[key]) + value
	}
	updateAverage := func(sumKey, countKey, avgKey string) {
		sum := parseFloatValue(stats[sumKey])
		count := maxIntValue(stats[countKey], 0)
		if count > 0 {
			stats[avgKey] = sum / float64(count)
		}
	}

	if len(proposalMap) > 0 {
		incrementInt("proposalCount")
		switch strings.ToLower(strings.TrimSpace(stringValue(proposalMap["status"]))) {
		case "dispatchable":
			incrementInt("dispatchableProposalCount")
		case "wait":
			incrementInt("waitProposalCount")
		case "blocked":
			incrementInt("blockedProposalCount")
		case "virtual-initial", "virtual-exit":
			incrementInt("virtualProposalCount")
		}
		switch strings.ToLower(strings.TrimSpace(stringValue(metadata["executionDecision"]))) {
		case "maker-resting":
			incrementInt("makerRestingDecisionCount")
		case "timeout-fallback":
			incrementInt("timeoutFallbackDecisionCount")
		case "direct-dispatch":
			incrementInt("directDispatchDecisionCount")
		case "wait-spread-too-wide":
			incrementInt("waitWideSpreadDecisionCount")
		}
		spreadBps := parseFloatValue(proposalMap["spreadBps"])
		if spreadBps > 0 {
			accumulateFloat("proposalSpreadBpsSum", spreadBps)
			incrementInt("proposalSpreadSampleCount")
			updateAverage("proposalSpreadBpsSum", "proposalSpreadSampleCount", "avgProposalSpreadBps")
		}
		bookImbalance := parseFloatValue(metadata["bookImbalance"])
		if bookImbalance != 0 {
			accumulateFloat("bookImbalanceSum", bookImbalance)
			incrementInt("bookImbalanceSampleCount")
			updateAverage("bookImbalanceSum", "bookImbalanceSampleCount", "avgBookImbalance")
		}
	}

	if len(dispatchSummary) > 0 {
		incrementInt("dispatchEventCount")
		switch strings.ToUpper(strings.TrimSpace(stringValue(dispatchSummary["status"]))) {
		case "FILLED":
			incrementInt("filledCount")
		case "REJECTED":
			incrementInt("rejectedCount")
		case "CANCELLED":
			incrementInt("cancelledCount")
		}
		if boolValue(dispatchSummary["fallback"]) {
			incrementInt("fallbackDispatchCount")
		}
		if boolValue(dispatchSummary["postOnly"]) {
			incrementInt("postOnlyDispatchCount")
		}
		if boolValue(dispatchSummary["reduceOnly"]) {
			incrementInt("reduceOnlyDispatchCount")
		}
		switch strings.ToUpper(strings.TrimSpace(stringValue(dispatchSummary["orderType"]))) {
		case "MARKET":
			incrementInt("marketOrderCount")
		case "LIMIT":
			incrementInt("limitOrderCount")
		}
		driftBps := parseFloatValue(dispatchSummary["priceDriftBps"])
		if driftBps >= 0 {
			accumulateFloat("priceDriftBpsSum", driftBps)
			incrementInt("priceDriftSampleCount")
			updateAverage("priceDriftBpsSum", "priceDriftSampleCount", "avgPriceDriftBps")
		}
	}

	state["executionEventStats"] = stats
}

func (p *Platform) SyncLiveSession(sessionID string) (domain.LiveSession, error) {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.syncLatestLiveSessionOrder(session, time.Now().UTC())
}

func (p *Platform) syncActiveLiveSessions(eventTime time.Time) error {
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if !strings.EqualFold(session.Status, "RUNNING") {
			continue
		}
		if strings.TrimSpace(stringValue(session.State["lastDispatchedOrderId"])) == "" {
			continue
		}
		_, _ = p.syncLatestLiveSessionOrder(session, eventTime)
	}
	return nil
}

func (p *Platform) DispatchLiveSessionIntent(sessionID string) (domain.Order, error) {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.Order{}, err
	}
	return p.dispatchLiveSessionIntent(session)
}

func (p *Platform) dispatchLiveSessionIntent(session domain.LiveSession) (domain.Order, error) {
	if !strings.EqualFold(session.Status, "RUNNING") && !strings.EqualFold(session.Status, "READY") {
		return domain.Order{}, fmt.Errorf("live session %s is not dispatchable in status %s", session.ID, session.Status)
	}

	proposalMap := cloneMetadata(mapValue(firstNonEmptyMapValue(session.State["lastExecutionProposal"], session.State["lastStrategyIntent"])))
	if len(proposalMap) == 0 {
		return domain.Order{}, fmt.Errorf("live session %s has no execution proposal", session.ID)
	}
	proposal := executionProposalFromMap(proposalMap)
	if !strings.EqualFold(proposal.Status, "dispatchable") {
		return domain.Order{}, fmt.Errorf("live session %s execution proposal is not dispatchable: %s", session.ID, firstNonEmpty(proposal.Status, "unknown"))
	}

	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.Order{}, err
	}
	order := buildLiveOrderFromExecutionProposal(session, version.ID, proposal, proposalMap)
	created, createErr := p.CreateOrder(order)
	if createErr != nil && created.ID == "" {
		return domain.Order{}, createErr
	}

	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	dispatchedAt := time.Now().UTC()
	state["lastDispatchedOrderId"] = created.ID
	state["lastDispatchedOrderStatus"] = created.Status
	state["lastExecutionDispatch"] = executionDispatchSummary(proposalMap, created, false)
	updateExecutionEventStats(state, proposalMap, mapValue(state["lastExecutionDispatch"]))
	if isTerminalOrderStatus(created.Status) {
		state["lastSyncedOrderId"] = created.ID
		state["lastSyncedOrderStatus"] = created.Status
	}
	state["lastDispatchedAt"] = dispatchedAt.Format(time.RFC3339)
	state["lastDispatchedIntent"] = proposalMap
	state["lastDispatchedIntentSignature"] = intentSignature
	delete(state, "lastExecutionTimeoutAt")
	delete(state, "lastExecutionTimeoutReason")
	delete(state, "lastExecutionTimeoutIntentSignature")
	if shouldAdvanceLivePlanForOrderStatus(created.Status) {
		state["planIndex"] = resolveNextLivePlanIndex(state)
		state["lastEventTime"] = firstNonEmpty(stringValue(proposalMap["plannedEventAt"]), dispatchedAt.Format(time.RFC3339))
		state["lastEventSide"] = created.Side
		state["lastEventRole"] = proposal.Role
		state["lastEventReason"] = proposal.Reason
		delete(state, "lastStrategyIntent")
		delete(state, "lastExecutionProposal")
	} else {
		state["lastDispatchRejectedAt"] = dispatchedAt.Format(time.RFC3339)
		state["lastDispatchRejectedStatus"] = created.Status
		if shouldMarkLiveExecutionFallback(created) {
			state["lastExecutionTimeoutAt"] = dispatchedAt.Format(time.RFC3339)
			state["lastExecutionTimeoutReason"] = "maker-rejected-post-only"
			state["lastExecutionTimeoutIntentSignature"] = intentSignature
		}
	}
	if createErr != nil {
		state["lastAutoDispatchError"] = createErr.Error()
		state["lastAutoDispatchAttemptAt"] = dispatchedAt.Format(time.RFC3339)
	} else {
		delete(state, "lastAutoDispatchError")
	}
	appendTimelineEvent(state, "order", dispatchedAt, "live-intent-dispatched", executionDispatchTimelineMetadata(proposalMap, created, createErr != nil))
	if strings.EqualFold(created.Status, "FILLED") {
		if _, syncErr := p.SyncLiveAccount(session.AccountID); syncErr != nil {
			state["lastSyncError"] = syncErr.Error()
		}
	}
	updatedSession, _ := p.store.UpdateLiveSessionState(session.ID, state)
	if strings.EqualFold(created.Status, "FILLED") && updatedSession.ID != "" {
		if refreshed, refreshErr := p.refreshLiveSessionPositionContext(updatedSession, dispatchedAt, "live-order-fill-sync"); refreshErr == nil {
			updatedSession = refreshed
		}
	}
	if updatedSession.ID != "" {
		_, _ = p.syncLatestLiveSessionOrder(updatedSession, time.Now().UTC())
	}
	if createErr != nil {
		return created, createErr
	}
	return created, nil
}

func buildLiveOrderFromExecutionProposal(session domain.LiveSession, strategyVersionID string, proposal ExecutionProposal, proposalMap map[string]any) domain.Order {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(proposal.Type, "MARKET")))
	quantity := firstPositive(proposal.Quantity, firstPositive(parseFloatValue(session.State["defaultOrderQuantity"]), 0.001))
	price := proposal.PriceHint
	if orderType != "MARKET" {
		price = firstPositive(proposal.LimitPrice, proposal.PriceHint)
	}
	return domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: strategyVersionID,
		Symbol:            NormalizeSymbol(firstNonEmpty(proposal.Symbol, stringValue(session.State["symbol"]))),
		Side:              strings.ToUpper(strings.TrimSpace(proposal.Side)),
		Type:              orderType,
		Quantity:          quantity,
		Price:             price,
		Metadata: map[string]any{
			"source":             "live-session-intent",
			"liveSessionId":      session.ID,
			"signalKind":         proposal.SignalKind,
			"dispatchMode":       stringValue(session.State["dispatchMode"]),
			"timeInForce":        proposal.TimeInForce,
			"postOnly":           proposal.PostOnly,
			"reduceOnly":         proposal.ReduceOnly,
			"executionStrategy":  proposal.ExecutionStrategy,
			"executionExpiresAt": stringValue(proposal.Metadata["executionExpiresAt"]),
			"executionProposal":  cloneMetadata(proposalMap),
			"intent":             cloneMetadata(proposalMap),
		},
	}
}

func (p *Platform) applyLiveVirtualInitialEvent(session domain.LiveSession, proposalMap map[string]any, eventTime time.Time) (domain.LiveSession, error) {
	proposal := executionProposalFromMap(proposalMap)
	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	entryPrice := firstPositive(
		parseFloatValue(proposalMap["plannedPrice"]),
		firstPositive(
			parseFloatValue(proposalMap["priceHint"]),
			firstPositive(
				parseFloatValue(mapValue(proposalMap["metadata"])["bestAsk"]),
				parseFloatValue(mapValue(proposalMap["metadata"])["bestBid"]),
			),
		),
	)
	virtualSide := "LONG"
	if strings.EqualFold(proposal.Side, "SELL") || strings.EqualFold(proposal.Side, "SHORT") {
		virtualSide = "SHORT"
	}
	state["lastDispatchedAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastDispatchedIntent"] = cloneMetadata(proposalMap)
	state["lastDispatchedIntentSignature"] = intentSignature
	state["lastDispatchedOrderStatus"] = liveOrderStatusVirtualInitial
	state["lastSyncedOrderStatus"] = liveOrderStatusVirtualInitial
	state["lastExecutionDispatch"] = executionDispatchSummary(proposalMap, domain.Order{
		Side:     proposal.Side,
		Symbol:   proposal.Symbol,
		Type:     proposal.Type,
		Quantity: proposal.Quantity,
		Price:    firstPositive(proposal.LimitPrice, proposal.PriceHint),
		Status:   liveOrderStatusVirtualInitial,
	}, false)
	updateExecutionEventStats(state, proposalMap, mapValue(state["lastExecutionDispatch"]))
	state["lastVirtualSignalAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastVirtualSignalType"] = "initial"
	state["virtualPosition"] = map[string]any{
		"found":      true,
		"virtual":    true,
		"symbol":     NormalizeSymbol(proposal.Symbol),
		"side":       virtualSide,
		"quantity":   0.0,
		"entryPrice": entryPrice,
		"markPrice":  entryPrice,
		"reason":     proposal.Reason,
		"recordedAt": eventTime.UTC().Format(time.RFC3339),
	}
	state["planIndex"] = resolveNextLivePlanIndex(state)
	state["lastEventTime"] = firstNonEmpty(stringValue(proposalMap["plannedEventAt"]), eventTime.UTC().Format(time.RFC3339))
	state["lastEventSide"] = proposal.Side
	state["lastEventRole"] = proposal.Role
	state["lastEventReason"] = proposal.Reason
	delete(state, "lastStrategyIntent")
	delete(state, "lastExecutionProposal")
	delete(state, "lastAutoDispatchError")
	appendTimelineEvent(state, "strategy", eventTime, "live-virtual-initial-recorded", executionDispatchTimelineMetadata(proposalMap, domain.Order{
		Side:     proposal.Side,
		Symbol:   proposal.Symbol,
		Type:     proposal.Type,
		Quantity: proposal.Quantity,
		Price:    entryPrice,
		Status:   liveOrderStatusVirtualInitial,
	}, false))
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) applyLiveVirtualExitEvent(session domain.LiveSession, proposalMap map[string]any, eventTime time.Time) (domain.LiveSession, error) {
	proposal := executionProposalFromMap(proposalMap)
	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	state["lastDispatchedAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastDispatchedIntent"] = cloneMetadata(proposalMap)
	state["lastDispatchedIntentSignature"] = intentSignature
	state["lastDispatchedOrderStatus"] = liveOrderStatusVirtualExit
	state["lastSyncedOrderStatus"] = liveOrderStatusVirtualExit
	state["lastExecutionDispatch"] = executionDispatchSummary(proposalMap, domain.Order{
		Side:     proposal.Side,
		Symbol:   proposal.Symbol,
		Type:     proposal.Type,
		Quantity: proposal.Quantity,
		Price:    firstPositive(proposal.LimitPrice, proposal.PriceHint),
		Status:   liveOrderStatusVirtualExit,
	}, false)
	updateExecutionEventStats(state, proposalMap, mapValue(state["lastExecutionDispatch"]))
	state["lastVirtualSignalAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastVirtualSignalType"] = "exit"
	delete(state, "virtualPosition")
	state["planIndex"] = resolveNextLivePlanIndex(state)
	state["lastEventTime"] = firstNonEmpty(stringValue(proposalMap["plannedEventAt"]), eventTime.UTC().Format(time.RFC3339))
	state["lastEventSide"] = proposal.Side
	state["lastEventRole"] = proposal.Role
	state["lastEventReason"] = proposal.Reason
	delete(state, "lastStrategyIntent")
	delete(state, "lastExecutionProposal")
	delete(state, "lastAutoDispatchError")
	appendTimelineEvent(state, "strategy", eventTime, "live-virtual-exit-recorded", executionDispatchTimelineMetadata(proposalMap, domain.Order{
		Side:     proposal.Side,
		Symbol:   proposal.Symbol,
		Type:     proposal.Type,
		Quantity: proposal.Quantity,
		Price:    firstPositive(proposal.LimitPrice, proposal.PriceHint),
		Status:   liveOrderStatusVirtualExit,
	}, false))
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) syncLatestLiveSessionOrder(session domain.LiveSession, eventTime time.Time) (domain.LiveSession, error) {
	orderID := stringValue(session.State["lastDispatchedOrderId"])
	if strings.TrimSpace(orderID) == "" {
		return session, nil
	}
	order, err := p.GetOrder(orderID)
	if err != nil {
		return session, err
	}
	state := cloneMetadata(session.State)
	if isTerminalOrderStatus(order.Status) {
		state["lastSyncedOrderId"] = order.ID
		state["lastSyncedOrderStatus"] = order.Status
		state["lastDispatchedOrderStatus"] = order.Status
		state["lastExecutionDispatch"] = executionDispatchSummary(mapValue(order.Metadata["executionProposal"]), order, false)
		updateExecutionEventStats(state, mapValue(order.Metadata["executionProposal"]), mapValue(state["lastExecutionDispatch"]))
		if strings.EqualFold(order.Status, "FILLED") {
			_, _ = p.SyncLiveAccount(session.AccountID)
		}
		updated, err := p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			return domain.LiveSession{}, err
		}
		if strings.EqualFold(order.Status, "FILLED") {
			if refreshed, refreshErr := p.refreshLiveSessionPositionContext(updated, eventTime, "live-order-sync"); refreshErr == nil {
				return refreshed, nil
			}
		}
		return updated, nil
	}
	if shouldCancelLiveOrderForExecutionTimeout(order, eventTime) {
		cancelledOrder, cancelErr := p.CancelLiveOrder(order.ID)
		state["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
		if cancelErr != nil {
			state["lastSyncError"] = cancelErr.Error()
			appendTimelineEvent(state, "order", eventTime, "live-order-cancel-error", map[string]any{
				"orderId": order.ID,
				"error":   cancelErr.Error(),
			})
			updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
			if updateErr != nil {
				return domain.LiveSession{}, updateErr
			}
			return updated, cancelErr
		}
		delete(state, "lastSyncError")
		state["lastSyncedOrderId"] = cancelledOrder.ID
		state["lastSyncedOrderStatus"] = cancelledOrder.Status
		state["lastDispatchedOrderStatus"] = cancelledOrder.Status
		state["lastSyncedAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastExecutionTimeoutAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastExecutionTimeoutReason"] = "resting-order-expired"
		state["lastExecutionDispatch"] = executionDispatchSummary(mapValue(order.Metadata["executionProposal"]), cancelledOrder, false)
		updateExecutionEventStats(state, mapValue(order.Metadata["executionProposal"]), mapValue(state["lastExecutionDispatch"]))
		timeoutSignature := buildLiveIntentSignature(mapValue(order.Metadata["executionProposal"]))
		if timeoutSignature == "" {
			timeoutSignature = buildLiveIntentSignature(mapValue(order.Metadata["intent"]))
		}
		if timeoutSignature != "" {
			state["lastExecutionTimeoutIntentSignature"] = timeoutSignature
		}
		appendTimelineEvent(state, "order", eventTime, "live-order-cancelled-timeout", executionTimeoutTimelineMetadata(order, cancelledOrder))
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
	syncedOrder, err := p.SyncLiveOrder(order.ID)
	state["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
	if err != nil {
		state["lastSyncError"] = err.Error()
		appendTimelineEvent(state, "order", eventTime, "live-order-sync-error", map[string]any{
			"orderId": order.ID,
			"error":   err.Error(),
		})
		updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return updated, err
	}
	delete(state, "lastSyncError")
	state["lastSyncedOrderId"] = syncedOrder.ID
	state["lastSyncedOrderStatus"] = syncedOrder.Status
	state["lastDispatchedOrderStatus"] = syncedOrder.Status
	state["lastSyncedAt"] = time.Now().UTC().Format(time.RFC3339)
	state["lastExecutionDispatch"] = executionDispatchSummary(mapValue(order.Metadata["executionProposal"]), syncedOrder, false)
	updateExecutionEventStats(state, mapValue(order.Metadata["executionProposal"]), mapValue(state["lastExecutionDispatch"]))
	if strings.EqualFold(syncedOrder.Status, "FILLED") {
		_, _ = p.SyncLiveAccount(session.AccountID)
	}
	appendTimelineEvent(state, "order", eventTime, "live-order-synced", executionDispatchTimelineMetadata(mapValue(order.Metadata["executionProposal"]), syncedOrder, false))
	updated, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if strings.EqualFold(syncedOrder.Status, "FILLED") {
		if refreshed, refreshErr := p.refreshLiveSessionPositionContext(updated, eventTime, "live-order-sync"); refreshErr == nil {
			return refreshed, nil
		}
	}
	return updated, nil
}

func isTerminalOrderStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FILLED", "CANCELLED", "REJECTED", liveOrderStatusVirtualInitial, liveOrderStatusVirtualExit:
		return true
	default:
		return false
	}
}

func shouldCancelLiveOrderForExecutionTimeout(order domain.Order, eventTime time.Time) bool {
	if isTerminalOrderStatus(order.Status) {
		return false
	}
	expiresAt := parseOptionalRFC3339(stringValue(order.Metadata["executionExpiresAt"]))
	if expiresAt.IsZero() {
		return false
	}
	return !eventTime.UTC().Before(expiresAt.UTC())
}

func shouldAdvanceLivePlanForOrderStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "NEW", "ACCEPTED", "PARTIALLY_FILLED", "FILLED", "CANCELLED", liveOrderStatusVirtualInitial, liveOrderStatusVirtualExit:
		return true
	default:
		return false
	}
}

func firstNonEmptyMapValue(values ...any) map[string]any {
	for _, value := range values {
		if resolved := cloneMetadata(mapValue(value)); len(resolved) > 0 {
			return resolved
		}
	}
	return nil
}

func executionDispatchSummary(proposalMap map[string]any, order domain.Order, failed bool) map[string]any {
	proposalMeta := cloneMetadata(mapValue(proposalMap["metadata"]))
	expectedPrice := firstPositive(parseFloatValue(proposalMap["limitPrice"]), parseFloatValue(proposalMap["priceHint"]))
	actualPrice := firstPositive(order.Price, expectedPrice)
	priceDriftBps := 0.0
	if expectedPrice > 0 && actualPrice > 0 {
		priceDriftBps = math.Abs(actualPrice/expectedPrice-1) * 10000
	}
	return map[string]any{
		"status":            firstNonEmpty(order.Status, stringValue(proposalMap["status"])),
		"side":              firstNonEmpty(order.Side, stringValue(proposalMap["side"])),
		"symbol":            firstNonEmpty(order.Symbol, stringValue(proposalMap["symbol"])),
		"orderType":         firstNonEmpty(order.Type, stringValue(proposalMap["type"])),
		"quantity":          firstPositive(order.Quantity, parseFloatValue(proposalMap["quantity"])),
		"price":             firstPositive(order.Price, firstPositive(parseFloatValue(proposalMap["limitPrice"]), parseFloatValue(proposalMap["priceHint"]))),
		"executionStrategy": firstNonEmpty(stringValue(proposalMap["executionStrategy"]), stringValue(proposalMeta["executionStrategy"])),
		"executionProfile":  firstNonEmpty(stringValue(proposalMeta["executionProfile"]), stringValue(proposalMap["role"])),
		"executionDecision": stringValue(proposalMeta["executionDecision"]),
		"executionMode":     stringValue(proposalMeta["executionMode"]),
		"timeInForce":       firstNonEmpty(stringValue(proposalMap["timeInForce"]), stringValue(proposalMeta["executionProfileTimeInForce"])),
		"postOnly":          boolValue(proposalMap["postOnly"]) || boolValue(proposalMeta["executionProfilePostOnly"]),
		"reduceOnly":        boolValue(proposalMap["reduceOnly"]) || boolValue(proposalMeta["executionProfileReduceOnly"]),
		"fallback":          boolValue(proposalMeta["fallbackFromTimeout"]),
		"fallbackOrderType": stringValue(proposalMeta["fallbackOrderType"]),
		"spreadBps":         parseFloatValue(proposalMap["spreadBps"]),
		"bookImbalance":     parseFloatValue(proposalMeta["bookImbalance"]),
		"expectedPrice":     expectedPrice,
		"priceDriftBps":     priceDriftBps,
		"decisionContext":   cloneMetadata(mapValue(proposalMeta["executionDecisionContext"])),
		"book":              cloneMetadata(mapValue(proposalMeta["orderBookSnapshot"])),
		"failed":            failed,
	}
}

func executionDispatchTimelineMetadata(proposalMap map[string]any, order domain.Order, failed bool) map[string]any {
	summary := executionDispatchSummary(proposalMap, order, failed)
	summary["orderId"] = order.ID
	summary["reason"] = stringValue(proposalMap["reason"])
	summary["signalKind"] = stringValue(proposalMap["signalKind"])
	return summary
}

func executionTimeoutTimelineMetadata(order domain.Order, cancelledOrder domain.Order) map[string]any {
	proposalMap := mapValue(order.Metadata["executionProposal"])
	metadata := executionDispatchTimelineMetadata(proposalMap, cancelledOrder, false)
	metadata["expiredAt"] = stringValue(order.Metadata["executionExpiresAt"])
	return metadata
}
