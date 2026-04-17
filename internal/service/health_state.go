package service

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func safeFloat(value any) float64 {
	var val float64
	switch v := value.(type) {
	case float64:
		val = v
	case float32:
		val = float64(v)
	case int:
		val = float64(v)
	case int64:
		val = float64(v)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		val = parsed
	default:
		return 0
	}
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

func healthDayKey(ts time.Time) string {
	if ts.IsZero() {
		ts = time.Now()
	}
	return ts.In(time.Local).Format("2006-01-02")
}

func ensureHealthSection(container map[string]any, sectionKey string) map[string]any {
	root := cloneMetadata(mapValue(container["healthSummary"]))
	section := cloneMetadata(mapValue(root[sectionKey]))
	root[sectionKey] = section
	container["healthSummary"] = root
	return section
}

func ensureHealthToday(section map[string]any, eventTime time.Time) map[string]any {
	day := healthDayKey(eventTime)
	today := cloneMetadata(mapValue(section["today"]))
	if strings.TrimSpace(stringValue(today["day"])) != day {
		today = map[string]any{"day": day}
	}
	section["today"] = today
	return today
}

func incrementHealthToday(section map[string]any, eventTime time.Time, key string, delta int) {
	if section == nil || delta == 0 {
		return
	}
	today := ensureHealthToday(section, eventTime)
	today[key] = maxIntValue(today[key], 0) + delta
}

func accumulateHealthTodayAverage(section map[string]any, eventTime time.Time, sumKey, countKey, avgKey string, value float64) {
	if section == nil || value == 0 {
		return
	}
	today := ensureHealthToday(section, eventTime)
	today[sumKey] = safeFloat(today[sumKey]) + value
	today[countKey] = maxIntValue(today[countKey], 0) + 1
	count := maxIntValue(today[countKey], 0)
	if count > 0 {
		today[avgKey] = safeFloat(today[sumKey]) / float64(count)
	}
}

func updateRuntimeHealthSummary(state map[string]any, summary map[string]any, eventTime time.Time) {
	if state == nil {
		return
	}
	streamType := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		stringValue(summary["streamType"]),
		inferStreamTypeFromEvent(strings.ToLower(strings.TrimSpace(stringValue(summary["event"])))),
	)))
	if streamType == "" {
		return
	}

	sectionKey := ""
	switch streamType {
	case "trade_tick":
		sectionKey = "tradeTick"
	case "order_book":
		sectionKey = "orderBook"
	default:
		return
	}

	section := ensureHealthSection(state, sectionKey)
	incrementHealthToday(section, eventTime, "eventCount", 1)
	section["streamType"] = streamType
	section["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastSourceKey"] = stringValue(summary["sourceKey"])
	section["lastEvent"] = firstNonEmpty(stringValue(summary["event"]), stringValue(summary["type"]))

	symbol := NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"])))
	if symbol != "" {
		section["lastSymbol"] = symbol
	}

	switch streamType {
	case "trade_tick":
		if price := safeFloat(summary["price"]); price > 0 {
			section["lastPrice"] = price
		}
	case "order_book":
		bestBid := safeFloat(summary["bestBid"])
		bestAsk := safeFloat(summary["bestAsk"])
		bestBidQty := safeFloat(summary["bestBidQty"])
		bestAskQty := safeFloat(summary["bestAskQty"])
		if bestBid > 0 {
			section["lastBestBid"] = bestBid
		}
		if bestAsk > 0 {
			section["lastBestAsk"] = bestAsk
		}
		if bestBidQty > 0 {
			section["lastBestBidQty"] = bestBidQty
		}
		if bestAskQty > 0 {
			section["lastBestAskQty"] = bestAskQty
		}
		if spreadBps := safeFloat(summary["spreadBps"]); spreadBps > 0 {
			section["lastSpreadBps"] = spreadBps
		}
		if imbalance := safeFloat(summary["imbalance"]); imbalance != 0 {
			section["lastBookImbalance"] = imbalance
		}
	}
}

func recordStrategyTriggerHealth(state map[string]any, summary map[string]any, eventTime time.Time) {
	if state == nil {
		return
	}
	section := ensureHealthSection(state, "strategyIngress")
	incrementHealthToday(section, eventTime, "triggeredCount", 1)

	streamType := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		stringValue(summary["streamType"]),
		inferStreamTypeFromEvent(strings.ToLower(strings.TrimSpace(stringValue(summary["event"])))),
	)))
	switch streamType {
	case "trade_tick":
		incrementHealthToday(section, eventTime, "tradeTickTriggeredCount", 1)
	case "order_book":
		incrementHealthToday(section, eventTime, "orderBookTriggeredCount", 1)
	}

	section["lastTriggeredAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastTriggerStreamType"] = streamType
	section["lastTriggerSymbol"] = NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"])))
	section["lastRuntimeEvent"] = firstNonEmpty(stringValue(summary["event"]), stringValue(summary["type"]))
}

func recordStrategySourceGateHealth(state map[string]any, sourceGate map[string]any, eventTime time.Time) {
	if state == nil {
		return
	}
	section := ensureHealthSection(state, "strategyIngress")
	missingCount := len(metadataList(sourceGate["missing"]))
	staleCount := len(metadataList(sourceGate["stale"]))
	section["lastSourceGateAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastSourceGateReady"] = boolValue(sourceGate["ready"])
	section["lastSourceGateMissingCount"] = missingCount
	section["lastSourceGateStaleCount"] = staleCount
	if !boolValue(sourceGate["ready"]) {
		incrementHealthToday(section, eventTime, "sourceGateBlockedCount", 1)
		incrementHealthToday(section, eventTime, "sourceGateMissingCount", missingCount)
		incrementHealthToday(section, eventTime, "sourceGateStaleCount", staleCount)
	}
}

func recordStrategyDecisionHealth(state map[string]any, decision StrategySignalDecision, eventTime time.Time) {
	if state == nil {
		return
	}
	section := ensureHealthSection(state, "strategyIngress")
	incrementHealthToday(section, eventTime, "evaluatedCount", 1)

	action := strings.ToLower(strings.TrimSpace(decision.Action))
	reason := strings.ToLower(strings.TrimSpace(decision.Reason))
	switch action {
	case "wait":
		incrementHealthToday(section, eventTime, "waitCount", 1)
	case "advance-plan":
		incrementHealthToday(section, eventTime, "advancePlanCount", 1)
	}

	section["lastEvaluationAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastDecisionAction"] = decision.Action
	section["lastDecisionReason"] = decision.Reason
	section["lastDecisionState"] = stringValue(decision.Metadata["decisionState"])
	section["lastSignalKind"] = stringValue(decision.Metadata["signalKind"])
	if marketPrice := safeFloat(decision.Metadata["marketPrice"]); marketPrice > 0 {
		section["lastMarketPrice"] = marketPrice
	}

	bestBid := safeFloat(decision.Metadata["bestBid"])
	bestAsk := safeFloat(decision.Metadata["bestAsk"])
	spreadBps := safeFloat(decision.Metadata["spreadBps"])
	imbalance := safeFloat(decision.Metadata["bookImbalance"])
	hasOrderBookContext := bestBid > 0 || bestAsk > 0 || spreadBps > 0 || strings.TrimSpace(stringValue(decision.Metadata["liquidityBias"])) != ""
	if !hasOrderBookContext {
		return
	}

	incrementHealthToday(section, eventTime, "orderBookEvaluatedCount", 1)
	section["lastOrderBookUsedAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastLiquidityBias"] = stringValue(decision.Metadata["liquidityBias"])
	if bestBid > 0 {
		section["lastBestBid"] = bestBid
	}
	if bestAsk > 0 {
		section["lastBestAsk"] = bestAsk
	}
	if spreadBps > 0 {
		section["lastSpreadBps"] = spreadBps
		accumulateHealthTodayAverage(section, eventTime, "spreadBpsSum", "spreadBpsSampleCount", "avgSpreadBps", spreadBps)
	}
	if imbalance != 0 {
		section["lastBookImbalance"] = imbalance
		accumulateHealthTodayAverage(section, eventTime, "bookImbalanceSum", "bookImbalanceSampleCount", "avgBookImbalance", imbalance)
	}
	if reason == "spread-too-wide" || reason == "bias-unfavorable" {
		incrementHealthToday(section, eventTime, "orderBookBlockedCount", 1)
	}
}

func recordStrategyDecisionErrorHealth(state map[string]any, eventTime time.Time, err error) {
	if state == nil || err == nil {
		return
	}
	section := ensureHealthSection(state, "strategyIngress")
	incrementHealthToday(section, eventTime, "decisionErrorCount", 1)
	section["lastDecisionErrorAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastDecisionError"] = err.Error()
}

func recordExecutionPlanningHealth(state map[string]any, executionProposal map[string]any, eventTime time.Time) {
	if state == nil || len(executionProposal) == 0 {
		return
	}
	section := ensureHealthSection(state, "execution")
	incrementHealthToday(section, eventTime, "intentReadyCount", 1)

	status := strings.ToLower(strings.TrimSpace(stringValue(executionProposal["status"])))
	if status == "dispatchable" {
		incrementHealthToday(section, eventTime, "dispatchableIntentCount", 1)
	}

	section["lastIntentAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastProposalStatus"] = status
	section["lastExecutionStrategy"] = stringValue(executionProposal["executionStrategy"])
	section["lastExecutionDecision"] = stringValue(mapValue(executionProposal["metadata"])["executionDecision"])
}

func recordExecutionPlanningErrorHealth(state map[string]any, eventTime time.Time, err error) {
	if state == nil || err == nil {
		return
	}
	section := ensureHealthSection(state, "execution")
	incrementHealthToday(section, eventTime, "planningErrorCount", 1)
	section["lastPlanningErrorAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastPlanningError"] = err.Error()
}

func recordExecutionDispatchHealth(state map[string]any, order domain.Order, eventTime time.Time, dispatchErr error) {
	if state == nil {
		return
	}
	section := ensureHealthSection(state, "execution")
	section["lastDispatchAt"] = eventTime.UTC().Format(time.RFC3339)
	if order.ID != "" || strings.TrimSpace(order.Status) != "" {
		incrementHealthToday(section, eventTime, "dispatchCount", 1)
		if order.ID != "" {
			section["lastOrderId"] = order.ID
		}
		section["lastOrderStatus"] = order.Status
		section["lastOrderType"] = order.Type
		section["lastOrderSymbol"] = NormalizeSymbol(order.Symbol)
	}
	if dispatchErr != nil {
		incrementHealthToday(section, eventTime, "dispatchErrorCount", 1)
		section["lastDispatchErrorAt"] = eventTime.UTC().Format(time.RFC3339)
		section["lastDispatchError"] = dispatchErr.Error()
		return
	}
	delete(section, "lastDispatchErrorAt")
	delete(section, "lastDispatchError")
}

func recordExecutionSyncAttemptHealth(state map[string]any, eventTime time.Time) {
	if state == nil {
		return
	}
	section := ensureHealthSection(state, "execution")
	incrementHealthToday(section, eventTime, "syncAttemptCount", 1)
	section["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
}

func recordExecutionSyncResultHealth(state map[string]any, eventTime time.Time, status string, syncErr error) {
	if state == nil {
		return
	}
	section := ensureHealthSection(state, "execution")
	if syncErr != nil {
		incrementHealthToday(section, eventTime, "syncErrorCount", 1)
		section["lastSyncErrorAt"] = eventTime.UTC().Format(time.RFC3339)
		section["lastSyncError"] = syncErr.Error()
		return
	}
	incrementHealthToday(section, eventTime, "syncSuccessCount", 1)
	section["lastSyncSuccessAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastSyncedOrderStatus"] = status
	delete(section, "lastSyncErrorAt")
	delete(section, "lastSyncError")
}

func updateAccountSyncSuccessHealth(account *domain.Account, syncedAt time.Time, previousSuccessAt time.Time) {
	if account == nil {
		return
	}
	account.Metadata = cloneMetadata(account.Metadata)
	section := ensureHealthSection(account.Metadata, "accountSync")
	incrementHealthToday(section, syncedAt, "syncCount", 1)

	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	section["lastAttemptAt"] = syncedAt.UTC().Format(time.RFC3339)
	section["lastSuccessAt"] = syncedAt.UTC().Format(time.RFC3339)
	section["lastStatus"] = firstNonEmpty(stringValue(snapshot["syncStatus"]), "SYNCED")
	section["lastSource"] = firstNonEmpty(stringValue(snapshot["source"]), stringValue(snapshot["adapterKey"]))
	section["positionCount"] = maxIntValue(snapshot["positionCount"], 0)
	section["openOrderCount"] = maxIntValue(snapshot["openOrderCount"], 0)
	section["consecutiveErrorCount"] = 0
	if !previousSuccessAt.IsZero() && syncedAt.After(previousSuccessAt) {
		section["lastSyncGapSeconds"] = int(syncedAt.Sub(previousSuccessAt).Seconds())
	}
	delete(section, "lastError")
	delete(section, "lastErrorAt")
}

func updateAccountSyncFailureHealth(account *domain.Account, attemptedAt time.Time, err error) {
	if account == nil || err == nil {
		return
	}
	account.Metadata = cloneMetadata(account.Metadata)
	section := ensureHealthSection(account.Metadata, "accountSync")
	incrementHealthToday(section, attemptedAt, "errorCount", 1)
	section["lastAttemptAt"] = attemptedAt.UTC().Format(time.RFC3339)
	section["lastErrorAt"] = attemptedAt.UTC().Format(time.RFC3339)
	section["lastError"] = err.Error()
	section["consecutiveErrorCount"] = maxIntValue(section["consecutiveErrorCount"], 0) + 1
}

// recordSignalSymbolMismatch records a cross-symbol contamination detection event.
// This is called when a signal's symbol doesn't match the live session's expected symbol.
func recordSignalSymbolMismatch(state map[string]any, triggerSymbol, sessionSymbol string, eventTime time.Time) {
	if state == nil {
		return
	}
	health := cloneMetadata(mapValue(state["healthSummary"]))
	section := cloneMetadata(mapValue(health["signalIsolation"]))
	section["lastMismatchAt"] = eventTime.UTC().Format(time.RFC3339)
	section["lastTriggerSymbol"] = triggerSymbol
	section["lastSessionSymbol"] = sessionSymbol
	section["mismatchCount"] = maxIntValue(section["mismatchCount"], 0) + 1
	health["signalIsolation"] = section
	state["healthSummary"] = health
}
