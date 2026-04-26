package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestLiveRecoveryRoutes(t *testing.T) {
	s := memory.NewStore()
	platform := service.NewPlatform(s)
	mux := http.NewServeMux()
	registerLiveRecoveryRoutes(mux, platform)

	// 1. 不完整 recovery URL 不 panic
	t.Run("incomplete URL no panic", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/live/accounts/acc1/recovery", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})

	// 2. GET diagnose 正常 (虽然因为没数据会报 500/error，但路由应该通)
	t.Run("GET diagnose route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/live/accounts/acc1/recovery/diagnose?symbol=BTCUSDT", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		// 应该因为找不到 account 而报错 500 (目前的逻辑)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 (account not found), got %d", rr.Code)
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
