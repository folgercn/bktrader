package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestLaunchLiveFlowTemplateSwitchReplacesBindingsAndRefreshesRuntimePlan(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	for _, payload := range []map[string]any{
		{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "BTCUSDT",
			"options":   map[string]any{"timeframe": "5m"},
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
		if _, err := platform.BindStrategySignalSource("strategy-bk-1d", payload); err != nil {
			t.Fatalf("bind old strategy source failed: %v", err)
		}
	}

	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create old runtime session failed: %v", err)
	}
	runtimeSession.Status = "RUNNING"
	platform.signalSessions[runtimeSession.ID] = runtimeSession

	oldLiveSession, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "5m",
	})
	if err != nil {
		t.Fatalf("create old live session failed: %v", err)
	}
	oldLiveSession, err = platform.store.UpdateLiveSessionStatus(oldLiveSession.ID, "RUNNING")
	if err != nil {
		t.Fatalf("mark old live session running failed: %v", err)
	}
	targetScopeSession, err := platform.CreateLiveSession("live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "ETHUSDT",
		"signalTimeframe": "4h",
	})
	if err != nil {
		t.Fatalf("create target-scope live session failed: %v", err)
	}
	targetScopeSession, err = platform.store.UpdateLiveSessionStatus(targetScopeSession.ID, "RUNNING")
	if err != nil {
		t.Fatalf("mark target-scope live session running failed: %v", err)
	}
	otherAccount, err := platform.store.CreateAccount("Live Secondary", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create secondary live account failed: %v", err)
	}
	otherAccountSession, err := platform.CreateLiveSession(otherAccount.ID, "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "5m",
	})
	if err != nil {
		t.Fatalf("create secondary account live session failed: %v", err)
	}
	otherAccountSession, err = platform.store.UpdateLiveSessionStatus(otherAccountSession.ID, "RUNNING")
	if err != nil {
		t.Fatalf("mark secondary account live session running failed: %v", err)
	}

	result, err := platform.LaunchLiveFlow("live-main", LiveLaunchOptions{
		StrategyID: "strategy-bk-1d",
		Binding: map[string]any{
			"adapterKey":    "binance-futures",
			"positionMode":  "ONE_WAY",
			"marginMode":    "CROSSED",
			"sandbox":       true,
			"executionMode": "rest",
			"credentialRefs": map[string]any{
				"apiKeyRef":    "BINANCE_TESTNET_API_KEY",
				"apiSecretRef": "BINANCE_TESTNET_API_SECRET",
			},
		},
		StrategySignalBindings: []map[string]any{
			{
				"sourceKey": "binance-kline",
				"role":      "signal",
				"symbol":    "ETHUSDT",
				"options":   map[string]any{"timeframe": "4h"},
			},
			{
				"sourceKey": "binance-trade-tick",
				"role":      "trigger",
				"symbol":    "ETHUSDT",
			},
			{
				"sourceKey": "binance-order-book",
				"role":      "feature",
				"symbol":    "ETHUSDT",
			},
		},
		LiveSessionOverrides: map[string]any{
			"symbol":               "ETHUSDT",
			"signalTimeframe":      "4h",
			"defaultOrderQuantity": 0.1,
		},
		LaunchTemplateKey:  "binance-testnet-eth-4h",
		LaunchTemplateName: "Binance Testnet ETHUSDT 4h",
		StartRuntime:       false,
		StartSession:       false,
	})
	if err != nil {
		t.Fatalf("launch live flow failed: %v", err)
	}

	if !result.TemplateApplied {
		t.Fatal("expected template bindings to be applied")
	}
	if !result.RuntimePlanRefreshed {
		t.Fatal("expected runtime plan to be refreshed")
	}
	if result.StoppedLiveSessions != 1 {
		t.Fatalf("expected one conflicting live session to stop, got %d", result.StoppedLiveSessions)
	}
	if result.RuntimeSession.ID != runtimeSession.ID {
		t.Fatalf("expected existing runtime session to be reused, got %s", result.RuntimeSession.ID)
	}

	bindings, err := platform.ListStrategySignalBindings("strategy-bk-1d")
	if err != nil {
		t.Fatalf("list strategy bindings failed: %v", err)
	}
	if len(bindings) != 3 {
		t.Fatalf("expected exactly three template bindings, got %#v", bindings)
	}
	for _, binding := range bindings {
		if binding.Symbol != "ETHUSDT" {
			t.Fatalf("expected ETH-only bindings after template switch, got %#v", bindings)
		}
	}

	refreshedRuntime, err := platform.GetSignalRuntimeSession(runtimeSession.ID)
	if err != nil {
		t.Fatalf("get refreshed runtime session failed: %v", err)
	}
	if !result.RuntimeSessionStarted {
		t.Fatal("expected template launch to restart the runtime after refreshing the plan")
	}
	subscriptions := metadataList(refreshedRuntime.State["subscriptions"])
	if len(subscriptions) != 3 {
		t.Fatalf("expected three refreshed subscriptions, got %#v", subscriptions)
	}
	for _, subscription := range subscriptions {
		if got := NormalizeSymbol(stringValue(subscription["symbol"])); got != "ETHUSDT" {
			t.Fatalf("expected ETH-only subscriptions, got %#v", subscriptions)
		}
	}
	if got := stringValue(refreshedRuntime.State["launchTemplateKey"]); got != "binance-testnet-eth-4h" {
		t.Fatalf("expected runtime launch template key, got %s", got)
	}

	stoppedOldLiveSession, err := platform.store.GetLiveSession(oldLiveSession.ID)
	if err != nil {
		t.Fatalf("load old live session failed: %v", err)
	}
	if stoppedOldLiveSession.Status != "STOPPED" {
		t.Fatalf("expected old live session to stop, got %s", stoppedOldLiveSession.Status)
	}
	preservedTargetSession, err := platform.store.GetLiveSession(targetScopeSession.ID)
	if err != nil {
		t.Fatalf("load target-scope live session failed: %v", err)
	}
	if preservedTargetSession.Status != "RUNNING" {
		t.Fatalf("expected target-scope live session to remain running, got %s", preservedTargetSession.Status)
	}
	preservedOtherAccountSession, err := platform.store.GetLiveSession(otherAccountSession.ID)
	if err != nil {
		t.Fatalf("load secondary account live session failed: %v", err)
	}
	if preservedOtherAccountSession.Status != "RUNNING" {
		t.Fatalf("expected other-account live session to remain running, got %s", preservedOtherAccountSession.Status)
	}

	if result.LiveSession.ID != targetScopeSession.ID {
		t.Fatalf("expected target-scope live session to be reused, got %s", result.LiveSession.ID)
	}
	if got := stringValue(result.LiveSession.State["symbol"]); got != "ETHUSDT" {
		t.Fatalf("expected live session symbol ETHUSDT, got %s", got)
	}
	if got := stringValue(result.LiveSession.State["signalTimeframe"]); got != "4h" {
		t.Fatalf("expected live session timeframe 4h, got %s", got)
	}
	if got := stringValue(result.LiveSession.State["launchTemplateKey"]); got != "binance-testnet-eth-4h" {
		t.Fatalf("expected live session launch template key, got %s", got)
	}
}

func TestSyncSignalRuntimeSessionPlanRefreshesStateWithoutStartingRuntime(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	for _, payload := range []map[string]any{
		{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "BTCUSDT",
			"options":   map[string]any{"timeframe": "5m"},
		},
		{
			"sourceKey": "binance-trade-tick",
			"role":      "trigger",
			"symbol":    "BTCUSDT",
		},
	} {
		if _, err := platform.BindStrategySignalSource("strategy-bk-1d", payload); err != nil {
			t.Fatalf("bind initial strategy source failed: %v", err)
		}
	}

	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	runtimeSession.Status = "STOPPED"
	platform.signalSessions[runtimeSession.ID] = runtimeSession

	if _, err := platform.replaceStrategySignalSources("strategy-bk-1d", []map[string]any{
		{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "ETHUSDT",
			"options":   map[string]any{"timeframe": "4h"},
		},
		{
			"sourceKey": "binance-trade-tick",
			"role":      "trigger",
			"symbol":    "ETHUSDT",
		},
	}); err != nil {
		t.Fatalf("replace strategy sources failed: %v", err)
	}

	refreshed, err := platform.syncSignalRuntimeSessionPlan(runtimeSession.ID)
	if err != nil {
		t.Fatalf("refresh runtime plan failed: %v", err)
	}
	if refreshed.Status != "STOPPED" {
		t.Fatalf("expected runtime status to remain stopped after plan refresh, got %s", refreshed.Status)
	}
	subscriptions := metadataList(refreshed.State["subscriptions"])
	if len(subscriptions) != 2 {
		t.Fatalf("expected two refreshed subscriptions, got %#v", subscriptions)
	}
	for _, subscription := range subscriptions {
		if got := NormalizeSymbol(stringValue(subscription["symbol"])); got != "ETHUSDT" {
			t.Fatalf("expected ETH-only subscriptions after plan refresh, got %#v", subscriptions)
		}
	}
	lastEvent := mapValue(refreshed.State["lastEventSummary"])
	if got := stringValue(lastEvent["type"]); got != "runtime_plan_refreshed" {
		t.Fatalf("expected runtime_plan_refreshed event, got %s", got)
	}
	if got := stringValue(lastEvent["message"]); got != "signal runtime plan refreshed; new subscriptions apply on next runtime start" {
		t.Fatalf("expected explicit plan refresh message, got %s", got)
	}
}
