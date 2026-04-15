package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestBuildSignalRuntimePlanWithoutBindingsIsNotReady(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	plan, err := platform.BuildSignalRuntimePlan("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	if boolValue(plan["ready"]) {
		t.Fatalf("expected runtime plan without subscriptions to be not ready: %#v", plan)
	}
	if subscriptions := metadataList(plan["subscriptions"]); len(subscriptions) != 0 {
		t.Fatalf("expected no subscriptions for unbound strategy, got %#v", subscriptions)
	}
	if matched := metadataList(plan["matchedBindings"]); len(matched) != 0 {
		t.Fatalf("expected no matched bindings for unbound strategy, got %#v", matched)
	}
	if missing := metadataList(plan["missingBindings"]); len(missing) != 0 {
		t.Fatalf("expected no missing bindings for unbound strategy, got %#v", missing)
	}
}

func TestBuildSignalRuntimePlanAllowsMissingFeatureBindings(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	platform.signalAdapters = map[string]SignalRuntimeAdapter{
		"binance-market-ws-lite": staticSignalRuntimeAdapter{
			key:          "binance-market-ws-lite",
			name:         "Binance Market Data WebSocket (Lite)",
			transport:    "websocket",
			exchange:     "BINANCE",
			streamTypes:  []string{"signal_bar", "trade_tick"},
			environments: []string{"paper", "live"},
		},
	}

	for _, binding := range []map[string]any{
		{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "BTCUSDT",
			"options":   map[string]any{"timeframe": "1d"},
		},
		{
			"sourceKey": "binance-trade-tick",
			"role":      "trigger",
			"symbol":    "BTCUSDT",
		},
		{
			"sourceKey": "binance-order-book",
			"role":      "feature",
			"symbol":    "BTCUSDT",
		},
	} {
		if _, err := platform.BindStrategySignalSource("strategy-bk-1d", binding); err != nil {
			t.Fatalf("bind strategy source failed: %v", err)
		}
	}

	plan, err := platform.BuildSignalRuntimePlan("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	if !boolValue(plan["ready"]) {
		t.Fatalf("expected runtime plan to stay ready when only feature binding is missing: %#v", plan)
	}
	if subscriptions := metadataList(plan["subscriptions"]); len(subscriptions) != 2 {
		t.Fatalf("expected signal+trigger subscriptions only, got %#v", subscriptions)
	}
	if missing := metadataList(plan["missingBindings"]); len(missing) != 1 || normalizeSignalSourceRole(stringValue(missing[0]["role"])) != "feature" {
		t.Fatalf("expected a single missing feature binding, got %#v", missing)
	}
	if blocking := metadataList(plan["blockingMissingBindings"]); len(blocking) != 0 {
		t.Fatalf("expected no blocking missing bindings, got %#v", blocking)
	}
	if optional := metadataList(plan["optionalMissingBindings"]); len(optional) != 1 || normalizeSignalSourceRole(stringValue(optional[0]["role"])) != "feature" {
		t.Fatalf("expected optional missing feature binding, got %#v", optional)
	}
}
