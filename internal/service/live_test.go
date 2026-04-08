package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestDeriveLiveSignalIntentUsesNextPlannedStep(t *testing.T) {
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

	intent := deriveLiveSignalIntent(decision, "BTCUSDT")
	if intent == nil {
		t.Fatal("expected intent")
	}
	if got := intent.Action; got != "exit" {
		t.Fatalf("expected exit action, got %v", got)
	}
	if got := intent.Side; got != "SELL" {
		t.Fatalf("expected SELL side, got %v", got)
	}
	if got := intent.Reason; got != "PT" {
		t.Fatalf("expected PT reason, got %v", got)
	}
}

func TestEvaluateSignalBarGateRequiresLongBreakoutAlignmentWithResearch(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"ma20":  68000.0,
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  68900.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "BUY", "entry")
	if boolValue(gate["longStructureReady"]) != true {
		t.Fatal("expected long structure to be ready")
	}
	if boolValue(gate["longBreakoutReady"]) {
		t.Fatal("expected breakout to remain not ready before current high breaks prevHigh2")
	}
	if boolValue(gate["longReady"]) {
		t.Fatal("expected long signal to stay blocked without breakout confirmation")
	}
	if got := stringValue(gate["reason"]); got != "long-signal-not-ready" {
		t.Fatalf("expected long-signal-not-ready, got %s", got)
	}
}

func TestEvaluateSignalBarGateAllowsLongAfterBreakoutAlignmentWithResearch(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"ma20":  68000.0,
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  69010.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "BUY", "entry")
	if !boolValue(gate["longStructureReady"]) {
		t.Fatal("expected long structure to be ready")
	}
	if !boolValue(gate["longBreakoutReady"]) {
		t.Fatal("expected breakout to be ready after current high breaks prevHigh2")
	}
	if !boolValue(gate["longReady"]) {
		t.Fatal("expected long signal to be ready after breakout confirmation")
	}
}

func TestEvaluateSignalBarGateDoesNotRequireOppositeBreakoutForExit(t *testing.T) {
	gate := evaluateSignalBarGate(map[string]any{
		"ma20":  68000.0,
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68100.0,
			"high":  68900.0,
			"low":   67800.0,
		},
		"prevBar1": map[string]any{
			"high": 68850.0,
			"low":  67750.0,
		},
		"prevBar2": map[string]any{
			"high": 69000.0,
			"low":  67600.0,
		},
	}, "SELL", "exit")
	if !boolValue(gate["ready"]) {
		t.Fatalf("expected exit gate to stay ready, got reason=%s", stringValue(gate["reason"]))
	}
}

func TestEvaluateLiveExitStateRequiresProtectionBeforePT(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
		"protected":  false,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68900.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68900.0, "PT")
	if boolValue(state["ready"]) {
		t.Fatal("expected PT exit to stay blocked before protection is armed")
	}
	if got := stringValue(state["waitReason"]); got != "profit-protection-not-armed" {
		t.Fatalf("expected profit-protection-not-armed, got %s", got)
	}
}

func TestEvaluateLiveExitStateArmsProtectionAndTriggersPTForLong(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
		"protected":  true,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68790.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68790.0, "PT")
	if !boolValue(state["ready"]) {
		t.Fatalf("expected PT exit to trigger once protected and price <= prevLow1, got waitReason=%s", stringValue(state["waitReason"]))
	}
	if got := parseFloatValue(state["targetPrice"]); got != 68800.0 {
		t.Fatalf("expected PT target price 68800, got %v", got)
	}
}

func TestEvaluateLiveExitStateTriggersSLForLong(t *testing.T) {
	state := evaluateLiveExitState(map[string]any{
		"profit_protect_atr": 1.0,
		"stop_loss_atr":      0.05,
		"stop_mode":          "atr",
	}, map[string]any{
		"found":      true,
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"quantity":   0.002,
		"entryPrice": 69000.0,
	}, map[string]any{
		"atr14": 900.0,
		"current": map[string]any{
			"close": 68940.0,
		},
		"prevBar1": map[string]any{
			"high": 69500.0,
			"low":  68800.0,
		},
		"prevBar2": map[string]any{
			"high": 69400.0,
			"low":  68700.0,
		},
	}, 68940.0, "SL")
	if !boolValue(state["ready"]) {
		t.Fatalf("expected SL exit to trigger once price <= stopLoss, got waitReason=%s", stringValue(state["waitReason"]))
	}
}

func TestAdjustLiveExecutionProposalForVirtualInitialWhenZeroInitialEnabled(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{}, map[string]any{
		"dir2_zero_initial": true,
	}, ExecutionProposal{
		Role:   "entry",
		Reason: "Initial",
		Status: "dispatchable",
		Metadata: map[string]any{
			"signalBarStateKey": "state-1",
		},
	})
	if proposal.Status != "virtual-initial" {
		t.Fatalf("expected virtual-initial proposal, got %s", proposal.Status)
	}
	if !boolValue(proposal.Metadata["virtualPosition"]) {
		t.Fatal("expected virtualPosition marker on proposal metadata")
	}
}

func TestAdjustLiveExecutionProposalLeavesReentryDispatchableWhenZeroInitialEnabled(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{}, map[string]any{
		"dir2_zero_initial": true,
	}, ExecutionProposal{
		Role:   "entry",
		Reason: "SL-Reentry",
		Status: "dispatchable",
	})
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected reentry proposal to remain dispatchable, got %s", proposal.Status)
	}
}

func TestAdjustLiveExecutionProposalMarksVirtualExitWhenSessionTracksVirtualPosition(t *testing.T) {
	proposal := adjustLiveExecutionProposalForVirtualSemantics(domain.LiveSession{
		State: map[string]any{
			"virtualPosition": map[string]any{
				"virtual": true,
				"symbol":  "BTCUSDT",
				"side":    "LONG",
			},
		},
	}, map[string]any{
		"dir2_zero_initial": true,
	}, ExecutionProposal{
		Role:   "exit",
		Reason: "PT",
		Status: "dispatchable",
	})
	if proposal.Status != "virtual-exit" {
		t.Fatalf("expected virtual-exit proposal, got %s", proposal.Status)
	}
	if !boolValue(proposal.Metadata["virtualExit"]) {
		t.Fatal("expected virtualExit marker on proposal metadata")
	}
}

func TestResolveLiveSessionPositionSnapshotUsesVirtualPosition(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session := domain.LiveSession{
		AccountID: "live-main",
		State: map[string]any{
			"virtualPosition": map[string]any{
				"symbol":     "BTCUSDT",
				"side":       "LONG",
				"quantity":   0.0,
				"entryPrice": 69000.0,
				"virtual":    true,
			},
		},
	}
	position, found, err := platform.resolveLiveSessionPositionSnapshot(session, "BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected virtual position to be treated as found")
	}
	if !boolValue(position["virtual"]) {
		t.Fatal("expected returned position snapshot to be marked virtual")
	}
}

func TestShouldAutoDispatchLiveIntentBlocksRepeatedVirtualInitialWithinCooldown(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                  "auto-dispatch",
			"lastDispatchedOrderStatus":     liveOrderStatusVirtualInitial,
			"lastDispatchedIntentSignature": signature,
			"lastDispatchedAt":              now.Format(time.RFC3339),
			"dispatchCooldownSeconds":       300,
		},
	}
	if shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected repeated virtual initial signal to be cooled down")
	}
}

func TestReconcileLivePlanIndexKeepsExitWhenVirtualPositionExists(t *testing.T) {
	plan := []paperPlannedOrder{
		{Role: "entry"},
		{Role: "exit"},
	}
	nextIndex, adjusted := reconcileLivePlanIndexWithPosition(plan, 1, map[string]any{
		"virtual":  true,
		"quantity": 0.0,
	}, true)
	if adjusted {
		t.Fatalf("expected no rewind when virtual position exists, got nextIndex=%d", nextIndex)
	}
}

func TestBookAwareExecutionStrategyBuildsProposalFromOrderBook(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"defaultOrderQuantity": 0.002,
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps": 8.0,
			},
		},
		Intent: SignalIntent{
			Action:        "entry",
			Role:          "entry",
			Reason:        "Initial",
			Side:          "BUY",
			Symbol:        "BTCUSDT",
			SignalKind:    "entry",
			DecisionState: "entry-ready",
			PriceHint:     68000,
			PriceSource:   "trade_tick.price",
			Quantity:      0.001,
			Metadata: map[string]any{
				"bestBid":           67999.5,
				"bestAsk":           68000.5,
				"spreadBps":         0.15,
				"signalBarStateKey": "state-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected dispatchable proposal, got %s", proposal.Status)
	}
	if proposal.PriceHint != 68000.5 {
		t.Fatalf("expected best ask price hint, got %v", proposal.PriceHint)
	}
	if proposal.Quantity != 0.002 {
		t.Fatalf("expected session default quantity to win, got %v", proposal.Quantity)
	}
}

func TestBookAwareExecutionStrategyBuildsProposalFromBalanceFraction(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"positionSizingMode":   "fixed_fraction",
				"defaultOrderFraction": 0.1,
				"defaultOrderQuantity": 0.002,
			},
		},
		Account: domain.Account{
			Metadata: map[string]any{
				"liveSyncSnapshot": map[string]any{
					"availableBalance": 1000.0,
				},
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps": 8.0,
			},
		},
		Intent: SignalIntent{
			Action:        "entry",
			Role:          "entry",
			Reason:        "SL-Reentry",
			Side:          "BUY",
			Symbol:        "BTCUSDT",
			SignalKind:    "sl-reentry",
			DecisionState: "entry-ready",
			PriceHint:     50000,
			PriceSource:   "trade_tick.price",
			Quantity:      0.001,
			Metadata: map[string]any{
				"bestBid":           49999.5,
				"bestAsk":           50000.0,
				"spreadBps":         0.1,
				"signalBarStateKey": "state-fraction",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Quantity != 0.002 {
		t.Fatalf("expected fraction-based quantity 0.002, got %v", proposal.Quantity)
	}
	if got := stringValue(proposal.Metadata["positionSizingMode"]); got != "fixed_fraction" {
		t.Fatalf("expected fixed_fraction metadata, got %s", got)
	}
	if got := stringValue(proposal.Metadata["sizingBalanceBasis"]); got != "availableBalance" {
		t.Fatalf("expected availableBalance sizing basis, got %s", got)
	}
}

func TestBookAwareExecutionStrategyUsesReduceOnlyMakerProfileForPTExit(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"defaultOrderQuantity": 0.002,
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionPTExitOrderType":                "LIMIT",
				"executionPTExitPostOnly":                 true,
				"executionPTExitTimeInForce":              "GTX",
				"executionPTExitWideSpreadMode":           "limit-maker",
				"executionPTExitTimeoutFallbackOrderType": "MARKET",
			},
		},
		Intent: SignalIntent{
			Action:        "exit",
			Role:          "exit",
			Reason:        "PT",
			Side:          "SELL",
			Symbol:        "BTCUSDT",
			SignalKind:    "protect-exit",
			DecisionState: "exit-ready",
			PriceHint:     68800,
			PriceSource:   "order_book.bestBid",
			Metadata: map[string]any{
				"bestBid":           68800.0,
				"bestAsk":           68801.0,
				"spreadBps":         0.2,
				"signalBarStateKey": "state-exit-pt",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "LIMIT" {
		t.Fatalf("expected PT exit to prefer LIMIT, got %s", proposal.Type)
	}
	if !proposal.PostOnly || proposal.TimeInForce != "GTX" {
		t.Fatalf("expected PT exit to be post-only GTX, got postOnly=%v tif=%s", proposal.PostOnly, proposal.TimeInForce)
	}
	if !proposal.ReduceOnly {
		t.Fatal("expected PT exit proposal to be reduceOnly")
	}
}

func TestBookAwareExecutionStrategyUsesAggressiveReduceOnlyProfileForSLExit(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"defaultOrderQuantity": 0.002,
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionOrderType":                "LIMIT",
				"executionWideSpreadMode":           "limit-maker",
				"executionTimeoutFallbackOrderType": "LIMIT",
			},
		},
		Intent: SignalIntent{
			Action:        "exit",
			Role:          "exit",
			Reason:        "SL",
			Side:          "SELL",
			Symbol:        "BTCUSDT",
			SignalKind:    "risk-exit",
			DecisionState: "exit-ready",
			PriceHint:     68900,
			PriceSource:   "order_book.bestBid",
			Metadata: map[string]any{
				"bestBid":           68900.0,
				"bestAsk":           68902.0,
				"spreadBps":         25.0,
				"signalBarStateKey": "state-exit-sl",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "MARKET" {
		t.Fatalf("expected SL exit to force MARKET, got %s", proposal.Type)
	}
	if proposal.PostOnly {
		t.Fatal("expected SL exit not to be post only")
	}
	if !proposal.ReduceOnly {
		t.Fatal("expected SL exit proposal to be reduceOnly")
	}
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected SL exit to stay dispatchable despite wide spread, got %s", proposal.Status)
	}
}

func TestBookAwareExecutionStrategyWaitsWhenSpreadTooWide(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps": 5.0,
			},
		},
		Intent: SignalIntent{
			Action:      "entry",
			Role:        "entry",
			Reason:      "Initial",
			Side:        "BUY",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "trade_tick.price",
			Metadata: map[string]any{
				"bestAsk":   68010.0,
				"spreadBps": 12.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "wait" {
		t.Fatalf("expected wait proposal, got %s", proposal.Status)
	}
	if proposal.Reason != "spread-too-wide" {
		t.Fatalf("expected spread-too-wide, got %s", proposal.Reason)
	}
}

func TestBookAwareExecutionStrategyUsesMakerLimitOnWideSpreadWhenConfigured(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	eventTime := time.Date(2026, 4, 7, 8, 0, 0, 0, time.UTC)
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps":          5.0,
				"executionWideSpreadMode":        "limit-maker",
				"executionRestingTimeoutSeconds": 45,
			},
		},
		EventTime: eventTime,
		Intent: SignalIntent{
			Action:      "entry",
			Role:        "entry",
			Reason:      "Initial",
			Side:        "BUY",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "trade_tick.price",
			Metadata: map[string]any{
				"bestBid":           67999.0,
				"bestAsk":           68005.0,
				"spreadBps":         12.0,
				"signalBarStateKey": "state-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Status != "dispatchable" {
		t.Fatalf("expected dispatchable proposal, got %s", proposal.Status)
	}
	if proposal.Type != "LIMIT" {
		t.Fatalf("expected LIMIT proposal, got %s", proposal.Type)
	}
	if proposal.LimitPrice != 67999.0 {
		t.Fatalf("expected passive bid limit price, got %v", proposal.LimitPrice)
	}
	if proposal.TimeInForce != "GTX" || !proposal.PostOnly {
		t.Fatalf("expected GTX post-only maker order, got tif=%s postOnly=%v", proposal.TimeInForce, proposal.PostOnly)
	}
	if got := stringValue(proposal.Metadata["executionExpiresAt"]); got != eventTime.Add(45*time.Second).Format(time.RFC3339) {
		t.Fatalf("expected execution expiry to be set, got %s", got)
	}
}

func TestBookAwareExecutionStrategyUsesFallbackOrderAfterTimeoutMatch(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	intent := SignalIntent{
		Action:      "entry",
		Role:        "entry",
		Reason:      "Initial",
		Side:        "BUY",
		Symbol:      "BTCUSDT",
		SignalKind:  "entry",
		PriceHint:   68000,
		PriceSource: "trade_tick.price",
		Metadata: map[string]any{
			"bestBid":           67999.0,
			"bestAsk":           68005.0,
			"spreadBps":         12.0,
			"signalBarStateKey": "state-1",
		},
	}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{
			State: map[string]any{
				"lastExecutionTimeoutIntentSignature": buildLiveIntentSignature(map[string]any{
					"action":            "entry",
					"side":              "BUY",
					"symbol":            "BTCUSDT",
					"signalKind":        "entry",
					"signalBarStateKey": "state-1",
				}),
			},
		},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionMaxSpreadBps":             5.0,
				"executionTimeoutFallbackOrderType": "MARKET",
			},
		},
		Intent: intent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "MARKET" {
		t.Fatalf("expected MARKET fallback proposal, got %s", proposal.Type)
	}
	if !boolValue(proposal.Metadata["fallbackFromTimeout"]) {
		t.Fatal("expected proposal to record fallbackFromTimeout")
	}
}

func TestBookAwareExecutionStrategyBuildsLimitProposalWhenConfigured(t *testing.T) {
	strategy := bookAwareExecutionStrategy{}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session: domain.LiveSession{},
		Execution: StrategyExecutionContext{
			Parameters: map[string]any{
				"executionOrderType":   "LIMIT",
				"executionTimeInForce": "IOC",
				"executionPostOnly":    true,
			},
		},
		Intent: SignalIntent{
			Action:      "entry",
			Role:        "entry",
			Reason:      "Initial",
			Side:        "BUY",
			Symbol:      "BTCUSDT",
			PriceHint:   68000,
			PriceSource: "trade_tick.price",
			Metadata: map[string]any{
				"bestAsk":   68001.0,
				"spreadBps": 1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal.Type != "LIMIT" {
		t.Fatalf("expected LIMIT proposal, got %s", proposal.Type)
	}
	if proposal.LimitPrice != 68001.0 {
		t.Fatalf("expected best ask as limit price, got %v", proposal.LimitPrice)
	}
	if proposal.TimeInForce != "GTX" {
		t.Fatalf("expected GTX time in force for post only limit, got %s", proposal.TimeInForce)
	}
	if !proposal.PostOnly {
		t.Fatal("expected post only proposal")
	}
}

func TestBuildLiveOrderFromExecutionProposalUsesExecutionFields(t *testing.T) {
	session := domain.LiveSession{
		ID:        "live-session-1",
		AccountID: "live-main",
		State: map[string]any{
			"dispatchMode": "auto-dispatch",
		},
	}
	proposal := ExecutionProposal{
		Side:              "BUY",
		Symbol:            "BTCUSDT",
		Type:              "LIMIT",
		Quantity:          0.002,
		LimitPrice:        68001.0,
		PriceHint:         68001.5,
		TimeInForce:       "GTX",
		PostOnly:          true,
		ReduceOnly:        true,
		SignalKind:        "entry",
		ExecutionStrategy: "book-aware-v1",
	}
	order := buildLiveOrderFromExecutionProposal(session, "strategy-version-1", proposal, executionProposalToMap(proposal))
	if order.Type != "LIMIT" {
		t.Fatalf("expected LIMIT order, got %s", order.Type)
	}
	if order.Price != 68001.0 {
		t.Fatalf("expected limit price to be used, got %v", order.Price)
	}
	if got := stringValue(order.Metadata["timeInForce"]); got != "GTX" {
		t.Fatalf("expected GTX in metadata, got %s", got)
	}
	if !boolValue(order.Metadata["postOnly"]) {
		t.Fatal("expected postOnly metadata")
	}
	if !boolValue(order.Metadata["reduceOnly"]) {
		t.Fatal("expected reduceOnly metadata")
	}
}

func TestBuildLiveOrderUsesProposalQuantityOverSessionDefault(t *testing.T) {
	session := domain.LiveSession{
		ID:        "live-session-1",
		AccountID: "live-main",
		State: map[string]any{
			"symbol":               "BTCUSDT",
			"defaultOrderQuantity": 0.01,
			"dispatchMode":         "manual-review",
		},
	}
	proposal := ExecutionProposal{
		Action:            "entry",
		Role:              "entry",
		Reason:            "SL-Reentry",
		Side:              "BUY",
		Symbol:            "BTCUSDT",
		Type:              "MARKET",
		Quantity:          0.002,
		PriceHint:         68000,
		ExecutionStrategy: "book-aware-v1",
		SignalKind:        "sl-reentry",
	}
	order := buildLiveOrderFromExecutionProposal(session, "strategy-version-1", proposal, executionProposalToMap(proposal))
	if order.Quantity != 0.002 {
		t.Fatalf("expected proposal quantity to win, got %v", order.Quantity)
	}
}

func TestDispatchLiveSessionIntentRejectsNonDispatchableProposal(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session := domain.LiveSession{
		ID:         "live-session-1",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "RUNNING",
		State: map[string]any{
			"lastExecutionProposal": map[string]any{
				"status": "wait",
				"side":   "BUY",
				"symbol": "BTCUSDT",
			},
		},
	}
	if _, err := platform.dispatchLiveSessionIntent(session); err == nil {
		t.Fatal("expected non-dispatchable proposal to be rejected")
	}
}

func TestNormalizeLiveSessionOverridesIncludesExecutionControls(t *testing.T) {
	overrides := normalizeLiveSessionOverrides(map[string]any{
		"executionStrategy":                   "book-aware-v1",
		"executionOrderType":                  "limit",
		"executionTimeInForce":                "ioc",
		"executionPostOnly":                   true,
		"executionMaxSpreadBps":               6.5,
		"executionWideSpreadMode":             "limit-maker",
		"executionRestingTimeoutSeconds":      25,
		"executionTimeoutFallbackOrderType":   "market",
		"executionTimeoutFallbackTimeInForce": "fok",
		"executionPTExitOrderType":            "limit",
		"executionPTExitPostOnly":             true,
		"executionPTExitTimeInForce":          "gtx",
		"executionSLExitOrderType":            "market",
		"executionSLExitMaxSpreadBps":         999.0,
	})
	if got := stringValue(overrides["executionStrategy"]); got != "book-aware-v1" {
		t.Fatalf("expected execution strategy override, got %s", got)
	}
	if got := stringValue(overrides["executionOrderType"]); got != "limit" {
		t.Fatalf("expected execution order type override, got %s", got)
	}
	if got := stringValue(overrides["executionTimeInForce"]); got != "IOC" {
		t.Fatalf("expected uppercase time in force, got %s", got)
	}
	if !boolValue(overrides["executionPostOnly"]) {
		t.Fatal("expected executionPostOnly override")
	}
	if got := parseFloatValue(overrides["executionMaxSpreadBps"]); got != 6.5 {
		t.Fatalf("expected execution max spread override, got %v", got)
	}
	if got := stringValue(overrides["executionWideSpreadMode"]); got != "limit-maker" {
		t.Fatalf("expected wide spread mode override, got %s", got)
	}
	if got := maxIntValue(overrides["executionRestingTimeoutSeconds"], 0); got != 25 {
		t.Fatalf("expected resting timeout override, got %d", got)
	}
	if got := stringValue(overrides["executionTimeoutFallbackOrderType"]); got != "MARKET" {
		t.Fatalf("expected uppercase fallback order type, got %s", got)
	}
	if got := stringValue(overrides["executionTimeoutFallbackTimeInForce"]); got != "FOK" {
		t.Fatalf("expected uppercase fallback tif, got %s", got)
	}
	if got := stringValue(overrides["executionPTExitOrderType"]); got != "LIMIT" {
		t.Fatalf("expected PT exit order type override, got %s", got)
	}
	if !boolValue(overrides["executionPTExitPostOnly"]) {
		t.Fatal("expected PT exit post only override")
	}
	if got := stringValue(overrides["executionPTExitTimeInForce"]); got != "GTX" {
		t.Fatalf("expected PT exit tif override, got %s", got)
	}
	if got := stringValue(overrides["executionSLExitOrderType"]); got != "MARKET" {
		t.Fatalf("expected SL exit order type override, got %s", got)
	}
	if got := parseFloatValue(overrides["executionSLExitMaxSpreadBps"]); got != 999.0 {
		t.Fatalf("expected SL exit max spread override, got %v", got)
	}
}

func TestNormalizeLiveSessionOverridesIncludesPositionSizing(t *testing.T) {
	overrides := normalizeLiveSessionOverrides(map[string]any{
		"positionSizingMode":   "fixed-fraction",
		"defaultOrderFraction": 0.12,
	})
	if got := stringValue(overrides["positionSizingMode"]); got != "fixed_fraction" {
		t.Fatalf("expected fixed_fraction mode, got %s", got)
	}
	if got := parseFloatValue(overrides["defaultOrderFraction"]); got != 0.12 {
		t.Fatalf("expected default order fraction 0.12, got %v", got)
	}
}

func TestCreateLiveSessionAppliesExecutionOverrides(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":                              "BTCUSDT",
		"executionStrategy":                   "book-aware-v1",
		"executionOrderType":                  "LIMIT",
		"executionTimeInForce":                "IOC",
		"executionPostOnly":                   true,
		"executionWideSpreadMode":             "limit-maker",
		"executionRestingTimeoutSeconds":      30,
		"executionTimeoutFallbackOrderType":   "MARKET",
		"executionTimeoutFallbackTimeInForce": "FOK",
		"executionPTExitOrderType":            "LIMIT",
		"executionPTExitPostOnly":             true,
		"executionPTExitTimeInForce":          "GTX",
		"executionSLExitOrderType":            "MARKET",
		"executionSLExitMaxSpreadBps":         999.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stringValue(session.State["executionStrategy"]); got != "book-aware-v1" {
		t.Fatalf("expected execution strategy in session state, got %s", got)
	}
	if got := stringValue(session.State["executionOrderType"]); got != "LIMIT" {
		t.Fatalf("expected execution order type in session state, got %s", got)
	}
	if got := stringValue(session.State["executionTimeInForce"]); got != "IOC" {
		t.Fatalf("expected execution tif in session state, got %s", got)
	}
	if !boolValue(session.State["executionPostOnly"]) {
		t.Fatal("expected executionPostOnly in session state")
	}
	if got := stringValue(session.State["executionWideSpreadMode"]); got != "limit-maker" {
		t.Fatalf("expected executionWideSpreadMode in session state, got %s", got)
	}
	if got := maxIntValue(session.State["executionRestingTimeoutSeconds"], 0); got != 30 {
		t.Fatalf("expected resting timeout in session state, got %d", got)
	}
	if got := stringValue(session.State["executionTimeoutFallbackOrderType"]); got != "MARKET" {
		t.Fatalf("expected fallback order type in session state, got %s", got)
	}
	if got := stringValue(session.State["executionTimeoutFallbackTimeInForce"]); got != "FOK" {
		t.Fatalf("expected fallback tif in session state, got %s", got)
	}
	if got := stringValue(session.State["executionPTExitOrderType"]); got != "LIMIT" {
		t.Fatalf("expected PT exit order type in session state, got %s", got)
	}
	if !boolValue(session.State["executionPTExitPostOnly"]) {
		t.Fatal("expected PT exit post only in session state")
	}
	if got := stringValue(session.State["executionSLExitOrderType"]); got != "MARKET" {
		t.Fatalf("expected SL exit order type in session state, got %s", got)
	}
	if got := parseFloatValue(session.State["executionSLExitMaxSpreadBps"]); got != 999.0 {
		t.Fatalf("expected SL exit max spread in session state, got %v", got)
	}
}

func TestStartSignalRuntimeSessionRefreshesPlanFromLatestBindings(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy 1d failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account 1d failed: %v", err)
	}
	session, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "4h"},
	}); err != nil {
		t.Fatalf("rebind strategy 4h failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "4h"},
	}); err != nil {
		t.Fatalf("rebind account 4h failed: %v", err)
	}
	started, err := platform.StartSignalRuntimeSession(session.ID)
	if err != nil {
		t.Fatalf("start runtime session failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = platform.StopSignalRuntimeSession(session.ID)
	})
	subscriptions := metadataList(started.State["subscriptions"])
	if len(subscriptions) == 0 {
		t.Fatal("expected subscriptions after runtime start")
	}
	if got := stringValue(subscriptions[0]["channel"]); got != "btcusdt@kline_4h" {
		t.Fatalf("expected refreshed 4h subscription, got %s", got)
	}
}

func TestShouldCancelLiveOrderForExecutionTimeout(t *testing.T) {
	now := time.Date(2026, 4, 7, 8, 30, 0, 0, time.UTC)
	order := domain.Order{
		Status: "NEW",
		Metadata: map[string]any{
			"executionExpiresAt": now.Add(-time.Second).Format(time.RFC3339),
		},
	}
	if !shouldCancelLiveOrderForExecutionTimeout(order, now) {
		t.Fatal("expected expired live order to be cancelled")
	}
}

func TestShouldAutoDispatchLiveIntentAllowsRetryAfterExecutionTimeout(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                        "auto-dispatch",
			"lastDispatchedOrderStatus":           "CANCELLED",
			"lastSyncedOrderStatus":               "CANCELLED",
			"lastDispatchedIntentSignature":       signature,
			"lastExecutionTimeoutIntentSignature": signature,
			"lastDispatchedAt":                    now.Format(time.RFC3339),
			"dispatchCooldownSeconds":             300,
		},
	}
	if !shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected timeout-cancelled intent to be eligible for immediate retry")
	}
}

func TestShouldAutoDispatchLiveIntentAllowsRetryAfterMakerRejectFallback(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "initial-entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                        "auto-dispatch",
			"lastDispatchedOrderStatus":           "REJECTED",
			"lastSyncedOrderStatus":               "REJECTED",
			"lastDispatchedIntentSignature":       signature,
			"lastExecutionTimeoutIntentSignature": signature,
			"lastExecutionTimeoutReason":          "maker-rejected-post-only",
			"lastDispatchedAt":                    now.Format(time.RFC3339),
			"dispatchCooldownSeconds":             300,
		},
	}
	if !shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected maker-rejected intent to be eligible for immediate fallback retry")
	}
}

func TestShouldAutoDispatchLiveIntentBlocksImmediateRetryAfterDispatchError(t *testing.T) {
	now := time.Now().UTC()
	intent := map[string]any{
		"action":            "entry",
		"side":              "BUY",
		"symbol":            "BTCUSDT",
		"signalKind":        "entry",
		"signalBarStateKey": "state-1",
	}
	signature := buildLiveIntentSignature(intent)
	session := domain.LiveSession{
		State: map[string]any{
			"dispatchMode":                  "auto-dispatch",
			"lastDispatchedOrderStatus":     "REJECTED",
			"lastDispatchedIntentSignature": signature,
			"lastDispatchedAt":              now.Format(time.RFC3339),
			"dispatchCooldownSeconds":       30,
		},
	}
	if shouldAutoDispatchLiveIntent(session, intent, now) {
		t.Fatal("expected immediate retry after dispatch error to be blocked by cooldown")
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

func TestShouldAdvanceLivePlanForOrderStatus(t *testing.T) {
	if shouldAdvanceLivePlanForOrderStatus("REJECTED") {
		t.Fatal("expected rejected live order to keep the current plan step actionable")
	}
	if !shouldAdvanceLivePlanForOrderStatus("NEW") {
		t.Fatal("expected accepted/in-flight live order to advance the plan")
	}
	if !shouldAdvanceLivePlanForOrderStatus("FILLED") {
		t.Fatal("expected filled live order to advance the plan")
	}
}

func TestParseBinanceSymbolRules(t *testing.T) {
	rules := parseBinanceSymbolRules(map[string]any{
		"symbol": "BTCUSDT",
		"filters": []any{
			map[string]any{
				"filterType": "PRICE_FILTER",
				"tickSize":   "0.10",
			},
			map[string]any{
				"filterType": "LOT_SIZE",
				"stepSize":   "0.001",
				"minQty":     "0.001",
				"maxQty":     "1000",
			},
			map[string]any{
				"filterType":  "MIN_NOTIONAL",
				"minNotional": "100",
			},
		},
	})
	if rules.TickSize != 0.1 {
		t.Fatalf("expected tick size 0.1, got %v", rules.TickSize)
	}
	if rules.StepSize != 0.001 {
		t.Fatalf("expected step size 0.001, got %v", rules.StepSize)
	}
	if rules.MinQty != 0.001 {
		t.Fatalf("expected min qty 0.001, got %v", rules.MinQty)
	}
	if rules.MinNotional != 100 {
		t.Fatalf("expected min notional 100, got %v", rules.MinNotional)
	}
}

func TestNormalizeBinancePriceAndQuantity(t *testing.T) {
	rules := binanceSymbolRules{
		TickSize: 0.1,
		StepSize: 0.001,
		MinQty:   0.001,
	}
	if got := normalizeBinancePrice(68643.67, rules); got != 68643.6 {
		t.Fatalf("expected price rounded down to tick size, got %v", got)
	}
	if got := normalizeBinanceQuantity(0.0019, rules); got != 0.001 {
		t.Fatalf("expected quantity rounded down to step size, got %v", got)
	}
	if got := normalizeBinanceQuantity(0.0004, rules); got != 0.001 {
		t.Fatalf("expected min quantity clamp, got %v", got)
	}
}

func TestFormatBinanceDecimalUsesExchangePrecision(t *testing.T) {
	if got := formatBinanceDecimal(68915.4, 0.1); got != "68915.4" {
		t.Fatalf("expected formatted price without float noise, got %s", got)
	}
	if got := formatBinanceDecimal(0.0015, 0.0001); got != "0.0015" {
		t.Fatalf("expected formatted quantity without float noise, got %s", got)
	}
}

func TestNormalizeBinanceQuantityForMinNotional(t *testing.T) {
	rules := binanceSymbolRules{
		StepSize:    0.001,
		MinQty:      0.001,
		MinNotional: 100,
	}
	if got := normalizeBinanceQuantityForMinNotional(0.001, 68643.6, rules); got != 0.002 {
		t.Fatalf("expected quantity bumped to 0.002 for min notional, got %v", got)
	}
	if got := normalizeBinanceQuantityForMinNotional(0.002, 68643.6, rules); got != 0.002 {
		t.Fatalf("expected existing quantity to remain unchanged, got %v", got)
	}
}

func TestShouldMarkLiveExecutionFallback(t *testing.T) {
	order := domain.Order{
		Status: "REJECTED",
		Metadata: map[string]any{
			"liveSubmitError": "binance request failed: 400 Bad Request {\"code\":-5022,\"msg\":\"Due to the order could not be executed as maker\"}",
		},
	}
	if !shouldMarkLiveExecutionFallback(order) {
		t.Fatal("expected maker-rejected post-only order to mark fallback eligibility")
	}
}

func TestEvaluateLiveSessionOnSignalRecordsVirtualInitialForZeroInitialStrategy(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 7, 9, 0, 0, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime,
		Price:     69000.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "Initial",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              69010.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["health"] = "healthy"
		state["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastEventSummary"] = cloneMetadata(summary)
		state["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 69010.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		state["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"close": 68100.0,
					"high":  69010.0,
					"low":   67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastDispatchedOrderStatus"]); got != liveOrderStatusVirtualInitial {
		t.Fatalf("expected virtual initial dispatch marker, got %s", got)
	}
	if got := stringValue(updated.State["lastVirtualSignalType"]); got != "initial" {
		t.Fatalf("expected lastVirtualSignalType=initial, got %s", got)
	}
	if !boolValue(mapValue(updated.State["virtualPosition"])["virtual"]) {
		t.Fatal("expected virtualPosition to be recorded in live session state")
	}
	if got := maxIntValue(updated.State["planIndex"], -1); got != 1 {
		t.Fatalf("expected planIndex to advance after virtual initial, got %d", got)
	}
}

func TestEvaluateLiveSessionOnSignalKeepsReentryDispatchable(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}
	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind strategy trigger failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind account signal failed: %v", err)
	}
	if _, err := platform.BindAccountSignalSource("live-main", map[string]any{
		"sourceKey": "binance-trade-tick",
		"role":      "trigger",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind account trigger failed: %v", err)
	}

	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":              "BTCUSDT",
		"signalTimeframe":     "1d",
		"executionDataSource": "tick",
		"dispatchMode":        "manual-review",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		t.Fatal("expected linked runtime session id")
	}

	eventTime := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)
	platform.mu.Lock()
	platform.livePlans[session.ID] = []paperPlannedOrder{{
		EventTime: eventTime,
		Price:     68900.0,
		Side:      "BUY",
		Role:      "entry",
		Reason:    "SL-Reentry",
	}}
	platform.mu.Unlock()

	signalKey := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT")
	triggerKey := signalBindingMatchKey("binance-trade-tick", "trigger", "BTCUSDT")
	summary := map[string]any{
		"role":               "trigger",
		"symbol":             "BTCUSDT",
		"subscriptionSymbol": "BTCUSDT",
		"price":              69010.0,
		"event":              "trade_tick",
	}
	err = platform.updateSignalRuntimeSessionState(runtimeSessionID, func(runtimeSession *domain.SignalRuntimeSession) {
		runtimeSession.Status = "RUNNING"
		state := cloneMetadata(runtimeSession.State)
		state["health"] = "healthy"
		state["lastEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastHeartbeatAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastEventSummary"] = cloneMetadata(summary)
		state["sourceStates"] = map[string]any{
			triggerKey: map[string]any{
				"sourceKey":   "binance-trade-tick",
				"role":        "trigger",
				"symbol":      "BTCUSDT",
				"streamType":  "trade_tick",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
				"summary": map[string]any{
					"price": 69010.0,
				},
			},
			signalKey: map[string]any{
				"sourceKey":   "binance-kline",
				"role":        "signal",
				"symbol":      "BTCUSDT",
				"streamType":  "signal_bar",
				"lastEventAt": eventTime.UTC().Format(time.RFC3339),
			},
		}
		state["signalBarStates"] = map[string]any{
			signalKey: map[string]any{
				"symbol":    "BTCUSDT",
				"timeframe": "1d",
				"ma20":      68000.0,
				"atr14":     900.0,
				"current": map[string]any{
					"close": 68100.0,
					"high":  69010.0,
					"low":   67800.0,
				},
				"prevBar1": map[string]any{
					"high": 68850.0,
					"low":  67750.0,
				},
				"prevBar2": map[string]any{
					"high": 69000.0,
					"low":  67600.0,
				},
			},
		}
		runtimeSession.State = state
		runtimeSession.UpdatedAt = eventTime
	})
	if err != nil {
		t.Fatalf("update runtime state failed: %v", err)
	}

	if err := platform.evaluateLiveSessionOnSignal(session, runtimeSessionID, summary, eventTime); err != nil {
		t.Fatalf("evaluate live session failed: %v", err)
	}

	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get updated live session failed: %v", err)
	}
	proposal := mapValue(updated.State["lastExecutionProposal"])
	if proposal == nil {
		t.Fatal("expected lastExecutionProposal to be recorded")
	}
	if got := stringValue(proposal["status"]); got != "dispatchable" {
		t.Fatalf("expected reentry proposal to remain dispatchable, got %s", got)
	}
	if got := stringValue(proposal["reason"]); got != "SL-Reentry" {
		t.Fatalf("expected SL-Reentry proposal, got %s", got)
	}
	if boolValue(mapValue(proposal["metadata"])["virtualPosition"]) {
		t.Fatal("expected reentry proposal to stay non-virtual")
	}
}

func TestRefreshLiveSessionPositionContextRebuildsLivePositionState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"openOrders": []map[string]any{
			{
				"symbol":        "BTCUSDT",
				"origType":      "TAKE_PROFIT_MARKET",
				"reduceOnly":    true,
				"closePosition": true,
			},
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         70000,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	session, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["lastStrategyEvaluationSignalBarStates"] = map[string]any{
		signalBindingMatchKey("binance-kline", "signal", "BTCUSDT"): map[string]any{
			"symbol":    "BTCUSDT",
			"timeframe": "1d",
			"ma20":      68000.0,
			"atr14":     900.0,
			"current": map[string]any{
				"close": 70000.0,
				"high":  70100.0,
				"low":   69500.0,
			},
			"prevBar1": map[string]any{
				"high": 69800.0,
				"low":  68800.0,
			},
			"prevBar2": map[string]any{
				"high": 69700.0,
				"low":  68700.0,
			},
		},
	}
	session, err = platform.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	updated, err := platform.refreshLiveSessionPositionContext(session, time.Now().UTC(), "test-refresh")
	if err != nil {
		t.Fatalf("refresh live session position context failed: %v", err)
	}
	liveState := mapValue(updated.State["livePositionState"])
	if liveState == nil {
		t.Fatal("expected livePositionState to be rebuilt")
	}
	if !boolValue(liveState["protected"]) {
		t.Fatal("expected live position to be marked protected after crossing profit threshold")
	}
	if parseFloatValue(liveState["stopLoss"]) <= 0 {
		t.Fatal("expected stopLoss to be rebuilt")
	}
	if got := stringValue(updated.State["positionRecoveryStatus"]); got != "protected-open-position" {
		t.Fatalf("expected protected-open-position, got %s", got)
	}
}
