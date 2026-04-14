package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/logging"
	"github.com/wuyaocheng/bktrader/internal/store/postgres"
)

func main() {
	_ = config.LoadEnvFile()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "配置验证失败: %v\n", err)
		os.Exit(1)
	}
	if err := logging.Configure(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "日志配置失败: %v\n", err)
		os.Exit(1)
	}

	slog.Info("running database migrations", "store_backend", cfg.StoreBackend)

	if err := postgres.Migrate(cfg.PostgresDSN); err != nil {
		slog.Error("database migrations failed", "error", err)
		os.Exit(1)
	}

	slog.Info("database migrations applied successfully")
}
