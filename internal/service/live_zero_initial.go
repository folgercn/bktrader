package service

import (
	"strings"
	"time"
)

const (
	livePendingZeroInitialWindowStateKey          = "pendingZeroInitialWindow"
	liveZeroInitialWindowOpenReasonBreakoutLocked = "breakout-confirmed"
)

func prepareLivePlanStepForSignalEvaluation(
	sessionState map[string]any,
	parameters map[string]any,
	signalBarStates map[string]any,
	symbol string,
	signalTimeframe string,
	currentPosition map[string]any,
	eventTime time.Time,
	breakoutPrice float64,
	breakoutPriceSource string,
	nextPlannedEvent time.Time,
	nextPlannedPrice float64,
	nextPlannedSide, nextPlannedRole, nextPlannedReason string,
) (map[string]any, time.Time, float64, string, string, string) {
	updatedState := cloneMetadata(sessionState)
	hasActivePosition := hasActiveLivePositionSnapshot(currentPosition)
	if !strategyZeroInitialReentryWindowEnabled(parameters) {
		alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason := alignLivePlanStepToCurrentMarket(
			parameters,
			signalBarStates,
			signalTimeframe,
			currentPosition,
			eventTime,
			breakoutPrice,
			breakoutPriceSource,
			nextPlannedEvent,
			nextPlannedPrice,
			nextPlannedSide,
			nextPlannedRole,
			nextPlannedReason,
		)
		return updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason
	}

	updatedState = refreshLiveZeroInitialWindowState(updatedState, signalBarStates, symbol, signalTimeframe, currentPosition, eventTime)
	staleExitReentry := liveExitReentryPlanStep(nextPlannedRole, nextPlannedReason)
	// Replay only keeps PT/SL reentry eligible through the next signal bar. Once
	// that boundary is crossed, live no longer trusts the historical trigger
	// timing and must re-align using the current signal bar instead of replaying
	// the stale plan step indefinitely.
	if staleExitReentry && hasActivePosition {
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	if staleExitReentry && !isLivePlanStepStale(nextPlannedEvent, signalTimeframe, eventTime) {
		clearLivePendingZeroInitialWindow(updatedState, eventTime, "exit-reentry-priority")
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	if hasActivePosition {
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	var alignedEvent time.Time
	var alignedPrice float64
	var alignedSide, alignedRole, alignedReason string
	var ok bool
	updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason, ok = liveSLReentryWindowPlanStep(
		updatedState,
		parameters,
		signalBarStates,
		symbol,
		signalTimeframe,
		eventTime,
	)
	if ok {
		clearLivePendingZeroInitialWindow(updatedState, eventTime, "sl-exit-reentry-priority")
		return updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason
	}
	updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason, ok = liveZeroInitialWindowPlanStep(
		updatedState,
		parameters,
		signalBarStates,
		symbol,
		signalTimeframe,
		eventTime,
	)
	if ok {
		if livePendingZeroInitialWindowShouldYieldSLReentry(updatedState, signalBarStates, symbol, signalTimeframe, eventTime) {
			clearLivePendingZeroInitialWindow(updatedState, eventTime, "sl-exit-reentry-priority")
			return updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, "SL-Reentry"
		}
		return updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason
	}
	if hasActiveLivePositionSnapshot(currentPosition) || !isLivePlanStepStale(nextPlannedEvent, signalTimeframe, eventTime) {
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}

	signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe)
	if signalBarState == nil {
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	alignmentMode := ""
	gate := evaluateSignalBarGate(signalBarState, "", "entry", "", breakoutPrice, breakoutPriceSource, signalBarGateOptionsFromParameters(parameters))
	longReady := boolValue(gate["longReady"])
	shortReady := boolValue(gate["shortReady"])
	if longReady != shortReady {
		alignmentMode = "breakout-confirmed"
	}
	if staleExitReentry && longReady == shortReady {
		if alignedEvent, alignedPrice, alignedSide, ok := liveBootstrapPlanStepFromSignalBar(
			signalBarState,
			eventTime,
			nextPlannedPrice,
			boolValue(gate["longStructureReady"]),
			boolValue(gate["shortStructureReady"]),
		); ok {
			return updatedState, alignedEvent, alignedPrice, alignedSide, "entry", "Initial"
		}
	}
	if longReady == shortReady {
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}

	currentBarStart := liveCurrentSignalBarStart(signalBarState, eventTime)
	expiresAt := liveZeroInitialWindowExpiresAt(currentBarStart, signalTimeframe)
	side := "BUY"
	if shortReady {
		side = "SELL"
	}
	plannedReentryPrice := resolveLiveReentryPlanPrice(parameters, signalBarState, side)
	pendingWindow := map[string]any{
		"side":                      side,
		"symbol":                    NormalizeSymbol(symbol),
		"signalTimeframe":           strings.ToLower(strings.TrimSpace(signalTimeframe)),
		"armedAt":                   eventTime.UTC().Format(time.RFC3339),
		"signalBarStart":            currentBarStart.UTC().Format(time.RFC3339),
		"expiresAt":                 expiresAt.UTC().Format(time.RFC3339),
		"breakoutBacked":            true,
		"openReason":                liveZeroInitialWindowOpenReasonBreakoutLocked,
		"breakoutSide":              side,
		"breakoutPrice":             breakoutPrice,
		"breakoutPriceSource":       strings.TrimSpace(breakoutPriceSource),
		"plannedReentryPrice":       plannedReentryPrice,
		"plannedReentryPriceSource": "zero-initial-window-armed",
		"longStructureReady":        boolValue(gate["longStructureReady"]),
		"shortStructureReady":       boolValue(gate["shortStructureReady"]),
		"longBreakoutReady":         boolValue(gate["longBreakoutReady"]),
		"shortBreakoutReady":        boolValue(gate["shortBreakoutReady"]),
		"longBreakoutPriceReady":    boolValue(gate["longBreakoutPriceReady"]),
		"shortBreakoutPriceReady":   boolValue(gate["shortBreakoutPriceReady"]),
		"longBreakoutShapeReady":    boolValue(gate["longBreakoutShapeReady"]),
		"shortBreakoutShapeReady":   boolValue(gate["shortBreakoutShapeReady"]),
		"longBreakoutQualityReady":  boolValue(gate["longBreakoutQualityReady"]),
		"shortBreakoutQualityReady": boolValue(gate["shortBreakoutQualityReady"]),
		"longBreakoutPatternReady":  boolValue(gate["longBreakoutPatternReady"]),
		"shortBreakoutPatternReady": boolValue(gate["shortBreakoutPatternReady"]),
		"longBreakoutShapeName":     stringValue(gate["longBreakoutShapeName"]),
		"shortBreakoutShapeName":    stringValue(gate["shortBreakoutShapeName"]),
	}
	updatedState[livePendingZeroInitialWindowStateKey] = pendingWindow
	timelineMetadata := map[string]any{
		livePendingZeroInitialWindowStateKey: cloneMetadata(pendingWindow),
		"reason":                             "Zero-Initial-Reentry",
		"side":                               side,
		"symbol":                             NormalizeSymbol(symbol),
		"signalTimeframe":                    strings.ToLower(strings.TrimSpace(signalTimeframe)),
	}
	if staleExitReentry {
		timelineMetadata["staleExitReentryContext"] = liveStaleExitReentryContext(
			signalBarState,
			signalTimeframe,
			eventTime,
			breakoutPrice,
			breakoutPriceSource,
			nextPlannedEvent,
			nextPlannedPrice,
			nextPlannedSide,
			nextPlannedRole,
			nextPlannedReason,
			alignmentMode,
		)
	}
	appendTimelineEvent(updatedState, "strategy", eventTime, "zero-initial-window-armed", timelineMetadata)
	updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason, _ = liveZeroInitialWindowPlanStep(updatedState, parameters, signalBarStates, symbol, signalTimeframe, eventTime)
	if livePendingZeroInitialWindowShouldYieldSLReentry(updatedState, signalBarStates, symbol, signalTimeframe, eventTime) {
		clearLivePendingZeroInitialWindow(updatedState, eventTime, "sl-exit-reentry-priority")
		return updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, "SL-Reentry"
	}
	return updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason
}

func refreshLiveZeroInitialWindowState(
	sessionState map[string]any,
	signalBarStates map[string]any,
	symbol string,
	signalTimeframe string,
	currentPosition map[string]any,
	eventTime time.Time,
) map[string]any {
	state := cloneMetadata(sessionState)
	if state == nil {
		state = map[string]any{}
	}
	pending := cloneMetadata(mapValue(state[livePendingZeroInitialWindowStateKey]))
	if len(pending) == 0 {
		return state
	}
	if !liveZeroInitialWindowHasBreakoutProof(pending) {
		clearLivePendingZeroInitialWindow(state, eventTime, "zero-initial-window-missing-breakout-proof")
		return state
	}
	if pendingSymbol := NormalizeSymbol(stringValue(pending["symbol"])); pendingSymbol != "" && pendingSymbol != NormalizeSymbol(symbol) {
		delete(state, livePendingZeroInitialWindowStateKey)
		return state
	}
	if pendingTimeframe := strings.ToLower(strings.TrimSpace(stringValue(pending["signalTimeframe"]))); pendingTimeframe != "" &&
		pendingTimeframe != strings.ToLower(strings.TrimSpace(signalTimeframe)) {
		delete(state, livePendingZeroInitialWindowStateKey)
		return state
	}
	pending = normalizeLiveZeroInitialWindowTiming(pending, signalTimeframe)
	expiresAt := parseOptionalRFC3339(stringValue(pending["expiresAt"]))
	if !expiresAt.IsZero() && !eventTime.UTC().Before(expiresAt.UTC()) {
		delete(state, livePendingZeroInitialWindowStateKey)
		return state
	}
	if signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe); signalBarState != nil {
		if currentBarStart := liveCurrentSignalBarStart(signalBarState, time.Time{}); !currentBarStart.IsZero() {
			pendingBarStart := parseOptionalRFC3339(stringValue(pending["signalBarStart"]))
			if !pendingBarStart.IsZero() && currentBarStart.UTC().Before(pendingBarStart.UTC()) {
				delete(state, livePendingZeroInitialWindowStateKey)
				return state
			}
		}
	}
	state[livePendingZeroInitialWindowStateKey] = pending
	return state
}

func liveExitReentryPlanStep(nextRole, nextReason string) bool {
	if !strings.EqualFold(strings.TrimSpace(nextRole), "entry") {
		return false
	}
	switch normalizeStrategyReasonTag(nextReason) {
	case "sl-reentry", "pt-reentry":
		return true
	default:
		return false
	}
}

func clearLivePendingZeroInitialWindow(state map[string]any, eventTime time.Time, reason string) {
	if state == nil {
		return
	}
	pending := cloneMetadata(mapValue(state[livePendingZeroInitialWindowStateKey]))
	if len(pending) == 0 {
		delete(state, livePendingZeroInitialWindowStateKey)
		return
	}
	delete(state, livePendingZeroInitialWindowStateKey)
	appendTimelineEvent(state, "strategy", eventTime, "zero-initial-window-consumed", map[string]any{
		livePendingZeroInitialWindowStateKey: pending,
		"reason":                             firstNonEmpty(strings.TrimSpace(reason), "consumed"),
	})
}

func liveSLReentryWindowConsumed(state map[string]any) bool {
	orderID := strings.TrimSpace(stringValue(state["lastSLExitOrderId"]))
	if orderID == "" {
		return false
	}
	return strings.TrimSpace(stringValue(state["lastSLExitReentryConsumedOrderId"])) == orderID
}

func consumeLiveSLReentryWindow(state map[string]any, eventTime time.Time, reason string) {
	if state == nil {
		return
	}
	if orderID := strings.TrimSpace(stringValue(state["lastSLExitOrderId"])); orderID != "" {
		state["lastSLExitReentryConsumedOrderId"] = orderID
	}
	state["lastSLExitReentryConsumedAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastSLExitReentryConsumedReason"] = firstNonEmpty(strings.TrimSpace(reason), "consumed")
	delete(state, "lastSLExitReentrySide")
}

func liveZeroInitialWindowHasBreakoutProof(pending map[string]any) bool {
	if !boolValue(pending["breakoutBacked"]) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(stringValue(pending["openReason"])), liveZeroInitialWindowOpenReasonBreakoutLocked)
}

func livePendingZeroInitialWindowOpen(sessionState map[string]any, symbol, signalTimeframe, side string, eventTime time.Time) bool {
	pending := cloneMetadata(mapValue(sessionState[livePendingZeroInitialWindowStateKey]))
	if len(pending) == 0 {
		return false
	}
	if !liveZeroInitialWindowHasBreakoutProof(pending) {
		return false
	}
	if !liveZeroInitialWindowHasSideBreakoutProof(pending, side) {
		return false
	}
	if pendingSide := strings.ToUpper(strings.TrimSpace(stringValue(pending["side"]))); pendingSide != "" &&
		pendingSide != strings.ToUpper(strings.TrimSpace(side)) {
		return false
	}
	if pendingSymbol := NormalizeSymbol(stringValue(pending["symbol"])); pendingSymbol != "" &&
		pendingSymbol != NormalizeSymbol(symbol) {
		return false
	}
	if pendingTimeframe := strings.ToLower(strings.TrimSpace(stringValue(pending["signalTimeframe"]))); pendingTimeframe != "" &&
		pendingTimeframe != strings.ToLower(strings.TrimSpace(signalTimeframe)) {
		return false
	}
	pending = normalizeLiveZeroInitialWindowTiming(pending, signalTimeframe)
	expiresAt := parseOptionalRFC3339(stringValue(pending["expiresAt"]))
	if !expiresAt.IsZero() && !eventTime.UTC().Before(expiresAt.UTC()) {
		return false
	}
	return true
}

func normalizeLiveZeroInitialWindowTiming(pending map[string]any, signalTimeframe string) map[string]any {
	normalized := cloneMetadata(pending)
	barStart := parseOptionalRFC3339(stringValue(normalized["signalBarStart"]))
	if barStart.IsZero() {
		return normalized
	}
	barStart = liveCanonicalSignalBarStart(barStart, signalTimeframe)
	if barStart.IsZero() {
		return normalized
	}
	normalized["signalBarStart"] = barStart.UTC().Format(time.RFC3339)
	expiresAt := liveZeroInitialWindowExpiresAt(barStart, signalTimeframe)
	if !expiresAt.IsZero() {
		normalized["expiresAt"] = expiresAt.UTC().Format(time.RFC3339)
	}
	return normalized
}

func liveCurrentSignalBarStart(signalBarState map[string]any, fallback time.Time) time.Time {
	current := mapValue(signalBarState["current"])
	if current == nil {
		return fallback.UTC()
	}
	return liveCanonicalSignalBarStart(resolveBreakoutSignalTime(current["barStart"], fallback), firstNonEmpty(
		stringValue(signalBarState["timeframe"]),
		stringValue(current["timeframe"]),
	))
}

func liveCanonicalSignalBarStart(value time.Time, signalTimeframe string) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	step := liveSignalBarStep(signalTimeframe)
	if step <= 0 {
		return value.UTC()
	}
	utc := value.UTC()
	nanos := utc.UnixNano()
	stepNanos := int64(step)
	return time.Unix(0, nanos-(nanos%stepNanos)).UTC()
}

func liveZeroInitialWindowExpiresAt(barStart time.Time, signalTimeframe string) time.Time {
	if barStart.IsZero() {
		return time.Time{}
	}
	step := liveSignalBarStep(signalTimeframe)
	if step <= 0 {
		return barStart.UTC()
	}
	return barStart.UTC().Add(2 * step)
}

func liveSignalBarStep(signalTimeframe string) time.Duration {
	step := resolutionToDuration(liveSignalResolution(signalTimeframe))
	if step <= 0 {
		return 4 * time.Hour
	}
	return step
}

func liveSignalBarKeyWithinReentryWindow(lastKey, currentKey, signalTimeframe string) bool {
	lastSymbol, lastTimeframe, lastStart, ok := parseLiveSignalBarTradeLimitKey(lastKey)
	if !ok {
		return false
	}
	currentSymbol, currentTimeframe, currentStart, ok := parseLiveSignalBarTradeLimitKey(currentKey)
	if !ok {
		return false
	}
	if lastSymbol != "" && currentSymbol != "" && lastSymbol != currentSymbol {
		return false
	}
	if lastTimeframe != "" && currentTimeframe != "" && lastTimeframe != currentTimeframe {
		return false
	}
	step := liveSignalBarStep(firstNonEmpty(signalTimeframe, currentTimeframe, lastTimeframe))
	if step <= 0 || currentStart.Before(lastStart) {
		return false
	}
	return currentStart.Sub(lastStart) <= step
}

func liveSignalBarKeyPastReentryWindow(lastKey, currentKey, signalTimeframe string) bool {
	lastSymbol, lastTimeframe, lastStart, ok := parseLiveSignalBarTradeLimitKey(lastKey)
	if !ok {
		return false
	}
	currentSymbol, currentTimeframe, currentStart, ok := parseLiveSignalBarTradeLimitKey(currentKey)
	if !ok {
		return false
	}
	if lastSymbol != "" && currentSymbol != "" && lastSymbol != currentSymbol {
		return false
	}
	if lastTimeframe != "" && currentTimeframe != "" && lastTimeframe != currentTimeframe {
		return false
	}
	step := liveSignalBarStep(firstNonEmpty(signalTimeframe, currentTimeframe, lastTimeframe))
	if step <= 0 || currentStart.Before(lastStart) {
		return false
	}
	return currentStart.Sub(lastStart) > step
}

func parseLiveSignalBarTradeLimitKey(key string) (string, string, time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(key), "|")
	if len(parts) != 3 {
		return "", "", time.Time{}, false
	}
	barStart := parseOptionalRFC3339(parts[2])
	if barStart.IsZero() {
		return "", "", time.Time{}, false
	}
	return NormalizeSymbol(parts[0]), strings.ToLower(strings.TrimSpace(parts[1])), barStart.UTC(), true
}

func liveBootstrapPlanStepFromSignalBar(
	signalBarState map[string]any,
	eventTime time.Time,
	fallbackPrice float64,
	longStructureReady, shortStructureReady bool,
) (time.Time, float64, string, bool) {
	if longStructureReady == shortStructureReady {
		return time.Time{}, 0, "", false
	}
	current := mapValue(signalBarState["current"])
	plannedEvent := liveCurrentSignalBarStart(signalBarState, eventTime)
	price := parseFloatValue(current["close"])
	if price <= 0 {
		price = fallbackPrice
	}
	side := "BUY"
	if shortStructureReady {
		side = "SELL"
	}
	return plannedEvent.UTC(), price, side, true
}

func liveStaleExitReentryContext(
	signalBarState map[string]any,
	signalTimeframe string,
	eventTime time.Time,
	breakoutPrice float64,
	breakoutPriceSource string,
	nextPlannedEvent time.Time,
	nextPlannedPrice float64,
	nextPlannedSide, nextPlannedRole, nextPlannedReason string,
	alignmentMode string,
) map[string]any {
	context := map[string]any{
		"alignmentMode":       strings.TrimSpace(alignmentMode),
		"breakoutPrice":       breakoutPrice,
		"breakoutPriceSource": strings.TrimSpace(breakoutPriceSource),
		"plannedPrice":        nextPlannedPrice,
		"plannedReason":       strings.TrimSpace(nextPlannedReason),
		"plannedRole":         strings.ToLower(strings.TrimSpace(nextPlannedRole)),
		"plannedSide":         strings.ToUpper(strings.TrimSpace(nextPlannedSide)),
	}
	if !nextPlannedEvent.IsZero() {
		context["plannedEvent"] = nextPlannedEvent.UTC().Format(time.RFC3339)
	}
	step := resolutionToDuration(liveSignalResolution(signalTimeframe))
	if step <= 0 {
		step = 4 * time.Hour
	}
	context["staleWindowSeconds"] = step.Seconds()
	if !nextPlannedEvent.IsZero() && eventTime.After(nextPlannedEvent) {
		context["staleAgeSeconds"] = eventTime.Sub(nextPlannedEvent).Seconds()
	}
	current := mapValue(signalBarState["current"])
	currentClose := parseFloatValue(current["close"])
	if currentClose > 0 {
		context["currentClose"] = currentClose
	}
	if currentBarStart := liveCurrentSignalBarStart(signalBarState, time.Time{}); !currentBarStart.IsZero() {
		context["currentBarStart"] = currentBarStart.UTC().Format(time.RFC3339)
	}
	if nextPlannedPrice > 0 && currentClose > 0 {
		context["currentCloseDeviationBps"] = computePriceProximityBps(nextPlannedPrice, currentClose)
	}
	if nextPlannedPrice > 0 && breakoutPrice > 0 {
		context["breakoutDeviationBps"] = computePriceProximityBps(nextPlannedPrice, breakoutPrice)
	}
	return context
}

func liveZeroInitialWindowPlanStep(
	sessionState map[string]any,
	parameters map[string]any,
	signalBarStates map[string]any,
	symbol string,
	signalTimeframe string,
	eventTime time.Time,
) (map[string]any, time.Time, float64, string, string, string, bool) {
	state := cloneMetadata(sessionState)
	pending := cloneMetadata(mapValue(state[livePendingZeroInitialWindowStateKey]))
	if len(pending) == 0 {
		return state, time.Time{}, 0, "", "", "", false
	}
	if !liveZeroInitialWindowHasBreakoutProof(pending) {
		clearLivePendingZeroInitialWindow(state, eventTime, "zero-initial-window-missing-breakout-proof")
		return state, time.Time{}, 0, "", "", "", false
	}
	side := strings.ToUpper(strings.TrimSpace(stringValue(pending["side"])))
	if side == "" {
		delete(state, livePendingZeroInitialWindowStateKey)
		return state, time.Time{}, 0, "", "", "", false
	}
	if !liveZeroInitialWindowHasSideBreakoutProof(pending, side) {
		clearLivePendingZeroInitialWindow(state, eventTime, "zero-initial-window-side-breakout-proof-mismatch")
		return state, time.Time{}, 0, "", "", "", false
	}
	signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe)
	if signalBarState == nil {
		return state, time.Time{}, 0, "", "", "", false
	}
	price := resolveLiveZeroInitialReentryPlanPrice(parameters, signalBarState, pending, side)
	if price <= 0 {
		return state, time.Time{}, 0, "", "", "", false
	}
	plannedEvent := liveCurrentSignalBarStart(signalBarState, eventTime)
	return state, plannedEvent.UTC(), price, side, "entry", "Zero-Initial-Reentry", true
}

func liveSLReentryWindowPlanStep(
	sessionState map[string]any,
	parameters map[string]any,
	signalBarStates map[string]any,
	symbol string,
	signalTimeframe string,
	eventTime time.Time,
) (map[string]any, time.Time, float64, string, string, string, bool) {
	state := cloneMetadata(sessionState)
	if parseFloatValue(state["sessionReentryCount"]) <= 0 {
		return state, time.Time{}, 0, "", "", "", false
	}
	if strings.TrimSpace(stringValue(state["lastSLExitOrderId"])) == "" || liveSLReentryWindowConsumed(state) {
		return state, time.Time{}, 0, "", "", "", false
	}
	side := strings.ToUpper(strings.TrimSpace(stringValue(state["lastSLExitReentrySide"])))
	if side == "" {
		return state, time.Time{}, 0, "", "", "", false
	}
	lastSLExitAt := parseOptionalRFC3339(stringValue(state["lastSLExitFilledAt"]))
	if lastSLExitAt.IsZero() || lastSLExitAt.UTC().After(eventTime.UTC()) {
		return state, time.Time{}, 0, "", "", "", false
	}
	signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe)
	if signalBarState == nil {
		return state, time.Time{}, 0, "", "", "", false
	}
	currentBarKey := resolveSignalBarTradeLimitKey(signalBarState, symbol, signalTimeframe)
	lastSLBarKey := strings.TrimSpace(stringValue(state["lastSLExitSignalBarStateKey"]))
	if !liveSignalBarKeyWithinReentryWindow(lastSLBarKey, currentBarKey, signalTimeframe) {
		if liveSignalBarKeyPastReentryWindow(lastSLBarKey, currentBarKey, signalTimeframe) {
			consumeLiveSLReentryWindow(state, eventTime, "expired")
		}
		return state, time.Time{}, 0, "", "", "", false
	}
	price := resolveLiveReentryPlanPrice(parameters, signalBarState, side)
	if price <= 0 {
		return state, time.Time{}, 0, "", "", "", false
	}
	return state, liveCurrentSignalBarStart(signalBarState, eventTime), price, side, "entry", "SL-Reentry", true
}

func livePendingZeroInitialWindowShouldYieldSLReentry(
	sessionState map[string]any,
	signalBarStates map[string]any,
	symbol string,
	signalTimeframe string,
	eventTime time.Time,
) bool {
	pending := cloneMetadata(mapValue(sessionState[livePendingZeroInitialWindowStateKey]))
	if len(pending) == 0 || !liveZeroInitialWindowHasBreakoutProof(pending) {
		return false
	}
	if parseFloatValue(sessionState["sessionReentryCount"]) <= 0 {
		return false
	}
	if liveSLReentryWindowConsumed(sessionState) {
		return false
	}
	lastSLExitAt := parseOptionalRFC3339(stringValue(sessionState["lastSLExitFilledAt"]))
	if lastSLExitAt.IsZero() || lastSLExitAt.UTC().After(eventTime.UTC()) {
		return false
	}
	if pendingSymbol := NormalizeSymbol(stringValue(pending["symbol"])); pendingSymbol != "" && pendingSymbol != NormalizeSymbol(symbol) {
		return false
	}
	if pendingTimeframe := strings.ToLower(strings.TrimSpace(stringValue(pending["signalTimeframe"]))); pendingTimeframe != "" &&
		pendingTimeframe != strings.ToLower(strings.TrimSpace(signalTimeframe)) {
		return false
	}
	signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe)
	currentBarKey := resolveSignalBarTradeLimitKey(signalBarState, symbol, signalTimeframe)
	lastSLBarKey := strings.TrimSpace(stringValue(sessionState["lastSLExitSignalBarStateKey"]))
	if currentBarKey == "" || lastSLBarKey == "" || currentBarKey != lastSLBarKey {
		return false
	}
	return true
}

func resolveLiveReentryPlanPrice(parameters map[string]any, signalBarState map[string]any, side string) float64 {
	prevBar1 := mapValue(signalBarState["prevBar1"])
	if prevBar1 == nil {
		return 0
	}
	atr14 := parseFloatValue(signalBarState["atr14"])
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "BUY":
		return parseFloatValue(prevBar1["low"]) + parseFloatValue(firstNonNil(parameters["long_reentry_atr"], 0.1))*atr14
	case "SELL", "SHORT":
		return parseFloatValue(prevBar1["high"]) + parseFloatValue(firstNonNil(parameters["short_reentry_atr"], 0.0))*atr14
	default:
		return 0
	}
}

func resolveLiveZeroInitialReentryPlanPrice(parameters map[string]any, signalBarState map[string]any, pending map[string]any, side string) float64 {
	if liveZeroInitialWindowHasBreakoutProof(pending) {
		if price := parseFloatValue(pending["plannedReentryPrice"]); price > 0 {
			return price
		}
	}
	return resolveLiveReentryPlanPrice(parameters, signalBarState, side)
}

func liveZeroInitialWindowHasSideBreakoutProof(pending map[string]any, side string) bool {
	if !liveZeroInitialWindowHasBreakoutProof(pending) {
		return false
	}
	normalizedSide := strings.ToUpper(strings.TrimSpace(side))
	if breakoutSide := strings.ToUpper(strings.TrimSpace(stringValue(pending["breakoutSide"]))); breakoutSide != "" && breakoutSide != normalizedSide {
		return false
	}
	hasSideProofFields := false
	for _, key := range []string{
		"longBreakoutReady",
		"longBreakoutShapeReady",
		"longBreakoutPriceReady",
		"longBreakoutQualityReady",
		"shortBreakoutReady",
		"shortBreakoutShapeReady",
		"shortBreakoutPriceReady",
		"shortBreakoutQualityReady",
	} {
		if _, ok := pending[key]; ok {
			hasSideProofFields = true
			break
		}
	}
	if !hasSideProofFields {
		return true
	}
	switch normalizedSide {
	case "BUY":
		return boolValue(pending["longBreakoutReady"]) ||
			(boolValue(pending["longBreakoutShapeReady"]) && boolValue(pending["longBreakoutPriceReady"]) && boolValue(pending["longBreakoutQualityReady"]))
	case "SELL", "SHORT":
		return boolValue(pending["shortBreakoutReady"]) ||
			(boolValue(pending["shortBreakoutShapeReady"]) && boolValue(pending["shortBreakoutPriceReady"]) && boolValue(pending["shortBreakoutQualityReady"]))
	default:
		return false
	}
}
