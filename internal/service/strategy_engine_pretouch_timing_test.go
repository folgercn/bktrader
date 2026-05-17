package service

import (
	"math"
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
	engine.model = nil

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
	return &bkLiveEthPretouchTimingEngine{
		platform: NewPlatform(memory.NewStore()),
		detector: NewPretouchEventDetector("ETHUSDT", config),
		config:   config,
		model: &PretouchModelBundle{
			TimingTree: &TreeNode{FeatureIndex: -1, LeafValue: timingRegime, LeafProba: 1},
			RFModel: &RandomForest{
				Trees:       []*TreeNode{{FeatureIndex: -1, LeafValue: "1", LeafProba: rfProba}},
				NEstimators: 1,
			},
			FeatureNames: pretouchTrainFeatures,
			Medians:      make([]float64, len(pretouchTrainFeatures)),
			Version:      "test",
			RFAccuracy:   0.7,
		},
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

func setPretouchOrderBook(ctx *StrategySignalEvaluationContext, bestBid, bestAsk float64) {
	ctx.SourceStates["binance-order-book|feature|ETHUSDT|"].(map[string]any)["summary"] = map[string]any{
		"bestBid":    bestBid,
		"bestAsk":    bestAsk,
		"bestBidQty": 20.0,
		"bestAskQty": 10.0,
	}
}
