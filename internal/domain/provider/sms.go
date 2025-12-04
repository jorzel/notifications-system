package provider

import (
	"context"
	"errors"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
	ErrEmptyMessage       = errors.New("SMS message cannot be empty")
)

// PhoneNumber represents a validated phone number in E.164 format.
type PhoneNumber struct {
	value string
}

// NewPhoneNumber creates a validated PhoneNumber.
func NewPhoneNumber(number string) (PhoneNumber, error) {
	if err := validatePhoneNumber(number); err != nil {
		return PhoneNumber{}, err
	}
	return PhoneNumber{value: number}, nil
}

// String returns the phone number string.
func (p PhoneNumber) String() string {
	return p.value
}

// validatePhoneNumber checks if the phone number format is valid.
// TODO: Implement E.164 validation in Phase 3
func validatePhoneNumber(number string) error {
	if number == "" {
		return ErrInvalidPhoneNumber
	}
	return nil
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
