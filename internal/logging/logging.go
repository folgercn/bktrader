package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
)

// Configure 初始化全局默认日志器，统一接管结构化日志输出。
func Configure(cfg config.Config) error {
	level, err := parseLevel(cfg.LogLevel)
	if err != nil {
		return err
	}

	options := &slog.HandlerOptions{
		AddSource: cfg.LogAddSource,
		Level:     level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				if value, ok := attr.Value.Any().(time.Time); ok {
					return slog.String(attr.Key, value.UTC().Format(time.RFC3339))
				}
			}
			return attr
		},
	}

	format := strings.ToLower(strings.TrimSpace(cfg.LogFormat))
	var handler slog.Handler
	switch format {
	case "", "text":
		handler = slog.NewTextHandler(os.Stdout, options)
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, options)
	default:
		return fmt.Errorf("unsupported log format: %s", cfg.LogFormat)
	}

	logger := slog.New(handler).With(
		"app", cfg.AppName,
		"env", cfg.Environment,
	)
	slog.SetDefault(logger)
	return nil
}

// HTTPLevel 根据响应状态码映射合适的日志级别。
func HTTPLevel(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level: %s", value)
	}
}
