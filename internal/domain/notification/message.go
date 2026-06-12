package notification

import "time"

// Message represents the notification payload sent through the message queue.
// It carries the template reference, not rendered content: rendering happens
// at the worker so messages stay small under burst, the API hot path stays
// cheap, and template fixes apply to in-flight messages.
type Message struct {
	ID           string
	Type         Type
	Priority     Priority
	Recipient    Recipient
	TemplateID   string
	TemplateData map[string]any
	Metadata     map[string]string
	CreatedAt    time.Time
	RetryCount   int
}

// RenderedContent holds the processed template content ready for delivery.
// It is produced by Template.Render at the worker and never travels over
// the queue. Fields are used differently depending on notification type:
//   - Email: Subject (subject line), Body (HTML content)
//   - SMS: Body (text message), Subject and Data ignored
//   - Push: Subject (title), Body (body text), Data (extra payload)
type RenderedContent struct {
	Subject string
	Body    string
	Data    map[string]string
}

// NewMessage creates a new Message from a Notification.
func NewMessage(n *Notification, metadata map[string]string) *Message {
	return &Message{
		ID:           n.ID,
		Type:         n.Type,
		Priority:     n.Priority,
		Recipient:    n.Recipient,
		TemplateID:   n.TemplateID,
		TemplateData: n.TemplateData,
		Metadata:     metadata,
		CreatedAt:    n.CreatedAt,
		RetryCount:   0,
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
