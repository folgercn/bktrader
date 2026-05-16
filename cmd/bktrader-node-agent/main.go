package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/nodeagent"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	_ = config.LoadEnvFile()

	agent, err := nodeagent.New(nodeagent.Config{
		Addr:       getenv("BKTRADER_NODE_AGENT_HTTP_ADDR", "127.0.0.1:18081"),
		Token:      strings.TrimSpace(os.Getenv("BKTRADER_NODE_AGENT_TOKEN")),
		TokenFile:  strings.TrimSpace(os.Getenv("BKTRADER_NODE_AGENT_TOKEN_FILE")),
		TargetsRaw: strings.TrimSpace(os.Getenv("BKTRADER_NODE_AGENT_TARGETS_JSON")),
		Version:    version + "@" + commit,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "node-agent config failed: %v\n", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              agent.Addr(),
		Handler:           agent.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("bktrader node-agent listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("bktrader node-agent exited unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("bktrader node-agent shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("bktrader node-agent graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("bktrader node-agent stopped")
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
