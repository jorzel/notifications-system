package template

import (
	"errors"
	"fmt"
)

var (
	// ErrTemplateNotFound is returned when a template cannot be found.
	ErrTemplateNotFound = errors.New("template not found")

	// ErrNoDefaultTemplate is returned when no default template exists for a type.
	ErrNoDefaultTemplate = errors.New("no default template for notification type")
)

// RenderError represents an error that occurred during template rendering.
type RenderError struct {
	Field string
	Err   error
}

// NewRenderError creates a new RenderError.
func NewRenderError(field string, err error) *RenderError {
	return &RenderError{
		Field: field,
		Err:   err,
	}
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("failed to render template %s: %v", e.Field, e.Err)
}

func (e *RenderError) Unwrap() error {
	return e.Err
}
