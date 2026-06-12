// Package notification provides notification use cases. The Service
// accepts send requests on the API hot path: it validates input, checks
// the template exists, and publishes the notification for asynchronous
// delivery. Rendering and provider calls happen at the worker.
package notification

import (
	"context"
	"fmt"
	"time"

	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
)

// SendRequest is the transport-agnostic input for sending a notification.
type SendRequest struct {
	Type domainnotification.Type

	// Priority is optional and defaults to transactional. Bulk senders
	// must opt in explicitly so campaigns land in the bulk queue lane.
	Priority domainnotification.Priority

	Recipient domainnotification.Recipient

	// TemplateID is optional; empty resolves the default template for the type.
	TemplateID string

	TemplateData map[string]any
	Metadata     map[string]string
}

// SendResponse confirms a notification was accepted for delivery.
type SendResponse struct {
	ID        string
	Status    domainnotification.Status
	CreatedAt time.Time
}

// Service handles notification send requests.
type Service struct {
	publisher   domainnotification.Publisher
	templateSvc *apptemplate.Service
}

// NewService creates a notification Service.
func NewService(publisher domainnotification.Publisher, templateSvc *apptemplate.Service) *Service {
	return &Service{
		publisher:   publisher,
		templateSvc: templateSvc,
	}
}

// Send accepts a notification request and publishes it for asynchronous
// delivery. The returned status is pending; actual delivery happens at
// the worker.
func (s *Service) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	priority := req.Priority
	if priority == "" {
		priority = domainnotification.PriorityTransactional
	}

	n, err := domainnotification.New(req.Type, priority, req.Recipient, req.TemplateID, req.TemplateData)
	if err != nil {
		return nil, err
	}

	tmpl, err := s.templateSvc.Resolve(ctx, n.TemplateID, n.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template: %w", err)
	}
	// Stamp the resolved ID so the worker fetches the same template even
	// when the request relied on the default.
	n.TemplateID = tmpl.ID

	if err := s.publisher.Publish(ctx, domainnotification.NewMessage(n, req.Metadata)); err != nil {
		return nil, fmt.Errorf("failed to publish notification %s: %w", n.ID, err)
	}

	return &SendResponse{
		ID:        n.ID,
		Status:    n.Status,
		CreatedAt: n.CreatedAt,
	}, nil
}
