package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerBacktestRoutes 注册回测管理相关路由。
func registerBacktestRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/backtests/options", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.BacktestOptions())
	})

	// GET|POST /api/v1/backtests — 回测记录列表/创建回测
	mux.HandleFunc("/api/v1/backtests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListBacktests()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				StrategyVersionID string         `json:"strategyVersionId"`
				Parameters        map[string]any `json:"parameters"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{"strategyVersionId": payload.StrategyVersionID}, "strategyVersionId"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.CreateBacktest(payload.StrategyVersionID, payload.Parameters)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
