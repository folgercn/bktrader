package service

import (
	"bytes"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestPretouchTimingEngineAdvancePlanProducesLiveIntentMetadata(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)

	first, err := engine.EvaluateSignal(testPretouchSignalContext(start, 101))
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	decision, err := engine.EvaluateSignal(testPretouchSignalContext(start.Add(60*time.Second), 105.1))
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if mapValue(decision.Metadata["signalBarDecision"]) == nil {
		t.Fatalf("expected signalBarDecision metadata: %#v", decision.Metadata)
	}
	if got := stringValue(decision.Metadata["nextPlannedSide"]); got != "BUY" {
		t.Fatalf("expected BUY next side, got %s", got)
	}
	if got := stringValue(decision.Metadata[liveSignalBarTradeLimitKeyField]); got != "ETHUSDT|1h|2026-05-15T12:00:00Z" {
		t.Fatalf("expected signal bar trade limit key, got %s", got)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("expected suggested quantity 0.12, got %v", got)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatalf("expected live signal intent from decision")
	}
	if math.Abs(intent.Quantity-0.12) > 1e-9 {
		t.Fatalf("expected intent quantity 0.12, got %v", intent.Quantity)
	}
}

func TestPretouchTimingEngineAppliesRFProbabilityAndCostPenaltyToIntentQuantity(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start, 101)
	ctx.ExecutionContext.Parameters["pretouchCostQ50Threshold"] = 0.005

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchSignalContext(start.Add(60*time.Second), 105.1)
	ctx.ExecutionContext.Parameters["pretouchCostQ50Threshold"] = 0.005
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := parseFloatValue(decision.Metadata["rfProbability"]); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("expected RF probability 0.75, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["sizingMultiplier"]); math.Abs(got-1.5) > 1e-9 {
		t.Fatalf("expected sizing multiplier 1.5, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["costPenalty"]); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("expected cost penalty 0.5, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["finalPositionSize"]); math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("expected final position size 0.6, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["productionSuggestedQuantity"]); math.Abs(got-0.06) > 1e-9 {
		t.Fatalf("expected production suggested quantity 0.06, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.06) > 1e-9 {
		t.Fatalf("expected suggested quantity 0.06, got %v", got)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatalf("expected live signal intent from decision")
	}
	if math.Abs(intent.Quantity-0.06) > 1e-9 {
		t.Fatalf("expected intent quantity 0.06, got %v", intent.Quantity)
	}
}

func TestPretouchTimingEngineAddsRiskOnShadowMetadataWithoutChangingIntentQuantity(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start, 101)
	ctx.ExecutionContext.Parameters["pretouchShadowMode"] = pretouchShadowModeTestnetCollect
	ctx.ExecutionContext.Parameters["pretouchShadowLeadScale"] = 1.5
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayScale"] = 2.0

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchSignalContext(start.Add(60*time.Second), 105.1)
	ctx.ExecutionContext.Parameters["pretouchShadowMode"] = pretouchShadowModeTestnetCollect
	ctx.ExecutionContext.Parameters["pretouchShadowLeadScale"] = 1.5
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayScale"] = 2.0
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("expected submitted suggested quantity to remain 0.12, got %v", got)
	}
	shadow := mapValue(decision.Metadata["pretouchShadowSizing"])
	if shadow == nil {
		t.Fatalf("expected pretouchShadowSizing metadata: %#v", decision.Metadata)
	}
	if got := stringValue(shadow["mode"]); got != pretouchShadowModeTestnetCollect {
		t.Fatalf("expected shadow mode %s, got %s", pretouchShadowModeTestnetCollect, got)
	}
	if got := parseFloatValue(shadow["leadScale"]); got != 1.5 {
		t.Fatalf("expected shadow lead scale 1.5, got %v", got)
	}
	if got := parseFloatValue(shadow["overlayScale"]); got != 2.0 {
		t.Fatalf("expected shadow overlay scale 2.0, got %v", got)
	}
	if got := parseFloatValue(shadow["shadowLeadQuantity"]); math.Abs(got-0.18) > 1e-9 {
		t.Fatalf("expected shadow lead quantity 0.18, got %v", got)
	}
	if !boolValue(shadow["submittedQuantityUnchanged"]) || boolValue(shadow["submittedRiskOnQuantityEnabled"]) {
		t.Fatalf("expected shadow metadata to keep submitted quantity unchanged: %#v", shadow)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatalf("expected live signal intent from decision")
	}
	if math.Abs(intent.Quantity-0.12) > 1e-9 {
		t.Fatalf("expected live intent quantity to remain submitted quantity 0.12, got %v", intent.Quantity)
	}
}

func TestPretouchTimingEngineSubmitsRiskOnLeadQuantityForSandboxShadow(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start, 101)
	enablePretouchRiskOnShadow(&ctx, true)

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchSignalContext(start.Add(60*time.Second), 105.1)
	enablePretouchRiskOnShadow(&ctx, true)
	setPretouchOrderBook(&ctx, 105.09, 105.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := parseFloatValue(decision.Metadata["productionSuggestedQuantity"]); math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("expected production suggested quantity 0.12, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.35) > 1e-9 {
		t.Fatalf("expected submitted suggested quantity to use lead quantity band size 0.35, got %v", got)
	}
	shadow := mapValue(decision.Metadata["pretouchShadowSizing"])
	if shadow == nil {
		t.Fatalf("expected pretouchShadowSizing metadata: %#v", decision.Metadata)
	}
	if !boolValue(shadow["submittedRiskOnQuantityRequested"]) || !boolValue(shadow["submittedRiskOnQuantityEnabled"]) {
		t.Fatalf("expected risk-on submitted quantity enabled: %#v", shadow)
	}
	if boolValue(shadow["submittedQuantityUnchanged"]) {
		t.Fatalf("expected submitted quantity to change under sandbox risk-on shadow: %#v", shadow)
	}
	if got := parseFloatValue(shadow["submittedQuantityBeforeShadow"]); math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("expected before-shadow quantity 0.12, got %v in %#v", got, shadow)
	}
	if got := stringValue(shadow["submittedSizingMode"]); got != "testnet_shadow_lead_rf_cost_quantity_band" {
		t.Fatalf("expected lead quantity band sizing mode, got %s in %#v", got, shadow)
	}
	if got := parseFloatValue(shadow["leadQuantityBandScore"]); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("expected lead quantity band score 0.75, got %v in %#v", got, shadow)
	}
	if got := parseFloatValue(shadow["submittedQuantityAfterShadow"]); math.Abs(got-0.35) > 1e-9 {
		t.Fatalf("expected after-shadow quantity 0.35, got %v in %#v", got, shadow)
	}
	if got := stringValue(shadow["submittedRiskOnQuantityBlockReason"]); got != "" {
		t.Fatalf("expected no risk-on block reason, got %s in %#v", got, shadow)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatalf("expected live signal intent from decision")
	}
	if math.Abs(intent.Quantity-0.35) > 1e-9 {
		t.Fatalf("expected live intent quantity to submit lead quantity band size 0.35, got %v", intent.Quantity)
	}
}

func TestPretouchTimingEngineMapsMaxRiskOnLeadQuantityToPointFour(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 1.0)
	ctx := testPretouchSignalContext(start, 101)
	enablePretouchRiskOnShadow(&ctx, true)

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchSignalContext(start.Add(60*time.Second), 105.1)
	enablePretouchRiskOnShadow(&ctx, true)
	setPretouchOrderBook(&ctx, 105.09, 105.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := parseFloatValue(decision.Metadata["productionSuggestedQuantity"]); math.Abs(got-0.16) > 1e-9 {
		t.Fatalf("expected max RF production quantity 0.16, got %v", got)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("expected max RF lead quantity band size 0.40, got %v", got)
	}
	shadow := mapValue(decision.Metadata["pretouchShadowSizing"])
	if shadow == nil {
		t.Fatalf("expected pretouchShadowSizing metadata: %#v", decision.Metadata)
	}
	if boolValue(shadow["shadowLeadQuantityCapped"]) {
		t.Fatalf("expected max lead quantity band not to be capped below 0.40: %#v", shadow)
	}
	if got := parseFloatValue(shadow["maxShadowSubmittedQuantity"]); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("expected max submitted quantity 0.40, got %v in %#v", got, shadow)
	}
	if got := parseFloatValue(shadow["submittedQuantityAfterShadow"]); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("expected submitted quantity 0.40, got %v in %#v", got, shadow)
	}
}

func TestPretouchTimingEngineBlocksRiskOnLeadQuantityOutsideSandbox(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start, 101)
	enablePretouchRiskOnShadow(&ctx, false)

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchSignalContext(start.Add(60*time.Second), 105.1)
	enablePretouchRiskOnShadow(&ctx, false)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("expected mainnet/non-sandbox to keep production quantity 0.12, got %v", got)
	}
	shadow := mapValue(decision.Metadata["pretouchShadowSizing"])
	if shadow == nil {
		t.Fatalf("expected pretouchShadowSizing metadata: %#v", decision.Metadata)
	}
	if boolValue(shadow["submittedRiskOnQuantityEnabled"]) || !boolValue(shadow["submittedQuantityUnchanged"]) {
		t.Fatalf("expected risk-on quantity to stay blocked outside sandbox: %#v", shadow)
	}
	if got := stringValue(shadow["submittedRiskOnQuantityBlockReason"]); got != "sandbox_required" {
		t.Fatalf("expected sandbox_required block reason, got %s in %#v", got, shadow)
	}
}

func TestPretouchTimingEngineHonorsRiskOnLeadQuantityOptOut(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start, 101)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowSubmitRiskOnQuantityParam] = false

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchSignalContext(start.Add(60*time.Second), 105.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowSubmitRiskOnQuantityParam] = false
	setPretouchOrderBook(&ctx, 105.09, 105.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.12) > 1e-9 {
		t.Fatalf("expected explicit opt-out to keep production quantity 0.12, got %v", got)
	}
	shadow := mapValue(decision.Metadata["pretouchShadowSizing"])
	if shadow == nil {
		t.Fatalf("expected pretouchShadowSizing metadata: %#v", decision.Metadata)
	}
	if boolValue(shadow["submittedRiskOnQuantityRequested"]) || boolValue(shadow["submittedRiskOnQuantityEnabled"]) {
		t.Fatalf("expected explicit risk-on opt-out to disable submitted quantity lift: %#v", shadow)
	}
	if got := stringValue(shadow["submittedRiskOnQuantityBlockReason"]); got != "risk_on_quantity_not_requested" {
		t.Fatalf("expected opt-out block reason, got %s in %#v", got, shadow)
	}
}

func TestPretouchShadowSizingBlocksWhenScaledTopDepthIsTooThin(t *testing.T) {
	shadow := pretouchShadowSizingFromParameters(
		map[string]any{
			"pretouchShadowMode":                    pretouchShadowModeTestnetCollect,
			"pretouchShadowLeadScale":               1.5,
			pretouchShadowSubmitRiskOnQuantityParam: true,
			"executionEntryMinTopBookCoverage":      0.5,
			"executionEntryMaxSpreadBps":            8.0,
		},
		"BUY",
		0.12,
		orderBookDecisionStats{
			bestAskQty: 0.01,
			spreadBps:  1.0,
		},
		pretouchShadowSubmitContext{
			liveExecution: true,
			sandbox:       true,
			executionMode: "rest",
		},
	)

	if shadow == nil {
		t.Fatal("expected shadow sizing metadata")
	}
	if boolValue(shadow["shadowPreSubmitPass"]) {
		t.Fatalf("expected thin top-depth to block shadow sizing: %#v", shadow)
	}
	if got := stringValue(shadow["shadowBlockReason"]); got != "shadow_top_depth_coverage_below_min" {
		t.Fatalf("expected top-depth block reason, got %s in %#v", got, shadow)
	}
	if !boolValue(shadow["submittedQuantityUnchanged"]) || boolValue(shadow["submittedRiskOnQuantityEnabled"]) {
		t.Fatalf("expected failed shadow sizing to remain telemetry-only: %#v", shadow)
	}
	if got := stringValue(shadow["submittedRiskOnQuantityBlockReason"]); got != "shadow_top_depth_coverage_below_min" {
		t.Fatalf("expected risk-on block to reuse shadow guard reason, got %s in %#v", got, shadow)
	}
}

func TestPretouchTimingEngineSubmitsT3OverlayForSandboxShadow(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, true)

	first, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first.Action != "wait" {
		t.Fatalf("expected first tick to wait, got %#v", first)
	}

	ctx = testPretouchT3OverlaySignalContext(start.Add(60*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	setPretouchOrderBook(&ctx, 106.09, 106.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected T3 overlay advance-plan, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := stringValue(decision.Metadata["pretouchEventShape"]); got != "t3_swing" {
		t.Fatalf("expected t3_swing event shape, got %s", got)
	}
	if got := stringValue(decision.Metadata["nextPlannedReason"]); got != "Pretouch-T3-Overlay" {
		t.Fatalf("expected T3 overlay reason, got %s", got)
	}
	if got := stringValue(decision.Metadata["signalKind"]); got != "entry-t3-overlay" {
		t.Fatalf("expected entry-t3-overlay signal kind, got %s", got)
	}
	baseKey := "ETHUSDT|1h|2026-05-15T12:00:00Z"
	overlayKey := baseKey + "|entry-t3-overlay"
	if got := stringValue(decision.Metadata["signalBarStateKey"]); got != baseKey {
		t.Fatalf("expected base signal bar state key %s, got %s", baseKey, got)
	}
	if got := stringValue(decision.Metadata[liveSignalBarTradeLimitKeyField]); got != overlayKey {
		t.Fatalf("expected overlay trade-limit key %s, got %s", overlayKey, got)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.08) > 1e-9 {
		t.Fatalf("expected T3 overlay 2.0x quantity 0.08, got %v", got)
	}
	overlay := mapValue(decision.Metadata["pretouchShadowOverlaySizing"])
	if overlay == nil {
		t.Fatalf("expected pretouchShadowOverlaySizing metadata: %#v", decision.Metadata)
	}
	if !boolValue(overlay["submittedOverlayOrderRequested"]) || !boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected overlay order enabled in sandbox shadow: %#v", overlay)
	}
	if got := parseFloatValue(overlay["overlayBaseShare"]); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("expected overlay base share 0.40, got %v in %#v", got, overlay)
	}
	if got := parseFloatValue(overlay["overlayScale"]); math.Abs(got-2.0) > 1e-9 {
		t.Fatalf("expected overlay scale 2.0, got %v in %#v", got, overlay)
	}
	if got := parseFloatValue(overlay["shadowOverlayQuantity"]); math.Abs(got-0.08) > 1e-9 {
		t.Fatalf("expected shadow overlay quantity 0.08, got %v in %#v", got, overlay)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatalf("expected T3 overlay live signal intent")
	}
	if intent.SignalKind != "entry-t3-overlay" || intent.Reason != "Pretouch-T3-Overlay" {
		t.Fatalf("unexpected T3 overlay intent: %#v", intent)
	}
	if got := stringValue(intent.Metadata[liveSignalBarTradeLimitKeyField]); got != overlayKey {
		t.Fatalf("expected overlay intent trade-limit key %s, got %s", overlayKey, got)
	}
	if math.Abs(intent.Quantity-0.08) > 1e-9 {
		t.Fatalf("expected T3 overlay intent quantity 0.08, got %v", intent.Quantity)
	}
}

func TestPretouchT3DeterministicStopGateSelectsHardStopProfile(t *testing.T) {
	event := domain.PretouchEvent{
		EventID:           "ETHUSDT_t3_20260515_120500_long",
		Symbol:            "ETHUSDT",
		Side:              "long",
		Speed300sATR:      0.70,
		Eff300s:           0.90,
		PreTouchSeconds:   300,
		TouchExtensionATR: 0.20,
		RoundtripCostATR:  0.01,
	}
	gate := pretouchT3DeterministicStopGate(map[string]any{
		pretouchShadowT3StopGateEnabledParam: true,
	}, event)
	if !boolValue(gate["pass"]) {
		t.Fatalf("expected deterministic stop gate to pass, got %#v", gate)
	}
	profile := mapValue(gate["selectedExitProfile"])
	if got := stringValue(profile["id"]); got != pretouchT3ExitProfileDeterministicHard3Delay79ID {
		t.Fatalf("expected selected hard3 delay profile, got %s in %#v", got, profile)
	}
	if got := parseFloatValue(profile["hardStopATR"]); got != defaultPretouchShadowT3StopGateHardStopATR {
		t.Fatalf("expected hardStopATR=%v, got %v", defaultPretouchShadowT3StopGateHardStopATR, got)
	}
	if got := parseFloatValue(profile["minHoldSecondsBeforeTrailingSL"]); got != defaultPretouchShadowT3StopGateTrailingDelaySeconds {
		t.Fatalf("expected trailing delay %v, got %v", defaultPretouchShadowT3StopGateTrailingDelaySeconds, got)
	}

	blocked := pretouchT3DeterministicStopGate(map[string]any{
		pretouchShadowT3StopGateEnabledParam: true,
	}, domain.PretouchEvent{
		EventID:           event.EventID,
		Symbol:            event.Symbol,
		Side:              event.Side,
		Speed300sATR:      0.70,
		Eff300s:           0.90,
		PreTouchSeconds:   300,
		TouchExtensionATR: 0.60,
	})
	if boolValue(blocked["pass"]) {
		t.Fatalf("expected oversized touch extension to fail, got %#v", blocked)
	}
	if got := stringValue(mapValue(blocked["selectedExitProfile"])["id"]); got != pretouchT3ExitProfileBaselineID {
		t.Fatalf("expected baseline fallback profile, got %s in %#v", got, blocked)
	}
}

func TestPretouchTimingEngineAttachesT3StopGateProfileToOverlayIntent(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowT3StopGateEnabledParam] = true

	_, _ = engine.EvaluateSignal(ctx)

	ctx = testPretouchT3OverlaySignalContext(start.Add(299*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowT3StopGateEnabledParam] = true
	setPretouchOrderBook(&ctx, 106.09, 106.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected T3 overlay advance-plan, got %#v", decision)
	}
	gate := mapValue(decision.Metadata["pretouchT3StopGate"])
	if !boolValue(gate["pass"]) {
		t.Fatalf("expected stop gate pass metadata, got %#v", gate)
	}
	profile := mapValue(decision.Metadata["pretouchT3ExitProfile"])
	if got := stringValue(profile["id"]); got != pretouchT3ExitProfileDeterministicHard3Delay79ID {
		t.Fatalf("expected selected exit profile in decision, got %s in %#v", got, profile)
	}
	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatal("expected live signal intent")
	}
	intentProfile := mapValue(intent.Metadata["pretouchT3ExitProfile"])
	if got := stringValue(intentProfile["id"]); got != pretouchT3ExitProfileDeterministicHard3Delay79ID {
		t.Fatalf("expected selected exit profile in intent metadata, got %s in %#v", got, intent.Metadata)
	}
	intentGate := mapValue(intent.Metadata["pretouchT3StopGate"])
	if !boolValue(intentGate["pass"]) {
		t.Fatalf("expected stop gate pass in intent metadata, got %#v", intentGate)
	}
}

func TestPretouchTimingEngineLogsT3OverlayDetectorMissOncePerSignalBar(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(previousLogger)

	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayMaxPreTouchSec"] = 30.0
	if _, err := engine.EvaluateSignal(ctx); err != nil {
		t.Fatalf("initial evaluate failed: %v", err)
	}

	logs.Reset()
	ctx = testPretouchT3OverlaySignalContext(start.Add(60*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayMaxPreTouchSec"] = 30.0
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "wait" || decision.Reason != "no_level_touch" {
		t.Fatalf("expected lead wait/no_level_touch to remain unchanged, got %#v", decision)
	}
	output := logs.String()
	if !strings.Contains(output, "pretouch T3 overlay rejected") {
		t.Fatalf("expected T3 overlay rejection log, got %q", output)
	}
	if !strings.Contains(output, "t3_miss_category=t3_pre_touch_seconds") {
		t.Fatalf("expected normalized T3 miss category in log, got %q", output)
	}
	if !strings.Contains(output, "preTouchSeconds:60") || !strings.Contains(output, "maxPreTouchSeconds:30") {
		t.Fatalf("expected timing diagnostics in log, got %q", output)
	}

	logs.Reset()
	ctx = testPretouchT3OverlaySignalContext(start.Add(61*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayMaxPreTouchSec"] = 30.0
	if _, err := engine.EvaluateSignal(ctx); err != nil {
		t.Fatalf("repeat evaluate failed: %v", err)
	}
	if strings.Contains(logs.String(), "pretouch T3 overlay rejected") {
		t.Fatalf("expected repeated same-bar rejection to be throttled, got %q", logs.String())
	}
}

func TestPretouchTimingEngineAppliesT3OverlayRFQualitySizingForSandboxShadow(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	engine.setT3OverlayModel(testPretouchT3OverlayQualityModel())
	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualitySizingParam] = true

	_, _ = engine.EvaluateSignal(ctx)

	ctx = testPretouchT3OverlaySignalContext(start.Add(60*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualitySizingParam] = true
	setPretouchOrderBook(&ctx, 106.09, 106.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected T3 overlay advance-plan, got %#v", decision)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.38) > 1e-9 {
		t.Fatalf("expected T3 overlay RF quality quantity 0.38, got %v", got)
	}
	overlay := mapValue(decision.Metadata["pretouchShadowOverlaySizing"])
	if overlay == nil {
		t.Fatalf("expected pretouchShadowOverlaySizing metadata: %#v", decision.Metadata)
	}
	if got := stringValue(overlay["overlaySizingMode"]); got != "testnet_shadow_t3_overlay_rf_cost_quality_quantity" {
		t.Fatalf("expected RF quality sizing mode, got %s in %#v", got, overlay)
	}
	if got := parseFloatValue(overlay["shadowOverlayQuantityBeforeQuality"]); math.Abs(got-0.08) > 1e-9 {
		t.Fatalf("expected pre-quality overlay quantity 0.08, got %v in %#v", got, overlay)
	}
	if got := parseFloatValue(overlay["overlayQualityMultiplier"]); math.Abs(got-4.75) > 1e-9 {
		t.Fatalf("expected quality multiplier 4.75, got %v in %#v", got, overlay)
	}
	quality := mapValue(overlay["pretouchShadowOverlayQualitySizing"])
	if !boolValue(quality["enabled"]) || stringValue(quality["status"]) != "applied" {
		t.Fatalf("expected applied quality sizing metadata, got %#v", quality)
	}
	if got := parseFloatValue(quality["qualityQuantity"]); math.Abs(got-0.38) > 1e-9 {
		t.Fatalf("expected T3 quality quantity 0.38, got %v in %#v", got, quality)
	}
	if got := parseFloatValue(quality["minQuantity"]); math.Abs(got-0.20) > 1e-9 {
		t.Fatalf("expected T3 quality min quantity 0.20, got %v in %#v", got, quality)
	}
	if got := parseFloatValue(quality["maxQuantity"]); math.Abs(got-0.40) > 1e-9 {
		t.Fatalf("expected T3 quality max quantity 0.40, got %v in %#v", got, quality)
	}
	if got := parseFloatValue(quality["probability"]); math.Abs(got-0.90) > 1e-9 {
		t.Fatalf("expected T3 model probability 0.90, got %v in %#v", got, quality)
	}
	features := mapValue(quality["features"])
	if got := parseFloatValue(features["leadRFProbability"]); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("expected lead RF feature 0.75, got %v in %#v", got, features)
	}
	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil || math.Abs(intent.Quantity-0.38) > 1e-9 {
		t.Fatalf("expected RF quality intent quantity 0.38, got %#v", intent)
	}
}

func TestPretouchTimingEngineBlocksT3OverlayWhenQualityModelMissing(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	engine.setT3OverlayModel(nil)
	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualitySizingParam] = true

	_, _ = engine.EvaluateSignal(ctx)

	ctx = testPretouchT3OverlaySignalContext(start.Add(60*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualitySizingParam] = true
	setPretouchOrderBook(&ctx, 106.09, 106.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "wait" || decision.Reason != "overlay_quality_model_missing" {
		t.Fatalf("expected model-missing T3 overlay to wait, got %#v", decision)
	}
	overlay := mapValue(decision.Metadata["pretouchShadowOverlaySizing"])
	if boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected model-missing quality sizing to block overlay submission: %#v", overlay)
	}
	if got := stringValue(overlay["overlayQualityBlockReason"]); got != "overlay_quality_model_missing" {
		t.Fatalf("expected overlay_quality_model_missing block reason, got %s in %#v", got, overlay)
	}
	quality := mapValue(overlay["pretouchShadowOverlayQualitySizing"])
	if boolValue(quality["enabled"]) || stringValue(quality["status"]) != "model_missing" {
		t.Fatalf("expected model_missing quality metadata, got %#v", quality)
	}
	if boolValue(quality["fallbackSubmitAllowed"]) {
		t.Fatalf("expected fallback submit disabled by default, got %#v", quality)
	}
	if intent := deriveLiveSignalIntent(decision, "ETHUSDT"); intent != nil {
		t.Fatalf("expected no intent for model-missing overlay, got %#v", intent)
	}
}

func TestPretouchTimingEngineAllowsExplicitT3OverlayQualityFallbackSubmit(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	engine.setT3OverlayModel(nil)
	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualitySizingParam] = true
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualityFallbackParam] = true

	_, _ = engine.EvaluateSignal(ctx)

	ctx = testPretouchT3OverlaySignalContext(start.Add(60*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, true)
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualitySizingParam] = true
	ctx.ExecutionContext.Parameters[pretouchShadowOverlayQualityFallbackParam] = true
	setPretouchOrderBook(&ctx, 106.09, 106.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected explicit fixed fallback T3 overlay advance-plan, got %#v", decision)
	}
	if got := parseFloatValue(decision.Metadata["suggestedQuantity"]); math.Abs(got-0.08) > 1e-9 {
		t.Fatalf("expected fixed T3 overlay fallback quantity 0.08, got %v", got)
	}
	overlay := mapValue(decision.Metadata["pretouchShadowOverlaySizing"])
	if !boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected explicit fallback submit to allow overlay order: %#v", overlay)
	}
	if got := stringValue(overlay["overlaySizingMode"]); got != "testnet_shadow_t3_overlay_scale_intent_quantity" {
		t.Fatalf("expected fixed overlay sizing mode on model missing, got %s", got)
	}
	quality := mapValue(overlay["pretouchShadowOverlayQualitySizing"])
	if !boolValue(quality["fallbackSubmitAllowed"]) || stringValue(quality["status"]) != "model_missing" {
		t.Fatalf("expected explicit model_missing fallback metadata, got %#v", quality)
	}
}

func TestPretouchShadowOverlaySizingBlocksFeatureBuildFailureFallbackByDefault(t *testing.T) {
	overlay := pretouchShadowOverlaySizingFromParameters(
		map[string]any{
			"pretouchShadowMode":                  pretouchShadowModeTestnetCollect,
			"pretouchShadowOverlayScale":          2.0,
			"pretouchShadowOverlayBaseShare":      0.40,
			pretouchShadowSubmitOverlayOrderParam: true,
			"executionEntryMinTopBookCoverage":    0.5,
			"executionEntryMaxSpreadBps":          8.0,
		},
		"BUY",
		0.1,
		orderBookDecisionStats{
			bestAskQty: 10.0,
			spreadBps:  1.0,
		},
		pretouchShadowSubmitContext{
			liveExecution: true,
			sandbox:       true,
			executionMode: "rest",
		},
		map[string]any{
			"requested":             true,
			"enabled":               false,
			"status":                "feature_build_failed",
			"fallbackSubmitAllowed": false,
		},
	)
	if overlay == nil {
		t.Fatal("expected overlay sizing metadata")
	}
	if !boolValue(overlay["shadowDepthPreSubmitPass"]) {
		t.Fatalf("expected depth guard to pass before quality fallback block: %#v", overlay)
	}
	if boolValue(overlay["shadowPreSubmitPass"]) || boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected feature-build failure to block overlay submission: %#v", overlay)
	}
	if got := stringValue(overlay["submittedOverlayOrderBlockReason"]); got != "overlay_quality_feature_build_failed" {
		t.Fatalf("expected feature-build fallback block reason, got %s in %#v", got, overlay)
	}
}

func TestPretouchTimingEngineBlocksT3OverlayOutsideSandbox(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchT3OverlaySignalContext(start, 100.0)
	enablePretouchRiskOnShadow(&ctx, false)

	_, _ = engine.EvaluateSignal(ctx)

	ctx = testPretouchT3OverlaySignalContext(start.Add(60*time.Second), 106.1)
	enablePretouchRiskOnShadow(&ctx, false)
	setPretouchOrderBook(&ctx, 106.09, 106.1)
	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "wait" {
		t.Fatalf("expected non-sandbox T3 overlay to wait, got %#v", decision)
	}
	if decision.Reason != "sandbox_required" {
		t.Fatalf("expected sandbox_required block reason, got %s", decision.Reason)
	}
	overlay := mapValue(decision.Metadata["pretouchShadowOverlaySizing"])
	if overlay == nil {
		t.Fatalf("expected overlay sizing metadata: %#v", decision.Metadata)
	}
	if boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected overlay order blocked outside sandbox: %#v", overlay)
	}
	if intent := deriveLiveSignalIntent(decision, "ETHUSDT"); intent != nil {
		t.Fatalf("expected no live intent for blocked overlay, got %#v", intent)
	}
}

func TestPretouchShadowOverlaySizingBlocksWhenScaledTopDepthIsTooThin(t *testing.T) {
	overlay := pretouchShadowOverlaySizingFromParameters(
		map[string]any{
			"pretouchShadowMode":                  pretouchShadowModeTestnetCollect,
			"pretouchShadowOverlayScale":          2.0,
			"pretouchShadowOverlayBaseShare":      0.40,
			pretouchShadowSubmitOverlayOrderParam: true,
			"executionEntryMinTopBookCoverage":    0.5,
			"executionEntryMaxSpreadBps":          8.0,
		},
		"BUY",
		0.1,
		orderBookDecisionStats{
			bestAskQty: 0.01,
			spreadBps:  1.0,
		},
		pretouchShadowSubmitContext{
			liveExecution: true,
			sandbox:       true,
			executionMode: "rest",
		},
		nil,
	)
	if overlay == nil {
		t.Fatal("expected overlay sizing metadata")
	}
	if boolValue(overlay["shadowPreSubmitPass"]) {
		t.Fatalf("expected thin top-depth to block overlay sizing: %#v", overlay)
	}
	if boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected failed overlay sizing to block order submission: %#v", overlay)
	}
	if got := stringValue(overlay["submittedOverlayOrderBlockReason"]); got != "shadow_top_depth_coverage_below_min" {
		t.Fatalf("expected overlay block to reuse depth reason, got %s in %#v", got, overlay)
	}
}

func TestPretouchShadowSizingCapsOversizedLeadQuantity(t *testing.T) {
	shadow := pretouchShadowSizingFromParameters(
		map[string]any{
			"pretouchShadowMode":                    pretouchShadowModeTestnetCollect,
			"pretouchShadowLeadScale":               99.0,
			pretouchShadowMaxSubmittedQuantityParam: 99.0,
			pretouchShadowSubmitRiskOnQuantityParam: true,
			"executionEntryMinTopBookCoverage":      0.5,
			"executionEntryMaxSpreadBps":            8.0,
		},
		"BUY",
		1.0,
		orderBookDecisionStats{
			bestAskQty: 10.0,
			spreadBps:  1.0,
		},
		pretouchShadowSubmitContext{
			liveExecution: true,
			sandbox:       true,
			executionMode: "rest",
		},
	)
	if shadow == nil {
		t.Fatal("expected shadow sizing metadata")
	}
	if got := parseFloatValue(shadow["leadScale"]); got != maxPretouchShadowLeadScale {
		t.Fatalf("expected capped lead scale %v, got %v in %#v", maxPretouchShadowLeadScale, got, shadow)
	}
	if got := parseFloatValue(shadow["maxShadowSubmittedQuantity"]); got != defaultPretouchShadowMaxSubmittedQuantity {
		t.Fatalf("expected max submitted cap %v, got %v in %#v", defaultPretouchShadowMaxSubmittedQuantity, got, shadow)
	}
	if got := parseFloatValue(shadow["shadowLeadQuantity"]); got != defaultPretouchShadowMaxSubmittedQuantity {
		t.Fatalf("expected lead quantity capped to %v, got %v in %#v", defaultPretouchShadowMaxSubmittedQuantity, got, shadow)
	}
	if !boolValue(shadow["shadowLeadQuantityCapped"]) || !boolValue(shadow["submittedRiskOnQuantityEnabled"]) {
		t.Fatalf("expected capped risk-on lead to remain enabled under depth guard: %#v", shadow)
	}
	if got := parseFloatValue(shadow["submittedQuantityAfterShadow"]); got != defaultPretouchShadowMaxSubmittedQuantity {
		t.Fatalf("expected submitted quantity capped to %v, got %v in %#v", defaultPretouchShadowMaxSubmittedQuantity, got, shadow)
	}
}

func TestPretouchBaseOrderQuantityFromParametersCapsShadowMode(t *testing.T) {
	got := pretouchBaseOrderQuantityFromParameters(map[string]any{
		"pretouchShadowMode":                    pretouchShadowModeTestnetCollect,
		"pretouchBaseOrderQuantity":             10.0,
		pretouchShadowMaxSubmittedQuantityParam: 99.0,
	})
	if got != defaultPretouchShadowMaxSubmittedQuantity {
		t.Fatalf("expected shadow base order quantity capped to %v, got %v", defaultPretouchShadowMaxSubmittedQuantity, got)
	}
}

func TestPretouchShadowOverlaySizingCapsOversizedOverlayQuantity(t *testing.T) {
	overlay := pretouchShadowOverlaySizingFromParameters(
		map[string]any{
			"pretouchShadowMode":                    pretouchShadowModeTestnetCollect,
			"pretouchShadowOverlayScale":            99.0,
			"pretouchShadowOverlayBaseShare":        9.0,
			pretouchShadowMaxSubmittedQuantityParam: 99.0,
			pretouchShadowSubmitOverlayOrderParam:   true,
			"executionEntryMinTopBookCoverage":      0.5,
			"executionEntryMaxSpreadBps":            8.0,
		},
		"BUY",
		10.0,
		orderBookDecisionStats{
			bestAskQty: 10.0,
			spreadBps:  1.0,
		},
		pretouchShadowSubmitContext{
			liveExecution: true,
			sandbox:       true,
			executionMode: "rest",
		},
		nil,
	)
	if overlay == nil {
		t.Fatal("expected overlay sizing metadata")
	}
	if got := parseFloatValue(overlay["overlayScale"]); got != maxPretouchShadowOverlayScale {
		t.Fatalf("expected capped overlay scale %v, got %v in %#v", maxPretouchShadowOverlayScale, got, overlay)
	}
	if got := parseFloatValue(overlay["overlayBaseShare"]); got != maxPretouchShadowOverlayBaseShare {
		t.Fatalf("expected capped overlay base share %v, got %v in %#v", maxPretouchShadowOverlayBaseShare, got, overlay)
	}
	if got := parseFloatValue(overlay["shadowOverlayQuantity"]); got != defaultPretouchShadowMaxSubmittedQuantity {
		t.Fatalf("expected overlay quantity capped to %v, got %v in %#v", defaultPretouchShadowMaxSubmittedQuantity, got, overlay)
	}
	if !boolValue(overlay["shadowOverlayQuantityCapped"]) || !boolValue(overlay["submittedOverlayOrderEnabled"]) {
		t.Fatalf("expected capped overlay to remain enabled under depth guard: %#v", overlay)
	}
	if got := parseFloatValue(overlay["submittedOverlayQuantity"]); got != defaultPretouchShadowMaxSubmittedQuantity {
		t.Fatalf("expected submitted overlay quantity capped to %v, got %v in %#v", defaultPretouchShadowMaxSubmittedQuantity, got, overlay)
	}
}

func TestPretouchSignalBarTradeLimitKeyForKindSeparatesT3OverlayFromLead(t *testing.T) {
	current := &HourlyBar{OpenTime: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)}
	leadKey := pretouchSignalBarTradeLimitKeyForKind("ETHUSDT", "1h", current, "entry")
	overlayKey := pretouchSignalBarTradeLimitKeyForKind("ETHUSDT", "1h", current, "entry-t3-overlay")
	if leadKey != "ETHUSDT|1h|2026-05-15T12:00:00Z" {
		t.Fatalf("unexpected lead key %s", leadKey)
	}
	if overlayKey != "ETHUSDT|1h|2026-05-15T12:00:00Z|entry-t3-overlay" {
		t.Fatalf("unexpected overlay key %s", overlayKey)
	}
	if leadKey == overlayKey {
		t.Fatal("expected lead and T3 overlay to use distinct trade-limit keys")
	}
}

func TestPretouchTimingEngineProducesRiskExitForLongStopLossBreach(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start.Add(60*time.Second), 99.0)
	ctx.ExecutionContext.Parameters["stop_loss_atr"] = 0.45
	ctx.CurrentPosition = map[string]any{
		"id":         "position-long",
		"found":      true,
		"symbol":     "ETHUSDT",
		"side":       "LONG",
		"quantity":   0.068,
		"entryPrice": 100.0,
	}
	ctx.SignalBarStates = testPretouchExitSignalBarStates(start, 99.0)
	setPretouchOrderBook(&ctx, 99.0, 99.1)

	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan risk exit, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := stringValue(decision.Metadata["nextPlannedRole"]); got != "exit" {
		t.Fatalf("expected exit role, got %s", got)
	}
	if got := stringValue(decision.Metadata["nextPlannedReason"]); got != "SL" {
		t.Fatalf("expected SL reason, got %s", got)
	}
	if got := stringValue(decision.Metadata["nextPlannedSide"]); got != "SELL" {
		t.Fatalf("expected SELL exit side, got %s", got)
	}
	if got := stringValue(decision.Metadata["signalKind"]); got != "risk-exit" {
		t.Fatalf("expected risk-exit signal kind, got %s", got)
	}
	livePositionState := mapValue(decision.Metadata["livePositionState"])
	if !boolValue(livePositionState["ready"]) {
		t.Fatalf("expected live position exit state ready, got %#v", livePositionState)
	}
	if got := parseFloatValue(livePositionState["targetPrice"]); math.Abs(got-99.1) > 1e-9 {
		t.Fatalf("expected target stop 99.1, got %v", got)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatal("expected exit intent")
	}
	if intent.Side != "SELL" || intent.Role != "exit" || intent.SignalKind != "risk-exit" {
		t.Fatalf("unexpected exit intent: %#v", intent)
	}
	if math.Abs(intent.Quantity-0.068) > 1e-9 {
		t.Fatalf("expected intent quantity to match current position, got %v", intent.Quantity)
	}
}

func TestPretouchTimingEngineProducesRiskExitForShortTrailingStopBreach(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start.Add(60*time.Second), 96.8)
	ctx.ExecutionContext.Parameters["stop_loss_atr"] = 0.45
	ctx.ExecutionContext.Parameters["trailing_stop_atr"] = 0.3
	ctx.CurrentPosition = map[string]any{
		"id":         "position-short",
		"found":      true,
		"symbol":     "ETHUSDT",
		"side":       "SHORT",
		"quantity":   0.069,
		"entryPrice": 100.0,
	}
	ctx.SessionState = map[string]any{
		"watermarkPositionKey": buildLivePositionWatermarkKey(ctx.CurrentPosition),
		"hwm":                  100.0,
		"lwm":                  96.0,
	}
	ctx.SignalBarStates = testPretouchExitSignalBarStates(start, 96.8)
	setPretouchOrderBook(&ctx, 96.7, 96.8)

	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "advance-plan" {
		t.Fatalf("expected advance-plan risk exit, got action=%s reason=%s metadata=%#v", decision.Action, decision.Reason, decision.Metadata)
	}
	if got := stringValue(decision.Metadata["nextPlannedSide"]); got != "BUY" {
		t.Fatalf("expected BUY exit side, got %s", got)
	}
	livePositionState := mapValue(decision.Metadata["livePositionState"])
	if got := stringValue(livePositionState["targetPriceSource"]); got != "trailing-stop" {
		t.Fatalf("expected trailing-stop source, got %s in %#v", got, livePositionState)
	}
	if got := parseFloatValue(livePositionState["targetPrice"]); math.Abs(got-96.6) > 1e-9 {
		t.Fatalf("expected trailing stop 96.6, got %v", got)
	}

	intent := deriveLiveSignalIntent(decision, "ETHUSDT")
	if intent == nil {
		t.Fatal("expected exit intent")
	}
	if intent.Side != "BUY" || intent.Role != "exit" || intent.SignalKind != "risk-exit" {
		t.Fatalf("unexpected exit intent: %#v", intent)
	}
	if math.Abs(intent.Quantity-0.069) > 1e-9 {
		t.Fatalf("expected intent quantity to match current position, got %v", intent.Quantity)
	}
}

func TestPretouchTimingEngineHoldsOpenPositionWhenStopNotBreached(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	ctx := testPretouchSignalContext(start.Add(60*time.Second), 100.5)
	ctx.ExecutionContext.Parameters["stop_loss_atr"] = 0.45
	ctx.CurrentPosition = map[string]any{
		"id":         "position-long",
		"found":      true,
		"symbol":     "ETHUSDT",
		"side":       "LONG",
		"quantity":   0.068,
		"entryPrice": 100.0,
	}
	ctx.SignalBarStates = testPretouchExitSignalBarStates(start, 100.5)
	setPretouchOrderBook(&ctx, 100.5, 100.6)

	decision, err := engine.EvaluateSignal(ctx)
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "wait" {
		t.Fatalf("expected open position to wait while stop is intact, got %#v", decision)
	}
	if got := stringValue(decision.Metadata["nextPlannedRole"]); got != "exit" {
		t.Fatalf("expected exit monitoring role, got %s", got)
	}
	if got := stringValue(decision.Metadata["signalKind"]); got != "risk-exit-watch" {
		t.Fatalf("expected risk-exit-watch signal kind, got %s", got)
	}
	if intent := deriveLiveSignalIntent(decision, "ETHUSDT"); intent != nil {
		t.Fatalf("expected no live intent while stop is intact, got %#v", intent)
	}
}

func TestPretouchTimingEngineSkipsWhenModelMissing(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("fast", 0.75)
	engine.setLeadModel(nil)

	_, _ = engine.EvaluateSignal(testPretouchSignalContext(start, 101))
	decision, err := engine.EvaluateSignal(testPretouchSignalContext(start.Add(60*time.Second), 105.1))
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "wait" || decision.Reason != "no_model_loaded" {
		t.Fatalf("expected no_model_loaded wait, got %#v", decision)
	}
}

func TestPretouchTimingEngineSkipsUnknownTimingRegime(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	engine := testPretouchTimingEngine("", 0.75)

	_, _ = engine.EvaluateSignal(testPretouchSignalContext(start, 101))
	decision, err := engine.EvaluateSignal(testPretouchSignalContext(start.Add(60*time.Second), 105.1))
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if decision.Action != "wait" || decision.Reason != "timing_skip" {
		t.Fatalf("expected timing_skip wait, got %#v", decision)
	}
}

func TestResolveExecutionQuantityIntentQuantityMode(t *testing.T) {
	quantity, metadata := resolveExecutionQuantity(
		domain.LiveSession{
			State: map[string]any{
				"positionSizingMode":   "intent_quantity",
				"defaultOrderQuantity": 0.5,
			},
		},
		domain.Account{},
		nil,
		SignalIntent{Role: "entry", Quantity: 0.12},
		105.1,
	)
	if math.Abs(quantity-0.12) > 1e-9 {
		t.Fatalf("expected intent quantity to override fixed quantity, got %v", quantity)
	}
	if got := stringValue(metadata["sizingMethod"]); got != "intent_quantity" {
		t.Fatalf("expected intent_quantity sizing method, got %s", got)
	}
}

func TestResolveExecutionQuantityIntentQuantityModeFallsBackWithWarning(t *testing.T) {
	quantity, metadata := resolveExecutionQuantity(
		domain.LiveSession{
			State: map[string]any{
				"positionSizingMode":   "intent_quantity",
				"defaultOrderQuantity": 0.5,
			},
		},
		domain.Account{},
		nil,
		SignalIntent{Role: "entry", Symbol: "ETHUSDT", Side: "BUY"},
		105.1,
	)
	if math.Abs(quantity-0.5) > 1e-9 {
		t.Fatalf("expected fixed quantity fallback, got %v", quantity)
	}
	if got := stringValue(metadata["sizingFallbackReason"]); got != "intent_quantity_missing_intent_quantity" {
		t.Fatalf("expected missing intent quantity fallback, got %s", got)
	}
	if got := stringValue(metadata["sizingWarning"]); got != "intent_quantity_missing_intent_quantity" {
		t.Fatalf("expected sizing warning metadata, got %s", got)
	}
}

func TestPretouchBarsFromEvaluationContextSynthesizesCurrentBar(t *testing.T) {
	currentStart := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	ctx := testPretouchSignalContext(currentStart.Add(10*time.Minute), 105.1)
	bars := testPretouchSignalBars(currentStart)
	ctx.SourceStates["binance-kline|signal|ETHUSDT|1h"].(map[string]any)["bars"] = bars[:len(bars)-1]

	closed, current := pretouchBarsFromEvaluationContext(ctx, 105.1)
	if len(closed) != len(pretouchDetectorClosedBars(currentStart)) {
		t.Fatalf("expected closed bars from source state, got %d", len(closed))
	}
	if current == nil {
		t.Fatalf("expected synthetic current bar")
	}
	if !current.OpenTime.Equal(currentStart) {
		t.Fatalf("expected synthetic current open %s, got %s", currentStart, current.OpenTime)
	}
	if current.Open != 105.1 || current.High != 105.1 || current.Low != 105.1 || current.Close != 105.1 {
		t.Fatalf("expected synthetic OHLC from trigger price, got %#v", current)
	}
}

func testPretouchTimingEngine(timingRegime string, rfProba float64) *bkLiveEthPretouchTimingEngine {
	config := DefaultPretouchDetectorConfig()
	engine := &bkLiveEthPretouchTimingEngine{
		platform:   NewPlatform(memory.NewStore()),
		detector:   NewPretouchEventDetector("ETHUSDT", config),
		t3Detector: NewPretouchEventDetector("ETHUSDT", config),
		config:     config,
	}
	engine.setLeadModel(&PretouchModelBundle{
		TimingTree: &TreeNode{FeatureIndex: -1, LeafValue: timingRegime, LeafProba: 1},
		RFModel: &RandomForest{
			Trees:       []*TreeNode{{FeatureIndex: -1, LeafValue: "1", LeafProba: rfProba}},
			NEstimators: 1,
		},
		FeatureNames: pretouchTrainFeatures,
		Medians:      make([]float64, len(pretouchTrainFeatures)),
		Version:      "test",
		RFAccuracy:   0.7,
	})
	return engine
}

func testPretouchT3OverlayQualityModel() *PretouchModelBundle {
	return &PretouchModelBundle{
		TimingTree: &TreeNode{FeatureIndex: -1, LeafValue: "fast", LeafProba: 1},
		RFModel: &RandomForest{
			Trees: []*TreeNode{
				{
					FeatureIndex: 0,
					Threshold:    0.5,
					Left:         &TreeNode{FeatureIndex: -1, LeafValue: "0", LeafProba: 0.20},
					Right:        &TreeNode{FeatureIndex: -1, LeafValue: "1", LeafProba: 0.90},
				},
			},
			NEstimators: 1,
		},
		FeatureNames: []string{
			"rf_probability",
			"speed_300s_abs",
			"eff_300s",
			"touch_extension_abs",
			"pre_touch_seconds",
			"roundtrip_cost_atr",
			"side_is_short",
		},
		Medians:    []float64{0.5, 0.35, 1.0, 0.0, 300.0, 0.10, 0.0},
		Version:    "test-t3-overlay-rf",
		RFAccuracy: 0.7,
	}
}

func testPretouchSignalContext(eventTime time.Time, price float64) StrategySignalEvaluationContext {
	currentStart := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	return StrategySignalEvaluationContext{
		ExecutionContext: StrategyExecutionContext{
			Symbol:          "ETHUSDT",
			SignalTimeframe: "1h",
			Parameters: map[string]any{
				"pretouchBaseOrderQuantity": 0.1,
			},
		},
		TriggerSummary: map[string]any{
			"symbol": "ETHUSDT",
			"price":  price,
		},
		SourceStates: map[string]any{
			"binance-kline|signal|ETHUSDT|1h": map[string]any{
				"sourceKey":  "binance-kline",
				"role":       "signal",
				"streamType": "signal_bar",
				"symbol":     "ETHUSDT",
				"timeframe":  "1h",
				"bars":       testPretouchSignalBars(currentStart),
			},
			"binance-order-book|feature|ETHUSDT|": map[string]any{
				"sourceKey":  "binance-order-book",
				"role":       "feature",
				"streamType": "order_book",
				"symbol":     "ETHUSDT",
				"summary": map[string]any{
					"bestBid":    105.0,
					"bestAsk":    105.1,
					"bestBidQty": 20.0,
					"bestAskQty": 10.0,
				},
			},
		},
		EventTime: eventTime,
	}
}

func testPretouchT3OverlaySignalContext(eventTime time.Time, price float64) StrategySignalEvaluationContext {
	currentStart := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	ctx := testPretouchSignalContext(eventTime, price)
	ctx.SourceStates["binance-kline|signal|ETHUSDT|1h"].(map[string]any)["bars"] = testPretouchT3SignalBars(currentStart)
	return ctx
}

func testPretouchSignalBars(currentStart time.Time) []any {
	bars := make([]any, 0, 7)
	for _, bar := range pretouchDetectorClosedBars(currentStart) {
		bars = append(bars, map[string]any{
			"symbol":    "ETHUSDT",
			"timeframe": "1h",
			"barStart":  bar.OpenTime.Format(time.RFC3339),
			"open":      bar.Open,
			"high":      bar.High,
			"low":       bar.Low,
			"close":     bar.Close,
			"isClosed":  true,
		})
	}
	bars = append(bars, map[string]any{
		"symbol":    "ETHUSDT",
		"timeframe": "1h",
		"barStart":  currentStart.Format(time.RFC3339),
		"open":      100.0,
		"high":      100.0,
		"low":       100.0,
		"close":     100.0,
		"isClosed":  false,
	})
	return bars
}

func testPretouchT3SignalBars(currentStart time.Time) []any {
	closed := []HourlyBar{
		{OpenTime: currentStart.Add(-6 * time.Hour), Open: 100, High: 101, Low: 99, Close: 100},
		{OpenTime: currentStart.Add(-5 * time.Hour), Open: 100, High: 102, Low: 98, Close: 100},
		{OpenTime: currentStart.Add(-4 * time.Hour), Open: 100, High: 104, Low: 96, Close: 100},
		{OpenTime: currentStart.Add(-3 * time.Hour), Open: 100, High: 106, Low: 94, Close: 100},
		{OpenTime: currentStart.Add(-2 * time.Hour), Open: 100, High: 103, Low: 95, Close: 100},
		{OpenTime: currentStart.Add(-1 * time.Hour), Open: 100, High: 104, Low: 96, Close: 100},
	}
	bars := make([]any, 0, len(closed)+1)
	for _, bar := range closed {
		bars = append(bars, map[string]any{
			"symbol":    "ETHUSDT",
			"timeframe": "1h",
			"barStart":  bar.OpenTime.Format(time.RFC3339),
			"open":      bar.Open,
			"high":      bar.High,
			"low":       bar.Low,
			"close":     bar.Close,
			"isClosed":  true,
		})
	}
	bars = append(bars, map[string]any{
		"symbol":    "ETHUSDT",
		"timeframe": "1h",
		"barStart":  currentStart.Format(time.RFC3339),
		"open":      100.0,
		"high":      100.0,
		"low":       100.0,
		"close":     100.0,
		"isClosed":  false,
	})
	return bars
}

func testPretouchExitSignalBarStates(currentStart time.Time, closePrice float64) map[string]any {
	key := signalBindingMatchKey("binance-kline", "signal", "ETHUSDT", map[string]any{"timeframe": "1h"})
	return map[string]any{
		key: map[string]any{
			"symbol":         "ETHUSDT",
			"timeframe":      "1h",
			"barCount":       20,
			"closedBarCount": 19,
			"currentClosed":  false,
			"atr14":          2.0,
			"atrPercentile":  0.5,
			"current": map[string]any{
				"symbol":    "ETHUSDT",
				"timeframe": "1h",
				"barStart":  currentStart.Format(time.RFC3339),
				"open":      closePrice,
				"high":      closePrice + 0.5,
				"low":       closePrice - 0.5,
				"close":     closePrice,
				"isClosed":  false,
			},
			"prevBar1": map[string]any{
				"symbol":    "ETHUSDT",
				"timeframe": "1h",
				"barStart":  currentStart.Add(-time.Hour).Format(time.RFC3339),
				"open":      100.0,
				"high":      101.0,
				"low":       99.0,
				"close":     100.0,
				"isClosed":  true,
			},
			"prevBar2": map[string]any{
				"symbol":    "ETHUSDT",
				"timeframe": "1h",
				"barStart":  currentStart.Add(-2 * time.Hour).Format(time.RFC3339),
				"open":      100.0,
				"high":      101.0,
				"low":       99.0,
				"close":     100.0,
				"isClosed":  true,
			},
			"prevBar3": map[string]any{
				"symbol":    "ETHUSDT",
				"timeframe": "1h",
				"barStart":  currentStart.Add(-3 * time.Hour).Format(time.RFC3339),
				"open":      100.0,
				"high":      101.0,
				"low":       99.0,
				"close":     100.0,
				"isClosed":  true,
			},
		},
	}
}

func enablePretouchRiskOnShadow(ctx *StrategySignalEvaluationContext, sandbox bool) {
	ctx.ExecutionContext.Parameters["pretouchShadowMode"] = pretouchShadowModeTestnetCollect
	ctx.ExecutionContext.Parameters["pretouchShadowLeadScale"] = 1.5
	ctx.ExecutionContext.Parameters[pretouchShadowLeadQuantityBandSizingParam] = true
	ctx.ExecutionContext.Parameters["pretouchShadowLeadQuantityMinQuantity"] = defaultPretouchShadowLeadQuantityMinQty
	ctx.ExecutionContext.Parameters["pretouchShadowLeadQuantityMaxQuantity"] = defaultPretouchShadowLeadQuantityMaxQty
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayScale"] = 2.0
	ctx.ExecutionContext.Parameters["pretouchShadowOverlayBaseShare"] = 0.40
	ctx.ExecutionContext.Parameters["pretouchShadowOverlaySpeedThreshold"] = 0.35
	ctx.ExecutionContext.Parameters[pretouchShadowSubmitRiskOnQuantityParam] = true
	ctx.ExecutionContext.Parameters[pretouchShadowSubmitOverlayOrderParam] = true
	ctx.ExecutionContext.Semantics = defaultExecutionSemantics(ExecutionModeLive, ctx.ExecutionContext.Parameters)
	ctx.LiveAccountBinding = map[string]any{
		"sandbox":       sandbox,
		"executionMode": "rest",
	}
}

func setPretouchOrderBook(ctx *StrategySignalEvaluationContext, bestBid, bestAsk float64) {
	ctx.SourceStates["binance-order-book|feature|ETHUSDT|"].(map[string]any)["summary"] = map[string]any{
		"bestBid":    bestBid,
		"bestAsk":    bestAsk,
		"bestBidQty": 20.0,
		"bestAskQty": 10.0,
	}
}
