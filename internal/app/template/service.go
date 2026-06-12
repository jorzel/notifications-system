// Package template provides template use cases: resolving and listing
// notification templates. Rendering is behavior of the Template entity
// itself (Template.Render) and happens at the worker.
package template

import (
	"context"

	"github.com/jorzel/notifications-system/internal/domain/notification"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

// Service handles template resolution and listing.
type Service struct {
	repo domaintemplate.Repository
}

// NewService creates a template Service.
func NewService(repo domaintemplate.Repository) *Service {
	return &Service{repo: repo}
}

// Resolve returns the template for the given ID and notification type.
// An empty templateID resolves to the default template for the type.
func (s *Service) Resolve(ctx context.Context, templateID string, notificationType notification.Type) (*domaintemplate.Template, error) {
	if templateID == "" {
		return s.repo.GetDefault(ctx, notificationType)
	}
	return s.repo.Get(ctx, notificationType, templateID)
}

// List returns available templates, optionally filtered by notification
// type. A zero-value typeFilter returns all templates.
func (s *Service) List(ctx context.Context, typeFilter notification.Type) ([]*domaintemplate.Template, error) {
	templates, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if typeFilter == "" {
		return templates, nil
	}

	filtered := make([]*domaintemplate.Template, 0, len(templates))
	for _, tmpl := range templates {
		if tmpl.Type == typeFilter {
			filtered = append(filtered, tmpl)
		}
	}
	return filtered, nil
}
