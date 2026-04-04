package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerSignalRoutes 注册信号源相关路由。
func registerSignalRoutes(mux *http.ServeMux, platform *service.Platform) {
	// GET /api/v1/signal-sources — 获取信号源列表
	mux.HandleFunc("/api/v1/signal-sources", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, platform.SignalSourceCatalog())
	})

	mux.HandleFunc("/api/v1/signal-source-types", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.SignalSourceTypes())
	})

	mux.HandleFunc("/api/v1/signal-runtime/adapters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.SignalRuntimeAdapters())
	})

	mux.HandleFunc("/api/v1/signal-runtime/plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		accountID := r.URL.Query().Get("accountId")
		strategyID := r.URL.Query().Get("strategyId")
		if accountID == "" || strategyID == "" {
			writeError(w, http.StatusBadRequest, "accountId and strategyId are required")
			return
		}
		plan, err := platform.BuildSignalRuntimePlan(accountID, strategyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, plan)
	})
}
