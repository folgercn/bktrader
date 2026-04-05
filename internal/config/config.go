package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config 存储平台运行所需的全部配置项。
type Config struct {
	AppName                        string // 应用名称
	Environment                    string // 运行环境（development / production）
	HTTPAddr                       string // HTTP 监听地址
	StoreBackend                   string // 存储后端类型（memory / postgres）
	AutoMigrate                    bool   // 是否在启动时自动执行数据库迁移
	PostgresDSN                    string // PostgreSQL 连接字符串
	RedisAddr                      string // Redis 地址
	NATSURL                        string // NATS 消息队列地址
	PaperTickInterval              int    // 模拟盘 Ticker 间隔（秒），默认 15
	MinuteDataDir                  string // 1min 数据目录
	TickDataDir                    string // tick 数据目录
	TradeTickFreshnessSeconds      int    // trade tick 新鲜度阈值
	OrderBookFreshnessSeconds      int    // order book 新鲜度阈值
	SignalBarFreshnessSeconds      int    // signal bar 新鲜度阈值
	RuntimeQuietSeconds            int    // runtime quiet 告警阈值
	PaperStartReadinessTimeoutSecs int    // paper 启动前 runtime readiness 等待阈值
	TelegramEnabled                bool   // Telegram 通知是否启用
	TelegramBotToken               string // Telegram Bot Token
	TelegramChatID                 string // Telegram Chat ID
	TelegramSendLevels             string // Telegram 默认发送等级（逗号分隔）
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
	paperReadinessTimeout := intFromEnv("PAPER_START_READINESS_TIMEOUT_SECONDS", 5)

	return Config{
		AppName:                        getenv("APP_NAME", "bkTrader-platform"),
		Environment:                    getenv("APP_ENV", "development"),
		HTTPAddr:                       getenv("HTTP_ADDR", ":8080"),
		StoreBackend:                   getenv("STORE_BACKEND", "memory"),
		AutoMigrate:                    getenv("AUTO_MIGRATE", "false") == "true",
		PostgresDSN:                    getenv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable"),
		RedisAddr:                      getenv("REDIS_ADDR", "localhost:6379"),
		NATSURL:                        getenv("NATS_URL", "nats://localhost:4222"),
		PaperTickInterval:              tickInterval,
		MinuteDataDir:                  getenv("MINUTE_DATA_DIR", "."),
		TickDataDir:                    getenv("TICK_DATA_DIR", "./dataset/archive"),
		TradeTickFreshnessSeconds:      tradeTickFreshness,
		OrderBookFreshnessSeconds:      orderBookFreshness,
		SignalBarFreshnessSeconds:      signalBarFreshness,
		RuntimeQuietSeconds:            runtimeQuiet,
		PaperStartReadinessTimeoutSecs: paperReadinessTimeout,
		TelegramEnabled:                getenv("TELEGRAM_ENABLED", "false") == "true",
		TelegramBotToken:               getenv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:                 getenv("TELEGRAM_CHAT_ID", ""),
		TelegramSendLevels:             getenv("TELEGRAM_SEND_LEVELS", "critical,warning"),
	}
}

// Validate 校验配置项的合法性，启动前快速失败。
func (c Config) Validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR 不能为空")
	}
	// 使用 postgres 后端时必须提供 DSN
	if c.StoreBackend == "postgres" && c.PostgresDSN == "" {
		return fmt.Errorf("STORE_BACKEND=postgres 时 POSTGRES_DSN 不能为空")
	}
	// 验证 StoreBackend 取值范围
	switch c.StoreBackend {
	case "memory", "postgres", "":
		// 合法值
	default:
		return fmt.Errorf("不支持的 STORE_BACKEND: %s（可选: memory, postgres）", c.StoreBackend)
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
