package main

import (
	"strings"
	"testing"
)

func TestBuildSupervisorStatusSummaryShowsFallbackReadiness(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"policy":{
			"applicationRestartEnabled":true,
			"serviceFailureThreshold":3,
			"containerRestartEnabled":true,
			"containerExecutorConfigured":false,
			"containerExecutorKind":"none",
			"containerExecutorDryRun":true
		},
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","statusCode":503,"reachable":true,"error":"http 503"},
			"runtimeStatus":{"path":"/api/v1/runtime/status","statusCode":200,"reachable":true},
			"serviceState":{
				"consecutiveFailures":3,
				"failureThreshold":3,
				"lastFailureReason":"healthz-unhealthy: http 503",
				"containerFallbackCandidate":true,
				"containerFallbackReason":"service probes failed 3/3",
				"containerFallbackAttemptCount":2,
				"containerFallbackSuppressed":false,
				"lastContainerFallbackDecisionAt":"2026-04-29T08:00:00Z",
				"lastContainerFallbackDecisionReason":"container-executor-not-configured"
			},
			"containerFallbackPlan":{
				"action":"container-restart",
				"candidate":true,
				"enabled":true,
				"executorConfigured":false,
				"executorKind":"none",
				"executorDryRun":true,
				"executable":false,
				"decision":"blocked",
				"suppressed":false,
				"backoffActive":false,
				"safetyGateOk":true,
				"blockedReason":"container-executor-not-configured"
			},
			"status":{
				"service":"platform-api",
				"runtimes":[{
					"runtimeId":"signal-1",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"ERROR",
					"health":"recovering"
				}]
			},
			"controlActions":[{"action":"runtime-restart","runtimeId":"signal-1"}]
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"Runtime supervisor snapshot",
		"policy: applicationRestartEnabled=true serviceFailureThreshold=3 containerRestartEnabled=true containerExecutorConfigured=false containerExecutorKind=none containerExecutorDryRun=true",
		"targets: total=1 fullyReachable=0 fallbackCandidates=1 fallbackExecutable=0 fallbackDryRun=0 runtimes=1 attention=1 controlActions=1",
		"serviceState: failures=3/3 fallback=candidate attempts=2 suppressed=false backoffUntil=--",
		"lastFallbackDecision=container-executor-not-configured at=2026-04-29T08:00:00Z",
		"fallbackPlan: action=container-restart decision=blocked enabled=true executorConfigured=false executorKind=none executorDryRun=true executable=false suppressed=false backoffActive=false safetyGateOk=true blockedReason=container-executor-not-configured eligibleReason=--",
		"lastFailure=healthz-unhealthy: http 503",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildSupervisorStatusSummaryCountsDryRunExecutableFallback(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"policy":{
			"applicationRestartEnabled":false,
			"serviceFailureThreshold":1,
			"containerRestartEnabled":true,
			"containerExecutorConfigured":true,
			"containerExecutorKind":"noop",
			"containerExecutorDryRun":true
		},
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","reachable":false,"error":"connection refused"},
			"runtimeStatus":{"path":"/api/v1/runtime/status","reachable":false,"error":"connection refused"},
			"serviceState":{
				"consecutiveFailures":1,
				"failureThreshold":1,
				"containerFallbackCandidate":true,
				"containerFallbackAttemptCount":1
			},
			"containerFallbackPlan":{
				"action":"container-restart",
				"candidate":true,
				"enabled":true,
				"executorConfigured":true,
				"executorKind":"noop",
				"executorDryRun":true,
				"executable":true,
				"decision":"eligible",
				"suppressed":false,
				"backoffActive":false,
				"safetyGateOk":true,
				"eligibleReason":"container-fallback-eligible"
			}
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"policy: applicationRestartEnabled=false serviceFailureThreshold=1 containerRestartEnabled=true containerExecutorConfigured=true containerExecutorKind=noop containerExecutorDryRun=true",
		"targets: total=1 fullyReachable=0 fallbackCandidates=1 fallbackExecutable=1 fallbackDryRun=1 runtimes=0 attention=0 controlActions=0",
		"fallbackPlan: action=container-restart decision=eligible enabled=true executorConfigured=true executorKind=noop executorDryRun=true executable=true suppressed=false backoffActive=false safetyGateOk=true blockedReason=-- eligibleReason=container-fallback-eligible",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildSupervisorStatusSummaryHandlesClearTarget(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"policy":{
			"applicationRestartEnabled":false,
			"serviceFailureThreshold":3,
			"containerRestartEnabled":false,
			"containerExecutorConfigured":false,
			"containerExecutorKind":"none",
			"containerExecutorDryRun":true
		},
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","statusCode":200,"reachable":true},
			"runtimeStatus":{"path":"/api/v1/runtime/status","statusCode":200,"reachable":true},
			"serviceState":{"consecutiveFailures":0,"failureThreshold":3,"containerFallbackCandidate":false},
			"status":{"service":"platform-api","runtimes":[]}
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"policy: applicationRestartEnabled=false serviceFailureThreshold=3 containerRestartEnabled=false containerExecutorConfigured=false containerExecutorKind=none containerExecutorDryRun=true",
		"targets: total=1 fullyReachable=1 fallbackCandidates=0 fallbackExecutable=0 fallbackDryRun=0 runtimes=0 attention=0 controlActions=0",
		"serviceState: failures=0/3 fallback=clear attempts=0 suppressed=false backoffUntil=--",
		"runtimes: total=0 attention=0 service=platform-api",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "fallbackPlan:") {
		t.Fatalf("did not expect fallback plan for clear target, got:\n%s", summary)
	}
}
