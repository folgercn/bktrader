package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoBinanceSignedRequestBacksOffAfter429(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}))
	defer server.Close()

	previousLimiter := binanceRESTLimiterState
	previousRate := binanceRESTRequestsPerSecond
	previousBurst := binanceRESTBurst
	previousBackoff := binanceRESTBackoffDuration
	binanceRESTLimiterState = newBinanceRESTLimiter()
	binanceRESTRequestsPerSecond = 1000
	binanceRESTBurst = 10
	binanceRESTBackoffDuration = 50 * time.Millisecond
	defer func() {
		binanceRESTLimiterState = previousLimiter
		binanceRESTRequestsPerSecond = previousRate
		binanceRESTBurst = previousBurst
		binanceRESTBackoffDuration = previousBackoff
	}()

	creds := binanceRESTCredentials{
		APIKeyRef: "test-key-ref",
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   server.URL,
	}
	params := map[string]string{
		"symbol":     "BTCUSDT",
		"timestamp":  "1",
		"signature":  "test-signature",
		"recvWindow": "5000",
	}

	if _, err := doBinanceSignedRequest(http.MethodGet, creds, "/fapi/v1/order", params, binanceRESTCategoryTradeCritical); err == nil {
		t.Fatal("expected first request to fail with 429")
	}
	if _, err := doBinanceSignedRequest(http.MethodGet, creds, "/fapi/v1/order", params, binanceRESTCategoryTradeCritical); err == nil {
		t.Fatal("expected second request during backoff to be rejected")
	} else if !strings.Contains(err.Error(), "rate-limited") {
		t.Fatalf("expected second request to be rejected by local backoff, got %v", err)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected only the first request to reach the server during backoff, got %d", got)
	}
}

func TestBinancePublicRequestsShareBackoffGateWithSignedRequests(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}))
	defer server.Close()

	previousLimiter := binanceRESTLimiterState
	previousRate := binanceRESTRequestsPerSecond
	previousBurst := binanceRESTBurst
	previousBackoff := binanceRESTBackoffDuration
	binanceRESTLimiterState = newBinanceRESTLimiter()
	binanceRESTRequestsPerSecond = 1000
	binanceRESTBurst = 10
	binanceRESTBackoffDuration = 50 * time.Millisecond
	defer func() {
		binanceRESTLimiterState = previousLimiter
		binanceRESTRequestsPerSecond = previousRate
		binanceRESTBurst = previousBurst
		binanceRESTBackoffDuration = previousBackoff
	}()

	creds := binanceRESTCredentials{
		APIKeyRef: "test-key-ref",
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   server.URL,
	}
	params := map[string]string{
		"symbol":     "BTCUSDT",
		"timestamp":  "1",
		"signature":  "test-signature",
		"recvWindow": "5000",
	}

	if _, err := doBinanceSignedRequest(http.MethodGet, creds, "/fapi/v1/order", params, binanceRESTCategoryTradeCritical); err == nil {
		t.Fatal("expected signed request to fail with 429")
	}
	if _, _, err := doBinancePublicGET(server.URL, "/fapi/v1/klines", map[string]string{
		"symbol":   "BTCUSDT",
		"interval": "1m",
	}, binanceRESTCategoryMarketData); err == nil {
		t.Fatal("expected public request during backoff to be rejected")
	} else if !strings.Contains(err.Error(), "rate-limited") {
		t.Fatalf("expected public request to be rejected by shared local backoff, got %v", err)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected only the signed request to reach the server during shared backoff, got %d", got)
	}
}
