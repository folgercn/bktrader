package http

import (
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
)

const supervisorDashboardPermissionBlocked = "supervisor-dashboard-permission-denied"

func supervisorDashboardPermissions(cfg config.Config) service.RuntimeSupervisorDashboardPermissions {
	canView := configBoolPtrDefault(cfg.SupervisorDashboardView, true)
	canRuntimeControl := configBoolPtrDefault(cfg.SupervisorDashboardRuntimeControl, true)
	canFallbackGate := configBoolPtrDefault(cfg.SupervisorDashboardFallbackGate, true)
	canFallbackSubmit := configBoolPtrDefault(cfg.SupervisorDashboardFallbackSubmit, false)
	return service.RuntimeSupervisorDashboardPermissions{
		CanView:                              canView,
		CanRuntimeControl:                    canRuntimeControl,
		CanContainerFallbackGate:             canFallbackGate,
		CanContainerFallbackSubmit:           canFallbackSubmit,
		ViewBlockedReason:                    permissionBlockedReason(canView),
		RuntimeControlBlockedReason:          permissionBlockedReason(canRuntimeControl),
		ContainerFallbackGateBlockedReason:   permissionBlockedReason(canFallbackGate),
		ContainerFallbackSubmitBlockedReason: permissionBlockedReason(canFallbackSubmit),
	}
}

func configBoolPtrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func permissionBlockedReason(allowed bool) string {
	if allowed {
		return ""
	}
	return supervisorDashboardPermissionBlocked
}

func writeSupervisorPermissionError(w http.ResponseWriter, permission string) {
	writeError(w, http.StatusForbidden, permission+" is denied by supervisor dashboard permission policy")
}

func requestAuditSource(r *http.Request) string {
	source := strings.ToLower(strings.TrimSpace(r.Header.Get("X-BKTRADER-Control-Source")))
	switch source {
	case "dashboard", "ctl", "api", "supervisor":
		return source
	default:
		return "api"
	}
}

func requestAuditOperator(r *http.Request) string {
	if r == nil {
		return ""
	}
	if claims, ok := authClaimsFromContext(r.Context()); ok {
		return strings.TrimSpace(claims.Username)
	}
	return ""
}
