package service

import (
	"math"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const livePositionRecoveryStatusClosingPending = "closing-pending"

func (p *Platform) refreshLiveSessionProtectionState(session domain.LiveSession) (domain.LiveSession, error) {
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	openOrders := metadataList(snapshot["openOrders"])
	protectionRecoverySource := strings.TrimSpace(stringValue(snapshot["source"]))
	protectionRecoveryAuthoritative := liveProtectionSnapshotIsAuthoritative(snapshot)
	sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
	position, found, err := p.resolvePaperSessionPositionSnapshot(session.AccountID, sessionSymbol)
	if err != nil {
		return domain.LiveSession{}, err
	}

	protectedOrders := make([]map[string]any, 0)
	stopOrders := make([]map[string]any, 0)
	takeProfitOrders := make([]map[string]any, 0)
	for _, item := range openOrders {
		if sessionSymbol != "" && NormalizeSymbol(stringValue(item["symbol"])) != sessionSymbol {
			continue
		}
		if !isProtectionOrder(item) {
			continue
		}
		protectedOrders = append(protectedOrders, cloneMetadata(item))
		if isStopProtectionOrder(item) {
			stopOrders = append(stopOrders, cloneMetadata(item))
		}
		if isTakeProfitProtectionOrder(item) {
			takeProfitOrders = append(takeProfitOrders, cloneMetadata(item))
		}
	}

	state := cloneMetadata(session.State)
	state["recoveredPosition"] = position
	state["hasRecoveredPosition"] = found
	state["hasRecoveredRealPosition"] = found
	state["hasRecoveredVirtualPosition"] = false
	state["recoveredProtectionOrders"] = protectedOrders
	state["recoveredProtectionCount"] = len(protectedOrders)
	state["recoveredStopOrderCount"] = len(stopOrders)
	state["recoveredTakeProfitOrderCount"] = len(takeProfitOrders)
	state["lastProtectionRecoveryAt"] = time.Now().UTC().Format(time.RFC3339)
	state["lastProtectionRecoverySymbol"] = sessionSymbol
	state["protectionRecoverySource"] = protectionRecoverySource
	state["protectionRecoveryAuthoritative"] = protectionRecoveryAuthoritative
	state["recoveredStopOrder"] = firstMetadataOrEmpty(stopOrders)
	state["recoveredTakeProfitOrder"] = firstMetadataOrEmpty(takeProfitOrders)

	status := "flat"
	switch {
	case found && len(protectedOrders) > 0:
		status = "protected-open-position"
	case found:
		status = "unprotected-open-position"
	}
	state["positionRecoveryStatus"] = status
	state["protectionRecoveryStatus"] = status
	if found {
		appendTimelineEvent(state, "recovery", time.Now().UTC(), "live-position-recovered", map[string]any{
			"symbol":               sessionSymbol,
			"protectionCount":      len(protectedOrders),
			"stopOrderCount":       len(stopOrders),
			"takeProfitOrderCount": len(takeProfitOrders),
			"status":               status,
		})
	}
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) refreshLiveSessionPositionContext(session domain.LiveSession, eventTime time.Time, source string) (domain.LiveSession, error) {
	takeoverActive := shouldActivateLiveRecoveryTakeover(source, session.State)
	applyRecoveryMode := func(state map[string]any) {
		if !isLiveSessionRecoveryCloseOnlyMode(state) {
			return
		}
		state["positionRecoveryStatus"] = liveRecoveryModeCloseOnlyTakeover
		state["lastStrategyEvaluationStatus"] = liveRecoveryModeCloseOnlyTakeover
	}
	persistSnapshot := func(updated domain.LiveSession) (domain.LiveSession, error) {
		if updated.ID == "" {
			return updated, nil
		}
		if err := p.recordLivePositionAccountSnapshot(updated, eventTime, source, ""); err != nil {
			p.logger("service.live_recovery", "session_id", updated.ID).Warn("record live position/account snapshot failed", "error", err)
		}
		return updated, nil
	}
	refreshed, err := p.refreshLiveSessionProtectionState(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	state := cloneMetadata(refreshed.State)
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(state["symbol"]), stringValue(state["lastSymbol"])))
	if symbol == "" {
		return refreshed, nil
	}
	positionSnapshot, foundPosition, err := p.resolveLiveSessionPositionSnapshot(refreshed, symbol)
	if err != nil {
		return domain.LiveSession{}, err
	}
	hasRealPositionContext := foundPosition || math.Abs(parseFloatValue(positionSnapshot["quantity"])) > 0
	account, err := p.store.GetAccount(refreshed.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	reconcileGate := resolveLivePositionReconcileGate(account, symbol, hasRealPositionContext)
	if boolValue(reconcileGate["blocking"]) {
		takeoverActive = true
	}
	state["recoveredPosition"] = positionSnapshot
	state["hasRecoveredPosition"] = foundPosition
	state["hasRecoveredRealPosition"] = foundPosition
	virtualPosition := cloneMetadata(mapValue(state["virtualPosition"]))
	hasVirtualPosition := !hasRealPositionContext && hasActiveVirtualPositionSnapshot(virtualPosition)
	state["hasRecoveredVirtualPosition"] = hasVirtualPosition
	state["lastRecoveredPositionAt"] = eventTime.UTC().Format(time.RFC3339)
	state["positionRecoverySource"] = firstNonEmpty(source, "live-position-refresh")
	if !hasRealPositionContext && !hasVirtualPosition {
		clearLiveWatchdogExitState(state)
		clearLivePositionWatermarks(state)
		delete(state, "livePositionState")
		state["lastLivePositionState"] = map[string]any{}
		state["positionRecoveryStatus"] = "flat"
		applyLivePositionReconcileGateState(state, reconcileGate)
		applyLiveRecoveryTakeoverState(state, takeoverActive)
		applyRecoveryMode(state)
		updated, updateErr := p.store.UpdateLiveSessionState(refreshed.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return persistSnapshot(updated)
	}
	if hasVirtualPosition {
		clearLiveWatchdogExitState(state)
		state["positionRecoveryStatus"] = "monitoring-virtual-position"
	}
	applyLivePositionReconcileGateState(state, reconcileGate)

	version, err := p.resolveCurrentStrategyVersion(refreshed.StrategyID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	parameters, err := p.resolveLiveSessionParameters(refreshed, version)
	if err != nil {
		return domain.LiveSession{}, err
	}
	timeframe := firstNonEmpty(stringValue(state["signalTimeframe"]), stringValue(parameters["signalTimeframe"]))
	signalBarStates := cloneMetadata(mapValue(state["lastStrategyEvaluationSignalBarStates"]))
	if len(signalBarStates) == 0 {
		bootstrapStates, bootstrapErr := p.liveSignalBarStates(symbol, timeframe)
		if bootstrapErr == nil {
			signalBarStates = bootstrapStates
			state["lastStrategyEvaluationSignalBarStates"] = signalBarStates
			state["lastStrategyEvaluationSignalBarBootstrap"] = "market-cache"
		}
	}
	signalBarState, _ := pickSignalBarState(signalBarStates, symbol, timeframe)
	if signalBarState == nil {
		if hasRealPositionContext || hasVirtualPosition {
			watermarks := resolveLivePositionWatermarks(positionSnapshot, state)
			if watermarks.PositionKey == "" {
				clearLivePositionWatermarks(state)
			} else {
				applyLivePositionWatermarks(state, watermarks)
			}
		}
		applyLivePositionReconcileGateState(state, reconcileGate)
		applyLiveRecoveryTakeoverState(state, takeoverActive)
		applyRecoveryMode(state)
		updated, updateErr := p.store.UpdateLiveSessionState(refreshed.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return persistSnapshot(updated)
	}
	marketPrice := firstPositive(parseFloatValue(positionSnapshot["markPrice"]), parseFloatValue(mapValue(signalBarState["current"])["close"]))
	livePositionState := evaluateLivePositionState(parameters, positionSnapshot, signalBarState, marketPrice, state)
	if len(livePositionState) == 0 {
		applyLivePositionReconcileGateState(state, reconcileGate)
		applyLiveRecoveryTakeoverState(state, takeoverActive)
		applyRecoveryMode(state)
		updated, updateErr := p.store.UpdateLiveSessionState(refreshed.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return persistSnapshot(updated)
	}
	state["livePositionState"] = livePositionState
	state["lastLivePositionState"] = livePositionState
	state["lastPositionContextRefreshAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastPositionContextSource"] = firstNonEmpty(source, "live-position-refresh")
	if boolValue(livePositionState["protected"]) && len(metadataList(state["recoveredProtectionOrders"])) > 0 {
		state["positionRecoveryStatus"] = "protected-open-position"
	}

	watchdogExitPending := false
	if hasRealPositionContext {
		watchdogExitPending = syncLiveWatchdogExitState(state, eventTime)
	}

	if !boolValue(reconcileGate["blocking"]) &&
		!watchdogExitPending &&
		boolValue(state["protectionRecoveryAuthoritative"]) &&
		stringValue(state["positionRecoveryStatus"]) == "unprotected-open-position" {
		stopLoss := parseFloatValue(livePositionState["stopLoss"])
		entryPrice := parseFloatValue(livePositionState["entryPrice"])
		quantity := math.Abs(parseFloatValue(positionSnapshot["quantity"]))
		side := strings.ToUpper(strings.TrimSpace(stringValue(livePositionState["side"])))

		if quantity > 0 && stopLoss > 0 && entryPrice > 0 && side != "" {
			var exitSide string
			if side == "LONG" {
				exitSide = "SELL"
			} else {
				exitSide = "BUY"
			}

			breached := false
			if side == "LONG" && marketPrice > 0 && marketPrice <= stopLoss {
				breached = true
			} else if side == "SHORT" && marketPrice > 0 && marketPrice >= stopLoss {
				breached = true
			}

			if breached {
				existingProposal := mapValue(state["lastExecutionProposal"])
				existingReason := stringValue(existingProposal["reason"])

				if existingReason != "sl-breached-fallback" {
					proposal := ExecutionProposal{
						Action:            "risk-exit-fallback",
						Role:              "exit",
						Reason:            "sl-breached-fallback",
						Side:              exitSide,
						Symbol:            stringValue(livePositionState["symbol"]),
						Type:              "MARKET",
						Quantity:          quantity,
						PriceHint:         marketPrice,
						PriceSource:       "fallback-watchdog",
						TimeInForce:       "GTC",
						PostOnly:          false,
						ReduceOnly:        true,
						SignalKind:        "recovery-watchdog",
						DecisionState:     "unprotected",
						SignalBarStateKey: "",
						SpreadBps:         0,
						BestBid:           0,
						BestAsk:           0,
						ExecutionStrategy: "book-aware-v1",
						Status:            "dispatchable",
						Metadata: map[string]any{
							"executionDecision": "direct-dispatch",
							"livePositionState": cloneMetadata(livePositionState),
						},
					}
					state["positionRecoveryStatus"] = livePositionRecoveryStatusClosingPending
					executionProposalMap := assembleLiveExecutionProposalMetadata(domain.LiveSession{
						ID:    refreshed.ID,
						State: state,
					}, version.ID, executionProposalToMap(proposal))
					state["lastExecutionProposal"] = executionProposalMap
					state["lastStrategyIntent"] = executionProposalMap
					state["lastStrategyEvaluationStatus"] = "intent-ready"
					markLiveWatchdogExitState(state, eventTime.UTC().Format(time.RFC3339), "sl-breached-fallback", "", "", "intent-ready")
				}
			}
		}
	}

	applyLivePositionReconcileGateState(state, reconcileGate)
	applyLiveRecoveryTakeoverState(state, takeoverActive)
	applyRecoveryMode(state)
	updated, updateErr := p.store.UpdateLiveSessionState(refreshed.ID, state)
	if updateErr != nil {
		return domain.LiveSession{}, updateErr
	}
	return persistSnapshot(updated)
}

func isProtectionOrder(order map[string]any) bool {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["origType"]), stringValue(order["type"]))))
	if boolValue(order["reduceOnly"]) || boolValue(order["closePosition"]) {
		return true
	}
	return strings.Contains(orderType, "STOP") || strings.Contains(orderType, "TAKE_PROFIT")
}

func isStopProtectionOrder(order map[string]any) bool {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["origType"]), stringValue(order["type"]))))
	return strings.Contains(orderType, "STOP")
}

func isTakeProfitProtectionOrder(order map[string]any) bool {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["origType"]), stringValue(order["type"]))))
	return strings.Contains(orderType, "TAKE_PROFIT")
}

func firstMetadataOrEmpty(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return map[string]any{}
	}
	return cloneMetadata(items[0])
}

func liveProtectionSnapshotIsAuthoritative(snapshot map[string]any) bool {
	source := strings.TrimSpace(stringValue(snapshot["source"]))
	return !strings.EqualFold(source, "platform-live-reconciliation")
}

func syncLiveWatchdogExitState(state map[string]any, eventTime time.Time) bool {
	if state == nil {
		return false
	}

	pendingProposal := firstNonEmptyMapValue(state["lastExecutionProposal"], state["lastStrategyIntent"])
	if isLiveWatchdogFallbackProposal(pendingProposal) {
		markLiveWatchdogExitState(
			state,
			firstNonEmpty(stringValue(state["watchdogExitTriggeredAt"]), eventTime.UTC().Format(time.RFC3339)),
			stringValue(pendingProposal["reason"]),
			"",
			"",
			"intent-ready",
		)
		state["positionRecoveryStatus"] = livePositionRecoveryStatusClosingPending
		return true
	}

	dispatchedIntent := cloneMetadata(mapValue(state["lastDispatchedIntent"]))
	if isLiveWatchdogFallbackProposal(dispatchedIntent) {
		orderID := stringValue(state["lastDispatchedOrderId"])
		orderStatus := strings.ToUpper(strings.TrimSpace(firstNonEmpty(
			stringValue(state["lastSyncedOrderStatus"]),
			stringValue(state["lastDispatchedOrderStatus"]),
		)))
		if orderStatus == "" || !isTerminalOrderStatus(orderStatus) {
			status := "dispatch-pending"
			if orderID != "" {
				status = "order-working"
			}
			markLiveWatchdogExitState(
				state,
				firstNonEmpty(stringValue(state["watchdogExitTriggeredAt"]), stringValue(state["lastDispatchedAt"]), eventTime.UTC().Format(time.RFC3339)),
				stringValue(dispatchedIntent["reason"]),
				orderID,
				orderStatus,
				status,
			)
			state["positionRecoveryStatus"] = livePositionRecoveryStatusClosingPending
			return true
		}
		markLiveWatchdogExitState(
			state,
			firstNonEmpty(stringValue(state["watchdogExitTriggeredAt"]), stringValue(state["lastDispatchedAt"]), eventTime.UTC().Format(time.RFC3339)),
			stringValue(dispatchedIntent["reason"]),
			orderID,
			orderStatus,
			"retry-eligible",
		)
	}

	if activeOrder, ok := activeLiveWatchdogExitOrder(metadataList(state["recoveredProtectionOrders"])); ok {
		markLiveWatchdogExitState(
			state,
			firstNonEmpty(stringValue(state["watchdogExitTriggeredAt"]), stringValue(state["lastProtectionRecoveryAt"]), eventTime.UTC().Format(time.RFC3339)),
			firstNonEmpty(stringValue(state["watchdogExitReason"]), "reduce-only-exit-order"),
			liveWatchdogExitOrderID(activeOrder),
			liveWatchdogExitOrderStatus(activeOrder),
			"order-working",
		)
		state["positionRecoveryStatus"] = livePositionRecoveryStatusClosingPending
		return true
	}

	return false
}

func markLiveWatchdogExitState(state map[string]any, triggeredAt, reason, orderID, orderStatus, status string) {
	if state == nil {
		return
	}
	if triggeredAt != "" {
		state["watchdogExitTriggeredAt"] = triggeredAt
	}
	if reason != "" {
		state["watchdogExitReason"] = reason
	}
	if orderID != "" {
		state["watchdogExitOrderId"] = orderID
	}
	if orderStatus != "" {
		state["watchdogExitOrderStatus"] = orderStatus
	}
	if status != "" {
		state["watchdogExitStatus"] = status
	}
}

func clearLiveWatchdogExitState(state map[string]any) {
	if state == nil {
		return
	}
	delete(state, "watchdogExitTriggeredAt")
	delete(state, "watchdogExitReason")
	delete(state, "watchdogExitOrderId")
	delete(state, "watchdogExitOrderStatus")
	delete(state, "watchdogExitStatus")
}

func isLiveWatchdogFallbackProposal(proposal map[string]any) bool {
	if len(proposal) == 0 {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(stringValue(proposal["reason"])))
	if reason != "sl-breached-fallback" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(stringValue(proposal["signalKind"])), "recovery-watchdog")
}

func activeLiveWatchdogExitOrder(orders []map[string]any) (map[string]any, bool) {
	for _, item := range orders {
		order := cloneMetadata(item)
		if len(order) == 0 {
			continue
		}
		if !boolValue(order["reduceOnly"]) && !boolValue(order["closePosition"]) {
			continue
		}
		if isStopProtectionOrder(order) || isTakeProfitProtectionOrder(order) {
			continue
		}
		status := liveWatchdogExitOrderStatus(order)
		if status != "" && isTerminalOrderStatus(status) {
			continue
		}
		return order, true
	}
	return nil, false
}

func liveWatchdogExitOrderID(order map[string]any) string {
	return firstNonEmpty(stringValue(order["orderId"]), stringValue(order["id"]), stringValue(order["clientOrderId"]))
}

func liveWatchdogExitOrderStatus(order map[string]any) string {
	return strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["status"]), stringValue(order["orderStatus"]))))
}
