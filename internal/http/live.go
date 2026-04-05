package http

import (
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerLiveRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/live/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListLiveSessions()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				AccountID           string `json:"accountId"`
				StrategyID          string `json:"strategyId"`
				SignalTimeframe     string `json:"signalTimeframe"`
				ExecutionDataSource string `json:"executionDataSource"`
				Symbol              string `json:"symbol"`
				From                string `json:"from"`
				To                  string `json:"to"`
				StrategyEngine      string `json:"strategyEngine"`
				DefaultOrderQty     any    `json:"defaultOrderQuantity"`
				DispatchMode        string `json:"dispatchMode"`
				DispatchCooldownSec int    `json:"dispatchCooldownSeconds"`
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
			overrides := map[string]any{}
			if payload.SignalTimeframe != "" {
				overrides["signalTimeframe"] = payload.SignalTimeframe
			}
			if payload.ExecutionDataSource != "" {
				overrides["executionDataSource"] = payload.ExecutionDataSource
			}
			if payload.Symbol != "" {
				overrides["symbol"] = payload.Symbol
			}
			if payload.From != "" {
				overrides["from"] = payload.From
			}
			if payload.To != "" {
				overrides["to"] = payload.To
			}
			if payload.StrategyEngine != "" {
				overrides["strategyEngine"] = payload.StrategyEngine
			}
			if payload.DefaultOrderQty != nil {
				overrides["defaultOrderQuantity"] = payload.DefaultOrderQty
			}
			if payload.DispatchMode != "" {
				overrides["dispatchMode"] = payload.DispatchMode
			}
			if payload.DispatchCooldownSec > 0 {
				overrides["dispatchCooldownSeconds"] = payload.DispatchCooldownSec
			}
			item, err := platform.CreateLiveSession(payload.AccountID, payload.StrategyID, overrides)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/live/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/live/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "live session route not found")
			return
		}

		sessionID := parts[0]
		action := parts[1]
		switch action {
		case "start":
			item, err := platform.StartLiveSession(sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "stop":
			item, err := platform.StopLiveSession(sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "dispatch":
			item, err := platform.DispatchLiveSessionIntent(sessionID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			writeError(w, http.StatusNotFound, "unsupported live session action")
		}
	})
}
