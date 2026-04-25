package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestRuntimePolicyPatchPreservesOmittedFieldsAndAllowsZeroHealthThresholds(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	mux := http.NewServeMux()
	registerSignalRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	updateRequest := func(body string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-policy", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected runtime policy update to succeed, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	updateRequest(`{"runtimeQuietSeconds":45}`)
	current := platform.RuntimePolicy()
	if current.RuntimeQuietSeconds != 45 {
		t.Fatalf("expected runtime quiet threshold to update to 45, got %d", current.RuntimeQuietSeconds)
	}
	if current.StrategyEvaluationQuietSeconds != 15 {
		t.Fatalf("expected omitted strategy evaluation quiet threshold to remain 15, got %d", current.StrategyEvaluationQuietSeconds)
	}
	if current.LiveAccountSyncFreshnessSecs != 60 {
		t.Fatalf("expected omitted live account sync freshness threshold to remain 60, got %d", current.LiveAccountSyncFreshnessSecs)
	}

	updateRequest(`{"strategyEvaluationQuietSeconds":0,"liveAccountSyncFreshnessSeconds":0}`)
	current = platform.RuntimePolicy()
	if current.RuntimeQuietSeconds != 45 {
		t.Fatalf("expected unrelated runtime quiet threshold to be preserved at 45, got %d", current.RuntimeQuietSeconds)
	}
	if current.StrategyEvaluationQuietSeconds != 0 {
		t.Fatalf("expected strategy evaluation quiet threshold to allow explicit zero, got %d", current.StrategyEvaluationQuietSeconds)
	}
	if current.LiveAccountSyncFreshnessSecs != 0 {
		t.Fatalf("expected live account sync freshness threshold to allow explicit zero, got %d", current.LiveAccountSyncFreshnessSecs)
	}
}

func TestSignalRuntimeActionsDisabledForAPIRole(t *testing.T) {
	cases := []string{
		"/api/v1/signal-runtime/sessions/runtime-1/start",
		"/api/v1/signal-runtime/sessions/runtime-1/stop",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			platform := service.NewPlatform(memory.NewStore())
			mux := http.NewServeMux()
			registerSignalRoutes(mux, platform, config.Config{ProcessRole: "api"})

			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusConflict {
				t.Fatalf("expected 409 for api role runtime action, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
