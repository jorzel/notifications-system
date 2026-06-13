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
	"github.com/jorzel/notifications-system/internal/config"
	"github.com/jorzel/notifications-system/internal/infrastructure/rabbitmq"
	infratemplate "github.com/jorzel/notifications-system/internal/infrastructure/template"
	"github.com/jorzel/notifications-system/internal/transport/rest"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "notifications-api").
		Logger()

	cfg, err := config.LoadAPI()
	if err != nil {
		logger.Fatal().Err(err).Msg("invalid api configuration")
	}

	templateRepo, err := infratemplate.NewFileRepository(cfg.TemplatesDir)
	if err != nil {
		logger.Fatal().Err(err).Str("templates_dir", cfg.TemplatesDir).Msg("failed to load templates")
	}

	conn, err := rabbitmq.Dial(cfg.RabbitURL)
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

	server, err := rest.NewServer(notifSvc, tmplSvc, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build server")
	}

	go func() {
		logger.Info().Str("addr", cfg.Addr).Msg("api server listening")
		if err := server.Start(cfg.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
