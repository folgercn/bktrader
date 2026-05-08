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
				"serviceFailureEpisodeStartedAt":"2026-04-29T07:58:00Z",
				"lastFailureReason":"healthz-unhealthy: http 503",
				"containerFallbackCandidate":true,
				"containerFallbackReason":"service probes failed 3/3",
				"containerFallbackCandidateSince":"2026-04-29T08:00:00Z",
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
						"health":"recovering",
						"applicationRestartPlan":{
							"candidate":true,
							"enabled":true,
							"healthzOk":false,
							"supported":true,
							"due":true,
							"decision":"blocked",
							"blockedReason":"runtime-restart-healthz-unhealthy"
						}
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
		"serviceState: failures=3/3 episodeStartedAt=2026-04-29T07:58:00Z fallback=candidate candidateSince=2026-04-29T08:00:00Z attempts=2 suppressed=false backoffUntil=-- submitted=false",
		"lastFallbackDecision=container-executor-not-configured at=2026-04-29T08:00:00Z",
		"fallbackPlan: action=container-restart decision=blocked enabled=true executorConfigured=false executorKind=none executorDryRun=true executable=false duplicate=false suppressed=false backoffActive=false safetyGateOk=true blockedReason=container-executor-not-configured eligibleReason=--",
		"runtimes: total=1 attention=1 restartPlans=1 restartEligible=0 restartBlockedReasons=runtime-restart-healthz-unhealthy:1 service=platform-api",
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
		"fallbackPlan: action=container-restart decision=eligible enabled=true executorConfigured=true executorKind=noop executorDryRun=true executable=true duplicate=false suppressed=false backoffActive=false safetyGateOk=true blockedReason=-- eligibleReason=container-fallback-eligible",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildSupervisorStatusSummaryShowsFallbackControlAudit(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"targets":[],
		"containerFallbackControls":[{
			"action":"defer-container-fallback",
			"targetName":"api",
			"targetBaseUrl":"http://127.0.0.1:8080",
			"suppressed":false,
			"backoffUntil":"2026-04-29T08:05:00Z",
			"backoffSeconds":300,
			"reason":"operator cooling down restart loop",
			"source":"ctl",
			"updatedAt":"2026-04-29T08:00:00Z"
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"targets: total=0 fullyReachable=0 fallbackCandidates=0 fallbackExecutable=0 fallbackDryRun=0 runtimes=0 attention=0 controlActions=0 serviceFailureEpisodes=0 fallbackControls=1 fallbackActions=0",
		"fallbackControls: total=1",
		"defer-container-fallback target=api suppressed=false backoffUntil=2026-04-29T08:05:00Z backoffSeconds=300 source=ctl updatedAt=2026-04-29T08:00:00Z reason=operator cooling down restart loop",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildSupervisorStatusSummaryShowsServiceFailureEpisodes(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:10:00Z",
		"targets":[],
		"serviceFailureEpisodes":[{
			"targetName":"api",
			"targetBaseUrl":"http://127.0.0.1:8080",
			"startedAt":"2026-04-29T08:00:00Z",
			"recoveredAt":"2026-04-29T08:10:00Z",
			"durationSeconds":600,
			"maxConsecutiveFailures":4,
			"lastFailureReason":"healthz-unhealthy: http 503",
			"lastFailureAt":"2026-04-29T08:09:00Z",
			"containerFallbackCandidate":true,
			"containerFallbackCandidateSince":"2026-04-29T08:02:00Z",
			"containerFallbackAttemptCount":2,
			"containerFallbackSubmitted":true,
			"containerFallbackSubmittedAt":"2026-04-29T08:03:00Z",
			"containerFallbackSubmittedError":"noop executor failed",
			"containerFallbackBackoffUntil":"2026-04-29T08:08:00Z",
			"lastContainerFallbackDecisionAt":"2026-04-29T08:03:00Z",
			"lastContainerFallbackDecisionReason":"container-fallback-backoff-active"
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"targets: total=0 fullyReachable=0 fallbackCandidates=0 fallbackExecutable=0 fallbackDryRun=0 runtimes=0 attention=0 controlActions=0 serviceFailureEpisodes=1 fallbackControls=0 fallbackActions=0",
		"serviceFailureEpisodes: total=1",
		"target=api startedAt=2026-04-29T08:00:00Z recoveredAt=2026-04-29T08:10:00Z durationSeconds=600 maxFailures=4 candidate=true candidateSince=2026-04-29T08:02:00Z attempts=2 submitted=true submittedAt=2026-04-29T08:03:00Z lastDecision=container-fallback-backoff-active lastFailure=healthz-unhealthy: http 503 submittedError=noop executor failed backoffUntil=2026-04-29T08:08:00Z",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildSupervisorStatusSummaryShowsFallbackSubmittedAndErrorBackoff(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","statusCode":503,"reachable":true,"error":"http 503"},
			"runtimeStatus":{"path":"/api/v1/runtime/status","statusCode":200,"reachable":true},
			"serviceState":{
				"consecutiveFailures":1,
				"failureThreshold":1,
				"serviceFailureEpisodeStartedAt":"2026-04-29T08:00:00Z",
				"containerFallbackCandidate":true,
				"containerFallbackCandidateSince":"2026-04-29T08:00:00Z",
				"containerFallbackAttemptCount":1,
				"containerFallbackBackoffUntil":"2026-04-29T08:05:00Z",
				"containerFallbackBackoffSetAt":"2026-04-29T08:00:00Z",
				"containerFallbackBackoffReason":"container fallback executor error: noop executor failed",
				"containerFallbackBackoffSource":"supervisor",
				"containerFallbackSubmitted":true,
				"containerFallbackSubmittedAt":"2026-04-29T08:00:00Z",
				"containerFallbackSubmittedReason":"service probes failed 1/1",
				"containerFallbackSubmittedError":"noop executor failed",
				"lastContainerFallbackDecisionAt":"2026-04-29T08:00:00Z",
				"lastContainerFallbackDecisionReason":"container-fallback-backoff-active"
			},
			"containerFallbackPlan":{
				"action":"container-restart",
				"candidate":true,
				"enabled":true,
				"executorConfigured":true,
				"executorKind":"noop",
				"executorDryRun":true,
				"executable":false,
				"decision":"blocked",
				"duplicate":true,
				"suppressed":false,
				"backoffActive":true,
				"safetyGateOk":true,
				"blockedReason":"container-fallback-backoff-active"
			}
		}],
		"containerFallbackActions":[{
			"action":"container-restart",
			"targetName":"api",
			"targetBaseUrl":"http://127.0.0.1:8080",
			"reason":"service probes failed 1/1",
			"serviceFailureEpisodeStartedAt":"2026-04-29T08:00:00Z",
			"containerFallbackCandidateSince":"2026-04-29T08:00:00Z",
			"executorKind":"noop",
			"executorDryRun":true,
			"submitted":true,
			"executed":false,
			"error":"noop executor failed",
			"backoffUntil":"2026-04-29T08:05:00Z",
			"backoffSeconds":300,
			"requestedAt":"2026-04-29T08:00:00Z"
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"serviceState: failures=1/1 episodeStartedAt=2026-04-29T08:00:00Z fallback=candidate candidateSince=2026-04-29T08:00:00Z attempts=1 suppressed=false backoffUntil=2026-04-29T08:05:00Z submitted=true",
		"fallbackBackoff reason=container fallback executor error: noop executor failed source=supervisor setAt=2026-04-29T08:00:00Z until=2026-04-29T08:05:00Z",
		"fallbackSubmitted at=2026-04-29T08:00:00Z reason=service probes failed 1/1 message=-- error=noop executor failed",
		"fallbackPlan: action=container-restart decision=blocked enabled=true executorConfigured=true executorKind=noop executorDryRun=true executable=false duplicate=true suppressed=false backoffActive=true safetyGateOk=true blockedReason=container-fallback-backoff-active eligibleReason=--",
		"container-restart target=api executorKind=noop executorDryRun=true submitted=true executed=false requestedAt=2026-04-29T08:00:00Z episodeStartedAt=2026-04-29T08:00:00Z candidateSince=2026-04-29T08:00:00Z reason=service probes failed 1/1 error=noop executor failed backoffUntil=2026-04-29T08:05:00Z backoffSeconds=300",
	}
	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildSupervisorStatusSummaryShowsFallbackActionAudit(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"targets":[],
		"containerFallbackActions":[{
			"action":"container-restart",
			"targetName":"api",
			"targetBaseUrl":"http://127.0.0.1:8080",
			"reason":"service probes failed 1/1",
			"executorKind":"noop",
			"executorDryRun":true,
			"submitted":true,
			"executed":false,
			"message":"noop container fallback executor",
			"requestedAt":"2026-04-29T08:00:00Z"
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	expected := []string{
		"targets: total=0 fullyReachable=0 fallbackCandidates=0 fallbackExecutable=0 fallbackDryRun=0 runtimes=0 attention=0 controlActions=0 serviceFailureEpisodes=0 fallbackControls=0 fallbackActions=1",
		"fallbackActions: total=1",
		"container-restart target=api executorKind=noop executorDryRun=true submitted=true executed=false requestedAt=2026-04-29T08:00:00Z episodeStartedAt=-- candidateSince=-- reason=service probes failed 1/1 message=noop container fallback executor",
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
		"serviceState: failures=0/3 episodeStartedAt=-- fallback=clear candidateSince=-- attempts=0 suppressed=false backoffUntil=-- submitted=false",
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

func TestBuildSupervisorStatusSummaryAggregatesRestartBlockedReasons(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","statusCode":200,"reachable":true},
			"runtimeStatus":{"path":"/api/v1/runtime/status","statusCode":200,"reachable":true},
			"serviceState":{"consecutiveFailures":0,"failureThreshold":3,"containerFallbackCandidate":false},
			"status":{
				"service":"platform-api",
				"runtimes":[{
					"runtimeId":"signal-1",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"ERROR",
					"health":"error",
					"applicationRestartPlan":{
						"candidate":true,
						"enabled":true,
						"healthzOk":true,
						"supported":true,
						"due":false,
						"decision":"blocked",
						"blockedReason":"runtime-restart-not-due"
					}
				},{
					"runtimeId":"signal-2",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"ERROR",
					"health":"error",
					"applicationRestartPlan":{
						"candidate":true,
						"enabled":true,
						"healthzOk":false,
						"supported":true,
						"due":true,
						"decision":"blocked",
						"blockedReason":"runtime-restart-healthz-unhealthy"
					}
				},{
					"runtimeId":"signal-3",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"ERROR",
					"health":"error",
					"applicationRestartPlan":{
						"candidate":true,
						"enabled":true,
						"healthzOk":true,
						"supported":true,
						"due":false,
						"decision":"blocked",
						"blockedReason":"runtime-restart-not-due"
					}
				}]
			}
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	want := "runtimes: total=3 attention=3 restartPlans=3 restartEligible=0 restartBlockedReasons=runtime-restart-healthz-unhealthy:1,runtime-restart-not-due:2 service=platform-api"
	if !strings.Contains(summary, want) {
		t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
	}
}

func TestBuildSupervisorStatusSummaryCountsRuntimeAutoRestartAudit(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","statusCode":200,"reachable":true},
			"runtimeStatus":{"path":"/api/v1/runtime/status","statusCode":200,"reachable":true},
			"serviceState":{"consecutiveFailures":0,"failureThreshold":3,"containerFallbackCandidate":false},
			"status":{
				"service":"platform-api",
				"runtimes":[{
					"runtimeId":"signal-1",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"ERROR",
					"health":"recovering",
					"autoRestartSuppressed":true,
					"autoRestartSuppressedAt":"2026-04-29T07:55:00Z",
					"autoRestartSuppressedReason":"maintenance",
					"autoRestartSuppressedSource":"api"
				},{
					"runtimeId":"signal-2",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"RUNNING",
					"health":"healthy",
					"autoRestartSuppressed":false,
					"autoRestartResumedAt":"2026-04-29T07:59:00Z",
					"autoRestartResumedReason":"maintenance finished",
					"autoRestartResumedSource":"dashboard"
				}]
			}
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	want := "runtimes: total=2 attention=1 service=platform-api autoRestartSuppressed=1 suppressAudit=1 resumeAudit=1"
	if !strings.Contains(summary, want) {
		t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
	}
}

func TestBuildSupervisorStatusSummaryCountsRuntimeLifecycleAudit(t *testing.T) {
	payload := []byte(`{
		"checkedAt":"2026-04-29T08:00:00Z",
		"targets":[{
			"name":"api",
			"baseUrl":"http://127.0.0.1:8080",
			"healthz":{"path":"/healthz","statusCode":200,"reachable":true},
			"runtimeStatus":{"path":"/api/v1/runtime/status","statusCode":200,"reachable":true},
			"serviceState":{"consecutiveFailures":0,"failureThreshold":3,"containerFallbackCandidate":false},
			"status":{
				"service":"platform-api",
				"runtimes":[{
					"runtimeId":"signal-1",
					"runtimeKind":"signal",
					"desiredStatus":"RUNNING",
					"actualStatus":"RUNNING",
					"health":"healthy",
					"restartRequestedAt":"2026-04-29T07:54:00Z",
					"restartRequestedReason":"operator requested rebinding",
					"restartRequestedSource":"api",
					"restartRequestedForce":true,
					"startRequestedAt":"2026-04-29T07:55:00Z",
					"startRequestedReason":"maintenance finished",
					"startRequestedSource":"api"
				},{
					"runtimeId":"signal-2",
					"runtimeKind":"signal",
					"desiredStatus":"STOPPED",
					"actualStatus":"STOPPED",
					"health":"stopped",
					"stopRequestedAt":"2026-04-29T07:59:00Z",
					"stopRequestedReason":"maintenance window",
					"stopRequestedSource":"dashboard",
					"stopRequestedForce":true
				}]
			}
		}]
	}`)

	summary, err := buildSupervisorStatusSummary(payload)
	if err != nil {
		t.Fatalf("build supervisor summary failed: %v", err)
	}
	want := "runtimes: total=2 attention=0 service=platform-api restartAudit=1 startAudit=1 stopAudit=1"
	if !strings.Contains(summary, want) {
		t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
	}
}
