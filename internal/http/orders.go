package http

import (
	"net/http"
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
			items, err := platform.ListOrders()
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

		// GET /api/v1/orders/{id}
		if r.Method == http.MethodGet {
			if len(parts) != 1 || parts[0] == "" {
				writeError(w, http.StatusNotFound, "order route not found")
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

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if len(parts) != 2 {
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
		items, err := platform.ListFills()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}
