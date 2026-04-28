package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestRuntimeRestartRouteAcceptsSignalRuntimeRestart(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}

	mux := http.NewServeMux()
	registerRuntimeControlRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/restart", strings.NewReader(`{"runtimeId":"`+runtimeSession.ID+`","runtimeKind":"signal","force":true,"confirm":true,"reason":"operator requested rebinding"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Status        string `json:"status"`
		RuntimeID     string `json:"runtimeId"`
		RuntimeKind   string `json:"runtimeKind"`
		DesiredStatus string `json:"desiredStatus"`
		ActualStatus  string `json:"actualStatus"`
		Force         bool   `json:"force"`
		Reason        string `json:"reason"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Status != "accepted" || payload.RuntimeID != runtimeSession.ID || payload.RuntimeKind != "signal" {
		t.Fatalf("unexpected restart response: %+v", payload)
	}
	if payload.DesiredStatus != "RUNNING" {
		t.Fatalf("expected desiredStatus RUNNING, got %s", payload.DesiredStatus)
	}
	if payload.ActualStatus != "STARTING" && payload.ActualStatus != "RUNNING" {
		t.Fatalf("expected actualStatus STARTING/RUNNING, got %s", payload.ActualStatus)
	}
	if !payload.Force || payload.Reason != "operator requested rebinding" {
		t.Fatalf("expected force/reason echoed, got %+v", payload)
	}
	stored, err := platform.GetSignalRuntimeSession(runtimeSession.ID)
	if err != nil {
		t.Fatalf("GetSignalRuntimeSession failed: %v", err)
	}
	if got := stored.State["restartRequestedForce"]; got != true {
		t.Fatalf("expected restartRequestedForce true, got %#v", got)
	}
	if got := stored.State["restartRequestedReason"]; got != "operator requested rebinding" {
		t.Fatalf("expected restartRequestedReason, got %#v", got)
	}
	if got := stored.State["restartRequestedSource"]; got != "api" {
		t.Fatalf("expected restartRequestedSource api, got %#v", got)
	}
	if got := stored.State["restartRequestedAt"]; got == "" {
		t.Fatalf("expected restartRequestedAt, got %#v", got)
	}
	if _, err := platform.StopSignalRuntimeSessionWithForce(runtimeSession.ID, true); err != nil {
		t.Fatalf("cleanup runtime failed: %v", err)
	}
}

func TestRuntimeRestartRouteRequiresConfirm(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}

	mux := http.NewServeMux()
	registerRuntimeControlRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/restart", strings.NewReader(`{"runtimeId":"`+runtimeSession.ID+`","runtimeKind":"signal"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuntimeRestartRouteRequiresReasonForForce(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	runtimeSession, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("CreateSignalRuntimeSession failed: %v", err)
	}

	mux := http.NewServeMux()
	registerRuntimeControlRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/restart", strings.NewReader(`{"runtimeId":"`+runtimeSession.ID+`","runtimeKind":"signal","confirm":true,"force":true}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuntimeRestartRouteRejectsDisabledRuntimeActions(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeControlRoutes(mux, service.NewPlatform(memory.NewStore()), config.Config{ProcessRole: "supervisor"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/restart", strings.NewReader(`{"runtimeId":"runtime-1","runtimeKind":"signal","confirm":true}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuntimeRestartRouteRejectsUnsupportedRuntimeKind(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeControlRoutes(mux, service.NewPlatform(memory.NewStore()), config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/restart", strings.NewReader(`{"runtimeId":"runtime-1","runtimeKind":"live-session","confirm":true}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
