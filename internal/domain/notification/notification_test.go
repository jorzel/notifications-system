package notification

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	recipient := NewEmailRecipient("anna.kowalska@example.com", "user_789")

	n := New(TypeEmail, PriorityTransactional, recipient, "welcome", map[string]any{"name": "Anna"})

	require.NotEmpty(t, n.ID)
	require.Equal(t, TypeEmail, n.Type)
	require.Equal(t, PriorityTransactional, n.Priority)
	require.Equal(t, recipient, n.Recipient)
	require.Equal(t, "welcome", n.TemplateID)
	require.Equal(t, StatusPending, n.Status)
	require.False(t, n.CreatedAt.IsZero())
	require.Nil(t, n.SentAt)
	require.Nil(t, n.Error)
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
