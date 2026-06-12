package template

import (
	"context"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

// Repository defines the interface for template storage operations.
// Templates are scoped by notification type: the same ID (e.g. "default")
// may exist independently for each channel.
type Repository interface {
	// Get retrieves a template by notification type and ID.
	// Returns ErrTemplateNotFound if the template does not exist.
	Get(ctx context.Context, notificationType notification.Type, id string) (*Template, error)

	// GetDefault retrieves the default template for a notification type.
	// Returns ErrNoDefaultTemplate if no default template exists.
	GetDefault(ctx context.Context, notificationType notification.Type) (*Template, error)

	// List retrieves all available templates.
	List(ctx context.Context) ([]*Template, error)
}
