// Package rabbitmq is the RabbitMQ adapter for the notification.Publisher
// port: connection management, the channel×priority topology, and a
// confirming publisher.
package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// Exchange is the durable direct exchange all notifications are published
// to. Routing keys are "<type>.<priority>", bound 1:1 to a queue per lane
// so bulk traffic can never delay transactional traffic.
const Exchange = "notifications"

// allTypes and allPriorities enumerate the lanes the topology declares.
var (
	allTypes      = []notification.Type{notification.TypeEmail, notification.TypeSMS, notification.TypePush}
	allPriorities = []notification.Priority{notification.PriorityTransactional, notification.PriorityBulk}
)

// RoutingKey returns the direct-exchange routing key for a notification
// lane: "<type>.<priority>", e.g. "email.transactional".
func RoutingKey(notifType notification.Type, priority notification.Priority) string {
	return string(notifType) + "." + string(priority)
}

// QueueName returns the durable queue serving a lane. For the direct
// exchange the queue name equals its binding/routing key.
func QueueName(notifType notification.Type, priority notification.Priority) string {
	return RoutingKey(notifType, priority)
}

// DeclareTopology idempotently declares the exchange and every
// type×priority queue with its binding. Safe to call from both the
// publisher and the consumer; RabbitMQ declarations are idempotent.
func DeclareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %q: %w", Exchange, err)
	}

	for _, notifType := range allTypes {
		for _, priority := range allPriorities {
			name := QueueName(notifType, priority)
			if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
				return fmt.Errorf("declare queue %q: %w", name, err)
			}
			if err := ch.QueueBind(name, RoutingKey(notifType, priority), Exchange, false, nil); err != nil {
				return fmt.Errorf("bind queue %q: %w", name, err)
			}
		}
	}
	return nil
}
