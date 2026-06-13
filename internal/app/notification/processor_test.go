package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/provider"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

// --- fakes -----------------------------------------------------------------

type fakeEmailSender struct {
	sent []provider.EmailMessage
	err  error
}

func (s *fakeEmailSender) Send(_ context.Context, m provider.EmailMessage) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, m)
	return nil
}

type fakeSMSSender struct {
	sent []provider.SMSMessage
	err  error
}

func (s *fakeSMSSender) Send(_ context.Context, m provider.SMSMessage) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, m)
	return nil
}

type fakePushSender struct {
	sent []provider.PushMessage
	err  error
}

func (s *fakePushSender) Send(_ context.Context, m provider.PushMessage) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, m)
	return nil
}

func processorRenderer() *Renderer {
	return NewRenderer(apptemplate.NewService(&fakeTemplateRepository{
		templates: []*domaintemplate.Template{
			domaintemplate.New("welcome", domainnotification.TypeEmail, "Welcome Email", "Welcome, {{.name}}!", "<h1>Hello, {{.name}}!</h1>", false),
			domaintemplate.New("otp", domainnotification.TypeSMS, "OTP", "", "Your code is {{.code}}.", false),
			domaintemplate.New("alert", domainnotification.TypePush, "Alert", "{{.title}}", "{{.body}}", false),
		},
	}))
}

func noLimit() *rate.Limiter { return rate.NewLimiter(rate.Inf, 1) }

// --- email -----------------------------------------------------------------

func TestEmailProcessorSuccess(t *testing.T) {
	sender := &fakeEmailSender{}
	p := NewEmailProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		ID:           "notif_1",
		Type:         domainnotification.TypeEmail,
		Priority:     domainnotification.PriorityTransactional,
		Recipient:    domainnotification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		TemplateID:   "welcome",
		TemplateData: map[string]any{"name": "Anna"},
	}

	err := p.Process(context.Background(), msg)

	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	require.Equal(t, "anna.kowalska@example.com", sender.sent[0].To().String())
	require.Equal(t, "Welcome, Anna!", sender.sent[0].Subject())
	require.Contains(t, sender.sent[0].Body(), "<h1>Hello, Anna!</h1>")
}

func TestEmailProcessorPermanentFailure(t *testing.T) {
	tests := []struct {
		name string
		msg  *domainnotification.Message
	}{
		{
			name: "invalid recipient address",
			msg: &domainnotification.Message{
				Type: domainnotification.TypeEmail, Recipient: domainnotification.Recipient{Email: "not-an-email"}, TemplateID: "welcome",
			},
		},
		{
			name: "unknown template",
			msg: &domainnotification.Message{
				Type: domainnotification.TypeEmail, Recipient: domainnotification.NewEmailRecipient("anna.kowalska@example.com", "u"), TemplateID: "missing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &fakeEmailSender{}
			p := NewEmailProcessor(processorRenderer(), sender, noLimit())

			err := p.Process(context.Background(), tt.msg)

			var perm *PermanentError
			require.ErrorAs(t, err, &perm, "must be permanent so the worker drops it instead of retrying")
			require.Empty(t, sender.sent)
		})
	}
}

func TestEmailProcessorTransientFailure(t *testing.T) {
	sender := &fakeEmailSender{err: errors.New("smtp: connection reset")}
	p := NewEmailProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypeEmail, Recipient: domainnotification.NewEmailRecipient("anna.kowalska@example.com", "u"),
		TemplateID: "welcome", TemplateData: map[string]any{"name": "Anna"},
	}

	err := p.Process(context.Background(), msg)

	require.Error(t, err)
	var perm *PermanentError
	require.False(t, errors.As(err, &perm), "a send failure is transient and must be retried")
}

// --- sms -------------------------------------------------------------------

func TestSMSProcessorSuccess(t *testing.T) {
	sender := &fakeSMSSender{}
	p := NewSMSProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypeSMS, Recipient: domainnotification.NewSMSRecipient("+48601234567", "u"),
		TemplateID: "otp", TemplateData: map[string]any{"code": "839201"},
	}

	err := p.Process(context.Background(), msg)

	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	require.Equal(t, "+48601234567", sender.sent[0].To().String())
	require.Equal(t, "Your code is 839201.", sender.sent[0].Text())
}

func TestSMSProcessorPermanentFailure(t *testing.T) {
	sender := &fakeSMSSender{}
	p := NewSMSProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypeSMS, Recipient: domainnotification.Recipient{PhoneNumber: "not-a-number"}, TemplateID: "otp",
	}

	err := p.Process(context.Background(), msg)

	var perm *PermanentError
	require.ErrorAs(t, err, &perm)
	require.Empty(t, sender.sent)
}

func TestSMSProcessorTransientFailure(t *testing.T) {
	sender := &fakeSMSSender{err: errors.New("twilio: 503")}
	p := NewSMSProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypeSMS, Recipient: domainnotification.NewSMSRecipient("+48601234567", "u"),
		TemplateID: "otp", TemplateData: map[string]any{"code": "839201"},
	}

	err := p.Process(context.Background(), msg)

	require.Error(t, err)
	var perm *PermanentError
	require.False(t, errors.As(err, &perm))
}

// --- push ------------------------------------------------------------------

func TestPushProcessorSuccess(t *testing.T) {
	sender := &fakePushSender{}
	p := NewPushProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypePush, Recipient: domainnotification.NewPushRecipient("fcm-token-d4c7a1b2e9f04c8a9b3d5e6f7a8b9c0d", "u"),
		TemplateID: "alert", TemplateData: map[string]any{"title": "Delivery", "body": "Your parcel arrived."},
	}

	err := p.Process(context.Background(), msg)

	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	require.Equal(t, "Delivery", sender.sent[0].Title())
	require.Equal(t, "Your parcel arrived.", sender.sent[0].Body())
}

func TestPushProcessorPermanentFailure(t *testing.T) {
	sender := &fakePushSender{}
	p := NewPushProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypePush, Recipient: domainnotification.Recipient{DeviceToken: ""}, TemplateID: "alert",
	}

	err := p.Process(context.Background(), msg)

	var perm *PermanentError
	require.ErrorAs(t, err, &perm)
	require.Empty(t, sender.sent)
}

func TestPushProcessorTransientFailure(t *testing.T) {
	sender := &fakePushSender{err: errors.New("fcm: unavailable")}
	p := NewPushProcessor(processorRenderer(), sender, noLimit())
	msg := &domainnotification.Message{
		Type: domainnotification.TypePush, Recipient: domainnotification.NewPushRecipient("fcm-token-d4c7a1b2e9f04c8a9b3d5e6f7a8b9c0d", "u"),
		TemplateID: "alert", TemplateData: map[string]any{"title": "Delivery", "body": "Your parcel arrived."},
	}

	err := p.Process(context.Background(), msg)

	require.Error(t, err)
	var perm *PermanentError
	require.False(t, errors.As(err, &perm))
}
