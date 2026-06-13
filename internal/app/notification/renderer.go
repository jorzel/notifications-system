package notification

import (
	"context"
	"fmt"

	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
)

// Renderer turns a queued Message into RenderedContent by resolving and
// rendering its template. Both failures are permanent: a bad template
// reference or a broken template will not succeed on retry, so they are
// wrapped in PermanentError for the worker to drop rather than retry.
type Renderer struct {
	templateSvc *apptemplate.Service
}

// NewRenderer creates a Renderer.
func NewRenderer(templateSvc *apptemplate.Service) *Renderer {
	return &Renderer{templateSvc: templateSvc}
}

// Render resolves the message's template and renders it with the message data.
func (r *Renderer) Render(ctx context.Context, msg *domainnotification.Message) (*domainnotification.RenderedContent, error) {
	tmpl, err := r.templateSvc.Resolve(ctx, msg.TemplateID, msg.Type)
	if err != nil {
		return nil, newPermanentError(fmt.Errorf("resolve template: %w", err))
	}

	content, err := tmpl.Render(msg.TemplateData)
	if err != nil {
		return nil, newPermanentError(fmt.Errorf("render template: %w", err))
	}
	return content, nil
}
