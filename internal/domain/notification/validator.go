package notification

// Validator defines the interface for validating notifications.
type Validator interface {
	// Validate checks if the notification is valid for sending.
	// Returns an error if validation fails.
	Validate(n *Notification) error
}
