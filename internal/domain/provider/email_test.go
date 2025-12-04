package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmailAddress_Valid(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{
			name:  "simple email",
			email: "user@example.com",
		},
		{
			name:  "email with subdomain",
			email: "user@mail.example.com",
		},
		{
			name:  "email with plus sign",
			email: "user+tag@example.com",
		},
		{
			name:  "email with dots in local part",
			email: "first.last@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := NewEmailAddress(tt.email)

			require.NoError(t, err)
			assert.Equal(t, tt.email, addr.String())
		})
	}
}

func TestNewEmailAddress_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{
			name:  "empty email",
			email: "",
		},
		{
			name:  "missing at sign",
			email: "userexample.com",
		},
		{
			name:  "missing domain",
			email: "user@",
		},
		{
			name:  "missing local part",
			email: "@example.com",
		},
		{
			name:  "multiple at signs",
			email: "user@@example.com",
		},
		{
			name:  "spaces in email",
			email: "user @example.com",
		},
		{
			name:  "missing TLD",
			email: "user@example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := NewEmailAddress(tt.email)

			assert.ErrorIs(t, err, ErrInvalidEmail)
			assert.Empty(t, addr.String())
		})
	}
}

func TestNewEmailMessage_Valid(t *testing.T) {
	validEmail, _ := NewEmailAddress("user@example.com")

	tests := []struct {
		name    string
		to      EmailAddress
		subject string
		body    string
	}{
		{
			name:    "message with subject and body",
			to:      validEmail,
			subject: "Welcome",
			body:    "<h1>Hello</h1>",
		},
		{
			name:    "message without subject",
			to:      validEmail,
			subject: "",
			body:    "Plain text body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewEmailMessage(tt.to, tt.subject, tt.body)

			require.NoError(t, err)
			assert.Equal(t, tt.to, msg.To())
			assert.Equal(t, tt.subject, msg.Subject())
			assert.Equal(t, tt.body, msg.Body())
		})
	}
}

func TestNewEmailMessage_Invalid(t *testing.T) {
	validEmail, _ := NewEmailAddress("user@example.com")

	tests := []struct {
		name    string
		to      EmailAddress
		subject string
		body    string
		wantErr error
	}{
		{
			name:    "empty body",
			to:      validEmail,
			subject: "Subject",
			body:    "",
			wantErr: ErrEmptyBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEmailMessage(tt.to, tt.subject, tt.body)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
