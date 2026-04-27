package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 存储平台运行所需的全部配置项。
type Config struct {
	AppName                        string // 应用名称
	Environment                    string // 运行环境（development / production）
	LogLevel                       string // 日志级别（debug / info / warn / error）
	LogFormat                      string // 日志格式（text / json）
	LogAddSource                   bool   // 是否在日志中附带源码位置信息
	LogDir                         string // 日志镜像落盘目录；为空时禁用日志持久化
	LogRetentionDays               int    // 日志保留天数
	LogMaxSizeMB                   int    // 单个日志文件滚动前的最大体积（MB）
	HTTPAddr                       string // HTTP 监听地址
	ProcessRole                    string // 进程角色：monolith / api / live-runner / signal-runtime-runner / notification-worker
	StoreBackend                   string // 存储后端类型（memory / postgres）
	AutoMigrate                    bool   // 是否在启动时自动执行数据库迁移
	PostgresDSN                    string // PostgreSQL 连接字符串
	RedisAddr                      string // Redis 地址
	NATSURL                        string // NATS 消息队列地址
	RuntimeEventBus                string // runtime event bus: nats / disabled
	PaperTickInterval              int    // 模拟盘 Ticker 间隔（秒），默认 15
	MinuteDataDir                  string // 1min 数据目录
	TickDataDir                    string // tick 数据目录
	AuthEnabled                    bool   // 是否启用 API 鉴权
	AuthUsername                   string // 管理后台用户名
	AuthPassword                   string // 管理后台密码
	AuthSecret                     string // Token 签名密钥
	AuthTokenTTLMinutes            int    // Token 有效期（分钟）
	TradeTickFreshnessSeconds      *int   // trade tick 新鲜度阈值
	OrderBookFreshnessSeconds      *int   // order book 新鲜度阈值
	SignalBarFreshnessSeconds      *int   // signal bar 新鲜度阈值
	RuntimeQuietSeconds            *int   // runtime quiet 告警阈值
	StrategyEvaluationQuietSeconds *int   // 策略触发进入评估的静默阈值
	LiveAccountSyncFreshnessSecs   *int   // live account 同步陈旧阈值
	PaperStartReadinessTimeoutSecs *int   // paper 启动前 runtime readiness 等待阈值
	TelegramEnabled                bool   // Telegram 通知是否启用
	TelegramBotToken               string // Telegram Bot Token
	TelegramChatID                 string // Telegram Chat ID
	TelegramSendLevels             string // Telegram 默认发送等级（逗号分隔）
	WSHandshakeTimeoutSeconds      *int   // WebSocket 握手超时
	WSReadStaleTimeoutSeconds      *int   // WebSocket 读取陈旧超时
	WSPingIntervalSeconds          *int   // WebSocket Ping 间隔
	WSPassiveCloseTimeoutSeconds   *int   // WebSocket 被动关闭超时
	WSReconnectBackoffs            []int  // WebSocket 普通重连退避序列
	WSReconnectRecoveryBackoffs    []int  // WebSocket 恢复模式重连退避序列
	RESTLimiterRPS                 *int   // Binance REST 每秒请求数限制
	RESTLimiterBurst               *int   // Binance REST 突发限制
	RESTBackoffSeconds             *int   // Binance REST 熔断时长
	LiveMarketCacheTTLMinutes      *int   // 市场快照缓存有效期
	TelegramHTTPTimeoutSeconds     *int   // Telegram HTTP 请求超时
	BinanceRecvWindowMs            *int   // Binance 请求 RecvWindow
	LiveSignalWarmWindowDays       *int   // 实盘信号预热窗口（天）
	LiveFastSignalWarmWindowDays   *int   // 实盘快速信号预热窗口（天）
	LiveMinuteWarmWindowDays       *int   // 实盘分钟数据预热窗口（天）
	DashboardLiveSessionsPollMs    int    // 仪表盘 Live Sessions 轮询间隔 (ms)
	DashboardPositionsPollMs       int    // 仪表盘 Positions 轮询间隔 (ms)
	DashboardOrdersPollMs          int    // 仪表盘 Orders 轮询间隔 (ms)
	DashboardFillsPollMs           int    // 仪表盘 Fills 轮询间隔 (ms)
	DashboardAlertsPollMs          int    // 仪表盘 Alerts 轮询间隔 (ms)
	DashboardNotificationsPollMs   int    // 仪表盘 Notifications 轮询间隔 (ms)
	DashboardMonitorHealthPollMs   int    // 仪表盘 Monitor Health 轮询间隔 (ms)
}

// Load 从环境变量加载配置，未设置的使用默认值。
func Load() Config {
	tickInterval := 15
	if v := os.Getenv("PAPER_TICK_INTERVAL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			tickInterval = parsed
		}
	}

	authTokenTTL := 720
	if v := os.Getenv("AUTH_TOKEN_TTL_MINUTES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			authTokenTTL = parsed
		}
	}

	return Config{
		AppName:                        getenv("APP_NAME", "bkTrader-platform"),
		Environment:                    getenv("APP_ENV", "development"),
		LogLevel:                       getenv("LOG_LEVEL", "info"),
		LogFormat:                      getenv("LOG_FORMAT", "text"),
		LogAddSource:                   boolFromEnv("LOG_ADD_SOURCE", false),
		LogDir:                         strings.TrimSpace(os.Getenv("LOG_DIR")),
		LogRetentionDays:               intFromEnv("LOG_RETENTION_DAYS", 7),
		LogMaxSizeMB:                   intFromEnv("LOG_MAX_SIZE_MB", 100),
		HTTPAddr:                       getenv("HTTP_ADDR", ":8080"),
		ProcessRole:                    strings.ToLower(strings.TrimSpace(getenv("BKTRADER_ROLE", "monolith"))),
		StoreBackend:                   getenv("STORE_BACKEND", "memory"),
		AutoMigrate:                    boolFromEnv("AUTO_MIGRATE", true),
		PostgresDSN:                    getenv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable"),
		RedisAddr:                      getenv("REDIS_ADDR", "localhost:6379"),
		NATSURL:                        getenv("NATS_URL", "nats://localhost:4222"),
		RuntimeEventBus:                strings.ToLower(strings.TrimSpace(getenv("RUNTIME_EVENT_BUS", "nats"))),
		PaperTickInterval:              tickInterval,
		MinuteDataDir:                  getenv("MINUTE_DATA_DIR", "."),
		TickDataDir:                    getenv("TICK_DATA_DIR", "./dataset/archive"),
		AuthEnabled:                    boolFromEnv("AUTH_ENABLED", false),
		AuthUsername:                   getenv("AUTH_USERNAME", "admin"),
		AuthPassword:                   os.Getenv("AUTH_PASSWORD"),
		AuthSecret:                     os.Getenv("AUTH_SECRET"),
		AuthTokenTTLMinutes:            authTokenTTL,
		TradeTickFreshnessSeconds:      IntPtrFromEnv("TRADE_TICK_FRESHNESS_SECONDS"),
		OrderBookFreshnessSeconds:      IntPtrFromEnv("ORDER_BOOK_FRESHNESS_SECONDS"),
		SignalBarFreshnessSeconds:      IntPtrFromEnv("SIGNAL_BAR_FRESHNESS_SECONDS"),
		RuntimeQuietSeconds:            IntPtrFromEnv("RUNTIME_QUIET_SECONDS"),
		StrategyEvaluationQuietSeconds: IntPtrFromEnv("STRATEGY_EVALUATION_QUIET_SECONDS"),
		LiveAccountSyncFreshnessSecs:   IntPtrFromEnv("LIVE_ACCOUNT_SYNC_FRESHNESS_SECONDS"),
		PaperStartReadinessTimeoutSecs: IntPtrFromEnv("PAPER_START_READINESS_TIMEOUT_SECONDS"),
		TelegramEnabled:                boolFromEnv("TELEGRAM_ENABLED", false),
		TelegramBotToken:               getenv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:                 getenv("TELEGRAM_CHAT_ID", ""),
		TelegramSendLevels:             getenv("TELEGRAM_SEND_LEVELS", "critical,warning"),
		WSHandshakeTimeoutSeconds:      IntPtrFromEnv("WS_HANDSHAKE_TIMEOUT_SECONDS"),
		WSReadStaleTimeoutSeconds:      IntPtrFromEnv("WS_READ_STALE_TIMEOUT_SECONDS"),
		WSPingIntervalSeconds:          IntPtrFromEnv("WS_PING_INTERVAL_SECONDS"),
		WSPassiveCloseTimeoutSeconds:   IntPtrFromEnv("WS_PASSIVE_CLOSE_TIMEOUT_SECONDS"),
		WSReconnectBackoffs:            intSliceFromEnv("WS_RECONNECT_BACKOFFS", nil),
		WSReconnectRecoveryBackoffs:    intSliceFromEnv("WS_RECONNECT_RECOVERY_BACKOFFS", nil),
		RESTLimiterRPS:                 IntPtrFromEnv("REST_LIMITER_RPS"),
		RESTLimiterBurst:               IntPtrFromEnv("REST_LIMITER_BURST"),
		RESTBackoffSeconds:             IntPtrFromEnv("REST_BACKOFF_SECONDS"),
		LiveMarketCacheTTLMinutes:      IntPtrFromEnv("LIVE_MARKET_CACHE_TTL_MINUTES"),
		TelegramHTTPTimeoutSeconds:     IntPtrFromEnv("TELEGRAM_HTTP_TIMEOUT_SECONDS"),
		BinanceRecvWindowMs:            IntPtrFromEnv("BINANCE_REST_RECV_WINDOW_MS"),
		LiveSignalWarmWindowDays:       IntPtrFromEnv("LIVE_SIGNAL_WARM_WINDOW_DAYS"),
		LiveFastSignalWarmWindowDays:   IntPtrFromEnv("LIVE_FAST_SIGNAL_WARM_WINDOW_DAYS"),
		LiveMinuteWarmWindowDays:       IntPtrFromEnv("LIVE_MINUTE_WARM_WINDOW_DAYS"),
		DashboardLiveSessionsPollMs:    intFromEnvWithMin("DASHBOARD_LIVE_SESSIONS_POLL_MS", 2000, 1000),
		DashboardPositionsPollMs:       intFromEnvWithMin("DASHBOARD_POSITIONS_POLL_MS", 2000, 1000),
		DashboardOrdersPollMs:          intFromEnvWithMin("DASHBOARD_ORDERS_POLL_MS", 2000, 1000),
		DashboardFillsPollMs:           intFromEnvWithMin("DASHBOARD_FILLS_POLL_MS", 2000, 1000),
		DashboardAlertsPollMs:          intFromEnvWithMin("DASHBOARD_ALERTS_POLL_MS", 2000, 1000),
		DashboardNotificationsPollMs:   intFromEnvWithMin("DASHBOARD_NOTIFICATIONS_POLL_MS", 2000, 1000),
		DashboardMonitorHealthPollMs:   intFromEnvWithMin("DASHBOARD_MONITOR_HEALTH_POLL_MS", 2000, 1000),
	}
}

func IntPtrFromEnv(key string) *int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			p := new(int)
			*p = parsed
			return p
		}
	}
	return nil
}

// Validate 校验配置项的合法性，启动前快速失败。
func (c Config) Validate() error {
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("不支持的 LOG_LEVEL: %s（可选: debug, info, warn, error）", c.LogLevel)
	}
	switch strings.ToLower(strings.TrimSpace(c.LogFormat)) {
	case "", "text", "json":
	default:
		return fmt.Errorf("不支持的 LOG_FORMAT: %s（可选: text, json）", c.LogFormat)
	}
	if strings.TrimSpace(c.LogDir) != "" {
		if c.LogRetentionDays <= 0 {
			return fmt.Errorf("LOG_RETENTION_DAYS 必须大于 0")
		}
		if c.LogMaxSizeMB <= 0 {
			return fmt.Errorf("LOG_MAX_SIZE_MB 必须大于 0")
		}
	}
	if c.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR 不能为空")
	}
	switch strings.ToLower(strings.TrimSpace(c.ProcessRole)) {
	case "", "monolith", "api", "live-runner", "signal-runtime-runner", "notification-worker":
	default:
		return fmt.Errorf("不支持的 BKTRADER_ROLE: %s（可选: monolith, api, live-runner, signal-runtime-runner, notification-worker）", c.ProcessRole)
	}
	if c.StoreBackend == "postgres" && c.PostgresDSN == "" {
		return fmt.Errorf("STORE_BACKEND=postgres 时 POSTGRES_DSN 不能为空")
	}
	switch c.StoreBackend {
	case "memory", "postgres", "":
	default:
		return fmt.Errorf("不支持的 STORE_BACKEND: %s（可选: memory, postgres）", c.StoreBackend)
	}
	if c.AuthEnabled {
		if c.AuthUsername == "" {
			return fmt.Errorf("AUTH_ENABLED=true 时 AUTH_USERNAME 不能为空")
		}
		if c.AuthPassword == "" {
			return fmt.Errorf("AUTH_ENABLED=true 时 AUTH_PASSWORD 不能为空")
		}
		if c.AuthSecret == "" {
			return fmt.Errorf("AUTH_ENABLED=true 时 AUTH_SECRET 不能为空")
		}
		if c.AuthTokenTTLMinutes <= 0 {
			return fmt.Errorf("AUTH_TOKEN_TTL_MINUTES 必须大于 0")
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.RuntimeEventBus)) {
	case "", "nats", "disabled":
	default:
		return fmt.Errorf("不支持的 RUNTIME_EVENT_BUS: %s（可选: nats, disabled）", c.RuntimeEventBus)
	}
	return nil
}

// RuntimeActionsEnabled reports whether this process is allowed to execute
// live runtime mutations such as start, dispatch, and explicit sync.
func (c Config) RuntimeActionsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(c.ProcessRole)) {
	case "", "monolith", "live-runner":
		return true
	default:
		return false
	}
}

// getenv 读取环境变量，未设置时返回 fallback 默认值。
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intFromEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func intFromEnvWithMin(key string, fallback, min int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			if parsed < min {
				return fallback
			}
			return parsed
		}
	}
	return fallback
}

func boolFromEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func intSliceFromEnv(key string, fallback []int) []int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if p, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && p > 0 {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
