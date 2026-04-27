package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	RuntimeEventStreamName     = "BKT_RUNTIME_EVENTS"
	RuntimeEventSubjectPattern = "bktrader.runtime.>"
	RuntimeSignalSubjectPrefix = "bktrader.runtime.signal.v1"

	runtimeEventPublishTimeout = 100 * time.Millisecond
)

var runtimeEventSubjectTokenRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// RuntimeEventEnvelope is the stable cross-process event contract for runtime
// market events. Payload is the already-summarized runtime message, not a raw
// exchange frame or trading fact source.
type RuntimeEventEnvelope struct {
	ID               string         `json:"id"`
	RuntimeSessionID string         `json:"runtime_session_id"`
	AccountID        string         `json:"account_id"`
	StrategyID       string         `json:"strategy_id"`
	SourceKey        string         `json:"source_key"`
	Role             string         `json:"role"`
	StreamType       string         `json:"stream_type"`
	Symbol           string         `json:"symbol"`
	Timeframe        string         `json:"timeframe,omitempty"`
	EventType        string         `json:"event_type"`
	EventTime        time.Time      `json:"event_time"`
	Fingerprint      string         `json:"fingerprint"`
	Subject          string         `json:"subject"`
	Payload          map[string]any `json:"payload"`
	CreatedAt        time.Time      `json:"created_at"`
}

type RuntimeEventPublisher interface {
	PublishRuntimeEvent(ctx context.Context, event RuntimeEventEnvelope) error
}

type runtimeEventPublishThrottleState struct {
	mu            sync.Mutex
	lastPublished time.Time
}

type NoopRuntimeEventPublisher struct{}

func (NoopRuntimeEventPublisher) PublishRuntimeEvent(context.Context, RuntimeEventEnvelope) error {
	return nil
}

// MemoryRuntimeEventPublisher is an in-process fake used by unit tests. It
// applies the same fingerprint/idempotency rule expected from JetStream Msg-Id.
type MemoryRuntimeEventPublisher struct {
	mu            sync.Mutex
	events        []RuntimeEventEnvelope
	seen          map[string]struct{}
	publishErr    error
	publishCalls  int
	duplicateHits int
}

func NewMemoryRuntimeEventPublisher() *MemoryRuntimeEventPublisher {
	return &MemoryRuntimeEventPublisher{
		seen: make(map[string]struct{}),
	}
}

func (p *MemoryRuntimeEventPublisher) PublishRuntimeEvent(ctx context.Context, event RuntimeEventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishCalls++
	if p.publishErr != nil {
		return p.publishErr
	}
	key := firstNonEmpty(event.Fingerprint, event.ID)
	if key == "" {
		key = runtimeEventHash(map[string]any{
			"runtime_session_id": event.RuntimeSessionID,
			"event_type":         event.EventType,
			"event_time":         event.EventTime.UTC().UnixMilli(),
		})
	}
	if _, exists := p.seen[key]; exists {
		p.duplicateHits++
		return nil
	}
	p.seen[key] = struct{}{}
	p.events = append(p.events, cloneRuntimeEvent(event))
	return nil
}

func (p *MemoryRuntimeEventPublisher) SetPublishError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishErr = err
}

func (p *MemoryRuntimeEventPublisher) Events() []RuntimeEventEnvelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]RuntimeEventEnvelope, 0, len(p.events))
	for _, event := range p.events {
		out = append(out, cloneRuntimeEvent(event))
	}
	return out
}

func (p *MemoryRuntimeEventPublisher) PublishCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.publishCalls
}

func (p *MemoryRuntimeEventPublisher) DuplicateHits() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.duplicateHits
}

func BuildRuntimeSignalEvent(session domain.SignalRuntimeSession, summary map[string]any, eventTime, createdAt time.Time) (RuntimeEventEnvelope, error) {
	payload := runtimeEventPayload(summary)
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	streamType := inferSignalRuntimeStreamType(payload)
	symbol := signalRuntimeSummarySymbol(payload)
	eventType := firstNonEmpty(strings.ToLower(strings.TrimSpace(stringValue(payload["event"]))), "message")
	sourceKey := stringValue(payload["sourceKey"])
	role := stringValue(payload["role"])
	timeframe := normalizeSignalBarInterval(stringValue(payload["timeframe"]))
	if timeframe == "" {
		timeframe = strings.ToLower(strings.TrimSpace(stringValue(payload["timeframe"])))
	}

	event := RuntimeEventEnvelope{
		RuntimeSessionID: session.ID,
		AccountID:        session.AccountID,
		StrategyID:       session.StrategyID,
		SourceKey:        sourceKey,
		Role:             role,
		StreamType:       streamType,
		Symbol:           symbol,
		Timeframe:        timeframe,
		EventType:        eventType,
		EventTime:        eventTime.UTC(),
		Payload:          payload,
		CreatedAt:        createdAt.UTC(),
	}
	if event.RuntimeSessionID == "" {
		return RuntimeEventEnvelope{}, errors.New("runtime event missing runtime session id")
	}
	if event.AccountID == "" {
		return RuntimeEventEnvelope{}, errors.New("runtime event missing account id")
	}
	if event.StrategyID == "" {
		return RuntimeEventEnvelope{}, errors.New("runtime event missing strategy id")
	}
	if event.Symbol == "" {
		return RuntimeEventEnvelope{}, errors.New("runtime event missing symbol")
	}
	if event.StreamType == "" {
		event.StreamType = "message"
	}
	event.Subject = RuntimeSignalEventSubject(event.AccountID, event.StrategyID, event.Symbol, event.StreamType)
	event.Fingerprint = RuntimeSignalEventFingerprint(event)
	event.ID = "runtime-event-" + event.Fingerprint
	return event, nil
}

func RuntimeSignalEventSubject(accountID, strategyID, symbol, streamType string) string {
	return strings.Join([]string{
		RuntimeSignalSubjectPrefix,
		runtimeEventSubjectToken(accountID),
		runtimeEventSubjectToken(strategyID),
		runtimeEventSubjectToken(NormalizeSymbol(symbol)),
		runtimeEventSubjectToken(strings.ToLower(strings.TrimSpace(streamType))),
	}, ".")
}

func RuntimeSignalEventFingerprint(event RuntimeEventEnvelope) string {
	base := map[string]any{
		"runtime_session_id": event.RuntimeSessionID,
		"account_id":         event.AccountID,
		"strategy_id":        event.StrategyID,
		"source_key":         event.SourceKey,
		"role":               event.Role,
		"stream_type":        event.StreamType,
		"symbol":             event.Symbol,
		"timeframe":          event.Timeframe,
		"event_type":         event.EventType,
	}
	if event.StreamType == "signal_bar" {
		base["bar_start"] = canonicalSignalBarTimestamp(event.Payload["barStart"])
		base["bar_end"] = canonicalSignalBarTimestamp(event.Payload["barEnd"])
		return runtimeEventHash(base)
	}
	if exchangeID := runtimeEventExchangeID(event.Payload); exchangeID != "" {
		base["exchange_event_id"] = exchangeID
		return runtimeEventHash(base)
	}
	base["event_time_ms"] = event.EventTime.UTC().UnixMilli()
	base["payload"] = event.Payload
	return runtimeEventHash(base)
}

func runtimeEventPayload(summary map[string]any) map[string]any {
	payload := cloneMetadata(summary)
	delete(payload, "message")
	return payload
}

func runtimeEventHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte(fmt.Sprint(value))
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func runtimeEventExchangeID(payload map[string]any) string {
	for _, key := range []string{"eventId", "eventID", "exchangeEventId", "exchangeEventID", "tradeId", "tradeID", "aggTradeId", "aggTradeID"} {
		if value := strings.TrimSpace(stringValue(payload[key])); value != "" {
			return value
		}
	}
	return ""
}

func runtimeEventSubjectToken(value string) string {
	token := strings.Trim(runtimeEventSubjectTokenRe.ReplaceAllString(strings.TrimSpace(value), "_"), "_")
	if token == "" {
		return "_"
	}
	return token
}

func cloneRuntimeEvent(event RuntimeEventEnvelope) RuntimeEventEnvelope {
	event.Payload = cloneMetadata(event.Payload)
	return event
}

func (p *Platform) SetRuntimeEventPublisher(publisher RuntimeEventPublisher) {
	if publisher == nil {
		publisher = NoopRuntimeEventPublisher{}
	}
	p.mu.Lock()
	p.runtimeEventPublisher = publisher
	p.mu.Unlock()
}

func (p *Platform) publishRuntimeSignalEvent(session domain.SignalRuntimeSession, summary map[string]any, eventTime time.Time) {
	if p == nil {
		return
	}
	p.mu.Lock()
	publisher := p.runtimeEventPublisher
	p.mu.Unlock()
	if publisher == nil {
		return
	}
	if signalRuntimeSummarySymbol(summary) == "" {
		return
	}
	if p.shouldThrottleRuntimeEventPublish(session.ID, summary, eventTime) {
		return
	}
	createdAt := time.Now().UTC()
	event, err := BuildRuntimeSignalEvent(session, summary, eventTime, createdAt)
	if err != nil {
		p.recordRuntimeEventPublishFailure(session.ID, summary, eventTime, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeEventPublishTimeout)
		defer cancel()
		if err := publisher.PublishRuntimeEvent(ctx, event); err != nil {
			p.recordRuntimeEventPublishFailure(session.ID, summary, eventTime, err)
			p.logger("service.runtime_event_bus",
				"session_id", session.ID,
				"subject", event.Subject,
				"fingerprint", event.Fingerprint,
			).Warn("runtime event publish failed", "error", err)
			return
		}
		p.recordRuntimeEventPublishSuccess(session.ID, event)
	}()
}

func (p *Platform) shouldThrottleRuntimeEventPublish(sessionID string, summary map[string]any, eventTime time.Time) bool {
	streamType := inferSignalRuntimeStreamType(summary)
	if streamType != "trade_tick" {
		return false
	}
	symbol := signalRuntimeSummarySymbol(summary)
	if symbol == "" {
		return false
	}
	key := sessionID + "|" + symbol + "|" + streamType
	raw, _ := p.runtimeEventThrottle.LoadOrStore(key, &runtimeEventPublishThrottleState{})
	state, ok := raw.(*runtimeEventPublishThrottleState)
	if !ok {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.lastPublished.IsZero() && eventTime.Sub(state.lastPublished) < time.Second {
		return true
	}
	state.lastPublished = eventTime
	return false
}

func (p *Platform) clearRuntimeEventPublishThrottleSession(sessionID string) {
	prefix := sessionID + "|"
	p.runtimeEventThrottle.Range(func(key, _ any) bool {
		if keyStr, ok := key.(string); ok && strings.HasPrefix(keyStr, prefix) {
			p.runtimeEventThrottle.Delete(keyStr)
		}
		return true
	})
}

func (p *Platform) recordRuntimeEventPublishSuccess(sessionID string, event RuntimeEventEnvelope) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		state := cloneMetadata(session.State)
		state["lastRuntimeEventPublishedAt"] = event.CreatedAt.Format(time.RFC3339)
		state["lastRuntimeEventSubject"] = event.Subject
		state["lastRuntimeEventFingerprint"] = event.Fingerprint
		delete(state, "lastRuntimeEventPublishError")
		delete(state, "lastRuntimeEventPublishErrorAt")
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	})
}

func (p *Platform) recordRuntimeEventPublishFailure(sessionID string, summary map[string]any, eventTime time.Time, err error) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		state := cloneMetadata(session.State)
		state["lastRuntimeEventPublishError"] = err.Error()
		state["lastRuntimeEventPublishErrorAt"] = time.Now().UTC().Format(time.RFC3339)
		state["lastRuntimeEventPublishEventAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastRuntimeEventPublishSummary"] = cloneMetadata(summary)
		state["runtimeEventPublishFailureCount"] = maxIntValue(state["runtimeEventPublishFailureCount"], 0) + 1
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	})
}
