// Package worker is the inbound transport for queue consumption: it reads
// deliveries from a lane, dispatches each to a processor, and applies the
// retry / dead-letter policy.
package queue

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/rs/zerolog"

	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// Delivery is a single queue message the consumer can acknowledge. It
// abstracts the broker delivery so the consumer logic is testable without
// a real broker.
type Delivery interface {
	Body() []byte
	Ack() error
	Nack(requeue bool) error
}

// Processor delivers a notification (render → build provider message →
// send). A *PermanentError means the message can never succeed.
type Processor interface {
	Process(ctx context.Context, msg *notification.Message) error
}

// Publisher re-enqueues a message for retry or parks it in the dead-letter
// queue when retries are exhausted.
type Publisher interface {
	Publish(ctx context.Context, msg *notification.Message) error
	PublishDeadLetter(ctx context.Context, msg *notification.Message) error
}

// Consumer drains one lane's deliveries through a processor.
type Consumer struct {
	deliveries <-chan Delivery
	processor  Processor
	publisher  Publisher
	maxRetries int
}

// NewConsumer creates a Consumer.
func NewConsumer(deliveries <-chan Delivery, processor Processor, publisher Publisher, maxRetries int) *Consumer {
	return &Consumer{
		deliveries: deliveries,
		processor:  processor,
		publisher:  publisher,
		maxRetries: maxRetries,
	}
}

// Run consumes until the context is cancelled or the delivery channel
// closes. It processes one delivery at a time; throughput scales by
// running more worker processes per lane, not in-process concurrency.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-c.deliveries:
			if !ok {
				return nil
			}
			c.handle(ctx, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d Delivery) {
	logger := zerolog.Ctx(ctx)

	var msg notification.Message
	if err := json.Unmarshal(d.Body(), &msg); err != nil {
		// Unparseable: we can't even route it to retry/dead-letter. Drop it.
		logger.Error().Err(err).Msg("dropping unparseable message")
		_ = d.Ack()
		return
	}

	err := c.processor.Process(ctx, &msg)
	switch {
	case err == nil:
		logger.Info().
			Str("notification_id", msg.ID).
			Str("type", string(msg.Type)).
			Str("priority", string(msg.Priority)).
			Msg("notification delivered")
		_ = d.Ack()

	case isPermanent(err):
		logger.Error().Err(err).Str("notification_id", msg.ID).Msg("dropping notification: permanent error")
		_ = d.Ack()

	case msg.CanRetry(c.maxRetries):
		msg.IncrementRetry()
		if rerr := c.publisher.Publish(ctx, &msg); rerr != nil {
			// Don't lose the message: leave it for broker redelivery.
			logger.Error().Err(rerr).Str("notification_id", msg.ID).Msg("requeue failed; nacking for redelivery")
			_ = d.Nack(true)
			return
		}
		logger.Warn().Err(err).Str("notification_id", msg.ID).Int("retry", msg.RetryCount).Msg("retrying notification")
		_ = d.Ack()

	default:
		if derr := c.publisher.PublishDeadLetter(ctx, &msg); derr != nil {
			logger.Error().Err(derr).Str("notification_id", msg.ID).Msg("dead-letter failed; nacking for redelivery")
			_ = d.Nack(true)
			return
		}
		logger.Error().Err(err).Str("notification_id", msg.ID).Int("retries", msg.RetryCount).Msg("notification exhausted retries; dead-lettered")
		_ = d.Ack()
	}
}

func isPermanent(err error) bool {
	var perm *appnotification.PermanentError
	return errors.As(err, &perm)
}
