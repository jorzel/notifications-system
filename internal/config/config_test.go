package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jorzel/notifications-system/internal/domain/notification"
)

func TestLoadAPIDefaults(t *testing.T) {
	cfg, err := LoadAPI()

	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.Addr)
	require.Equal(t, "configs/templates", cfg.TemplatesDir)
	require.Contains(t, cfg.RabbitURL, "amqp://")
}

func TestLoadAPIFromEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("TEMPLATES_DIR", "/etc/templates")
	t.Setenv("RABBITMQ_URL", "amqp://user:pass@broker:5672/")

	cfg, err := LoadAPI()

	require.NoError(t, err)
	require.Equal(t, ":9090", cfg.Addr)
	require.Equal(t, "/etc/templates", cfg.TemplatesDir)
	require.Equal(t, "amqp://user:pass@broker:5672/", cfg.RabbitURL)
}

func TestLoadWorkerSuccess(t *testing.T) {
	t.Setenv("WORKER_TYPE", "email")
	t.Setenv("WORKER_PRIORITY", "bulk")
	t.Setenv("WORKER_PREFETCH", "20")
	t.Setenv("WORKER_MAX_RETRIES", "5")
	t.Setenv("WORKER_RATE_LIMIT", "100")

	cfg, err := LoadWorker()

	require.NoError(t, err)
	require.Equal(t, notification.TypeEmail, cfg.Type)
	require.Equal(t, notification.PriorityBulk, cfg.Priority)
	require.Equal(t, 20, cfg.Prefetch)
	require.Equal(t, 5, cfg.MaxRetries)
	require.Equal(t, 100, cfg.RateLimit)
}

func TestLoadWorkerDefaults(t *testing.T) {
	t.Setenv("WORKER_TYPE", "sms")
	t.Setenv("WORKER_PRIORITY", "transactional")

	cfg, err := LoadWorker()

	require.NoError(t, err)
	require.Equal(t, 1, cfg.Prefetch)
	require.Equal(t, 3, cfg.MaxRetries)
	require.Equal(t, 50, cfg.RateLimit)
}

func TestLoadWorkerFailed(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "missing type",
			env:  map[string]string{"WORKER_PRIORITY": "bulk"},
		},
		{
			name: "invalid type",
			env:  map[string]string{"WORKER_TYPE": "fax", "WORKER_PRIORITY": "bulk"},
		},
		{
			name: "invalid priority",
			env:  map[string]string{"WORKER_TYPE": "email", "WORKER_PRIORITY": "urgent"},
		},
		{
			name: "non-integer prefetch",
			env:  map[string]string{"WORKER_TYPE": "email", "WORKER_PRIORITY": "bulk", "WORKER_PREFETCH": "lots"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := LoadWorker()

			require.Error(t, err)
		})
	}
}
