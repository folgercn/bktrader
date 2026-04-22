package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

type runtimePolicyPatch struct {
	TradeTickFreshnessSeconds      *int `json:"tradeTickFreshnessSeconds"`
	OrderBookFreshnessSeconds      *int `json:"orderBookFreshnessSeconds"`
	SignalBarFreshnessSeconds      *int `json:"signalBarFreshnessSeconds"`
	RuntimeQuietSeconds            *int `json:"runtimeQuietSeconds"`
	StrategyEvaluationQuietSeconds *int `json:"strategyEvaluationQuietSeconds"`
	LiveAccountSyncFreshnessSecs   *int `json:"liveAccountSyncFreshnessSeconds"`
	PaperStartReadinessTimeoutSecs *int `json:"paperStartReadinessTimeoutSeconds"`
}

// registerSignalRoutes 注册信号源相关路由。
func registerSignalRoutes(mux *http.ServeMux, platform *service.Platform) {
	// GET /api/v1/signal-sources — 获取信号源列表
	mux.HandleFunc("/api/v1/signal-sources", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, platform.SignalSourceCatalog())
	})

	mux.HandleFunc("/api/v1/signal-source-types", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.SignalSourceTypes())
	})

	mux.HandleFunc("/api/v1/signal-runtime/adapters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.SignalRuntimeAdapters())
	})

	mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		items, err := platform.ListAlerts()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/v1/monitor/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		snapshot, err := platform.HealthSnapshot()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})

	mux.HandleFunc("/api/v1/notifications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		includeAcked := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeAcked")), "true")
		items, err := platform.ListNotifications(includeAcked)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/v1/notifications/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/notifications/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 2 && parts[1] == "ack" {
			notificationID := parts[0]
			switch r.Method {
			case http.MethodPost:
				item, err := platform.AckNotification(notificationID)
				if err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			case http.MethodDelete:
				if err := platform.UnackNotification(notificationID); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}
		if len(parts) == 2 && parts[1] == "telegram" {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if err := platform.SendNotificationToTelegram(parts[0]); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
			return
		}
		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "notification route not found")
			return
		}
		writeError(w, http.StatusNotFound, "notification route not found")
	})

	mux.HandleFunc("/api/v1/telegram/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.TelegramConfigView())
		case http.MethodPost:
			var payload struct {
				Enabled                       bool     `json:"enabled"`
				BotToken                      string   `json:"botToken"`
				ChatID                        string   `json:"chatId"`
				SendLevels                    []string `json:"sendLevels"`
				TradeEventsEnabled            bool     `json:"tradeEventsEnabled"`
				PositionReportEnabled         bool     `json:"positionReportEnabled"`
				PositionReportIntervalMinutes int      `json:"positionReportIntervalMinutes"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.UpdateTelegramConfig(payload.Enabled, payload.BotToken, payload.ChatID, payload.SendLevels, payload.TradeEventsEnabled, payload.PositionReportEnabled, payload.PositionReportIntervalMinutes)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/telegram/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := platform.SendTelegramTestMessage(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
	})

	mux.HandleFunc("/api/v1/runtime-policy", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.RuntimePolicy())
		case http.MethodPost:
			var payload runtimePolicyPatch
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			current := platform.RuntimePolicy()
			if payload.TradeTickFreshnessSeconds != nil {
				current.TradeTickFreshnessSeconds = *payload.TradeTickFreshnessSeconds
			}
			if payload.OrderBookFreshnessSeconds != nil {
				current.OrderBookFreshnessSeconds = *payload.OrderBookFreshnessSeconds
			}
			if payload.SignalBarFreshnessSeconds != nil {
				current.SignalBarFreshnessSeconds = *payload.SignalBarFreshnessSeconds
			}
			if payload.RuntimeQuietSeconds != nil {
				current.RuntimeQuietSeconds = *payload.RuntimeQuietSeconds
			}
			if payload.StrategyEvaluationQuietSeconds != nil {
				current.StrategyEvaluationQuietSeconds = *payload.StrategyEvaluationQuietSeconds
			}
			if payload.LiveAccountSyncFreshnessSecs != nil {
				current.LiveAccountSyncFreshnessSecs = *payload.LiveAccountSyncFreshnessSecs
			}
			if payload.PaperStartReadinessTimeoutSecs != nil {
				current.PaperStartReadinessTimeoutSecs = *payload.PaperStartReadinessTimeoutSecs
			}
			policy, err := platform.UpdateRuntimePolicy(current)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid runtime policy payload")
				return
			}
			writeJSON(w, http.StatusOK, policy)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/signal-runtime/plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		accountID := r.URL.Query().Get("accountId")
		strategyID := r.URL.Query().Get("strategyId")
		if accountID == "" || strategyID == "" {
			writeError(w, http.StatusBadRequest, "accountId and strategyId are required")
			return
		}
		plan, err := platform.BuildSignalRuntimePlan(accountID, strategyID)
		if err != nil {
			if strings.Contains(err.Error(), "strategy not found:") {
				writeJSON(w, http.StatusOK, map[string]any{
					"accountId":        accountID,
					"strategyId":       strategyID,
					"requiredBindings": []any{},
					"matchedBindings":  []any{},
					"missingBindings":  []any{},
					"subscriptions":    []any{},
					"ready":            false,
					"notes":            []string{"strategy not found; likely stale session or form state"},
				})
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, plan)
	})

	mux.HandleFunc("/api/v1/signal-runtime/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.ListSignalRuntimeSessions())
		case http.MethodPost:
			var payload struct {
				AccountID  string `json:"accountId"`
				StrategyID string `json:"strategyId"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if payload.AccountID == "" || payload.StrategyID == "" {
				writeError(w, http.StatusBadRequest, "accountId and strategyId are required")
				return
			}
			item, err := platform.CreateSignalRuntimeSession(payload.AccountID, payload.StrategyID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/signal-runtime/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/signal-runtime/sessions/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				item, err := platform.GetSignalRuntimeSession(parts[0])
				if err != nil {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			case http.MethodDelete:
				if err := platform.DeleteSignalRuntimeSessionWithForce(parts[0], queryFlagEnabled(r, "force")); err != nil {
					if errors.Is(err, service.ErrActivePositionsOrOrders) {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}

		if len(parts) != 2 {
			writeError(w, http.StatusNotFound, "signal runtime session route not found")
			return
		}
		sessionID, action := parts[0], parts[1]
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch action {
		case "start":
			item, err := platform.StartSignalRuntimeSession(sessionID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		case "stop":
			item, err := platform.StopSignalRuntimeSessionWithForce(sessionID, queryFlagEnabled(r, "force"))
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, item)
		default:
			writeError(w, http.StatusNotFound, "signal runtime session action not found")
		}
	})
}
