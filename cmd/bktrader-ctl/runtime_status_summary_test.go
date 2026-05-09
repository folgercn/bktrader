package main

import (
	"strings"
	"testing"
)

func TestBuildRuntimeStatusSummaryShowsAttentionAndAuditCounts(t *testing.T) {
	payload := []byte(`{
		"service":"platform-api",
		"checkedAt":"2026-05-09T08:00:00Z",
		"runtimes":[{
			"runtimeId":"signal-1",
			"runtimeKind":"signal",
			"accountId":"acct-1",
			"strategyId":"strategy-1",
			"desiredStatus":"RUNNING",
			"actualStatus":"ERROR",
			"health":"recovering",
			"restartAttempt":2,
			"nextRestartAt":"2026-05-09T08:05:00Z",
			"restartBackoff":"3m0s",
			"restartReason":"runtime-error",
			"restartSeverity":"transient",
			"lastRestartError":"websocket timeout",
			"restartRequestedAt":"2026-05-09T07:54:00Z",
			"restartRequestedSource":"api",
			"restartRequestedForce":true,
			"startRequestedAt":"2026-05-09T07:55:00Z",
			"startRequestedSource":"dashboard",
			"autoRestartSuppressed":true,
			"autoRestartSuppressedAt":"2026-05-09T07:56:00Z",
			"autoRestartSuppressedSource":"api",
			"lastHealthyAt":"2026-05-09T07:50:00Z",
			"lastCheckedAt":"2026-05-09T08:00:00Z",
			"updatedAt":"2026-05-09T08:00:01Z"
		},{
			"runtimeId":"live-1",
			"runtimeKind":"live-session",
			"desiredStatus":"RUNNING",
			"actualStatus":"RUNNING",
			"health":"healthy",
			"restartAttempt":0,
			"stopRequestedAt":"2026-05-09T07:58:00Z",
			"stopRequestedSource":"dashboard",
			"autoRestartSuppressed":false,
			"autoRestartResumedAt":"2026-05-09T07:59:00Z",
			"autoRestartResumedSource":"api",
			"lastCheckedAt":"2026-05-09T08:00:00Z"
		}]
	}`)

	summary, err := buildRuntimeStatusSummary(payload)
	if err != nil {
		t.Fatalf("build runtime status summary failed: %v", err)
	}
	expected := []string{
		"Runtime status snapshot",
		"service: platform-api",
		"checkedAt: 2026-05-09T08:00:00Z",
		"runtimes: total=2 attention=1 byKind=live-session:1,signal:1 restartAudit=1 startAudit=1 stopAudit=1 autoRestartSuppressed=1 suppressAudit=1 resumeAudit=1",
		"- signal signal-1",
		"account=acct-1 strategy=strategy-1 desired=RUNNING actual=ERROR health=recovering",
		"restart: attempt=2 next=2026-05-09T08:05:00Z backoff=3m0s reason=runtime-error severity=transient lastError=websocket timeout",
		"lifecycle: restartAt=2026-05-09T07:54:00Z restartSource=api restartForce=true startAt=2026-05-09T07:55:00Z startSource=dashboard stopAt=-- stopSource=-- stopForce=false",
		"autoRestart: suppressed=true suppressAt=2026-05-09T07:56:00Z suppressSource=api resumeAt=-- resumeSource=--",
		"lastHealthy=2026-05-09T07:50:00Z lastChecked=2026-05-09T08:00:00Z updated=2026-05-09T08:00:01Z",
		"- live-session live-1",
		"autoRestart: suppressed=false suppressAt=-- suppressSource=-- resumeAt=2026-05-09T07:59:00Z resumeSource=api",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildRuntimeStatusSummaryHandlesEmptyRuntimeList(t *testing.T) {
	payload := []byte(`{"service":"platform-api","checkedAt":"2026-05-09T08:00:00Z","runtimes":[]}`)

	summary, err := buildRuntimeStatusSummary(payload)
	if err != nil {
		t.Fatalf("build runtime status summary failed: %v", err)
	}
	expected := []string{
		"Runtime status snapshot",
		"service: platform-api",
		"runtimes: total=0 attention=0 byKind=--",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}
