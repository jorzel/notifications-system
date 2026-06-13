package notification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

func rendererWith(templates ...*domaintemplate.Template) *Renderer {
	return NewRenderer(apptemplate.NewService(&fakeTemplateRepository{templates: templates}))
}

func TestRendererSuccess(t *testing.T) {
	r := rendererWith(
		domaintemplate.New("welcome", domainnotification.TypeEmail, "Welcome", "Welcome, {{.name}}!", "<h1>Hi {{.name}}</h1>", false),
	)
	msg := &domainnotification.Message{Type: domainnotification.TypeEmail, TemplateID: "welcome", TemplateData: map[string]any{"name": "Anna"}}

	content, err := r.Render(context.Background(), msg)

	require.NoError(t, err)
	require.Equal(t, "Welcome, Anna!", content.Subject)
	require.Contains(t, content.Body, "<h1>Hi Anna</h1>")
}

func TestRendererPermanentFailure(t *testing.T) {
	tests := []struct {
		name string
		msg  *domainnotification.Message
	}{
		{
			name: "unknown template",
			msg:  &domainnotification.Message{Type: domainnotification.TypeEmail, TemplateID: "missing"},
		},
		{
			name: "render execution error",
			// Body indexes a field on a string value, which text/template rejects at execution.
			msg: &domainnotification.Message{Type: domainnotification.TypeEmail, TemplateID: "broken", TemplateData: map[string]any{"x": "not-a-map"}},
		},
	}

	r := rendererWith(
		domaintemplate.New("broken", domainnotification.TypeEmail, "Broken", "", "{{.x.y}}", false),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Render(context.Background(), tt.msg)

			var perm *PermanentError
			require.ErrorAs(t, err, &perm, "template failures must be permanent")
		})
	}
}
