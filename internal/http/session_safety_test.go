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

func TestLiveSessionStopRouteRespectsForceQuery(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	if _, err := store.SavePosition(domain.Position{
		AccountID:         "live-main",
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	mux := http.NewServeMux()
	registerLiveRoutes(mux, platform)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/stop", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blocked stop, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/stop?force=true", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for forced stop, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPositionCloseAndOrderDetailRoutes(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	account, err := platform.CreateAccount("Paper HTTP", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create account failed: %v", err)
	}
	position, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.1,
		EntryPrice:        68000,
		MarkPrice:         68100,
	})
	if err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	mux := http.NewServeMux()
	registerAccountRoutes(mux, platform)
	registerOrderRoutes(mux, platform)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/positions/"+position.ID+"/close", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for position close, got %d body=%s", rec.Code, rec.Body.String())
	}
	var closed domain.Order
	if err := json.NewDecoder(rec.Body).Decode(&closed); err != nil {
		t.Fatalf("decode close position response failed: %v", err)
	}
	if closed.ID == "" {
		t.Fatal("expected created close order id")
	}
	if !serviceBool(closed.Metadata["reduceOnly"]) {
		t.Fatal("expected reduceOnly metadata on close order")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+closed.ID, nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for order detail, got %d body=%s", rec.Code, rec.Body.String())
	}
	var detail domain.Order
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode order detail response failed: %v", err)
	}
	if detail.ID != closed.ID {
		t.Fatalf("expected fetched order %s, got %s", closed.ID, detail.ID)
	}
}

func serviceBool(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}
