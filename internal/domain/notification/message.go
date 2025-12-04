package notification

import "time"

// Message represents the notification payload sent through the message queue.
// It contains the rendered content ready for delivery.
type Message struct {
	ID              string
	Type            Type
	Recipient       Recipient
	RenderedContent RenderedContent
	Metadata        map[string]string
	CreatedAt       time.Time
	RetryCount      int
}

// RenderedContent holds the processed template content ready for delivery.
// Fields are used differently depending on notification type:
//   - Email: Subject (subject line), Body (HTML content)
//   - SMS: Body (text message), Subject and Data ignored
//   - Push: Subject (title), Body (body text), Data (extra payload)
type RenderedContent struct {
	Subject string
	Body    string
	Data    map[string]string
}

// NewMessage creates a new Message from a Notification with rendered content.
func NewMessage(n *Notification, content RenderedContent, metadata map[string]string) *Message {
	return &Message{
		ID:              n.ID,
		Type:            n.Type,
		Recipient:       n.Recipient,
		RenderedContent: content,
		Metadata:        metadata,
		CreatedAt:       n.CreatedAt,
		RetryCount:      0,
	}
}

// IncrementRetry increases the retry count.
func (m *Message) IncrementRetry() {
	m.RetryCount++
}

// CanRetry checks if the message can be retried based on max retries.
func (m *Message) CanRetry(maxRetries int) bool {
	return m.RetryCount < maxRetries
}
