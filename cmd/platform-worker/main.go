package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/wuyaocheng/bktrader/internal/app"
	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/logging"
)

func main() {
	_ = config.LoadEnvFile()

	cfg := config.Load()
	role := strings.ToLower(strings.TrimSpace(os.Getenv("BKTRADER_ROLE")))
	if role == "" {
		role = "live-runner"
	}
	cfg.ProcessRole = role
	if err := cfg.Validate(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "配置验证失败: %v\n", err)
		os.Exit(1)
	}
	if err := logging.Configure(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "日志配置失败: %v\n", err)
		os.Exit(1)
	}
	if _, err := logging.BootstrapFromDisk(cfg.LogDir); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "日志预热失败: %v\n", err)
	}

	switch role {
	case "live-runner", "signal-runtime-runner", "notification-worker":
	default:
		_, _ = fmt.Fprintf(os.Stderr, "platform-worker 不支持 BKTRADER_ROLE=%s\n", role)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	platform, err := app.NewPlatform(cfg)
	if err != nil {
		slog.Error("platform worker bootstrap failed", "role", role, "error", err)
		os.Exit(1)
	}
	options := app.RuntimeOptionsForRole(role)
	app.StartRuntimeComponents(ctx, platform, cfg, options)
	slog.Info("platform worker started", "role", role)

	<-ctx.Done()
	slog.Info("platform worker stopped", "role", role)
}
