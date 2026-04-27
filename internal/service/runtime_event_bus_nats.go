package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	runtimeEventNATSConnectTimeout = 500 * time.Millisecond
	runtimeEventNATSMaxReconnects  = 3
	runtimeEventCircuitThreshold   = 3
	runtimeEventCircuitOpenFor     = 5 * time.Second
)

type NATSRuntimeEventPublisher struct {
	nc             *nats.Conn
	js             nats.JetStreamContext
	mu             sync.Mutex
	failures       int
	suspendedUntil time.Time
}

func NewNATSRuntimeEventPublisher(natsURL string) (*NATSRuntimeEventPublisher, error) {
	nc, js, err := newNATSRuntimeEventJetStream(natsURL, "bktrader-runtime-event-publisher")
	if err != nil {
		return nil, err
	}
	publisher := &NATSRuntimeEventPublisher{nc: nc, js: js}
	if err := publisher.EnsureRuntimeEventStream(); err != nil {
		nc.Close()
		return nil, err
	}
	return publisher, nil
}

func newNATSRuntimeEventJetStream(natsURL, name string) (*nats.Conn, nats.JetStreamContext, error) {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		return nil, nil, errors.New("nats url is required")
	}
	nc, err := nats.Connect(
		natsURL,
		nats.Name(name),
		nats.Timeout(runtimeEventNATSConnectTimeout),
		nats.MaxReconnects(runtimeEventNATSMaxReconnects),
	)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	return nc, js, nil
}

func RuntimeEventStreamConfig() *nats.StreamConfig {
	return &nats.StreamConfig{
		Name:      RuntimeEventStreamName,
		Subjects:  []string{RuntimeEventSubjectPattern},
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		MaxAge:    7 * 24 * time.Hour,
	}
}

func (p *NATSRuntimeEventPublisher) EnsureRuntimeEventStream() error {
	cfg := RuntimeEventStreamConfig()
	if _, err := p.js.StreamInfo(cfg.Name); err == nil {
		_, err = p.js.UpdateStream(cfg)
		return err
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		return err
	}
	_, err := p.js.AddStream(cfg)
	return err
}

func (p *NATSRuntimeEventPublisher) PublishRuntimeEvent(ctx context.Context, event RuntimeEventEnvelope) error {
	if p == nil || p.js == nil {
		return errors.New("nats runtime event publisher is not initialized")
	}
	if err := p.checkCircuit(ctx); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		p.recordPublishFailure()
		return err
	}
	msg := &nats.Msg{
		Subject: event.Subject,
		Header:  nats.Header{},
		Data:    data,
	}
	msg.Header.Set(nats.MsgIdHdr, event.ID)
	_, err = p.js.PublishMsg(msg, nats.Context(ctx), nats.MsgId(event.ID))
	if err != nil {
		p.recordPublishFailure()
		return err
	}
	p.recordPublishSuccess()
	return nil
}

func (p *NATSRuntimeEventPublisher) Close() {
	if p == nil || p.nc == nil {
		return
	}
	p.nc.Close()
}

func (p *NATSRuntimeEventPublisher) checkCircuit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.suspendedUntil.IsZero() || time.Now().UTC().After(p.suspendedUntil) {
		return nil
	}
	return errors.New("runtime event publisher circuit open")
}

func (p *NATSRuntimeEventPublisher) recordPublishFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures++
	if p.failures >= runtimeEventCircuitThreshold {
		p.suspendedUntil = time.Now().UTC().Add(runtimeEventCircuitOpenFor)
	}
}

func (p *NATSRuntimeEventPublisher) recordPublishSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = 0
	p.suspendedUntil = time.Time{}
}
