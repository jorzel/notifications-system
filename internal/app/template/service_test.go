package template

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jorzel/notifications-system/internal/domain/notification"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

// fakeRepository is an in-memory template.Repository for testing.
type fakeRepository struct {
	templates []*domaintemplate.Template
}

func (r *fakeRepository) Get(_ context.Context, notificationType notification.Type, id string) (*domaintemplate.Template, error) {
	for _, tmpl := range r.templates {
		if tmpl.Type == notificationType && tmpl.ID == id {
			return tmpl, nil
		}
	}
	return nil, fmt.Errorf("%w: %s/%s", domaintemplate.ErrTemplateNotFound, notificationType, id)
}

func (r *fakeRepository) GetDefault(_ context.Context, notificationType notification.Type) (*domaintemplate.Template, error) {
	for _, tmpl := range r.templates {
		if tmpl.Type == notificationType && tmpl.IsDefault {
			return tmpl, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", domaintemplate.ErrNoDefaultTemplate, notificationType)
}

func (r *fakeRepository) List(_ context.Context) ([]*domaintemplate.Template, error) {
	return r.templates, nil
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		templates: []*domaintemplate.Template{
			domaintemplate.New("default", notification.TypeEmail, "Default Email", "{{.subject}}", "<p>{{.message}}</p>", true),
			domaintemplate.New("welcome", notification.TypeEmail, "Welcome Email", "Welcome, {{.name}}!", "<h1>Hello, {{.name}}!</h1>", false),
			domaintemplate.New("default", notification.TypeSMS, "Default SMS", "", "{{.message}}", true),
		},
	}
}

func TestServiceResolveSuccess(t *testing.T) {
	tests := []struct {
		name       string
		templateID string
		notifType  notification.Type
		wantID     string
	}{
		{
			name:       "explicit template id",
			templateID: "welcome",
			notifType:  notification.TypeEmail,
			wantID:     "welcome",
		},
		{
			name:       "empty template id falls back to default",
			templateID: "",
			notifType:  notification.TypeEmail,
			wantID:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newFakeRepository())

			tmpl, err := svc.Resolve(context.Background(), tt.templateID, tt.notifType)

			require.NoError(t, err)
			require.Equal(t, tt.wantID, tmpl.ID)
			require.Equal(t, tt.notifType, tmpl.Type)
		})
	}
}

func TestServiceResolveFailed(t *testing.T) {
	tests := []struct {
		name       string
		templateID string
		notifType  notification.Type
		wantErr    error
	}{
		{
			name:       "unknown template id",
			templateID: "order_confirmation",
			notifType:  notification.TypeEmail,
			wantErr:    domaintemplate.ErrTemplateNotFound,
		},
		{
			name:       "no default template for type",
			templateID: "",
			notifType:  notification.TypePush,
			wantErr:    domaintemplate.ErrNoDefaultTemplate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newFakeRepository())

			_, err := svc.Resolve(context.Background(), tt.templateID, tt.notifType)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestServiceList(t *testing.T) {
	svc := NewService(newFakeRepository())

	templates, err := svc.List(context.Background())

	require.NoError(t, err)
	require.Len(t, templates, 3)
}
