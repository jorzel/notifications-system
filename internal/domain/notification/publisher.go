package notification

import "context"

//go:generate go run go.uber.org/mock/mockgen -source=publisher.go -destination=mocks/publisher.go -package=mocks

// Publisher defines the interface for publishing notifications to a message queue.
type Publisher interface {
	// Publish sends a notification message to the appropriate queue
	// based on the message type.
	Publish(ctx context.Context, msg *Message) error
}
