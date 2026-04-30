package domain

import (
	"fmt"
	"math"
	"strings"
)

const (
	TradingReplayChainOpenLongCloseLong   = "OPEN_LONG_CLOSE_LONG"
	TradingReplayChainOpenShortCloseShort = "OPEN_SHORT_CLOSE_SHORT"

	ViolationExitWithoutPosition       = "EXIT_WITHOUT_POSITION"
	ViolationReduceOnlyWithoutPosition = "REDUCE_ONLY_WITHOUT_POSITION"
	ViolationSignalKindIntentMismatch  = "SIGNAL_KIND_INTENT_MISMATCH"
	ViolationUnsupportedReversal       = "UNSUPPORTED_REVERSAL"
	ViolationQuantityMismatch          = "QUANTITY_MISMATCH"
	ViolationUnknownIntent             = "UNKNOWN_INTENT"

	OrphanReasonNoMatchingPosition = "no_matching_position"

	IgnoredReasonCancelledEntry = "cancelled_entry"
	IgnoredReasonCancelledExit  = "cancelled_exit"
	IgnoredReasonRejectedEntry  = "rejected_entry"
	IgnoredReasonRejectedExit   = "rejected_exit"
	IgnoredReasonNotFilled      = "not_filled"
)

// TradingReplayOrder is the stable JSON input shape for the offline replay harness.
// It intentionally stays smaller than Order while preserving fields needed for
// audit and future quantity-aware replay.
type TradingReplayOrder struct {
	ID            string         `json:"id"`
	Time          string         `json:"time,omitempty"`
	Side          string         `json:"side"`
	ReduceOnly    bool           `json:"reduceOnly,omitempty"`
	ClosePosition bool           `json:"closePosition,omitempty"`
	PositionSide  string         `json:"positionSide,omitempty"`
	Quantity      float64        `json:"quantity,omitempty"`
	Status        string         `json:"status"`
	SignalKind    string         `json:"signalKind,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

func (o TradingReplayOrder) EffectiveSignalKind() string {
	if signalKind := strings.TrimSpace(o.SignalKind); signalKind != "" {
		return signalKind
	}
	if o.Metadata != nil {
		if v, ok := o.Metadata["signalKind"]; ok {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func (o TradingReplayOrder) ToDomainOrder() Order {
	metadata := make(map[string]any, len(o.Metadata)+1)
	for k, v := range o.Metadata {
		metadata[k] = v
	}
	if signalKind := strings.TrimSpace(o.SignalKind); signalKind != "" {
		metadata["signalKind"] = signalKind
	}
	return Order{
		ID:            o.ID,
		Side:          o.Side,
		Status:        o.Status,
		Quantity:      o.Quantity,
		ReduceOnly:    o.ReduceOnly,
		ClosePosition: o.ClosePosition,
		Metadata:      metadata,
	}
}

type TradingReplayResult struct {
	Chains     []TradingReplayChain     `json:"chains"`
	Ignored    []TradingReplayIgnored   `json:"ignored,omitempty"`
	Orphans    []TradingReplayOrphan    `json:"orphans,omitempty"`
	Violations []TradingReplayViolation `json:"violations,omitempty"`
}

type TradingReplayChain struct {
	ChainType     string   `json:"chainType"`
	EntryOrderIDs []string `json:"entryOrderIds,omitempty"`
	ExitOrderIDs  []string `json:"exitOrderIds,omitempty"`
	EntryIntents  []string `json:"entryIntents,omitempty"`
	ExitIntents   []string `json:"exitIntents,omitempty"`
	Display       string   `json:"display"`
}

type TradingReplayIgnored struct {
	OrderID string `json:"orderId"`
	Intent  string `json:"intent"`
	Reason  string `json:"reason"`
}

type TradingReplayOrphan struct {
	OrderID string `json:"orderId"`
	Intent  string `json:"intent"`
	Reason  string `json:"reason"`
}

type replayEntry struct {
	OrderID    string
	Intent     OrderIntent
	SignalKind string
	Quantity   float64
	Remaining  float64
}

func ReplayTradingOrders(orders []TradingReplayOrder) TradingReplayResult {
	result := TradingReplayResult{
		Chains: []TradingReplayChain{},
	}
	var longEntryStack []replayEntry
	var shortEntryStack []replayEntry

	for _, order := range orders {
		domainOrder := order.ToDomainOrder()
		intent := ClassifyOrderIntent(domainOrder)
		intentText := string(intent)
		status := strings.ToUpper(strings.TrimSpace(order.Status))

		if reason, ignored := tradingReplayIgnoredReason(status, intent); ignored {
			result.Ignored = append(result.Ignored, TradingReplayIgnored{
				OrderID: order.ID,
				Intent:  intentText,
				Reason:  reason,
			})
			continue
		}
		if status != "FILLED" {
			result.Ignored = append(result.Ignored, TradingReplayIgnored{
				OrderID: order.ID,
				Intent:  intentText,
				Reason:  IgnoredReasonNotFilled,
			})
			continue
		}

		if violation, ok := CheckSignalKindContract(order.ID, order.EffectiveSignalKind(), intent); ok {
			result.Violations = append(result.Violations, violation)
		}

		switch intent {
		case OrderIntentOpenLong:
			longEntryStack = append(longEntryStack, replayEntryFromOrder(order, intent))
		case OrderIntentOpenShort:
			shortEntryStack = append(shortEntryStack, replayEntryFromOrder(order, intent))
		case OrderIntentCloseLong:
			if len(longEntryStack) == 0 {
				result = appendTradingReplayExitViolation(result, order, domainOrder, intent, len(shortEntryStack) > 0)
				continue
			}
			var chain TradingReplayChain
			var violation TradingReplayViolation
			var ok bool
			chain, longEntryStack, violation, ok = consumeTradingReplayExit(TradingReplayChainOpenLongCloseLong, longEntryStack, order, intent)
			result.Chains = append(result.Chains, chain)
			if !ok {
				result.Violations = append(result.Violations, violation)
			}
		case OrderIntentCloseShort:
			if len(shortEntryStack) == 0 {
				result = appendTradingReplayExitViolation(result, order, domainOrder, intent, len(longEntryStack) > 0)
				continue
			}
			var chain TradingReplayChain
			var violation TradingReplayViolation
			var ok bool
			chain, shortEntryStack, violation, ok = consumeTradingReplayExit(TradingReplayChainOpenShortCloseShort, shortEntryStack, order, intent)
			result.Chains = append(result.Chains, chain)
			if !ok {
				result.Violations = append(result.Violations, violation)
			}
		case OrderIntentUnknown:
			result.Violations = append(result.Violations, TradingReplayViolation{
				OrderID: order.ID,
				Code:    ViolationUnknownIntent,
				Message: "order intent is UNKNOWN",
			})
		}
	}

	return result
}

func tradingReplayIgnoredReason(status string, intent OrderIntent) (string, bool) {
	switch status {
	case "CANCELLED", "CANCELED":
		if intent.IsExit() {
			return IgnoredReasonCancelledExit, true
		}
		return IgnoredReasonCancelledEntry, true
	case "REJECTED":
		if intent.IsExit() {
			return IgnoredReasonRejectedExit, true
		}
		return IgnoredReasonRejectedEntry, true
	default:
		return "", false
	}
}

func replayEntryFromOrder(order TradingReplayOrder, intent OrderIntent) replayEntry {
	return replayEntry{
		OrderID:    order.ID,
		Intent:     intent,
		SignalKind: order.EffectiveSignalKind(),
		Quantity:   order.Quantity,
		Remaining:  order.Quantity,
	}
}

func consumeTradingReplayExit(chainType string, stack []replayEntry, exit TradingReplayOrder, exitIntent OrderIntent) (TradingReplayChain, []replayEntry, TradingReplayViolation, bool) {
	if exit.Quantity <= 0 {
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return tradingReplayChain(chainType, []replayEntry{entry}, exit, exitIntent), stack, TradingReplayViolation{}, true
	}

	remainingExit := exit.Quantity
	consumed := []replayEntry{}
	for remainingExit > tradingReplayQuantityTolerance && len(stack) > 0 {
		idx := len(stack) - 1
		entry := stack[idx]
		entryRemaining := entry.Remaining
		if entryRemaining <= 0 {
			entryRemaining = entry.Quantity
		}
		if entryRemaining <= tradingReplayQuantityTolerance {
			stack = stack[:idx]
			continue
		}

		take := math.Min(entryRemaining, remainingExit)
		entry.Remaining = take
		consumed = append(consumed, entry)
		entryRemaining -= take
		remainingExit -= take

		if entryRemaining <= tradingReplayQuantityTolerance {
			stack = stack[:idx]
		} else {
			stack[idx].Remaining = entryRemaining
		}
	}

	chain := tradingReplayChain(chainType, chronologicalReplayEntries(consumed), exit, exitIntent)
	if remainingExit > tradingReplayQuantityTolerance {
		return chain, stack, TradingReplayViolation{
			OrderID: exit.ID,
			Code:    ViolationQuantityMismatch,
			Message: fmt.Sprintf("%s quantity %.8f exceeds matching virtual position by %.8f", exitIntent, exit.Quantity, remainingExit),
		}, false
	}
	return chain, stack, TradingReplayViolation{}, true
}

const tradingReplayQuantityTolerance = 1e-9

func chronologicalReplayEntries(entries []replayEntry) []replayEntry {
	chronological := make([]replayEntry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		chronological = append(chronological, entries[i])
	}
	return chronological
}

func tradingReplayChain(chainType string, entries []replayEntry, exit TradingReplayOrder, exitIntent OrderIntent) TradingReplayChain {
	chain := TradingReplayChain{
		ChainType:    chainType,
		ExitOrderIDs: []string{exit.ID},
		ExitIntents:  []string{string(exitIntent)},
		Display:      exitIntent.IntentLabel(),
	}
	for _, entry := range entries {
		chain.EntryOrderIDs = append(chain.EntryOrderIDs, entry.OrderID)
		chain.EntryIntents = append(chain.EntryIntents, string(entry.Intent))
		if chain.Display == exitIntent.IntentLabel() {
			chain.Display = entry.Intent.IntentLabel() + " → " + exitIntent.IntentLabel()
		}
	}
	return chain
}

func appendTradingReplayExitViolation(result TradingReplayResult, order TradingReplayOrder, domainOrder Order, intent OrderIntent, oppositePositionOpen bool) TradingReplayResult {
	result.Orphans = append(result.Orphans, TradingReplayOrphan{
		OrderID: order.ID,
		Intent:  string(intent),
		Reason:  OrphanReasonNoMatchingPosition,
	})
	if violation, ok := CheckReduceOnlyConstraint(domainOrder, intent, false); ok {
		result.Violations = append(result.Violations, violation)
	}
	result.Violations = append(result.Violations, TradingReplayViolation{
		OrderID: order.ID,
		Code:    ViolationExitWithoutPosition,
		Message: fmt.Sprintf("%s has no matching virtual position", intent),
	})
	if oppositePositionOpen {
		result.Violations = append(result.Violations, TradingReplayViolation{
			OrderID: order.ID,
			Code:    ViolationUnsupportedReversal,
			Message: fmt.Sprintf("%s cannot close the opposite virtual position in replay v1", intent),
		})
	}
	return result
}
