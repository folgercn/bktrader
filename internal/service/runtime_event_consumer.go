package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	RuntimeEventLiveEvaluationDurable = "live-evaluation"
	RuntimeSignalEventSubjectPattern  = RuntimeSignalSubjectPrefix + ".>"
)

type RuntimeEventMessage struct {
	Event RuntimeEventEnvelope
	Ack   func() error
	Nak   func(error) error
}

func (p *Platform) HandleRuntimeEventMessage(ctx context.Context, msg RuntimeEventMessage) error {
	return p.handleRuntimeEventMessage(ctx, msg, time.Now().UTC())
}

func (p *Platform) handleRuntimeEventMessage(ctx context.Context, msg RuntimeEventMessage, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	event := msg.Event
	if err := validateRuntimeEventForLiveEvaluation(event); err != nil {
		return p.nakRuntimeEventMessage(msg, err)
	}
	if p.runtimeEventIsStale(event, now) {
		p.logger("service.runtime_event_consumer",
			"runtime_session_id", event.RuntimeSessionID,
			"event_id", event.ID,
			"fingerprint", event.Fingerprint,
			"event_time", event.EventTime.UTC().Format(time.RFC3339),
		).Warn("dropping stale runtime event")
		return p.ackRuntimeEventMessage(msg)
	}
	if err := p.handleSignalRuntimeMessageForConsumer(event.RuntimeSessionID, event.Payload, event.EventTime); err != nil {
		p.logger("service.runtime_event_consumer",
			"runtime_session_id", event.RuntimeSessionID,
			"event_id", event.ID,
			"fingerprint", event.Fingerprint,
		).Warn("runtime event live evaluation failed; leaving event unacked", "error", err)
		return p.nakRuntimeEventMessage(msg, err)
	}
	return p.ackRuntimeEventMessage(msg)
}

func validateRuntimeEventForLiveEvaluation(event RuntimeEventEnvelope) error {
	if event.ID == "" {
		return errors.New("runtime event missing id")
	}
	if event.RuntimeSessionID == "" {
		return errors.New("runtime event missing runtime session id")
	}
	if event.Fingerprint == "" {
		return errors.New("runtime event missing fingerprint")
	}
	if event.EventTime.IsZero() {
		return errors.New("runtime event missing event time")
	}
	if len(event.Payload) == 0 {
		return errors.New("runtime event missing payload")
	}
	return nil
}

func (p *Platform) runtimeEventIsStale(event RuntimeEventEnvelope, now time.Time) bool {
	thresholdSeconds := p.runtimeEventFreshnessSeconds(event.StreamType)
	if thresholdSeconds <= 0 {
		return false
	}
	return now.UTC().Sub(event.EventTime.UTC()) > time.Duration(thresholdSeconds)*time.Second
}

func (p *Platform) runtimeEventFreshnessSeconds(streamType string) int {
	switch streamType {
	case "trade_tick", "replay_tick":
		return p.runtimePolicy.TradeTickFreshnessSeconds
	case "order_book":
		return p.runtimePolicy.OrderBookFreshnessSeconds
	case "signal_bar":
		return p.runtimePolicy.SignalBarFreshnessSeconds
	default:
		return p.runtimePolicy.SignalBarFreshnessSeconds
	}
}

func (p *Platform) ackRuntimeEventMessage(msg RuntimeEventMessage) error {
	if msg.Ack == nil {
		return nil
	}
	return msg.Ack()
}

func (p *Platform) nakRuntimeEventMessage(msg RuntimeEventMessage, cause error) error {
	if msg.Nak != nil {
		if err := msg.Nak(cause); err != nil {
			return errors.Join(cause, fmt.Errorf("nak failed: %w", err))
		}
	}
	return cause
}

func (p *Platform) handleSignalRuntimeMessageForConsumer(runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	return p.handleSignalRuntimeMessageWithOptions(runtimeSessionID, summary, eventTime, signalRuntimeFanoutOptions{
		returnTriggerErrors: true,
	})
}
