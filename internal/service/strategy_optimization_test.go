package service

import (
	"math"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestEvaluateLivePositionStateTrailingStop(t *testing.T) {
	parameters := map[string]any{
		"trailing_stop_atr":               0.3,
		"delayed_trailing_activation_atr": 0.5,
		"stop_loss_atr":                   0.05,
		"stop_mode":                       "atr",
	}

	signalBarState := map[string]any{
		"atr14":    1000.0,
		"current":  map[string]any{"close": 50000.0},
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
			"sessionReentryCount":  2.0, // Two prior confirmed reentries => this is the 3rd reentry.
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
	baseQuantity, _ := resolveExecutionQuantity(ctx.Session, ctx.Account, ctx.Execution.Parameters, ctx.Intent, priceHint)

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

func TestEffectiveReentryCountForSizingResetsOnNewSignalBar(t *testing.T) {
	sessionState := map[string]any{
		"sessionReentryCount":   2.0,
		"lastSignalBarStateKey": "bar-1",
	}
	if got := effectiveReentryCountForSizing(sessionState, map[string]any{
		"signalBarStateKey": "bar-2",
	}); got != 0 {
		t.Fatalf("expected new bar to reset effective reentry count, got %v", got)
	}
	if got := effectiveReentryCountForSizing(sessionState, map[string]any{
		"signalBarStateKey": "bar-1",
	}); got != 2 {
		t.Fatalf("expected same bar to keep reentry count, got %v", got)
	}
}

func TestResolveExecutionQuantityVolatilityAdjustedUsesStopDistance(t *testing.T) {
	quantity, metadata := resolveExecutionQuantity(
		domain.LiveSession{
			State: map[string]any{
				"positionSizingMode": "volatility_adjusted",
				"atr14":              1000.0,
				"targetRiskBps":      100.0,
			},
		},
		domain.Account{
			Metadata: map[string]any{
				"liveSyncSnapshot": map[string]any{
					"availableBalance": 10000.0,
				},
			},
		},
		map[string]any{
			"stop_loss_atr": 0.05,
		},
		SignalIntent{},
		50000.0,
	)
	if quantity != 2.0 {
		t.Fatalf("expected quantity 2.0 with 50 USDT/unit risk, got %v", quantity)
	}
	if got := parseFloatValue(metadata["sizingRiskPerUnit"]); got != 50.0 {
		t.Fatalf("expected risk per unit 50, got %v", got)
	}
}

func TestUpdateLivePositionWatermarksResetsWhenPositionChanges(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-old",
	}
	hwm, lwm := updateLivePositionWatermarks(sessionState, map[string]any{
		"id":         "position-new",
		"symbol":     "ETHUSDT",
		"side":       "short",
		"entryPrice": 48000.0,
		"quantity":   1.0,
		"found":      true,
	}, 47900.0)
	if hwm != 48000.0 {
		t.Fatalf("expected hwm to reset to new entry price, got %v", hwm)
	}
	if lwm != 47900.0 {
		t.Fatalf("expected lwm to update from new position context, got %v", lwm)
	}
	if got := stringValue(sessionState["watermarkPositionKey"]); got != "position-new|ETHUSDT|SHORT|48000.00000000" {
		t.Fatalf("expected watermark key to reset, got %s", got)
	}
}

func TestResolveAndApplyLivePositionWatermarksAreSeparated(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-1|BTCUSDT|LONG|50000.00000000",
	}
	currentPosition := map[string]any{
		"id":         "position-1",
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   1.0,
		"found":      true,
	}
	expectedKey := "position-1|BTCUSDT|LONG|50000.00000000"
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if watermarks.HWM != 52000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected resolved watermarks from session state, got %+v", watermarks)
	}
	if got := stringValue(sessionState["watermarkPositionKey"]); got != expectedKey {
		t.Fatalf("expected resolve to stay side-effect free, got %s", got)
	}
	if watermarks.PositionKey != expectedKey {
		t.Fatalf("expected resolved position key %s, got %s", expectedKey, watermarks.PositionKey)
	}
	advanced := advanceLivePositionWatermarks(watermarks, 52500.0)
	if advanced.HWM != 52500.0 {
		t.Fatalf("expected advanced HWM 52500, got %v", advanced.HWM)
	}
	if parseFloatValue(sessionState["hwm"]) != 52000.0 {
		t.Fatalf("expected advance to stay side-effect free, got %v", sessionState["hwm"])
	}
	applyLivePositionWatermarks(sessionState, advanced)
	if parseFloatValue(sessionState["hwm"]) != 52500.0 {
		t.Fatalf("expected apply to persist advanced HWM, got %v", sessionState["hwm"])
	}
	if parseFloatValue(sessionState["lwm"]) != 50000.0 {
		t.Fatalf("expected apply to preserve LWM, got %v", sessionState["lwm"])
	}
	if got := stringValue(sessionState["watermarkPositionKey"]); got != expectedKey {
		t.Fatalf("expected apply to persist watermark key, got %s", got)
	}
}

func TestResolveLivePositionWatermarksUsesExpandedPositionContextKey(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-1|BTCUSDT|LONG|50000.00000000",
	}
	currentPosition := map[string]any{
		"id":         "position-2",
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   1.0,
		"found":      true,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if got := watermarks.PositionKey; got != "position-2|BTCUSDT|LONG|50000.00000000" {
		t.Fatalf("expected new position id to reset watermark key, got %s", got)
	}
	if watermarks.HWM != 50000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected watermarks to reset for new position id, got %+v", watermarks)
	}
}

func TestResolveLivePositionWatermarksUsesVirtualPositionID(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "virtual|session-1|signal-1",
	}
	currentPosition := map[string]any{
		"id":         "virtual|session-1|signal-2",
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   1.0,
		"virtual":    true,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if got := watermarks.PositionKey; got != "virtual|session-1|signal-2|BTCUSDT|LONG|50000.00000000" {
		t.Fatalf("expected virtual position id to drive watermark key, got %s", got)
	}
	if watermarks.HWM != 50000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected watermarks to reset for a new virtual position identity, got %+v", watermarks)
	}
}

func TestResolveLivePositionWatermarksSupportsLegacyVirtualKey(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "virtual|session-1|signal-2",
	}
	currentPosition := map[string]any{
		"id":         "virtual|session-1|signal-2",
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   1.0,
		"virtual":    true,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if got := watermarks.PositionKey; got != "virtual|session-1|signal-2" {
		t.Fatalf("expected legacy virtual watermark key to remain stable, got %s", got)
	}
	if watermarks.HWM != 52000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected legacy virtual key to preserve watermarks, got %+v", watermarks)
	}
}

func TestResolveLivePositionWatermarksPreservesPreviousKeyWhenIDTemporarilyMissing(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-1|BTCUSDT|LONG|50000.00000000",
	}
	currentPosition := map[string]any{
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   1.0,
		"found":      true,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if got := watermarks.PositionKey; got != "position-1|BTCUSDT|LONG|50000.00000000" {
		t.Fatalf("expected missing id to preserve previous stable watermark key, got %s", got)
	}
	if watermarks.HWM != 52000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected missing id to preserve existing watermarks, got %+v", watermarks)
	}
}

func TestRefreshLivePositionWatermarksClearsStateWhenPositionIsInactive(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-1|BTCUSDT|LONG|50000.00000000",
	}
	watermarks := refreshLivePositionWatermarks(sessionState, map[string]any{
		"symbol":   "BTCUSDT",
		"quantity": 0.0,
		"found":    false,
	}, 0)
	if watermarks.PositionKey != "" || watermarks.HWM != 0 || watermarks.LWM != 0 {
		t.Fatalf("expected empty watermarks for inactive position, got %+v", watermarks)
	}
	if _, ok := sessionState["watermarkPositionKey"]; ok {
		t.Fatal("expected watermarkPositionKey to be cleared when position becomes inactive")
	}
	if _, ok := sessionState["hwm"]; ok {
		t.Fatal("expected hwm to be cleared when position becomes inactive")
	}
	if _, ok := sessionState["lwm"]; ok {
		t.Fatal("expected lwm to be cleared when position becomes inactive")
	}
}

func TestResolveLivePositionWatermarksSupportsLegacyRealPositionKey(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "LONG|50000.00000000",
	}
	currentPosition := map[string]any{
		"id":         "position-1",
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   1.0,
		"found":      true,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if got := watermarks.PositionKey; got != "LONG|50000.00000000" {
		t.Fatalf("expected legacy real-position watermark key to remain stable during migration, got %s", got)
	}
	if watermarks.HWM != 52000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected legacy real-position key to preserve watermarks, got %+v", watermarks)
	}
}

func TestHasActiveLivePositionSnapshotUsesAbsoluteQuantity(t *testing.T) {
	if !hasActiveLivePositionSnapshot(map[string]any{"quantity": -1.0}) {
		t.Fatal("expected negative quantity snapshot to count as active")
	}
}

func TestDeriveLivePositionStateUsesProvidedWatermarks(t *testing.T) {
	parameters := map[string]any{
		"trailing_stop_atr":               0.3,
		"delayed_trailing_activation_atr": 0.5,
		"stop_loss_atr":                   0.05,
		"stop_mode":                       "atr",
	}
	signalBarState := map[string]any{
		"atr14":    1000.0,
		"current":  map[string]any{"close": 50600.0},
		"prevBar1": map[string]any{"high": 50500.0, "low": 49500.0},
		"prevBar2": map[string]any{"high": 50600.0, "low": 49400.0},
	}
	currentPosition := map[string]any{
		"found":      true,
		"side":       "LONG",
		"entryPrice": 50000.0,
		"stopLoss":   49950.0,
		"quantity":   1.0,
	}
	watermarks := livePositionWatermarks{
		PositionKey: "position-1|BTCUSDT|LONG",
		HWM:         50600.0,
		LWM:         50000.0,
	}
	state := deriveLivePositionState(parameters, currentPosition, signalBarState, 50600.0, watermarks)
	if got := parseFloatValue(state["stopLoss"]); got != 50300.0 {
		t.Fatalf("expected trailing SL 50300 from provided watermarks, got %v", got)
	}
	if got := parseFloatValue(state["hwm"]); got != 50600.0 {
		t.Fatalf("expected returned HWM 50600, got %v", got)
	}
}

func TestDeriveLivePositionStateUsesProvidedWatermarksForShort(t *testing.T) {
	parameters := map[string]any{
		"trailing_stop_atr":               0.3,
		"delayed_trailing_activation_atr": 0.5,
		"stop_loss_atr":                   0.05,
		"stop_mode":                       "atr",
	}
	signalBarState := map[string]any{
		"atr14":    1000.0,
		"current":  map[string]any{"close": 49400.0},
		"prevBar1": map[string]any{"high": 50500.0, "low": 49500.0},
		"prevBar2": map[string]any{"high": 50600.0, "low": 49400.0},
	}
	currentPosition := map[string]any{
		"found":      true,
		"side":       "SHORT",
		"entryPrice": 50000.0,
		"stopLoss":   50050.0,
		"quantity":   1.0,
	}
	watermarks := livePositionWatermarks{
		PositionKey: "position-2|BTCUSDT|SHORT",
		HWM:         50000.0,
		LWM:         49400.0,
	}
	state := deriveLivePositionState(parameters, currentPosition, signalBarState, 49400.0, watermarks)
	if got := parseFloatValue(state["stopLoss"]); got != 49700.0 {
		t.Fatalf("expected trailing SL 49700 from provided short watermarks, got %v", got)
	}
	if got := parseFloatValue(state["lwm"]); got != 49400.0 {
		t.Fatalf("expected returned LWM 49400, got %v", got)
	}
}

func TestResolveLivePositionWatermarksIgnoresInactiveSnapshot(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-1|BTCUSDT|LONG",
	}
	currentPosition := map[string]any{
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   0.0,
		"found":      false,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if watermarks.PositionKey != "" || watermarks.HWM != 0 || watermarks.LWM != 0 {
		t.Fatalf("expected inactive snapshot to skip watermark refresh, got %+v", watermarks)
	}
	if got := stringValue(sessionState["watermarkPositionKey"]); got != "position-1|BTCUSDT|LONG" {
		t.Fatalf("expected inactive snapshot to preserve persisted watermark key, got %s", got)
	}
	if got := parseFloatValue(sessionState["hwm"]); got != 52000.0 {
		t.Fatalf("expected inactive snapshot to preserve persisted HWM, got %v", got)
	}
	if got := parseFloatValue(sessionState["lwm"]); got != 50000.0 {
		t.Fatalf("expected inactive snapshot to preserve persisted LWM, got %v", got)
	}
}

func TestResolveLivePositionWatermarksIgnoresQuantityChangesWithinSamePosition(t *testing.T) {
	sessionState := map[string]any{
		"hwm":                  52000.0,
		"lwm":                  50000.0,
		"watermarkPositionKey": "position-1|BTCUSDT|LONG",
	}
	currentPosition := map[string]any{
		"id":         "position-1",
		"symbol":     "BTCUSDT",
		"side":       "LONG",
		"entryPrice": 50000.0,
		"quantity":   0.6,
		"found":      true,
	}
	watermarks := resolveLivePositionWatermarks(currentPosition, sessionState)
	if got := watermarks.PositionKey; got != "position-1|BTCUSDT|LONG" {
		t.Fatalf("expected quantity changes to keep the same watermark key, got %s", got)
	}
	if watermarks.HWM != 52000.0 || watermarks.LWM != 50000.0 {
		t.Fatalf("expected quantity changes to preserve watermarks, got %+v", watermarks)
	}
}
