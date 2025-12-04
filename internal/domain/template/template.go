package template

import (
	"bytes"
	htmltemplate "html/template"
	texttemplate "text/template"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// Template represents a notification template.
type Template struct {
	// ID is the unique identifier for the template.
	ID string

	// Type specifies which notification channel this template is for.
	Type notification.Type

	// Name is a human-readable name for the template.
	Name string

	// Subject is the template for email subject or push title.
	// Uses Go template syntax.
	Subject string

	// Body is the main content template.
	// Uses Go template syntax. HTML for email, plain text for SMS/push.
	Body string

	// IsDefault indicates if this is the default template for its type.
	IsDefault bool
}

// New creates a new Template.
func New(id string, notificationType notification.Type, name, subject, body string, isDefault bool) *Template {
	return &Template{
		ID:        id,
		Type:      notificationType,
		Name:      name,
		Subject:   subject,
		Body:      body,
		IsDefault: isDefault,
	}
}

// Render processes the template with the given data and returns rendered content.
func (t *Template) Render(data map[string]any) (*notification.RenderedContent, error) {
	subject, err := t.renderSubject(data)
	if err != nil {
		return nil, err
	}

	body, err := t.renderBody(data)
	if err != nil {
		return nil, err
	}

	return &notification.RenderedContent{
		Subject: subject,
		Body:    body,
	}, nil
}

func (t *Template) renderSubject(data map[string]any) (string, error) {
	if t.Subject == "" {
		return "", nil
	}

	tmpl, err := texttemplate.New("subject").Parse(t.Subject)
	if err != nil {
		return "", NewRenderError("subject", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", NewRenderError("subject", err)
	}

	return buf.String(), nil
}

func (t *Template) renderBody(data map[string]any) (string, error) {
	if t.Body == "" {
		return "", nil
	}

	var buf bytes.Buffer

	if t.Type == notification.TypeEmail {
		tmpl, err := htmltemplate.New("body").Parse(t.Body)
		if err != nil {
			return "", NewRenderError("body", err)
		}
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", NewRenderError("body", err)
		}
	} else {
		tmpl, err := texttemplate.New("body").Parse(t.Body)
		if err != nil {
			return "", NewRenderError("body", err)
		}
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", NewRenderError("body", err)
		}
	}

	return buf.String(), nil
}
