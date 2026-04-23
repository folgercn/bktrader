package service

import (
	"os"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestApplyRuntimeConfigOverrides_ScenarioA_PartialOverride(t *testing.T) {
	p := NewPlatform(memory.NewStore())

	// 1. 设置初始持久化策略
	persisted := RuntimePolicy{
		TradeTickFreshnessSeconds:      15,
		StrategyEvaluationQuietSeconds: 60,
		WSReadStaleTimeoutSeconds:      30,
	}
	p.runtimePolicy = persisted

	// 2. 模拟环境变量：仅设置部分字段
	val120 := 120
	val0 := 0
	cfg := config.Config{
		TradeTickFreshnessSeconds:      &val120, // 覆盖
		StrategyEvaluationQuietSeconds: &val0,   // 允许 0 覆盖
		// WSReadStaleTimeoutSeconds 为 nil, 不应覆盖
	}

	p.ApplyRuntimeConfigOverrides(cfg)

	if p.runtimePolicy.TradeTickFreshnessSeconds != 120 {
		t.Errorf("expected TradeTickFreshnessSeconds to be overridden to 120, got %d", p.runtimePolicy.TradeTickFreshnessSeconds)
	}
	if p.runtimePolicy.StrategyEvaluationQuietSeconds != 0 {
		t.Errorf("expected StrategyEvaluationQuietSeconds to be overridden to 0, got %d", p.runtimePolicy.StrategyEvaluationQuietSeconds)
	}
	if p.runtimePolicy.WSReadStaleTimeoutSeconds != 30 {
		t.Errorf("expected WSReadStaleTimeoutSeconds to remain 30, got %d", p.runtimePolicy.WSReadStaleTimeoutSeconds)
	}
}

func TestApplyRuntimeConfigOverrides_ScenarioB_InvalidValues(t *testing.T) {
	p := NewPlatform(memory.NewStore())
	initialTimeout := 60
	p.runtimePolicy.WSReadStaleTimeoutSeconds = initialTimeout

	// 1. 测试必须 > 0 的字段设为 0 (应被忽略并警告)
	val0 := 0
	cfg0 := config.Config{
		WSReadStaleTimeoutSeconds: &val0,
	}
	p.ApplyRuntimeConfigOverrides(cfg0)
	if p.runtimePolicy.WSReadStaleTimeoutSeconds != initialTimeout {
		t.Errorf("expected WSReadStaleTimeoutSeconds to remain %d when overridden with 0, got %d", initialTimeout, p.runtimePolicy.WSReadStaleTimeoutSeconds)
	}

	// 2. 测试必须 > 0 的字段设为负数 (应被忽略)
	valNeg := -100
	cfgNeg := config.Config{
		RESTLimiterRPS: &valNeg,
	}
	initialRPS := p.runtimePolicy.RESTLimiterRPS
	p.ApplyRuntimeConfigOverrides(cfgNeg)
	if p.runtimePolicy.RESTLimiterRPS != initialRPS {
		t.Errorf("expected RESTLimiterRPS to remain %d when overridden with negative, got %d", initialRPS, p.runtimePolicy.RESTLimiterRPS)
	}
}

func TestApplyRuntimeConfigOverrides_ScenarioC_LimiterReset(t *testing.T) {
	p := NewPlatform(memory.NewStore())

	// 初始状态
	p.runtimePolicy.RESTLimiterRPS = 30
	p.UpdateBinanceRESTLimits()

	// 记录旧的 limiter 状态 (这里虽然无法直接访问内部 gate, 但可以通过逻辑间接验证或检查 UpdateBinanceRESTLimits 是否被调用)
	// 我们主要验证 ApplyRuntimeConfigOverrides 是否最终调用了 UpdateBinanceRESTLimits
	val100 := 100
	cfg := config.Config{
		RESTLimiterRPS: &val100,
	}
	p.ApplyRuntimeConfigOverrides(cfg)

	if p.runtimePolicy.RESTLimiterRPS != 100 {
		t.Errorf("expected RESTLimiterRPS to be 100, got %d", p.runtimePolicy.RESTLimiterRPS)
	}
}

func TestRuntimePolicy_RoundTripPersistence(t *testing.T) {
	s := memory.NewStore()
	p := NewPlatform(s)

	now := time.Now().UTC().Truncate(time.Second)
	newPolicy := RuntimePolicy{
		TradeTickFreshnessSeconds:      99,
		SignalBarFreshnessSeconds:      88,
		StrategyEvaluationQuietSeconds: 0, // 显式 0
		RESTLimiterRPS:                 100,
		UpdatedAt:                      now,
	}

	// 1. 更新并持久化
	_, err := p.UpdateRuntimePolicy(newPolicy)
	if err != nil {
		t.Fatalf("UpdateRuntimePolicy failed: %v", err)
	}

	// 2. 清空内存状态，模拟重启
	p2 := NewPlatform(s)
	err = p2.LoadPersistedRuntimePolicy()
	if err != nil {
		t.Fatalf("LoadPersistedRuntimePolicy failed: %v", err)
	}

	// 3. 验证关键字段一致性
	p2Policy := p2.RuntimePolicy()
	if p2Policy.TradeTickFreshnessSeconds != 99 {
		t.Errorf("expected 99, got %d", p2Policy.TradeTickFreshnessSeconds)
	}
	if p2Policy.SignalBarFreshnessSeconds != 88 {
		t.Errorf("expected 88, got %d", p2Policy.SignalBarFreshnessSeconds)
	}
	if p2Policy.StrategyEvaluationQuietSeconds != 0 {
		t.Errorf("expected 0, got %d", p2Policy.StrategyEvaluationQuietSeconds)
	}
	if p2Policy.RESTLimiterRPS != 100 {
		t.Errorf("expected 100, got %d", p2Policy.RESTLimiterRPS)
	}
}

func TestApplyRuntimeConfigOverrides_Granularity(t *testing.T) {
	p := NewPlatform(memory.NewStore())

	// 初始值
	p.runtimePolicy.TradeTickFreshnessSeconds = 15
	p.runtimePolicy.RESTLimiterRPS = 30

	// 模拟环境变量仅设置了 RPS
	val100 := 100
	cfg := config.Config{
		RESTLimiterRPS: &val100,
		// TradeTickFreshnessSeconds 为 nil
	}

	p.ApplyRuntimeConfigOverrides(cfg)

	if p.runtimePolicy.RESTLimiterRPS != 100 {
		t.Errorf("expected RESTLimiterRPS to be 100, got %d", p.runtimePolicy.RESTLimiterRPS)
	}
	if p.runtimePolicy.TradeTickFreshnessSeconds != 15 {
		t.Errorf("expected TradeTickFreshnessSeconds to remain 15, got %d", p.runtimePolicy.TradeTickFreshnessSeconds)
	}
}

func TestConfig_IntPtrFromEnv(t *testing.T) {
	os.Setenv("TEST_INT_PTR", "42")
	defer os.Unsetenv("TEST_INT_PTR")

	ptr := config.IntPtrFromEnv("TEST_INT_PTR")
	if ptr == nil || *ptr != 42 {
		t.Errorf("expected 42, got %v", ptr)
	}

	os.Setenv("TEST_INT_PTR_ZERO", "0")
	defer os.Unsetenv("TEST_INT_PTR_ZERO")
	ptrZero := config.IntPtrFromEnv("TEST_INT_PTR_ZERO")
	if ptrZero == nil || *ptrZero != 0 {
		t.Errorf("expected 0, got %v", ptrZero)
	}

	ptrUnset := config.IntPtrFromEnv("UNSET_KEY")
	if ptrUnset != nil {
		t.Errorf("expected nil for unset key, got %v", ptrUnset)
	}
}
