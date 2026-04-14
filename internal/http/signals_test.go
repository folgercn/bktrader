package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestRuntimePolicyPatchPreservesOmittedFieldsAndAllowsZeroHealthThresholds(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	mux := http.NewServeMux()
	registerSignalRoutes(mux, platform)

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
