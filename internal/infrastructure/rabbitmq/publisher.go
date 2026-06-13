package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// DefaultPublishTimeout bounds a single publish, including waiting for
// the channel lock and the broker confirm. It is a failure-mode ceiling,
// not the expected latency — a healthy confirm returns in well under a
// millisecond.
const DefaultPublishTimeout = 5 * time.Second

// Publisher publishes notifications to RabbitMQ with publisher confirms:
// Publish returns only after the broker has acknowledged the message, so
// a notification is never silently lost between API and broker.
//
// An AMQP channel is not safe for concurrent use, so publishes are
// serialized with a mutex. One confirming channel handles thousands of
// publishes per second; a channel pool is the scaling step if the API's
// publish rate ever outgrows it.
type Publisher struct {
	ch      *amqp.Channel
	timeout time.Duration
	mu      sync.Mutex
}

var _ notification.Publisher = (*Publisher)(nil)

// Option configures a Publisher.
type Option func(*Publisher)

// WithPublishTimeout overrides DefaultPublishTimeout.
func WithPublishTimeout(d time.Duration) Option {
	return func(p *Publisher) { p.timeout = d }
}

// NewPublisher puts the connection's channel into confirm mode and
// idempotently declares the topology.
func NewPublisher(conn *Connection, opts ...Option) (*Publisher, error) {
	ch := conn.Channel()
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("put channel in confirm mode: %w", err)
	}
	if err := DeclareTopology(ch); err != nil {
		return nil, err
	}

	p := &Publisher{ch: ch, timeout: DefaultPublishTimeout}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

// Publish sends the message to its lane's routing key and waits for the
// broker confirm. A nack, a missing confirm, or a timeout is reported as
// an error.
func (p *Publisher) Publish(ctx context.Context, msg *notification.Message) error {
	return p.publish(ctx, RoutingKey(msg.Type, msg.Priority), msg)
}

// PublishDeadLetter parks a message in the dead-letter queue. The consumer
// calls it for notifications that have exhausted their retries.
func (p *Publisher) PublishDeadLetter(ctx context.Context, msg *notification.Message) error {
	return p.publish(ctx, DeadLetterRoutingKey, msg)
}

func (p *Publisher) publish(ctx context.Context, routingKey string, msg *notification.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal notification %s: %w", msg.ID, err)
	}

	// Bound the whole operation, including the lock wait, so a hung broker
	// can't stall the publish path: once the budget passes, queued publishes
	// fail fast instead of piling up behind the mutex.
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(
		ctx,
		Exchange,
		routingKey,
		false, // mandatory: routing is guaranteed by validated type+priority + declared topology
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    msg.ID,
			Timestamp:    msg.CreatedAt,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish notification %s: %w", msg.ID, err)
	}

	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("await confirm for notification %s: %w", msg.ID, err)
	}
	if !acked {
		return fmt.Errorf("%w: notification %s nacked by broker", notification.ErrPublishFailed, msg.ID)
	}
	return nil
}
