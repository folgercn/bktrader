package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

type supervisorContainerFallbackControlRequest struct {
	TargetName string `json:"targetName"`
	Confirm    bool   `json:"confirm"`
	Reason     string `json:"reason"`
}

func registerSupervisorStatusRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/supervisor/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		snapshot, ok := platform.RuntimeSupervisorSnapshot()
		if !ok {
			writeError(w, http.StatusNotFound, "runtime supervisor is not configured")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})
	registerSupervisorContainerFallbackControlRoute(mux, platform, "/api/v1/supervisor/container-fallback/suppress", true)
	registerSupervisorContainerFallbackControlRoute(mux, platform, "/api/v1/supervisor/container-fallback/resume", false)
}

func registerSupervisorContainerFallbackControlRoute(mux *http.ServeMux, platform *service.Platform, path string, suppressed bool) {
	action := "resume"
	if suppressed {
		action = "suppress"
	}
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request supervisorContainerFallbackControlRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		targetName := strings.TrimSpace(request.TargetName)
		if targetName == "" {
			writeError(w, http.StatusBadRequest, "targetName is required")
			return
		}
		if !request.Confirm {
			writeError(w, http.StatusBadRequest, "confirm=true is required for supervisor container fallback "+action)
			return
		}
		reason := strings.TrimSpace(request.Reason)
		if reason == "" {
			writeError(w, http.StatusBadRequest, "reason is required for supervisor container fallback "+action)
			return
		}
		options := service.RuntimeSupervisorContainerFallbackControlOptions{
			Confirm: true,
			Reason:  reason,
			Source:  "api",
		}
		var result service.RuntimeSupervisorContainerFallbackControlResult
		var err error
		if suppressed {
			result, err = platform.SuppressRuntimeSupervisorContainerFallback(targetName, options)
		} else {
			result, err = platform.ResumeRuntimeSupervisorContainerFallback(targetName, options)
		}
		if err != nil {
			writeSupervisorContainerFallbackControlError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":       "accepted",
			"message":      "supervisor container fallback " + action + " accepted",
			"targetName":   result.TargetName,
			"suppressed":   result.Suppressed,
			"reason":       result.Reason,
			"source":       result.Source,
			"updatedAt":    result.UpdatedAt,
			"serviceState": result.ServiceState,
		})
	})
}

func writeSupervisorContainerFallbackControlError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrRuntimeSupervisorControlConfirmRequired),
		errors.Is(err, service.ErrRuntimeSupervisorControlReasonRequired),
		errors.Is(err, service.ErrRuntimeSupervisorTargetRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrRuntimeSupervisorTargetNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrRuntimeSupervisorTargetAmbiguous):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
