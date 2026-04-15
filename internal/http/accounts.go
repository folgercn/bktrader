package http

import (
	"net/http"
	"strings"

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

	mux.HandleFunc("/api/v1/live-adapters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.LiveAdapters())
	})

	mux.HandleFunc("/api/v1/live/launch-templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		items, err := platform.LiveLaunchTemplates()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/v1/live/accounts/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/live/accounts/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "live account route not found")
			return
		}
		accountID := parts[0]
		action := parts[1]
		if r.Method == http.MethodGet {
			account, err := platform.GetAccount(accountID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			snapshot := map[string]any{}
			if account.Metadata != nil {
				if resolved, ok := account.Metadata["liveSyncSnapshot"].(map[string]any); ok {
					snapshot = resolved
				}
			}
			switch action {
			case "positions":
				writeJSON(w, http.StatusOK, snapshot["positions"])
			case "open-orders":
				writeJSON(w, http.StatusOK, snapshot["openOrders"])
			default:
				writeError(w, http.StatusNotFound, "live account route not found")
			}
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch action {
		case "binding":
			var payload map[string]any
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.BindLiveAccount(accountID, payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "sync":
			item, err := platform.SyncLiveAccount(accountID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "launch":
			var payload service.LiveLaunchOptions
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.LaunchLiveFlow(accountID, payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			writeError(w, http.StatusNotFound, "live account route not found")
		}
	})

	mux.HandleFunc("/api/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/accounts/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "signal-bindings" {
			writeError(w, http.StatusNotFound, "account signal binding route not found")
			return
		}
		accountID := parts[0]
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListAccountSignalBindings(accountID)
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
			item, err := platform.BindAccountSignalSource(accountID, payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case http.MethodDelete:
			bindingID := r.URL.Query().Get("id")
			if bindingID == "" {
				writeError(w, http.StatusBadRequest, "binding id is required via ?id=")
				return
			}
			item, found, err := platform.UnbindAccountSignalSource(accountID, bindingID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if !found {
				writeError(w, http.StatusNotFound, "binding not found")
				return
			}
			writeJSON(w, http.StatusOK, item)
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
