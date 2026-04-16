package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/logging"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestLogRoutesExposeSystemAndHTTPRequestLogs(t *testing.T) {
	logging.ResetForTests()
	t.Cleanup(logging.ResetForTests)

	platform := service.NewPlatform(memory.NewStore())
	mux := http.NewServeMux()
	registerLogRoutes(mux, platform)

	base := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	logging.RecordSystemLog(logging.SystemLogEntry{
		Level:     "error",
		Message:   "bootstrap failed",
		CreatedAt: base,
		Attributes: map[string]any{
			"component": "app.server",
		},
	})
	logging.RecordSystemLog(logging.SystemLogEntry{
		Level:     "info",
		Message:   "bootstrap recovered",
		CreatedAt: base.Add(time.Minute),
	})
	sensitiveRequestLog := logging.RecordHTTPRequest(logging.HTTPRequestLogEntry{
		Level:        "error",
		Message:      "http request failed",
		Method:       http.MethodGet,
		Path:         "/api/v1/live/sessions",
		Query:        "token=abc123&mode=read",
		RemoteAddr:   "10.1.2.3:443",
		Status:       http.StatusServiceUnavailable,
		DurationMs:   420,
		PanicMessage: "panic: sensitive downstream error",
		Stack:        "top-secret stack trace",
		CreatedAt:    base.Add(2 * time.Minute),
	})
	logging.RecordHTTPRequest(logging.HTTPRequestLogEntry{
		Level:      "info",
		Message:    "http request completed",
		Method:     http.MethodGet,
		Path:       "/healthz",
		Status:     http.StatusOK,
		DurationMs: 12,
		CreatedAt:  base.Add(3 * time.Minute),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/system?level=error&limit=10", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system logs, got %d", rec.Code)
	}
	var systemPage logging.SystemLogPage
	if err := json.NewDecoder(rec.Body).Decode(&systemPage); err != nil {
		t.Fatalf("decode system logs response: %v", err)
	}
	if len(systemPage.Items) != 1 || systemPage.Items[0].Message != "bootstrap failed" {
		t.Fatalf("unexpected system log page: %#v", systemPage.Items)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/http?status=503&path=/api/v1/live&limit=10", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for http logs, got %d", rec.Code)
	}
	var httpPage logging.HTTPRequestLogPage
	if err := json.NewDecoder(rec.Body).Decode(&httpPage); err != nil {
		t.Fatalf("decode http logs response: %v", err)
	}
	if len(httpPage.Items) != 1 || httpPage.Items[0].Path != "/api/v1/live/sessions" {
		t.Fatalf("unexpected http log page: %#v", httpPage.Items)
	}
	if httpPage.Items[0].Query != "token=REDACTED&mode=REDACTED" {
		t.Fatalf("expected redacted query in list response, got %#v", httpPage.Items[0].Query)
	}
	if httpPage.Items[0].RemoteAddr != "10.1.x.x" {
		t.Fatalf("expected masked remote addr in list response, got %#v", httpPage.Items[0].RemoteAddr)
	}
	if httpPage.Items[0].PanicMessage != "[redacted]" || httpPage.Items[0].Stack != "[redacted]" {
		t.Fatalf("expected sensitive fields to be redacted in list response, got %#v", httpPage.Items[0])
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/logs/http/"+sensitiveRequestLog.ID, nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for http log detail, got %d", rec.Code)
	}
	var detail logging.HTTPRequestLogEntry
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode http log detail response: %v", err)
	}
	if detail.Query != "token=abc123&mode=read" || detail.RemoteAddr != "10.1.2.3:443" {
		t.Fatalf("expected raw detail fields, got %#v", detail)
	}
	if detail.PanicMessage != "panic: sensitive downstream error" || detail.Stack != "top-secret stack trace" {
		t.Fatalf("expected raw sensitive detail fields, got %#v", detail)
	}
}

func TestLogRoutesExposeUnifiedEvents(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)
	mux := http.NewServeMux()
	registerLogRoutes(mux, platform)

	base := time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)
	if _, err := store.CreateStrategyDecisionEvent(domain.StrategyDecisionEvent{
		ID:               "decision-1",
		LiveSessionID:    "live-1",
		RuntimeSessionID: "runtime-1",
		AccountID:        "account-1",
		StrategyID:       "strategy-1",
		Action:           "wait",
		Reason:           "blocked by gate",
		SourceGateReady:  false,
		MissingCount:     1,
		EventTime:        base,
		RecordedAt:       base,
	}); err != nil {
		t.Fatalf("seed decision event: %v", err)
	}
	if _, err := store.CreateOrderExecutionEvent(domain.OrderExecutionEvent{
		ID:              "execution-1",
		OrderID:         "order-1",
		AccountID:       "account-1",
		LiveSessionID:   "live-1",
		DecisionEventID: "decision-1",
		Status:          "FAILED",
		EventType:       "submit",
		Failed:          true,
		EventTime:       base.Add(time.Minute),
		RecordedAt:      base.Add(time.Minute),
		Metadata: map[string]any{
			"strategyId": "strategy-1",
		},
	}); err != nil {
		t.Fatalf("seed order event: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/events?type=order-execution&accountId=account-1&level=critical&limit=10", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unified events, got %d", rec.Code)
	}
	var page service.UnifiedLogEventPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decode unified log page: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 unified event, got %d", len(page.Items))
	}
	if page.Items[0].ID != "execution-1" || page.Items[0].OrderID != "order-1" {
		t.Fatalf("unexpected unified event: %#v", page.Items[0])
	}
}

func TestLogStreamEndpointWritesSSE(t *testing.T) {
	logging.ResetForTests()
	t.Cleanup(logging.ResetForTests)

	platform := service.NewPlatform(memory.NewStore())
	mux := http.NewServeMux()
	registerLogRoutes(mux, platform)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream?source=system", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	logging.RecordSystemLog(logging.SystemLogEntry{
		Level:     "warning",
		Message:   "watcher tripped",
		CreatedAt: time.Now().UTC(),
	})
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream handler to exit")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: system-log") {
		t.Fatalf("expected system-log event in SSE output, got %q", body)
	}
	if !strings.Contains(body, "watcher tripped") {
		t.Fatalf("expected streamed payload in SSE output, got %q", body)
	}
}

func TestLogStreamEndpointSanitizesHTTPRequestPayload(t *testing.T) {
	logging.ResetForTests()
	t.Cleanup(logging.ResetForTests)

	platform := service.NewPlatform(memory.NewStore())
	mux := http.NewServeMux()
	registerLogRoutes(mux, platform)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream?source=http", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	logging.RecordHTTPRequest(logging.HTTPRequestLogEntry{
		Level:        "error",
		Message:      "http request failed",
		Method:       http.MethodPost,
		Path:         "/api/v1/orders",
		Query:        "token=abc123&account=live-main",
		RemoteAddr:   "10.1.2.3:443",
		Status:       http.StatusInternalServerError,
		DurationMs:   88,
		PanicMessage: "panic: exchange timeout",
		Stack:        "top-secret stack trace",
		CreatedAt:    time.Now().UTC(),
	})
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for http stream handler to exit")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: http-request") {
		t.Fatalf("expected http-request event in SSE output, got %q", body)
	}
	if strings.Contains(body, "abc123") || strings.Contains(body, "10.1.2.3:443") || strings.Contains(body, "top-secret stack trace") {
		t.Fatalf("expected http SSE payload to stay redacted, got %q", body)
	}
	if !strings.Contains(body, "token=REDACTED\\u0026account=REDACTED") || !strings.Contains(body, "10.1.x.x") || !strings.Contains(body, "[redacted]") {
		t.Fatalf("expected sanitized http SSE payload, got %q", body)
	}
}

func TestCollectPolledStreamMessagesDetectsAlertAndTimelineDeltas(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)

	previous := captureStreamSnapshot(platform)

	account, err := store.CreateAccount("live account", "LIVE", "BINANCE")
	if err != nil {
		t.Fatalf("create live account: %v", err)
	}
	session, err := store.CreateLiveSession(account.ID, "strategy-1")
	if err != nil {
		t.Fatalf("create live session: %v", err)
	}
	_, err = store.UpdateLiveSessionState(session.ID, map[string]any{
		"runner":       "strategy-engine",
		"dispatchMode": "manual-review",
		"timeline": []any{
			map[string]any{
				"time":     time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"category": "strategy",
				"title":    "decision",
				"metadata": map[string]any{"symbol": "BTCUSDT"},
			},
		},
	})
	if err != nil {
		t.Fatalf("update live session state: %v", err)
	}

	next, messages, err := collectPolledStreamMessages(platform, previous)
	if err != nil {
		t.Fatalf("collect polled stream messages: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected alert and timeline messages, got %#v", messages)
	}
	var alertCount, timelineCount int
	for _, message := range messages {
		switch message.Source {
		case "alert":
			alertCount++
		case "timeline":
			timelineCount++
		}
	}
	if alertCount == 0 || timelineCount == 0 {
		t.Fatalf("expected at least one alert and one timeline message, got alert=%d timeline=%d", alertCount, timelineCount)
	}

	_, repeatMessages, err := collectPolledStreamMessages(platform, next)
	if err != nil {
		t.Fatalf("collect repeated snapshot: %v", err)
	}
	if len(repeatMessages) != 0 {
		t.Fatalf("expected no duplicate messages on identical snapshot, got %#v", repeatMessages)
	}
}

func TestCollectPolledStreamMessagesSkipsInvalidTimelineTimestamp(t *testing.T) {
	store := memory.NewStore()
	platform := service.NewPlatform(store)

	previous := captureStreamSnapshot(platform)

	account, err := store.CreateAccount("paper account", "PAPER", "BINANCE")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	session, err := store.CreateLiveSession(account.ID, "strategy-1")
	if err != nil {
		t.Fatalf("create live session: %v", err)
	}
	_, err = store.UpdateLiveSessionState(session.ID, map[string]any{
		"runner":       "strategy-engine",
		"dispatchMode": "manual-review",
		"timeline": []any{
			map[string]any{
				"time":     "not-a-timestamp",
				"category": "strategy",
				"title":    "decision",
				"metadata": map[string]any{"symbol": "BTCUSDT"},
			},
		},
	})
	if err != nil {
		t.Fatalf("update live session state: %v", err)
	}

	next, messages, err := collectPolledStreamMessages(platform, previous)
	if err != nil {
		t.Fatalf("collect messages with invalid timestamp: %v", err)
	}
	for _, message := range messages {
		if message.Source == "timeline" {
			t.Fatalf("expected invalid timeline entries to be skipped, got %#v", message)
		}
	}

	_, repeatMessages, err := collectPolledStreamMessages(platform, next)
	if err != nil {
		t.Fatalf("collect repeated messages with invalid timestamp: %v", err)
	}
	for _, message := range repeatMessages {
		if message.Source == "timeline" {
			t.Fatalf("expected invalid timeline entries to stay skipped, got %#v", message)
		}
	}
}
