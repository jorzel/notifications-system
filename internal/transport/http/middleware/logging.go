// Package middleware provides HTTP middleware for the transport layer.
package middleware

import (
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
)

// ContextLogger attaches a request-scoped logger (enriched with the
// request ID) to the request context. All downstream code — handlers,
// services, the error handler — retrieves it with zerolog.Ctx(ctx)
// instead of receiving a logger dependency.
func ContextLogger(logger zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestLogger := logger.With().
				Str("request_id", c.Response().Header().Get(echo.HeaderXRequestID)).
				Logger()

			ctx := requestLogger.WithContext(c.Request().Context())
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// RequestLogger logs one line per completed request: INFO for successes,
// WARN for client errors, ERROR for server errors. The logger comes from
// the request context, so every line carries the request ID. Recipient
// addresses and payloads are never logged.
func RequestLogger() echo.MiddlewareFunc {
	return echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		LogMethod:  true,
		LogURIPath: true,
		LogStatus:  true,
		LogLatency: true,
		LogError:   true,
		// Run the error handler before logging so the logged status is
		// the one actually sent to the client (otherwise errors log 200).
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			logger := zerolog.Ctx(c.Request().Context())

			event := logger.Info()
			switch {
			case v.Status >= 500:
				event = logger.Error().Err(v.Error)
			case v.Status >= 400:
				event = logger.Warn()
			}

			event.
				Str("method", v.Method).
				Str("path", v.URIPath).
				Int("status", v.Status).
				Dur("duration_ms", v.Latency).
				Msg("request handled")
			return nil
		},
	})
}
