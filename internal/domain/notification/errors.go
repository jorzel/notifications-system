package notification

import "errors"

var (
	// ErrInvalidType is returned when notification type is not supported.
	ErrInvalidType = errors.New("invalid notification type")

	// ErrEmptyRecipient is returned when recipient address is empty.
	ErrEmptyRecipient = errors.New("recipient address is empty")

	// ErrPublishFailed is returned when publishing to queue fails.
	ErrPublishFailed = errors.New("failed to publish notification")
)
