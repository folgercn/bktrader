package service

import (
	"math"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) refreshLiveSessionProtectionState(session domain.LiveSession) (domain.LiveSession, error) {
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	openOrders := metadataList(snapshot["openOrders"])
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
	state["recoveredPosition"] = positionSnapshot
	state["hasRecoveredPosition"] = foundPosition
	state["hasRecoveredRealPosition"] = foundPosition
	virtualPosition := cloneMetadata(mapValue(state["virtualPosition"]))
	hasVirtualPosition := !hasRealPositionContext && hasActiveVirtualPositionSnapshot(virtualPosition)
	state["hasRecoveredVirtualPosition"] = hasVirtualPosition
	state["lastRecoveredPositionAt"] = eventTime.UTC().Format(time.RFC3339)
	state["positionRecoverySource"] = firstNonEmpty(source, "live-position-refresh")
	if !hasRealPositionContext && !hasVirtualPosition {
		clearLivePositionWatermarks(state)
		delete(state, "livePositionState")
		state["lastLivePositionState"] = map[string]any{}
		state["positionRecoveryStatus"] = "flat"
		updated, updateErr := p.store.UpdateLiveSessionState(refreshed.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return persistSnapshot(updated)
	}
	if hasVirtualPosition {
		state["positionRecoveryStatus"] = "monitoring-virtual-position"
	}

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
		updated, updateErr := p.store.UpdateLiveSessionState(refreshed.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return persistSnapshot(updated)
	}
	marketPrice := firstPositive(parseFloatValue(positionSnapshot["markPrice"]), parseFloatValue(mapValue(signalBarState["current"])["close"]))
	livePositionState := evaluateLivePositionState(parameters, positionSnapshot, signalBarState, marketPrice, state)
	if len(livePositionState) == 0 {
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
