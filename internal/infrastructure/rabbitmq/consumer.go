package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Delivery adapts an AMQP delivery to the queue transport's Delivery
// contract (Body/Ack/Nack), so the consumer stays broker-agnostic.
type Delivery struct {
	raw amqp.Delivery
}

// Body returns the raw message payload.
func (d Delivery) Body() []byte { return d.raw.Body }

// Ack acknowledges the delivery.
func (d Delivery) Ack() error { return d.raw.Ack(false) }

// Nack negatively acknowledges the delivery, optionally requeuing it.
func (d Delivery) Nack(requeue bool) error { return d.raw.Nack(false, requeue) }

// Consume registers a consumer on the queue and streams deliveries.
// prefetch bounds unacknowledged messages in flight. Deliveries require
// manual ack: the caller acks or nacks each one. The returned channel
// closes when the underlying connection/channel closes.
func (c *Connection) Consume(queue string, prefetch int) (<-chan Delivery, error) {
	if err := c.ch.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("set qos: %w", err)
	}

	raw, err := c.ch.Consume(queue, "", false /* autoAck */, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("consume %q: %w", queue, err)
	}

	out := make(chan Delivery)
	go func() {
		defer close(out)
		for d := range raw {
			out <- Delivery{raw: d}
		}
	}()
	return out, nil
}
