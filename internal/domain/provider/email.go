package provider

import (
	"context"
	"errors"
	"net/mail"
	"strings"
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
func validateEmail(email string) error {
	if email == "" {
		return ErrInvalidEmail
	}

	// Check for spaces
	if strings.Contains(email, " ") {
		return ErrInvalidEmail
	}

	// Parse using net/mail for RFC 5322 compliance
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return ErrInvalidEmail
	}

	// Extract the actual address (removes any display name)
	email = addr.Address

	// Split into local and domain parts
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ErrInvalidEmail
	}

	local, domain := parts[0], parts[1]

	// Validate local part is not empty
	if local == "" {
		return ErrInvalidEmail
	}

	// Validate domain has at least one dot (TLD requirement)
	if !strings.Contains(domain, ".") {
		return ErrInvalidEmail
	}

	// Validate domain is not empty and doesn't end with a dot
	if domain == "" || strings.HasSuffix(domain, ".") {
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
