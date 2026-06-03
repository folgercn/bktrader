package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildLiveSessionDetailPathUsesDefaultFields(t *testing.T) {
	path := buildLiveSessionDetailPath("live/session 1", "")
	if !strings.HasPrefix(path, "/api/v1/live/sessions/live%2Fsession%201/detail?") {
		t.Fatalf("expected escaped live session detail path, got %s", path)
	}
	for _, field := range []string{
		"timeline",
		"breakoutHistory",
		"lastStrategyEvaluationSignalBarStates",
		"lastStrategyDecision",
		"lastExecutionDispatch",
	} {
		if !strings.Contains(path, field) {
			t.Fatalf("expected default detail field %s in path %s", field, path)
		}
	}
}

func TestBuildLiveSessionDetailPathAllowsCustomFields(t *testing.T) {
	path := buildLiveSessionDetailPath("live-1", "timeline,lastStrategyDecision")
	if !strings.Contains(path, "fields=timeline%2ClastStrategyDecision") {
		t.Fatalf("expected custom fields query, got %s", path)
	}
	if strings.Contains(path, "lastExecutionDispatch") {
		t.Fatalf("expected custom fields to override defaults, got %s", path)
	}
}

func TestBuildLiveSessionControlStatusPendingDuration(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	session := liveSessionControlView{
		ID:         "live-1",
		AccountID:  "account-1",
		StrategyID: "strategy-1",
		Status:     "STOPPED",
		State: map[string]any{
			"desiredStatus":       "RUNNING",
			"actualStatus":        "STARTING",
			"lastControlAction":   "start",
			"controlRequestId":    "request-1",
			"controlVersion":      3,
			"controlRequestedAt":  now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
			"lastControlUpdateAt": now.Add(-90 * time.Second).Format(time.RFC3339Nano),
		},
	}

	status := buildLiveSessionControlStatus(session, now)
	if !status.Pending {
		t.Fatal("expected STARTING status to be pending")
	}
	if status.PendingSeconds != 90 {
		t.Fatalf("expected pending duration to use lastControlUpdateAt for in-progress control, got %d", status.PendingSeconds)
	}
	if status.ControlVersion != "3" {
		t.Fatalf("expected stringified control version 3, got %s", status.ControlVersion)
	}
}

func TestBuildLiveSessionControlStatusErrorHint(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	session := liveSessionControlView{
		ID:     "live-1",
		Status: "RUNNING",
		State: map[string]any{
			"desiredStatus":        "STOPPED",
			"actualStatus":         "ERROR",
			"lastControlErrorCode": "ACTIVE_POSITIONS_OR_ORDERS",
			"lastControlError":     "active position exists",
		},
	}

	status := buildLiveSessionControlStatus(session, now)
	if status.Pending {
		t.Fatal("expected ERROR status not to be pending")
	}
	if status.Hint != "close positions/orders first or retry stop with --force" {
		t.Fatalf("unexpected error hint: %s", status.Hint)
	}
}

func TestFilterLiveSessionControlStatuses(t *testing.T) {
	statuses := []liveSessionControlStatus{
		{ID: "running", ActualStatus: "RUNNING"},
		{ID: "pending", DesiredStatus: "RUNNING", ActualStatus: "STARTING", Pending: true},
		{ID: "error", ActualStatus: "ERROR", ErrorCode: "CONFIG_ERROR"},
	}

	pending := filterLiveSessionControlStatuses(statuses, true, false)
	if len(pending) != 1 || pending[0].ID != "pending" {
		t.Fatalf("expected only pending status, got %#v", pending)
	}

	errors := filterLiveSessionControlStatuses(statuses, false, true)
	if len(errors) != 1 || errors[0].ID != "error" {
		t.Fatalf("expected only error status, got %#v", errors)
	}

	combined := filterLiveSessionControlStatuses(statuses, true, true)
	if len(combined) != 2 || combined[0].ID != "pending" || combined[1].ID != "error" {
		t.Fatalf("expected pending and error statuses, got %#v", combined)
	}
}
