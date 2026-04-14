package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/config"
)

func TestConfigureJSONLoggerWritesStructuredOutput(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	originalStdout := os.Stdout
	originalLogger := slog.Default()
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
		slog.SetDefault(originalLogger)
	})

	cfg := config.Config{
		AppName:     "bkTrader-test",
		Environment: "test",
		LogLevel:    "debug",
		LogFormat:   "json",
	}
	if err := Configure(cfg); err != nil {
		t.Fatalf("configure logger: %v", err)
	}

	slog.Info("hello logging test")
	_ = writer.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read logger output: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, `"msg":"hello logging test"`) {
		t.Fatalf("expected message in logger output, got %q", text)
	}
	if !strings.Contains(text, `"app":"bkTrader-test"`) {
		t.Fatalf("expected app attribute in logger output, got %q", text)
	}
	if !strings.Contains(text, `"env":"test"`) {
		t.Fatalf("expected env attribute in logger output, got %q", text)
	}
}

func TestConfigureRejectsUnsupportedFormat(t *testing.T) {
	err := Configure(config.Config{
		AppName:     "bkTrader-test",
		Environment: "test",
		LogLevel:    "info",
		LogFormat:   "yaml",
	})
	if err == nil {
		t.Fatalf("expected unsupported format to fail")
	}
}

func TestHTTPLevel(t *testing.T) {
	tests := []struct {
		status int
		level  slog.Level
	}{
		{status: 200, level: slog.LevelInfo},
		{status: 404, level: slog.LevelWarn},
		{status: 503, level: slog.LevelError},
	}
	for _, tc := range tests {
		if got := HTTPLevel(tc.status); got != tc.level {
			t.Fatalf("status %d: expected %v, got %v", tc.status, tc.level, got)
		}
	}
}
