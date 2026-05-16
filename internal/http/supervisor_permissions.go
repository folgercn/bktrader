package http

import (
	"net/http"

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
