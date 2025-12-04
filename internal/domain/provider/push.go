package provider

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidDeviceToken = errors.New("invalid device token")
	ErrEmptyPushBody      = errors.New("push notification body cannot be empty")
)

// Minimum length for device tokens (FCM and APNs tokens are typically 100+ chars)
const minDeviceTokenLength = 10

// DeviceToken represents a validated device token for push notifications.
type DeviceToken struct {
	value string
}

// NewDeviceToken creates a validated DeviceToken.
func NewDeviceToken(token string) (DeviceToken, error) {
	if err := validateDeviceToken(token); err != nil {
		return DeviceToken{}, err
	}
	return DeviceToken{value: token}, nil
}

// String returns the device token string.
func (d DeviceToken) String() string {
	return d.value
}

// validateDeviceToken checks if the device token is valid.
func validateDeviceToken(token string) error {
	// Trim whitespace and check if empty
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ErrInvalidDeviceToken
	}

	// Check minimum length
	if len(trimmed) < minDeviceTokenLength {
		return ErrInvalidDeviceToken
	}

	return nil
}

// PushMessage represents a validated push notification ready for sending.
type PushMessage struct {
	to    DeviceToken
	title string
	body  string
	data  map[string]string
}

// NewPushMessage creates a validated PushMessage.
func NewPushMessage(to DeviceToken, title, body string, data map[string]string) (PushMessage, error) {
	if body == "" {
		return PushMessage{}, ErrEmptyPushBody
	}
	return PushMessage{
		to:    to,
		title: title,
		body:  body,
		data:  data,
	}, nil
}

// To returns the recipient device token.
func (m PushMessage) To() DeviceToken {
	return m.to
}

// Title returns the push notification title.
func (m PushMessage) Title() string {
	return m.title
}

// Body returns the push notification body.
func (m PushMessage) Body() string {
	return m.body
}

// Data returns the additional payload data.
func (m PushMessage) Data() map[string]string {
	return m.data
}

// PushSender defines the interface for sending push notifications.
type PushSender interface {
	Send(ctx context.Context, message PushMessage) error
}
