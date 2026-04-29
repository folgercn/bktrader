package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRuntimeSupervisorCollectsHealthAndRuntimeStatus(t *testing.T) {
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{
				"service":   "platform-api",
				"status":    "ok",
				"checkedAt": "2026-04-28T12:30:00Z",
			})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: time.Date(2026, 4, 28, 12, 30, 0, 0, time.UTC),
				Runtimes: []RuntimeStatus{{
					Service:       "platform-api",
					RuntimeID:     "runtime-1",
					RuntimeKind:   "signal",
					DesiredStatus: "RUNNING",
					ActualStatus:  "RUNNING",
					Health:        "healthy",
				}},
			})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisor([]RuntimeSupervisorTarget{{
		Name:    "api",
		BaseURL: server.URL + "/",
	}}, server.Client())
	snapshot := supervisor.Collect(context.Background())
	if snapshot.Policy.ApplicationRestartEnabled || snapshot.Policy.ContainerRestartEnabled || snapshot.Policy.ContainerExecutorConfigured {
		t.Fatalf("expected default supervisor policy to stay read-only, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ServiceFailureThreshold != defaultRuntimeSupervisorServiceFailThresh {
		t.Fatalf("expected default service failure threshold %d, got %+v", defaultRuntimeSupervisorServiceFailThresh, snapshot.Policy)
	}
	if len(snapshot.Targets) != 1 {
		t.Fatalf("expected one target snapshot, got %#v", snapshot.Targets)
	}
	target := snapshot.Targets[0]
	if target.Name != "api" {
		t.Fatalf("expected target name api, got %s", target.Name)
	}
	if !target.Healthz.Reachable || target.Healthz.StatusCode != http.StatusOK {
		t.Fatalf("expected reachable healthz 200, got %+v", target.Healthz)
	}
	if got := target.Healthz.Payload["status"]; got != "ok" {
		t.Fatalf("expected healthz status ok, got %#v", got)
	}
	if !target.RuntimeStatus.Reachable || target.RuntimeStatus.StatusCode != http.StatusOK {
		t.Fatalf("expected reachable runtime status 200, got %+v", target.RuntimeStatus)
	}
	if target.Status == nil || len(target.Status.Runtimes) != 1 {
		t.Fatalf("expected decoded runtime status, got %+v", target.Status)
	}
	if target.Status.Runtimes[0].RuntimeID != "runtime-1" {
		t.Fatalf("expected runtime-1, got %+v", target.Status.Runtimes[0])
	}
	if requested["GET /healthz"] != 1 || requested["GET /api/v1/runtime/status"] != 1 {
		t.Fatalf("expected one GET per read-only endpoint, got %#v", requested)
	}
}

func TestRuntimeSupervisorSnapshotReportsPolicyWithoutFallbackCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		case "/api/v1/runtime/restart":
			t.Errorf("policy reporting must not call control path %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			EnableApplicationRestart: true,
			ServiceFailureThreshold:  4,
			EnableContainerFallback:  true,
		},
	)
	snapshot := supervisor.Collect(context.Background())
	if !snapshot.Policy.ApplicationRestartEnabled || !snapshot.Policy.ContainerRestartEnabled {
		t.Fatalf("expected policy to expose enabled restart settings, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ContainerExecutorConfigured {
		t.Fatalf("expected policy to expose missing container executor, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ServiceFailureThreshold != 4 {
		t.Fatalf("expected service failure threshold 4, got %+v", snapshot.Policy)
	}
	if len(snapshot.Targets) != 1 || snapshot.Targets[0].ContainerFallbackPlan != nil {
		t.Fatalf("expected policy without fallback candidate plan, got %+v", snapshot.Targets)
	}
}

func TestRuntimeSupervisorRecordsProbeFailuresWithoutControlActions(t *testing.T) {
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		if r.Method != http.MethodGet {
			t.Errorf("read-only supervisor must not issue %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{"))
		case "/api/v1/runtime/restart", "/api/v1/runtime/start", "/api/v1/runtime/stop":
			t.Errorf("read-only supervisor must not call control path %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisor([]RuntimeSupervisorTarget{{BaseURL: server.URL}}, server.Client())
	snapshot := supervisor.Collect(context.Background())
	if len(snapshot.Targets) != 1 {
		t.Fatalf("expected one target, got %#v", snapshot.Targets)
	}
	target := snapshot.Targets[0]
	if !target.Healthz.Reachable || target.Healthz.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected reachable 503 healthz, got %+v", target.Healthz)
	}
	if target.Healthz.Error == "" {
		t.Fatal("expected healthz error for non-2xx status")
	}
	if !target.RuntimeStatus.Reachable || target.RuntimeStatus.StatusCode != http.StatusOK {
		t.Fatalf("expected runtime status response to be reachable, got %+v", target.RuntimeStatus)
	}
	if target.RuntimeStatus.Error == "" {
		t.Fatal("expected runtime status decode error")
	}
	if requested["GET /api/v1/runtime/restart"] != 0 || requested["GET /api/v1/runtime/start"] != 0 || requested["GET /api/v1/runtime/stop"] != 0 {
		t.Fatalf("unexpected control endpoint requests: %#v", requested)
	}
}

func TestRuntimeSupervisorMarksContainerFallbackCandidateAfterServiceFailures(t *testing.T) {
	requested := make(map[string]int)
	healthy := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			if !healthy {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		case "/api/v1/runtime/restart":
			t.Errorf("service fallback planning must not call control path %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{ServiceFailureThreshold: 2},
	)
	first := supervisor.Collect(context.Background()).Targets[0]
	if first.ServiceState.ConsecutiveFailures != 1 || first.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected first failure below fallback threshold, got %+v", first.ServiceState)
	}
	if first.ContainerFallbackPlan != nil {
		t.Fatalf("expected no fallback plan below threshold, got %+v", first.ContainerFallbackPlan)
	}
	second := supervisor.Collect(context.Background()).Targets[0]
	if second.ServiceState.ConsecutiveFailures != 2 || !second.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected second failure to become fallback candidate, got %+v", second.ServiceState)
	}
	if second.ServiceState.ContainerFallbackReason == "" || second.ServiceState.LastFailureReason == "" || second.ServiceState.LastFailureAt == nil {
		t.Fatalf("expected fallback reason and failure metadata, got %+v", second.ServiceState)
	}
	if second.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan for candidate, got %+v", second)
	}
	if second.ContainerFallbackPlan.Action != "container-restart" || !second.ContainerFallbackPlan.Candidate {
		t.Fatalf("unexpected fallback plan identity, got %+v", second.ContainerFallbackPlan)
	}
	if second.ContainerFallbackPlan.Executable || second.ContainerFallbackPlan.BlockedReason != "container-restart-disabled" {
		t.Fatalf("expected fallback plan to stay blocked without explicit opt-in, got %+v", second.ContainerFallbackPlan)
	}
	if second.ContainerFallbackPlan.Decision != runtimeSupervisorContainerFallbackDecisionBlocked {
		t.Fatalf("expected blocked fallback decision, got %+v", second.ContainerFallbackPlan)
	}
	if second.ContainerFallbackPlan.Suppressed || second.ContainerFallbackPlan.BackoffActive || !second.ContainerFallbackPlan.SafetyGateOK {
		t.Fatalf("expected dry-run gates to be clear with safety gate ok, got %+v", second.ContainerFallbackPlan)
	}
	if second.ContainerFallbackPlan.Enabled || second.ContainerFallbackPlan.ExecutorConfigured {
		t.Fatalf("expected fallback readiness to show disabled/no executor, got %+v", second.ContainerFallbackPlan)
	}
	if second.ContainerFallbackPlan.Reason != second.ServiceState.ContainerFallbackReason {
		t.Fatalf("expected fallback plan reason to mirror service state, got %+v", second.ContainerFallbackPlan)
	}
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no control action for service fallback candidate, got %#v", requested)
	}

	healthy = true
	recovered := supervisor.Collect(context.Background()).Targets[0]
	if recovered.ServiceState.ConsecutiveFailures != 0 || recovered.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected healthy probe to clear fallback candidate, got %+v", recovered.ServiceState)
	}
	if recovered.ContainerFallbackPlan != nil {
		t.Fatalf("expected healthy probe to clear fallback plan, got %+v", recovered.ContainerFallbackPlan)
	}
	if recovered.ServiceState.LastHealthyAt == nil {
		t.Fatalf("expected healthy probe to record last healthy time, got %+v", recovered.ServiceState)
	}
}

func TestRuntimeSupervisorContainerFallbackOptInStillRequiresExecutor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		case "/api/v1/runtime/restart":
			t.Errorf("container fallback opt-in must not call runtime control path %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold: 1,
			EnableContainerFallback: true,
		},
	)
	target := supervisor.Collect(context.Background()).Targets[0]
	if target.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan for candidate, got %+v", target)
	}
	if target.ContainerFallbackPlan.Executable || target.ContainerFallbackPlan.BlockedReason != "container-executor-not-configured" {
		t.Fatalf("expected opt-in plan to stay blocked without executor, got %+v", target.ContainerFallbackPlan)
	}
	if target.ContainerFallbackPlan.Decision != runtimeSupervisorContainerFallbackDecisionBlocked {
		t.Fatalf("expected opt-in plan to report blocked decision, got %+v", target.ContainerFallbackPlan)
	}
	if !target.ContainerFallbackPlan.Enabled || target.ContainerFallbackPlan.ExecutorConfigured {
		t.Fatalf("expected opt-in readiness to show enabled/no executor, got %+v", target.ContainerFallbackPlan)
	}
}

func TestRuntimeSupervisorContainerFallbackDecisionContract(t *testing.T) {
	base := runtimeSupervisorContainerFallbackDecisionInput{
		Candidate:          true,
		Enabled:            true,
		ExecutorConfigured: true,
		SafetyGateOK:       true,
	}
	tests := []struct {
		name            string
		input           runtimeSupervisorContainerFallbackDecisionInput
		wantDecision    string
		wantExecutable  bool
		wantBlocked     string
		wantEligibleSet bool
	}{
		{
			name:         "not candidate",
			input:        runtimeSupervisorContainerFallbackDecisionInput{},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-not-candidate",
		},
		{
			name:         "disabled",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, ExecutorConfigured: true, SafetyGateOK: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-restart-disabled",
		},
		{
			name:         "executor missing",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, Enabled: true, SafetyGateOK: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-executor-not-configured",
		},
		{
			name: "suppressed",
			input: runtimeSupervisorContainerFallbackDecisionInput{
				Candidate:          true,
				Enabled:            true,
				ExecutorConfigured: true,
				Suppressed:         true,
				SafetyGateOK:       true,
			},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-suppressed",
		},
		{
			name: "backoff active",
			input: runtimeSupervisorContainerFallbackDecisionInput{
				Candidate:          true,
				Enabled:            true,
				ExecutorConfigured: true,
				BackoffActive:      true,
				SafetyGateOK:       true,
			},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-backoff-active",
		},
		{
			name:         "safety gate blocked",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, Enabled: true, ExecutorConfigured: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-safety-gate-blocked",
		},
		{
			name:            "eligible",
			input:           base,
			wantDecision:    runtimeSupervisorContainerFallbackDecisionEligible,
			wantExecutable:  true,
			wantEligibleSet: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateRuntimeSupervisorContainerFallbackDecision(tt.input)
			if got.Decision != tt.wantDecision || got.Executable != tt.wantExecutable || got.BlockedReason != tt.wantBlocked {
				t.Fatalf("unexpected decision: got %+v want decision=%s executable=%t blocked=%s", got, tt.wantDecision, tt.wantExecutable, tt.wantBlocked)
			}
			if tt.wantEligibleSet && got.EligibleReason == "" {
				t.Fatalf("expected eligible reason, got %+v", got)
			}
			if !tt.wantEligibleSet && got.EligibleReason != "" {
				t.Fatalf("did not expect eligible reason, got %+v", got)
			}
		})
	}
}

func TestRuntimeSupervisorDoesNotPlanContainerFallbackForRuntimeStatusDecodeError(t *testing.T) {
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{"))
		case "/api/v1/runtime/restart":
			t.Errorf("runtime status decode errors must not trigger control paths")
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{ServiceFailureThreshold: 1},
	)
	target := supervisor.Collect(context.Background()).Targets[0]
	if target.RuntimeStatus.Error == "" {
		t.Fatalf("expected runtime status decode error, got %+v", target.RuntimeStatus)
	}
	if target.ServiceState.ConsecutiveFailures != 0 || target.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected decode error to stay outside service fallback planning, got %+v", target.ServiceState)
	}
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no control action for decode error, got %#v", requested)
	}
}

func TestRuntimeSupervisorDefaultSkipsDueSignalRestart(t *testing.T) {
	now := time.Now().UTC()
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: now,
				Runtimes: []RuntimeStatus{{
					RuntimeID:       "signal-runtime-1",
					RuntimeKind:     "signal",
					DesiredStatus:   "RUNNING",
					ActualStatus:    "ERROR",
					RestartSeverity: "transient",
					NextRestartAt:   now.Add(-time.Second).Format(time.RFC3339),
				}},
			})
		case "/api/v1/runtime/restart":
			t.Errorf("default supervisor must stay read-only")
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisor([]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}}, server.Client())
	snapshot := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no restart POST by default, got %#v", requested)
	}
	if len(snapshot.Targets) != 1 || len(snapshot.Targets[0].ControlActions) != 0 {
		t.Fatalf("expected no recorded control actions by default, got %#v", snapshot.Targets)
	}
}

func TestRuntimeSupervisorSubmitsDueSignalRestartWhenEnabled(t *testing.T) {
	const token = "supervisor-restart-token"
	now := time.Now().UTC()
	requested := make(map[string]int)
	var restartPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: now,
				Runtimes: []RuntimeStatus{{
					Service:         "platform-api",
					RuntimeID:       "signal-runtime-1",
					RuntimeKind:     "signal",
					DesiredStatus:   "RUNNING",
					ActualStatus:    "ERROR",
					Health:          "error",
					RestartReason:   "runtime-error",
					RestartSeverity: "transient",
					NextRestartAt:   now.Add(-time.Second).Format(time.RFC3339),
				}},
			})
		case "/api/v1/runtime/restart":
			if r.Method != http.MethodPost {
				t.Errorf("expected POST restart, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&restartPayload); err != nil {
				t.Errorf("decode restart payload failed: %v", err)
			}
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "accepted"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		ParseRuntimeSupervisorTargets([]string{"api=" + server.URL}, token),
		server.Client(),
		RuntimeSupervisorOptions{EnableApplicationRestart: true},
	)
	snapshot := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 1 {
		t.Fatalf("expected one restart POST, got %#v", requested)
	}
	if restartPayload["runtimeId"] != "signal-runtime-1" || restartPayload["runtimeKind"] != "signal" {
		t.Fatalf("unexpected restart payload identity: %#v", restartPayload)
	}
	if restartPayload["confirm"] != true || restartPayload["force"] != false {
		t.Fatalf("expected confirm=true force=false, got %#v", restartPayload)
	}
	if got := stringValue(restartPayload["reason"]); got == "" {
		t.Fatalf("expected restart reason in payload, got %#v", restartPayload)
	}
	if len(snapshot.Targets) != 1 || len(snapshot.Targets[0].ControlActions) != 1 {
		t.Fatalf("expected one recorded control action, got %#v", snapshot.Targets)
	}
	action := snapshot.Targets[0].ControlActions[0]
	if !action.Submitted || action.StatusCode != http.StatusOK || action.Error != "" {
		t.Fatalf("expected submitted restart action, got %+v", action)
	}
	second := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 1 {
		t.Fatalf("expected duplicate restart plan to be submitted once, got %#v", requested)
	}
	if len(second.Targets) != 1 || len(second.Targets[0].ControlActions) != 0 {
		t.Fatalf("expected duplicate restart plan to skip new control actions, got %#v", second.Targets)
	}
}

func TestRuntimeSupervisorRetriesRestartAfterFailedPost(t *testing.T) {
	now := time.Now().UTC()
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: now,
				Runtimes: []RuntimeStatus{{
					RuntimeID:       "signal-runtime-1",
					RuntimeKind:     "signal",
					DesiredStatus:   "RUNNING",
					ActualStatus:    "ERROR",
					RestartReason:   "runtime-error",
					RestartSeverity: "transient",
					NextRestartAt:   now.Add(-time.Second).Format(time.RFC3339),
				}},
			})
		case "/api/v1/runtime/restart":
			if requested["POST /api/v1/runtime/restart"] == 1 {
				http.Error(w, "temporary restart failure", http.StatusInternalServerError)
				return
			}
			writeRuntimeSupervisorTestJSONStatus(t, w, http.StatusAccepted, map[string]any{"status": "accepted"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{EnableApplicationRestart: true},
	)

	first := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 1 {
		t.Fatalf("expected first restart POST, got %#v", requested)
	}
	if len(first.Targets) != 1 || len(first.Targets[0].ControlActions) != 1 {
		t.Fatalf("expected first failed action to be recorded, got %#v", first.Targets)
	}
	firstAction := first.Targets[0].ControlActions[0]
	if firstAction.Submitted || firstAction.StatusCode != http.StatusInternalServerError || firstAction.Error == "" {
		t.Fatalf("expected failed restart action with error, got %+v", firstAction)
	}

	second := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 2 {
		t.Fatalf("expected failed restart plan to be retried, got %#v", requested)
	}
	if len(second.Targets) != 1 || len(second.Targets[0].ControlActions) != 1 {
		t.Fatalf("expected second action to be recorded, got %#v", second.Targets)
	}
	secondAction := second.Targets[0].ControlActions[0]
	if !secondAction.Submitted || secondAction.StatusCode != http.StatusAccepted || secondAction.Error != "" {
		t.Fatalf("expected successful retry action, got %+v", secondAction)
	}

	third := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 2 {
		t.Fatalf("expected successful restart plan to be deduplicated, got %#v", requested)
	}
	if len(third.Targets) != 1 || len(third.Targets[0].ControlActions) != 0 {
		t.Fatalf("expected no action after successful submit, got %#v", third.Targets)
	}
}

func TestRuntimeSupervisorSkipsApplicationRestartWhenHealthzFails(t *testing.T) {
	now := time.Now().UTC()
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: now,
				Runtimes: []RuntimeStatus{{
					RuntimeID:       "signal-runtime-1",
					RuntimeKind:     "signal",
					DesiredStatus:   "RUNNING",
					ActualStatus:    "ERROR",
					RestartSeverity: "transient",
					NextRestartAt:   now.Add(-time.Second).Format(time.RFC3339),
				}},
			})
		case "/api/v1/runtime/restart":
			t.Errorf("expected supervisor to skip restart when healthz fails")
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{EnableApplicationRestart: true},
	)
	snapshot := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no restart POST when healthz fails, got %#v", requested)
	}
	if len(snapshot.Targets) != 1 || len(snapshot.Targets[0].ControlActions) != 0 {
		t.Fatalf("expected no recorded control actions when healthz fails, got %#v", snapshot.Targets)
	}
}

func TestRuntimeSupervisorSkipsApplicationRestartWhenSuppressedOrNotDue(t *testing.T) {
	now := time.Now().UTC()
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: now,
				Runtimes: []RuntimeStatus{
					{
						RuntimeID:             "suppressed-signal-runtime",
						RuntimeKind:           "signal",
						DesiredStatus:         "RUNNING",
						ActualStatus:          "ERROR",
						RestartSeverity:       "fatal",
						NextRestartAt:         now.Add(-time.Second).Format(time.RFC3339),
						AutoRestartSuppressed: true,
					},
					{
						RuntimeID:       "future-signal-runtime",
						RuntimeKind:     "signal",
						DesiredStatus:   "RUNNING",
						ActualStatus:    "ERROR",
						RestartSeverity: "transient",
						NextRestartAt:   now.Add(time.Hour).Format(time.RFC3339),
					},
					{
						RuntimeKind:     "signal",
						DesiredStatus:   "RUNNING",
						ActualStatus:    "ERROR",
						RestartSeverity: "transient",
						NextRestartAt:   now.Add(-time.Second).Format(time.RFC3339),
					},
					{
						RuntimeID:       "live-runtime",
						RuntimeKind:     "live-session",
						DesiredStatus:   "RUNNING",
						ActualStatus:    "ERROR",
						RestartSeverity: "transient",
						NextRestartAt:   now.Add(-time.Second).Format(time.RFC3339),
					},
				},
			})
		case "/api/v1/runtime/restart":
			t.Errorf("expected supervisor to skip restart for suppressed/not-due/non-signal runtimes")
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{EnableApplicationRestart: true},
	)
	snapshot := supervisor.Collect(context.Background())
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no restart POST, got %#v", requested)
	}
	if len(snapshot.Targets) != 1 || len(snapshot.Targets[0].ControlActions) != 0 {
		t.Fatalf("expected no recorded control actions, got %#v", snapshot.Targets)
	}
}

func TestRuntimeSupervisorBearerTokenAllowsProtectedTargets(t *testing.T) {
	const token = "supervisor-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/healthz":
			writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{
				Service:   "platform-api",
				CheckedAt: time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	withoutToken := NewRuntimeSupervisor(ParseRuntimeSupervisorTargets([]string{"api=" + server.URL}), server.Client())
	unauthorized := withoutToken.Collect(context.Background()).Targets[0]
	if unauthorized.RuntimeStatus.StatusCode != http.StatusUnauthorized || unauthorized.RuntimeStatus.Error == "" {
		t.Fatalf("expected protected runtime status to reject missing token, got %+v", unauthorized.RuntimeStatus)
	}

	withToken := NewRuntimeSupervisor(ParseRuntimeSupervisorTargets([]string{"api=" + server.URL}, " "+token+" "), server.Client())
	authorized := withToken.Collect(context.Background()).Targets[0]
	if !authorized.Healthz.Reachable || authorized.Healthz.StatusCode != http.StatusOK {
		t.Fatalf("expected authorized healthz probe, got %+v", authorized.Healthz)
	}
	if !authorized.RuntimeStatus.Reachable || authorized.RuntimeStatus.StatusCode != http.StatusOK || authorized.RuntimeStatus.Error != "" {
		t.Fatalf("expected authorized runtime status probe, got %+v", authorized.RuntimeStatus)
	}
}

func TestParseRuntimeSupervisorTargetsSupportsNamedTargets(t *testing.T) {
	targets := ParseRuntimeSupervisorTargets([]string{"api=http://127.0.0.1:8080", " http://127.0.0.1:8081/ "})
	supervisor := NewRuntimeSupervisor(targets, nil)
	normalized := supervisor.Targets()
	if len(normalized) != 2 {
		t.Fatalf("expected two targets, got %#v", normalized)
	}
	if normalized[0].Name != "api" || normalized[0].BaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected named target: %+v", normalized[0])
	}
	if normalized[1].Name != "127.0.0.1:8081" || normalized[1].BaseURL != "http://127.0.0.1:8081" {
		t.Fatalf("unexpected inferred target: %+v", normalized[1])
	}
}

func writeRuntimeSupervisorTestJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	writeRuntimeSupervisorTestJSONStatus(t, w, http.StatusOK, payload)
}

func writeRuntimeSupervisorTestJSONStatus(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Errorf("write json failed: %v", err)
	}
}
