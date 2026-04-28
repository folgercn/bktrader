package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	registerSupervisorStatusRoutes(mux, platform)

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
}

func TestSupervisorStatusRouteReturnsNotFoundWhenUnconfigured(t *testing.T) {
	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, service.NewPlatform(memory.NewStore()))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/supervisor/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSupervisorStatusRouteRejectsNonGet(t *testing.T) {
	mux := http.NewServeMux()
	registerSupervisorStatusRoutes(mux, service.NewPlatform(memory.NewStore()))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supervisor/status", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
