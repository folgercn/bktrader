package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
)

func NewRouter(cfg config.Config) http.Handler {
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
			"notes": "Scaffold API only. Persistence and exchange adapters will be implemented next.",
		})
	})

	registerSignalRoutes(mux)
	registerStrategyRoutes(mux)
	registerAccountRoutes(mux)
	registerOrderRoutes(mux)
	registerBacktestRoutes(mux)
	registerPaperRoutes(mux)
	registerChartRoutes(mux)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
