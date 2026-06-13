// Package httpserver assembles the Echo server: routes registered from
// the generated OpenAPI code, request validation against the spec, and
// centralized domain-error-to-HTTP mapping.
package rest

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	oapimiddleware "github.com/oapi-codegen/echo-middleware"
	"github.com/rs/zerolog"

	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	"github.com/jorzel/notifications-system/internal/transport/rest/api"
	"github.com/jorzel/notifications-system/internal/transport/rest/handler"
	"github.com/jorzel/notifications-system/internal/transport/rest/middleware"
)

// HTTP server timeouts harden against slow clients and hung connections.
// WriteTimeout must exceed the publisher's confirm timeout so a valid but
// slow publish isn't cut off mid-response.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// NewServer builds the HTTP server with all routes and middleware.
// The logger is injected only here, at the composition seam: middleware
// attaches a request-scoped child to each request context, and all
// downstream code uses zerolog.Ctx(ctx).
func NewServer(notifSvc *appnotification.Service, tmplSvc *apptemplate.Service, logger zerolog.Logger) (*echo.Echo, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded OpenAPI spec: %w", err)
	}

	server := echo.New()
	server.HideBanner = true
	server.HidePort = true
	server.HTTPErrorHandler = httpErrorHandler

	server.Server.ReadHeaderTimeout = readHeaderTimeout
	server.Server.ReadTimeout = readTimeout
	server.Server.WriteTimeout = writeTimeout
	server.Server.IdleTimeout = idleTimeout

	server.Use(echomiddleware.RequestID())
	server.Use(middleware.ContextLogger(logger))
	server.Use(middleware.RequestLogger())
	server.Use(echomiddleware.Recover())

	server.GET("/health", handler.HealthCheck)

	apiGroup := server.Group("/api/v1")
	apiGroup.Use(oapimiddleware.OapiRequestValidatorWithOptions(spec, &oapimiddleware.Options{
		// The spec's `servers` entry carries the /api/v1 base path used
		// for route matching; no Host validation is intended.
		SilenceServersWarning: true,
	}))
	api.RegisterHandlers(apiGroup, api.NewStrictHandler(handler.New(notifSvc, tmplSvc), nil))

	return server, nil
}

// httpErrorHandler is the single place translating errors into HTTP
// responses, per the error-mapping table in PLAN.md.
func httpErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status, resp := handler.MapError(err)

	// A missing template referenced from a send request stays 400, but
	// when the template itself is the requested resource it's a 404.
	if resp.Code == handler.CodeTemplateNotFound && strings.HasPrefix(c.Path(), "/api/v1/templates") {
		status = http.StatusNotFound
	}

	logger := zerolog.Ctx(c.Request().Context())

	if status >= http.StatusInternalServerError {
		// Details stay server-side; clients get an opaque message.
		logger.Err(err).Msg("request failed")
	}

	if jsonErr := c.JSON(status, resp); jsonErr != nil {
		logger.Err(jsonErr).Msg("failed to write error response")
	}
}
