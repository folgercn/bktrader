package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoBinanceRESTRequestClassifiesHTTPFailureAsAdapterError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":-2015,"msg":"invalid api-key"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	previousLimiter := binanceRESTLimiterState
	previousRate := binanceRESTRequestsPerSecond
	previousBurst := binanceRESTBurst
	binanceRESTLimiterState = newBinanceRESTLimiter()
	binanceRESTRequestsPerSecond = 1000
	binanceRESTBurst = 10
	defer func() {
		binanceRESTLimiterState = previousLimiter
		binanceRESTRequestsPerSecond = previousRate
		binanceRESTBurst = previousBurst
	}()

	_, _, err := doBinancePublicGET(server.URL, "/fapi/v1/exchangeInfo", map[string]string{
		"symbol": "BTCUSDT",
	}, binanceRESTCategoryMetadataRead)
	if err == nil {
		t.Fatal("expected HTTP failure")
	}
	if !errors.Is(err, ErrLiveControlAdapter) {
		t.Fatalf("expected adapter error sentinel, got %v", err)
	}
	if got := liveSessionControlErrorCode(err); got != LiveSessionControlErrorCodeAdapterError {
		t.Fatalf("expected %s, got %s", LiveSessionControlErrorCodeAdapterError, got)
	}
}

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

func TestDoBinanceSignedRequestRefreshesTimestampAfterLimiterWait(t *testing.T) {
	var requestCount atomic.Int32
	var secondTimestamp int64
	var secondReceivedAt int64
	var secondSignature string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 2 {
			query := r.URL.Query()
			secondReceivedAt = time.Now().UTC().UnixMilli()
			secondSignature = query.Get("signature")
			value, err := strconv.ParseInt(query.Get("timestamp"), 10, 64)
			if err != nil {
				t.Fatalf("parse timestamp failed: %v", err)
			}
			secondTimestamp = value
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	previousLimiter := binanceRESTLimiterState
	previousRate := binanceRESTRequestsPerSecond
	previousBurst := binanceRESTBurst
	binanceRESTLimiterState = newBinanceRESTLimiter()
	binanceRESTRequestsPerSecond = 1
	binanceRESTBurst = 1
	defer func() {
		binanceRESTLimiterState = previousLimiter
		binanceRESTRequestsPerSecond = previousRate
		binanceRESTBurst = previousBurst
	}()

	creds := binanceRESTCredentials{
		APIKeyRef:    "test-key-ref",
		APISecretRef: "test-secret-ref",
		APIKey:       "test-key",
		APISecret:    "test-secret",
		BaseURL:      server.URL,
	}
	staleParams := map[string]string{
		"symbol":     "BTCUSDT",
		"timestamp":  "1",
		"signature":  "stale-signature",
		"recvWindow": "5000",
	}

	if _, err := doBinanceSignedRequest(http.MethodGet, creds, "/fapi/v1/order", staleParams, binanceRESTCategoryTradeCritical); err != nil {
		t.Fatalf("first signed request failed: %v", err)
	}
	startedAt := time.Now().UTC()
	if _, err := doBinanceSignedRequest(http.MethodGet, creds, "/fapi/v1/order", staleParams, binanceRESTCategoryTradeCritical); err != nil {
		t.Fatalf("second signed request failed: %v", err)
	}
	elapsed := time.Since(startedAt)
	if elapsed < 700*time.Millisecond {
		t.Fatalf("expected second request to wait for limiter token, waited %s", elapsed)
	}
	if secondTimestamp <= startedAt.UnixMilli()+500 {
		t.Fatalf("expected signed timestamp to be refreshed after limiter wait, start=%d signed=%d elapsed=%s", startedAt.UnixMilli(), secondTimestamp, elapsed)
	}
	if diff := secondReceivedAt - secondTimestamp; diff < 0 || diff > 500 {
		t.Fatalf("expected signed timestamp to be close to actual send time, diff_ms=%d", diff)
	}
	if secondSignature == "" || secondSignature == "stale-signature" {
		t.Fatalf("expected stale signature to be replaced, got %q", secondSignature)
	}
	expectedParams := map[string]string{
		"symbol":     "BTCUSDT",
		"timestamp":  strconv.FormatInt(secondTimestamp, 10),
		"recvWindow": "5000",
	}
	if expected := signBinanceQuery(expectedParams, creds.APISecret); secondSignature != expected {
		t.Fatalf("expected signature to match refreshed timestamp, got %s want %s", secondSignature, expected)
	}
}

func TestBinanceRESTGatePrioritizesTradeCriticalOverQueuedAccountSync(t *testing.T) {
	gate := newBinanceRESTGate(20, 1)
	if err := gate.acquire(binanceRESTCategoryAccountSync); err != nil {
		t.Fatalf("initial acquire failed: %v", err)
	}

	completed := make(chan binanceRESTRequestCategory, 2)
	go func() {
		if err := gate.acquire(binanceRESTCategoryAccountSync); err != nil {
			t.Errorf("account-sync acquire failed: %v", err)
			return
		}
		completed <- binanceRESTCategoryAccountSync
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		if err := gate.acquire(binanceRESTCategoryTradeCritical); err != nil {
			t.Errorf("trade-critical acquire failed: %v", err)
			return
		}
		completed <- binanceRESTCategoryTradeCritical
	}()

	select {
	case got := <-completed:
		if got != binanceRESTCategoryTradeCritical {
			t.Fatalf("expected trade-critical to bypass queued account-sync request, got %s", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for prioritized acquire")
	}
	select {
	case got := <-completed:
		if got != binanceRESTCategoryAccountSync {
			t.Fatalf("expected account-sync to acquire next, got %s", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for account-sync acquire")
	}
}

func TestBinanceRESTGateDoesNotConsumeTokensDuringBackoff(t *testing.T) {
	gate := newBinanceRESTGate(20, 1)
	if err := gate.acquire(binanceRESTCategoryTradeCritical); err != nil {
		t.Fatalf("initial acquire failed: %v", err)
	}

	completed := make(chan error, 1)
	go func() {
		completed <- gate.acquire(binanceRESTCategoryTradeCritical)
	}()
	time.Sleep(10 * time.Millisecond)
	gate.markBackoff(80 * time.Millisecond)

	select {
	case err := <-completed:
		t.Fatalf("expected queued request to wait during backoff without consuming token, got err=%v", err)
	case <-time.After(60 * time.Millisecond):
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("expected queued request to acquire after backoff without consuming token early, got %v", err)
		}
	case <-time.After(160 * time.Millisecond):
		t.Fatal("timed out waiting for queued request after backoff")
	}
}

func TestBinanceRESTGateUnknownCategoryDefaultsToLowestPriority(t *testing.T) {
	if got := normalizeBinanceRESTRequestCategory(binanceRESTRequestCategory("new-background-class")); got != binanceRESTCategoryMarketData {
		t.Fatalf("expected unknown category to default to lowest priority, got %s", got)
	}
}
