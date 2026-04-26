package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestRuntimeEventLiveEvaluationConsumerConfig(t *testing.T) {
	cfg := RuntimeEventLiveEvaluationConsumerConfig()
	if cfg.Durable != RuntimeEventLiveEvaluationDurable {
		t.Fatalf("unexpected durable: %s", cfg.Durable)
	}
	if cfg.FilterSubject != RuntimeSignalEventSubjectPattern {
		t.Fatalf("unexpected filter subject: %s", cfg.FilterSubject)
	}
	if cfg.AckPolicy != nats.AckExplicitPolicy {
		t.Fatalf("unexpected ack policy: %v", cfg.AckPolicy)
	}
	if cfg.AckWait != 30*time.Second {
		t.Fatalf("unexpected ack wait: %s", cfg.AckWait)
	}
	if cfg.MaxDeliver != 5 {
		t.Fatalf("unexpected max deliver: %d", cfg.MaxDeliver)
	}
}

func TestHandleRuntimeEventMessageTriggersLiveEvaluationAndAcks(t *testing.T) {
	platform, session, runtimeSessionID, summary, eventTime := prepareLiveDecisionTelemetryFixture(t)
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}
	ack := &testRuntimeEventAck{}

	err := platform.handleRuntimeEventMessage(context.Background(), RuntimeEventMessage{
		Event: testLiveRuntimeEvent(runtimeSessionID, summary, eventTime),
		Ack:   ack.Ack,
		Nak:   ack.Nak,
	}, eventTime.Add(time.Second))
	if err != nil {
		t.Fatalf("handle runtime event failed: %v", err)
	}
	if ack.ackCount != 1 || ack.nakCount != 0 {
		t.Fatalf("expected ack once and no nak, got ack=%d nak=%d", ack.ackCount, ack.nakCount)
	}
	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got == "" {
		t.Fatalf("expected runtime event to update live session trigger state")
	}
	events, err := platform.store.ListStrategyDecisionEvents(session.ID)
	if err != nil {
		t.Fatalf("list decision events failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one strategy decision event, got %d", len(events))
	}
}

func TestHandleRuntimeEventMessageDuplicateIsIdempotent(t *testing.T) {
	platform, session, runtimeSessionID, summary, eventTime := prepareLiveDecisionTelemetryFixture(t)
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}
	ack := &testRuntimeEventAck{}
	event := testLiveRuntimeEvent(runtimeSessionID, summary, eventTime)

	for i := 0; i < 2; i++ {
		if err := platform.handleRuntimeEventMessage(context.Background(), RuntimeEventMessage{
			Event: event,
			Ack:   ack.Ack,
			Nak:   ack.Nak,
		}, eventTime.Add(time.Duration(i+1)*time.Second)); err != nil {
			t.Fatalf("handle runtime event #%d failed: %v", i+1, err)
		}
	}
	if ack.ackCount != 2 || ack.nakCount != 0 {
		t.Fatalf("expected duplicate delivery to ack twice and never nak, got ack=%d nak=%d", ack.ackCount, ack.nakCount)
	}
	events, err := platform.store.ListStrategyDecisionEvents(session.ID)
	if err != nil {
		t.Fatalf("list decision events failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected duplicate runtime event to reuse decision event, got %d events", len(events))
	}
}

func TestHandleRuntimeEventMessageStaleEventAcksWithoutEvaluation(t *testing.T) {
	platform, session, runtimeSessionID, summary, eventTime := prepareLiveDecisionTelemetryFixture(t)
	if _, err := platform.store.UpdateLiveSessionStatus(session.ID, "RUNNING"); err != nil {
		t.Fatalf("mark live session running failed: %v", err)
	}
	ack := &testRuntimeEventAck{}

	err := platform.handleRuntimeEventMessage(context.Background(), RuntimeEventMessage{
		Event: testLiveRuntimeEvent(runtimeSessionID, summary, eventTime),
		Ack:   ack.Ack,
		Nak:   ack.Nak,
	}, eventTime.Add(time.Duration(platform.runtimePolicy.TradeTickFreshnessSeconds+1)*time.Second))
	if err != nil {
		t.Fatalf("stale runtime event should be acked without error, got %v", err)
	}
	if ack.ackCount != 1 || ack.nakCount != 0 {
		t.Fatalf("expected stale event to ack and not nak, got ack=%d nak=%d", ack.ackCount, ack.nakCount)
	}
	updated, err := platform.store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastSignalRuntimeEventAt"]); got != "" {
		t.Fatalf("expected stale event to skip live evaluation, got lastSignalRuntimeEventAt=%s", got)
	}
}

func TestHandleRuntimeEventMessageFailureDoesNotAck(t *testing.T) {
	platform, _, _, _, eventTime := prepareLiveDecisionTelemetryFixture(t)
	ack := &testRuntimeEventAck{}

	err := platform.handleRuntimeEventMessage(context.Background(), RuntimeEventMessage{
		Event: RuntimeEventEnvelope{
			ID:               "runtime-event-invalid",
			RuntimeSessionID: "runtime-1",
			Fingerprint:      "fingerprint-invalid",
			StreamType:       "trade_tick",
			EventTime:        eventTime,
		},
		Ack: ack.Ack,
		Nak: ack.Nak,
	}, eventTime.Add(time.Second))
	if err == nil {
		t.Fatal("expected invalid runtime event to fail")
	}
	if ack.ackCount != 0 || ack.nakCount != 1 {
		t.Fatalf("expected failure to nak without ack, got ack=%d nak=%d", ack.ackCount, ack.nakCount)
	}
	if !errors.Is(err, errTestRuntimeEventNak) {
		t.Fatalf("expected wrapped nak error marker, got %v", err)
	}
}

func testLiveRuntimeEvent(runtimeSessionID string, summary map[string]any, eventTime time.Time) RuntimeEventEnvelope {
	return RuntimeEventEnvelope{
		ID:               "runtime-event-" + runtimeEventHash(summary),
		RuntimeSessionID: runtimeSessionID,
		Fingerprint:      "fingerprint-" + runtimeEventHash(summary),
		StreamType:       firstNonEmpty(inferSignalRuntimeStreamType(summary), "trade_tick"),
		Symbol:           signalRuntimeSummarySymbol(summary),
		EventType:        firstNonEmpty(stringValue(summary["event"]), "message"),
		EventTime:        eventTime.UTC(),
		Payload:          cloneMetadata(summary),
	}
}

var errTestRuntimeEventNak = errors.New("test nak")

type testRuntimeEventAck struct {
	ackCount int
	nakCount int
}

func (a *testRuntimeEventAck) Ack() error {
	a.ackCount++
	return nil
}

func (a *testRuntimeEventAck) Nak(error) error {
	a.nakCount++
	return errTestRuntimeEventNak
}
