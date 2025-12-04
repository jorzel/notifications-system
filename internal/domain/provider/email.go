package provider

import (
	"context"
	"errors"
)

var (
	ErrInvalidEmail = errors.New("invalid email address")
	ErrEmptyBody    = errors.New("email body cannot be empty")
)

// EmailAddress represents a validated email address.
type EmailAddress struct {
	value string
}

// NewEmailAddress creates a validated EmailAddress.
func NewEmailAddress(email string) (EmailAddress, error) {
	if err := validateEmail(email); err != nil {
		return EmailAddress{}, err
	}
	return EmailAddress{value: email}, nil
}

// String returns the email address string.
func (e EmailAddress) String() string {
	return e.value
}

// validateEmail checks if the email format is valid.
// TODO: Implement in Phase 3
func validateEmail(email string) error {
	if email == "" {
		return ErrInvalidEmail
	}
	return nil
}

// EmailMessage represents a validated email ready for sending.
type EmailMessage struct {
	to      EmailAddress
	subject string
	body    string
}

// NewEmailMessage creates a validated EmailMessage.
func NewEmailMessage(to EmailAddress, subject, body string) (EmailMessage, error) {
	if body == "" {
		return EmailMessage{}, ErrEmptyBody
	}
	return EmailMessage{
		to:      to,
		subject: subject,
		body:    body,
	}, nil
}

// To returns the recipient email address.
func (m EmailMessage) To() EmailAddress {
	return m.to
}

// Subject returns the email subject.
func (m EmailMessage) Subject() string {
	return m.subject
}

// Body returns the email body.
func (m EmailMessage) Body() string {
	return m.body
}

// EmailSender defines the interface for sending email notifications.
type EmailSender interface {
	Send(ctx context.Context, message EmailMessage) error
}
