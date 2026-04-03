package http

import (
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerPaperRoutes 注册模拟交易会话相关路由。
func registerPaperRoutes(mux *http.ServeMux, platform *service.Platform) {
	// GET|POST /api/v1/paper/sessions — 会话列表/创建
	mux.HandleFunc("/api/v1/paper/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListPaperSessions()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
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
				payload.StartEquity = 100000 // 默认 10 万初始权益
			}
			item, err := platform.CreatePaperSession(payload.AccountID, payload.StrategyID, payload.StartEquity)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// POST /api/v1/paper/sessions/{id}/{action} — 控制会话（start/stop/tick）
	mux.HandleFunc("/api/v1/paper/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/paper/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "模拟交易会话操作未找到")
			return
		}

		sessionID := parts[0]
		action := parts[1]

		switch action {
		case "start":
			// 启动会话后台回放
			item, err := platform.StartPaperSession(sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "stop":
			// 停止会话回放
			item, err := platform.StopPaperSession(sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "tick":
			// 手动推进一步
			item, err := platform.TickPaperSession(sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			writeError(w, http.StatusNotFound, "不支持的会话操作")
		}
	})
}
