package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
)

func NewRouter(cfg config.Config, platform *service.Platform) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"app":    cfg.AppName,
			"env":    cfg.Environment,
			"time":   time.Now().UTC(),
		})
	})

	mux.HandleFunc("/api/v1/overview", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"modules": []string{
				"signal-sources",
				"strategies",
				"accounts",
				"orders",
				"positions",
				"live-monitoring",
				"paper-trading",
				"backtests",
				"chart-feed",
			},
			"notes": "Phase 1 MVP API with pluggable storage, paper execution flow, CRUD-style endpoints, and TradingView-friendly chart feed scaffolding.",
		})
	})

	registerSignalRoutes(mux, platform)
	registerStrategyRoutes(mux, platform)
	registerAccountRoutes(mux, platform)
	registerOrderRoutes(mux, platform)
	registerBacktestRoutes(mux, platform)
	registerPaperRoutes(mux, platform)
	registerChartRoutes(mux, platform)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("request body is required")
	}
	return json.Unmarshal(body, target)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}
