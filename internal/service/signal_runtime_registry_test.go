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

func TestBuildSignalRuntimePlanCarriesLiveBindingSandboxToSubscriptions(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.store.CreateAccount("runtime-live", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create live account failed: %v", err)
	}
	strategy, err := platform.store.CreateStrategy("runtime-strategy", "runtime strategy", map[string]any{
		"strategyEngine": "bk-default",
	})
	if err != nil {
		t.Fatalf("create strategy failed: %v", err)
	}
	strategyID := strategy["id"].(string)
	if _, err := platform.BindStrategySignalSource(strategyID, map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "30m"},
	}); err != nil {
		t.Fatalf("bind strategy signal source failed: %v", err)
	}
	if _, err := platform.BindLiveAccount(account.ID, map[string]any{
		"adapterKey":  "binance-futures",
		"sandbox":     true,
		"restBaseUrl": "https://testnet.binancefuture.com",
		"wsBaseUrl":   "wss://stream.binancefuture.com/ws",
	}); err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}

	plan, err := platform.BuildSignalRuntimePlan(account.ID, strategyID)
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	subscriptions := metadataList(plan["subscriptions"])
	if len(subscriptions) != 1 {
		t.Fatalf("expected one subscription, got %#v", subscriptions)
	}
	subscription := subscriptions[0]
	if !boolValue(subscription["sandbox"]) {
		t.Fatalf("expected sandbox=true on runtime subscription, got %#v", subscription)
	}
	if got := stringValue(subscription["restBaseUrl"]); got != "https://testnet.binancefuture.com" {
		t.Fatalf("expected restBaseUrl to propagate to runtime subscription, got %q", got)
	}
	if got := stringValue(subscription["wsBaseUrl"]); got != "wss://stream.binancefuture.com/ws" {
		t.Fatalf("expected wsBaseUrl to propagate to runtime subscription, got %q", got)
	}
}
