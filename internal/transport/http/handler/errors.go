package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/jorzel/notifications-system/internal/api"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

// Error codes exposed in ErrorResponse.code.
const (
	CodeValidationError  = "validation_error"
	CodeTemplateNotFound = "template_not_found"
	CodeInternalError    = "internal_error"
)

// MapError translates an error into the HTTP status and spec-defined
// ErrorResponse body. Template-not-found maps to 400 (a bad reference in
// a send request); the server's error handler upgrades it to 404 when
// the template itself is the requested resource. Unrecognized errors map
// to an opaque 500 — internals never leak to clients.
func MapError(err error) (int, api.ErrorResponse) {
	var valErr *domainnotification.ValidationError
	var httpErr *echo.HTTPError

	switch {
	case errors.As(err, &valErr):
		return http.StatusBadRequest, api.ErrorResponse{
			Code:    CodeValidationError,
			Message: valErr.Error(),
			Field:   &valErr.Field,
		}
	case errors.Is(err, domaintemplate.ErrTemplateNotFound) || errors.Is(err, domaintemplate.ErrNoDefaultTemplate):
		return http.StatusBadRequest, api.ErrorResponse{
			Code:    CodeTemplateNotFound,
			Message: err.Error(),
		}
	case errors.As(err, &httpErr):
		// Echo and the spec-validation middleware report client errors
		// (malformed JSON, schema violations, unknown routes) this way.
		return httpErr.Code, api.ErrorResponse{
			Code:    codeForStatus(httpErr.Code),
			Message: fmt.Sprintf("%v", httpErr.Message),
		}
	default:
		return http.StatusInternalServerError, api.ErrorResponse{
			Code:    CodeInternalError,
			Message: "internal server error",
		}
	}
}

func codeForStatus(status int) string {
	if status >= http.StatusInternalServerError {
		return CodeInternalError
	}
	return strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_"))
}
