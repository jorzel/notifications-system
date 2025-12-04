package provider

import (
	"context"
	"errors"
	"strings"
	"unicode"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
	ErrEmptyMessage       = errors.New("SMS message cannot be empty")
)

// E.164 format constraints
const (
	minPhoneDigits = 7  // Minimum digits (excluding +)
	maxPhoneDigits = 15 // Maximum digits (excluding +)
)

// PhoneNumber represents a validated phone number in E.164 format.
type PhoneNumber struct {
	value string
}

// NewPhoneNumber creates a validated PhoneNumber.
// The number is normalized to E.164 format (spaces, dashes, parentheses removed).
func NewPhoneNumber(number string) (PhoneNumber, error) {
	normalized, err := normalizeAndValidatePhoneNumber(number)
	if err != nil {
		return PhoneNumber{}, err
	}
	return PhoneNumber{value: normalized}, nil
}

// String returns the phone number string in E.164 format.
func (p PhoneNumber) String() string {
	return p.value
}

// normalizeAndValidatePhoneNumber normalizes and validates a phone number.
func normalizeAndValidatePhoneNumber(number string) (string, error) {
	if number == "" {
		return "", ErrInvalidPhoneNumber
	}

	// Normalize: remove spaces, dashes, parentheses
	normalized := normalizePhoneNumber(number)

	// Must start with +
	if !strings.HasPrefix(normalized, "+") {
		return "", ErrInvalidPhoneNumber
	}

	// Extract digits after +
	digits := normalized[1:]

	// Check all remaining characters are digits
	for _, r := range digits {
		if !unicode.IsDigit(r) {
			return "", ErrInvalidPhoneNumber
		}
	}

	// Validate length (E.164: 7-15 digits)
	if len(digits) < minPhoneDigits || len(digits) > maxPhoneDigits {
		return "", ErrInvalidPhoneNumber
	}

	return normalized, nil
}

// normalizePhoneNumber removes common formatting characters.
func normalizePhoneNumber(number string) string {
	var builder strings.Builder
	builder.Grow(len(number))

	for _, r := range number {
		switch r {
		case ' ', '-', '(', ')':
			// Skip formatting characters
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// SMSMessage represents a validated SMS ready for sending.
type SMSMessage struct {
	to   PhoneNumber
	text string
}

// NewSMSMessage creates a validated SMSMessage.
func NewSMSMessage(to PhoneNumber, text string) (SMSMessage, error) {
	if text == "" {
		return SMSMessage{}, ErrEmptyMessage
	}
	return SMSMessage{
		to:   to,
		text: text,
	}, nil
}

// To returns the recipient phone number.
func (m SMSMessage) To() PhoneNumber {
	return m.to
}

// Text returns the SMS text.
func (m SMSMessage) Text() string {
	return m.text
}

// SMSSender defines the interface for sending SMS notifications.
type SMSSender interface {
	Send(ctx context.Context, message SMSMessage) error
}
