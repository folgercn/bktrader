package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
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
