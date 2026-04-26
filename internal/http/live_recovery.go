package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerLiveRecoveryRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/live/accounts/", func(w http.ResponseWriter, r *http.Request) {
		// 路由匹配：/api/v1/live/accounts/:id/recovery/:action
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/live/accounts/")
		parts := strings.Split(strings.Trim(path, "/"), "/")

		// 我们只处理 recovery 相关的子路由
		if len(parts) < 2 || parts[1] != "recovery" {
			// 这里不处理，交给 registerAccountRoutes 处理（如果它也被注册到同一个前缀）
			// 但因为 mux.HandleFunc 会精确匹配或最长前缀匹配，我们需要小心。
			// 实际上 registerAccountRoutes 可能已经注册了 /api/v1/accounts/。
			// 这里的路径是 /api/v1/live/accounts/，是 live 模块下的恢复逻辑。
			return
		}

		accountID := parts[0]
		subAction := parts[2] // diagnose 或 execute

		switch subAction {
		case "diagnose":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var options service.LiveRecoveryDiagnoseOptions
			if err := decodeJSON(r, &options); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			options.AccountID = accountID

			result, err := platform.DiagnoseLiveRecovery(r.Context(), options)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, result)

		case "execute":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Action  string         `json:"action"`
				Payload map[string]any `json:"payload"`
			}
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			result, err := platform.ExecuteLiveRecoveryAction(r.Context(), accountID, req.Action, req.Payload)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, result)

		default:
			writeError(w, http.StatusNotFound, fmt.Sprintf("unsupported recovery sub-action: %s", subAction))
		}
	})
}
