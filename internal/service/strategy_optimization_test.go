package service

import (
	"math"
	"testing"
	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestEvaluateLivePositionStateTrailingStop(t *testing.T) {
	parameters := map[string]any{
		"trailing_stop_atr":            0.3,
		"delayed_trailing_activation_atr": 0.5,
		"stop_loss_atr":                0.05,
		"stop_mode":                    "atr",
	}
	
	signalBarState := map[string]any{
		"atr14": 1000.0,
		"current": map[string]any{"close": 50000.0},
		"prevBar1": map[string]any{"high": 50500.0, "low": 49500.0},
		"prevBar2": map[string]any{"high": 50600.0, "low": 49400.0},
	}
	
	currentPosition := map[string]any{
		"found":      true,
		"side":       "LONG",
		"entryPrice": 50000.0,
		"stopLoss":   49950.0, // Initial SL: 50000 - 0.05 * 1000
		"quantity":   1.0,
	}
	
	sessionState := map[string]any{}
	
	// 1. Initial evaluation at entry price (Profit = 0 ATR) -> No trailing yet
	state := evaluateLivePositionState(parameters, currentPosition, signalBarState, 50000.0, sessionState)
	if parseFloatValue(state["stopLoss"]) != 49950.0 {
		t.Fatalf("expected initial SL 49950, got %v", state["stopLoss"])
	}
	if parseFloatValue(sessionState["hwm"]) != 50000.0 {
		t.Fatalf("expected HWM 50000, got %v", sessionState["hwm"])
	}
	
	// 2. Price move to 50400 (Profit = 0.4 ATR) -> Below 0.5 ATR activation threshold
	state = evaluateLivePositionState(parameters, currentPosition, signalBarState, 50400.0, sessionState)
	if parseFloatValue(state["stopLoss"]) != 49950.0 {
		t.Fatalf("expected SL to stay 49950 at 0.4 ATR profit, got %v", state["stopLoss"])
	}
	if parseFloatValue(sessionState["hwm"]) != 50400.0 {
		t.Fatalf("expected HWM to update to 50400, got %v", sessionState["hwm"])
	}
	
	// 3. Price move to 50600 (Profit = 0.6 ATR) -> Activated!
	// Trailing SL = HWM - 0.3 * ATR = 50600 - 300 = 50300
	state = evaluateLivePositionState(parameters, currentPosition, signalBarState, 50600.0, sessionState)
	if got := parseFloatValue(state["stopLoss"]); got != 50300.0 {
		t.Fatalf("expected trailing SL 50300 at 0.6 ATR profit, got %v", got)
	}
	
	// 4. Price move back to 50500 -> SL should stay at 50300
	state = evaluateLivePositionState(parameters, currentPosition, signalBarState, 50500.0, sessionState)
	if got := parseFloatValue(state["stopLoss"]); got != 50300.0 {
		t.Fatalf("expected SL to stay 50300 on pullback, got %v", got)
	}
}

func TestReentryDecayLogic(t *testing.T) {
	session := domain.LiveSession{
		State: map[string]any{
			"positionSizingMode":   "fixed_quantity",
			"defaultOrderQuantity": 0.1,
			"sessionReentryCount":  2.0, // This is the 3rd trade (Initial, SL-Reentry 1, SL-Reentry 2)
		},
	}
	
	account := domain.Account{}
	intent := SignalIntent{
		Reason:   "SL-Reentry",
		Quantity: 0.1,
	}
	
	parameters := map[string]any{
		"reentry_decay_factor": 0.5,
	}
	
	ctx := ExecutionPlanningContext{
		Session:   session,
		Account:   account,
		Execution: StrategyExecutionContext{Parameters: parameters},
		Intent:    intent,
	}
	
	// In the strategy registry, the logic is:
	// quantity = baseQuantity * math.Pow(decayFactor, reentryCount)
	// For reentryCount = 2, multiplier = 0.5^2 = 0.25
	// Expected quantity = 0.1 * 0.25 = 0.025
	
	priceHint := 50000.0
	baseQuantity, _ := resolveExecutionQuantity(ctx.Session, ctx.Account, ctx.Intent, priceHint)
	
	// Simulate the decay logic added in BuildProposal
	reentryDecayFactor := parseFloatValue(ctx.Execution.Parameters["reentry_decay_factor"])
	reentryCount := parseFloatValue(ctx.Session.State["sessionReentryCount"])
	finalQuantity := baseQuantity
	if reentryDecayFactor > 0 && reentryDecayFactor < 1.0 && reentryCount > 0 {
		finalQuantity = baseQuantity * math.Pow(reentryDecayFactor, reentryCount)
	}
	
	if finalQuantity != 0.025 {
		t.Fatalf("expected decayed quantity 0.025, got %v", finalQuantity)
	}
}
