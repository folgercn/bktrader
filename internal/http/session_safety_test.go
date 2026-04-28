package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func boolValue(value any) bool {
	v, ok := value.(bool)
	return ok && v
}

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
	registerLiveRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/stop", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for accepted stop intent, got %d body=%s", rec.Code, rec.Body.String())
	}
	session, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(session.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected desiredStatus STOPPED, got %s", got)
	}
	if boolValue(session.State["desiredStopForce"]) {
		t.Fatalf("did not expect desiredStopForce for normal stop")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/stop?force=true", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for accepted forced stop intent, got %d body=%s", rec.Code, rec.Body.String())
	}
	session, err = store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if !boolValue(session.State["desiredStopForce"]) {
		t.Fatalf("expected desiredStopForce for forced stop")
	}
}

func TestLiveSessionRuntimeActionsDisabledForAPIRole(t *testing.T) {
	cases := []string{
		"/api/v1/live/sessions/live-session-main/dispatch",
		"/api/v1/live/sessions/live-session-main/sync",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			platform := service.NewPlatform(memory.NewStore())
			mux := http.NewServeMux()
			registerLiveRoutes(mux, platform, config.Config{ProcessRole: "api"})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusConflict {
				t.Fatalf("expected 409 for api role runtime action, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLiveSessionStartStopRoutesAcceptedForAPIRole(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	mux := http.NewServeMux()
	registerLiveRoutes(mux, platform, config.Config{ProcessRole: "api"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/start", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for api role start intent, got %d body=%s", rec.Code, rec.Body.String())
	}
	session, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(session.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected desiredStatus RUNNING, got %s", got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/stop?force=true", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for api role stop intent, got %d body=%s", rec.Code, rec.Body.String())
	}
	session, err = store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(session.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected desiredStatus STOPPED, got %s", got)
	}
	if !boolValue(session.State["desiredStopForce"]) {
		t.Fatalf("expected desiredStopForce")
	}
}

func TestLiveSessionDeleteCancelsPendingDesiredControlIntent(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	mux := http.NewServeMux()
	registerLiveRoutes(mux, platform, config.Config{ProcessRole: "api"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/start", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for pending start intent, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/live/sessions/live-session-main?force=true", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete after pending intent, got %d body=%s", rec.Code, rec.Body.String())
	}

	deleted, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if deleted.Status != "DELETED" {
		t.Fatalf("expected DELETED status, got %s", deleted.Status)
	}
	if got := stringValue(deleted.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected delete to cancel desiredStatus as STOPPED, got %s", got)
	}
	if got := stringValue(deleted.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected delete to mark actualStatus STOPPED, got %s", got)
	}

	listed, err := store.ListLiveSessions()
	if err != nil {
		t.Fatalf("list live sessions failed: %v", err)
	}
	for _, item := range listed {
		if item.ID == deleted.ID {
			t.Fatalf("expected deleted session to be hidden from live session list")
		}
	}
}

func TestLiveSessionStartRouteWritesDesiredStateForCorruptedControlKey(t *testing.T) {
	store := memory.NewStore()
	session, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	session.StrategyID = ""
	if _, err := store.UpdateLiveSession(session); err != nil {
		t.Fatalf("corrupt live session strategy id failed: %v", err)
	}
	platform := service.NewPlatform(store)
	mux := http.NewServeMux()
	registerLiveRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/live/sessions/live-session-main/start", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for desired start despite corrupted control key, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected desiredStatus RUNNING, got %s", got)
	}
}

func TestLiveSessionDetailRouteFiltersStateFields(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	if _, err := store.UpdateLiveSessionState("live-session-main", map[string]any{
		"timeline":                             []any{map[string]any{"title": "first"}},
		"breakoutHistory":                      []any{map[string]any{"side": "BUY"}},
		"lastStrategyEvaluationSourceStates":   map[string]any{"heavy": true},
		"lastStrategyEvaluationSignalBarState": "keep-out",
	}); err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	mux := http.NewServeMux()
	registerLiveRoutes(mux, platform, config.Config{ProcessRole: "monolith"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/live/sessions/live-session-main/detail?fields=timeline,breakoutHistory,timeline", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for detail route, got %d body=%s", rec.Code, rec.Body.String())
	}

	var detail domain.LiveSession
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode live session detail failed: %v", err)
	}
	if detail.ID != "live-session-main" {
		t.Fatalf("expected live-session-main, got %s", detail.ID)
	}
	if _, ok := detail.State["timeline"]; !ok {
		t.Fatalf("expected timeline in filtered state, got %#v", detail.State)
	}
	if _, ok := detail.State["breakoutHistory"]; !ok {
		t.Fatalf("expected breakoutHistory in filtered state, got %#v", detail.State)
	}
	if _, ok := detail.State["lastStrategyEvaluationSourceStates"]; ok {
		t.Fatalf("did not expect unrequested source states, got %#v", detail.State)
	}
	if len(detail.State) != 2 {
		t.Fatalf("expected only 2 requested fields, got %#v", detail.State)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/live/sessions/live-session-main/detail", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when fields are missing, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/live/sessions/live-session-main/detail?fields=sourceStates", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported detail field, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLiveAccountStopRouteStopsRunningFlow(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)

	if _, err := platform.BindStrategySignalSource("strategy-bk-1d", map[string]any{
		"sourceKey": "binance-kline",
		"role":      "signal",
		"symbol":    "BTCUSDT",
		"options":   map[string]any{"timeframe": "1d"},
	}); err != nil {
		t.Fatalf("bind strategy signal failed: %v", err)
	}

	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	runtime, err := platform.CreateSignalRuntimeSession("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("create runtime failed: %v", err)
	}
	if _, err := platform.StartSignalRuntimeSession(runtime.ID); err != nil {
		t.Fatalf("start runtime failed: %v", err)
	}
	state := map[string]any{
		"signalRuntimeSessionId": runtime.ID,
	}
	if _, err := store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}

	mux := http.NewServeMux()
	registerAccountRoutes(mux, platform)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/live/accounts/live-main/stop", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for account stop, got %d body=%s", rec.Code, rec.Body.String())
	}

	updatedSession, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("load live session failed: %v", err)
	}
	if updatedSession.Status != "STOPPED" {
		t.Fatalf("expected live session to be stopped, got %s", updatedSession.Status)
	}

	updatedRuntime, err := platform.GetSignalRuntimeSession(runtime.ID)
	if err != nil {
		t.Fatalf("load runtime failed: %v", err)
	}
	if updatedRuntime.Status != "STOPPED" {
		t.Fatalf("expected runtime to be stopped, got %s", updatedRuntime.Status)
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
