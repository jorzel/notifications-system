package notification

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSuccess(t *testing.T) {
	tests := []struct {
		name      string
		notifType Type
		priority  Priority
		recipient Recipient
	}{
		{
			name:      "email notification with valid address",
			notifType: TypeEmail,
			priority:  PriorityTransactional,
			recipient: NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		},
		{
			name:      "sms notification with valid phone number",
			notifType: TypeSMS,
			priority:  PriorityTransactional,
			recipient: NewSMSRecipient("+48601234567", "user_789"),
		},
		{
			name:      "push notification with valid device token",
			notifType: TypePush,
			priority:  PriorityBulk,
			recipient: NewPushRecipient("fcm-token-d4c7a1b2e9f04c8a9b3d5e6f7a8b9c0d", "user_789"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := New(tt.notifType, tt.priority, tt.recipient, "welcome", map[string]any{"name": "Anna"})

			require.NoError(t, err)
			require.NotEmpty(t, n.ID)
			require.Equal(t, tt.notifType, n.Type)
			require.Equal(t, tt.priority, n.Priority)
			require.Equal(t, tt.recipient, n.Recipient)
			require.Equal(t, "welcome", n.TemplateID)
			require.Equal(t, StatusPending, n.Status)
			require.False(t, n.CreatedAt.IsZero())
			require.Nil(t, n.SentAt)
			require.Nil(t, n.Error)
		})
	}
}

func TestNewFailed(t *testing.T) {
	tests := []struct {
		name      string
		notifType Type
		priority  Priority
		recipient Recipient
		wantErr   error
	}{
		{
			name:      "unknown notification type",
			notifType: Type("fax"),
			priority:  PriorityTransactional,
			recipient: NewEmailRecipient("anna.kowalska@example.com", "user_789"),
			wantErr:   ErrInvalidType,
		},
		{
			name:      "unknown priority",
			notifType: TypeEmail,
			priority:  Priority("urgent"),
			recipient: NewEmailRecipient("anna.kowalska@example.com", "user_789"),
			wantErr:   ErrInvalidPriority,
		},
		{
			name:      "email notification without email address",
			notifType: TypeEmail,
			priority:  PriorityTransactional,
			recipient: Recipient{UserID: "user_789"},
			wantErr:   ErrEmptyRecipient,
		},
		{
			name:      "sms notification without phone number",
			notifType: TypeSMS,
			priority:  PriorityTransactional,
			recipient: Recipient{UserID: "user_789"},
			wantErr:   ErrEmptyRecipient,
		},
		{
			name:      "push notification without device token",
			notifType: TypePush,
			priority:  PriorityBulk,
			recipient: Recipient{UserID: "user_789"},
			wantErr:   ErrEmptyRecipient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.notifType, tt.priority, tt.recipient, "welcome", nil)

			require.ErrorIs(t, err, tt.wantErr)

			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr, "validation failures must be mappable to a 400 response")
			require.NotEmpty(t, valErr.Field)
		})
	}
}

func TestNewMalformedRecipient(t *testing.T) {
	tests := []struct {
		name      string
		notifType Type
		recipient Recipient
	}{
		{
			name:      "malformed email address",
			notifType: TypeEmail,
			recipient: NewEmailRecipient("not-an-email", "user_789"),
		},
		{
			name:      "malformed phone number",
			notifType: TypeSMS,
			recipient: NewSMSRecipient("0601-234-567 ext 9", "user_789"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.notifType, PriorityTransactional, tt.recipient, "welcome", nil)

			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr, "validation failures must be mappable to a 400 response")
		})
	}
}

func TestPriorityIsValidAllowed(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
	}{
		{name: "transactional is valid", priority: PriorityTransactional},
		{name: "bulk is valid", priority: PriorityBulk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, tt.priority.IsValid())
		})
	}
}

func TestPriorityIsValidNotAllowed(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
	}{
		{name: "empty priority is invalid", priority: Priority("")},
		{name: "unknown priority is invalid", priority: Priority("urgent")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.False(t, tt.priority.IsValid())
		})
	}
}
