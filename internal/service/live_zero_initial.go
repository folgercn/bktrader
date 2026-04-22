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
	if !strategyZeroInitialReentryWindowEnabled(parameters) {
		alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason := alignLivePlanStepToCurrentMarket(
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
	if staleExitReentry &&
		(hasActiveLivePositionSnapshot(currentPosition) || !isLivePlanStepStale(nextPlannedEvent, signalTimeframe, eventTime)) {
		clearLivePendingZeroInitialWindow(updatedState, eventTime, "exit-reentry-priority")
		return updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	if updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason, ok := liveZeroInitialWindowPlanStep(
		updatedState,
		parameters,
		signalBarStates,
		symbol,
		signalTimeframe,
		eventTime,
	); ok {
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
	gate := evaluateSignalBarGate(signalBarState, "", "entry", "", breakoutPrice, breakoutPriceSource)
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

	current := mapValue(signalBarState["current"])
	currentBarStart := parseOptionalRFC3339(stringValue(current["barStart"]))
	if currentBarStart.IsZero() {
		currentBarStart = eventTime.UTC()
	}
	step := resolutionToDuration(liveSignalResolution(signalTimeframe))
	if step <= 0 {
		step = 4 * time.Hour
	}
	side := "BUY"
	if shortReady {
		side = "SELL"
	}
	pendingWindow := map[string]any{
		"side":                      side,
		"symbol":                    NormalizeSymbol(symbol),
		"signalTimeframe":           strings.ToLower(strings.TrimSpace(signalTimeframe)),
		"armedAt":                   eventTime.UTC().Format(time.RFC3339),
		"signalBarStart":            currentBarStart.UTC().Format(time.RFC3339),
		"expiresAt":                 currentBarStart.UTC().Add(2 * step).Format(time.RFC3339),
		"breakoutBacked":            true,
		"openReason":                liveZeroInitialWindowOpenReasonBreakoutLocked,
		"breakoutPrice":             breakoutPrice,
		"breakoutPriceSource":       strings.TrimSpace(breakoutPriceSource),
		"longStructureReady":        boolValue(gate["longStructureReady"]),
		"shortStructureReady":       boolValue(gate["shortStructureReady"]),
		"longBreakoutReady":         boolValue(gate["longBreakoutReady"]),
		"shortBreakoutReady":        boolValue(gate["shortBreakoutReady"]),
		"longBreakoutPriceReady":    boolValue(gate["longBreakoutPriceReady"]),
		"shortBreakoutPriceReady":   boolValue(gate["shortBreakoutPriceReady"]),
		"longBreakoutShapeReady":    boolValue(gate["longBreakoutShapeReady"]),
		"shortBreakoutShapeReady":   boolValue(gate["shortBreakoutShapeReady"]),
		"longBreakoutPatternReady":  boolValue(gate["longBreakoutPatternReady"]),
		"shortBreakoutPatternReady": boolValue(gate["shortBreakoutPatternReady"]),
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
	updatedState, alignedEvent, alignedPrice, alignedSide, alignedRole, alignedReason, _ := liveZeroInitialWindowPlanStep(updatedState, parameters, signalBarStates, symbol, signalTimeframe, eventTime)
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
	if hasActiveLivePositionSnapshot(currentPosition) {
		clearLivePendingZeroInitialWindow(state, eventTime, "real-position-confirmed")
		return state
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
	expiresAt := parseOptionalRFC3339(stringValue(pending["expiresAt"]))
	if !expiresAt.IsZero() && !eventTime.UTC().Before(expiresAt.UTC()) {
		delete(state, livePendingZeroInitialWindowStateKey)
		return state
	}
	if signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe); signalBarState != nil {
		current := mapValue(signalBarState["current"])
		if currentBarStart := parseOptionalRFC3339(stringValue(current["barStart"])); !currentBarStart.IsZero() {
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
	expiresAt := parseOptionalRFC3339(stringValue(pending["expiresAt"]))
	if !expiresAt.IsZero() && !eventTime.UTC().Before(expiresAt.UTC()) {
		return false
	}
	return true
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
	plannedEvent := parseOptionalRFC3339(stringValue(current["barStart"]))
	if plannedEvent.IsZero() {
		plannedEvent = eventTime.UTC()
	}
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
	if currentBarStart := parseOptionalRFC3339(stringValue(current["barStart"])); !currentBarStart.IsZero() {
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
	signalBarState, _ := pickSignalBarState(signalBarStates, symbol, signalTimeframe)
	if signalBarState == nil {
		return state, time.Time{}, 0, "", "", "", false
	}
	price := resolveLiveReentryPlanPrice(parameters, signalBarState, side)
	if price <= 0 {
		return state, time.Time{}, 0, "", "", "", false
	}
	current := mapValue(signalBarState["current"])
	plannedEvent := parseOptionalRFC3339(stringValue(current["barStart"]))
	if plannedEvent.IsZero() {
		plannedEvent = eventTime.UTC()
	}
	return state, plannedEvent.UTC(), price, side, "entry", "Zero-Initial-Reentry", true
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
