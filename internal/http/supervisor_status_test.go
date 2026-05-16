package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestSupervisorStatusRouteReturnsLastSnapshot(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			writeJSON(w, http.StatusOK, map[string]any{
				"service": "platform-api",
				"status":  "ok",
			})
		case "/api/v1/runtime/status":
			writeJSON(w, http.StatusOK, service.RuntimeStatusSnapshot{
				Service: "platform-api",
				Runtimes: []service.RuntimeStatus{{
					RuntimeID:     "runtime-1",
					RuntimeKind:   "signal",
					DesiredStatus: "RUNNING",
					ActualStatus:  "RUNNING",
					Health:        "healthy",
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer targetServer.Close()

	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisor([]service.RuntimeSupervisorTarget{{
		Name:    "api",
		BaseURL: targetServer.URL,
	}}, targetServer.Client())
	supervisor.Collect(context.Background())
	platform.SetRuntimeSupervisor(supervisor)

	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, platform, config.Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/supervisor/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload service.RuntimeSupervisorSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode supervisor status failed: %v", err)
	}
	if len(payload.Targets) != 1 {
		t.Fatalf("expected one target, got %#v", payload.Targets)
	}
	target := payload.Targets[0]
	if target.Name != "api" {
		t.Fatalf("expected target name api, got %s", target.Name)
	}
	if !target.Healthz.Reachable || target.Healthz.StatusCode != http.StatusOK {
		t.Fatalf("expected reachable healthz 200, got %+v", target.Healthz)
	}
	if target.Status == nil || len(target.Status.Runtimes) != 1 || target.Status.Runtimes[0].RuntimeID != "runtime-1" {
		t.Fatalf("expected runtime-1 status, got %+v", target.Status)
	}
	if !payload.Policy.DashboardPermissions.CanView || payload.Policy.DashboardPermissions.CanContainerFallbackSubmit {
		t.Fatalf("expected default permissions to allow view and deny fallback submit, got %+v", payload.Policy.DashboardPermissions)
	}
}

func TestSupervisorStatusRouteReturnsNotFoundWhenUnconfigured(t *testing.T) {
	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, service.NewPlatform(memory.NewStore()), config.Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/supervisor/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSupervisorStatusRouteRejectsNonGet(t *testing.T) {
	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, service.NewPlatform(memory.NewStore()), config.Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestSupervisorContainerFallbackControlSuppressesAndResumesTarget(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeJSON(w, http.StatusOK, service.RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer targetServer.Close()

	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisorWithOptions(
		[]service.RuntimeSupervisorTarget{{Name: "api", BaseURL: targetServer.URL}},
		targetServer.Client(),
		service.RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   true,
			ContainerFallbackExecutor: service.NewNoopContainerFallbackExecutor(true),
		},
	)
	supervisor.Collect(context.Background())
	platform.SetRuntimeSupervisor(supervisor)
	allowSubmit := true

	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, platform, config.Config{SupervisorDashboardFallbackSubmit: &allowSubmit})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/suppress", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"maintenance window"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected suppress 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var suppressPayload struct {
		Status       string                                `json:"status"`
		TargetName   string                                `json:"targetName"`
		Suppressed   bool                                  `json:"suppressed"`
		Reason       string                                `json:"reason"`
		ServiceState service.RuntimeSupervisorServiceState `json:"serviceState"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&suppressPayload); err != nil {
		t.Fatalf("decode suppress payload failed: %v", err)
	}
	if suppressPayload.Status != "accepted" || suppressPayload.TargetName != "api" || !suppressPayload.Suppressed || suppressPayload.Reason != "maintenance window" {
		t.Fatalf("unexpected suppress payload: %+v", suppressPayload)
	}
	if !suppressPayload.ServiceState.ContainerFallbackSuppressed || suppressPayload.ServiceState.ContainerFallbackSuppressedReason != "maintenance window" || suppressPayload.ServiceState.ContainerFallbackSuppressedSource != "api" {
		t.Fatalf("expected suppress audit state, got %+v", suppressPayload.ServiceState)
	}

	blocked := supervisor.Collect(context.Background()).Targets[0]
	if blocked.ContainerFallbackPlan == nil || blocked.ContainerFallbackPlan.BlockedReason != "container-fallback-suppressed" {
		t.Fatalf("expected suppressed plan to block fallback, got %+v", blocked.ContainerFallbackPlan)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/resume", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"maintenance finished"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected resume 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resumePayload struct {
		Suppressed   bool                                  `json:"suppressed"`
		Reason       string                                `json:"reason"`
		ServiceState service.RuntimeSupervisorServiceState `json:"serviceState"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resumePayload); err != nil {
		t.Fatalf("decode resume payload failed: %v", err)
	}
	if resumePayload.Suppressed || resumePayload.Reason != "maintenance finished" || resumePayload.ServiceState.ContainerFallbackSuppressed {
		t.Fatalf("unexpected resume payload: %+v", resumePayload)
	}
	if resumePayload.ServiceState.ContainerFallbackResumedReason != "maintenance finished" || resumePayload.ServiceState.ContainerFallbackResumedSource != "api" {
		t.Fatalf("expected resume audit state, got %+v", resumePayload.ServiceState)
	}
}

func TestSupervisorContainerFallbackControlDefersAndClearsBackoff(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeJSON(w, http.StatusOK, service.RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer targetServer.Close()

	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisorWithOptions(
		[]service.RuntimeSupervisorTarget{{Name: "api", BaseURL: targetServer.URL}},
		targetServer.Client(),
		service.RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   true,
			ContainerFallbackExecutor: service.NewNoopContainerFallbackExecutor(true),
		},
	)
	supervisor.Collect(context.Background())
	platform.SetRuntimeSupervisor(supervisor)
	allowSubmit := true

	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, platform, config.Config{SupervisorDashboardFallbackSubmit: &allowSubmit})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/defer", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"cooldown","backoffSeconds":300}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected defer 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var deferPayload struct {
		BackoffUntil string                                `json:"backoffUntil"`
		Reason       string                                `json:"reason"`
		ServiceState service.RuntimeSupervisorServiceState `json:"serviceState"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&deferPayload); err != nil {
		t.Fatalf("decode defer payload failed: %v", err)
	}
	if deferPayload.BackoffUntil == "" || deferPayload.Reason != "cooldown" || deferPayload.ServiceState.ContainerFallbackBackoffUntil == nil {
		t.Fatalf("unexpected defer payload: %+v", deferPayload)
	}
	if deferPayload.ServiceState.ContainerFallbackBackoffReason != "cooldown" || deferPayload.ServiceState.ContainerFallbackBackoffSource != "api" {
		t.Fatalf("expected backoff audit state, got %+v", deferPayload.ServiceState)
	}

	blocked := supervisor.Collect(context.Background()).Targets[0]
	if blocked.ContainerFallbackPlan == nil || blocked.ContainerFallbackPlan.BlockedReason != "container-fallback-backoff-active" {
		t.Fatalf("expected backoff plan to block fallback, got %+v", blocked.ContainerFallbackPlan)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/clear-backoff", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"stable"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected clear-backoff 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var clearPayload struct {
		BackoffUntil *string                               `json:"backoffUntil"`
		Reason       string                                `json:"reason"`
		ServiceState service.RuntimeSupervisorServiceState `json:"serviceState"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&clearPayload); err != nil {
		t.Fatalf("decode clear payload failed: %v", err)
	}
	if clearPayload.BackoffUntil != nil || clearPayload.Reason != "stable" || clearPayload.ServiceState.ContainerFallbackBackoffUntil != nil {
		t.Fatalf("unexpected clear payload: %+v", clearPayload)
	}
	if clearPayload.ServiceState.ContainerFallbackBackoffClearedReason != "stable" || clearPayload.ServiceState.ContainerFallbackBackoffClearedSource != "api" {
		t.Fatalf("expected backoff clear audit state, got %+v", clearPayload.ServiceState)
	}
}

func TestSupervisorContainerFallbackSubmitEndpointSubmitsEligiblePlan(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		case "/api/v1/runtime/status":
			writeJSON(w, http.StatusOK, service.RuntimeStatusSnapshot{Service: "platform-api"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer targetServer.Close()

	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisorWithOptions(
		[]service.RuntimeSupervisorTarget{{Name: "api", BaseURL: targetServer.URL}},
		targetServer.Client(),
		service.RuntimeSupervisorOptions{
			ServiceFailureThreshold:   1,
			EnableContainerFallback:   true,
			ContainerFallbackExecutor: service.NewNoopContainerFallbackExecutor(true),
		},
	)
	if _, err := supervisor.SuppressContainerFallback("api", service.RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "hold before operator review",
		Source:  "test",
	}); err != nil {
		t.Fatalf("suppress before submit setup failed: %v", err)
	}
	supervisor.Collect(context.Background())
	if _, err := supervisor.ResumeContainerFallback("api", service.RuntimeSupervisorContainerFallbackControlOptions{
		Confirm: true,
		Reason:  "operator reviewed preview",
		Source:  "test",
	}); err != nil {
		t.Fatalf("resume before submit setup failed: %v", err)
	}
	platform.SetRuntimeSupervisor(supervisor)
	allowSubmit := true

	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, platform, config.Config{SupervisorDashboardFallbackSubmit: &allowSubmit})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/submit", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"operator reviewed static command preview"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected submit 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var submitPayload struct {
		Status       string                                            `json:"status"`
		TargetName   string                                            `json:"targetName"`
		Reason       string                                            `json:"reason"`
		Source       string                                            `json:"source"`
		ServiceState service.RuntimeSupervisorServiceState             `json:"serviceState"`
		Plan         *service.RuntimeSupervisorContainerFallbackPlan   `json:"containerFallbackPlan"`
		Action       *service.RuntimeSupervisorContainerFallbackAction `json:"action"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&submitPayload); err != nil {
		t.Fatalf("decode submit payload failed: %v", err)
	}
	if submitPayload.Status != "accepted" || submitPayload.TargetName != "api" || submitPayload.Reason != "operator reviewed static command preview" || submitPayload.Source != "api" {
		t.Fatalf("unexpected submit payload: %+v", submitPayload)
	}
	if submitPayload.Action == nil || !submitPayload.Action.Submitted || submitPayload.Action.Source != "api" || submitPayload.Action.PlanReason == "" {
		t.Fatalf("expected action audit with source and plan reason, got %+v", submitPayload.Action)
	}
	if submitPayload.ServiceState.ContainerFallbackSubmittedReason != "operator reviewed static command preview" || !submitPayload.ServiceState.ContainerFallbackSubmitted {
		t.Fatalf("expected submitted service state, got %+v", submitPayload.ServiceState)
	}
	if submitPayload.Plan == nil || submitPayload.Plan.BlockedReason != "container-fallback-already-submitted" {
		t.Fatalf("expected duplicate blocker after submit, got %+v", submitPayload.Plan)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/clear-backoff", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"operator approved retry"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected clear retry gate 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var clearPayload struct {
		Reason       string                                `json:"reason"`
		ServiceState service.RuntimeSupervisorServiceState `json:"serviceState"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&clearPayload); err != nil {
		t.Fatalf("decode clear retry gate payload failed: %v", err)
	}
	if clearPayload.Reason != "operator approved retry" || clearPayload.ServiceState.ContainerFallbackSubmitted || clearPayload.ServiceState.ContainerFallbackSubmittedAt != nil {
		t.Fatalf("expected clear-backoff to clear submitted retry gate, got %+v", clearPayload)
	}
	clearedSnapshot := supervisor.LastSnapshot()
	if len(clearedSnapshot.Targets) != 1 || clearedSnapshot.Targets[0].ContainerFallbackPlan == nil || !clearedSnapshot.Targets[0].ContainerFallbackPlan.Executable || clearedSnapshot.Targets[0].ContainerFallbackPlan.Duplicate {
		t.Fatalf("expected clear-backoff API to make plan executable again, got %+v", clearedSnapshot.Targets)
	}
}

func TestSupervisorContainerFallbackControlValidation(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisor(
		[]service.RuntimeSupervisorTarget{{Name: "api", BaseURL: "http://127.0.0.1:8080"}},
		nil,
	)
	platform.SetRuntimeSupervisor(supervisor)
	allowSubmit := true

	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, platform, config.Config{SupervisorDashboardFallbackSubmit: &allowSubmit})

	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing target",
			path:       "/api/v1/supervisor/container-fallback/suppress",
			body:       `{"confirm":true,"reason":"maintenance"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing confirm",
			path:       "/api/v1/supervisor/container-fallback/suppress",
			body:       `{"targetName":"api","reason":"maintenance"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing reason",
			path:       "/api/v1/supervisor/container-fallback/suppress",
			body:       `{"targetName":"api","confirm":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing backoff seconds",
			path:       "/api/v1/supervisor/container-fallback/defer",
			body:       `{"targetName":"api","confirm":true,"reason":"cooldown"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "backoff seconds too large",
			path:       "/api/v1/supervisor/container-fallback/defer",
			body:       `{"targetName":"api","confirm":true,"reason":"cooldown","backoffSeconds":86401}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing configured target",
			path:       "/api/v1/supervisor/container-fallback/suppress",
			body:       `{"targetName":"missing","confirm":true,"reason":"maintenance"}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "submit missing confirm",
			path:       "/api/v1/supervisor/container-fallback/submit",
			body:       `{"targetName":"api","reason":"operator reviewed"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "submit missing reason",
			path:       "/api/v1/supervisor/container-fallback/submit",
			body:       `{"targetName":"api","confirm":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "submit blocked without candidate",
			path:       "/api/v1/supervisor/container-fallback/submit",
			body:       `{"targetName":"api","confirm":true,"reason":"operator reviewed"}`,
			wantStatus: http.StatusConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSupervisorContainerFallbackControlRejectsReadOnlyRoles(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisor(
		[]service.RuntimeSupervisorTarget{{Name: "api", BaseURL: "http://127.0.0.1:8080"}},
		nil,
	)
	platform.SetRuntimeSupervisor(supervisor)

	for _, role := range []string{"api", "supervisor"} {
		t.Run(role, func(t *testing.T) {
			mux := http.NewServeMux()
			registerSupervisorStatusRoutes(mux, platform, config.Config{ProcessRole: role})

			for _, path := range []string{
				"/api/v1/supervisor/container-fallback/suppress",
				"/api/v1/supervisor/container-fallback/submit",
			} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"targetName":"api","confirm":true,"reason":"maintenance"}`))
				mux.ServeHTTP(rec, req)
				if rec.Code != http.StatusConflict {
					t.Fatalf("expected read-only role to get 409 for %s, got %d body=%s", path, rec.Code, rec.Body.String())
				}
			}
		})
	}
}

func TestSupervisorDashboardPermissionsBlockViewGateAndSubmit(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	supervisor := service.NewRuntimeSupervisor(
		[]service.RuntimeSupervisorTarget{{Name: "api", BaseURL: "http://127.0.0.1:8080"}},
		nil,
	)
	platform.SetRuntimeSupervisor(supervisor)
	deny := false

	t.Run("view", func(t *testing.T) {
		mux := http.NewServeMux()
		registerSupervisorStatusRoutes(mux, platform, config.Config{SupervisorDashboardView: &deny})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/supervisor/status", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("gate", func(t *testing.T) {
		mux := http.NewServeMux()
		registerSupervisorStatusRoutes(mux, platform, config.Config{
			ProcessRole:                     "monolith",
			SupervisorDashboardFallbackGate: &deny,
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/suppress", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"maintenance"}`))
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("submit", func(t *testing.T) {
		mux := http.NewServeMux()
		registerSupervisorStatusRoutes(mux, platform, config.Config{
			ProcessRole:                       "monolith",
			SupervisorDashboardFallbackSubmit: &deny,
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/container-fallback/submit", strings.NewReader(`{"targetName":"api","confirm":true,"reason":"operator reviewed"}`))
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}
