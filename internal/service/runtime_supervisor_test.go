package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type runtimeSupervisorTestContainerExecutor struct {
	configured bool
	kind       string
	dryRun     bool
	calls      int
	lastReason string
	err        error
}

func (e *runtimeSupervisorTestContainerExecutor) Configured() bool {
	return e.configured
}

func (e *runtimeSupervisorTestContainerExecutor) Descriptor() ContainerFallbackExecutorDescriptor {
	return ContainerFallbackExecutorDescriptor{Kind: e.kind, DryRun: e.dryRun}
}

func (e *runtimeSupervisorTestContainerExecutor) Restart(_ context.Context, target RuntimeSupervisorTarget, reason string) (ContainerFallbackExecutionResult, error) {
	e.calls++
	e.lastReason = reason
	if e.err != nil {
		return ContainerFallbackExecutionResult{}, e.err
	}
	return ContainerFallbackExecutionResult{
		Executed: false,
		Message:  fmt.Sprintf("dry-run restart target=%s reason=%s", target.Name, reason),
	}, nil
}

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
	if snapshot.Policy.ContainerFallbackAutoSubmit {
		t.Fatalf("expected default supervisor policy to disable container fallback auto submit, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ServiceFailureThreshold != defaultRuntimeSupervisorServiceFailThresh {
		t.Fatalf("expected default service failure threshold %d, got %+v", defaultRuntimeSupervisorServiceFailThresh, snapshot.Policy)
	}
	if snapshot.Policy.ContainerExecutorKind != runtimeSupervisorContainerExecutorKindNone || !snapshot.Policy.ContainerExecutorDryRun {
		t.Fatalf("expected default policy to expose no dry-run executor, got %+v", snapshot.Policy)
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
	if snapshot.Policy.ContainerFallbackAutoSubmit {
		t.Fatalf("expected fallback auto submit to stay disabled by default, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ContainerExecutorConfigured {
		t.Fatalf("expected policy to expose missing container executor, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ContainerExecutorKind != runtimeSupervisorContainerExecutorKindNone || !snapshot.Policy.ContainerExecutorDryRun {
		t.Fatalf("expected policy to expose no dry-run executor metadata, got %+v", snapshot.Policy)
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
	if first.ServiceState.ServiceFailureEpisodeStartedAt == nil || first.ServiceState.ContainerFallbackCandidateSince != nil {
		t.Fatalf("expected first failure to start episode before fallback candidate, got %+v", first.ServiceState)
	}
	if first.ContainerFallbackPlan != nil {
		t.Fatalf("expected no fallback plan below threshold, got %+v", first.ContainerFallbackPlan)
	}
	second := supervisor.Collect(context.Background()).Targets[0]
	if second.ServiceState.ConsecutiveFailures != 2 || !second.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected second failure to become fallback candidate, got %+v", second.ServiceState)
	}
	if second.ServiceState.ServiceFailureEpisodeStartedAt == nil || !second.ServiceState.ServiceFailureEpisodeStartedAt.Equal(*first.ServiceState.ServiceFailureEpisodeStartedAt) {
		t.Fatalf("expected service failure episode start to remain stable, got first=%+v second=%+v", first.ServiceState, second.ServiceState)
	}
	if second.ServiceState.ContainerFallbackCandidateSince == nil {
		t.Fatalf("expected fallback candidate since when threshold is reached, got %+v", second.ServiceState)
	}
	if second.ServiceState.ContainerFallbackReason == "" || second.ServiceState.LastFailureReason == "" || second.ServiceState.LastFailureAt == nil {
		t.Fatalf("expected fallback reason and failure metadata, got %+v", second.ServiceState)
	}
	if second.ServiceState.ContainerFallbackAttemptCount != 1 || second.ServiceState.LastContainerFallbackDecisionAt == nil || second.ServiceState.LastContainerFallbackDecisionReason != "container-restart-disabled" {
		t.Fatalf("expected first fallback decision audit, got %+v", second.ServiceState)
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
	if second.ContainerFallbackPlan.ExecutorKind != runtimeSupervisorContainerExecutorKindNone || !second.ContainerFallbackPlan.ExecutorDryRun {
		t.Fatalf("expected fallback plan to expose no dry-run executor metadata, got %+v", second.ContainerFallbackPlan)
	}
	if second.ContainerFallbackPlan.Reason != second.ServiceState.ContainerFallbackReason {
		t.Fatalf("expected fallback plan reason to mirror service state, got %+v", second.ContainerFallbackPlan)
	}
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no control action for service fallback candidate, got %#v", requested)
	}

	third := supervisor.Collect(context.Background()).Targets[0]
	if third.ServiceState.ContainerFallbackAttemptCount != 2 || third.ServiceState.LastContainerFallbackDecisionReason != "container-restart-disabled" {
		t.Fatalf("expected fallback decision audit to advance while candidate remains active, got %+v", third.ServiceState)
	}
	if third.ServiceState.ServiceFailureEpisodeStartedAt == nil || !third.ServiceState.ServiceFailureEpisodeStartedAt.Equal(*first.ServiceState.ServiceFailureEpisodeStartedAt) {
		t.Fatalf("expected service failure episode start to remain stable while candidate remains active, got %+v", third.ServiceState)
	}
	if third.ServiceState.ContainerFallbackCandidateSince == nil || !third.ServiceState.ContainerFallbackCandidateSince.Equal(*second.ServiceState.ContainerFallbackCandidateSince) {
		t.Fatalf("expected fallback candidate since to remain stable while candidate remains active, got %+v", third.ServiceState)
	}

	healthy = true
	recoveredSnapshot := supervisor.Collect(context.Background())
	recovered := recoveredSnapshot.Targets[0]
	if recovered.ServiceState.ConsecutiveFailures != 0 || recovered.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected healthy probe to clear fallback candidate, got %+v", recovered.ServiceState)
	}
	if recovered.ServiceState.ServiceFailureEpisodeStartedAt != nil || recovered.ServiceState.ContainerFallbackCandidateSince != nil {
		t.Fatalf("expected healthy probe to clear service failure episode timestamps, got %+v", recovered.ServiceState)
	}
	if recovered.ContainerFallbackPlan != nil {
		t.Fatalf("expected healthy probe to clear fallback plan, got %+v", recovered.ContainerFallbackPlan)
	}
	if recovered.ServiceState.LastHealthyAt == nil {
		t.Fatalf("expected healthy probe to record last healthy time, got %+v", recovered.ServiceState)
	}
	if recovered.ServiceState.ContainerFallbackAttemptCount != 0 || recovered.ServiceState.LastContainerFallbackDecisionAt != nil || recovered.ServiceState.LastContainerFallbackDecisionReason != "" {
		t.Fatalf("expected healthy probe to clear fallback decision audit, got %+v", recovered.ServiceState)
	}
	if len(recoveredSnapshot.ServiceFailureEpisodes) != 1 {
		t.Fatalf("expected recovered failure episode audit, got %+v", recoveredSnapshot.ServiceFailureEpisodes)
	}
	episode := recoveredSnapshot.ServiceFailureEpisodes[0]
	if episode.TargetName != "api" || episode.MaxConsecutiveFailures != 3 || !episode.ContainerFallbackCandidate {
		t.Fatalf("unexpected recovered failure episode identity, got %+v", episode)
	}
	if episode.ContainerFallbackCandidateSince == nil || episode.ContainerFallbackAttemptCount != 2 || episode.LastContainerFallbackDecisionReason != "container-restart-disabled" {
		t.Fatalf("expected recovered episode fallback decision audit, got %+v", episode)
	}
	if episode.DurationSeconds < 0 || episode.LastFailureReason == "" || episode.LastFailureAt == nil {
		t.Fatalf("expected recovered episode failure timing and reason, got %+v", episode)
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
	if target.ServiceState.ContainerFallbackAttemptCount != 1 || target.ServiceState.LastContainerFallbackDecisionReason != "container-executor-not-configured" {
		t.Fatalf("expected opt-in plan to audit executor blocker, got %+v", target.ServiceState)
	}
	if !target.ContainerFallbackPlan.Enabled || target.ContainerFallbackPlan.ExecutorConfigured {
		t.Fatalf("expected opt-in readiness to show enabled/no executor, got %+v", target.ContainerFallbackPlan)
	}
	if target.ContainerFallbackPlan.ExecutorKind != runtimeSupervisorContainerExecutorKindNone || !target.ContainerFallbackPlan.ExecutorDryRun {
		t.Fatalf("expected opt-in plan to expose no dry-run executor metadata, got %+v", target.ContainerFallbackPlan)
	}
}

func TestRuntimeSupervisorContainerFallbackAutoSubmitDisabledLeavesExecutablePlanForManualSubmit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := &runtimeSupervisorTestContainerExecutor{
		configured: true,
		kind:       runtimeSupervisorContainerExecutorKindNoop,
		dryRun:     true,
	}
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   true,
			ContainerFallbackExecutor: executor,
		},
	)

	snapshot := supervisor.Collect(context.Background())
	if snapshot.Policy.ContainerFallbackAutoSubmit {
		t.Fatalf("expected policy to keep auto submit disabled, got %+v", snapshot.Policy)
	}
	target := snapshot.Targets[0]
	if target.ContainerFallbackPlan == nil || !target.ContainerFallbackPlan.Executable || target.ContainerFallbackPlan.Decision != runtimeSupervisorContainerFallbackDecisionEligible {
		t.Fatalf("expected executable manual-submit plan without auto submission, got %+v", target.ContainerFallbackPlan)
	}
	if executor.calls != 0 || len(snapshot.ContainerFallbackActions) != 0 || target.ServiceState.ContainerFallbackSubmitted {
		t.Fatalf("expected disabled auto submit to leave executor untouched, calls=%d actions=%+v state=%+v", executor.calls, snapshot.ContainerFallbackActions, target.ServiceState)
	}

	result, err := supervisor.SubmitContainerFallback(context.Background(), "api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator approved manual fallback",
		Source:  "ctl",
	})
	if err != nil {
		t.Fatalf("manual container fallback submit failed while auto submit disabled: %v", err)
	}
	if executor.calls != 1 || result.Action == nil || !result.Action.Submitted || result.Action.Source != "ctl" {
		t.Fatalf("expected manual submit to call executor once with audit, calls=%d result=%+v", executor.calls, result)
	}
	if result.Plan == nil || result.Plan.BlockedReason != "container-fallback-already-submitted" {
		t.Fatalf("expected manual submit to leave duplicate blocker, got %+v", result.Plan)
	}
}

func TestRuntimeSupervisorDryRunContainerFallbackExecutorRecordsActionOncePerFailureEpisode(t *testing.T) {
	requested := make(map[string]int)
	healthy := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			if healthy {
				writeRuntimeSupervisorTestJSON(t, w, map[string]any{"status": "ok"})
				return
			}
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		case "/api/v1/runtime/restart":
			t.Errorf("noop container fallback executor plumbing must not call runtime control path %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := &runtimeSupervisorTestContainerExecutor{
		configured: true,
		kind:       runtimeSupervisorContainerExecutorKindNoop,
		dryRun:     true,
	}
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:     1,
			EnableContainerFallback:     true,
			ContainerFallbackAutoSubmit: true,
			ContainerFallbackExecutor:   executor,
		},
	)
	snapshot := supervisor.Collect(context.Background())
	if !snapshot.Policy.ContainerExecutorConfigured {
		t.Fatalf("expected noop executor readiness in policy, got %+v", snapshot.Policy)
	}
	if snapshot.Policy.ContainerExecutorKind != runtimeSupervisorContainerExecutorKindNoop || !snapshot.Policy.ContainerExecutorDryRun {
		t.Fatalf("expected noop executor metadata in policy, got %+v", snapshot.Policy)
	}
	target := snapshot.Targets[0]
	if target.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan for candidate, got %+v", target)
	}
	if !target.ContainerFallbackPlan.ExecutorConfigured || target.ContainerFallbackPlan.Executable {
		t.Fatalf("expected submitted noop action to block duplicate fallback in snapshot, got %+v", target.ContainerFallbackPlan)
	}
	if target.ContainerFallbackPlan.ExecutorKind != runtimeSupervisorContainerExecutorKindNoop || !target.ContainerFallbackPlan.ExecutorDryRun {
		t.Fatalf("expected eligible noop plan to expose dry-run executor metadata, got %+v", target.ContainerFallbackPlan)
	}
	if target.ContainerFallbackPlan.Decision != runtimeSupervisorContainerFallbackDecisionBlocked || target.ContainerFallbackPlan.BlockedReason != "container-fallback-already-submitted" || !target.ContainerFallbackPlan.Duplicate {
		t.Fatalf("expected dry-run submission to leave duplicate blocker in snapshot, got %+v", target.ContainerFallbackPlan)
	}
	if !target.ServiceState.ContainerFallbackSubmitted || target.ServiceState.ContainerFallbackSubmittedAt == nil || target.ServiceState.ContainerFallbackSubmittedMessage == "" {
		t.Fatalf("expected dry-run submission audit in service state, got %+v", target.ServiceState)
	}
	if target.ServiceState.LastContainerFallbackDecisionReason != "container-fallback-already-submitted" {
		t.Fatalf("expected duplicate decision audit after dry-run submit, got %+v", target.ServiceState)
	}
	if executor.calls != 1 {
		t.Fatalf("expected one dry-run container fallback executor call, got %d", executor.calls)
	}
	if len(snapshot.ContainerFallbackActions) != 1 {
		t.Fatalf("expected one container fallback action audit, got %+v", snapshot.ContainerFallbackActions)
	}
	action := snapshot.ContainerFallbackActions[0]
	if action.Action != "container-restart" || action.TargetName != "api" || action.ExecutorKind != runtimeSupervisorContainerExecutorKindNoop || !action.ExecutorDryRun {
		t.Fatalf("unexpected container fallback action identity, got %+v", action)
	}
	if !action.Submitted || action.Executed || action.Message == "" || action.Error != "" {
		t.Fatalf("expected dry-run non-executing action audit, got %+v", action)
	}
	if action.ServiceFailureEpisodeStartedAt == nil || action.ContainerFallbackCandidateSince == nil {
		t.Fatalf("expected action audit to carry failure episode anchors, got %+v", action)
	}
	next := supervisor.Collect(context.Background())
	if executor.calls != 1 || len(next.ContainerFallbackActions) != 1 {
		t.Fatalf("expected container fallback action to be deduped while failure episode remains active, calls=%d actions=%+v", executor.calls, next.ContainerFallbackActions)
	}
	if next.Targets[0].ContainerFallbackPlan == nil || next.Targets[0].ContainerFallbackPlan.BlockedReason != "container-fallback-already-submitted" {
		t.Fatalf("expected duplicate blocker on next collect, got %+v", next.Targets[0].ContainerFallbackPlan)
	}
	healthy = true
	recovered := supervisor.Collect(context.Background())
	if len(recovered.ContainerFallbackActions) != 1 {
		t.Fatalf("expected healthy collect to preserve action history, got %+v", recovered.ContainerFallbackActions)
	}
	if len(recovered.ServiceFailureEpisodes) != 1 || !recovered.ServiceFailureEpisodes[0].ContainerFallbackSubmitted || recovered.ServiceFailureEpisodes[0].ContainerFallbackSubmittedAt == nil {
		t.Fatalf("expected recovered episode to preserve dry-run submission audit, got %+v", recovered.ServiceFailureEpisodes)
	}
	healthy = false
	newEpisode := supervisor.Collect(context.Background())
	if executor.calls != 2 || len(newEpisode.ContainerFallbackActions) != 2 {
		t.Fatalf("expected recovered target to allow a new dry-run action in the next failure episode, calls=%d actions=%+v", executor.calls, newEpisode.ContainerFallbackActions)
	}
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no control action for noop executor readiness, got %#v", requested)
	}
}

func TestRuntimeSupervisorContainerFallbackExecutorErrorSetsBackoffAndAllowsManualRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := &runtimeSupervisorTestContainerExecutor{
		configured: true,
		kind:       runtimeSupervisorContainerExecutorKindNoop,
		dryRun:     true,
		err:        errors.New("noop executor failed"),
	}
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:     1,
			EnableContainerFallback:     true,
			ContainerFallbackAutoSubmit: true,
			ContainerFallbackExecutor:   executor,
		},
	)

	snapshot := supervisor.Collect(context.Background())
	if executor.calls != 1 || len(snapshot.ContainerFallbackActions) != 1 {
		t.Fatalf("expected one failed dry-run submission, calls=%d actions=%+v", executor.calls, snapshot.ContainerFallbackActions)
	}
	action := snapshot.ContainerFallbackActions[0]
	if action.Error != "noop executor failed" || action.BackoffUntil == nil || action.BackoffSeconds != 300 {
		t.Fatalf("expected action error backoff audit, got %+v", action)
	}
	if action.ServiceFailureEpisodeStartedAt == nil || action.ContainerFallbackCandidateSince == nil {
		t.Fatalf("expected failed action audit to carry failure episode anchors, got %+v", action)
	}
	target := snapshot.Targets[0]
	if target.ContainerFallbackPlan == nil || target.ContainerFallbackPlan.BlockedReason != "container-fallback-backoff-active" || !target.ContainerFallbackPlan.BackoffActive {
		t.Fatalf("expected failed executor to leave active backoff blocker, got %+v", target.ContainerFallbackPlan)
	}
	if !target.ServiceState.ContainerFallbackSubmitted || target.ServiceState.ContainerFallbackSubmittedError != "noop executor failed" {
		t.Fatalf("expected submitted error state, got %+v", target.ServiceState)
	}
	if target.ServiceState.ContainerFallbackBackoffUntil == nil || target.ServiceState.ContainerFallbackBackoffSetAt == nil {
		t.Fatalf("expected backoff timestamps after executor error, got %+v", target.ServiceState)
	}
	if target.ServiceState.ContainerFallbackBackoffReason != "container fallback executor error: noop executor failed" || target.ServiceState.ContainerFallbackBackoffSource != "supervisor" {
		t.Fatalf("expected supervisor error backoff reason/source, got %+v", target.ServiceState)
	}
	if target.ServiceState.LastContainerFallbackDecisionReason != "container-fallback-backoff-active" {
		t.Fatalf("expected backoff decision audit after executor error, got %+v", target.ServiceState)
	}

	next := supervisor.Collect(context.Background())
	if executor.calls != 1 || len(next.ContainerFallbackActions) != 1 {
		t.Fatalf("expected active backoff/submitted state to prevent duplicate executor calls, calls=%d actions=%+v", executor.calls, next.ContainerFallbackActions)
	}

	executor.err = nil
	cleared, err := supervisor.ClearContainerFallbackBackoff("api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator reviewed failed dry-run",
		Source:  "ctl",
	})
	if err != nil {
		t.Fatalf("clear container fallback backoff failed: %v", err)
	}
	if cleared.ServiceState.ContainerFallbackSubmitted || cleared.ServiceState.ContainerFallbackSubmittedError != "" {
		t.Fatalf("expected clear-backoff to clear submitted dedupe state, got %+v", cleared.ServiceState)
	}
	clearedSnapshot := supervisor.LastSnapshot()
	if clearedSnapshot.Targets[0].ContainerFallbackPlan == nil || !clearedSnapshot.Targets[0].ContainerFallbackPlan.Executable {
		t.Fatalf("expected clear-backoff to expose retry-eligible plan before submit, got %+v", clearedSnapshot.Targets[0].ContainerFallbackPlan)
	}

	retried := supervisor.Collect(context.Background())
	if executor.calls != 2 || len(retried.ContainerFallbackActions) != 2 {
		t.Fatalf("expected clear-backoff to allow one retry submission, calls=%d actions=%+v", executor.calls, retried.ContainerFallbackActions)
	}
	if retried.ContainerFallbackActions[0].Error != "" || retried.ContainerFallbackActions[0].Message == "" {
		t.Fatalf("expected retry to record successful dry-run message, got %+v", retried.ContainerFallbackActions[0])
	}
	if retried.Targets[0].ContainerFallbackPlan == nil || retried.Targets[0].ContainerFallbackPlan.BlockedReason != "container-fallback-already-submitted" {
		t.Fatalf("expected successful retry to leave duplicate blocker, got %+v", retried.Targets[0].ContainerFallbackPlan)
	}
}

func TestRuntimeSupervisorContainerFallbackExecutorRequiresArmedGateBeforeNonDryRunSubmit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := &runtimeSupervisorTestContainerExecutor{
		configured: true,
		kind:       "docker",
		dryRun:     false,
	}
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   true,
			ContainerFallbackExecutor: executor,
		},
	)
	target := supervisor.Collect(context.Background()).Targets[0]
	if target.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan for candidate, got %+v", target)
	}
	if target.ContainerFallbackPlan.Executable || target.ContainerFallbackPlan.BlockedReason != "container-executor-not-armed" {
		t.Fatalf("expected non-dry-run executor to stay blocked, got %+v", target.ContainerFallbackPlan)
	}
	if target.ServiceState.LastContainerFallbackDecisionReason != "container-executor-not-armed" {
		t.Fatalf("expected non-dry-run decision audit, got %+v", target.ServiceState)
	}
	if executor.calls != 0 {
		t.Fatalf("expected non-dry-run executor not to be called, got %d", executor.calls)
	}
}

func TestRuntimeSupervisorCommandContainerFallbackExecutorExecutesWhenArmedAndAllowlisted(t *testing.T) {
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.Method+" "+r.URL.Path]++
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		case "/api/v1/runtime/restart":
			t.Errorf("command container fallback executor must not call runtime control path %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := newRuntimeSupervisorTestCommandExecutor(t, "api", "--runtime-supervisor-command-executor-success")
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:        1,
			EnableContainerFallback:        true,
			ContainerFallbackAutoSubmit:    true,
			ContainerFallbackExecutor:      executor,
			ContainerFallbackExecutorArmed: true,
		},
	)
	snapshot := supervisor.Collect(context.Background())
	if !snapshot.Policy.ContainerExecutorConfigured || snapshot.Policy.ContainerExecutorKind != runtimeSupervisorContainerExecutorKindCommand || snapshot.Policy.ContainerExecutorDryRun || !snapshot.Policy.ContainerExecutorArmed {
		t.Fatalf("expected armed command executor policy, got %+v", snapshot.Policy)
	}
	if len(snapshot.ContainerFallbackActions) != 1 {
		t.Fatalf("expected one command fallback action, got %+v", snapshot.ContainerFallbackActions)
	}
	action := snapshot.ContainerFallbackActions[0]
	if !action.Submitted || !action.Executed || action.ExecutorDryRun || action.ExecutorKind != runtimeSupervisorContainerExecutorKindCommand || action.Error != "" {
		t.Fatalf("expected submitted/executed command fallback action, got %+v", action)
	}
	if action.ExecutorPreview == nil || action.ExecutorPreview.Kind != runtimeSupervisorContainerExecutorKindCommand || action.ExecutorPreview.CommandPath == "" || action.ExecutorPreview.TimeoutSeconds != 5 {
		t.Fatalf("expected command preview on executed action, got %+v", action.ExecutorPreview)
	}
	if !strings.Contains(action.Message, "helper restart ok") {
		t.Fatalf("expected command output audit, got %+v", action)
	}
	target := snapshot.Targets[0]
	if target.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan for command executor, got %+v", target)
	}
	if target.ContainerFallbackPlan.BlockedReason != "container-fallback-already-submitted" || !target.ContainerFallbackPlan.Duplicate || !target.ContainerFallbackPlan.ExecutorArmed || !target.ContainerFallbackPlan.TargetAllowed {
		t.Fatalf("expected executed command fallback to leave duplicate blocker with gates visible, got %+v", target.ContainerFallbackPlan)
	}
	preview := target.ContainerFallbackPlan.ExecutorPreview
	if preview == nil || preview.Kind != runtimeSupervisorContainerExecutorKindCommand || preview.CommandPath == "" || len(preview.CommandArgs) != 3 || preview.TimeoutSeconds != 5 {
		t.Fatalf("expected command preview on fallback plan, got %+v", preview)
	}
	if !target.ServiceState.ContainerFallbackSubmitted || target.ServiceState.ContainerFallbackSubmittedMessage == "" || target.ServiceState.ContainerFallbackSubmittedError != "" {
		t.Fatalf("expected command submission audit in service state, got %+v", target.ServiceState)
	}
	if requested["POST /api/v1/runtime/restart"] != 0 {
		t.Fatalf("expected no runtime restart control call from command executor, got %#v", requested)
	}
}

func TestRuntimeSupervisorCommandContainerFallbackExecutorBlocksTargetsOutsideAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := newRuntimeSupervisorTestCommandExecutor(t, "worker", "--runtime-supervisor-command-executor-success")
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:        1,
			EnableContainerFallback:        true,
			ContainerFallbackAutoSubmit:    true,
			ContainerFallbackExecutor:      executor,
			ContainerFallbackExecutorArmed: true,
		},
	)
	snapshot := supervisor.Collect(context.Background())
	target := snapshot.Targets[0]
	if target.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan for command executor, got %+v", target)
	}
	if target.ContainerFallbackPlan.Executable || target.ContainerFallbackPlan.TargetAllowed || target.ContainerFallbackPlan.BlockedReason != "container-executor-target-not-allowlisted" {
		t.Fatalf("expected non-allowlisted target to block command executor, got %+v", target.ContainerFallbackPlan)
	}
	if target.ContainerFallbackPlan.ExecutorPreview != nil {
		t.Fatalf("expected no command preview for non-allowlisted target, got %+v", target.ContainerFallbackPlan.ExecutorPreview)
	}
	if len(snapshot.ContainerFallbackActions) != 0 || target.ServiceState.ContainerFallbackSubmitted {
		t.Fatalf("expected no command submission for non-allowlisted target, actions=%+v state=%+v", snapshot.ContainerFallbackActions, target.ServiceState)
	}
}

func TestRuntimeSupervisorCommandContainerFallbackExecutorRejectsDuplicateNormalizedTargets(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	if _, err := NewCommandContainerFallbackExecutor(map[string]CommandContainerFallbackSpec{
		"api":  {Path: executable},
		" api": {Path: executable},
	}); err == nil {
		t.Fatalf("expected duplicate normalized command target names to fail")
	}
}

func TestRuntimeSupervisorCommandContainerFallbackExecutorFailureSetsBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := newRuntimeSupervisorTestCommandExecutor(t, "api", "--runtime-supervisor-command-executor-fail")
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:        1,
			EnableContainerFallback:        true,
			ContainerFallbackAutoSubmit:    true,
			ContainerFallbackExecutor:      executor,
			ContainerFallbackExecutorArmed: true,
		},
	)
	snapshot := supervisor.Collect(context.Background())
	if len(snapshot.ContainerFallbackActions) != 1 {
		t.Fatalf("expected one failed command fallback action, got %+v", snapshot.ContainerFallbackActions)
	}
	action := snapshot.ContainerFallbackActions[0]
	if action.Error == "" || !strings.Contains(action.Error, "helper restart failed") || action.BackoffUntil == nil || action.BackoffSeconds != 300 {
		t.Fatalf("expected command failure and backoff audit, got %+v", action)
	}
	target := snapshot.Targets[0]
	if target.ContainerFallbackPlan == nil || target.ContainerFallbackPlan.BlockedReason != "container-fallback-backoff-active" || !target.ContainerFallbackPlan.BackoffActive {
		t.Fatalf("expected failed command executor to leave backoff blocker, got %+v", target.ContainerFallbackPlan)
	}
	if !target.ServiceState.ContainerFallbackSubmitted || !strings.Contains(target.ServiceState.ContainerFallbackSubmittedError, "helper restart failed") {
		t.Fatalf("expected failed command submission audit in service state, got %+v", target.ServiceState)
	}
}

func TestRuntimeSupervisorManualContainerFallbackSubmitRecordsOperatorAudit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := &runtimeSupervisorTestContainerExecutor{
		configured: true,
		kind:       runtimeSupervisorContainerExecutorKindNoop,
		dryRun:     true,
	}
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: server.URL}},
		server.Client(),
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   false,
			ContainerFallbackExecutor: executor,
		},
	)
	candidate := supervisor.Collect(context.Background()).Targets[0]
	if candidate.ContainerFallbackPlan == nil || candidate.ContainerFallbackPlan.Executable || candidate.ContainerFallbackPlan.BlockedReason != "container-restart-disabled" {
		t.Fatalf("expected disabled candidate plan before manual submit, got %+v", candidate.ContainerFallbackPlan)
	}
	if executor.calls != 0 {
		t.Fatalf("expected disabled auto submit to skip executor, got %d", executor.calls)
	}

	supervisor.options.EnableContainerFallback = true
	result, err := supervisor.SubmitContainerFallback(context.Background(), "api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator reviewed static command preview",
		Source:  "ctl",
	})
	if err != nil {
		t.Fatalf("manual container fallback submit failed: %v", err)
	}
	if executor.calls != 1 || executor.lastReason != "operator reviewed static command preview" {
		t.Fatalf("expected one executor call with operator reason, calls=%d reason=%q", executor.calls, executor.lastReason)
	}
	if result.Action == nil || !result.Action.Submitted || result.Action.Source != "ctl" || result.Action.PlanReason == "" {
		t.Fatalf("expected manual action to carry source and plan reason, got %+v", result.Action)
	}
	if !strings.Contains(result.Action.PlanReason, "service probes failed 1/1") {
		t.Fatalf("expected plan reason to preserve service failure context, got %+v", result.Action)
	}
	if result.ServiceState.ContainerFallbackSubmittedReason != "operator reviewed static command preview" || result.ServiceState.ContainerFallbackSubmittedMessage == "" {
		t.Fatalf("expected submitted state to carry operator reason and message, got %+v", result.ServiceState)
	}
	if result.Plan == nil || result.Plan.BlockedReason != "container-fallback-already-submitted" || !result.Plan.Duplicate {
		t.Fatalf("expected post-submit plan to show duplicate blocker, got %+v", result.Plan)
	}
	snapshot := supervisor.LastSnapshot()
	if len(snapshot.ContainerFallbackActions) != 1 || snapshot.ContainerFallbackActions[0].Source != "ctl" {
		t.Fatalf("expected manual action audit in last snapshot, got %+v", snapshot.ContainerFallbackActions)
	}
}

func TestRuntimeSupervisorManualContainerFallbackSubmitRejectsMissingCandidate(t *testing.T) {
	executor := &runtimeSupervisorTestContainerExecutor{
		configured: true,
		kind:       runtimeSupervisorContainerExecutorKindNoop,
		dryRun:     true,
	}
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: "http://127.0.0.1:8080"}},
		nil,
		RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   true,
			ContainerFallbackExecutor: executor,
		},
	)
	result, err := supervisor.SubmitContainerFallback(context.Background(), "api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator reviewed static command preview",
		Source:  "ctl",
	})
	if !errors.Is(err, ErrRuntimeSupervisorContainerFallbackBlocked) {
		t.Fatalf("expected blocked submit error, got result=%+v err=%v", result, err)
	}
	if executor.calls != 0 || result.Plan != nil || result.ServiceState.ContainerFallbackCandidate {
		t.Fatalf("expected missing candidate to skip executor, calls=%d result=%+v", executor.calls, result)
	}
}

func TestRuntimeSupervisorContainerFallbackSuppressionBlocksEligiblePlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
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
			ServiceFailureThreshold:     1,
			EnableContainerFallback:     true,
			ContainerFallbackAutoSubmit: true,
			ContainerFallbackExecutor:   NewNoopContainerFallbackExecutor(true),
		},
	)
	suppressed, err := supervisor.SuppressContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator paused container fallback",
		Source:  "ctl",
	})
	if err != nil {
		t.Fatalf("suppress container fallback failed: %v", err)
	}
	if !suppressed.Suppressed || !suppressed.ServiceState.ContainerFallbackSuppressed {
		t.Fatalf("expected suppressed result, got %+v", suppressed)
	}
	if suppressed.ServiceState.ContainerFallbackSuppressedReason != "operator paused container fallback" || suppressed.ServiceState.ContainerFallbackSuppressedSource != "ctl" || suppressed.ServiceState.ContainerFallbackSuppressedAt == nil {
		t.Fatalf("expected suppression audit fields, got %+v", suppressed.ServiceState)
	}

	blocked := supervisor.Collect(context.Background()).Targets[0]
	if blocked.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan after suppression, got %+v", blocked)
	}
	if blocked.ContainerFallbackPlan.Executable || blocked.ContainerFallbackPlan.BlockedReason != "container-fallback-suppressed" {
		t.Fatalf("expected suppressed plan to block execution, got %+v", blocked.ContainerFallbackPlan)
	}
	if blocked.ServiceState.LastContainerFallbackDecisionReason != "container-fallback-suppressed" {
		t.Fatalf("expected suppressed decision audit, got %+v", blocked.ServiceState)
	}
	if len(supervisor.LastSnapshot().ContainerFallbackControls) != 1 || supervisor.LastSnapshot().ContainerFallbackControls[0].Action != "suppress-container-fallback" {
		t.Fatalf("expected suppression control audit in last snapshot, got %+v", supervisor.LastSnapshot().ContainerFallbackControls)
	}

	resumed, err := supervisor.ResumeContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "maintenance finished",
		Source:  "ctl",
	})
	if err != nil {
		t.Fatalf("resume container fallback failed: %v", err)
	}
	if resumed.Suppressed || resumed.ServiceState.ContainerFallbackSuppressed || resumed.ServiceState.ContainerFallbackSuppressedReason != "" {
		t.Fatalf("expected resumed state to clear suppression, got %+v", resumed)
	}
	if resumed.ServiceState.ContainerFallbackResumedReason != "maintenance finished" || resumed.ServiceState.ContainerFallbackResumedSource != "ctl" || resumed.ServiceState.ContainerFallbackResumedAt == nil {
		t.Fatalf("expected resume audit fields, got %+v", resumed.ServiceState)
	}
	resumedSnapshot := supervisor.LastSnapshot()
	if len(resumedSnapshot.ContainerFallbackControls) != 2 || resumedSnapshot.ContainerFallbackControls[0].Action != "resume-container-fallback" {
		t.Fatalf("expected resume control audit in last snapshot, got %+v", resumedSnapshot.ContainerFallbackControls)
	}
	if resumedSnapshot.Targets[0].ContainerFallbackPlan == nil || !resumedSnapshot.Targets[0].ContainerFallbackPlan.Executable {
		t.Fatalf("expected resume to make the in-memory plan eligible before executor submission, got %+v", resumedSnapshot.Targets[0].ContainerFallbackPlan)
	}

	submitted := supervisor.Collect(context.Background())
	if len(submitted.ContainerFallbackActions) != 1 {
		t.Fatalf("expected resume to allow one noop dry-run action, got %+v", submitted.ContainerFallbackActions)
	}
	if submitted.Targets[0].ContainerFallbackPlan == nil || submitted.Targets[0].ContainerFallbackPlan.BlockedReason != "container-fallback-already-submitted" {
		t.Fatalf("expected dry-run submission to leave duplicate blocker, got %+v", submitted.Targets[0].ContainerFallbackPlan)
	}
}

func TestRuntimeSupervisorContainerFallbackBackoffBlocksEligiblePlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeRuntimeSupervisorTestJSON(t, w, RuntimeStatusSnapshot{Service: "platform-api"})
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
			ServiceFailureThreshold:     1,
			EnableContainerFallback:     true,
			ContainerFallbackAutoSubmit: true,
			ContainerFallbackExecutor:   NewNoopContainerFallbackExecutor(true),
		},
	)
	deferred, err := supervisor.DeferContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm:         true,
		Reason:          "operator cooling down restart loop",
		Source:          "ctl",
		BackoffDuration: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("defer container fallback failed: %v", err)
	}
	if deferred.BackoffUntil == nil || deferred.ServiceState.ContainerFallbackBackoffUntil == nil {
		t.Fatalf("expected backoff until in result, got %+v", deferred)
	}
	if deferred.ServiceState.ContainerFallbackBackoffReason != "operator cooling down restart loop" || deferred.ServiceState.ContainerFallbackBackoffSource != "ctl" || deferred.ServiceState.ContainerFallbackBackoffSetAt == nil {
		t.Fatalf("expected backoff audit fields, got %+v", deferred.ServiceState)
	}

	blocked := supervisor.Collect(context.Background()).Targets[0]
	if blocked.ContainerFallbackPlan == nil {
		t.Fatalf("expected fallback plan after backoff, got %+v", blocked)
	}
	if blocked.ContainerFallbackPlan.Executable || blocked.ContainerFallbackPlan.BlockedReason != "container-fallback-backoff-active" {
		t.Fatalf("expected backoff plan to block execution, got %+v", blocked.ContainerFallbackPlan)
	}
	if blocked.ServiceState.LastContainerFallbackDecisionReason != "container-fallback-backoff-active" {
		t.Fatalf("expected backoff decision audit, got %+v", blocked.ServiceState)
	}

	cleared, err := supervisor.ClearContainerFallbackBackoff("api", RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator verified target stable",
		Source:  "ctl",
	})
	if err != nil {
		t.Fatalf("clear container fallback backoff failed: %v", err)
	}
	if cleared.BackoffUntil != nil || cleared.ServiceState.ContainerFallbackBackoffUntil != nil {
		t.Fatalf("expected clear to remove backoff, got %+v", cleared)
	}
	if cleared.ServiceState.ContainerFallbackBackoffClearedReason != "operator verified target stable" || cleared.ServiceState.ContainerFallbackBackoffClearedSource != "ctl" || cleared.ServiceState.ContainerFallbackBackoffClearedAt == nil {
		t.Fatalf("expected backoff clear audit fields, got %+v", cleared.ServiceState)
	}
	clearedSnapshot := supervisor.LastSnapshot()
	if len(clearedSnapshot.ContainerFallbackControls) != 2 || clearedSnapshot.ContainerFallbackControls[0].Action != "clear-container-fallback-backoff" {
		t.Fatalf("expected clear-backoff control audit in last snapshot, got %+v", clearedSnapshot.ContainerFallbackControls)
	}
	if clearedSnapshot.Targets[0].ContainerFallbackPlan == nil || !clearedSnapshot.Targets[0].ContainerFallbackPlan.Executable {
		t.Fatalf("expected clear to make the in-memory plan eligible before executor submission, got %+v", clearedSnapshot.Targets[0].ContainerFallbackPlan)
	}

	submitted := supervisor.Collect(context.Background())
	if len(submitted.ContainerFallbackActions) != 1 {
		t.Fatalf("expected clear to allow one noop dry-run action, got %+v", submitted.ContainerFallbackActions)
	}
	if submitted.Targets[0].ContainerFallbackPlan == nil || submitted.Targets[0].ContainerFallbackPlan.BlockedReason != "container-fallback-already-submitted" {
		t.Fatalf("expected dry-run submission to leave duplicate blocker, got %+v", submitted.Targets[0].ContainerFallbackPlan)
	}
}

func TestRuntimeSupervisorContainerFallbackControlValidation(t *testing.T) {
	supervisor := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{{Name: "api", BaseURL: "http://127.0.0.1:8080"}},
		nil,
		RuntimeSupervisorOptions{},
	)
	if _, err := supervisor.SuppressContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{Reason: "maintenance"}); !errors.Is(err, ErrRuntimeSupervisorControlConfirmRequired) {
		t.Fatalf("expected confirm required, got %v", err)
	}
	if _, err := supervisor.SuppressContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{Confirm: true}); !errors.Is(err, ErrRuntimeSupervisorControlReasonRequired) {
		t.Fatalf("expected reason required, got %v", err)
	}
	if _, err := supervisor.DeferContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{Confirm: true, Reason: "maintenance"}); !errors.Is(err, ErrRuntimeSupervisorBackoffDurationRequired) {
		t.Fatalf("expected backoff duration required, got %v", err)
	}
	if _, err := supervisor.DeferContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{Confirm: true, Reason: "maintenance", BackoffDuration: maxRuntimeSupervisorContainerFallbackBackoff + time.Second}); !errors.Is(err, ErrRuntimeSupervisorBackoffDurationTooLarge) {
		t.Fatalf("expected backoff duration too large, got %v", err)
	}
	if _, err := supervisor.SuppressContainerFallback("", RuntimeSupervisorContainerFallbackControlOptions{Confirm: true, Reason: "maintenance"}); !errors.Is(err, ErrRuntimeSupervisorTargetRequired) {
		t.Fatalf("expected target required, got %v", err)
	}
	if _, err := supervisor.SuppressContainerFallback("missing", RuntimeSupervisorContainerFallbackControlOptions{Confirm: true, Reason: "maintenance"}); !errors.Is(err, ErrRuntimeSupervisorTargetNotFound) {
		t.Fatalf("expected target not found, got %v", err)
	}
	ambiguous := NewRuntimeSupervisorWithOptions(
		[]RuntimeSupervisorTarget{
			{Name: "api", BaseURL: "http://127.0.0.1:8080"},
			{Name: "api", BaseURL: "http://127.0.0.1:8081"},
		},
		nil,
		RuntimeSupervisorOptions{},
	)
	if _, err := ambiguous.SuppressContainerFallback("api", RuntimeSupervisorContainerFallbackControlOptions{Confirm: true, Reason: "maintenance"}); !errors.Is(err, ErrRuntimeSupervisorTargetAmbiguous) {
		t.Fatalf("expected target ambiguous, got %v", err)
	}
}

func TestNoopContainerFallbackExecutorDoesNotExecuteRestart(t *testing.T) {
	executor := NewNoopContainerFallbackExecutor(true)
	if !executor.Configured() {
		t.Fatal("expected configured noop executor")
	}
	descriptor := executor.Descriptor()
	if descriptor.Kind != runtimeSupervisorContainerExecutorKindNoop || !descriptor.DryRun {
		t.Fatalf("expected noop executor descriptor to report dry-run noop, got %+v", descriptor)
	}
	result, err := executor.Restart(context.Background(), RuntimeSupervisorTarget{Name: "api"}, "test")
	if err != nil {
		t.Fatalf("noop restart failed: %v", err)
	}
	if result.Executed || result.Message == "" {
		t.Fatalf("expected noop executor to report non-execution, got %+v", result)
	}
}

func TestRuntimeSupervisorContainerFallbackDecisionContract(t *testing.T) {
	base := runtimeSupervisorContainerFallbackDecisionInput{
		Candidate:          true,
		Enabled:            true,
		ExecutorConfigured: true,
		ExecutorDryRun:     true,
		ExecutorArmed:      true,
		TargetAllowed:      true,
		SafetyGateOK:       true,
	}
	tests := []struct {
		name            string
		input           runtimeSupervisorContainerFallbackDecisionInput
		wantDecision    string
		wantExecutable  bool
		wantBlocked     string
		wantEligible    string
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
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, ExecutorConfigured: true, ExecutorDryRun: true, ExecutorArmed: true, TargetAllowed: true, SafetyGateOK: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-restart-disabled",
		},
		{
			name:         "executor missing",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, Enabled: true, ExecutorDryRun: true, ExecutorArmed: true, TargetAllowed: true, SafetyGateOK: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-executor-not-configured",
		},
		{
			name:         "non dry-run executor not armed",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, Enabled: true, ExecutorConfigured: true, TargetAllowed: true, SafetyGateOK: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-executor-not-armed",
		},
		{
			name:         "non dry-run executor target not allowlisted",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, Enabled: true, ExecutorConfigured: true, ExecutorArmed: true, SafetyGateOK: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-executor-target-not-allowlisted",
		},
		{
			name: "suppressed",
			input: runtimeSupervisorContainerFallbackDecisionInput{
				Candidate:          true,
				Enabled:            true,
				ExecutorConfigured: true,
				ExecutorDryRun:     true,
				ExecutorArmed:      true,
				TargetAllowed:      true,
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
				ExecutorDryRun:     true,
				ExecutorArmed:      true,
				TargetAllowed:      true,
				BackoffActive:      true,
				SafetyGateOK:       true,
			},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-backoff-active",
		},
		{
			name: "duplicate submitted",
			input: runtimeSupervisorContainerFallbackDecisionInput{
				Candidate:          true,
				Enabled:            true,
				ExecutorConfigured: true,
				ExecutorDryRun:     true,
				ExecutorArmed:      true,
				TargetAllowed:      true,
				Duplicate:          true,
				SafetyGateOK:       true,
			},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-already-submitted",
		},
		{
			name:         "safety gate blocked",
			input:        runtimeSupervisorContainerFallbackDecisionInput{Candidate: true, Enabled: true, ExecutorConfigured: true, ExecutorDryRun: true, ExecutorArmed: true, TargetAllowed: true},
			wantDecision: runtimeSupervisorContainerFallbackDecisionBlocked,
			wantBlocked:  "container-fallback-safety-gate-blocked",
		},
		{
			name: "non dry-run eligible",
			input: runtimeSupervisorContainerFallbackDecisionInput{
				Candidate:          true,
				Enabled:            true,
				ExecutorConfigured: true,
				ExecutorArmed:      true,
				TargetAllowed:      true,
				SafetyGateOK:       true,
			},
			wantDecision:    runtimeSupervisorContainerFallbackDecisionEligible,
			wantExecutable:  true,
			wantBlocked:     "",
			wantEligible:    "container-fallback-eligible",
			wantEligibleSet: true,
		},
		{
			name:            "eligible",
			input:           base,
			wantDecision:    runtimeSupervisorContainerFallbackDecisionEligible,
			wantExecutable:  true,
			wantBlocked:     "",
			wantEligible:    "container-fallback-eligible",
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
			if tt.wantEligible != "" && got.EligibleReason != tt.wantEligible {
				t.Fatalf("unexpected eligible reason: got %+v want %s", got, tt.wantEligible)
			}
			if !tt.wantEligibleSet && got.EligibleReason != "" {
				t.Fatalf("did not expect eligible reason, got %+v", got)
			}
		})
	}
}

func TestRuntimeSupervisorContainerFallbackBackoffActive(t *testing.T) {
	now := time.Date(2026, 4, 29, 8, 55, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	if !runtimeSupervisorContainerFallbackBackoffActive(&future, now) {
		t.Fatal("expected future backoff to be active")
	}
	past := now.Add(-time.Minute)
	if runtimeSupervisorContainerFallbackBackoffActive(&past, now) {
		t.Fatal("expected past backoff to be inactive")
	}
	if runtimeSupervisorContainerFallbackBackoffActive(nil, now) {
		t.Fatal("expected missing backoff to be inactive")
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
	if snapshot.Targets[0].Status == nil || len(snapshot.Targets[0].Status.Runtimes) != 1 {
		t.Fatalf("expected supervisor status with one runtime, got %#v", snapshot.Targets[0].Status)
	}
	plan := snapshot.Targets[0].Status.Runtimes[0].ApplicationRestartPlan
	if plan == nil {
		t.Fatalf("expected application restart plan for due signal runtime, got %#v", snapshot.Targets[0].Status.Runtimes[0])
	}
	if plan.Decision != runtimeSupervisorApplicationRestartDecisionEligible || plan.EligibleReason != "runtime-restart-eligible" {
		t.Fatalf("expected eligible application restart plan, got %+v", plan)
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
	if second.Targets[0].Status == nil || len(second.Targets[0].Status.Runtimes) != 1 || second.Targets[0].Status.Runtimes[0].ApplicationRestartPlan == nil {
		t.Fatalf("expected duplicate application restart plan in second snapshot, got %#v", second.Targets[0].Status)
	}
	secondPlan := second.Targets[0].Status.Runtimes[0].ApplicationRestartPlan
	if !secondPlan.Duplicate || secondPlan.BlockedReason != "runtime-restart-duplicate" {
		t.Fatalf("expected duplicate application restart plan to be blocked, got %+v", secondPlan)
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
	if snapshot.Targets[0].Status == nil || len(snapshot.Targets[0].Status.Runtimes) != 1 || snapshot.Targets[0].Status.Runtimes[0].ApplicationRestartPlan == nil {
		t.Fatalf("expected blocked restart plan when healthz fails, got %#v", snapshot.Targets[0].Status)
	}
	plan := snapshot.Targets[0].Status.Runtimes[0].ApplicationRestartPlan
	if plan.Decision != runtimeSupervisorApplicationRestartDecisionBlocked || plan.BlockedReason != "runtime-restart-healthz-unhealthy" || plan.HealthzOK {
		t.Fatalf("expected healthz blocker in restart plan, got %+v", plan)
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
	if snapshot.Targets[0].Status == nil {
		t.Fatalf("expected runtime status with blocked restart plans")
	}
	plans := make(map[string]*RuntimeSupervisorApplicationRestartPlan)
	for _, runtime := range snapshot.Targets[0].Status.Runtimes {
		if runtime.ApplicationRestartPlan != nil {
			plans[runtime.RuntimeID] = runtime.ApplicationRestartPlan
		}
	}
	expectedBlocked := map[string]string{
		"suppressed-signal-runtime": "runtime-restart-suppressed",
		"future-signal-runtime":     "runtime-restart-not-due",
		"live-runtime":              "runtime-restart-unsupported-kind",
	}
	for runtimeID, want := range expectedBlocked {
		plan := plans[runtimeID]
		if plan == nil {
			t.Fatalf("expected blocked restart plan for %s, got %#v", runtimeID, plans)
		}
		if plan.Decision != runtimeSupervisorApplicationRestartDecisionBlocked || plan.BlockedReason != want {
			t.Fatalf("expected %s blocker for %s, got %+v", want, runtimeID, plan)
		}
	}
	if _, ok := plans[""]; ok {
		t.Fatalf("expected runtime without runtimeId to have no restart plan, got %#v", plans)
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

func newRuntimeSupervisorTestCommandExecutor(t *testing.T, targetName, mode string) CommandContainerFallbackExecutor {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	executor, err := NewCommandContainerFallbackExecutor(map[string]CommandContainerFallbackSpec{
		targetName: {
			Path: executable,
			Args: []string{
				"-test.run=TestRuntimeSupervisorCommandContainerFallbackExecutorHelperProcess",
				"--",
				mode,
			},
			Timeout: 5 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("build test command executor: %v", err)
	}
	return executor
}

func TestRuntimeSupervisorCommandContainerFallbackExecutorHelperProcess(t *testing.T) {
	for _, arg := range os.Args {
		switch arg {
		case "--runtime-supervisor-command-executor-success":
			fmt.Fprint(os.Stdout, "helper restart ok")
			os.Exit(0)
		case "--runtime-supervisor-command-executor-fail":
			fmt.Fprint(os.Stderr, "helper restart failed")
			os.Exit(7)
		}
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
