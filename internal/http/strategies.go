package http

import (
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerStrategyRoutes 注册策略管理相关路由。
func registerStrategyRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/strategy-engines", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.StrategyEngines())
	})

	mux.HandleFunc("/api/v1/strategies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// 获取所有策略列表
			items, err := platform.ListStrategies()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			// 创建新策略
			var payload struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{"name": payload.Name}, "name"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.CreateStrategy(payload.Name, payload.Description, payload.Parameters)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/strategies/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/strategies/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "signal-bindings" {
			writeError(w, http.StatusNotFound, "strategy signal binding route not found")
			return
		}
		strategyID := parts[0]
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListStrategySignalBindings(strategyID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload map[string]any
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.BindStrategySignalSource(strategyID, payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
