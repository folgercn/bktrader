package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestAccountEquitySnapshotRouteAppliesDefaultLimit(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	for i := 0; i < defaultAccountEquitySnapshotLimit+5; i++ {
		if _, err := store.CreateAccountEquitySnapshot(domain.AccountEquitySnapshot{
			AccountID: "live-main",
			NetEquity: float64(i),
		}); err != nil {
			t.Fatalf("create equity snapshot %d: %v", i, err)
		}
	}

	mux := http.NewServeMux()
	registerAccountRoutes(mux, platform)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account-equity-snapshots?accountId=live-main", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var items []domain.AccountEquitySnapshot
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != defaultAccountEquitySnapshotLimit {
		t.Fatalf("expected default limit %d, got %d", defaultAccountEquitySnapshotLimit, len(items))
	}
}

func TestAccountEquitySnapshotRouteRejectsOversizedLimit(t *testing.T) {
	mux := http.NewServeMux()
	registerAccountRoutes(mux, service.NewPlatform(memory.NewStore()))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account-equity-snapshots?accountId=live-main&limit=5001", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized limit, got %d body=%s", rec.Code, rec.Body.String())
	}
}
