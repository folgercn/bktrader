package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
)

// Configure 初始化全局默认日志器，统一接管结构化日志输出。
func Configure(cfg config.Config) error {
	if err := configureDiskMirror(cfg); err != nil {
		return err
	}

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

	handler = newMultiHandler(handler, newSystemCaptureHandler())

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

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) slog.Handler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, handler := range handlers {
		if handler != nil {
			filtered = append(filtered, handler)
		}
	}
	return &multiHandler{handlers: filtered}
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return &multiHandler{handlers: handlers}
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
