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
	"github.com/jorzel/notifications-system/internal/infrastructure/rabbitmq"
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
	rabbitURL := envOr("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	templateRepo, err := infratemplate.NewFileRepository(templatesDir)
	if err != nil {
		logger.Fatal().Err(err).Str("templates_dir", templatesDir).Msg("failed to load templates")
	}

	conn, err := rabbitmq.Dial(rabbitURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to rabbitmq")
	}
	defer func() { _ = conn.Close() }()

	publisher, err := rabbitmq.NewPublisher(conn)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to set up publisher")
	}

	tmplSvc := apptemplate.NewService(templateRepo)
	notifSvc := appnotification.NewService(publisher, tmplSvc)

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
