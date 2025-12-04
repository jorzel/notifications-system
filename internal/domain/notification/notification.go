package notification

import (
	"time"

	"github.com/google/uuid"
)

// Type represents the notification channel.
type Type string

const (
	TypeEmail Type = "email"
	TypeSMS   Type = "sms"
	TypePush  Type = "push"
)

// IsValid checks if the notification type is valid.
func (t Type) IsValid() bool {
	switch t {
	case TypeEmail, TypeSMS, TypePush:
		return true
	default:
		return false
	}
}

// Status represents the current state of a notification.
type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

// Notification represents a notification request.
type Notification struct {
	ID           string
	Type         Type
	Recipient    Recipient
	TemplateID   string
	TemplateData map[string]any
	Status       Status
	CreatedAt    time.Time
	SentAt       *time.Time
	Error        *string
}

// New creates a new Notification with pending status.
func New(notifType Type, recipient Recipient, templateID string, templateData map[string]any) *Notification {
	return &Notification{
		ID:           uuid.New().String(),
		Type:         notifType,
		Recipient:    recipient,
		TemplateID:   templateID,
		TemplateData: templateData,
		Status:       StatusPending,
		CreatedAt:    time.Now().UTC(),
	}
}

// MarkSent updates the notification status to sent.
func (n *Notification) MarkSent() {
	now := time.Now().UTC()
	n.Status = StatusSent
	n.SentAt = &now
}

// MarkFailed updates the notification status to failed with an error message.
func (n *Notification) MarkFailed(err string) {
	n.Status = StatusFailed
	n.Error = &err
}
