package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestRuntimeStatusFromLiveSessionUsesFreshestStateTimestamp(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	lastEventAt := createdAt.Add(3 * time.Minute)
	lastEvaluationAt := createdAt.Add(9 * time.Minute)
	session := domain.LiveSession{
		ID:         "live-session-1",
		AccountID:  "live-main",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		CreatedAt:  createdAt,
		State: map[string]any{
			"lastEventAt":              lastEventAt.Format(time.RFC3339),
			"lastStrategyEvaluationAt": lastEvaluationAt.Format(time.RFC3339),
		},
	}

	status := runtimeStatusFromLiveSession("platform-api", createdAt.Add(time.Hour), session)
	if status.UpdatedAt == nil {
		t.Fatal("expected live runtime status updatedAt from runtime state")
	}
	if !status.UpdatedAt.Equal(lastEvaluationAt) {
		t.Fatalf("expected updatedAt %s, got %s", lastEvaluationAt.Format(time.RFC3339), status.UpdatedAt.Format(time.RFC3339))
	}
}

func TestRuntimeStatusFromLiveSessionOmitsUpdatedAtWhenStateHasNoFreshnessTime(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	session := domain.LiveSession{
		ID:         "live-session-1",
		AccountID:  "live-main",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		CreatedAt:  createdAt,
		State:      map[string]any{},
	}

	status := runtimeStatusFromLiveSession("platform-api", createdAt.Add(time.Hour), session)
	if status.UpdatedAt != nil {
		t.Fatalf("expected live runtime status to avoid createdAt fallback, got %s", status.UpdatedAt.Format(time.RFC3339))
	}
}

func TestRuntimeStatusFromStateExposesAutoRestartAuditFields(t *testing.T) {
	checkedAt := time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC)
	suppressedAt := checkedAt.Add(-10 * time.Minute).Format(time.RFC3339)
	resumedAt := checkedAt.Add(-5 * time.Minute).Format(time.RFC3339)
	status := runtimeStatusFromState("platform-api", "signal", "runtime-1", "ERROR", map[string]any{
		"desiredStatus":               "RUNNING",
		"actualStatus":                "ERROR",
		"autoRestartSuppressed":       true,
		"autoRestartSuppressedAt":     suppressedAt,
		"autoRestartSuppressedReason": "operator paused runtime recovery during maintenance",
		"autoRestartSuppressedSource": "api",
		"autoRestartResumedAt":        resumedAt,
		"autoRestartResumedReason":    "maintenance finished",
		"autoRestartResumedSource":    "dashboard",
		"supervisorRestartReason":     "manual-suppress-auto-restart",
		"supervisorRestartSeverity":   "fatal",
		"lastSupervisorError":         "auth failed",
	}, checkedAt)

	if !status.AutoRestartSuppressed {
		t.Fatal("expected autoRestartSuppressed true")
	}
	if status.AutoRestartSuppressedAt != suppressedAt {
		t.Fatalf("expected suppressed at %s, got %s", suppressedAt, status.AutoRestartSuppressedAt)
	}
	if status.AutoRestartSuppressedReason != "operator paused runtime recovery during maintenance" {
		t.Fatalf("expected suppress reason, got %s", status.AutoRestartSuppressedReason)
	}
	if status.AutoRestartSuppressedSource != "api" {
		t.Fatalf("expected suppress source api, got %s", status.AutoRestartSuppressedSource)
	}
	if status.AutoRestartResumedAt != resumedAt {
		t.Fatalf("expected resumed at %s, got %s", resumedAt, status.AutoRestartResumedAt)
	}
	if status.AutoRestartResumedReason != "maintenance finished" {
		t.Fatalf("expected resume reason, got %s", status.AutoRestartResumedReason)
	}
	if status.AutoRestartResumedSource != "dashboard" {
		t.Fatalf("expected resume source dashboard, got %s", status.AutoRestartResumedSource)
	}
	if status.RestartReason != "manual-suppress-auto-restart" || status.RestartSeverity != "fatal" {
		t.Fatalf("expected restart audit reason/severity, got %s/%s", status.RestartReason, status.RestartSeverity)
	}
	if status.LastRestartError != "auth failed" {
		t.Fatalf("expected last restart error, got %s", status.LastRestartError)
	}
}

func TestRuntimeStatusFromStateExposesLifecycleAuditFields(t *testing.T) {
	checkedAt := time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC)
	restartedAt := checkedAt.Add(-20 * time.Minute).Format(time.RFC3339)
	startedAt := checkedAt.Add(-15 * time.Minute).Format(time.RFC3339)
	stoppedAt := checkedAt.Add(-3 * time.Minute).Format(time.RFC3339)
	status := runtimeStatusFromState("platform-api", "signal", "runtime-1", "STOPPED", map[string]any{
		"desiredStatus":          "STOPPED",
		"actualStatus":           "STOPPED",
		"restartRequestedAt":     restartedAt,
		"restartRequestedReason": "operator requested rebinding",
		"restartRequestedSource": "api",
		"restartRequestedForce":  true,
		"startRequestedAt":       startedAt,
		"startRequestedReason":   "maintenance finished",
		"startRequestedSource":   "api",
		"stopRequestedAt":        stoppedAt,
		"stopRequestedReason":    "maintenance window",
		"stopRequestedSource":    "dashboard",
		"stopRequestedForce":     true,
	}, checkedAt)

	if status.RestartRequestedAt != restartedAt {
		t.Fatalf("expected restart requested at %s, got %s", restartedAt, status.RestartRequestedAt)
	}
	if status.RestartRequestedReason != "operator requested rebinding" {
		t.Fatalf("expected restart reason, got %s", status.RestartRequestedReason)
	}
	if status.RestartRequestedSource != "api" {
		t.Fatalf("expected restart source api, got %s", status.RestartRequestedSource)
	}
	if !status.RestartRequestedForce {
		t.Fatal("expected restart requested force true")
	}
	if status.StartRequestedAt != startedAt {
		t.Fatalf("expected start requested at %s, got %s", startedAt, status.StartRequestedAt)
	}
	if status.StartRequestedReason != "maintenance finished" {
		t.Fatalf("expected start reason, got %s", status.StartRequestedReason)
	}
	if status.StartRequestedSource != "api" {
		t.Fatalf("expected start source api, got %s", status.StartRequestedSource)
	}
	if status.StopRequestedAt != stoppedAt {
		t.Fatalf("expected stop requested at %s, got %s", stoppedAt, status.StopRequestedAt)
	}
	if status.StopRequestedReason != "maintenance window" {
		t.Fatalf("expected stop reason, got %s", status.StopRequestedReason)
	}
	if status.StopRequestedSource != "dashboard" {
		t.Fatalf("expected stop source dashboard, got %s", status.StopRequestedSource)
	}
	if !status.StopRequestedForce {
		t.Fatal("expected stop requested force true")
	}
}
