package http

import (
	"net/http"
	"strings"

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

	mux.HandleFunc("/api/v1/runtime-policy", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.RuntimePolicy())
		case http.MethodPost:
			var payload service.RuntimePolicy
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			policy, err := platform.UpdateRuntimePolicy(payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid runtime policy payload")
				return
			}
			writeJSON(w, http.StatusOK, policy)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
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

	mux.HandleFunc("/api/v1/signal-runtime/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.ListSignalRuntimeSessions())
		case http.MethodPost:
			var payload struct {
				AccountID  string `json:"accountId"`
				StrategyID string `json:"strategyId"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if payload.AccountID == "" || payload.StrategyID == "" {
				writeError(w, http.StatusBadRequest, "accountId and strategyId are required")
				return
			}
			item, err := platform.CreateSignalRuntimeSession(payload.AccountID, payload.StrategyID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/signal-runtime/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/signal-runtime/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 1 && r.Method == http.MethodGet {
			item, err := platform.GetSignalRuntimeSession(parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
			return
		}
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "signal runtime session route not found")
			return
		}
		sessionID, action := parts[0], parts[1]
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch action {
		case "start":
			item, err := platform.StartSignalRuntimeSession(sessionID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "stop":
			item, err := platform.StopSignalRuntimeSession(sessionID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			writeError(w, http.StatusNotFound, "signal runtime session action not found")
		}
	})
}
