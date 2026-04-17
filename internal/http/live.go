package http

import (
	"errors"
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
				AccountID                                 string `json:"accountId"`
				StrategyID                                string `json:"strategyId"`
				PositionSizingMode                        string `json:"positionSizingMode"`
				DefaultOrderFraction                      any    `json:"defaultOrderFraction"`
				SignalTimeframe                           string `json:"signalTimeframe"`
				ExecutionDataSource                       string `json:"executionDataSource"`
				ExecutionStrategy                         string `json:"executionStrategy"`
				ExecutionOrderType                        string `json:"executionOrderType"`
				ExecutionTimeInForce                      string `json:"executionTimeInForce"`
				ExecutionPostOnly                         *bool  `json:"executionPostOnly"`
				ExecutionMaxSpreadBps                     any    `json:"executionMaxSpreadBps"`
				ExecutionWideSpreadMode                   string `json:"executionWideSpreadMode"`
				ExecutionRestingTimeoutSeconds            int    `json:"executionRestingTimeoutSeconds"`
				ExecutionTimeoutFallbackOrderType         string `json:"executionTimeoutFallbackOrderType"`
				ExecutionTimeoutFallbackTimeInForce       string `json:"executionTimeoutFallbackTimeInForce"`
				ExecutionEntryOrderType                   string `json:"executionEntryOrderType"`
				ExecutionEntryTimeInForce                 string `json:"executionEntryTimeInForce"`
				ExecutionEntryPostOnly                    *bool  `json:"executionEntryPostOnly"`
				ExecutionEntryMaxSpreadBps                any    `json:"executionEntryMaxSpreadBps"`
				ExecutionEntryWideSpreadMode              string `json:"executionEntryWideSpreadMode"`
				ExecutionEntryRestingTimeoutSeconds       int    `json:"executionEntryRestingTimeoutSeconds"`
				ExecutionEntryTimeoutFallbackOrderType    string `json:"executionEntryTimeoutFallbackOrderType"`
				ExecutionEntryTimeoutFallbackTimeInForce  string `json:"executionEntryTimeoutFallbackTimeInForce"`
				ExecutionPTExitOrderType                  string `json:"executionPTExitOrderType"`
				ExecutionPTExitTimeInForce                string `json:"executionPTExitTimeInForce"`
				ExecutionPTExitPostOnly                   *bool  `json:"executionPTExitPostOnly"`
				ExecutionPTExitMaxSpreadBps               any    `json:"executionPTExitMaxSpreadBps"`
				ExecutionPTExitWideSpreadMode             string `json:"executionPTExitWideSpreadMode"`
				ExecutionPTExitRestingTimeoutSeconds      int    `json:"executionPTExitRestingTimeoutSeconds"`
				ExecutionPTExitTimeoutFallbackOrderType   string `json:"executionPTExitTimeoutFallbackOrderType"`
				ExecutionPTExitTimeoutFallbackTimeInForce string `json:"executionPTExitTimeoutFallbackTimeInForce"`
				ExecutionSLExitOrderType                  string `json:"executionSLExitOrderType"`
				ExecutionSLExitTimeInForce                string `json:"executionSLExitTimeInForce"`
				ExecutionSLExitPostOnly                   *bool  `json:"executionSLExitPostOnly"`
				ExecutionSLExitMaxSpreadBps               any    `json:"executionSLExitMaxSpreadBps"`
				ExecutionSLExitWideSpreadMode             string `json:"executionSLExitWideSpreadMode"`
				ExecutionSLExitRestingTimeoutSeconds      int    `json:"executionSLExitRestingTimeoutSeconds"`
				ExecutionSLExitTimeoutFallbackOrderType   string `json:"executionSLExitTimeoutFallbackOrderType"`
				ExecutionSLExitTimeoutFallbackTimeInForce string `json:"executionSLExitTimeoutFallbackTimeInForce"`
				Symbol                                    string `json:"symbol"`
				From                                      string `json:"from"`
				To                                        string `json:"to"`
				StrategyEngine                            string `json:"strategyEngine"`
				DefaultOrderQty                           any    `json:"defaultOrderQuantity"`
				DispatchMode                              string `json:"dispatchMode"`
				DispatchCooldownSec                       int    `json:"dispatchCooldownSeconds"`
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
			if payload.PositionSizingMode != "" {
				overrides["positionSizingMode"] = payload.PositionSizingMode
			}
			if payload.DefaultOrderFraction != nil {
				overrides["defaultOrderFraction"] = payload.DefaultOrderFraction
			}
			if payload.ExecutionDataSource != "" {
				overrides["executionDataSource"] = payload.ExecutionDataSource
			}
			if payload.ExecutionStrategy != "" {
				overrides["executionStrategy"] = payload.ExecutionStrategy
			}
			if payload.ExecutionOrderType != "" {
				overrides["executionOrderType"] = payload.ExecutionOrderType
			}
			if payload.ExecutionTimeInForce != "" {
				overrides["executionTimeInForce"] = payload.ExecutionTimeInForce
			}
			if payload.ExecutionPostOnly != nil {
				overrides["executionPostOnly"] = *payload.ExecutionPostOnly
			}
			if payload.ExecutionMaxSpreadBps != nil {
				overrides["executionMaxSpreadBps"] = payload.ExecutionMaxSpreadBps
			}
			if payload.ExecutionWideSpreadMode != "" {
				overrides["executionWideSpreadMode"] = payload.ExecutionWideSpreadMode
			}
			if payload.ExecutionRestingTimeoutSeconds > 0 {
				overrides["executionRestingTimeoutSeconds"] = payload.ExecutionRestingTimeoutSeconds
			}
			if payload.ExecutionTimeoutFallbackOrderType != "" {
				overrides["executionTimeoutFallbackOrderType"] = payload.ExecutionTimeoutFallbackOrderType
			}
			if payload.ExecutionTimeoutFallbackTimeInForce != "" {
				overrides["executionTimeoutFallbackTimeInForce"] = payload.ExecutionTimeoutFallbackTimeInForce
			}
			if payload.ExecutionEntryOrderType != "" {
				overrides["executionEntryOrderType"] = payload.ExecutionEntryOrderType
			}
			if payload.ExecutionEntryTimeInForce != "" {
				overrides["executionEntryTimeInForce"] = payload.ExecutionEntryTimeInForce
			}
			if payload.ExecutionEntryPostOnly != nil {
				overrides["executionEntryPostOnly"] = *payload.ExecutionEntryPostOnly
			}
			if payload.ExecutionEntryMaxSpreadBps != nil {
				overrides["executionEntryMaxSpreadBps"] = payload.ExecutionEntryMaxSpreadBps
			}
			if payload.ExecutionEntryWideSpreadMode != "" {
				overrides["executionEntryWideSpreadMode"] = payload.ExecutionEntryWideSpreadMode
			}
			if payload.ExecutionEntryRestingTimeoutSeconds > 0 {
				overrides["executionEntryRestingTimeoutSeconds"] = payload.ExecutionEntryRestingTimeoutSeconds
			}
			if payload.ExecutionEntryTimeoutFallbackOrderType != "" {
				overrides["executionEntryTimeoutFallbackOrderType"] = payload.ExecutionEntryTimeoutFallbackOrderType
			}
			if payload.ExecutionEntryTimeoutFallbackTimeInForce != "" {
				overrides["executionEntryTimeoutFallbackTimeInForce"] = payload.ExecutionEntryTimeoutFallbackTimeInForce
			}
			if payload.ExecutionPTExitOrderType != "" {
				overrides["executionPTExitOrderType"] = payload.ExecutionPTExitOrderType
			}
			if payload.ExecutionPTExitTimeInForce != "" {
				overrides["executionPTExitTimeInForce"] = payload.ExecutionPTExitTimeInForce
			}
			if payload.ExecutionPTExitPostOnly != nil {
				overrides["executionPTExitPostOnly"] = *payload.ExecutionPTExitPostOnly
			}
			if payload.ExecutionPTExitMaxSpreadBps != nil {
				overrides["executionPTExitMaxSpreadBps"] = payload.ExecutionPTExitMaxSpreadBps
			}
			if payload.ExecutionPTExitWideSpreadMode != "" {
				overrides["executionPTExitWideSpreadMode"] = payload.ExecutionPTExitWideSpreadMode
			}
			if payload.ExecutionPTExitRestingTimeoutSeconds > 0 {
				overrides["executionPTExitRestingTimeoutSeconds"] = payload.ExecutionPTExitRestingTimeoutSeconds
			}
			if payload.ExecutionPTExitTimeoutFallbackOrderType != "" {
				overrides["executionPTExitTimeoutFallbackOrderType"] = payload.ExecutionPTExitTimeoutFallbackOrderType
			}
			if payload.ExecutionPTExitTimeoutFallbackTimeInForce != "" {
				overrides["executionPTExitTimeoutFallbackTimeInForce"] = payload.ExecutionPTExitTimeoutFallbackTimeInForce
			}
			if payload.ExecutionSLExitOrderType != "" {
				overrides["executionSLExitOrderType"] = payload.ExecutionSLExitOrderType
			}
			if payload.ExecutionSLExitTimeInForce != "" {
				overrides["executionSLExitTimeInForce"] = payload.ExecutionSLExitTimeInForce
			}
			if payload.ExecutionSLExitPostOnly != nil {
				overrides["executionSLExitPostOnly"] = *payload.ExecutionSLExitPostOnly
			}
			if payload.ExecutionSLExitMaxSpreadBps != nil {
				overrides["executionSLExitMaxSpreadBps"] = payload.ExecutionSLExitMaxSpreadBps
			}
			if payload.ExecutionSLExitWideSpreadMode != "" {
				overrides["executionSLExitWideSpreadMode"] = payload.ExecutionSLExitWideSpreadMode
			}
			if payload.ExecutionSLExitRestingTimeoutSeconds > 0 {
				overrides["executionSLExitRestingTimeoutSeconds"] = payload.ExecutionSLExitRestingTimeoutSeconds
			}
			if payload.ExecutionSLExitTimeoutFallbackOrderType != "" {
				overrides["executionSLExitTimeoutFallbackOrderType"] = payload.ExecutionSLExitTimeoutFallbackOrderType
			}
			if payload.ExecutionSLExitTimeoutFallbackTimeInForce != "" {
				overrides["executionSLExitTimeoutFallbackTimeInForce"] = payload.ExecutionSLExitTimeoutFallbackTimeInForce
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
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/live/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if r.Method == http.MethodPut {
			if len(parts) != 1 || parts[0] == "" {
				writeError(w, http.StatusNotFound, "live session route not found")
				return
			}
			var payload struct {
				AccountID                                 string `json:"accountId"`
				StrategyID                                string `json:"strategyId"`
				PositionSizingMode                        string `json:"positionSizingMode"`
				DefaultOrderFraction                      any    `json:"defaultOrderFraction"`
				SignalTimeframe                           string `json:"signalTimeframe"`
				ExecutionDataSource                       string `json:"executionDataSource"`
				ExecutionStrategy                         string `json:"executionStrategy"`
				ExecutionOrderType                        string `json:"executionOrderType"`
				ExecutionTimeInForce                      string `json:"executionTimeInForce"`
				ExecutionPostOnly                         *bool  `json:"executionPostOnly"`
				ExecutionMaxSpreadBps                     any    `json:"executionMaxSpreadBps"`
				ExecutionWideSpreadMode                   string `json:"executionWideSpreadMode"`
				ExecutionRestingTimeoutSeconds            int    `json:"executionRestingTimeoutSeconds"`
				ExecutionTimeoutFallbackOrderType         string `json:"executionTimeoutFallbackOrderType"`
				ExecutionTimeoutFallbackTimeInForce       string `json:"executionTimeoutFallbackTimeInForce"`
				ExecutionEntryOrderType                   string `json:"executionEntryOrderType"`
				ExecutionEntryTimeInForce                 string `json:"executionEntryTimeInForce"`
				ExecutionEntryPostOnly                    *bool  `json:"executionEntryPostOnly"`
				ExecutionEntryMaxSpreadBps                any    `json:"executionEntryMaxSpreadBps"`
				ExecutionEntryWideSpreadMode              string `json:"executionEntryWideSpreadMode"`
				ExecutionEntryRestingTimeoutSeconds       int    `json:"executionEntryRestingTimeoutSeconds"`
				ExecutionEntryTimeoutFallbackOrderType    string `json:"executionEntryTimeoutFallbackOrderType"`
				ExecutionEntryTimeoutFallbackTimeInForce  string `json:"executionEntryTimeoutFallbackTimeInForce"`
				ExecutionPTExitOrderType                  string `json:"executionPTExitOrderType"`
				ExecutionPTExitTimeInForce                string `json:"executionPTExitTimeInForce"`
				ExecutionPTExitPostOnly                   *bool  `json:"executionPTExitPostOnly"`
				ExecutionPTExitMaxSpreadBps               any    `json:"executionPTExitMaxSpreadBps"`
				ExecutionPTExitWideSpreadMode             string `json:"executionPTExitWideSpreadMode"`
				ExecutionPTExitRestingTimeoutSeconds      int    `json:"executionPTExitRestingTimeoutSeconds"`
				ExecutionPTExitTimeoutFallbackOrderType   string `json:"executionPTExitTimeoutFallbackOrderType"`
				ExecutionPTExitTimeoutFallbackTimeInForce string `json:"executionPTExitTimeoutFallbackTimeInForce"`
				ExecutionSLExitOrderType                  string `json:"executionSLExitOrderType"`
				ExecutionSLExitTimeInForce                string `json:"executionSLExitTimeInForce"`
				ExecutionSLExitPostOnly                   *bool  `json:"executionSLExitPostOnly"`
				ExecutionSLExitMaxSpreadBps               any    `json:"executionSLExitMaxSpreadBps"`
				ExecutionSLExitWideSpreadMode             string `json:"executionSLExitWideSpreadMode"`
				ExecutionSLExitRestingTimeoutSeconds      int    `json:"executionSLExitRestingTimeoutSeconds"`
				ExecutionSLExitTimeoutFallbackOrderType   string `json:"executionSLExitTimeoutFallbackOrderType"`
				ExecutionSLExitTimeoutFallbackTimeInForce string `json:"executionSLExitTimeoutFallbackTimeInForce"`
				Symbol                                    string `json:"symbol"`
				From                                      string `json:"from"`
				To                                        string `json:"to"`
				StrategyEngine                            string `json:"strategyEngine"`
				DefaultOrderQty                           any    `json:"defaultOrderQuantity"`
				DispatchMode                              string `json:"dispatchMode"`
				DispatchCooldownSec                       int    `json:"dispatchCooldownSeconds"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			overrides := map[string]any{}
			if payload.SignalTimeframe != "" {
				overrides["signalTimeframe"] = payload.SignalTimeframe
			}
			if payload.PositionSizingMode != "" {
				overrides["positionSizingMode"] = payload.PositionSizingMode
			}
			if payload.DefaultOrderFraction != nil {
				overrides["defaultOrderFraction"] = payload.DefaultOrderFraction
			}
			if payload.ExecutionDataSource != "" {
				overrides["executionDataSource"] = payload.ExecutionDataSource
			}
			if payload.ExecutionStrategy != "" {
				overrides["executionStrategy"] = payload.ExecutionStrategy
			}
			if payload.ExecutionOrderType != "" {
				overrides["executionOrderType"] = payload.ExecutionOrderType
			}
			if payload.ExecutionTimeInForce != "" {
				overrides["executionTimeInForce"] = payload.ExecutionTimeInForce
			}
			if payload.ExecutionPostOnly != nil {
				overrides["executionPostOnly"] = *payload.ExecutionPostOnly
			}
			if payload.ExecutionMaxSpreadBps != nil {
				overrides["executionMaxSpreadBps"] = payload.ExecutionMaxSpreadBps
			}
			if payload.ExecutionWideSpreadMode != "" {
				overrides["executionWideSpreadMode"] = payload.ExecutionWideSpreadMode
			}
			if payload.ExecutionRestingTimeoutSeconds > 0 {
				overrides["executionRestingTimeoutSeconds"] = payload.ExecutionRestingTimeoutSeconds
			}
			if payload.ExecutionTimeoutFallbackOrderType != "" {
				overrides["executionTimeoutFallbackOrderType"] = payload.ExecutionTimeoutFallbackOrderType
			}
			if payload.ExecutionTimeoutFallbackTimeInForce != "" {
				overrides["executionTimeoutFallbackTimeInForce"] = payload.ExecutionTimeoutFallbackTimeInForce
			}
			if payload.ExecutionEntryOrderType != "" {
				overrides["executionEntryOrderType"] = payload.ExecutionEntryOrderType
			}
			if payload.ExecutionEntryTimeInForce != "" {
				overrides["executionEntryTimeInForce"] = payload.ExecutionEntryTimeInForce
			}
			if payload.ExecutionEntryPostOnly != nil {
				overrides["executionEntryPostOnly"] = *payload.ExecutionEntryPostOnly
			}
			if payload.ExecutionEntryMaxSpreadBps != nil {
				overrides["executionEntryMaxSpreadBps"] = payload.ExecutionEntryMaxSpreadBps
			}
			if payload.ExecutionEntryWideSpreadMode != "" {
				overrides["executionEntryWideSpreadMode"] = payload.ExecutionEntryWideSpreadMode
			}
			if payload.ExecutionEntryRestingTimeoutSeconds > 0 {
				overrides["executionEntryRestingTimeoutSeconds"] = payload.ExecutionEntryRestingTimeoutSeconds
			}
			if payload.ExecutionEntryTimeoutFallbackOrderType != "" {
				overrides["executionEntryTimeoutFallbackOrderType"] = payload.ExecutionEntryTimeoutFallbackOrderType
			}
			if payload.ExecutionEntryTimeoutFallbackTimeInForce != "" {
				overrides["executionEntryTimeoutFallbackTimeInForce"] = payload.ExecutionEntryTimeoutFallbackTimeInForce
			}
			if payload.ExecutionPTExitOrderType != "" {
				overrides["executionPTExitOrderType"] = payload.ExecutionPTExitOrderType
			}
			if payload.ExecutionPTExitTimeInForce != "" {
				overrides["executionPTExitTimeInForce"] = payload.ExecutionPTExitTimeInForce
			}
			if payload.ExecutionPTExitPostOnly != nil {
				overrides["executionPTExitPostOnly"] = *payload.ExecutionPTExitPostOnly
			}
			if payload.ExecutionPTExitMaxSpreadBps != nil {
				overrides["executionPTExitMaxSpreadBps"] = payload.ExecutionPTExitMaxSpreadBps
			}
			if payload.ExecutionPTExitWideSpreadMode != "" {
				overrides["executionPTExitWideSpreadMode"] = payload.ExecutionPTExitWideSpreadMode
			}
			if payload.ExecutionPTExitRestingTimeoutSeconds > 0 {
				overrides["executionPTExitRestingTimeoutSeconds"] = payload.ExecutionPTExitRestingTimeoutSeconds
			}
			if payload.ExecutionPTExitTimeoutFallbackOrderType != "" {
				overrides["executionPTExitTimeoutFallbackOrderType"] = payload.ExecutionPTExitTimeoutFallbackOrderType
			}
			if payload.ExecutionPTExitTimeoutFallbackTimeInForce != "" {
				overrides["executionPTExitTimeoutFallbackTimeInForce"] = payload.ExecutionPTExitTimeoutFallbackTimeInForce
			}
			if payload.ExecutionSLExitOrderType != "" {
				overrides["executionSLExitOrderType"] = payload.ExecutionSLExitOrderType
			}
			if payload.ExecutionSLExitTimeInForce != "" {
				overrides["executionSLExitTimeInForce"] = payload.ExecutionSLExitTimeInForce
			}
			if payload.ExecutionSLExitPostOnly != nil {
				overrides["executionSLExitPostOnly"] = *payload.ExecutionSLExitPostOnly
			}
			if payload.ExecutionSLExitMaxSpreadBps != nil {
				overrides["executionSLExitMaxSpreadBps"] = payload.ExecutionSLExitMaxSpreadBps
			}
			if payload.ExecutionSLExitWideSpreadMode != "" {
				overrides["executionSLExitWideSpreadMode"] = payload.ExecutionSLExitWideSpreadMode
			}
			if payload.ExecutionSLExitRestingTimeoutSeconds > 0 {
				overrides["executionSLExitRestingTimeoutSeconds"] = payload.ExecutionSLExitRestingTimeoutSeconds
			}
			if payload.ExecutionSLExitTimeoutFallbackOrderType != "" {
				overrides["executionSLExitTimeoutFallbackOrderType"] = payload.ExecutionSLExitTimeoutFallbackOrderType
			}
			if payload.ExecutionSLExitTimeoutFallbackTimeInForce != "" {
				overrides["executionSLExitTimeoutFallbackTimeInForce"] = payload.ExecutionSLExitTimeoutFallbackTimeInForce
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
			item, err := platform.UpdateLiveSession(parts[0], payload.AccountID, payload.StrategyID, overrides)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
			return
		}
		if r.Method == http.MethodDelete {
			if len(parts) != 1 || parts[0] == "" {
				writeError(w, http.StatusNotFound, "live session route not found")
				return
			}
			if err := platform.DeleteLiveSessionWithForce(parts[0], queryFlagEnabled(r, "force")); err != nil {
				if errors.Is(err, service.ErrActivePositionsOrOrders) {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "sessionId": parts[0]})
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
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
			item, err := platform.StopLiveSessionWithForce(sessionID, queryFlagEnabled(r, "force"))
			if err != nil {
				if errors.Is(err, service.ErrActivePositionsOrOrders) {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
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
		case "sync":
			item, err := platform.SyncLiveSession(sessionID)
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
