package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wuyaocheng/bktrader/internal/app"
	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/logging"
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

	slog.Info("platform-api starting",
		"http_addr", cfg.HTTPAddr,
		"store_backend", cfg.StoreBackend,
		"auto_migrate", cfg.AutoMigrate,
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
	)

	server, err := app.NewServer(cfg)
	if err != nil {
		slog.Error("platform-api bootstrap failed", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("platform-api listening", "http_addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server exited unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("platform-api stopped")
}
