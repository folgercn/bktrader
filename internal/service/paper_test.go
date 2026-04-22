package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestPlatformFreshnessOverridePriority(t *testing.T) {
	p := &Platform{
		runtimePolicy: RuntimePolicy{
			SignalBarFreshnessSeconds: 60,
			TradeTickFreshnessSeconds: 30,
			OrderBookFreshnessSeconds: 15,
		},
	}

	binding := domain.AccountSignalBinding{
		StreamType: "signal_bar",
		Options: map[string]any{
			"freshnessSeconds": 120.0,
		},
	}

	// 1. 无覆盖时使用 Binding Options
	if got := p.signalSourceFreshnessWindowWithOverride(binding, nil); got != 120*time.Second {
		t.Fatalf("expected 120s from binding options, got %v", got)
	}

	// 2. 有 Session 覆盖时，Session 优先级最高
	sessionState := map[string]any{
		"freshnessOverride": map[string]any{
			"signalBarFreshnessSeconds": 45.0,
		},
	}
	if got := p.signalSourceFreshnessWindowWithOverride(binding, sessionState); got != 45*time.Second {
		t.Fatalf("expected 45s from session override, got %v", got)
	}

	// 3. 无 Binding Options 且无 Session 覆盖时，使用全局默认
	bindingEmpty := domain.AccountSignalBinding{StreamType: "signal_bar"}
	if got := p.signalSourceFreshnessWindowWithOverride(bindingEmpty, nil); got != 60*time.Second {
		t.Fatalf("expected 60s from global policy, got %v", got)
	}
}

func TestPlatformRuntimeQuietIsolation(t *testing.T) {
	p := &Platform{
		runtimePolicy: RuntimePolicy{
			SignalBarFreshnessSeconds: 60,
			RuntimeQuietSeconds:       300,
		},
	}

	binding := domain.AccountSignalBinding{StreamType: "signal_bar"}

	// 设置只含有 runtimeQuietSeconds 的覆盖
	sessionState := map[string]any{
		"freshnessOverride": map[string]any{
			"runtimeQuietSeconds": 10.0,
		},
	}

	// 1. 验证 runtimeQuietSeconds 覆盖生效于 quiet 判断
	if !p.runtimeSessionQuiet(map[string]any{
		"lastEventAt":       time.Now().Add(-15 * time.Second).Format(time.RFC3339),
		"freshnessOverride": sessionState["freshnessOverride"],
	}) {
		t.Fatal("expected session to be quiet with 10s threshold (event was 15s ago)")
	}

	// 2. 核心验证：runtimeQuietSeconds 绝不应该影响数据源新鲜度校验 (应回退到全局 60s 而不是误用 10s)
	if got := p.signalSourceFreshnessWindowWithOverride(binding, sessionState); got != 60*time.Second {
		t.Fatalf("runtimeQuietSeconds override must NOT affect signal bar freshness. expected 60s, got %v", got)
	}
}

func TestEvaluateRuntimeSignalSourceReadinessAllowsMissingFeatureBinding(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	bindRuntimeReadinessSources(t, platform)

	eventTime := time.Date(2026, 4, 22, 2, 35, 31, 0, time.UTC)
	runtimeSession := domain.SignalRuntimeSession{
		StrategyID: "strategy-bk-1d",
		State: map[string]any{
			"sourceStates": runtimeSourceStatesForReadiness(t, platform, eventTime, map[string]time.Duration{
				"signal":  -5 * time.Second,
				"trigger": -5 * time.Second,
			}),
		},
	}

	sourceGate := platform.evaluateRuntimeSignalSourceReadiness("strategy-bk-1d", runtimeSession, eventTime)
	if !boolValue(sourceGate["ready"]) {
		t.Fatalf("expected missing feature binding to stay advisory, got %#v", sourceGate)
	}
	if got := len(metadataList(sourceGate["missing"])); got != 0 {
		t.Fatalf("expected no blocking missing bindings, got %d", got)
	}
	if got := len(metadataList(sourceGate["stale"])); got != 0 {
		t.Fatalf("expected no blocking stale bindings, got %d", got)
	}
	if advisory := metadataList(sourceGate["advisoryMissing"]); len(advisory) != 1 || normalizeSignalSourceRole(stringValue(advisory[0]["role"])) != "feature" {
		t.Fatalf("expected one advisory missing feature binding, got %#v", advisory)
	}
	if got := maxIntValue(sourceGate["requiredCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking bindings, got %d", got)
	}
	if got := maxIntValue(sourceGate["availableCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking available bindings, got %d", got)
	}
	if got := maxIntValue(sourceGate["freshCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking fresh bindings, got %d", got)
	}
}

func TestEvaluateRuntimeSignalSourceReadinessAllowsStaleFeatureBinding(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	bindRuntimeReadinessSources(t, platform)

	eventTime := time.Date(2026, 4, 22, 2, 35, 31, 0, time.UTC)
	runtimeSession := domain.SignalRuntimeSession{
		StrategyID: "strategy-bk-1d",
		State: map[string]any{
			"sourceStates": runtimeSourceStatesForReadiness(t, platform, eventTime, map[string]time.Duration{
				"signal":  -5 * time.Second,
				"trigger": -5 * time.Second,
				"feature": -11 * time.Second,
			}),
		},
	}

	sourceGate := platform.evaluateRuntimeSignalSourceReadiness("strategy-bk-1d", runtimeSession, eventTime)
	if !boolValue(sourceGate["ready"]) {
		t.Fatalf("expected stale feature binding to stay advisory, got %#v", sourceGate)
	}
	if got := len(metadataList(sourceGate["missing"])); got != 0 {
		t.Fatalf("expected no blocking missing bindings, got %d", got)
	}
	if got := len(metadataList(sourceGate["stale"])); got != 0 {
		t.Fatalf("expected no blocking stale bindings, got %d", got)
	}
	if advisory := metadataList(sourceGate["advisoryStale"]); len(advisory) != 1 || normalizeSignalSourceRole(stringValue(advisory[0]["role"])) != "feature" {
		t.Fatalf("expected one advisory stale feature binding, got %#v", advisory)
	}
	if got := maxIntValue(sourceGate["requiredCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking bindings, got %d", got)
	}
	if got := maxIntValue(sourceGate["availableCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking available bindings, got %d", got)
	}
	if got := maxIntValue(sourceGate["freshCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking fresh bindings, got %d", got)
	}
}

func TestEvaluateRuntimeSignalSourceReadinessBlocksStaleTriggerBinding(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	bindRuntimeReadinessSources(t, platform)

	eventTime := time.Date(2026, 4, 22, 2, 35, 31, 0, time.UTC)
	runtimeSession := domain.SignalRuntimeSession{
		StrategyID: "strategy-bk-1d",
		State: map[string]any{
			"sourceStates": runtimeSourceStatesForReadiness(t, platform, eventTime, map[string]time.Duration{
				"signal":  -5 * time.Second,
				"trigger": -16 * time.Second,
				"feature": -5 * time.Second,
			}),
		},
	}

	sourceGate := platform.evaluateRuntimeSignalSourceReadiness("strategy-bk-1d", runtimeSession, eventTime)
	if boolValue(sourceGate["ready"]) {
		t.Fatalf("expected stale trigger binding to block readiness, got %#v", sourceGate)
	}
	if stale := metadataList(sourceGate["stale"]); len(stale) != 1 || normalizeSignalSourceRole(stringValue(stale[0]["role"])) != "trigger" {
		t.Fatalf("expected one blocking stale trigger binding, got %#v", stale)
	}
	if advisory := metadataList(sourceGate["advisoryStale"]); len(advisory) != 0 {
		t.Fatalf("expected no advisory stale bindings, got %#v", advisory)
	}
	if got := maxIntValue(sourceGate["requiredCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking bindings, got %d", got)
	}
	if got := maxIntValue(sourceGate["availableCount"], -1); got != 2 {
		t.Fatalf("expected 2 blocking available bindings, got %d", got)
	}
	if got := maxIntValue(sourceGate["freshCount"], -1); got != 1 {
		t.Fatalf("expected only 1 blocking fresh binding, got %d", got)
	}
}

func bindRuntimeReadinessSources(t *testing.T, platform *Platform) {
	t.Helper()

	for _, binding := range []map[string]any{
		{
			"sourceKey": "binance-kline",
			"role":      "signal",
			"symbol":    "BTCUSDT",
			"options":   map[string]any{"timeframe": "30m"},
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
}

func runtimeSourceStatesForReadiness(t *testing.T, platform *Platform, eventTime time.Time, offsets map[string]time.Duration) map[string]any {
	t.Helper()

	bindings, err := platform.ListStrategySignalBindings("strategy-bk-1d")
	if err != nil {
		t.Fatalf("list strategy signal bindings failed: %v", err)
	}

	sourceStates := make(map[string]any, len(bindings))
	for _, binding := range bindings {
		offset, ok := offsets[normalizeSignalSourceRole(binding.Role)]
		if !ok {
			continue
		}
		entry := map[string]any{
			"sourceKey":   binding.SourceKey,
			"role":        binding.Role,
			"streamType":  binding.StreamType,
			"symbol":      binding.Symbol,
			"options":     cloneMetadata(binding.Options),
			"lastEventAt": eventTime.Add(offset).Format(time.RFC3339),
		}
		if timeframe := signalBindingTimeframe(binding.SourceKey, binding.Options); timeframe != "" {
			entry["timeframe"] = timeframe
		}
		sourceStates[signalBindingKey(binding)] = entry
	}
	return sourceStates
}
