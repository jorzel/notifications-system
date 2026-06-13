// Package config loads typed application configuration from environment
// variables, applying defaults and validating values up front so the
// binaries fail fast on misconfiguration.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

const defaultRabbitURL = "amqp://guest:guest@127.0.0.1:5672/"

// API is the configuration for the API server.
type API struct {
	Addr         string
	TemplatesDir string
	RabbitURL    string
}

// LoadAPI reads API configuration from the environment.
func LoadAPI() (API, error) {
	return API{
		Addr:         envOr("HTTP_ADDR", ":8080"),
		TemplatesDir: envOr("TEMPLATES_DIR", "configs/templates"),
		RabbitURL:    envOr("RABBITMQ_URL", defaultRabbitURL),
	}, nil
}

// Worker is the configuration for a single-lane worker process.
type Worker struct {
	Type         notification.Type
	Priority     notification.Priority
	TemplatesDir string
	RabbitURL    string
	Prefetch     int
	MaxRetries   int
	RateLimit    int // provider calls per second
}

// LoadWorker reads worker configuration from the environment and validates
// the lane selection.
func LoadWorker() (Worker, error) {
	notifType := notification.Type(os.Getenv("WORKER_TYPE"))
	if !notifType.IsValid() {
		return Worker{}, fmt.Errorf("WORKER_TYPE must be one of email|sms|push, got %q", notifType)
	}

	priority := notification.Priority(os.Getenv("WORKER_PRIORITY"))
	if !priority.IsValid() {
		return Worker{}, fmt.Errorf("WORKER_PRIORITY must be one of transactional|bulk, got %q", priority)
	}

	prefetch, err := envInt("WORKER_PREFETCH", 1)
	if err != nil {
		return Worker{}, err
	}
	maxRetries, err := envInt("WORKER_MAX_RETRIES", 3)
	if err != nil {
		return Worker{}, err
	}
	rateLimit, err := envInt("WORKER_RATE_LIMIT", 50)
	if err != nil {
		return Worker{}, err
	}

	return Worker{
		Type:         notifType,
		Priority:     priority,
		TemplatesDir: envOr("TEMPLATES_DIR", "configs/templates"),
		RabbitURL:    envOr("RABBITMQ_URL", defaultRabbitURL),
		Prefetch:     prefetch,
		MaxRetries:   maxRetries,
		RateLimit:    rateLimit,
	}, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return value, nil
}
