package notification

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/notification/mocks"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

// fakeTemplateRepository is an in-memory template.Repository for testing.
type fakeTemplateRepository struct {
	templates []*domaintemplate.Template
}

func (r *fakeTemplateRepository) Get(_ context.Context, notificationType domainnotification.Type, id string) (*domaintemplate.Template, error) {
	for _, tmpl := range r.templates {
		if tmpl.Type == notificationType && tmpl.ID == id {
			return tmpl, nil
		}
	}
	return nil, fmt.Errorf("%w: %s/%s", domaintemplate.ErrTemplateNotFound, notificationType, id)
}

func (r *fakeTemplateRepository) GetDefault(_ context.Context, notificationType domainnotification.Type) (*domaintemplate.Template, error) {
	for _, tmpl := range r.templates {
		if tmpl.Type == notificationType && tmpl.IsDefault {
			return tmpl, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", domaintemplate.ErrNoDefaultTemplate, notificationType)
}

func (r *fakeTemplateRepository) List(_ context.Context) ([]*domaintemplate.Template, error) {
	return r.templates, nil
}

func newTemplateService() *apptemplate.Service {
	return apptemplate.NewService(&fakeTemplateRepository{
		templates: []*domaintemplate.Template{
			domaintemplate.New("default", domainnotification.TypeEmail, "Default Email", "{{.subject}}", "<p>{{.message}}</p>", true),
			domaintemplate.New("welcome", domainnotification.TypeEmail, "Welcome Email", "Welcome, {{.name}}!", "<h1>Hello, {{.name}}!</h1>", false),
			domaintemplate.New("default", domainnotification.TypeSMS, "Default SMS", "", "{{.message}}", true),
		},
	})
}

func TestServiceSendSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)

	var published *domainnotification.Message
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msg *domainnotification.Message) error {
			published = msg
			return nil
		})

	svc := NewService(publisher, newTemplateService())
	req := &SendRequest{
		Type:         domainnotification.TypeEmail,
		Priority:     domainnotification.PriorityBulk,
		Recipient:    domainnotification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		TemplateID:   "welcome",
		TemplateData: map[string]any{"name": "Anna"},
		Metadata:     map[string]string{"correlation_id": "campaign_456"},
	}

	resp, err := svc.Send(context.Background(), req)

	require.NoError(t, err)
	require.NotEmpty(t, resp.ID)
	require.Equal(t, domainnotification.StatusPending, resp.Status)
	require.False(t, resp.CreatedAt.IsZero())

	require.NotNil(t, published)
	require.Equal(t, resp.ID, published.ID)
	require.Equal(t, domainnotification.TypeEmail, published.Type)
	require.Equal(t, domainnotification.PriorityBulk, published.Priority)
	require.Equal(t, req.Recipient, published.Recipient)
	require.Equal(t, "welcome", published.TemplateID)
	require.Equal(t, req.TemplateData, published.TemplateData)
	require.Equal(t, req.Metadata, published.Metadata)
}

func TestServiceSendDefaultsPriorityToTransactional(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)

	var published *domainnotification.Message
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msg *domainnotification.Message) error {
			published = msg
			return nil
		})

	svc := NewService(publisher, newTemplateService())

	_, err := svc.Send(context.Background(), &SendRequest{
		Type:       domainnotification.TypeEmail,
		Recipient:  domainnotification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		TemplateID: "welcome",
	})

	require.NoError(t, err)
	require.Equal(t, domainnotification.PriorityTransactional, published.Priority)
}

func TestServiceSendResolvesDefaultTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)

	var published *domainnotification.Message
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msg *domainnotification.Message) error {
			published = msg
			return nil
		})

	svc := NewService(publisher, newTemplateService())

	_, err := svc.Send(context.Background(), &SendRequest{
		Type:         domainnotification.TypeSMS,
		Recipient:    domainnotification.NewSMSRecipient("+48601234567", "user_789"),
		TemplateData: map[string]any{"message": "Your parcel arrived."},
	})

	require.NoError(t, err)
	require.Equal(t, "default", published.TemplateID,
		"the resolved template ID must be stamped on the message so the worker resolves deterministically")
}

func TestServiceSendFailed(t *testing.T) {
	tests := []struct {
		name    string
		req     *SendRequest
		wantErr error
	}{
		{
			name: "unknown notification type",
			req: &SendRequest{
				Type:      domainnotification.Type("fax"),
				Recipient: domainnotification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
			},
			wantErr: domainnotification.ErrInvalidType,
		},
		{
			name: "malformed email recipient",
			req: &SendRequest{
				Type:      domainnotification.TypeEmail,
				Recipient: domainnotification.NewEmailRecipient("not-an-email", "user_789"),
			},
		},
		{
			name: "unknown template",
			req: &SendRequest{
				Type:       domainnotification.TypeEmail,
				Recipient:  domainnotification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
				TemplateID: "order_confirmation",
			},
			wantErr: domaintemplate.ErrTemplateNotFound,
		},
		{
			name: "no default template for type",
			req: &SendRequest{
				Type:      domainnotification.TypePush,
				Recipient: domainnotification.NewPushRecipient("fcm-token-d4c7a1b2e9f04c8a9b3d5e6f7a8b9c0d", "user_789"),
			},
			wantErr: domaintemplate.ErrNoDefaultTemplate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// No EXPECT: publishing anything fails the test.
			publisher := mocks.NewMockPublisher(ctrl)
			svc := NewService(publisher, newTemplateService())

			_, err := svc.Send(context.Background(), tt.req)

			require.Error(t, err)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestServiceSendPublishFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)
	brokerErr := errors.New("rabbitmq connection refused")
	publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(brokerErr)

	svc := NewService(publisher, newTemplateService())

	_, err := svc.Send(context.Background(), &SendRequest{
		Type:       domainnotification.TypeEmail,
		Recipient:  domainnotification.NewEmailRecipient("anna.kowalska@example.com", "user_789"),
		TemplateID: "welcome",
	})

	require.ErrorIs(t, err, brokerErr)
}
