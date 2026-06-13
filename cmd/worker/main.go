package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	"github.com/jorzel/notifications-system/internal/config"
	"github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/provider"
	"github.com/jorzel/notifications-system/internal/infrastructure/rabbitmq"
	infratemplate "github.com/jorzel/notifications-system/internal/infrastructure/template"
	"github.com/jorzel/notifications-system/internal/transport/queue"
)

// A worker process serves exactly one lane: WORKER_TYPE × WORKER_PRIORITY.
// Deploy one process per lane so transactional lanes get dedicated
// capacity and each lane scales independently on its own queue depth.
func main() {
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "notifications-worker").
		Logger()

	cfg, err := config.LoadWorker()
	if err != nil {
		logger.Fatal().Err(err).Msg("invalid worker configuration")
	}
	logger = logger.With().Str("type", string(cfg.Type)).Str("priority", string(cfg.Priority)).Logger()

	templateRepo, err := infratemplate.NewFileRepository(cfg.TemplatesDir)
	if err != nil {
		logger.Fatal().Err(err).Str("templates_dir", cfg.TemplatesDir).Msg("failed to load templates")
	}
	renderer := appnotification.NewRenderer(apptemplate.NewService(templateRepo))
	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.RateLimit)

	processor, err := buildProcessor(cfg.Type, renderer, limiter)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build processor")
	}

	// Separate connections for consuming and for retry/dead-letter publishing.
	consumerConn, err := rabbitmq.Dial(cfg.RabbitURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to rabbitmq (consumer)")
	}
	defer func() { _ = consumerConn.Close() }()

	publisherConn, err := rabbitmq.Dial(cfg.RabbitURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to rabbitmq (publisher)")
	}
	defer func() { _ = publisherConn.Close() }()

	// Declares the topology (idempotent) so the lane queue exists before we consume.
	publisher, err := rabbitmq.NewPublisher(publisherConn)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to set up publisher")
	}

	queueName := rabbitmq.QueueName(cfg.Type, cfg.Priority)
	deliveries, err := consumerConn.Consume(queueName, cfg.Prefetch)
	if err != nil {
		logger.Fatal().Err(err).Str("queue", queueName).Msg("failed to consume")
	}

	ctx, cancel := context.WithCancel(logger.WithContext(context.Background()))
	defer cancel()

	consumer := queue.NewConsumer(adapt(ctx, deliveries), processor, publisher, cfg.MaxRetries)

	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()
	logger.Info().Str("queue", queueName).Int("prefetch", cfg.Prefetch).Msg("worker consuming")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case <-quit:
		logger.Info().Msg("shutting down")
		cancel()
	case err := <-done:
		if err != nil && ctx.Err() == nil {
			logger.Fatal().Err(err).Msg("consumer stopped unexpectedly")
		}
	}
	logger.Info().Msg("worker stopped")
}

// buildProcessor selects the processor for the lane's channel. Senders are
// stubs until Phase 9 wires the real providers; the consumer's delivery
// log already shows the pipeline draining.
func buildProcessor(notifType notification.Type, renderer *appnotification.Renderer, limiter *rate.Limiter) (queue.Processor, error) {
	switch notifType {
	case notification.TypeEmail:
		return appnotification.NewEmailProcessor(renderer, stubEmailSender{}, limiter), nil
	case notification.TypeSMS:
		return appnotification.NewSMSProcessor(renderer, stubSMSSender{}, limiter), nil
	case notification.TypePush:
		return appnotification.NewPushProcessor(renderer, stubPushSender{}, limiter), nil
	default:
		return nil, fmt.Errorf("unknown worker type %q", notifType)
	}
}

// adapt converts broker deliveries to the queue transport's Delivery
// contract, stopping when the context is cancelled.
func adapt(ctx context.Context, in <-chan rabbitmq.Delivery) <-chan queue.Delivery {
	out := make(chan queue.Delivery)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-in:
				if !ok {
					return
				}
				out <- d
			}
		}
	}()
	return out
}

// Stub senders accept and drop messages so the worker is runnable before
// the real providers exist (Phase 9 replaces these).
type stubEmailSender struct{}

func (stubEmailSender) Send(context.Context, provider.EmailMessage) error { return nil }

type stubSMSSender struct{}

func (stubSMSSender) Send(context.Context, provider.SMSMessage) error { return nil }

type stubPushSender struct{}

func (stubPushSender) Send(context.Context, provider.PushMessage) error { return nil }
