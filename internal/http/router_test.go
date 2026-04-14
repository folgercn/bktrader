package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type capturedRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

type captureHandler struct {
	mu      *sync.Mutex
	records *[]capturedRecord
	attrs   []slog.Attr
}

func requirePositiveIntAttr(t *testing.T, value any, name string) {
	t.Helper()
	switch typed := value.(type) {
	case int:
		if typed <= 0 {
			t.Fatalf("expected %s > 0, got %d", name, typed)
		}
	case int64:
		if typed <= 0 {
			t.Fatalf("expected %s > 0, got %d", name, typed)
		}
	default:
		t.Fatalf("expected integer attr for %s, got %#v", name, value)
	}
}

func newCaptureHandler() (*captureHandler, *[]capturedRecord) {
	records := make([]capturedRecord, 0, 8)
	return &captureHandler{
		mu:      &sync.Mutex{},
		records: &records,
	}, &records
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]any)
	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Any()
	}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	h.mu.Lock()
	*h.records = append(*h.records, capturedRecord{
		level: record.Level,
		msg:   record.Message,
		attrs: attrs,
	})
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &captureHandler{mu: h.mu, records: h.records, attrs: combined}
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return &captureHandler{mu: h.mu, records: h.records, attrs: append([]slog.Attr{}, h.attrs...)}
}

func TestRequestLogMiddlewareCapturesCompletedRequest(t *testing.T) {
	handler, records := newCaptureHandler()
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(original) })

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test?foo=bar", nil)
	rec := httptest.NewRecorder()
	requestLogMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	if len(*records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(*records))
	}
	record := (*records)[0]
	if record.msg != "http request completed" {
		t.Fatalf("expected completion log, got %q", record.msg)
	}
	if status, ok := record.attrs["status"].(int64); !ok || status != http.StatusCreated {
		t.Fatalf("expected status attr 201, got %#v", record.attrs["status"])
	}
	if component := record.attrs["component"]; component != "http" {
		t.Fatalf("expected component attr http, got %#v", component)
	}
	requirePositiveIntAttr(t, record.attrs["bytes_written"], "bytes_written")
}

func TestRequestLogMiddlewareRecoversPanics(t *testing.T) {
	handler, records := newCaptureHandler()
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(original) })

	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	requestLogMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Fatalf("expected panic response body, got %q", body)
	}
	if len(*records) != 2 {
		t.Fatalf("expected 2 log records, got %d", len(*records))
	}
	if (*records)[0].msg != "http request panicked" {
		t.Fatalf("expected panic log first, got %q", (*records)[0].msg)
	}
	if (*records)[1].msg != "http request failed" {
		t.Fatalf("expected failed request log second, got %q", (*records)[1].msg)
	}
	if status, ok := (*records)[1].attrs["status"].(int64); !ok || status != http.StatusInternalServerError {
		t.Fatalf("expected status attr 500, got %#v", (*records)[1].attrs["status"])
	}
}
