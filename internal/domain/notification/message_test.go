package notification

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMessage(t *testing.T) {
	recipient := NewSMSRecipient("+48601234567", "user_789")
	templateData := map[string]any{"code": "839201"}
	n := New(TypeSMS, PriorityTransactional, recipient, "otp", templateData)
	metadata := map[string]string{"correlation_id": "login_456"}

	msg := NewMessage(n, metadata)

	require.Equal(t, n.ID, msg.ID)
	require.Equal(t, TypeSMS, msg.Type)
	require.Equal(t, PriorityTransactional, msg.Priority)
	require.Equal(t, recipient, msg.Recipient)
	require.Equal(t, "otp", msg.TemplateID)
	require.Equal(t, templateData, msg.TemplateData)
	require.Equal(t, metadata, msg.Metadata)
	require.Equal(t, n.CreatedAt, msg.CreatedAt)
	require.Zero(t, msg.RetryCount)
}

func TestMessageCanRetry(t *testing.T) {
	n := New(TypeEmail, PriorityBulk, NewEmailRecipient("anna.kowalska@example.com", "user_789"), "campaign_spring", nil)
	msg := NewMessage(n, nil)
	maxRetries := 2

	require.True(t, msg.CanRetry(maxRetries))

	msg.IncrementRetry()
	require.True(t, msg.CanRetry(maxRetries))

	msg.IncrementRetry()
	require.False(t, msg.CanRetry(maxRetries))
}
