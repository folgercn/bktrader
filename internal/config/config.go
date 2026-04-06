package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config 存储平台运行所需的全部配置项。
type Config struct {
	AppName             string // 应用名称
	Environment         string // 运行环境（development / production）
	HTTPAddr            string // HTTP 监听地址
	StoreBackend        string // 存储后端类型（memory / postgres）
	AutoMigrate         bool   // 是否在启动时自动执行数据库迁移
	PostgresDSN         string // PostgreSQL 连接字符串
	RedisAddr           string // Redis 地址
	NATSURL             string // NATS 消息队列地址
	PaperTickInterval   int    // 模拟盘 Ticker 间隔（秒），默认 15
	MinuteDataDir       string // 1min 数据目录
	TickDataDir         string // tick 数据目录
	AuthEnabled         bool   // 是否启用 API 鉴权
	AuthUsername        string // 管理后台用户名
	AuthPassword        string // 管理后台密码
	AuthSecret          string // Token 签名密钥
	AuthTokenTTLMinutes int    // Token 有效期（分钟）
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
		AppName:             getenv("APP_NAME", "bkTrader-platform"),
		Environment:         getenv("APP_ENV", "development"),
		HTTPAddr:            getenv("HTTP_ADDR", ":8080"),
		StoreBackend:        getenv("STORE_BACKEND", "memory"),
		AutoMigrate:         getenv("AUTO_MIGRATE", "false") == "true",
		PostgresDSN:         getenv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable"),
		RedisAddr:           getenv("REDIS_ADDR", "localhost:6379"),
		NATSURL:             getenv("NATS_URL", "nats://localhost:4222"),
		PaperTickInterval:   tickInterval,
		MinuteDataDir:       getenv("MINUTE_DATA_DIR", "."),
		TickDataDir:         getenv("TICK_DATA_DIR", "."),
		AuthEnabled:         getenv("AUTH_ENABLED", "false") == "true",
		AuthUsername:        getenv("AUTH_USERNAME", "admin"),
		AuthPassword:        os.Getenv("AUTH_PASSWORD"),
		AuthSecret:          os.Getenv("AUTH_SECRET"),
		AuthTokenTTLMinutes: authTokenTTL,
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
