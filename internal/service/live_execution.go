package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
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
		case "sl-slippage-protected":
			incrementInt("slProtectedDispatchCount")
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
	evaluateExecutionQuality(state)
}

// evaluateExecutionQuality 基于累计执行统计数据评估执行质量等级。
// 输出 "good" / "degraded" / "poor" 三档评级及原因列表，写入 session state。
func evaluateExecutionQuality(state map[string]any) {
	stats := mapValue(state["executionEventStats"])
	if len(stats) == 0 {
		return
	}
	avgDrift := parseFloatValue(stats["avgPriceDriftBps"])
	avgSpread := parseFloatValue(stats["avgProposalSpreadBps"])
	filledCount := maxIntValue(stats["filledCount"], 0)
	rejectedCount := maxIntValue(stats["rejectedCount"], 0)
	cancelledCount := maxIntValue(stats["cancelledCount"], 0)

	quality := "good"
	reasons := make([]string, 0)

	// Drift quality
	if avgDrift > 10 {
		quality = "poor"
		reasons = append(reasons, fmt.Sprintf("high-drift:%.1fbps", avgDrift))
	} else if avgDrift > 5 {
		quality = "degraded"
		reasons = append(reasons, fmt.Sprintf("elevated-drift:%.1fbps", avgDrift))
	}

	// Spread quality
	if avgSpread > 15 {
		quality = "poor"
		reasons = append(reasons, fmt.Sprintf("wide-spread:%.1fbps", avgSpread))
	} else if avgSpread > 8 {
		if quality != "poor" {
			quality = "degraded"
		}
		reasons = append(reasons, fmt.Sprintf("elevated-spread:%.1fbps", avgSpread))
	}

	// Rejection rate
	totalDispatched := filledCount + rejectedCount
	if totalDispatched > 3 {
		rejectionRate := float64(rejectedCount) / float64(totalDispatched)
		if rejectionRate > 0.3 {
			quality = "poor"
			reasons = append(reasons, fmt.Sprintf("high-rejection:%.0f%%", rejectionRate*100))
		}
	}
	if cancelledCount > 0 {
		cancelRate := float64(cancelledCount) / float64(maxInt(totalDispatched+cancelledCount, 1))
		if cancelRate >= 0.5 && quality == "good" {
			quality = "degraded"
		}
		if cancelRate >= 0.5 {
			reasons = append(reasons, fmt.Sprintf("high-cancel:%.0f%%", cancelRate*100))
		}
	}

	// SL slippage protection events
	slProtectedCount := maxIntValue(stats["slProtectedDispatchCount"], 0)
	if slProtectedCount > 0 {
		reasons = append(reasons, fmt.Sprintf("sl-protected:%d", slProtectedCount))
	}

	state["executionQuality"] = quality
	state["executionQualityReasons"] = reasons
	state["executionQualityEvaluatedAt"] = time.Now().UTC().Format(time.RFC3339)
}

func (p *Platform) SyncLiveSession(sessionID string) (domain.LiveSession, error) {
	logger := p.logger("service.live_execution", "session_id", sessionID)
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		logger.Warn("load live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	logger.Debug("syncing latest live session order")
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
	logger := p.logger("service.live_execution", "session_id", sessionID)
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		logger.Warn("load live session failed", "error", err)
		return domain.Order{}, err
	}
	logger.Info("dispatching live session intent")
	return p.dispatchLiveSessionIntent(session)
}

func isRecoveryTriggeredPassiveCloseProposal(proposalMap map[string]any) bool {
	if len(proposalMap) == 0 {
		return false
	}
	metadata := mapValue(proposalMap["metadata"])
	if boolValue(metadata["recoveryTriggered"]) {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(stringValue(proposalMap["role"])), "exit") {
		return false
	}
	if !boolValue(proposalMap["reduceOnly"]) && !boolValue(metadata["reduceOnly"]) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(stringValue(proposalMap["signalKind"])), "recovery-watchdog")
}

func buildLiveExecutionContextMetadata(session domain.LiveSession, strategyVersionID string, proposalMap map[string]any, metadata map[string]any) map[string]any {
	context := cloneMetadata(mapValue(metadata["executionContext"]))
	sessionContext := cloneMetadata(mapValue(session.State["lastStrategyEvaluationContext"]))
	if context == nil {
		context = map[string]any{}
	}
	for key, value := range sessionContext {
		if stringValue(context[key]) == "" {
			context[key] = value
		}
	}
	if value := firstNonEmpty(
		stringValue(context["strategyVersionId"]),
		stringValue(sessionContext["strategyVersionId"]),
		stringValue(metadata["strategyVersionId"]),
		strategyVersionID,
		stringValue(session.State["strategyVersionId"]),
	); value != "" {
		context["strategyVersionId"] = value
	}
	if value := NormalizeSymbol(firstNonEmpty(
		stringValue(context["symbol"]),
		stringValue(sessionContext["symbol"]),
		stringValue(proposalMap["symbol"]),
		stringValue(session.State["symbol"]),
		stringValue(session.State["lastSymbol"]),
	)); value != "" {
		context["symbol"] = value
	}
	if value := firstNonEmpty(
		stringValue(context["signalTimeframe"]),
		stringValue(sessionContext["signalTimeframe"]),
		stringValue(session.State["signalTimeframe"]),
	); value != "" {
		context["signalTimeframe"] = value
	}
	if value := firstNonEmpty(
		stringValue(context["executionDataSource"]),
		stringValue(sessionContext["executionDataSource"]),
		stringValue(session.State["executionDataSource"]),
	); value != "" {
		context["executionDataSource"] = value
	}
	if value := firstNonEmpty(
		stringValue(context["executionMode"]),
		stringValue(sessionContext["executionMode"]),
		stringValue(metadata["executionMode"]),
		stringValue(session.State["executionMode"]),
		"live",
	); value != "" {
		context["executionMode"] = value
	}
	if value := firstNonEmpty(
		stringValue(context["strategyEngineKey"]),
		stringValue(sessionContext["strategyEngineKey"]),
		stringValue(session.State["strategyEngine"]),
	); value != "" {
		context["strategyEngineKey"] = value
	}
	return context
}

func assembleLiveExecutionProposalMetadata(session domain.LiveSession, strategyVersionID string, proposalMap map[string]any) map[string]any {
	if len(proposalMap) == 0 {
		return map[string]any{}
	}
	normalized := cloneMetadata(proposalMap)
	metadata := cloneMetadata(mapValue(normalized["metadata"]))
	if metadata == nil {
		metadata = map[string]any{}
	}
	strategyVersionID = firstNonEmpty(
		strategyVersionID,
		stringValue(normalized["strategyVersionId"]),
		stringValue(metadata["strategyVersionId"]),
		stringValue(mapValue(session.State["lastStrategyEvaluationContext"])["strategyVersionId"]),
		stringValue(session.State["strategyVersionId"]),
	)
	if strategyVersionID != "" {
		normalized["strategyVersionId"] = strategyVersionID
		metadata["strategyVersionId"] = strategyVersionID
	}
	if runtimeSessionID := firstNonEmpty(
		stringValue(metadata["runtimeSessionId"]),
		stringValue(session.State["signalRuntimeSessionId"]),
		stringValue(session.State["lastSignalRuntimeSessionId"]),
	); runtimeSessionID != "" {
		metadata["runtimeSessionId"] = runtimeSessionID
	}
	if session.ID != "" {
		metadata["liveSessionId"] = session.ID
	}
	// executionMode here describes the live-session execution pipeline semantics.
	// It is intentionally separate from account binding modes such as mock/rest.
	metadata["executionMode"] = firstNonEmpty(stringValue(metadata["executionMode"]), "live")
	metadata["executionContext"] = buildLiveExecutionContextMetadata(session, strategyVersionID, normalized, metadata)
	if isRecoveryTriggeredPassiveCloseProposal(normalized) {
		metadata["recoveryTriggered"] = true
		if value := firstNonEmpty(stringValue(metadata["positionRecoveryStatus"]), stringValue(session.State["positionRecoveryStatus"])); value != "" {
			metadata["positionRecoveryStatus"] = value
		}
		if value := firstNonEmpty(stringValue(metadata["positionRecoverySource"]), stringValue(session.State["positionRecoverySource"])); value != "" {
			metadata["positionRecoverySource"] = value
		}
	}
	normalized["metadata"] = metadata
	return normalized
}

func validateLiveExecutionProposalMetadata(session domain.LiveSession, proposalMap map[string]any) error {
	if len(proposalMap) == 0 {
		return fmt.Errorf("live session %s has no execution proposal metadata", session.ID)
	}
	metadata := mapValue(proposalMap["metadata"])
	strategyVersionID := firstNonEmpty(stringValue(proposalMap["strategyVersionId"]), stringValue(metadata["strategyVersionId"]))
	if strings.TrimSpace(strategyVersionID) == "" {
		return fmt.Errorf("live session %s execution proposal missing strategyVersionId", session.ID)
	}
	executionContext := mapValue(metadata["executionContext"])
	if len(executionContext) == 0 {
		return fmt.Errorf("live session %s execution proposal missing execution context", session.ID)
	}
	missing := make([]string, 0, 5)
	if strings.TrimSpace(stringValue(executionContext["strategyVersionId"])) == "" {
		missing = append(missing, "strategyVersionId")
	}
	if NormalizeSymbol(stringValue(executionContext["symbol"])) == "" {
		missing = append(missing, "symbol")
	}
	if strings.TrimSpace(stringValue(executionContext["signalTimeframe"])) == "" {
		missing = append(missing, "signalTimeframe")
	}
	if strings.TrimSpace(stringValue(executionContext["executionDataSource"])) == "" {
		missing = append(missing, "executionDataSource")
	}
	if strings.TrimSpace(stringValue(executionContext["executionMode"])) == "" {
		missing = append(missing, "executionMode")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"live session %s execution proposal missing execution context fields: %s",
			session.ID,
			strings.Join(missing, ", "),
		)
	}
	// Recovery-triggered passive closes stay fail-closed here: if the current session
	// no longer has an explicit runtime linkage, we do not recover lineage indirectly
	// from historical evaluation context and we block dispatch instead.
	if isRecoveryTriggeredPassiveCloseProposal(proposalMap) && strings.TrimSpace(stringValue(metadata["runtimeSessionId"])) == "" {
		return fmt.Errorf("live session %s recovery close proposal missing runtimeSessionId", session.ID)
	}
	return nil
}

func shouldBlockAutoDispatchForRecoveryIntent(session domain.LiveSession, intent map[string]any) bool {
	if isLiveSessionBlockedByPositionReconcileGate(session.State) {
		return true
	}
	recoveryActions := currentLiveRecoveryActionMatrix(session.State)
	if boolValue(session.State["recoveryTakeoverActive"]) &&
		!recoveryActions.AutoDispatch &&
		!shouldAllowAutoDispatchRecoveredClose(session, intent, recoveryActions) {
		return true
	}
	if !isRecoveryTriggeredPassiveCloseProposal(intent) {
		return false
	}
	status := strings.TrimSpace(stringValue(session.State["positionRecoveryStatus"]))
	if status == "" || status == "unprotected-open-position" {
		return true
	}
	if !boolValue(session.State["hasRecoveredPosition"]) && !boolValue(session.State["hasRecoveredRealPosition"]) {
		return true
	}
	proposalMap := assembleLiveExecutionProposalMetadata(session, "", intent)
	return validateLiveExecutionProposalMetadata(session, proposalMap) != nil
}

// Verified takeover sessions may auto-dispatch close intents so recovered
// positions can be exited without reopening manual-review debt. Entry/protection
// intents remain blocked until the recovery state clears.
func shouldAllowAutoDispatchRecoveredClose(session domain.LiveSession, intent map[string]any, recoveryActions liveRecoveryActionMatrix) bool {
	if !boolValue(session.State["recoveryTakeoverActive"]) || recoveryActions.CloseExistingPosition == false {
		return false
	}
	if strings.TrimSpace(stringValue(session.State["recoveryTakeoverState"])) != liveRecoveryTakeoverStateMonitoring {
		return false
	}
	switch strings.TrimSpace(stringValue(session.State["positionReconcileGateStatus"])) {
	case livePositionReconcileGateStatusVerified, livePositionReconcileGateStatusAdopted:
	default:
		return false
	}
	if resolveLiveRecoveryIntentAction(intent) != "close-existing-position" {
		return false
	}
	if !boolValue(session.State["hasRecoveredPosition"]) && !boolValue(session.State["hasRecoveredRealPosition"]) {
		return false
	}
	proposalMap := assembleLiveExecutionProposalMetadata(session, "", intent)
	return validateLiveExecutionProposalMetadata(session, proposalMap) == nil
}

func shouldBlockAutoDispatchForLiveEntryTradeLimit(session domain.LiveSession, intent map[string]any) bool {
	return validateLiveSignalBarEntryTradeLimit(session, intent) != nil
}

func validateLiveSignalBarEntryTradeLimit(session domain.LiveSession, proposalMap map[string]any) error {
	if !liveEntryCountsTowardSignalBarLimit(proposalMap) {
		return nil
	}
	maxTradesPerBar := maxIntValue(session.State["max_trades_per_bar"], 0)
	if maxTradesPerBar <= 0 {
		return nil
	}
	currentBarKey := liveProposalSignalBarTradeLimitKey(proposalMap)
	if currentBarKey == "" || currentBarKey != stringValue(session.State["lastSignalBarStateKey"]) {
		return nil
	}
	currentCount := maxIntValue(session.State["sessionReentryCount"], 0)
	if currentCount < maxTradesPerBar {
		return nil
	}
	return fmt.Errorf(
		"live session %s reached max_trades_per_bar=%d for signal bar %s",
		session.ID,
		maxTradesPerBar,
		currentBarKey,
	)
}

func liveEntryCountsTowardSignalBarLimit(proposalMap map[string]any) bool {
	if !strings.EqualFold(strings.TrimSpace(stringValue(proposalMap["role"])), "entry") {
		return false
	}
	switch normalizeStrategyReasonTag(stringValue(proposalMap["reason"])) {
	case "zero-initial-reentry", "sl-reentry", "pt-reentry":
		return true
	default:
		return false
	}
}

func liveProposalSignalBarTradeLimitKey(proposalMap map[string]any) string {
	if key := strings.TrimSpace(stringValue(proposalMap[liveSignalBarTradeLimitKeyField])); key != "" {
		return key
	}
	if key := effectiveSignalBarTradeLimitKey(mapValue(proposalMap["metadata"])); key != "" {
		return key
	}
	return strings.TrimSpace(stringValue(proposalMap["signalBarStateKey"]))
}

func liveProposalSignalBarStateKey(proposalMap map[string]any) string {
	if key := strings.TrimSpace(stringValue(proposalMap["signalBarStateKey"])); key != "" {
		return key
	}
	return strings.TrimSpace(stringValue(mapValue(proposalMap["metadata"])["signalBarStateKey"]))
}

const (
	liveDispatchRejectedEntrySubmissionGuard         = "ENTRY_SUBMISSION_GUARD_BLOCKED"
	liveEntrySubmissionSlippageGuardKey              = "entrySubmissionSlippageGuard"
	liveEntrySubmissionDefaultMaxBookAge             = 500 * time.Millisecond
	liveEntrySubmissionDefaultMinTopBookCoverage     = 0.5
	liveEntrySubmissionDefaultMaxSourceDivergenceBps = 8.0
)

func (p *Platform) applyLiveEntrySubmissionSlippageGuard(session domain.LiveSession, proposalMap map[string]any, eventTime time.Time) (map[string]any, error) {
	if !liveEntrySubmissionSlippageGuardApplies(proposalMap) {
		return proposalMap, nil
	}
	maxSlippageBps := liveEntrySubmissionMaxSlippageBps(session, proposalMap)
	if maxSlippageBps <= 0 {
		return proposalMap, nil
	}

	checked := cloneMetadata(proposalMap)
	metadata := cloneMetadata(mapValue(checked["metadata"]))
	if metadata == nil {
		metadata = map[string]any{}
	}
	side := strings.ToUpper(strings.TrimSpace(stringValue(checked["side"])))
	expectedPrice := firstPositive(parseFloatValue(checked["limitPrice"]), parseFloatValue(checked["priceHint"]))
	guard := map[string]any{
		"status":         "evaluating",
		"checkedAt":      eventTime.UTC().Format(time.RFC3339Nano),
		"maxSlippageBps": maxSlippageBps,
		"side":           side,
		"expectedPrice":  expectedPrice,
	}
	block := func(reason string) (map[string]any, error) {
		guard["status"] = "blocked"
		guard["reason"] = reason
		metadata[liveEntrySubmissionSlippageGuardKey] = guard
		checked["metadata"] = metadata
		return checked, fmt.Errorf("live session %s entry submission slippage guard blocked: %s", session.ID, reason)
	}

	if expectedPrice <= 0 {
		return block("missing-expected-price")
	}
	if side != "BUY" && side != "SELL" {
		return block("unsupported-entry-side")
	}
	book, bookMetadata, ok := p.latestLiveOrderBookStatsForProposal(session, checked)
	if !ok {
		return block("missing-current-order-book")
	}
	maxBookAge := liveEntrySubmissionMaxBookAge(session, checked)
	if maxBookAge > 0 {
		bookAt := parseOptionalRFC3339(stringValue(bookMetadata["lastEventAt"]))
		if bookAt.IsZero() {
			guard["currentBook"] = bookMetadata
			return block("missing-current-order-book-timestamp")
		}
		bookAge := time.Duration(0)
		if eventTime.After(bookAt) {
			bookAge = eventTime.Sub(bookAt)
		}
		guard["bookAgeMs"] = float64(bookAge) / float64(time.Millisecond)
		guard["maxBookAgeMs"] = float64(maxBookAge) / float64(time.Millisecond)
		if bookAge > maxBookAge {
			guard["currentBook"] = bookMetadata
			return block("stale-current-order-book")
		}
	}
	currentPrice := book.bestAsk
	topDepthQty := book.bestAskQty
	if side == "SELL" {
		currentPrice = book.bestBid
		topDepthQty = book.bestBidQty
	}
	guard["currentPrice"] = currentPrice
	guard["currentBook"] = bookMetadata
	guard["currentSpreadBps"] = book.spreadBps
	if currentPrice <= 0 {
		return block("missing-current-executable-price")
	}
	quantity := parseFloatValue(checked["quantity"])
	guard["quantity"] = quantity
	guard["topDepthQty"] = topDepthQty
	if quantity <= 0 {
		return block("missing-entry-quantity")
	}
	if minCoverage := liveEntrySubmissionMinTopBookCoverage(session, checked); minCoverage > 0 {
		coverage := 0.0
		if topDepthQty > 0 {
			coverage = topDepthQty / quantity
		}
		guard["topDepthCoverage"] = coverage
		guard["minTopDepthCoverage"] = minCoverage
		if topDepthQty <= 0 {
			return block("missing-top-book-depth")
		}
		if executionGuardBelow(coverage, minCoverage) {
			return block("top-book-coverage-too-thin")
		}
	}
	if maxDivergenceBps := liveEntrySubmissionMaxSourceDivergenceBps(session, checked); maxDivergenceBps > 0 {
		sourceDivergences, maxDivergence := liveEntrySubmissionSourceDivergences(currentPrice, bookMetadata, eventTime, maxBookAge)
		if len(sourceDivergences) > 0 {
			guard["sourceDivergences"] = sourceDivergences
			guard["sourceDivergenceBps"] = maxDivergence
			guard["maxSourceDivergenceBps"] = maxDivergenceBps
			if executionGuardExceeds(maxDivergence, maxDivergenceBps) {
				return block("market-source-divergence-too-wide")
			}
		}
	}

	adverseDriftBps := liveEntryAdverseSubmissionDriftBps(side, expectedPrice, currentPrice)
	guard["adverseDriftBps"] = adverseDriftBps
	if executionGuardExceeds(adverseDriftBps, maxSlippageBps) {
		guard["status"] = "blocked"
		guard["reason"] = "slippage-too-wide"
		metadata[liveEntrySubmissionSlippageGuardKey] = guard
		checked["metadata"] = metadata
		return checked, fmt.Errorf(
			"live session %s entry submission slippage %.4fbps exceeds max %.4fbps",
			session.ID,
			adverseDriftBps,
			maxSlippageBps,
		)
	}

	guard["status"] = "passed"
	metadata[liveEntrySubmissionSlippageGuardKey] = guard
	checked["metadata"] = metadata
	return checked, nil
}

func liveEntrySubmissionMaxBookAge(session domain.LiveSession, proposalMap map[string]any) time.Duration {
	maxAgeMs := liveEntrySubmissionGuardConfigValue(session, proposalMap, "executionEntryMaxBookAgeMs")
	if maxAgeMs <= 0 {
		return liveEntrySubmissionDefaultMaxBookAge
	}
	return time.Duration(maxAgeMs * float64(time.Millisecond))
}

func liveEntrySubmissionMinTopBookCoverage(session domain.LiveSession, proposalMap map[string]any) float64 {
	coverage := liveEntrySubmissionGuardConfigValue(session, proposalMap, "executionEntryMinTopBookCoverage")
	if coverage <= 0 {
		return liveEntrySubmissionDefaultMinTopBookCoverage
	}
	return coverage
}

func liveEntrySubmissionMaxSourceDivergenceBps(session domain.LiveSession, proposalMap map[string]any) float64 {
	divergence := liveEntrySubmissionGuardConfigValue(session, proposalMap, "executionEntryMaxSourceDivergenceBps")
	if divergence <= 0 {
		return liveEntrySubmissionDefaultMaxSourceDivergenceBps
	}
	return divergence
}

func liveEntrySubmissionGuardConfigValue(session domain.LiveSession, proposalMap map[string]any, key string) float64 {
	metadata := mapValue(proposalMap["metadata"])
	executionContext := mapValue(metadata["executionContext"])
	return firstPositive(
		parseFloatValue(session.State[key]),
		firstPositive(
			parseFloatValue(executionContext[key]),
			parseFloatValue(metadata[key]),
		),
	)
}

func liveEntrySubmissionSourceDivergences(currentPrice float64, bookMetadata map[string]any, eventTime time.Time, maxSourceAge time.Duration) ([]map[string]any, float64) {
	if currentPrice <= 0 {
		return nil, 0
	}
	sources := []struct {
		name     string
		priceKey string
		timeKey  string
	}{
		{name: "trade_tick", priceKey: "tradeTickPrice", timeKey: "tradeTickLastEventAt"},
		{name: "signal_bar", priceKey: "signalBarPrice", timeKey: "signalBarLastEventAt"},
	}
	out := make([]map[string]any, 0, len(sources))
	maxDivergence := 0.0
	for _, source := range sources {
		price := parseFloatValue(bookMetadata[source.priceKey])
		if price <= 0 {
			continue
		}
		sourceAtRaw := stringValue(bookMetadata[source.timeKey])
		sourceAt := parseOptionalRFC3339(sourceAtRaw)
		sourceAge := time.Duration(0)
		if !sourceAt.IsZero() && eventTime.After(sourceAt) {
			sourceAge = eventTime.Sub(sourceAt)
		}
		if maxSourceAge > 0 && (sourceAt.IsZero() || sourceAge > maxSourceAge) {
			continue
		}
		divergence := math.Abs(currentPrice/price-1) * 10000
		if divergence > maxDivergence {
			maxDivergence = divergence
		}
		out = append(out, map[string]any{
			"source":        source.name,
			"price":         price,
			"lastEventAt":   sourceAtRaw,
			"sourceAgeMs":   float64(sourceAge) / float64(time.Millisecond),
			"divergenceBps": divergence,
		})
	}
	return out, maxDivergence
}

func liveEntrySubmissionSlippageGuardApplies(proposalMap map[string]any) bool {
	if !strings.EqualFold(strings.TrimSpace(stringValue(proposalMap["role"])), "entry") {
		return false
	}
	metadata := mapValue(proposalMap["metadata"])
	if boolValue(proposalMap["reduceOnly"]) || boolValue(metadata["reduceOnly"]) {
		return false
	}
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["type"]), "MARKET")))
	return orderType == "MARKET"
}

func liveEntrySubmissionMaxSlippageBps(session domain.LiveSession, proposalMap map[string]any) float64 {
	metadata := mapValue(proposalMap["metadata"])
	executionContext := mapValue(metadata["executionContext"])
	return firstPositive(
		parseFloatValue(session.State["executionEntryMaxSlippageBps"]),
		firstPositive(
			parseFloatValue(executionContext["executionEntryMaxSlippageBps"]),
			parseFloatValue(metadata["executionEntryMaxSlippageBps"]),
		),
	)
}

func (p *Platform) latestLiveOrderBookStatsForProposal(session domain.LiveSession, proposalMap map[string]any) (orderBookDecisionStats, map[string]any, bool) {
	metadata := mapValue(proposalMap["metadata"])
	runtimeSessionID := firstNonEmpty(
		stringValue(session.State["signalRuntimeSessionId"]),
		stringValue(metadata["runtimeSessionId"]),
		stringValue(session.State["lastSignalRuntimeSessionId"]),
	)
	if strings.TrimSpace(runtimeSessionID) == "" {
		return orderBookDecisionStats{}, nil, false
	}
	runtimeSession, err := p.GetSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return orderBookDecisionStats{}, nil, false
	}
	sourceStates := mapValue(runtimeSession.State["sourceStates"])
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(proposalMap["symbol"]), stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
	return latestOrderBookStatsFromSourceStates(runtimeSessionID, symbol, sourceStates)
}

func latestOrderBookStatsFromSourceStates(runtimeSessionID, symbol string, sourceStates map[string]any) (orderBookDecisionStats, map[string]any, bool) {
	var selected orderBookDecisionStats
	var selectedMeta map[string]any
	var selectedAt time.Time
	found := false
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
		bestBid := parseFloatValue(summary["bestBid"])
		bestAsk := parseFloatValue(summary["bestAsk"])
		if bestBid <= 0 && bestAsk <= 0 {
			continue
		}
		eventAt := parseOptionalRFC3339(stringValue(entry["lastEventAt"]))
		if eventAt.IsZero() {
			continue
		}
		if found && !eventAt.After(selectedAt) {
			continue
		}
		spreadBps := parseFloatValue(summary["spreadBps"])
		if spreadBps <= 0 && bestBid > 0 && bestAsk > 0 {
			mid := (bestBid + bestAsk) / 2
			if mid > 0 {
				spreadBps = (bestAsk - bestBid) / mid * 10000
			}
		}
		selected = orderBookDecisionStats{
			bestBid:    bestBid,
			bestAsk:    bestAsk,
			bestBidQty: parseFloatValue(summary["bestBidQty"]),
			bestAskQty: parseFloatValue(summary["bestAskQty"]),
			spreadBps:  spreadBps,
			imbalance:  parseFloatValue(summary["bookImbalance"]),
			bias:       stringValue(summary["liquidityBias"]),
		}
		selectedMeta = map[string]any{
			"runtimeSessionId": runtimeSessionID,
			"sourceKey":        stringValue(entry["sourceKey"]),
			"symbol":           NormalizeSymbol(stringValue(entry["symbol"])),
			"lastEventAt":      stringValue(entry["lastEventAt"]),
			"bestBid":          bestBid,
			"bestAsk":          bestAsk,
			"bestBidQty":       selected.bestBidQty,
			"bestAskQty":       selected.bestAskQty,
			"spreadBps":        spreadBps,
		}
		selectedAt = eventAt
		found = true
	}
	if found {
		enrichLiveOrderBookMetadataWithSourcePrices(selectedMeta, symbol, sourceStates)
	}
	return selected, selectedMeta, found
}

type liveSourcePriceSnapshot struct {
	source      string
	price       float64
	lastEventAt string
}

func enrichLiveOrderBookMetadataWithSourcePrices(metadata map[string]any, symbol string, sourceStates map[string]any) {
	if len(metadata) == 0 {
		return
	}
	if trade := latestLiveSourcePriceSnapshot(sourceStates, symbol, "trade_tick"); trade.price > 0 {
		metadata["tradeTickPrice"] = trade.price
		metadata["tradeTickLastEventAt"] = trade.lastEventAt
	}
	if signal := latestLiveSourcePriceSnapshot(sourceStates, symbol, "signal_bar"); signal.price > 0 {
		metadata["signalBarPrice"] = signal.price
		metadata["signalBarLastEventAt"] = signal.lastEventAt
	}
}

func latestLiveSourcePriceSnapshot(sourceStates map[string]any, symbol, streamType string) liveSourcePriceSnapshot {
	var selected liveSourcePriceSnapshot
	var selectedAt time.Time
	for _, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringValue(entry["streamType"]))) != streamType {
			continue
		}
		if symbol != "" && NormalizeSymbol(stringValue(entry["symbol"])) != symbol {
			continue
		}
		eventAtRaw := stringValue(entry["lastEventAt"])
		eventAt := parseOptionalRFC3339(eventAtRaw)
		if eventAt.IsZero() {
			continue
		}
		if selected.price > 0 && !eventAt.After(selectedAt) {
			continue
		}
		summary := mapValue(entry["summary"])
		price := firstPositive(parseFloatValue(summary["price"]), parseFloatValue(summary["close"]))
		if price <= 0 {
			continue
		}
		selected = liveSourcePriceSnapshot{
			source:      streamType,
			price:       price,
			lastEventAt: eventAtRaw,
		}
		selectedAt = eventAt
	}
	return selected
}

func liveEntryAdverseSubmissionDriftBps(side string, expectedPrice, currentPrice float64) float64 {
	if expectedPrice <= 0 || currentPrice <= 0 {
		return 0
	}
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "BUY":
		if currentPrice <= expectedPrice {
			return 0
		}
		return (currentPrice/expectedPrice - 1) * 10000
	case "SELL":
		if currentPrice >= expectedPrice {
			return 0
		}
		return (expectedPrice - currentPrice) / expectedPrice * 10000
	default:
		return 0
	}
}

func (p *Platform) recordLiveDispatchPreflightRejection(session domain.LiveSession, proposalMap map[string]any, status string, rejectionErr error, eventTime time.Time) {
	state := cloneMetadata(session.State)
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	state["lastDispatchRejectedAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastDispatchRejectedStatus"] = status
	state["lastAutoDispatchError"] = rejectionErr.Error()
	state["lastAutoDispatchAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastExecutionProposal"] = cloneMetadata(proposalMap)
	if guard := cloneMetadata(mapValue(mapValue(proposalMap["metadata"])[liveEntrySubmissionSlippageGuardKey])); len(guard) > 0 {
		state["lastExecutionSubmissionGuard"] = guard
	}
	appendTimelineEvent(state, "order", eventTime, "live-dispatch-preflight-rejected", map[string]any{
		"status": status,
		"error":  rejectionErr.Error(),
		"guard":  cloneMetadata(mapValue(state["lastExecutionSubmissionGuard"])),
	})
	_, _ = p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) dispatchLiveSessionIntent(session domain.LiveSession) (domain.Order, error) {
	releaseDispatch := p.lockLiveSessionDispatch(session.ID)
	defer releaseDispatch()
	if latestSession, latestErr := p.store.GetLiveSession(session.ID); latestErr == nil {
		session = latestSession
	}
	if !strings.EqualFold(session.Status, "RUNNING") && !strings.EqualFold(session.Status, "READY") {
		return domain.Order{}, fmt.Errorf("live session %s is not dispatchable in status %s", session.ID, session.Status)
	}
	if isLiveSessionBlockedByPositionReconcileGate(session.State) {
		return domain.Order{}, fmt.Errorf(
			"live session %s is blocked by reconcile gate: %s (%s)",
			session.ID,
			firstNonEmpty(stringValue(session.State["positionReconcileGateStatus"]), livePositionReconcileGateStatusError),
			firstNonEmpty(stringValue(session.State["positionReconcileGateScenario"]), "unknown"),
		)
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
	proposalMap = assembleLiveExecutionProposalMetadata(session, version.ID, proposalMap)
	if err := validateLiveExecutionProposalMetadata(session, proposalMap); err != nil {
		return domain.Order{}, err
	}
	if err := validateLiveSignalBarEntryTradeLimit(session, proposalMap); err != nil {
		return domain.Order{}, err
	}
	if err := validateLiveDispatchIdempotency(session.State, proposalMap); err != nil {
		return domain.Order{}, err
	}
	dispatchStartedAt := time.Now().UTC()
	proposalMap, err = p.ensureStrategyDecisionEventForExecutionProposal(session, version.ID, proposalMap, dispatchStartedAt, "dispatch-preflight")
	if err != nil {
		return domain.Order{}, err
	}
	proposal = executionProposalFromMap(proposalMap)
	proposalMap, err = p.applyLiveEntrySubmissionSlippageGuard(session, proposalMap, dispatchStartedAt)
	if err != nil {
		p.recordLiveDispatchPreflightRejection(session, proposalMap, liveDispatchRejectedEntrySubmissionGuard, err, dispatchStartedAt)
		return domain.Order{}, err
	}
	proposal = executionProposalFromMap(proposalMap)
	order := buildLiveOrderFromExecutionProposal(session, version.ID, proposal, proposalMap)
	created, createErr := p.CreateOrder(order)
	if createErr != nil && created.ID == "" {
		return domain.Order{}, createErr
	}

	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	dispatchedAt := time.Now().UTC()
	if dispatchedAt.Before(dispatchStartedAt) {
		dispatchedAt = dispatchStartedAt
	}
	if decisionEventID := stringValue(proposalMap["decisionEventId"]); decisionEventID != "" {
		state["lastStrategyDecisionEventId"] = decisionEventID
		if shouldAdvanceLivePlanForOrderStatus(created.Status) {
			state["lastDispatchedDecisionEventId"] = decisionEventID
		}
	}
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
	delete(state, "lastDispatchRejectedAt")
	delete(state, "lastDispatchRejectedStatus")
	recordExecutionDispatchHealth(state, created, dispatchedAt, createErr)
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
	settlementPending := liveOrderSettlementSyncPending(created)
	if strings.EqualFold(created.Status, "FILLED") && !settlementPending {
		if _, syncErr := p.requestLiveAccountSync(session.AccountID, "live-intent-dispatched-filled"); syncErr != nil {
			if !errors.Is(syncErr, ErrLiveAccountOperationInProgress) {
				state["lastSyncError"] = syncErr.Error()
			}
		}
	}
	updatedSession, _ := p.store.UpdateLiveSessionState(session.ID, state)
	if strings.EqualFold(created.Status, "FILLED") && !settlementPending && updatedSession.ID != "" {
		if refreshed, refreshErr := p.refreshLiveSessionPositionContext(updatedSession, dispatchedAt, "live-order-fill-sync"); refreshErr == nil {
			updatedSession = refreshed
		}
	}
	if updatedSession.ID != "" {
		_, _ = p.syncLatestLiveSessionOrder(updatedSession, time.Now().UTC())
	}
	if createErr != nil {
		p.logger("service.live_execution",
			"session_id", session.ID,
			"order_id", created.ID,
		).Warn("live session intent dispatched with error",
			"status", created.Status,
			"error", createErr,
		)
		return created, createErr
	}
	p.logger("service.live_execution",
		"session_id", session.ID,
		"order_id", created.ID,
	).Info("live session intent dispatched",
		"status", created.Status,
		"symbol", created.Symbol,
		"side", created.Side,
		"type", created.Type,
		"quantity", created.Quantity,
	)
	return created, nil
}

func (p *Platform) lockLiveSessionDispatch(sessionID string) func() {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return func() {}
	}
	actual, _ := p.liveDispatchMu.LoadOrStore(sessionID, &sync.Mutex{})
	mu, _ := actual.(*sync.Mutex)
	if mu == nil {
		return func() {}
	}
	mu.Lock()
	return mu.Unlock
}

func validateLiveDispatchIdempotency(state map[string]any, proposalMap map[string]any) error {
	if state == nil || len(proposalMap) == 0 {
		return nil
	}
	decisionEventID := firstNonEmpty(
		stringValue(proposalMap["decisionEventId"]),
		stringValue(mapValue(proposalMap["metadata"])["decisionEventId"]),
	)
	if strings.TrimSpace(decisionEventID) == "" {
		return nil
	}
	if decisionEventID == stringValue(state["lastDispatchedDecisionEventId"]) {
		return fmt.Errorf("live execution proposal already dispatched for decision event %s", decisionEventID)
	}
	dispatchedIntent := mapValue(state["lastDispatchedIntent"])
	if decisionEventID == firstNonEmpty(
		stringValue(dispatchedIntent["decisionEventId"]),
		stringValue(mapValue(dispatchedIntent["metadata"])["decisionEventId"]),
	) {
		return fmt.Errorf("live execution proposal already dispatched for decision event %s", decisionEventID)
	}
	return nil
}

func buildLiveOrderFromExecutionProposal(session domain.LiveSession, strategyVersionID string, proposal ExecutionProposal, proposalMap map[string]any) domain.Order {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(proposal.Type, "MARKET")))
	quantity := firstPositive(proposal.Quantity, firstPositive(parseFloatValue(session.State["defaultOrderQuantity"]), 0.001))
	price := proposal.PriceHint
	if orderType != "MARKET" {
		price = firstPositive(proposal.LimitPrice, proposal.PriceHint)
	}
	proposalMeta := cloneMetadata(mapValue(proposalMap["metadata"]))
	if proposalMeta == nil {
		proposalMeta = map[string]any{}
	}
	orderMetadata := map[string]any{
		"source":             "live-session-intent",
		"liveSessionId":      session.ID,
		"signalKind":         proposal.SignalKind,
		"dispatchMode":       stringValue(session.State["dispatchMode"]),
		"timeInForce":        proposal.TimeInForce,
		"postOnly":           proposal.PostOnly,
		"reduceOnly":         proposal.ReduceOnly,
		"decisionEventId":    stringValue(proposalMap["decisionEventId"]),
		"executionStrategy":  proposal.ExecutionStrategy,
		"executionExpiresAt": stringValue(proposal.Metadata["executionExpiresAt"]),
		"executionProposal":  cloneMetadata(proposalMap),
		"intent":             cloneMetadata(proposalMap),
	}
	applyExecutionMetadata(orderMetadata, map[string]any{
		"strategyVersionId":      firstNonEmpty(stringValue(proposalMeta["strategyVersionId"]), strategyVersionID),
		"runtimeSessionId":       stringValue(proposalMeta["runtimeSessionId"]),
		"executionContext":       cloneMetadata(mapValue(proposalMeta["executionContext"])),
		"executionMode":          firstNonEmpty(stringValue(proposalMeta["executionMode"]), "live"),
		"recoveryTriggered":      boolValue(proposalMeta["recoveryTriggered"]),
		"positionRecoveryStatus": stringValue(proposalMeta["positionRecoveryStatus"]),
		"positionRecoverySource": stringValue(proposalMeta["positionRecoverySource"]),
	})
	return domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: strategyVersionID,
		Symbol:            NormalizeSymbol(firstNonEmpty(proposal.Symbol, stringValue(session.State["symbol"]))),
		Side:              strings.ToUpper(strings.TrimSpace(proposal.Side)),
		Type:              orderType,
		Quantity:          quantity,
		Price:             price,
		ReduceOnly:        proposal.ReduceOnly,
		Metadata:          orderMetadata,
	}
}

func (p *Platform) applyLiveVirtualInitialEvent(session domain.LiveSession, proposalMap map[string]any, eventTime time.Time) (domain.LiveSession, error) {
	proposal := executionProposalFromMap(proposalMap)
	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	if !hasInformativeLiveIntentSignature(intentSignature) {
		intentSignature = buildFallbackLiveIntentSignature(proposalMap, proposal)
	}
	virtualPositionID := fmt.Sprintf("virtual|%s|%s", session.ID, intentSignature)
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
	delete(state, "lastDispatchRejectedAt")
	delete(state, "lastDispatchRejectedStatus")
	recordExecutionDispatchHealth(state, domain.Order{
		Side:   proposal.Side,
		Symbol: proposal.Symbol,
		Type:   proposal.Type,
		Status: liveOrderStatusVirtualInitial,
	}, eventTime, nil)
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
		"id":         virtualPositionID,
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

func hasInformativeLiveIntentSignature(signature string) bool {
	nonEmpty := 0
	for _, part := range strings.Split(signature, "|") {
		if strings.TrimSpace(part) != "" {
			nonEmpty++
		}
	}
	return nonEmpty >= 3
}

func proposalBooleanValue(proposalMap map[string]any, key string, fallback bool) bool {
	value, ok := proposalMap[key]
	if !ok {
		return fallback
	}
	if typed, ok := value.(bool); ok {
		return typed
	}
	return boolValue(value)
}

func buildFallbackLiveIntentSignature(proposalMap map[string]any, proposal ExecutionProposal) string {
	anchor := firstNonEmpty(
		strings.TrimSpace(stringValue(proposalMap["signalBarStateKey"])),
		strings.TrimSpace(stringValue(proposalMap["plannedEventAt"])),
		strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["decisionState"]), proposal.DecisionState)),
	)
	return strings.Join([]string{
		firstNonEmpty(strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["action"]), proposal.Action)), "virtual"),
		strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["role"]), proposal.Role)),
		firstNonEmpty(strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["reason"]), proposal.Reason)), "virtual-initial"),
		strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["side"]), proposal.Side))),
		NormalizeSymbol(firstNonEmpty(stringValue(proposalMap["symbol"]), proposal.Symbol)),
		strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["type"]), proposal.Type, "MARKET"))),
		strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["signalKind"]), proposal.SignalKind)),
		anchor,
		fmt.Sprintf("%.8f", firstPositive(parseFloatValue(proposalMap["quantity"]), proposal.Quantity)),
		fmt.Sprintf("%.8f", firstPositive(parseFloatValue(proposalMap["plannedPrice"]), 0)),
		fmt.Sprintf("%.8f", firstPositive(parseFloatValue(proposalMap["limitPrice"]), proposal.LimitPrice)),
		fmt.Sprintf("%.8f", firstPositive(parseFloatValue(proposalMap["priceHint"]), proposal.PriceHint)),
		strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["priceSource"]), proposal.PriceSource)),
		strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(proposalMap["timeInForce"]), proposal.TimeInForce))),
		normalizeExecutionStrategyKey(firstNonEmpty(stringValue(proposalMap["executionStrategy"]), proposal.ExecutionStrategy)),
		fmt.Sprintf("%t", proposalBooleanValue(proposalMap, "postOnly", proposal.PostOnly)),
		fmt.Sprintf("%t", proposalBooleanValue(proposalMap, "reduceOnly", proposal.ReduceOnly)),
		fmt.Sprintf("%t", proposalBooleanValue(proposalMap, "closePosition", false)),
	}, "|")
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
	delete(state, "lastDispatchRejectedAt")
	delete(state, "lastDispatchRejectedStatus")
	recordExecutionDispatchHealth(state, domain.Order{
		Side:   proposal.Side,
		Symbol: proposal.Symbol,
		Type:   proposal.Type,
		Status: liveOrderStatusVirtualExit,
	}, eventTime, nil)
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
		if shouldBackfillTerminalFilledLiveOrder(order, state) {
			state["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
			recordExecutionSyncAttemptHealth(state, eventTime)
			syncedOrder, syncErr := p.SyncLiveOrder(order.ID)
			if syncErr != nil {
				state["lastSyncError"] = syncErr.Error()
				recordExecutionSyncResultHealth(state, eventTime, order.Status, syncErr)
				appendTimelineEvent(state, "order", eventTime, "live-order-sync-error", map[string]any{
					"orderId": order.ID,
					"error":   syncErr.Error(),
				})
				updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
				if updateErr != nil {
					return domain.LiveSession{}, updateErr
				}
				return updated, syncErr
			}
			order = syncedOrder
			delete(state, "lastSyncError")
			state["lastSyncedAt"] = eventTime.UTC().Format(time.RFC3339)
			recordExecutionSyncResultHealth(state, eventTime, order.Status, nil)
		}
		maybeIncrementLiveSessionReentryCount(state, mapValue(order.Metadata["executionProposal"]), order.ID, order.Status)
		state["lastSyncedOrderId"] = order.ID
		state["lastSyncedOrderStatus"] = order.Status
		state["lastDispatchedOrderStatus"] = order.Status
		state["lastExecutionDispatch"] = executionDispatchSummary(mapValue(order.Metadata["executionProposal"]), order, false)
		updateExecutionEventStats(state, mapValue(order.Metadata["executionProposal"]), mapValue(state["lastExecutionDispatch"]))
		if shouldSyncLiveAccountAfterTerminalFilledOrder(order, state, eventTime) {
			state["lastTerminalAccountSyncAttemptOrderId"] = order.ID
			state["lastTerminalAccountSyncAttemptOrderStatus"] = order.Status
			state["lastTerminalAccountSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
			if _, syncErr := p.requestLiveAccountSync(session.AccountID, "live-terminal-order-sync"); syncErr != nil && !errors.Is(syncErr, ErrLiveAccountOperationInProgress) {
				state["lastTerminalAccountSyncError"] = syncErr.Error()
				p.logger("service.live_execution", "session_id", session.ID, "account_id", session.AccountID).Warn("live account sync failed after terminal order sync", "error", syncErr)
			} else {
				state["lastTerminalAccountSyncedOrderId"] = order.ID
				state["lastTerminalAccountSyncedOrderStatus"] = order.Status
				state["lastTerminalAccountSyncedAt"] = eventTime.UTC().Format(time.RFC3339)
				delete(state, "lastTerminalAccountSyncError")
			}
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
		recordExecutionSyncAttemptHealth(state, eventTime)
		if cancelErr != nil {
			state["lastSyncError"] = cancelErr.Error()
			recordExecutionSyncResultHealth(state, eventTime, order.Status, cancelErr)
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
		recordExecutionSyncResultHealth(state, eventTime, cancelledOrder.Status, nil)
		state["lastExecutionTimeoutAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastExecutionTimeoutReason"] = "resting-order-expired"
		timeoutOrder := withExecutionSubmissionFallback(cancelledOrder, order)
		state["lastExecutionDispatch"] = executionDispatchSummary(mapValue(order.Metadata["executionProposal"]), timeoutOrder, false)
		updateExecutionEventStats(state, mapValue(order.Metadata["executionProposal"]), mapValue(state["lastExecutionDispatch"]))
		timeoutSignature := buildLiveIntentSignature(mapValue(order.Metadata["executionProposal"]))
		if timeoutSignature == "" {
			timeoutSignature = buildLiveIntentSignature(mapValue(order.Metadata["intent"]))
		}
		if timeoutSignature != "" {
			state["lastExecutionTimeoutIntentSignature"] = timeoutSignature
		}
		appendTimelineEvent(state, "order", eventTime, "live-order-cancelled-timeout", executionTimeoutTimelineMetadata(order, timeoutOrder))
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
	syncedOrder, err := p.SyncLiveOrder(order.ID)
	state["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
	recordExecutionSyncAttemptHealth(state, eventTime)
	if err != nil {
		state["lastSyncError"] = err.Error()
		recordExecutionSyncResultHealth(state, eventTime, order.Status, err)
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
	maybeIncrementLiveSessionReentryCount(state, mapValue(order.Metadata["executionProposal"]), syncedOrder.ID, syncedOrder.Status)
	state["lastSyncedOrderId"] = syncedOrder.ID
	state["lastSyncedOrderStatus"] = syncedOrder.Status
	state["lastDispatchedOrderStatus"] = syncedOrder.Status
	state["lastSyncedAt"] = time.Now().UTC().Format(time.RFC3339)
	recordExecutionSyncResultHealth(state, eventTime, syncedOrder.Status, nil)
	state["lastExecutionDispatch"] = executionDispatchSummary(mapValue(order.Metadata["executionProposal"]), syncedOrder, false)
	updateExecutionEventStats(state, mapValue(order.Metadata["executionProposal"]), mapValue(state["lastExecutionDispatch"]))
	if strings.EqualFold(syncedOrder.Status, "FILLED") {
		if _, syncErr := p.requestLiveAccountSync(session.AccountID, "live-filled-order-sync"); syncErr != nil && !errors.Is(syncErr, ErrLiveAccountOperationInProgress) {
			p.logger("service.live_execution", "session_id", session.ID, "account_id", session.AccountID).Warn("live account sync failed after filled order sync", "error", syncErr)
		}
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

func shouldBackfillTerminalFilledLiveOrder(order domain.Order, state map[string]any) bool {
	if !strings.EqualFold(order.Status, "FILLED") {
		return false
	}
	if tradingQuantityBelow(parseFloatValue(order.Metadata["filledQuantity"]), order.Quantity) {
		return true
	}
	if strings.TrimSpace(stringValue(order.Metadata["lastFilledAt"])) == "" {
		return true
	}
	return false
}

func shouldSyncLiveAccountAfterTerminalFilledOrder(order domain.Order, state map[string]any, eventTime time.Time) bool {
	if !strings.EqualFold(order.Status, "FILLED") {
		return false
	}
	if stringValue(state["lastTerminalAccountSyncedOrderId"]) == order.ID &&
		strings.EqualFold(stringValue(state["lastTerminalAccountSyncedOrderStatus"]), order.Status) {
		return false
	}
	if stringValue(state["lastTerminalAccountSyncAttemptOrderId"]) == order.ID &&
		strings.EqualFold(stringValue(state["lastTerminalAccountSyncAttemptOrderStatus"]), order.Status) {
		lastAttemptAt := parseOptionalRFC3339(stringValue(state["lastTerminalAccountSyncAttemptAt"]))
		if !lastAttemptAt.IsZero() && eventTime.UTC().Sub(lastAttemptAt.UTC()) < 30*time.Second {
			return false
		}
	}
	return true
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

func maybeIncrementLiveSessionReentryCount(state map[string]any, proposalMap map[string]any, orderID, status string) {
	if state == nil || !strings.EqualFold(strings.TrimSpace(status), "FILLED") {
		return
	}
	reasonTag := normalizeStrategyReasonTag(stringValue(proposalMap["reason"]))
	if reasonTag != "zero-initial-reentry" && reasonTag != "sl-reentry" && reasonTag != "pt-reentry" {
		return
	}
	if orderID != "" && stringValue(state["lastCountedReentryOrderId"]) == orderID {
		return
	}

	currentBarKey := liveProposalSignalBarTradeLimitKey(proposalMap)
	lastBarKey := stringValue(state["lastSignalBarStateKey"])
	reentryCount := parseFloatValue(state["sessionReentryCount"])
	if currentBarKey != "" && currentBarKey != lastBarKey {
		reentryCount = 0
		state["lastSignalBarStateKey"] = currentBarKey
		delete(state, "lastCountedReentryOrderId")
	}
	reentryCount++
	state["sessionReentryCount"] = reentryCount
	if orderID != "" {
		state["lastCountedReentryOrderId"] = orderID
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

func withExecutionSubmissionFallback(order domain.Order, fallback domain.Order) domain.Order {
	currentSubmission := cloneMetadata(mapValue(order.Metadata["adapterSubmission"]))
	fallbackSubmission := cloneMetadata(mapValue(fallback.Metadata["adapterSubmission"]))
	if len(currentSubmission) > 0 && len(fallbackSubmission) == 0 {
		return order
	}
	mergedSubmission := mergeExecutionSubmissionFallback(currentSubmission, fallbackSubmission)
	if len(mergedSubmission) == 0 {
		return order
	}
	enriched := order
	enriched.Metadata = cloneMetadata(order.Metadata)
	if enriched.Metadata == nil {
		enriched.Metadata = map[string]any{}
	}
	enriched.Metadata["adapterSubmission"] = mergedSubmission
	return enriched
}

func mergeExecutionSubmissionFallback(current map[string]any, fallback map[string]any) map[string]any {
	return mergeExecutionSubmissionFallbackWithPath(current, fallback, "")
}

func mergeExecutionSubmissionFallbackWithPath(current map[string]any, fallback map[string]any, path string) map[string]any {
	if len(current) == 0 {
		return cloneMetadata(fallback)
	}
	if len(fallback) == 0 {
		return cloneMetadata(current)
	}
	merged := cloneMetadata(fallback)
	for key, value := range current {
		childPath := key
		if path != "" {
			childPath = path + "." + key
		}
		currentMap := mapValue(value)
		fallbackMap := mapValue(merged[key])
		if len(currentMap) > 0 && len(fallbackMap) > 0 {
			merged[key] = mergeExecutionSubmissionFallbackWithPath(currentMap, fallbackMap, childPath)
			continue
		}
		if executionSubmissionValuePresent(childPath, value) {
			merged[key] = value
		}
	}
	return merged
}

func executionSubmissionValuePresent(path string, value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case bool:
		return executionSubmissionBooleanValuePresent(path, typed)
	case int:
		return executionSubmissionNumericValuePresent(path, float64(typed))
	case int64:
		return executionSubmissionNumericValuePresent(path, float64(typed))
	case float64:
		return executionSubmissionNumericValuePresent(path, typed)
	case []any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func executionSubmissionBooleanValuePresent(path string, value bool) bool {
	if value {
		return true
	}
	switch path {
	case "postOnly", "reduceOnly", "closePosition", "slProtectionActive":
		return false
	default:
		return true
	}
}

func executionSubmissionNumericValuePresent(path string, value float64) bool {
	if value != 0 {
		return true
	}
	switch path {
	case "rawQuantity",
		"normalizedQuantity",
		"normalization.rawQuantity",
		"normalization.normalizedQuantity":
		return false
	case "rawPriceReference",
		"normalizedPrice",
		"normalization.rawPriceReference",
		"normalization.normalizedPrice":
		return false
	case "symbolRules.minQty",
		"symbolRules.stepSize",
		"symbolRules.tickSize",
		"symbolRules.minNotional":
		return false
	default:
		return true
	}
}

func executionDispatchSummary(proposalMap map[string]any, order domain.Order, failed bool) map[string]any {
	proposalMeta := cloneMetadata(mapValue(proposalMap["metadata"]))
	adapterSubmission := cloneMetadata(mapValue(order.Metadata["adapterSubmission"]))
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
		"rawQuantity":       firstPositive(parseFloatValue(adapterSubmission["rawQuantity"]), parseFloatValue(mapValue(adapterSubmission["normalization"])["rawQuantity"])),
		"normalizedQuantity": firstPositive(
			parseFloatValue(adapterSubmission["normalizedQuantity"]),
			parseFloatValue(mapValue(adapterSubmission["normalization"])["normalizedQuantity"]),
		),
		"rawPriceReference": firstPositive(
			parseFloatValue(adapterSubmission["rawPriceReference"]),
			parseFloatValue(mapValue(adapterSubmission["normalization"])["rawPriceReference"]),
		),
		"normalizedPrice": firstPositive(
			parseFloatValue(adapterSubmission["normalizedPrice"]),
			parseFloatValue(mapValue(adapterSubmission["normalization"])["normalizedPrice"]),
		),
		"normalization": cloneMetadata(mapValue(adapterSubmission["normalization"])),
		"symbolRules":   cloneMetadata(mapValue(adapterSubmission["symbolRules"])),
		"failed":        failed,
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
