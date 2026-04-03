package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerAccountRoutes 注册账户管理、账户汇总、净值快照和持仓相关路由。
func registerAccountRoutes(mux *http.ServeMux, platform *service.Platform) {
	// GET|POST /api/v1/accounts — 账户列表/创建
	mux.HandleFunc("/api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListAccounts()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				Name     string `json:"name"`
				Mode     string `json:"mode"`
				Exchange string `json:"exchange"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{
				"name":     payload.Name,
				"mode":     payload.Mode,
				"exchange": payload.Exchange,
			}, "name", "mode", "exchange"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.CreateAccount(payload.Name, payload.Mode, payload.Exchange)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// GET /api/v1/account-summaries — 账户汇总（权益、PnL、费用、敞口）
	mux.HandleFunc("/api/v1/account-summaries", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		items, err := platform.ListAccountSummaries()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	// GET /api/v1/account-equity-snapshots — 账户净值快照时间序列
	mux.HandleFunc("/api/v1/account-equity-snapshots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		accountID := r.URL.Query().Get("accountId")
		if accountID == "" {
			writeError(w, http.StatusBadRequest, "accountId is required")
			return
		}
		items, err := platform.ListAccountEquitySnapshots(accountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	// GET /api/v1/positions — 当前持仓列表
	mux.HandleFunc("/api/v1/positions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		items, err := platform.ListPositions()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
}
