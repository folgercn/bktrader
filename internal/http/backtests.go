package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerBacktestRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/backtests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.ListBacktests())
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
			writeJSON(w, http.StatusCreated, platform.CreateBacktest(payload.StrategyVersionID, payload.Parameters))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
