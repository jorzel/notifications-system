package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPhoneNumber_Valid(t *testing.T) {
	tests := []struct {
		name       string
		number     string
		normalized string
	}{
		{
			name:       "US number",
			number:     "+14155551234",
			normalized: "+14155551234",
		},
		{
			name:       "UK number",
			number:     "+442071234567",
			normalized: "+442071234567",
		},
		{
			name:       "Polish number",
			number:     "+48123456789",
			normalized: "+48123456789",
		},
		{
			name:       "number with spaces normalized",
			number:     "+1 415 555 1234",
			normalized: "+14155551234",
		},
		{
			name:       "number with dashes normalized",
			number:     "+1-415-555-1234",
			normalized: "+14155551234",
		},
		{
			name:       "number with mixed formatting",
			number:     "+1 (415) 555-1234",
			normalized: "+14155551234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phone, err := NewPhoneNumber(tt.number)

			require.NoError(t, err)
			assert.Equal(t, tt.normalized, phone.String())
		})
	}
}

func TestNewPhoneNumber_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		number string
	}{
		{
			name:   "empty number",
			number: "",
		},
		{
			name:   "missing plus sign",
			number: "14155551234",
		},
		{
			name:   "only plus sign",
			number: "+",
		},
		{
			name:   "contains letters",
			number: "+1415555ABCD",
		},
		{
			name:   "too short",
			number: "+123",
		},
		{
			name:   "too long",
			number: "+1234567890123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phone, err := NewPhoneNumber(tt.number)

			assert.ErrorIs(t, err, ErrInvalidPhoneNumber)
			assert.Empty(t, phone.String())
		})
	}
}

func TestNewSMSMessage_Valid(t *testing.T) {
	validPhone, _ := NewPhoneNumber("+14155551234")

	tests := []struct {
		name string
		to   PhoneNumber
		text string
	}{
		{
			name: "simple message",
			to:   validPhone,
			text: "Hello, this is a test message.",
		},
		{
			name: "message with unicode",
			to:   validPhone,
			text: "Hello! Cześć! 你好!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewSMSMessage(tt.to, tt.text)

			require.NoError(t, err)
			assert.Equal(t, tt.to, msg.To())
			assert.Equal(t, tt.text, msg.Text())
		})
	}
}

func TestNewSMSMessage_Invalid(t *testing.T) {
	validPhone, _ := NewPhoneNumber("+14155551234")

	tests := []struct {
		name    string
		to      PhoneNumber
		text    string
		wantErr error
	}{
		{
			name:    "empty text",
			to:      validPhone,
			text:    "",
			wantErr: ErrEmptyMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSMSMessage(tt.to, tt.text)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
