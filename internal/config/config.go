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
	StoreBackend                   string // 存储后端类型（memory / postgres）
	AutoMigrate                    bool   // 是否在启动时自动执行数据库迁移
	PostgresDSN                    string // PostgreSQL 连接字符串
	RedisAddr                      string // Redis 地址
	NATSURL                        string // NATS 消息队列地址
	PaperTickInterval              int    // 模拟盘 Ticker 间隔（秒），默认 15
	MinuteDataDir                  string // 1min 数据目录
	TickDataDir                    string // tick 数据目录
	AuthEnabled                    bool   // 是否启用 API 鉴权
	AuthUsername                   string // 管理后台用户名
	AuthPassword                   string // 管理后台密码
	AuthSecret                     string // Token 签名密钥
	AuthTokenTTLMinutes            int    // Token 有效期（分钟）
	TradeTickFreshnessSeconds      int    // trade tick 新鲜度阈值
	OrderBookFreshnessSeconds      int    // order book 新鲜度阈值
	SignalBarFreshnessSeconds      int    // signal bar 新鲜度阈值
	RuntimeQuietSeconds            int    // runtime quiet 告警阈值
	StrategyEvaluationQuietSeconds int    // 策略触发进入评估的静默阈值
	LiveAccountSyncFreshnessSecs   int    // live account 同步陈旧阈值
	PaperStartReadinessTimeoutSecs int    // paper 启动前 runtime readiness 等待阈值
	TelegramEnabled                bool   // Telegram 通知是否启用
	TelegramBotToken               string // Telegram Bot Token
	TelegramChatID                 string // Telegram Chat ID
	TelegramSendLevels             string // Telegram 默认发送等级（逗号分隔）
	WSHandshakeTimeoutSeconds      int    // WebSocket 握手超时
	WSReadStaleTimeoutSeconds      int    // WebSocket 读取陈旧超时
	WSPingIntervalSeconds          int    // WebSocket Ping 间隔
	WSPassiveCloseTimeoutSeconds   int    // WebSocket 被动关闭超时
	WSReconnectBackoffs            []int  // WebSocket 普通重连退避序列
	WSReconnectRecoveryBackoffs    []int  // WebSocket 恢复模式重连退避序列
	RESTLimiterRPS                 int    // Binance REST 每秒请求数限制
	RESTLimiterBurst               int    // Binance REST 突发限制
	RESTBackoffSeconds             int    // Binance REST 熔断时长
	LiveMarketCacheTTLMinutes      int    // 市场快照缓存有效期
	TelegramHTTPTimeoutSeconds     int    // Telegram HTTP 请求超时
	BinanceRecvWindowMs            int    // Binance 请求 RecvWindow
	LiveSignalWarmWindowDays       int    // 实盘信号预热窗口（天）
	LiveFastSignalWarmWindowDays   int    // 实盘快速信号预热窗口（天）
	LiveMinuteWarmWindowDays       int    // 实盘分钟数据预热窗口（天）
}

// Load 从环境变量加载配置，未设置的使用默认值。
func Load() Config {
	tickInterval := 15
	if v := os.Getenv("PAPER_TICK_INTERVAL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			tickInterval = parsed
		}
	}
	tradeTickFreshness := intFromEnv("TRADE_TICK_FRESHNESS_SECONDS", 15)
	orderBookFreshness := intFromEnv("ORDER_BOOK_FRESHNESS_SECONDS", 10)
	signalBarFreshness := intFromEnv("SIGNAL_BAR_FRESHNESS_SECONDS", 30)
	runtimeQuiet := intFromEnv("RUNTIME_QUIET_SECONDS", 30)
	strategyEvaluationQuiet := intFromEnv("STRATEGY_EVALUATION_QUIET_SECONDS", 15)
	liveAccountSyncFreshness := intFromEnv("LIVE_ACCOUNT_SYNC_FRESHNESS_SECONDS", 60)
	paperReadinessTimeout := intFromEnv("PAPER_START_READINESS_TIMEOUT_SECONDS", 5)

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
		StoreBackend:                   getenv("STORE_BACKEND", "memory"),
		AutoMigrate:                    boolFromEnv("AUTO_MIGRATE", true),
		PostgresDSN:                    getenv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable"),
		RedisAddr:                      getenv("REDIS_ADDR", "localhost:6379"),
		NATSURL:                        getenv("NATS_URL", "nats://localhost:4222"),
		PaperTickInterval:              tickInterval,
		MinuteDataDir:                  getenv("MINUTE_DATA_DIR", "."),
		TickDataDir:                    getenv("TICK_DATA_DIR", "./dataset/archive"),
		AuthEnabled:                    boolFromEnv("AUTH_ENABLED", false),
		AuthUsername:                   getenv("AUTH_USERNAME", "admin"),
		AuthPassword:                   os.Getenv("AUTH_PASSWORD"),
		AuthSecret:                     os.Getenv("AUTH_SECRET"),
		AuthTokenTTLMinutes:            authTokenTTL,
		TradeTickFreshnessSeconds:      tradeTickFreshness,
		OrderBookFreshnessSeconds:      orderBookFreshness,
		SignalBarFreshnessSeconds:      signalBarFreshness,
		RuntimeQuietSeconds:            runtimeQuiet,
		StrategyEvaluationQuietSeconds: strategyEvaluationQuiet,
		LiveAccountSyncFreshnessSecs:   liveAccountSyncFreshness,
		PaperStartReadinessTimeoutSecs: paperReadinessTimeout,
		TelegramEnabled:                boolFromEnv("TELEGRAM_ENABLED", false),
		TelegramBotToken:               getenv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:                 getenv("TELEGRAM_CHAT_ID", ""),
		TelegramSendLevels:             getenv("TELEGRAM_SEND_LEVELS", "critical,warning"),
		WSHandshakeTimeoutSeconds:      intFromEnv("WS_HANDSHAKE_TIMEOUT_SECONDS", 10),
		WSReadStaleTimeoutSeconds:      intFromEnv("WS_READ_STALE_TIMEOUT_SECONDS", 60),
		WSPingIntervalSeconds:          intFromEnv("WS_PING_INTERVAL_SECONDS", 20),
		WSPassiveCloseTimeoutSeconds:   intFromEnv("WS_PASSIVE_CLOSE_TIMEOUT_SECONDS", 2),
		WSReconnectBackoffs:            intSliceFromEnv("WS_RECONNECT_BACKOFFS", []int{10, 30, 60}),
		WSReconnectRecoveryBackoffs:    intSliceFromEnv("WS_RECONNECT_RECOVERY_BACKOFFS", []int{30, 120}),
		RESTLimiterRPS:                 intFromEnv("REST_LIMITER_RPS", 30),
		RESTLimiterBurst:               intFromEnv("REST_LIMITER_BURST", 50),
		RESTBackoffSeconds:             intFromEnv("REST_BACKOFF_SECONDS", 60),
		LiveMarketCacheTTLMinutes:      intFromEnv("LIVE_MARKET_CACHE_TTL_MINUTES", 10),
		TelegramHTTPTimeoutSeconds:     intFromEnv("TELEGRAM_HTTP_TIMEOUT_SECONDS", 8),
		BinanceRecvWindowMs:            intFromEnv("BINANCE_REST_RECV_WINDOW_MS", 5000),
		LiveSignalWarmWindowDays:       intFromEnv("LIVE_SIGNAL_WARM_WINDOW_DAYS", 400),
		LiveFastSignalWarmWindowDays:   intFromEnv("LIVE_FAST_SIGNAL_WARM_WINDOW_DAYS", 7),
		LiveMinuteWarmWindowDays:       intFromEnv("LIVE_MINUTE_WARM_WINDOW_DAYS", 30),
	}
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
	return nil
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
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
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
		if p, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
