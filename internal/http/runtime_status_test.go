package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestRuntimeStatusRouteReturnsUnifiedRuntimeSnapshot(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	runtime, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create signal runtime failed: %v", err)
	}
	nextRestartAt := time.Date(2026, 4, 28, 12, 33, 0, 0, time.UTC).Format(time.RFC3339)
	runtime.Status = "ERROR"
	runtime.State = map[string]any{
		"desiredStatus":             "RUNNING",
		"actualStatus":              "ERROR",
		"health":                    "recovering",
		"supervisorRestartAttempt":  float64(2),
		"nextAutoRestartAt":         nextRestartAt,
		"supervisorRestartBackoff":  "3m0s",
		"supervisorRestartReason":   "runtime-error",
		"supervisorRestartSeverity": "transient",
		"lastSupervisorError":       "websocket timeout",
		"autoRestartSuppressed":     false,
	}
	if _, err := store.UpdateSignalRuntimeSession(runtime); err != nil {
		t.Fatalf("update signal runtime failed: %v", err)
	}
	liveSession, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	liveState := map[string]any{}
	for key, value := range liveSession.State {
		liveState[key] = value
	}
	liveState["desiredStatus"] = "RUNNING"
	liveState["actualStatus"] = "RUNNING"
	liveState["health"] = "healthy"
	if _, err := store.UpdateLiveSessionState(liveSession.ID, liveState); err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	mux := http.NewServeMux()
	registerRuntimeStatusRoutes(mux, platform, config.Config{ProcessRole: "platform-api"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Service  string `json:"service"`
		Runtimes []struct {
			RuntimeID             string `json:"runtimeId"`
			RuntimeKind           string `json:"runtimeKind"`
			AccountID             string `json:"accountId"`
			StrategyID            string `json:"strategyId"`
			DesiredStatus         string `json:"desiredStatus"`
			ActualStatus          string `json:"actualStatus"`
			Health                string `json:"health"`
			RestartAttempt        int    `json:"restartAttempt"`
			NextRestartAt         string `json:"nextRestartAt"`
			RestartBackoff        string `json:"restartBackoff"`
			RestartReason         string `json:"restartReason"`
			RestartSeverity       string `json:"restartSeverity"`
			LastRestartError      string `json:"lastRestartError"`
			AutoRestartSuppressed bool   `json:"autoRestartSuppressed"`
			LastCheckedAt         string `json:"lastCheckedAt"`
		} `json:"runtimes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode runtime status failed: %v", err)
	}
	if payload.Service != "platform-api" {
		t.Fatalf("expected service platform-api, got %s", payload.Service)
	}
	var signalRuntime, liveRuntime *struct {
		RuntimeID             string `json:"runtimeId"`
		RuntimeKind           string `json:"runtimeKind"`
		AccountID             string `json:"accountId"`
		StrategyID            string `json:"strategyId"`
		DesiredStatus         string `json:"desiredStatus"`
		ActualStatus          string `json:"actualStatus"`
		Health                string `json:"health"`
		RestartAttempt        int    `json:"restartAttempt"`
		NextRestartAt         string `json:"nextRestartAt"`
		RestartBackoff        string `json:"restartBackoff"`
		RestartReason         string `json:"restartReason"`
		RestartSeverity       string `json:"restartSeverity"`
		LastRestartError      string `json:"lastRestartError"`
		AutoRestartSuppressed bool   `json:"autoRestartSuppressed"`
		LastCheckedAt         string `json:"lastCheckedAt"`
	}
	for i := range payload.Runtimes {
		switch payload.Runtimes[i].RuntimeKind {
		case "signal":
			if payload.Runtimes[i].RuntimeID == runtime.ID {
				signalRuntime = &payload.Runtimes[i]
			}
		case "live-session":
			if payload.Runtimes[i].RuntimeID == liveSession.ID {
				liveRuntime = &payload.Runtimes[i]
			}
		}
	}
	if signalRuntime == nil {
		t.Fatalf("expected signal runtime status in %#v", payload.Runtimes)
	}
	if signalRuntime.DesiredStatus != "RUNNING" || signalRuntime.ActualStatus != "ERROR" {
		t.Fatalf("expected signal desired RUNNING actual ERROR, got desired=%s actual=%s", signalRuntime.DesiredStatus, signalRuntime.ActualStatus)
	}
	if signalRuntime.Health != "recovering" {
		t.Fatalf("expected signal health recovering, got %s", signalRuntime.Health)
	}
	if signalRuntime.RestartAttempt != 2 {
		t.Fatalf("expected restart attempt 2, got %d", signalRuntime.RestartAttempt)
	}
	if signalRuntime.NextRestartAt != nextRestartAt || signalRuntime.RestartBackoff != "3m0s" {
		t.Fatalf("expected restart schedule %s/3m0s, got %s/%s", nextRestartAt, signalRuntime.NextRestartAt, signalRuntime.RestartBackoff)
	}
	if signalRuntime.RestartReason != "runtime-error" || signalRuntime.RestartSeverity != "transient" {
		t.Fatalf("expected restart reason/severity, got %s/%s", signalRuntime.RestartReason, signalRuntime.RestartSeverity)
	}
	if signalRuntime.LastRestartError != "websocket timeout" {
		t.Fatalf("expected last restart error, got %s", signalRuntime.LastRestartError)
	}
	if signalRuntime.LastCheckedAt == "" {
		t.Fatal("expected signal runtime lastCheckedAt")
	}
	if liveRuntime == nil {
		t.Fatalf("expected live session runtime status in %#v", payload.Runtimes)
	}
	if liveRuntime.DesiredStatus != "RUNNING" || liveRuntime.ActualStatus != "RUNNING" || liveRuntime.Health != "healthy" {
		t.Fatalf("expected live desired/actual/health RUNNING/RUNNING/healthy, got %s/%s/%s", liveRuntime.DesiredStatus, liveRuntime.ActualStatus, liveRuntime.Health)
	}
}

func TestRuntimeStatusRouteRejectsNonGet(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeStatusRoutes(mux, service.NewPlatform(memory.NewStore()), config.Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHealthzIncludesServiceLevelStatusFields(t *testing.T) {
	router := NewRouter(config.Config{
		AppName:     "bktrader",
		Environment: "test",
		ProcessRole: "platform-api",
	}, service.NewPlatform(memory.NewStore()))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Status    string `json:"status"`
		Service   string `json:"service"`
		CheckedAt string `json:"checkedAt"`
		App       string `json:"app"`
		Env       string `json:"env"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode healthz failed: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %s", payload.Status)
	}
	if payload.Service != "platform-api" {
		t.Fatalf("expected service platform-api, got %s", payload.Service)
	}
	if payload.CheckedAt == "" {
		t.Fatal("expected checkedAt")
	}
	if payload.App != "bktrader" || payload.Env != "test" {
		t.Fatalf("expected app/env bktrader/test, got %s/%s", payload.App, payload.Env)
	}
}
