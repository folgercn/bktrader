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

func TestNewRouterRegistersCoreRoutesWithoutPanic(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	router := NewRouter(config.Config{AppName: "test", Environment: "test"}, platform)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "healthz", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK},
		{name: "overview", method: http.MethodGet, path: "/api/v1/overview", wantStatus: http.StatusOK},
		{name: "live account positions", method: http.MethodGet, path: "/api/v1/live/accounts/live-main/positions", wantStatus: http.StatusOK},
		{name: "live recovery diagnose missing account", method: http.MethodGet, path: "/api/v1/live/accounts/missing-account/recovery/diagnose?symbol=BTCUSDT", wantStatus: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestLiveRecoveryRoutes(t *testing.T) {
	s := memory.NewStore()
	platform := service.NewPlatform(s)
	mux := http.NewServeMux()
	registerAccountRoutes(mux, platform)

	// 1. 不完整 recovery URL 不 panic
	t.Run("incomplete URL no panic", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/live/accounts/acc1/recovery", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})

	// 2. GET diagnose 正常处理缺失 account，不应冒泡为 500
	t.Run("GET diagnose route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/live/accounts/acc1/recovery/diagnose?symbol=BTCUSDT", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 (account not found), got %d", rr.Code)
		}
	})

	// 3. POST execute 缺 action 返回 400
	t.Run("POST execute missing action", func(t *testing.T) {
		body := `{"payload": {}}`
		req := httptest.NewRequest("POST", "/api/v1/live/accounts/acc1/recovery/execute", strings.NewReader(body))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})
}
