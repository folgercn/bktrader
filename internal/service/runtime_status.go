package service

import (
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type RuntimeStatusSnapshot struct {
	Service   string          `json:"service"`
	CheckedAt time.Time       `json:"checkedAt"`
	Runtimes  []RuntimeStatus `json:"runtimes"`
}

type RuntimeStatus struct {
	Service                     string                                   `json:"service"`
	RuntimeID                   string                                   `json:"runtimeId"`
	RuntimeKind                 string                                   `json:"runtimeKind"`
	AccountID                   string                                   `json:"accountId,omitempty"`
	StrategyID                  string                                   `json:"strategyId,omitempty"`
	DesiredStatus               string                                   `json:"desiredStatus,omitempty"`
	ActualStatus                string                                   `json:"actualStatus,omitempty"`
	Health                      string                                   `json:"health,omitempty"`
	RestartAttempt              int                                      `json:"restartAttempt"`
	NextRestartAt               string                                   `json:"nextRestartAt,omitempty"`
	RestartBackoff              string                                   `json:"restartBackoff,omitempty"`
	RestartReason               string                                   `json:"restartReason,omitempty"`
	RestartSeverity             string                                   `json:"restartSeverity,omitempty"`
	LastRestartError            string                                   `json:"lastRestartError,omitempty"`
	AutoRestartSuppressed       bool                                     `json:"autoRestartSuppressed"`
	AutoRestartSuppressedAt     string                                   `json:"autoRestartSuppressedAt,omitempty"`
	AutoRestartSuppressedReason string                                   `json:"autoRestartSuppressedReason,omitempty"`
	AutoRestartSuppressedSource string                                   `json:"autoRestartSuppressedSource,omitempty"`
	AutoRestartResumedAt        string                                   `json:"autoRestartResumedAt,omitempty"`
	AutoRestartResumedReason    string                                   `json:"autoRestartResumedReason,omitempty"`
	AutoRestartResumedSource    string                                   `json:"autoRestartResumedSource,omitempty"`
	LastHealthyAt               string                                   `json:"lastHealthyAt,omitempty"`
	LastCheckedAt               string                                   `json:"lastCheckedAt"`
	UpdatedAt                   *time.Time                               `json:"updatedAt,omitempty"`
	ApplicationRestartPlan      *RuntimeSupervisorApplicationRestartPlan `json:"applicationRestartPlan,omitempty"`
}

var liveRuntimeStatusUpdatedAtKeys = []string{
	"lastStrategyEvaluationAt",
	"lastSignalRuntimeEventAt",
	"lastSyncedAt",
	"lastDispatchedAt",
	"lastEventAt",
	"lastHeartbeatAt",
	"lastRuntimeEventPublishedAt",
}

func (p *Platform) RuntimeStatusSnapshot(serviceName string, checkedAt time.Time) (RuntimeStatusSnapshot, error) {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = "platform-api"
	}
	runtimes := make([]RuntimeStatus, 0)
	for _, session := range p.ListSignalRuntimeSessionsSummary() {
		runtimes = append(runtimes, runtimeStatusFromSignalSession(serviceName, checkedAt, session))
	}
	liveSessions, err := p.ListLiveSessionsSummary()
	if err != nil {
		return RuntimeStatusSnapshot{}, err
	}
	for _, session := range liveSessions {
		runtimes = append(runtimes, runtimeStatusFromLiveSession(serviceName, checkedAt, session))
	}
	return RuntimeStatusSnapshot{
		Service:   serviceName,
		CheckedAt: checkedAt.UTC(),
		Runtimes:  runtimes,
	}, nil
}

func runtimeStatusFromSignalSession(serviceName string, checkedAt time.Time, session domain.SignalRuntimeSession) RuntimeStatus {
	status := runtimeStatusFromState(serviceName, "signal", session.ID, session.Status, session.State, checkedAt)
	status.AccountID = session.AccountID
	status.StrategyID = session.StrategyID
	setRuntimeStatusUpdatedAt(&status, session.UpdatedAt)
	return status
}

func runtimeStatusFromLiveSession(serviceName string, checkedAt time.Time, session domain.LiveSession) RuntimeStatus {
	status := runtimeStatusFromState(serviceName, "live-session", session.ID, session.Status, session.State, checkedAt)
	status.AccountID = session.AccountID
	status.StrategyID = session.StrategyID
	setRuntimeStatusUpdatedAt(&status, runtimeStatusLatestStateTime(session.State, liveRuntimeStatusUpdatedAtKeys...))
	return status
}

func setRuntimeStatusUpdatedAt(status *RuntimeStatus, updatedAt time.Time) {
	if status == nil || updatedAt.IsZero() {
		return
	}
	normalized := updatedAt.UTC()
	status.UpdatedAt = &normalized
}

func runtimeStatusLatestStateTime(state map[string]any, keys ...string) time.Time {
	var latest time.Time
	for _, key := range keys {
		candidate := parseOptionalRFC3339(stringValue(state[key]))
		if candidate.IsZero() {
			continue
		}
		candidate = candidate.UTC()
		if latest.IsZero() || candidate.After(latest) {
			latest = candidate
		}
	}
	return latest
}

func runtimeStatusFromState(serviceName, runtimeKind, runtimeID, status string, state map[string]any, checkedAt time.Time) RuntimeStatus {
	desiredStatus := strings.ToUpper(strings.TrimSpace(stringValue(state["desiredStatus"])))
	actualStatus := strings.ToUpper(strings.TrimSpace(stringValue(state["actualStatus"])))
	status = strings.ToUpper(strings.TrimSpace(status))
	if actualStatus == "" {
		actualStatus = status
	}
	health := strings.ToLower(strings.TrimSpace(stringValue(state["health"])))
	if health == "" {
		health = runtimeHealthFromActualStatus(actualStatus)
	}
	return RuntimeStatus{
		Service:                     serviceName,
		RuntimeID:                   runtimeID,
		RuntimeKind:                 runtimeKind,
		DesiredStatus:               desiredStatus,
		ActualStatus:                actualStatus,
		Health:                      health,
		RestartAttempt:              firstRestartAttempt(state, "restartAttempt", "supervisorRestartAttempt"),
		NextRestartAt:               firstNonEmpty(stringValue(state["nextRestartAt"]), stringValue(state["nextAutoRestartAt"])),
		RestartBackoff:              firstNonEmpty(stringValue(state["restartBackoff"]), stringValue(state["supervisorRestartBackoff"])),
		RestartReason:               firstNonEmpty(stringValue(state["restartReason"]), stringValue(state["supervisorRestartReason"])),
		RestartSeverity:             firstNonEmpty(stringValue(state["restartSeverity"]), stringValue(state["supervisorRestartSeverity"])),
		LastRestartError:            firstNonEmpty(stringValue(state["lastRestartError"]), stringValue(state["lastSupervisorError"])),
		AutoRestartSuppressed:       boolValue(state["autoRestartSuppressed"]),
		AutoRestartSuppressedAt:     stringValue(state["autoRestartSuppressedAt"]),
		AutoRestartSuppressedReason: stringValue(state["autoRestartSuppressedReason"]),
		AutoRestartSuppressedSource: stringValue(state["autoRestartSuppressedSource"]),
		AutoRestartResumedAt:        stringValue(state["autoRestartResumedAt"]),
		AutoRestartResumedReason:    stringValue(state["autoRestartResumedReason"]),
		AutoRestartResumedSource:    stringValue(state["autoRestartResumedSource"]),
		LastHealthyAt:               stringValue(state["lastHealthyAt"]),
		LastCheckedAt:               checkedAt.UTC().Format(time.RFC3339),
	}
}

func firstRestartAttempt(state map[string]any, keys ...string) int {
	for _, key := range keys {
		if attempt := RestartAttempt(state, key); attempt > 0 {
			return attempt
		}
	}
	return 0
}

func runtimeHealthFromActualStatus(actualStatus string) string {
	switch strings.ToUpper(strings.TrimSpace(actualStatus)) {
	case "RUNNING":
		return "healthy"
	case "STARTING", "RECOVERING":
		return "recovering"
	case "ERROR", "BLOCKED":
		return "error"
	case "STOPPED":
		return "stopped"
	default:
		return "unknown"
	}
}
