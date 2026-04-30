package domain

import (
	"fmt"
	"strings"
)

type TradingConstraintViolation struct {
	OrderID string `json:"orderId,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type TradingReplayViolation = TradingConstraintViolation

func CheckSignalKindContract(orderID string, signalKind string, intent OrderIntent) (TradingConstraintViolation, bool) {
	normalizedSignalKind := strings.ToLower(strings.TrimSpace(signalKind))
	if normalizedSignalKind == "" {
		return TradingConstraintViolation{}, false
	}

	entrySignalKinds := map[string]bool{
		"initial":              true,
		"initial-entry":        true,
		"zero-initial-reentry": true,
		"sl-reentry":           true,
		"pt-reentry":           true,
		"entry":                true,
	}
	exitSignalKinds := map[string]bool{
		"risk-exit":         true,
		"sl":                true,
		"pt":                true,
		"protect-exit":      true,
		"recovery-watchdog": true,
	}

	if entrySignalKinds[normalizedSignalKind] && !intent.IsEntry() {
		return TradingConstraintViolation{
			OrderID: orderID,
			Code:    ViolationSignalKindIntentMismatch,
			Message: fmt.Sprintf("signalKind %q expects entry intent, got %s", normalizedSignalKind, intent),
		}, true
	}
	if exitSignalKinds[normalizedSignalKind] && !intent.IsExit() {
		return TradingConstraintViolation{
			OrderID: orderID,
			Code:    ViolationSignalKindIntentMismatch,
			Message: fmt.Sprintf("signalKind %q expects exit intent, got %s", normalizedSignalKind, intent),
		}, true
	}
	return TradingConstraintViolation{}, false
}

func CheckReduceOnlyConstraint(order Order, intent OrderIntent, hasMatchingPosition bool) (TradingConstraintViolation, bool) {
	if hasMatchingPosition {
		return TradingConstraintViolation{}, false
	}
	if !order.EffectiveReduceOnly() && !order.EffectiveClosePosition() {
		return TradingConstraintViolation{}, false
	}
	return TradingConstraintViolation{
		OrderID: order.ID,
		Code:    ViolationReduceOnlyWithoutPosition,
		Message: fmt.Sprintf("%s has no matching virtual position", intent),
	}, true
}
