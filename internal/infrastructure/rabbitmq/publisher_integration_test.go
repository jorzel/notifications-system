//go:build integration

package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

func startBroker(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := tcrabbitmq.Run(ctx, "rabbitmq:3.13-management-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	amqpURL, err := container.AmqpURL(ctx)
	require.NoError(t, err)
	return amqpURL
}

func TestPublisherRoutesToLaneQueue(t *testing.T) {
	url := startBroker(t)

	conn, err := Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	publisher, err := NewPublisher(conn)
	require.NoError(t, err)

	// Independent connection for consuming.
	consumerConn, err := Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = consumerConn.Close() })

	n, err := notification.New(
		notification.TypeEmail,
		notification.PriorityBulk,
		notification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		"welcome",
		map[string]any{"name": "Anna"},
	)
	require.NoError(t, err)
	msg := notification.NewMessage(n, map[string]string{"correlation_id": "campaign_456"})

	require.NoError(t, publisher.Publish(context.Background(), msg))

	// The message must arrive on email.bulk, intact after the round-trip.
	bulk := consume(t, consumerConn, QueueName(notification.TypeEmail, notification.PriorityBulk))
	select {
	case delivery := <-bulk:
		var got notification.Message
		require.NoError(t, json.Unmarshal(delivery.Body, &got))
		require.Equal(t, msg.ID, got.ID)
		require.Equal(t, notification.TypeEmail, got.Type)
		require.Equal(t, notification.PriorityBulk, got.Priority)
		require.Equal(t, "welcome", got.TemplateID)
		require.Equal(t, "Anna", got.TemplateData["name"])
		require.Equal(t, "application/json", delivery.ContentType)
	case <-time.After(5 * time.Second):
		t.Fatal("message did not arrive on email.bulk")
	}
}

func TestPublisherIsolatesPriorityLanes(t *testing.T) {
	url := startBroker(t)

	conn, err := Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	publisher, err := NewPublisher(conn)
	require.NoError(t, err)

	consumerConn, err := Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = consumerConn.Close() })

	// A bulk send must never appear in the transactional lane — the core
	// design claim that campaigns can't delay OTPs/password resets.
	n, err := notification.New(
		notification.TypeEmail,
		notification.PriorityBulk,
		notification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		"campaign_spring",
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, publisher.Publish(context.Background(), notification.NewMessage(n, nil)))

	transactional := consume(t, consumerConn, QueueName(notification.TypeEmail, notification.PriorityTransactional))
	select {
	case delivery := <-transactional:
		t.Fatalf("bulk message leaked into the transactional lane: %s", delivery.Body)
	case <-time.After(time.Second):
		// expected: the transactional lane stays empty
	}
}

func consume(t *testing.T, conn *Connection, queue string) <-chan amqp.Delivery {
	t.Helper()
	deliveries, err := conn.Channel().Consume(queue, "", true, false, false, false, nil)
	require.NoError(t, err)
	return deliveries
}
