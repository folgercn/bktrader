package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestSignalBindingMatchKeyDefaultsEmptyBinanceKlineTimeframeTo1d(t *testing.T) {
	withExplicit1D := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{
		"timeframe": "1d",
	})
	withImplicitDefault := signalBindingMatchKey("binance-kline", "signal", "BTCUSDT", map[string]any{})
	if withExplicit1D != withImplicitDefault {
		t.Fatalf("expected empty kline timeframe to canonicalize to 1d, got %q vs %q", withExplicit1D, withImplicitDefault)
	}
}

func TestBindStrategySignalSourceDoesNotDuplicateImplicitDefault1dKlineBinding(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind explicit 1d kline failed: %v", err)
	}

	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
	}); err != nil {
		t.Fatalf("bind implicit default kline failed: %v", err)
	}

	bindings, err := platform.ListStrategySignalBindings("strategy-bk-1d")
	if err != nil {
		t.Fatalf("list strategy bindings failed: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected a single deduped kline binding, got %#v", bindings)
	}
	if got := stringValue(bindings[0].Options["timeframe"]); got != "1d" {
		t.Fatalf("expected canonicalized timeframe 1d, got %s", got)
	}

	plan, err := platform.BuildSignalRuntimePlan("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	subscriptions := metadataList(plan["subscriptions"])
	if len(subscriptions) != 1 {
		t.Fatalf("expected one kline subscription after dedupe, got %#v", subscriptions)
	}
	if got := stringValue(subscriptions[0]["channel"]); got != "btcusdt@kline_1d" {
		t.Fatalf("expected canonicalized 1d kline subscription, got %s", got)
	}
}

func TestLegacyStrategyBindingWithoutTimeframeCanonicalizesToSingle1dBinding(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	strategy, err := platform.GetStrategy("strategy-bk-1d")
	if err != nil {
		t.Fatalf("get strategy failed: %v", err)
	}
	currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
	if !ok {
		t.Fatalf("expected current version for strategy, got %#v", strategy["currentVersion"])
	}
	parameters := cloneMetadata(currentVersion.Parameters)
	parameters["signalBindings"] = []map[string]any{
		{
			"id":         "legacy-kline-binding",
			"sourceKey":  "binance-kline",
			"sourceName": "Binance Futures Kline",
			"exchange":   "BINANCE",
			"role":       "signal",
			"streamType": "signal_bar",
			"symbol":     "BTCUSDT",
			"status":     "ACTIVE",
		},
	}
	if _, err := platform.store.UpdateStrategyParameters("strategy-bk-1d", parameters); err != nil {
		t.Fatalf("inject legacy strategy binding failed: %v", err)
	}

	legacyBindings, err := platform.ListStrategySignalBindings("strategy-bk-1d")
	if err != nil {
		t.Fatalf("list legacy bindings failed: %v", err)
	}
	if len(legacyBindings) != 1 {
		t.Fatalf("expected single legacy binding, got %#v", legacyBindings)
	}
	if got := stringValue(legacyBindings[0].Options["timeframe"]); got != "1d" {
		t.Fatalf("expected legacy binding timeframe to canonicalize to 1d, got %#v", legacyBindings[0].Options)
	}

	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind explicit 1d over legacy binding failed: %v", err)
	}

	bindings, err := platform.ListStrategySignalBindings("strategy-bk-1d")
	if err != nil {
		t.Fatalf("list strategy bindings after upgrade failed: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected legacy + explicit 1d to collapse into one canonical binding, got %#v", bindings)
	}

	plan, err := platform.BuildSignalRuntimePlan("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	subscriptions := metadataList(plan["subscriptions"])
	if len(subscriptions) != 1 {
		t.Fatalf("expected exactly one canonicalized subscription, got %#v", subscriptions)
	}
	if got := stringValue(subscriptions[0]["channel"]); got != "btcusdt@kline_1d" {
		t.Fatalf("expected legacy binding upgrade to keep canonical 1d channel, got %s", got)
	}
}
