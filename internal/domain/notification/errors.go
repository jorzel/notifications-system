package notification

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidType is returned when notification type is not supported.
	ErrInvalidType = errors.New("invalid notification type")

	// ErrInvalidPriority is returned when notification priority is not supported.
	ErrInvalidPriority = errors.New("invalid notification priority")

	// ErrEmptyRecipient is returned when recipient address is empty.
	ErrEmptyRecipient = errors.New("recipient address is empty")

	// ErrPublishFailed is returned when publishing to queue fails.
	ErrPublishFailed = errors.New("failed to publish notification")
)

// ValidationError marks input that can never produce a valid notification:
// unknown type or priority, or a recipient that doesn't match the channel.
// Transport layers detect it with errors.As and map it to a client error
// (HTTP 400); the wrapped sentinel stays reachable via errors.Is.
type ValidationError struct {
	// Field identifies the invalid input, e.g. "type" or "recipient.email".
	Field string
	Err   error
}

// NewValidationError creates a ValidationError for the given field.
func NewValidationError(field string, err error) *ValidationError {
	return &ValidationError{Field: field, Err: err}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %v", e.Field, e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}
