package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func handleLiveAccountRecoveryRoute(w http.ResponseWriter, r *http.Request, platform *service.Platform, parts []string) {
	// 路由匹配：/api/v1/live/accounts/:id/recovery/:action
	if len(parts) < 3 || parts[0] == "" || parts[1] != "recovery" || parts[2] == "" {
		writeError(w, http.StatusNotFound, "unsupported live recovery route")
		return
	}

	accountID := parts[0]
	subAction := parts[2]

	switch subAction {
	case "diagnose":
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var options service.LiveRecoveryDiagnoseOptions
		if r.Method == http.MethodPost {
			if err := decodeJSON(r, &options); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		} else {
			query := r.URL.Query()
			options.Symbol = query.Get("symbol")
			options.SessionID = query.Get("sessionId")
			if lookbackRaw := strings.TrimSpace(query.Get("lookbackHours")); lookbackRaw != "" {
				lookback, err := strconv.Atoi(lookbackRaw)
				if err != nil || lookback <= 0 {
					writeError(w, http.StatusBadRequest, "invalid lookbackHours")
					return
				}
				options.LookbackHours = lookback
			}
		}
		options.AccountID = accountID

		result, err := platform.DiagnoseLiveRecovery(r.Context(), options)
		if err != nil {
			if strings.Contains(err.Error(), "account not found:") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
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
		if strings.TrimSpace(req.Action) == "" {
			writeError(w, http.StatusBadRequest, "action is required")
			return
		}
		if req.Payload == nil {
			req.Payload = map[string]any{}
		}

		result, err := platform.ExecuteLiveRecoveryAction(r.Context(), accountID, req.Action, req.Payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)

	default:
		writeError(w, http.StatusNotFound, fmt.Sprintf("unsupported recovery sub-action: %s", subAction))
	}
}
