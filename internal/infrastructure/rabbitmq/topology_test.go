package rabbitmq

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

func TestRoutingKey(t *testing.T) {
	tests := []struct {
		name      string
		notifType notification.Type
		priority  notification.Priority
		want      string
	}{
		{"email transactional", notification.TypeEmail, notification.PriorityTransactional, "email.transactional"},
		{"email bulk", notification.TypeEmail, notification.PriorityBulk, "email.bulk"},
		{"sms transactional", notification.TypeSMS, notification.PriorityTransactional, "sms.transactional"},
		{"sms bulk", notification.TypeSMS, notification.PriorityBulk, "sms.bulk"},
		{"push transactional", notification.TypePush, notification.PriorityTransactional, "push.transactional"},
		{"push bulk", notification.TypePush, notification.PriorityBulk, "push.bulk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, RoutingKey(tt.notifType, tt.priority))
			require.Equal(t, tt.want, QueueName(tt.notifType, tt.priority),
				"queue name and routing key coincide for the direct exchange")
		})
	}
}

func TestRoutingKeysAreDistinctPerLane(t *testing.T) {
	seen := make(map[string]bool)
	for _, notifType := range allTypes {
		for _, priority := range allPriorities {
			key := RoutingKey(notifType, priority)
			require.False(t, seen[key], "duplicate routing key %q", key)
			seen[key] = true
		}
	}
	require.Len(t, seen, 6, "expected a distinct lane per type×priority")
}
