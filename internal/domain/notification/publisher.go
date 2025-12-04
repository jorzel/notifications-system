package notification

import "context"

// Publisher defines the interface for publishing notifications to a message queue.
type Publisher interface {
	// Publish sends a notification message to the appropriate queue
	// based on the message type.
	Publish(ctx context.Context, msg *Message) error
}
