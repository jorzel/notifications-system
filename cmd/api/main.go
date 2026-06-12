package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	infratemplate "github.com/jorzel/notifications-system/internal/infrastructure/template"
	httpserver "github.com/jorzel/notifications-system/internal/transport/http"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "notifications-api").
		Logger()

	templatesDir := envOr("TEMPLATES_DIR", "configs/templates")
	addr := envOr("HTTP_ADDR", ":8080")

	templateRepo, err := infratemplate.NewFileRepository(templatesDir)
	if err != nil {
		logger.Fatal().Err(err).Str("templates_dir", templatesDir).Msg("failed to load templates")
	}

	tmplSvc := apptemplate.NewService(templateRepo)
	notifSvc := appnotification.NewService(&stubPublisher{}, tmplSvc)

	server, err := httpserver.NewServer(notifSvc, tmplSvc, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build server")
	}

	go func() {
		logger.Info().Str("addr", addr).Msg("api server listening")
		if err := server.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("shutdown failed")
	}
	logger.Info().Msg("api server stopped")
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// stubPublisher accepts and drops messages so the API is runnable before
// the RabbitMQ publisher exists (Phase 7).
type stubPublisher struct{}

func (p *stubPublisher) Publish(ctx context.Context, msg *domainnotification.Message) error {
	zerolog.Ctx(ctx).Warn().
		Str("notification_id", msg.ID).
		Str("type", string(msg.Type)).
		Str("priority", string(msg.Priority)).
		Msg("stub publisher: dropping notification")
	return nil
}
