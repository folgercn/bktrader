package config

import "os"

type Config struct {
	AppName      string
	Environment  string
	HTTPAddr     string
	StoreBackend string
	AutoMigrate  bool
	PostgresDSN  string
	RedisAddr    string
	NATSURL      string
}

func Load() Config {
	return Config{
		AppName:      getenv("APP_NAME", "bkTrader-platform"),
		Environment:  getenv("APP_ENV", "development"),
		HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
		StoreBackend: getenv("STORE_BACKEND", "memory"),
		AutoMigrate:  getenv("AUTO_MIGRATE", "false") == "true",
		PostgresDSN:  getenv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		NATSURL:      getenv("NATS_URL", "nats://localhost:4222"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
