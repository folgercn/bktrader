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

	"github.com/wuyaocheng/bktrader/internal/config"
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
	signalRun              map[string]*signalRuntimeRun  // 运行中的 signal runtime session -> run handle
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
	liveAccountSyncState   sync.Map // accountID -> *liveAccountSyncState
	runtimeSourceGateState sync.Map // runtimeSessionID -> last blocked source gate signature
	runtimeEventPublisher  RuntimeEventPublisher
	runtimeEventConsumerOn bool
	runtimeEventThrottle   sync.Map // runtimeSessionID|symbol|streamType -> *runtimeEventPublishThrottleState
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
	tickEvalThrottle       sync.Map // runtimeSessionID or runtimeSessionID|symbol -> *tickEvalThrottleState
	logBroker              *logging.Broker
	dashboardBroker        *DashboardBroker
}

type RuntimePolicy struct {
	TradeTickFreshnessSeconds      int       `json:"tradeTickFreshnessSeconds"`
	OrderBookFreshnessSeconds      int       `json:"orderBookFreshnessSeconds"`
	SignalBarFreshnessSeconds      int       `json:"signalBarFreshnessSeconds"`
	RuntimeQuietSeconds            int       `json:"runtimeQuietSeconds"`
	StrategyEvaluationQuietSeconds int       `json:"strategyEvaluationQuietSeconds"`
	LiveAccountSyncFreshnessSecs   int       `json:"liveAccountSyncFreshnessSeconds"`
	PaperStartReadinessTimeoutSecs int       `json:"paperStartReadinessTimeoutSeconds"`
	WSHandshakeTimeoutSeconds      int       `json:"wsHandshakeTimeoutSeconds"`
	WSReadStaleTimeoutSeconds      int       `json:"wsReadStaleTimeoutSeconds"`
	WSPingIntervalSeconds          int       `json:"wsPingIntervalSeconds"`
	WSPassiveCloseTimeoutSeconds   int       `json:"wsPassiveCloseTimeoutSeconds"`
	WSReconnectBackoffs            []int     `json:"wsReconnectBackoffs"`
	WSReconnectRecoveryBackoffs    []int     `json:"wsReconnectRecoveryBackoffs"`
	RESTLimiterRPS                 int       `json:"restLimiterRps"`
	RESTLimiterBurst               int       `json:"restLimiterBurst"`
	RESTBackoffSeconds             int       `json:"restBackoffSeconds"`
	LiveMarketCacheTTLMinutes      int       `json:"liveMarketCacheTTLMinutes"`
	TelegramHTTPTimeoutSeconds     int       `json:"telegramHTTPTimeoutSeconds"`
	BinanceRecvWindowMs            int       `json:"binanceRecvWindowMs"`
	LiveSignalWarmWindowDays       int       `json:"liveSignalWarmWindowDays"`
	LiveFastSignalWarmWindowDays   int       `json:"liveFastSignalWarmWindowDays"`
	LiveMinuteWarmWindowDays       int       `json:"liveMinuteWarmWindowDays"`
	UpdatedAt                      time.Time `json:"updatedAt"`
}

// NewPlatform 创建并初始化平台服务实例。
func NewPlatform(store store.Repository) *Platform {
	platform := &Platform{
		store:                 store,
		run:                   make(map[string]context.CancelFunc),
		signalRun:             make(map[string]*signalRuntimeRun),
		paperPlans:            make(map[string][]paperPlannedOrder),
		livePlans:             make(map[string][]paperPlannedOrder),
		strategyEngines:       make(map[string]StrategyEngine),
		liveAdapters:          make(map[string]LiveExecutionAdapter),
		signalSources:         make(map[string]SignalSourceProvider),
		signalAdapters:        make(map[string]SignalRuntimeAdapter),
		executionStrategies:   make(map[string]ExecutionStrategy),
		signalSessions:        make(map[string]domain.SignalRuntimeSession),
		runtimeEventPublisher: NoopRuntimeEventPublisher{},
		liveMarketData:        make(map[string]liveMarketSnapshot),
		logBroker:             logging.NewBroker(),
		telegramConfig: domain.TelegramConfig{
			SendLevels:                    []string{"critical", "warning"},
			TradeEventsEnabled:            true,
			PositionReportEnabled:         true,
			PositionReportIntervalMinutes: 30,
			UpdatedAt:                     time.Now().UTC(),
		},
		runtimePolicy: RuntimePolicy{
			TradeTickFreshnessSeconds:      15,
			OrderBookFreshnessSeconds:      10,
			SignalBarFreshnessSeconds:      30,
			RuntimeQuietSeconds:            30,
			StrategyEvaluationQuietSeconds: 15,
			LiveAccountSyncFreshnessSecs:   60,
			PaperStartReadinessTimeoutSecs: 5,
			WSHandshakeTimeoutSeconds:      10,
			WSReadStaleTimeoutSeconds:      60,
			WSPingIntervalSeconds:          20,
			WSPassiveCloseTimeoutSeconds:   2,
			WSReconnectBackoffs:            []int{10, 30, 60},
			WSReconnectRecoveryBackoffs:    []int{30, 120},
			RESTLimiterRPS:                 30,
			RESTLimiterBurst:               50,
			RESTBackoffSeconds:             60,
			LiveMarketCacheTTLMinutes:      10,
			TelegramHTTPTimeoutSeconds:     8,
			BinanceRecvWindowMs:            5000,
			LiveSignalWarmWindowDays:       400,
			LiveFastSignalWarmWindowDays:   7,
			LiveMinuteWarmWindowDays:       30,
			UpdatedAt:                      time.Now().UTC(),
		},
	}
	platform.registerBuiltInStrategyEngines()
	platform.registerBuiltInLiveAdapters()
	platform.registerBuiltInSignalSources()
	platform.registerBuiltInSignalRuntimeAdapters()
	platform.registerBuiltInExecutionStrategies()
	platform.dashboardBroker = NewDashboardBroker(platform)
	platform.logger("service.platform").Info("platform initialized",
		"strategy_engine_count", len(platform.strategyEngines),
		"live_adapter_count", len(platform.liveAdapters),
		"signal_source_count", len(platform.signalSources),
		"signal_adapter_count", len(platform.signalAdapters),
		"execution_strategy_count", len(platform.executionStrategies),
	)
	return platform
}

// StartDashboardBroker 启动仪表盘实时数据轮询检测
func (p *Platform) StartDashboardBroker(ctx context.Context, cfg config.Config) {
	p.logger("service.platform").Info("starting dashboard broker polling")
	go p.dashboardBroker.StartPolling(ctx, cfg)
}

// DashboardBroker 返回内部仪表盘事件分发器
func (p *Platform) DashboardBroker() *DashboardBroker {
	return p.dashboardBroker
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
	if policy.WSHandshakeTimeoutSeconds > 0 {
		p.runtimePolicy.WSHandshakeTimeoutSeconds = policy.WSHandshakeTimeoutSeconds
	}
	if policy.WSReadStaleTimeoutSeconds > 0 {
		p.runtimePolicy.WSReadStaleTimeoutSeconds = policy.WSReadStaleTimeoutSeconds
	}
	if policy.WSPingIntervalSeconds > 0 {
		p.runtimePolicy.WSPingIntervalSeconds = policy.WSPingIntervalSeconds
	}
	if policy.WSPassiveCloseTimeoutSeconds > 0 {
		p.runtimePolicy.WSPassiveCloseTimeoutSeconds = policy.WSPassiveCloseTimeoutSeconds
	}
	if len(policy.WSReconnectBackoffs) > 0 {
		p.runtimePolicy.WSReconnectBackoffs = append([]int(nil), policy.WSReconnectBackoffs...)
	}
	if len(policy.WSReconnectRecoveryBackoffs) > 0 {
		p.runtimePolicy.WSReconnectRecoveryBackoffs = append([]int(nil), policy.WSReconnectRecoveryBackoffs...)
	}
	if policy.RESTLimiterRPS > 0 {
		p.runtimePolicy.RESTLimiterRPS = policy.RESTLimiterRPS
	}
	if policy.RESTLimiterBurst > 0 {
		p.runtimePolicy.RESTLimiterBurst = policy.RESTLimiterBurst
	}
	if policy.RESTBackoffSeconds > 0 {
		p.runtimePolicy.RESTBackoffSeconds = policy.RESTBackoffSeconds
	}
	if policy.LiveMarketCacheTTLMinutes > 0 {
		p.runtimePolicy.LiveMarketCacheTTLMinutes = policy.LiveMarketCacheTTLMinutes
	}
	if policy.TelegramHTTPTimeoutSeconds > 0 {
		p.runtimePolicy.TelegramHTTPTimeoutSeconds = policy.TelegramHTTPTimeoutSeconds
	}
	if policy.BinanceRecvWindowMs > 0 {
		p.runtimePolicy.BinanceRecvWindowMs = policy.BinanceRecvWindowMs
	}
	if policy.LiveSignalWarmWindowDays > 0 {
		p.runtimePolicy.LiveSignalWarmWindowDays = policy.LiveSignalWarmWindowDays
	}
	if policy.LiveFastSignalWarmWindowDays > 0 {
		p.runtimePolicy.LiveFastSignalWarmWindowDays = policy.LiveFastSignalWarmWindowDays
	}
	if policy.LiveMinuteWarmWindowDays > 0 {
		p.runtimePolicy.LiveMinuteWarmWindowDays = policy.LiveMinuteWarmWindowDays
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
		"ws_stale_timeout_seconds", p.runtimePolicy.WSReadStaleTimeoutSeconds,
		"updated_at", p.runtimePolicy.UpdatedAt,
	)
}

func (p *Platform) ApplyRuntimeConfigOverrides(cfg config.Config) {
	restLimitsChanged := false

	// 辅助函数：仅当 source 非 nil 且 > 0 时应用
	applyIfPositive := func(target *int, source *int, name string, isRest bool) {
		if source != nil {
			if *source > 0 {
				if *target != *source && isRest {
					restLimitsChanged = true
				}
				*target = *source
			} else {
				p.logger("service.platform").Warn("invalid runtime config (must > 0), ignoring", "key", name, "value", *source)
			}
		}
	}

	// 辅助函数：仅当 source 非 nil 且 >= 0 时应用
	applyIfNonNegative := func(target *int, source *int, name string) {
		if source != nil {
			if *source >= 0 {
				*target = *source
			} else {
				p.logger("service.platform").Warn("invalid runtime config (must >= 0), ignoring", "key", name, "value", *source)
			}
		}
	}

	applyIfPositive(&p.runtimePolicy.TradeTickFreshnessSeconds, cfg.TradeTickFreshnessSeconds, "TRADE_TICK_FRESHNESS_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.OrderBookFreshnessSeconds, cfg.OrderBookFreshnessSeconds, "ORDER_BOOK_FRESHNESS_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.SignalBarFreshnessSeconds, cfg.SignalBarFreshnessSeconds, "SIGNAL_BAR_FRESHNESS_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.RuntimeQuietSeconds, cfg.RuntimeQuietSeconds, "RUNTIME_QUIET_SECONDS", false)

	// 允许 0 的特殊字段
	applyIfNonNegative(&p.runtimePolicy.StrategyEvaluationQuietSeconds, cfg.StrategyEvaluationQuietSeconds, "STRATEGY_EVALUATION_QUIET_SECONDS")
	applyIfNonNegative(&p.runtimePolicy.LiveAccountSyncFreshnessSecs, cfg.LiveAccountSyncFreshnessSecs, "LIVE_ACCOUNT_SYNC_FRESHNESS_SECONDS")

	applyIfPositive(&p.runtimePolicy.PaperStartReadinessTimeoutSecs, cfg.PaperStartReadinessTimeoutSecs, "PAPER_START_READINESS_TIMEOUT_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.WSHandshakeTimeoutSeconds, cfg.WSHandshakeTimeoutSeconds, "WS_HANDSHAKE_TIMEOUT_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.WSReadStaleTimeoutSeconds, cfg.WSReadStaleTimeoutSeconds, "WS_READ_STALE_TIMEOUT_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.WSPingIntervalSeconds, cfg.WSPingIntervalSeconds, "WS_PING_INTERVAL_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.WSPassiveCloseTimeoutSeconds, cfg.WSPassiveCloseTimeoutSeconds, "WS_PASSIVE_CLOSE_TIMEOUT_SECONDS", false)

	if len(cfg.WSReconnectBackoffs) > 0 {
		p.runtimePolicy.WSReconnectBackoffs = append([]int(nil), cfg.WSReconnectBackoffs...)
	}
	if len(cfg.WSReconnectRecoveryBackoffs) > 0 {
		p.runtimePolicy.WSReconnectRecoveryBackoffs = append([]int(nil), cfg.WSReconnectRecoveryBackoffs...)
	}

	applyIfPositive(&p.runtimePolicy.RESTLimiterRPS, cfg.RESTLimiterRPS, "REST_LIMITER_RPS", true)
	applyIfPositive(&p.runtimePolicy.RESTLimiterBurst, cfg.RESTLimiterBurst, "REST_LIMITER_BURST", true)
	applyIfPositive(&p.runtimePolicy.RESTBackoffSeconds, cfg.RESTBackoffSeconds, "REST_BACKOFF_SECONDS", true)

	applyIfPositive(&p.runtimePolicy.LiveMarketCacheTTLMinutes, cfg.LiveMarketCacheTTLMinutes, "LIVE_MARKET_CACHE_TTL_MINUTES", false)
	applyIfPositive(&p.runtimePolicy.TelegramHTTPTimeoutSeconds, cfg.TelegramHTTPTimeoutSeconds, "TELEGRAM_HTTP_TIMEOUT_SECONDS", false)
	applyIfPositive(&p.runtimePolicy.BinanceRecvWindowMs, cfg.BinanceRecvWindowMs, "BINANCE_REST_RECV_WINDOW_MS", false)
	applyIfPositive(&p.runtimePolicy.LiveSignalWarmWindowDays, cfg.LiveSignalWarmWindowDays, "LIVE_SIGNAL_WARM_WINDOW_DAYS", false)
	applyIfPositive(&p.runtimePolicy.LiveFastSignalWarmWindowDays, cfg.LiveFastSignalWarmWindowDays, "LIVE_FAST_SIGNAL_WARM_WINDOW_DAYS", false)
	applyIfPositive(&p.runtimePolicy.LiveMinuteWarmWindowDays, cfg.LiveMinuteWarmWindowDays, "LIVE_MINUTE_WARM_WINDOW_DAYS", false)

	// 最终同步更新 limiter (仅当 REST 相关参数确实变化时)
	if restLimitsChanged {
		p.UpdateBinanceRESTLimits()
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
		WSHandshakeTimeoutSeconds:      p.runtimePolicy.WSHandshakeTimeoutSeconds,
		WSReadStaleTimeoutSeconds:      p.runtimePolicy.WSReadStaleTimeoutSeconds,
		WSPingIntervalSeconds:          p.runtimePolicy.WSPingIntervalSeconds,
		WSPassiveCloseTimeoutSeconds:   p.runtimePolicy.WSPassiveCloseTimeoutSeconds,
		WSReconnectBackoffs:            p.runtimePolicy.WSReconnectBackoffs,
		WSReconnectRecoveryBackoffs:    p.runtimePolicy.WSReconnectRecoveryBackoffs,
		RESTLimiterRPS:                 p.runtimePolicy.RESTLimiterRPS,
		RESTLimiterBurst:               p.runtimePolicy.RESTLimiterBurst,
		RESTBackoffSeconds:             p.runtimePolicy.RESTBackoffSeconds,
		LiveMarketCacheTTLMinutes:      p.runtimePolicy.LiveMarketCacheTTLMinutes,
		TelegramHTTPTimeoutSeconds:     p.runtimePolicy.TelegramHTTPTimeoutSeconds,
		BinanceRecvWindowMs:            p.runtimePolicy.BinanceRecvWindowMs,
		LiveSignalWarmWindowDays:       p.runtimePolicy.LiveSignalWarmWindowDays,
		LiveFastSignalWarmWindowDays:   p.runtimePolicy.LiveFastSignalWarmWindowDays,
		LiveMinuteWarmWindowDays:       p.runtimePolicy.LiveMinuteWarmWindowDays,
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
		WSHandshakeTimeoutSeconds:      saved.WSHandshakeTimeoutSeconds,
		WSReadStaleTimeoutSeconds:      saved.WSReadStaleTimeoutSeconds,
		WSPingIntervalSeconds:          saved.WSPingIntervalSeconds,
		WSPassiveCloseTimeoutSeconds:   saved.WSPassiveCloseTimeoutSeconds,
		WSReconnectBackoffs:            saved.WSReconnectBackoffs,
		WSReconnectRecoveryBackoffs:    saved.WSReconnectRecoveryBackoffs,
		RESTLimiterRPS:                 saved.RESTLimiterRPS,
		RESTLimiterBurst:               saved.RESTLimiterBurst,
		RESTBackoffSeconds:             saved.RESTBackoffSeconds,
		LiveMarketCacheTTLMinutes:      saved.LiveMarketCacheTTLMinutes,
		TelegramHTTPTimeoutSeconds:     saved.TelegramHTTPTimeoutSeconds,
		BinanceRecvWindowMs:            saved.BinanceRecvWindowMs,
		LiveSignalWarmWindowDays:       saved.LiveSignalWarmWindowDays,
		LiveFastSignalWarmWindowDays:   saved.LiveFastSignalWarmWindowDays,
		LiveMinuteWarmWindowDays:       saved.LiveMinuteWarmWindowDays,
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
		WSHandshakeTimeoutSeconds:      policy.WSHandshakeTimeoutSeconds,
		WSReadStaleTimeoutSeconds:      policy.WSReadStaleTimeoutSeconds,
		WSPingIntervalSeconds:          policy.WSPingIntervalSeconds,
		WSPassiveCloseTimeoutSeconds:   policy.WSPassiveCloseTimeoutSeconds,
		WSReconnectBackoffs:            policy.WSReconnectBackoffs,
		WSReconnectRecoveryBackoffs:    policy.WSReconnectRecoveryBackoffs,
		RESTLimiterRPS:                 policy.RESTLimiterRPS,
		RESTLimiterBurst:               policy.RESTLimiterBurst,
		RESTBackoffSeconds:             policy.RESTBackoffSeconds,
		LiveMarketCacheTTLMinutes:      policy.LiveMarketCacheTTLMinutes,
		TelegramHTTPTimeoutSeconds:     policy.TelegramHTTPTimeoutSeconds,
		BinanceRecvWindowMs:            policy.BinanceRecvWindowMs,
		LiveSignalWarmWindowDays:       policy.LiveSignalWarmWindowDays,
		LiveFastSignalWarmWindowDays:   policy.LiveFastSignalWarmWindowDays,
		LiveMinuteWarmWindowDays:       policy.LiveMinuteWarmWindowDays,
		UpdatedAt:                      policy.UpdatedAt,
	})
	p.logger("service.platform").Info("persisted runtime policy loaded")
	return nil
}

func (p *Platform) SetTelegramConfig(config domain.TelegramConfig) {
	p.telegramConfig.Enabled = config.Enabled
	p.telegramConfig.TradeEventsEnabled = config.TradeEventsEnabled
	p.telegramConfig.PositionReportEnabled = config.PositionReportEnabled
	p.telegramConfig.PositionReportIntervalMinutes = normalizeTelegramPositionReportInterval(config.PositionReportIntervalMinutes)
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
		"enabled":                       p.telegramConfig.Enabled,
		"chatId":                        p.telegramConfig.ChatID,
		"sendLevels":                    append([]string(nil), p.telegramConfig.SendLevels...),
		"tradeEventsEnabled":            p.telegramConfig.TradeEventsEnabled,
		"positionReportEnabled":         p.telegramConfig.PositionReportEnabled,
		"positionReportIntervalMinutes": normalizeTelegramPositionReportInterval(p.telegramConfig.PositionReportIntervalMinutes),
		"hasBotToken":                   token != "",
		"maskedBotToken":                maskTelegramToken(token),
		"updatedAt":                     p.telegramConfig.UpdatedAt,
	}
}

func (p *Platform) UpdateTelegramConfig(enabled bool, botToken, chatID string, sendLevels []string, tradeEventsEnabled, positionReportEnabled bool, positionReportIntervalMinutes int) (map[string]any, error) {
	config := p.telegramConfig
	config.Enabled = enabled
	config.TradeEventsEnabled = tradeEventsEnabled
	config.PositionReportEnabled = positionReportEnabled
	config.PositionReportIntervalMinutes = normalizeTelegramPositionReportInterval(positionReportIntervalMinutes)
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
		if p.telegramConfig.PositionReportIntervalMinutes <= 0 {
			p.telegramConfig.PositionReportIntervalMinutes = 30
		}
		return nil
	}
	config.PositionReportIntervalMinutes = normalizeTelegramPositionReportInterval(config.PositionReportIntervalMinutes)
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

func normalizeTelegramPositionReportInterval(minutes int) int {
	if minutes <= 0 {
		return 30
	}
	if minutes < 5 {
		return 5
	}
	if minutes > 1440 {
		return 1440
	}
	return minutes
}

func maskTelegramToken(token string) string {
	if len(token) <= 8 {
		return ""
	}
	return token[:4] + "..." + token[len(token)-4:]
}
