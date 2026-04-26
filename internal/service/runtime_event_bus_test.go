package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestBuildRuntimeSignalEventEnvelopeAndStableFingerprint(t *testing.T) {
	session := testRuntimeEventSession()
	eventTime := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	createdAt := eventTime.Add(time.Second)
	summary := map[string]any{
		"event":              "kline",
		"sourceKey":          "fast:1m",
		"role":               "trigger",
		"streamType":         "signal_bar",
		"subscriptionSymbol": "BTCUSDT",
		"timeframe":          "1m",
		"barStart":           "1777197600000",
		"barEnd":             "1777197659999",
		"close":              "65000",
	}

	event, err := BuildRuntimeSignalEvent(session, summary, eventTime, createdAt)
	if err != nil {
		t.Fatalf("BuildRuntimeSignalEvent returned error: %v", err)
	}
	if event.ID == "" || event.Fingerprint == "" || event.Subject == "" {
		t.Fatalf("expected id/fingerprint/subject, got %#v", event)
	}
	if event.RuntimeSessionID != session.ID || event.AccountID != session.AccountID || event.StrategyID != session.StrategyID {
		t.Fatalf("event identity mismatch: %#v", event)
	}
	if event.SourceKey != "fast:1m" || event.Role != "trigger" || event.StreamType != "signal_bar" {
		t.Fatalf("event source fields mismatch: %#v", event)
	}
	if event.Symbol != "BTCUSDT" || event.Timeframe != "1m" || event.EventType != "kline" {
		t.Fatalf("event market fields mismatch: %#v", event)
	}
	if event.Subject != "bktrader.runtime.signal.v1.account-1.strategy-1.BTCUSDT.signal_bar" {
		t.Fatalf("unexpected subject: %s", event.Subject)
	}

	eventAgain, err := BuildRuntimeSignalEvent(session, summary, eventTime.Add(5*time.Second), createdAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildRuntimeSignalEvent second call returned error: %v", err)
	}
	if eventAgain.Fingerprint != event.Fingerprint {
		t.Fatalf("signal bar fingerprint should ignore receive/create time: %s != %s", eventAgain.Fingerprint, event.Fingerprint)
	}
	if eventAgain.ID != event.ID {
		t.Fatalf("event id should be derived from fingerprint: %s != %s", eventAgain.ID, event.ID)
	}
}

func TestMemoryRuntimeEventPublisherIdempotentByFingerprint(t *testing.T) {
	publisher := NewMemoryRuntimeEventPublisher()
	event := RuntimeEventEnvelope{
		ID:               "runtime-event-1",
		RuntimeSessionID: "runtime-1",
		EventType:        "kline",
		EventTime:        time.Now().UTC(),
		Fingerprint:      "fingerprint-1",
		Payload:          map[string]any{"symbol": "BTCUSDT"},
	}

	if err := publisher.PublishRuntimeEvent(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := publisher.PublishRuntimeEvent(context.Background(), event); err != nil {
		t.Fatalf("duplicate publish failed: %v", err)
	}
	if got := len(publisher.Events()); got != 1 {
		t.Fatalf("expected one stored event after duplicate publish, got %d", got)
	}
	if publisher.PublishCalls() != 2 || publisher.DuplicateHits() != 1 {
		t.Fatalf("unexpected publish accounting: calls=%d duplicates=%d", publisher.PublishCalls(), publisher.DuplicateHits())
	}
}

func TestRuntimeEventStreamConfig(t *testing.T) {
	cfg := RuntimeEventStreamConfig()
	if cfg.Name != RuntimeEventStreamName {
		t.Fatalf("unexpected stream name: %s", cfg.Name)
	}
	if len(cfg.Subjects) != 1 || cfg.Subjects[0] != RuntimeEventSubjectPattern {
		t.Fatalf("unexpected subjects: %#v", cfg.Subjects)
	}
	if cfg.Retention != nats.WorkQueuePolicy {
		t.Fatalf("unexpected retention policy: %v", cfg.Retention)
	}
	if cfg.MaxAge != 7*24*time.Hour {
		t.Fatalf("unexpected max age: %s", cfg.MaxAge)
	}
}

func TestRuntimeEventPublishFailureRecordsStateWithoutBlocking(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	publisher := NewMemoryRuntimeEventPublisher()
	publisher.SetPublishError(errors.New("nats unavailable"))
	platform.SetRuntimeEventPublisher(publisher)
	session := testRuntimeEventSession()
	if _, err := store.CreateSignalRuntimeSession(session); err != nil {
		t.Fatalf("create runtime session failed: %v", err)
	}
	platform.cacheSignalRuntimeSession(session)
	summary := map[string]any{
		"event":              "kline",
		"sourceKey":          "fast:1m",
		"role":               "trigger",
		"streamType":         "signal_bar",
		"subscriptionSymbol": "BTCUSDT",
		"timeframe":          "1m",
		"barStart":           "1777197600000",
		"barEnd":             "1777197659999",
	}

	started := time.Now()
	platform.publishRuntimeSignalEvent(session, summary, started.UTC())
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("publishRuntimeSignalEvent blocked for %s", elapsed)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		updated, err := platform.GetSignalRuntimeSession(session.ID)
		if err != nil {
			t.Fatalf("get runtime session failed: %v", err)
		}
		if stringValue(updated.State["lastRuntimeEventPublishError"]) == "nats unavailable" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected publish failure to be recorded in runtime session state")
}

func TestRuntimeEventTickPublishThrottledPerSymbol(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	publisher := NewMemoryRuntimeEventPublisher()
	platform.SetRuntimeEventPublisher(publisher)
	session := testRuntimeEventSession()
	baseTime := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	summary := map[string]any{
		"event":              "trade",
		"sourceKey":          "tick",
		"role":               "trigger",
		"streamType":         "trade_tick",
		"subscriptionSymbol": "BTCUSDT",
		"price":              "65000",
	}

	platform.publishRuntimeSignalEvent(session, summary, baseTime)
	platform.publishRuntimeSignalEvent(session, summary, baseTime.Add(500*time.Millisecond))
	platform.publishRuntimeSignalEvent(session, summary, baseTime.Add(time.Second))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(publisher.Events()) == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected two published tick events after throttle, got %d", len(publisher.Events()))
}

func testRuntimeEventSession() domain.SignalRuntimeSession {
	now := time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC)
	return domain.SignalRuntimeSession{
		ID:             "runtime-1",
		AccountID:      "account-1",
		StrategyID:     "strategy-1",
		Status:         "RUNNING",
		RuntimeAdapter: "binance-market-ws",
		Transport:      "websocket",
		State:          map[string]any{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
