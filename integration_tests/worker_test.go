//go:build integration

// Package integration_tests exercises the worker delivery path end to end
// against a real RabbitMQ broker: publish → consume → process, including
// the retry / dead-letter loop.
package integration_tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	"github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/infrastructure/rabbitmq"
	"github.com/jorzel/notifications-system/internal/transport/queue"
)

func startBroker(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := tcrabbitmq.Run(ctx, "rabbitmq:3.13-management-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	url, err := container.AmqpURL(ctx)
	require.NoError(t, err)
	return url
}

type recordingProcessor struct {
	mu       sync.Mutex
	received []notification.Message
	err      error
}

func (p *recordingProcessor) Process(_ context.Context, msg *notification.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, *msg)
	return p.err
}

func (p *recordingProcessor) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.received)
}

// runWorker wires a real broker consumer for a lane and runs it until the
// returned cancel is called.
func runWorker(t *testing.T, url string, notifType notification.Type, priority notification.Priority, processor queue.Processor, maxRetries int) context.CancelFunc {
	t.Helper()

	consumerConn, err := rabbitmq.Dial(url)
	require.NoError(t, err)
	publisherConn, err := rabbitmq.Dial(url)
	require.NoError(t, err)
	publisher, err := rabbitmq.NewPublisher(publisherConn)
	require.NoError(t, err)

	raw, err := consumerConn.Consume(rabbitmq.QueueName(notifType, priority), 1)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	deliveries := make(chan queue.Delivery)
	go func() {
		defer close(deliveries)
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-raw:
				if !ok {
					return
				}
				deliveries <- d
			}
		}
	}()

	go func() { _ = queue.NewConsumer(deliveries, processor, publisher, maxRetries).Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		_ = consumerConn.Close()
		_ = publisherConn.Close()
	})
	return cancel
}

func publish(t *testing.T, url string, msg *notification.Message) {
	t.Helper()
	conn, err := rabbitmq.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	publisher, err := rabbitmq.NewPublisher(conn)
	require.NoError(t, err)
	require.NoError(t, publisher.Publish(context.Background(), msg))
}

func TestWorkerDeliversMessage(t *testing.T) {
	url := startBroker(t)

	processor := &recordingProcessor{}
	runWorker(t, url, notification.TypeEmail, notification.PriorityTransactional, processor, 3)

	n, err := notification.New(
		notification.TypeEmail, notification.PriorityTransactional,
		notification.NewEmailRecipient("anna.kowalska@example.com", "user_789"), "welcome", nil,
	)
	require.NoError(t, err)
	publish(t, url, notification.NewMessage(n, nil))

	require.Eventually(t, func() bool { return processor.count() == 1 }, 10*time.Second, 100*time.Millisecond,
		"the worker should consume and process the published message")
	require.Equal(t, n.ID, processor.received[0].ID)
}

func TestWorkerDeadLettersAfterRetries(t *testing.T) {
	url := startBroker(t)

	// Always fails with a transient error so the message is retried then
	// parked in the dead-letter queue.
	processor := &recordingProcessor{err: errTransient}
	maxRetries := 2
	runWorker(t, url, notification.TypeSMS, notification.PriorityBulk, processor, maxRetries)

	// A separate consumer on the dead-letter queue to observe the result.
	dlqConn, err := rabbitmq.Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = dlqConn.Close() })
	dlq, err := dlqConn.Consume(rabbitmq.DeadLetterQueue, 1)
	require.NoError(t, err)

	n, err := notification.New(
		notification.TypeSMS, notification.PriorityBulk,
		notification.NewSMSRecipient("+48601234567", "user_789"), "otp", nil,
	)
	require.NoError(t, err)
	publish(t, url, notification.NewMessage(n, nil))

	select {
	case d := <-dlq:
		require.Contains(t, string(d.Body()), n.ID, "exhausted message must land in the dead-letter queue")
	case <-time.After(15 * time.Second):
		t.Fatal("message was not dead-lettered")
	}

	// Initial attempt + maxRetries attempts were all processed.
	require.GreaterOrEqual(t, processor.count(), maxRetries+1)
}

type transientError struct{}

func (transientError) Error() string { return "transient: provider unavailable" }

var errTransient = transientError{}
