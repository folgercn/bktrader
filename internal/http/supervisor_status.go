package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
)

type supervisorContainerFallbackControlRequest struct {
	TargetName     string `json:"targetName"`
	Confirm        bool   `json:"confirm"`
	Reason         string `json:"reason"`
	BackoffSeconds int    `json:"backoffSeconds,omitempty"`
}

const maxSupervisorContainerFallbackBackoffSeconds = 24 * 60 * 60

func registerSupervisorStatusRoutes(mux *http.ServeMux, platform *service.Platform, cfg config.Config) {
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
	registerSupervisorContainerFallbackControlRoute(mux, platform, cfg, "/api/v1/supervisor/container-fallback/suppress", true)
	registerSupervisorContainerFallbackControlRoute(mux, platform, cfg, "/api/v1/supervisor/container-fallback/resume", false)
	registerSupervisorContainerFallbackBackoffRoute(mux, platform, cfg, "/api/v1/supervisor/container-fallback/defer", false)
	registerSupervisorContainerFallbackBackoffRoute(mux, platform, cfg, "/api/v1/supervisor/container-fallback/clear-backoff", true)
}

func registerSupervisorContainerFallbackControlRoute(mux *http.ServeMux, platform *service.Platform, cfg config.Config, path string, suppressed bool) {
	action := "resume"
	if suppressed {
		action = "suppress"
	}
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !cfg.RuntimeActionsEnabled() {
			writeError(w, http.StatusConflict, "supervisor container fallback "+action+" is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
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

func registerSupervisorContainerFallbackBackoffRoute(mux *http.ServeMux, platform *service.Platform, cfg config.Config, path string, clear bool) {
	action := "defer"
	if clear {
		action = "clear-backoff"
	}
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !cfg.RuntimeActionsEnabled() {
			writeError(w, http.StatusConflict, "supervisor container fallback "+action+" is disabled for BKTRADER_ROLE="+cfg.ProcessRole)
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
		if !clear && request.BackoffSeconds <= 0 {
			writeError(w, http.StatusBadRequest, "backoffSeconds must be positive for supervisor container fallback "+action)
			return
		}
		if !clear && request.BackoffSeconds > maxSupervisorContainerFallbackBackoffSeconds {
			writeError(w, http.StatusBadRequest, "backoffSeconds must be <= 86400 for supervisor container fallback "+action)
			return
		}
		options := service.RuntimeSupervisorContainerFallbackControlOptions{
			Confirm:         true,
			Reason:          reason,
			Source:          "api",
			BackoffDuration: time.Duration(request.BackoffSeconds) * time.Second,
		}
		var result service.RuntimeSupervisorContainerFallbackControlResult
		var err error
		if clear {
			result, err = platform.ClearRuntimeSupervisorContainerFallbackBackoff(targetName, options)
		} else {
			result, err = platform.DeferRuntimeSupervisorContainerFallback(targetName, options)
		}
		if err != nil {
			writeSupervisorContainerFallbackControlError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":       "accepted",
			"message":      "supervisor container fallback " + action + " accepted",
			"targetName":   result.TargetName,
			"backoffUntil": result.BackoffUntil,
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
		errors.Is(err, service.ErrRuntimeSupervisorBackoffDurationRequired),
		errors.Is(err, service.ErrRuntimeSupervisorBackoffDurationTooLarge),
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
