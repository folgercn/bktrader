package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
)

type runtimeRestartRequest struct {
	RuntimeID   string `json:"runtimeId"`
	RuntimeKind string `json:"runtimeKind"`
	Force       bool   `json:"force"`
	Confirm     bool   `json:"confirm"`
	Reason      string `json:"reason,omitempty"`
}

func registerRuntimeControlRoutes(mux *http.ServeMux, platform *service.Platform, cfg config.Config) {
	mux.HandleFunc("/api/v1/runtime/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !cfg.RuntimeActionsEnabled() {
			writeError(w, http.StatusConflict, "runtime action restart is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
			return
		}
		var request runtimeRestartRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		runtimeID := strings.TrimSpace(request.RuntimeID)
		if runtimeID == "" {
			writeError(w, http.StatusBadRequest, "runtimeId is required")
			return
		}
		if !request.Confirm {
			writeError(w, http.StatusBadRequest, "confirm=true is required for runtime restart")
			return
		}
		reason := strings.TrimSpace(request.Reason)
		if request.Force && reason == "" {
			writeError(w, http.StatusBadRequest, "reason is required when force=true")
			return
		}
		switch strings.ToLower(strings.TrimSpace(request.RuntimeKind)) {
		case "signal", "signal-runtime":
			item, err := platform.RestartSignalRuntimeSessionWithOptions(runtimeID, service.SignalRuntimeRestartOptions{
				Force:  request.Force,
				Reason: reason,
				Source: "api",
			})
			if err != nil {
				writeRuntimeControlError(w, err)
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]any{
				"status":        "accepted",
				"message":       "runtime restart intent accepted; execution is asynchronous and must be confirmed through actualStatus",
				"runtimeId":     item.ID,
				"runtimeKind":   "signal",
				"desiredStatus": item.State["desiredStatus"],
				"actualStatus":  item.State["actualStatus"],
				"force":         request.Force,
				"reason":        reason,
				"runtime":       item,
			})
		case "":
			writeError(w, http.StatusBadRequest, "runtimeKind is required")
		default:
			writeError(w, http.StatusBadRequest, "unsupported runtimeKind: "+request.RuntimeKind)
		}
	})
}

func writeRuntimeControlError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLiveControlOperationInProgress),
		errors.Is(err, service.ErrLiveAccountOperationInProgress),
		errors.Is(err, service.ErrRuntimeLeaseNotAcquired):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrActivePositionsOrOrders):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
