package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// --- fakes -----------------------------------------------------------------

type fakeDelivery struct {
	body          []byte
	acked         bool
	nackedRequeue *bool
}

func (d *fakeDelivery) Body() []byte { return d.body }
func (d *fakeDelivery) Ack() error   { d.acked = true; return nil }
func (d *fakeDelivery) Nack(requeue bool) error {
	d.nackedRequeue = &requeue
	return nil
}

type fakeProcessor struct{ err error }

func (p fakeProcessor) Process(context.Context, *notification.Message) error { return p.err }

type fakePublisher struct {
	published     []notification.Message
	deadLettered  []notification.Message
	publishErr    error
	deadLetterErr error
}

func (p *fakePublisher) Publish(_ context.Context, msg *notification.Message) error {
	if p.publishErr != nil {
		return p.publishErr
	}
	p.published = append(p.published, *msg)
	return nil
}

func (p *fakePublisher) PublishDeadLetter(_ context.Context, msg *notification.Message) error {
	if p.deadLetterErr != nil {
		return p.deadLetterErr
	}
	p.deadLettered = append(p.deadLettered, *msg)
	return nil
}

func deliveryFor(t *testing.T, msg *notification.Message) *fakeDelivery {
	t.Helper()
	body, err := json.Marshal(msg)
	require.NoError(t, err)
	return &fakeDelivery{body: body}
}

func runConsumer(t *testing.T, proc Processor, pub Publisher, maxRetries int, deliveries ...Delivery) {
	t.Helper()
	ch := make(chan Delivery, len(deliveries))
	for _, d := range deliveries {
		ch <- d
	}
	close(ch)
	require.NoError(t, NewConsumer(ch, proc, pub, maxRetries).Run(context.Background()))
}

func sampleMessage() *notification.Message {
	return &notification.Message{
		ID:         "notif_1",
		Type:       notification.TypeEmail,
		Priority:   notification.PriorityTransactional,
		Recipient:  notification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		TemplateID: "welcome",
	}
}

// --- tests -----------------------------------------------------------------

func TestConsumerAcksOnSuccess(t *testing.T) {
	d := deliveryFor(t, sampleMessage())
	pub := &fakePublisher{}

	runConsumer(t, fakeProcessor{err: nil}, pub, 3, d)

	require.True(t, d.acked)
	require.Nil(t, d.nackedRequeue)
	require.Empty(t, pub.published)
	require.Empty(t, pub.deadLettered)
}

func TestConsumerDropsPermanentError(t *testing.T) {
	d := deliveryFor(t, sampleMessage())
	pub := &fakePublisher{}
	permanent := &appnotification.PermanentError{Err: errors.New("invalid recipient")}

	runConsumer(t, fakeProcessor{err: permanent}, pub, 3, d)

	require.True(t, d.acked, "permanent errors are dropped, not retried")
	require.Empty(t, pub.published)
	require.Empty(t, pub.deadLettered)
}

func TestConsumerRetriesTransientError(t *testing.T) {
	d := deliveryFor(t, sampleMessage())
	pub := &fakePublisher{}

	runConsumer(t, fakeProcessor{err: errors.New("smtp timeout")}, pub, 3, d)

	require.True(t, d.acked)
	require.Len(t, pub.published, 1, "transient error re-enqueues for retry")
	require.Equal(t, 1, pub.published[0].RetryCount, "retry count is incremented")
	require.Empty(t, pub.deadLettered)
}

func TestConsumerDeadLettersWhenRetriesExhausted(t *testing.T) {
	msg := sampleMessage()
	msg.RetryCount = 3 // already at the limit
	d := deliveryFor(t, msg)
	pub := &fakePublisher{}

	runConsumer(t, fakeProcessor{err: errors.New("smtp timeout")}, pub, 3, d)

	require.True(t, d.acked)
	require.Empty(t, pub.published, "no further retries once exhausted")
	require.Len(t, pub.deadLettered, 1, "exhausted message is parked in the dead-letter queue")
	require.Equal(t, "notif_1", pub.deadLettered[0].ID)
}

func TestConsumerNacksWhenRequeueFails(t *testing.T) {
	d := deliveryFor(t, sampleMessage())
	pub := &fakePublisher{publishErr: errors.New("broker unreachable")}

	runConsumer(t, fakeProcessor{err: errors.New("smtp timeout")}, pub, 3, d)

	require.False(t, d.acked, "must not ack when requeue failed — broker should redeliver")
	require.NotNil(t, d.nackedRequeue)
	require.True(t, *d.nackedRequeue)
}

func TestConsumerDropsUnparseableMessage(t *testing.T) {
	d := &fakeDelivery{body: []byte(`{not valid json`)}
	pub := &fakePublisher{}

	runConsumer(t, fakeProcessor{err: nil}, pub, 3, d)

	require.True(t, d.acked)
	require.Empty(t, pub.published)
	require.Empty(t, pub.deadLettered)
}
