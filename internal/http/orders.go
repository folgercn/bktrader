package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerOrderRoutes 注册订单和成交记录相关路由。
func registerOrderRoutes(mux *http.ServeMux, platform *service.Platform) {
	// GET|POST /api/v1/orders — 订单列表/下单
	mux.HandleFunc("/api/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			limit := 500
			if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
				if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
					limit = parsed
					if limit > 2000 {
						limit = 2000
					}
				}
			}
			items, err := platform.ListOrdersWithLimit(limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				AccountID         string         `json:"accountId"`
				StrategyVersionID string         `json:"strategyVersionId"`
				Symbol            string         `json:"symbol"`
				Side              string         `json:"side"`
				Type              string         `json:"type"`
				Quantity          float64        `json:"quantity"`
				Price             float64        `json:"price"`
				ReduceOnly        bool           `json:"reduceOnly"`
				ClosePosition     bool           `json:"closePosition"`
				Metadata          map[string]any `json:"metadata"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{
				"accountId": payload.AccountID,
				"symbol":    payload.Symbol,
				"side":      payload.Side,
				"type":      payload.Type,
			}, "accountId", "symbol", "side", "type"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			order := domain.Order{
				AccountID:         payload.AccountID,
				StrategyVersionID: payload.StrategyVersionID,
				Symbol:            service.NormalizeSymbol(payload.Symbol),
				Side:              payload.Side,
				Type:              payload.Type,
				Quantity:          payload.Quantity,
				Price:             payload.Price,
				ReduceOnly:        payload.ReduceOnly,
				ClosePosition:     payload.ClosePosition,
				Metadata:          payload.Metadata,
			}
			item, err := platform.CreateOrder(order)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/orders/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/orders/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 1 {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			item, err := platform.GetOrder(parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
			return
		}
		if r.Method != http.MethodPost || len(parts) != 2 {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			writeError(w, http.StatusNotFound, "order route not found")
			return
		}
		switch parts[1] {
		case "sync":
			item, err := platform.SyncLiveOrder(parts[0])
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "cancel":
			item, err := platform.CancelLiveOrder(parts[0])
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			writeError(w, http.StatusNotFound, "order route not found")
		}
	})

	// GET /api/v1/fills — 成交流水列表
	mux.HandleFunc("/api/v1/fills", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		limit := 500
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
				if limit > 2000 {
					limit = 2000
				}
			}
		}
		items, err := platform.ListFillsWithLimit(limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}
