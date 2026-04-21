package service

import (
	"strings"
	"time"
)

const livePendingZeroInitialWindowStateKey = "pendingZeroInitialWindow"

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
	if liveExitReentryPlanStep(nextPlannedRole, nextPlannedReason) {
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
	gate := evaluateSignalBarGate(signalBarState, "", "entry", "", breakoutPrice, breakoutPriceSource)
	longReady := boolValue(gate["longReady"])
	shortReady := boolValue(gate["shortReady"])
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
		"side":            side,
		"symbol":          NormalizeSymbol(symbol),
		"signalTimeframe": strings.ToLower(strings.TrimSpace(signalTimeframe)),
		"armedAt":         eventTime.UTC().Format(time.RFC3339),
		"signalBarStart":  currentBarStart.UTC().Format(time.RFC3339),
		"expiresAt":       currentBarStart.UTC().Add(2 * step).Format(time.RFC3339),
	}
	updatedState[livePendingZeroInitialWindowStateKey] = pendingWindow
	appendTimelineEvent(updatedState, "strategy", eventTime, "zero-initial-window-armed", map[string]any{
		livePendingZeroInitialWindowStateKey: cloneMetadata(pendingWindow),
		"reason":                             "Zero-Initial-Reentry",
		"side":                               side,
		"symbol":                             NormalizeSymbol(symbol),
		"signalTimeframe":                    strings.ToLower(strings.TrimSpace(signalTimeframe)),
	})
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
