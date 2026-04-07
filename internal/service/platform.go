// Package service 提供 bkTrader 平台的核心业务逻辑。
//
// Platform 是主要的服务门面（Facade），委托给各个子模块:
//   - strategy.go  — 策略管理、账户管理、信号源、回测
//   - order.go     — 订单执行、持仓管理
//   - paper.go     — 模拟交易引擎、统一策略执行计划回放
//   - pnl.go       — PnL 计算、工具函数
//   - chart.go     — K 线数据、图表标注
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store"
)

// Platform 是平台服务的核心门面，持有存储接口和运行时状态。
// 所有业务方法通过 Platform 对外暴露，内部拆分到各个子文件中。
type Platform struct {
	store           store.Repository              // 存储层接口（内存 / PostgreSQL）
	mu              sync.Mutex                    // 保护 run map 的并发访问
	run             map[string]context.CancelFunc // 运行中的 paper session -> cancel 函数
	signalRun       map[string]context.CancelFunc // 运行中的 signal runtime session -> cancel 函数
	paperPlans      map[string][]paperPlannedOrder
	livePlans       map[string][]paperPlannedOrder
	strategyEngines map[string]StrategyEngine
	liveAdapters    map[string]LiveExecutionAdapter
	signalSources   map[string]SignalSourceProvider
	signalAdapters  map[string]SignalRuntimeAdapter
	signalSessions  map[string]domain.SignalRuntimeSession
	liveMarketMu    sync.RWMutex
	liveMarketData  map[string]liveMarketSnapshot
	manifestMu      sync.Mutex
	once            sync.Once             // 确保 CSV ledger 只加载一次
	ledger          []strategyReplayEvent // 缓存的策略回放账本
	ledgerErr       error                 // 加载账本时的错误
	candleOnce      sync.Once
	candles         []candleBar
	candleErr       error
	tickInterval    int // 模拟盘 Ticker 间隔（秒）
	minuteDataDir   string
	tickDataDir     string
	tickManifest    []tradeArchiveManifestEntry
	runtimePolicy   RuntimePolicy
	telegramConfig  domain.TelegramConfig
}

type RuntimePolicy struct {
	TradeTickFreshnessSeconds      int `json:"tradeTickFreshnessSeconds"`
	OrderBookFreshnessSeconds      int `json:"orderBookFreshnessSeconds"`
	SignalBarFreshnessSeconds      int `json:"signalBarFreshnessSeconds"`
	RuntimeQuietSeconds            int `json:"runtimeQuietSeconds"`
	PaperStartReadinessTimeoutSecs int `json:"paperStartReadinessTimeoutSeconds"`
}

// NewPlatform 创建并初始化平台服务实例。
func NewPlatform(store store.Repository) *Platform {
	platform := &Platform{
		store:           store,
		run:             make(map[string]context.CancelFunc),
		signalRun:       make(map[string]context.CancelFunc),
		paperPlans:      make(map[string][]paperPlannedOrder),
		livePlans:       make(map[string][]paperPlannedOrder),
		strategyEngines: make(map[string]StrategyEngine),
		liveAdapters:    make(map[string]LiveExecutionAdapter),
		signalSources:   make(map[string]SignalSourceProvider),
		signalAdapters:  make(map[string]SignalRuntimeAdapter),
		signalSessions:  make(map[string]domain.SignalRuntimeSession),
		liveMarketData:  make(map[string]liveMarketSnapshot),
		runtimePolicy: RuntimePolicy{
			TradeTickFreshnessSeconds:      15,
			OrderBookFreshnessSeconds:      10,
			SignalBarFreshnessSeconds:      30,
			RuntimeQuietSeconds:            30,
			PaperStartReadinessTimeoutSecs: 5,
		},
	}
	platform.registerBuiltInStrategyEngines()
	platform.registerBuiltInLiveAdapters()
	platform.registerBuiltInSignalSources()
	platform.registerBuiltInSignalRuntimeAdapters()
	return platform
}

func (p *Platform) SetBacktestDataDirs(minuteDataDir, tickDataDir string) {
	p.minuteDataDir = minuteDataDir
	p.tickDataDir = tickDataDir
	p.manifestMu.Lock()
	p.tickManifest = nil
	p.manifestMu.Unlock()
}

func (p *Platform) SetRuntimePolicy(policy RuntimePolicy) {
	if policy.TradeTickFreshnessSeconds > 0 {
		p.runtimePolicy.TradeTickFreshnessSeconds = policy.TradeTickFreshnessSeconds
	}
	if policy.OrderBookFreshnessSeconds > 0 {
		p.runtimePolicy.OrderBookFreshnessSeconds = policy.OrderBookFreshnessSeconds
	}
	if policy.SignalBarFreshnessSeconds > 0 {
		p.runtimePolicy.SignalBarFreshnessSeconds = policy.SignalBarFreshnessSeconds
	}
	if policy.RuntimeQuietSeconds > 0 {
		p.runtimePolicy.RuntimeQuietSeconds = policy.RuntimeQuietSeconds
	}
	if policy.PaperStartReadinessTimeoutSecs > 0 {
		p.runtimePolicy.PaperStartReadinessTimeoutSecs = policy.PaperStartReadinessTimeoutSecs
	}
}

func (p *Platform) RuntimePolicy() RuntimePolicy {
	return p.runtimePolicy
}

func (p *Platform) UpdateRuntimePolicy(policy RuntimePolicy) (RuntimePolicy, error) {
	if policy.TradeTickFreshnessSeconds < 0 ||
		policy.OrderBookFreshnessSeconds < 0 ||
		policy.SignalBarFreshnessSeconds < 0 ||
		policy.RuntimeQuietSeconds < 0 ||
		policy.PaperStartReadinessTimeoutSecs < 0 {
		return p.runtimePolicy, fmt.Errorf("runtime policy values must be non-negative")
	}
	p.SetRuntimePolicy(policy)
	saved, err := p.store.UpsertRuntimePolicy(domain.RuntimePolicy{
		TradeTickFreshnessSeconds:      p.runtimePolicy.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      p.runtimePolicy.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      p.runtimePolicy.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            p.runtimePolicy.RuntimeQuietSeconds,
		PaperStartReadinessTimeoutSecs: p.runtimePolicy.PaperStartReadinessTimeoutSecs,
	})
	if err != nil {
		return p.runtimePolicy, err
	}
	p.SetRuntimePolicy(RuntimePolicy{
		TradeTickFreshnessSeconds:      saved.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      saved.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      saved.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            saved.RuntimeQuietSeconds,
		PaperStartReadinessTimeoutSecs: saved.PaperStartReadinessTimeoutSecs,
	})
	return p.runtimePolicy, nil
}

func (p *Platform) LoadPersistedRuntimePolicy() error {
	policy, ok, err := p.store.GetRuntimePolicy()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	p.SetRuntimePolicy(RuntimePolicy{
		TradeTickFreshnessSeconds:      policy.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      policy.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      policy.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            policy.RuntimeQuietSeconds,
		PaperStartReadinessTimeoutSecs: policy.PaperStartReadinessTimeoutSecs,
	})
	return nil
}

func (p *Platform) SetTelegramConfig(config domain.TelegramConfig) {
	p.telegramConfig.Enabled = config.Enabled
	if strings.TrimSpace(config.BotToken) != "" {
		p.telegramConfig.BotToken = strings.TrimSpace(config.BotToken)
	}
	if strings.TrimSpace(config.ChatID) != "" {
		p.telegramConfig.ChatID = strings.TrimSpace(config.ChatID)
	}
	if len(config.SendLevels) > 0 {
		p.telegramConfig.SendLevels = normalizeTelegramSendLevels(config.SendLevels)
	}
	if !config.UpdatedAt.IsZero() {
		p.telegramConfig.UpdatedAt = config.UpdatedAt
	}
}

func (p *Platform) TelegramConfigView() map[string]any {
	token := strings.TrimSpace(p.telegramConfig.BotToken)
	return map[string]any{
		"enabled":        p.telegramConfig.Enabled,
		"chatId":         p.telegramConfig.ChatID,
		"sendLevels":     append([]string(nil), p.telegramConfig.SendLevels...),
		"hasBotToken":    token != "",
		"maskedBotToken": maskTelegramToken(token),
		"updatedAt":      p.telegramConfig.UpdatedAt,
	}
}

func (p *Platform) UpdateTelegramConfig(enabled bool, botToken, chatID string, sendLevels []string) (map[string]any, error) {
	config := p.telegramConfig
	config.Enabled = enabled
	if strings.TrimSpace(botToken) != "" {
		config.BotToken = strings.TrimSpace(botToken)
	}
	if strings.TrimSpace(chatID) != "" {
		config.ChatID = strings.TrimSpace(chatID)
	}
	if len(sendLevels) > 0 {
		config.SendLevels = normalizeTelegramSendLevels(sendLevels)
	}
	saved, err := p.store.UpsertTelegramConfig(config)
	if err != nil {
		return nil, err
	}
	p.telegramConfig = saved
	return p.TelegramConfigView(), nil
}

func (p *Platform) LoadPersistedTelegramConfig() error {
	config, ok, err := p.store.GetTelegramConfig()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	p.telegramConfig = config
	return nil
}

func normalizeTelegramSendLevels(levels []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(levels))
	for _, level := range levels {
		normalized := strings.ToLower(strings.TrimSpace(level))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return []string{"critical", "warning"}
	}
	return out
}

func maskTelegramToken(token string) string {
	if len(token) <= 8 {
		return ""
	}
	return token[:4] + "..." + token[len(token)-4:]
}
