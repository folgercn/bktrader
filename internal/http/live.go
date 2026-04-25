package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerLiveRoutes(mux *http.ServeMux, platform *service.Platform, cfg config.Config) {
	mux.HandleFunc("/api/v1/live/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			view := r.URL.Query().Get("view")
			var items []domain.LiveSession
			var err error
			if view == "summary" {
				items, err = platform.ListLiveSessionsSummary()
			} else {
				items, err = platform.ListLiveSessions()
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				Alias                                     string `json:"alias"`
				AccountID                                 string `json:"accountId"`
				StrategyID                                string `json:"strategyId"`
				PositionSizingMode                        string `json:"positionSizingMode"`
				DefaultOrderFraction                      any    `json:"defaultOrderFraction"`
				ReentrySizeSchedule                       any    `json:"reentry_size_schedule"`
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
				FreshnessOverrideSignalBarFreshnessSecs   any    `json:"freshnessOverrideSignalBarFreshnessSeconds"`
				FreshnessOverrideTradeTickFreshnessSecs   any    `json:"freshnessOverrideTradeTickFreshnessSeconds"`
				FreshnessOverrideOrderBookFreshnessSecs   any    `json:"freshnessOverrideOrderBookFreshnessSeconds"`
				FreshnessOverrideRuntimeQuietSecs         any    `json:"freshnessOverrideRuntimeQuietSeconds"`
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
			if payload.ReentrySizeSchedule != nil {
				overrides["reentry_size_schedule"] = payload.ReentrySizeSchedule
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

			// 处理新鲜度覆盖
			freshnessOverride := map[string]any{}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideSignalBarFreshnessSecs); ok && val > 0 {
				freshnessOverride["signalBarFreshnessSeconds"] = val
			}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideTradeTickFreshnessSecs); ok && val > 0 {
				freshnessOverride["tradeTickFreshnessSeconds"] = val
			}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideOrderBookFreshnessSecs); ok && val > 0 {
				freshnessOverride["orderBookFreshnessSeconds"] = val
			}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideRuntimeQuietSecs); ok && val > 0 {
				freshnessOverride["runtimeQuietSeconds"] = val
			}
			if len(freshnessOverride) > 0 {
				overrides["freshnessOverride"] = freshnessOverride
			}

			item, err := platform.CreateLiveSession(payload.Alias, payload.AccountID, payload.StrategyID, overrides)
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
		if r.Method == http.MethodGet {
			if len(parts) == 2 && parts[0] != "" && parts[1] == "trade-pairs" {
				limit, err := parseOptionalPositiveInt(r.URL.Query().Get("limit"))
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid limit")
					return
				}
				items, err := platform.ListLiveTradePairs(domain.LiveTradePairQuery{
					LiveSessionID: parts[0],
					Status:        r.URL.Query().Get("status"),
					Limit:         limit,
				})
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, items)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodPut {
			if len(parts) != 1 || parts[0] == "" {
				writeError(w, http.StatusNotFound, "live session route not found")
				return
			}
			var payload struct {
				Alias                                     string `json:"alias"`
				AccountID                                 string `json:"accountId"`
				StrategyID                                string `json:"strategyId"`
				PositionSizingMode                        string `json:"positionSizingMode"`
				DefaultOrderFraction                      any    `json:"defaultOrderFraction"`
				ReentrySizeSchedule                       any    `json:"reentry_size_schedule"`
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
				FreshnessOverrideSignalBarFreshnessSecs   any    `json:"freshnessOverrideSignalBarFreshnessSeconds"`
				FreshnessOverrideTradeTickFreshnessSecs   any    `json:"freshnessOverrideTradeTickFreshnessSeconds"`
				FreshnessOverrideOrderBookFreshnessSecs   any    `json:"freshnessOverrideOrderBookFreshnessSeconds"`
				FreshnessOverrideRuntimeQuietSecs         any    `json:"freshnessOverrideRuntimeQuietSeconds"`
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
			if payload.ReentrySizeSchedule != nil {
				overrides["reentry_size_schedule"] = payload.ReentrySizeSchedule
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

			// 处理新鲜度覆盖
			freshnessOverride := map[string]any{}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideSignalBarFreshnessSecs); ok && val > 0 {
				freshnessOverride["signalBarFreshnessSeconds"] = val
			} else if payload.FreshnessOverrideSignalBarFreshnessSecs != nil {
				freshnessOverride["signalBarFreshnessSeconds"] = nil
			}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideTradeTickFreshnessSecs); ok && val > 0 {
				freshnessOverride["tradeTickFreshnessSeconds"] = val
			} else if payload.FreshnessOverrideTradeTickFreshnessSecs != nil {
				freshnessOverride["tradeTickFreshnessSeconds"] = nil
			}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideOrderBookFreshnessSecs); ok && val > 0 {
				freshnessOverride["orderBookFreshnessSeconds"] = val
			} else if payload.FreshnessOverrideOrderBookFreshnessSecs != nil {
				freshnessOverride["orderBookFreshnessSeconds"] = nil
			}
			if val, ok := service.ToFloat64(payload.FreshnessOverrideRuntimeQuietSecs); ok && val > 0 {
				freshnessOverride["runtimeQuietSeconds"] = val
			} else if payload.FreshnessOverrideRuntimeQuietSecs != nil {
				freshnessOverride["runtimeQuietSeconds"] = nil
			}
			if len(freshnessOverride) > 0 {
				overrides["freshnessOverride"] = freshnessOverride
			}

			item, err := platform.UpdateLiveSession(parts[0], payload.Alias, payload.AccountID, payload.StrategyID, overrides)
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
		sessionID := parts[0]
		if len(parts) == 4 && parts[1] == "orders" && parts[3] == "verifications" {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			orderID := parts[2]
			var payload struct {
				Notes string `json:"notes"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			// 获取订单详情以补全核验记录所需信息
			order, err := platform.GetOrder(orderID)
			if err != nil {
				writeError(w, http.StatusNotFound, "order not found: "+err.Error())
				return
			}

			// 获取会话以补全 StrategyID
			session, err := platform.GetLiveSession(sessionID)
			if err != nil {
				writeError(w, http.StatusNotFound, "live session not found: "+err.Error())
				return
			}

			// 安全校验 1：归属确认 (Metadata 标识)
			// 注意：domain.Order 本身不直接持有 LiveSessionID 字段，关联关系存储在 Metadata 中
			orderSessionID, _ := order.Metadata["liveSessionId"].(string)
			if orderSessionID != sessionID {
				writeError(w, http.StatusForbidden, "order does not belong to this session (metadata mismatch)")
				return
			}

			// 业务校验：仅允许复核退出类订单
			if !order.EffectiveReduceOnly() && !order.EffectiveClosePosition() {
				writeError(w, http.StatusForbidden, "only exit/reduce-only orders can be manually verified")
				return
			}

			// 状态校验：仅允许复核 mismatch 或 orphan-exit 状态的交易对
			pairs, err := platform.ListLiveTradePairs(domain.LiveTradePairQuery{LiveSessionID: sessionID})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to analyze trade pairs: "+err.Error())
				return
			}

			var targetPair *domain.LiveTradePair
			for _, p := range pairs {
				for _, eid := range p.ExitOrderIDs {
					if eid == orderID {
						targetPair = &p
						break
					}
				}
				if targetPair != nil {
					break
				}
			}

			if targetPair == nil {
				writeError(w, http.StatusNotFound, "trade pair containing this exit order not found")
				return
			}

			verdict := strings.ToLower(targetPair.ExitVerdict)
			if verdict != "mismatch" && verdict != "orphan-exit" {
				writeError(w, http.StatusForbidden, fmt.Sprintf("manual verification only allowed for mismatch/orphan-exit states (current: %s)", verdict))
				return
			}

			// 创建核验记录
			verification := domain.OrderCloseVerification{
				ID:                   fmt.Sprintf("manual-%d", time.Now().UnixNano()),
				LiveSessionID:        sessionID,
				OrderID:              orderID,
				AccountID:            order.AccountID,
				StrategyID:           session.StrategyID,
				Symbol:               order.Symbol,
				VerifiedClosed:       true,
				RemainingPositionQty: 0,
				VerificationSource:   "manual-review",
				EventTime:            time.Now().UTC(),
				RecordedAt:           time.Now().UTC(),
				Metadata: map[string]any{
					"notes": payload.Notes,
				},
			}

			if _, err := platform.CreateOrderCloseVerification(verification); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			writeJSON(w, http.StatusCreated, verification)
			return
		}

		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "live session route not found")
			return
		}

		action := parts[1]
		switch action {
		case "start":
			if !cfg.RuntimeActionsEnabled() {
				writeError(w, http.StatusConflict, "runtime action start is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
				return
			}
			item, err := platform.StartLiveSession(sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "stop":
			if !cfg.RuntimeActionsEnabled() {
				writeError(w, http.StatusConflict, "runtime action stop is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
				return
			}
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
			if !cfg.RuntimeActionsEnabled() {
				writeError(w, http.StatusConflict, "runtime action dispatch is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
				return
			}
			item, err := platform.DispatchLiveSessionIntent(sessionID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "sync":
			if !cfg.RuntimeActionsEnabled() {
				writeError(w, http.StatusConflict, "runtime action sync is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
				return
			}
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
