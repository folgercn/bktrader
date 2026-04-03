package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerPaperRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/paper/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.ListPaperSessions())
		case http.MethodPost:
			var payload struct {
				AccountID   string  `json:"accountId"`
				StrategyID  string  `json:"strategyId"`
				StartEquity float64 `json:"startEquity"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{
				"accountId":  payload.AccountID,
				"strategyId": payload.StrategyID,
			}, "accountId", "strategyId"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if payload.StartEquity <= 0 {
				payload.StartEquity = 100000
			}
			writeJSON(w, http.StatusCreated, platform.CreatePaperSession(payload.AccountID, payload.StrategyID, payload.StartEquity))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
