package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/service"
)

type runtimeRestartRequest struct {
	RuntimeID   string `json:"runtimeId"`
	RuntimeKind string `json:"runtimeKind"`
	Force       bool   `json:"force"`
	Confirm     bool   `json:"confirm"`
	Reason      string `json:"reason,omitempty"`
}

type runtimeLifecycleControlRequest struct {
	RuntimeID   string `json:"runtimeId"`
	RuntimeKind string `json:"runtimeKind"`
	Force       bool   `json:"force,omitempty"`
	Confirm     bool   `json:"confirm"`
	Reason      string `json:"reason"`
}

type runtimeAutoRestartControlRequest struct {
	RuntimeID   string `json:"runtimeId"`
	RuntimeKind string `json:"runtimeKind"`
	Confirm     bool   `json:"confirm"`
	Reason      string `json:"reason"`
}

func registerRuntimeControlRoutes(mux *http.ServeMux, platform *service.Platform, cfg config.Config) {
	registerRuntimeLifecycleControlRoute(mux, platform, cfg, "/api/v1/runtime/start", "start")
	registerRuntimeLifecycleControlRoute(mux, platform, cfg, "/api/v1/runtime/stop", "stop")
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
	registerRuntimeAutoRestartControlRoute(mux, platform, cfg, "/api/v1/runtime/suppress-auto-restart", "suppress")
	registerRuntimeAutoRestartControlRoute(mux, platform, cfg, "/api/v1/runtime/resume-auto-restart", "resume")
}

func registerRuntimeLifecycleControlRoute(mux *http.ServeMux, platform *service.Platform, cfg config.Config, path, action string) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !cfg.RuntimeActionsEnabled() {
			writeError(w, http.StatusConflict, "runtime action "+action+" is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
			return
		}
		var request runtimeLifecycleControlRequest
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
			writeError(w, http.StatusBadRequest, "confirm=true is required for runtime "+action)
			return
		}
		reason := strings.TrimSpace(request.Reason)
		if reason == "" {
			writeError(w, http.StatusBadRequest, "reason is required for runtime "+action)
			return
		}
		var item domain.SignalRuntimeSession
		switch strings.ToLower(strings.TrimSpace(request.RuntimeKind)) {
		case "signal", "signal-runtime":
			var updated domain.SignalRuntimeSession
			var err error
			if action == "start" {
				updated, err = platform.StartSignalRuntimeSessionWithOptions(runtimeID, service.SignalRuntimeStartOptions{
					Reason: reason,
					Source: "api",
				})
			} else {
				updated, err = platform.StopSignalRuntimeSessionWithOptions(runtimeID, service.SignalRuntimeStopOptions{
					Force:  request.Force,
					Reason: reason,
					Source: "api",
				})
			}
			if err != nil {
				writeRuntimeControlError(w, err)
				return
			}
			item = updated
		case "":
			writeError(w, http.StatusBadRequest, "runtimeKind is required")
			return
		default:
			writeError(w, http.StatusBadRequest, "unsupported runtimeKind: "+request.RuntimeKind)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":        "accepted",
			"message":       "runtime " + action + " intent accepted",
			"runtimeId":     item.ID,
			"runtimeKind":   "signal",
			"desiredStatus": item.State["desiredStatus"],
			"actualStatus":  item.State["actualStatus"],
			"force":         request.Force,
			"reason":        reason,
			"runtime":       item,
		})
	})
}

func registerRuntimeAutoRestartControlRoute(mux *http.ServeMux, platform *service.Platform, cfg config.Config, path, action string) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !cfg.RuntimeActionsEnabled() {
			writeError(w, http.StatusConflict, "runtime action "+action+" auto-restart is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
			return
		}
		var request runtimeAutoRestartControlRequest
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
			writeError(w, http.StatusBadRequest, "confirm=true is required for runtime auto-restart "+action)
			return
		}
		reason := strings.TrimSpace(request.Reason)
		if reason == "" {
			writeError(w, http.StatusBadRequest, "reason is required for runtime auto-restart "+action)
			return
		}
		var item any
		var suppressed bool
		switch strings.ToLower(strings.TrimSpace(request.RuntimeKind)) {
		case "signal", "signal-runtime":
			var updated any
			var err error
			options := service.SignalRuntimeAutoRestartControlOptions{
				Reason: reason,
				Source: "api",
			}
			if action == "suppress" {
				updated, err = platform.SuppressSignalRuntimeAutoRestart(runtimeID, options)
				suppressed = true
			} else {
				updated, err = platform.ResumeSignalRuntimeAutoRestart(runtimeID, options)
			}
			if err != nil {
				writeRuntimeControlError(w, err)
				return
			}
			item = updated
		case "":
			writeError(w, http.StatusBadRequest, "runtimeKind is required")
			return
		default:
			writeError(w, http.StatusBadRequest, "unsupported runtimeKind: "+request.RuntimeKind)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":                "accepted",
			"message":               "runtime auto-restart " + action + " intent accepted",
			"runtimeId":             runtimeID,
			"runtimeKind":           "signal",
			"autoRestartSuppressed": suppressed,
			"reason":                reason,
			"runtime":               item,
		})
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
