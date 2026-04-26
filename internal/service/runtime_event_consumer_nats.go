package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	runtimeEventConsumerAckWait    = 30 * time.Second
	runtimeEventConsumerMaxDeliver = 5
	runtimeEventConsumerFetchBatch = 10
	runtimeEventConsumerFetchWait  = 2 * time.Second
)

type NATSRuntimeEventConsumer struct {
	nc       *nats.Conn
	js       nats.JetStreamContext
	platform *Platform
	sub      *nats.Subscription
}

func NewNATSRuntimeEventConsumer(natsURL string, platform *Platform) (*NATSRuntimeEventConsumer, error) {
	if platform == nil {
		return nil, errors.New("platform is required")
	}
	nc, js, err := newNATSRuntimeEventJetStream(natsURL, "bktrader-runtime-event-live-evaluation-consumer")
	if err != nil {
		return nil, err
	}
	consumer := &NATSRuntimeEventConsumer{
		nc:       nc,
		js:       js,
		platform: platform,
	}
	publisher := &NATSRuntimeEventPublisher{nc: nc, js: js}
	if err := publisher.EnsureRuntimeEventStream(); err != nil {
		consumer.Close()
		return nil, err
	}
	if err := consumer.EnsureLiveEvaluationConsumer(); err != nil {
		consumer.Close()
		return nil, err
	}
	return consumer, nil
}

func RuntimeEventLiveEvaluationConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		Durable:       RuntimeEventLiveEvaluationDurable,
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       runtimeEventConsumerAckWait,
		MaxDeliver:    runtimeEventConsumerMaxDeliver,
		FilterSubject: RuntimeSignalEventSubjectPattern,
	}
}

func (c *NATSRuntimeEventConsumer) EnsureLiveEvaluationConsumer() error {
	cfg := RuntimeEventLiveEvaluationConsumerConfig()
	if _, err := c.js.ConsumerInfo(RuntimeEventStreamName, cfg.Durable); err == nil {
		_, err = c.js.UpdateConsumer(RuntimeEventStreamName, cfg)
		return err
	} else if !errors.Is(err, nats.ErrConsumerNotFound) {
		return err
	}
	_, err := c.js.AddConsumer(RuntimeEventStreamName, cfg)
	return err
}

func (c *NATSRuntimeEventConsumer) Start(ctx context.Context) error {
	if c == nil || c.js == nil || c.platform == nil {
		return errors.New("runtime event consumer is not initialized")
	}
	sub, err := c.js.PullSubscribe(
		RuntimeSignalEventSubjectPattern,
		RuntimeEventLiveEvaluationDurable,
		nats.Bind(RuntimeEventStreamName, RuntimeEventLiveEvaluationDurable),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}
	c.sub = sub
	go c.run(ctx)
	return nil
}

func (c *NATSRuntimeEventConsumer) run(ctx context.Context) {
	logger := c.platform.logger("service.runtime_event_consumer",
		"stream", RuntimeEventStreamName,
		"durable", RuntimeEventLiveEvaluationDurable,
	)
	logger.Info("runtime event consumer started")
	defer logger.Info("runtime event consumer stopped")
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		msgs, err := c.sub.Fetch(runtimeEventConsumerFetchBatch, nats.MaxWait(runtimeEventConsumerFetchWait), nats.Context(ctx))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.Canceled) {
				continue
			}
			logger.Warn("runtime event consumer fetch failed", "error", err)
			continue
		}
		for _, msg := range msgs {
			c.handleNATSMessage(ctx, msg)
		}
	}
}

func (c *NATSRuntimeEventConsumer) handleNATSMessage(ctx context.Context, msg *nats.Msg) {
	var event RuntimeEventEnvelope
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.platform.logger("service.runtime_event_consumer").Warn("runtime event decode failed; leaving event unacked", "error", err)
		return
	}
	_ = c.platform.HandleRuntimeEventMessage(ctx, RuntimeEventMessage{
		Event: event,
		Ack: func() error {
			return msg.Ack()
		},
		Nak: func(error) error {
			return msg.Nak()
		},
	})
}

func (c *NATSRuntimeEventConsumer) Close() {
	if c == nil {
		return
	}
	if c.sub != nil {
		_ = c.sub.Drain()
	}
	if c.nc != nil {
		c.nc.Close()
	}
}
