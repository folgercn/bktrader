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
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/logging"
	"github.com/wuyaocheng/bktrader/internal/store"
)

// Platform 是平台服务的核心门面，持有存储接口和运行时状态。
// 所有业务方法通过 Platform 对外暴露，内部拆分到各个子文件中。
type Platform struct {
	store                  store.Repository              // 存储层接口（内存 / PostgreSQL）
	mu                     sync.Mutex                    // 保护 run map 的并发访问
	run                    map[string]context.CancelFunc // 运行中的 paper session -> cancel 函数
	signalRun              map[string]context.CancelFunc // 运行中的 signal runtime session -> cancel 函数
	paperPlans             map[string][]paperPlannedOrder
	livePlans              map[string][]paperPlannedOrder
	strategyEngines        map[string]StrategyEngine
	liveAdapters           map[string]LiveExecutionAdapter
	signalSources          map[string]SignalSourceProvider
	signalAdapters         map[string]SignalRuntimeAdapter
	executionStrategies    map[string]ExecutionStrategy
	signalSessions         map[string]domain.SignalRuntimeSession
	liveMarketMu           sync.RWMutex
	liveMarketData         map[string]liveMarketSnapshot
	liveAccountOpMu        sync.Map // accountID -> *sync.Mutex
	manifestMu             sync.Mutex
	once                   sync.Once             // 确保 CSV ledger 只加载一次
	ledger                 []strategyReplayEvent // 缓存的策略回放账本
	ledgerErr              error                 // 加载账本时的错误
	candleOnce             sync.Once
	candles                []candleBar
	candleErr              error
	tickInterval           int // 模拟盘 Ticker 间隔（秒）
	minuteDataDir          string
	tickDataDir            string
	tickManifest           []tradeArchiveManifestEntry
	runtimePolicy          RuntimePolicy
	telegramConfig         domain.TelegramConfig
	telegramSentAlertCache sync.Map // notificationID -> alertTitle
	logBroker              *logging.Broker
}

type RuntimePolicy struct {
	TradeTickFreshnessSeconds      int       `json:"tradeTickFreshnessSeconds"`
	OrderBookFreshnessSeconds      int       `json:"orderBookFreshnessSeconds"`
	SignalBarFreshnessSeconds      int       `json:"signalBarFreshnessSeconds"`
	RuntimeQuietSeconds            int       `json:"runtimeQuietSeconds"`
	StrategyEvaluationQuietSeconds int       `json:"strategyEvaluationQuietSeconds"`
	LiveAccountSyncFreshnessSecs   int       `json:"liveAccountSyncFreshnessSeconds"`
	PaperStartReadinessTimeoutSecs int       `json:"paperStartReadinessTimeoutSeconds"`
	UpdatedAt                      time.Time `json:"updatedAt"`
}

// NewPlatform 创建并初始化平台服务实例。
func NewPlatform(store store.Repository) *Platform {
	platform := &Platform{
		store:               store,
		run:                 make(map[string]context.CancelFunc),
		signalRun:           make(map[string]context.CancelFunc),
		paperPlans:          make(map[string][]paperPlannedOrder),
		livePlans:           make(map[string][]paperPlannedOrder),
		strategyEngines:     make(map[string]StrategyEngine),
		liveAdapters:        make(map[string]LiveExecutionAdapter),
		signalSources:       make(map[string]SignalSourceProvider),
		signalAdapters:      make(map[string]SignalRuntimeAdapter),
		executionStrategies: make(map[string]ExecutionStrategy),
		signalSessions:      make(map[string]domain.SignalRuntimeSession),
		liveMarketData:      make(map[string]liveMarketSnapshot),
		logBroker:           logging.NewBroker(),
		runtimePolicy: RuntimePolicy{
			TradeTickFreshnessSeconds:      15,
			OrderBookFreshnessSeconds:      10,
			SignalBarFreshnessSeconds:      30,
			RuntimeQuietSeconds:            30,
			StrategyEvaluationQuietSeconds: 15,
			LiveAccountSyncFreshnessSecs:   60,
			PaperStartReadinessTimeoutSecs: 5,
			UpdatedAt:                      time.Now().UTC(),
		},
	}
	platform.registerBuiltInStrategyEngines()
	platform.registerBuiltInLiveAdapters()
	platform.registerBuiltInSignalSources()
	platform.registerBuiltInSignalRuntimeAdapters()
	platform.registerBuiltInExecutionStrategies()
	platform.logger("service.platform").Info("platform initialized",
		"strategy_engine_count", len(platform.strategyEngines),
		"live_adapter_count", len(platform.liveAdapters),
		"signal_source_count", len(platform.signalSources),
		"signal_adapter_count", len(platform.signalAdapters),
		"execution_strategy_count", len(platform.executionStrategies),
	)
	return platform
}

func (p *Platform) SetBacktestDataDirs(minuteDataDir, tickDataDir string) {
	p.minuteDataDir = minuteDataDir
	p.tickDataDir = tickDataDir
	p.manifestMu.Lock()
	p.tickManifest = nil
	p.manifestMu.Unlock()
	p.logger("service.platform").Info("backtest data directories configured",
		"minute_data_dir", minuteDataDir,
		"tick_data_dir", tickDataDir,
	)
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
	if policy.StrategyEvaluationQuietSeconds >= 0 {
		p.runtimePolicy.StrategyEvaluationQuietSeconds = policy.StrategyEvaluationQuietSeconds
	}
	if policy.LiveAccountSyncFreshnessSecs >= 0 {
		p.runtimePolicy.LiveAccountSyncFreshnessSecs = policy.LiveAccountSyncFreshnessSecs
	}
	if policy.PaperStartReadinessTimeoutSecs > 0 {
		p.runtimePolicy.PaperStartReadinessTimeoutSecs = policy.PaperStartReadinessTimeoutSecs
	}
	if !policy.UpdatedAt.IsZero() {
		p.runtimePolicy.UpdatedAt = policy.UpdatedAt
	}
	p.logger("service.platform").Info("runtime policy applied",
		"trade_tick_freshness_seconds", p.runtimePolicy.TradeTickFreshnessSeconds,
		"order_book_freshness_seconds", p.runtimePolicy.OrderBookFreshnessSeconds,
		"signal_bar_freshness_seconds", p.runtimePolicy.SignalBarFreshnessSeconds,
		"runtime_quiet_seconds", p.runtimePolicy.RuntimeQuietSeconds,
		"paper_start_readiness_timeout_seconds", p.runtimePolicy.PaperStartReadinessTimeoutSecs,
		"updated_at", p.runtimePolicy.UpdatedAt,
	)
}

func (p *Platform) RuntimePolicy() RuntimePolicy {
	return p.runtimePolicy
}

func (p *Platform) UpdateRuntimePolicy(policy RuntimePolicy) (RuntimePolicy, error) {
	if policy.TradeTickFreshnessSeconds < 0 ||
		policy.OrderBookFreshnessSeconds < 0 ||
		policy.SignalBarFreshnessSeconds < 0 ||
		policy.RuntimeQuietSeconds < 0 ||
		policy.StrategyEvaluationQuietSeconds < 0 ||
		policy.LiveAccountSyncFreshnessSecs < 0 ||
		policy.PaperStartReadinessTimeoutSecs < 0 {
		p.logger("service.platform").Warn("reject invalid runtime policy",
			"trade_tick_freshness_seconds", policy.TradeTickFreshnessSeconds,
			"order_book_freshness_seconds", policy.OrderBookFreshnessSeconds,
			"signal_bar_freshness_seconds", policy.SignalBarFreshnessSeconds,
			"runtime_quiet_seconds", policy.RuntimeQuietSeconds,
			"paper_start_readiness_timeout_seconds", policy.PaperStartReadinessTimeoutSecs,
		)
		return p.runtimePolicy, fmt.Errorf("runtime policy values must be non-negative")
	}
	p.SetRuntimePolicy(policy)
	saved, err := p.store.UpsertRuntimePolicy(domain.RuntimePolicy{
		TradeTickFreshnessSeconds:      p.runtimePolicy.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      p.runtimePolicy.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      p.runtimePolicy.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            p.runtimePolicy.RuntimeQuietSeconds,
		StrategyEvaluationQuietSeconds: p.runtimePolicy.StrategyEvaluationQuietSeconds,
		LiveAccountSyncFreshnessSecs:   p.runtimePolicy.LiveAccountSyncFreshnessSecs,
		PaperStartReadinessTimeoutSecs: p.runtimePolicy.PaperStartReadinessTimeoutSecs,
	})
	if err != nil {
		p.logger("service.platform").Error("persist runtime policy failed", "error", err)
		return p.runtimePolicy, err
	}
	p.SetRuntimePolicy(RuntimePolicy{
		TradeTickFreshnessSeconds:      saved.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      saved.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      saved.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            saved.RuntimeQuietSeconds,
		StrategyEvaluationQuietSeconds: saved.StrategyEvaluationQuietSeconds,
		LiveAccountSyncFreshnessSecs:   saved.LiveAccountSyncFreshnessSecs,
		PaperStartReadinessTimeoutSecs: saved.PaperStartReadinessTimeoutSecs,
		UpdatedAt:                      saved.UpdatedAt,
	})
	p.logger("service.platform").Info("runtime policy updated")
	return p.runtimePolicy, nil
}

func (p *Platform) LoadPersistedRuntimePolicy() error {
	policy, ok, err := p.store.GetRuntimePolicy()
	if err != nil {
		p.logger("service.platform").Error("load persisted runtime policy failed", "error", err)
		return err
	}
	if !ok {
		p.logger("service.platform").Debug("no persisted runtime policy found")
		return nil
	}
	p.SetRuntimePolicy(RuntimePolicy{
		TradeTickFreshnessSeconds:      policy.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      policy.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      policy.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            policy.RuntimeQuietSeconds,
		StrategyEvaluationQuietSeconds: policy.StrategyEvaluationQuietSeconds,
		LiveAccountSyncFreshnessSecs:   policy.LiveAccountSyncFreshnessSecs,
		PaperStartReadinessTimeoutSecs: policy.PaperStartReadinessTimeoutSecs,
		UpdatedAt:                      policy.UpdatedAt,
	})
	p.logger("service.platform").Info("persisted runtime policy loaded")
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
	p.logger("service.platform").Info("telegram config applied",
		"enabled", p.telegramConfig.Enabled,
		"send_levels", p.telegramConfig.SendLevels,
		"has_bot_token", strings.TrimSpace(p.telegramConfig.BotToken) != "",
		"has_chat_id", strings.TrimSpace(p.telegramConfig.ChatID) != "",
	)
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
		p.logger("service.platform").Error("persist telegram config failed", "error", err)
		return nil, err
	}
	p.telegramConfig = saved
	p.logger("service.platform").Info("telegram config updated",
		"enabled", saved.Enabled,
		"send_levels", saved.SendLevels,
		"has_bot_token", strings.TrimSpace(saved.BotToken) != "",
		"has_chat_id", strings.TrimSpace(saved.ChatID) != "",
	)
	return p.TelegramConfigView(), nil
}

func (p *Platform) LoadPersistedTelegramConfig() error {
	config, ok, err := p.store.GetTelegramConfig()
	if err != nil {
		p.logger("service.platform").Error("load persisted telegram config failed", "error", err)
		return err
	}
	if !ok {
		p.logger("service.platform").Debug("no persisted telegram config found")
		return nil
	}
	p.telegramConfig = config
	p.logger("service.platform").Info("persisted telegram config loaded",
		"enabled", config.Enabled,
		"send_levels", config.SendLevels,
		"has_bot_token", strings.TrimSpace(config.BotToken) != "",
		"has_chat_id", strings.TrimSpace(config.ChatID) != "",
	)
	return nil
}

func (p *Platform) logger(component string, args ...any) *slog.Logger {
	logger := slog.Default()
	if strings.TrimSpace(component) != "" {
		logger = logger.With("component", component)
	}
	if len(args) > 0 {
		logger = logger.With(args...)
	}
	return logger
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
