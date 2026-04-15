package service

import (
	"testing"

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
