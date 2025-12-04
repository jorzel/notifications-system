package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeviceToken_Valid(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "FCM token",
			token: "dGVzdC10b2tlbi1mb3ItZmNtLXB1c2gtbm90aWZpY2F0aW9ucw",
		},
		{
			name:  "APNs token",
			token: "740f4707bebcf74f9b7c25d48e3358945f6aa01da5ddb387462c7eaf61bb78ad",
		},
		{
			name:  "long token",
			token: "a]b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2g3h4i5j6k7l8m9n0o1p2q3r4s5t6u7v8w9x0y1z2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := NewDeviceToken(tt.token)

			require.NoError(t, err)
			assert.Equal(t, tt.token, token.String())
		})
	}
}

func TestNewDeviceToken_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "whitespace only",
			token: "   ",
		},
		{
			name:  "too short",
			token: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := NewDeviceToken(tt.token)

			assert.ErrorIs(t, err, ErrInvalidDeviceToken)
			assert.Empty(t, token.String())
		})
	}
}

func TestNewPushMessage_Valid(t *testing.T) {
	validToken, _ := NewDeviceToken("dGVzdC10b2tlbi1mb3ItZmNtLXB1c2gtbm90aWZpY2F0aW9ucw")

	tests := []struct {
		name  string
		to    DeviceToken
		title string
		body  string
		data  map[string]string
	}{
		{
			name:  "message with title and body",
			to:    validToken,
			title: "New Message",
			body:  "You have a new notification",
			data:  nil,
		},
		{
			name:  "message without title",
			to:    validToken,
			title: "",
			body:  "Silent notification body",
			data:  nil,
		},
		{
			name:  "message with data payload",
			to:    validToken,
			title: "Order Update",
			body:  "Your order has shipped",
			data:  map[string]string{"order_id": "12345", "action": "view_order"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewPushMessage(tt.to, tt.title, tt.body, tt.data)

			require.NoError(t, err)
			assert.Equal(t, tt.to, msg.To())
			assert.Equal(t, tt.title, msg.Title())
			assert.Equal(t, tt.body, msg.Body())
			assert.Equal(t, tt.data, msg.Data())
		})
	}
}

func TestNewPushMessage_Invalid(t *testing.T) {
	validToken, _ := NewDeviceToken("dGVzdC10b2tlbi1mb3ItZmNtLXB1c2gtbm90aWZpY2F0aW9ucw")

	tests := []struct {
		name    string
		to      DeviceToken
		title   string
		body    string
		data    map[string]string
		wantErr error
	}{
		{
			name:    "empty body",
			to:      validToken,
			title:   "Title",
			body:    "",
			data:    nil,
			wantErr: ErrEmptyPushBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPushMessage(tt.to, tt.title, tt.body, tt.data)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
