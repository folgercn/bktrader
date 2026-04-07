package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestDeriveLiveSessionIntentUsesNextPlannedStep(t *testing.T) {
	decision := StrategySignalDecision{
		Action: "advance-plan",
		Metadata: map[string]any{
			"signalBarDecision": map[string]any{
				"ready":      true,
				"shortReady": true,
				"ma20":       68000.0,
				"atr14":      1200.0,
			},
			"marketPrice":       67950.0,
			"marketSource":      "order_book.bestBid",
			"signalKind":        "protect-exit",
			"decisionState":     "exit-ready",
			"signalBarStateKey": "binance|BTCUSDT|trigger|4h",
			"nextPlannedSide":   "SELL",
			"nextPlannedRole":   "exit",
			"nextPlannedReason": "PT",
			"nextPlannedEvent":  time.Date(2026, 4, 7, 4, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"nextPlannedPrice":  67900.0,
		},
	}

	intent := deriveLiveSessionIntent(decision, "BTCUSDT")
	if intent == nil {
		t.Fatal("expected intent")
	}
	if got := intent["action"]; got != "exit" {
		t.Fatalf("expected exit action, got %v", got)
	}
	if got := intent["side"]; got != "SELL" {
		t.Fatalf("expected SELL side, got %v", got)
	}
	if got := intent["reason"]; got != "PT" {
		t.Fatalf("expected PT reason, got %v", got)
	}
}

func TestShouldAutoDispatchLiveIntentBlocksOpenOrder(t *testing.T) {
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":              "auto-dispatch",
			"lastDispatchedOrderStatus": "ACCEPTED",
		},
	}
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	if shouldAutoDispatchLiveIntent(session, intent, time.Now().UTC()) {
		t.Fatal("expected open order to block auto dispatch")
	}
}

func TestShouldAutoDispatchLiveIntentAllowsTerminalOrder(t *testing.T) {
	now := time.Now().UTC()
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                  "auto-dispatch",
			"lastDispatchedOrderStatus":     "FILLED",
			"lastDispatchedIntentSignature": "entry|BUY|BTCUSDT|initial-entry|state-0",
			"lastDispatchedAt":              now.Add(-time.Minute).Format(time.RFC3339),
			"dispatchCooldownSeconds":       5,
		},
	}
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	if !shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected terminal order to allow auto dispatch for new intent")
	}
}
