package template

import (
	"context"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// Repository defines the interface for template storage operations.
type Repository interface {
	// GetByID retrieves a template by its ID.
	// Returns ErrTemplateNotFound if the template does not exist.
	GetByID(ctx context.Context, id string) (*Template, error)

	// GetDefault retrieves the default template for a notification type.
	// Returns ErrNoDefaultTemplate if no default template exists.
	GetDefault(ctx context.Context, notificationType notification.Type) (*Template, error)

	// List retrieves all available templates.
	List(ctx context.Context) ([]*Template, error)
}
