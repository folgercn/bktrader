package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestAccountEquitySnapshotRouteAppliesTimeWindowAndLatestLimit(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)

	first, err := store.CreateAccountEquitySnapshot(domain.AccountEquitySnapshot{
		AccountID: "live-main",
		NetEquity: 101,
	})
	if err != nil {
		t.Fatalf("create first snapshot: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := store.CreateAccountEquitySnapshot(domain.AccountEquitySnapshot{
		AccountID: "live-main",
		NetEquity: 202,
	})
	if err != nil {
		t.Fatalf("create second snapshot: %v", err)
	}
	time.Sleep(time.Millisecond)
	third, err := store.CreateAccountEquitySnapshot(domain.AccountEquitySnapshot{
		AccountID: "live-main",
		NetEquity: 303,
	})
	if err != nil {
		t.Fatalf("create third snapshot: %v", err)
	}

	mux := http.NewServeMux()
	registerAccountRoutes(mux, platform)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/account-equity-snapshots?accountId=live-main&from="+
			second.CreatedAt.Format(time.RFC3339Nano)+
			"&to="+third.CreatedAt.Format(time.RFC3339Nano)+
			"&limit=2",
		nil,
	)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var items []domain.AccountEquitySnapshot
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != second.ID || items[1].ID != third.ID {
		t.Fatalf("expected latest two snapshots in ascending order, got %#v", items)
	}
	if items[0].CreatedAt.Before(second.CreatedAt) || items[1].CreatedAt.After(third.CreatedAt) {
		t.Fatalf("expected items within requested time window, got %#v", items)
	}
	if items[0].ID == first.ID {
		t.Fatalf("did not expect earliest snapshot in filtered response")
	}
}
