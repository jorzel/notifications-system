# Notifications System - Implementation Plan

## Overview

A scalable, microservice-ready notification system built in Go that supports multiple notification channels (email, SMS, push) with template-based messaging, asynchronous processing via message queues, and clean architecture principles.

---

## 1. System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              External Clients                                │
│                         (Other Microservices, Apps)                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            API Server (REST)                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐              │
│  │  POST /notify   │  │ GET /templates  │  │ Health/Metrics  │              │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘              │
│                                                                              │
│  • Request validation (email format, phone format, etc.)                     │
│  • Template resolution                                                       │
│  • Notification event publishing                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Message Queue (RabbitMQ)                           │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                    │
│  │  email_queue  │  │   sms_queue   │  │  push_queue   │                    │
│  └───────────────┘  └───────────────┘  └───────────────┘                    │
└─────────────────────────────────────────────────────────────────────────────┘
                    │                 │                 │
                    ▼                 ▼                 ▼
         ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
         │Email Workers │  │ SMS Workers  │  │ Push Workers │
         │  (N pods)    │  │  (N pods)    │  │  (N pods)    │
         └──────────────┘  └──────────────┘  └──────────────┘
                    │                 │                 │
                    ▼                 ▼                 ▼
         ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
         │   SendGrid   │  │    Twilio    │  │     FCM      │
         │   (SMTP)     │  │              │  │    (APNs)    │
         └──────────────┘  └──────────────┘  └──────────────┘
```

### Key Components

1. **API Server**: Stateless REST API that validates requests, resolves templates, and publishes notification events to message queues
2. **Message Queue**: RabbitMQ for reliable, asynchronous message delivery with separate queues per notification type
3. **Workers**: Independent processor services that consume from queues and deliver notifications via third-party providers

---

## 2. Project Structure

```
notifications-system/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── internal/
│   ├── api/
│   │   └── openapi.yaml
│   │
│   ├── domain/
│   │   ├── notification/           # Notification aggregate
│   │   │   ├── notification.go     # Notification entity
│   │   │   ├── recipient.go        # Recipient value object
│   │   │   ├── message.go          # NotificationMessage (queue payload)
│   │   │   ├── errors.go           # Notification-specific errors
│   │   │   ├── publisher.go        # NotificationPublisher interface
│   │   │   └── validator.go        # NotificationValidator interface
│   │   │
│   │   ├── template/               # Template aggregate
│   │   │   ├── template.go         # Template entity
│   │   │   ├── errors.go           # Template-specific errors
│   │   │   └── repository.go       # TemplateRepository interface
│   │   │
│   │   └── provider/               # Provider interfaces
│   │       ├── email.go            # EmailProvider interface
│   │       ├── sms.go              # SMSProvider interface
│   │       └── push.go             # PushProvider interface
│   │
│   ├── app/
│   │   ├── notification/           # Notification use cases
│   │   │   ├── service.go          # NotificationService (create & publish)
│   │   │   ├── service_test.go
│   │   │   ├── email_processor.go  # Email processing logic
│   │   │   ├── email_processor_test.go
│   │   │   ├── sms_processor.go    # SMS processing logic
│   │   │   ├── sms_processor_test.go
│   │   │   ├── push_processor.go   # Push processing logic
│   │   │   └── push_processor_test.go
│   │   │
│   │   └── template/               # Template use cases
│   │       ├── service.go          # TemplateService (load, render, resolve)
│   │       └── service_test.go
│   │
│   ├── infrastructure/
│   │   ├── queue/
│   │   │   ├── rabbitmq.go         # RabbitMQ connection
│   │   │   └── publisher.go        # Publisher implementation
│   │   │
│   │   ├── provider/
│   │   │   ├── sendgrid.go         # SendGrid email provider
│   │   │   ├── twilio.go           # Twilio SMS provider
│   │   │   └── fcm.go              # FCM push provider
│   │   │
│   │   └── template/
│   │       ├── file_repository.go  # File-based template repository
│   │       └── file_repository_test.go
│   │
│   ├── transport/
│   │   ├── http/
│   │   │   ├── handler/
│   │   │   │   ├── notification.go
│   │   │   │   ├── notification_test.go
│   │   │   │   ├── template.go
│   │   │   │   ├── template_test.go
│   │   │   │   └── health.go
│   │   │   ├── middleware/
│   │   │   │   ├── logging.go
│   │   │   │   └── recovery.go
│   │   │   ├── dto/
│   │   │   │   ├── request.go
│   │   │   │   └── response.go
│   │   │   └── server.go
│   │   │
│   │   └── worker/
│   │       ├── consumer.go
│   │       └── consumer_test.go
│   │
│   └── validation/
│       ├── email.go
│       ├── email_test.go
│       ├── phone.go
│       ├── phone_test.go
│       └── composite.go
│
├── configs/
│   ├── config.yaml
│   └── templates/
│       ├── email/
│       │   ├── default.html
│       │   └── welcome.html
│       ├── sms/
│       │   └── default.txt
│       └── push/
│           └── default.json
│
├── integration_tests/
│   ├── api_test.go
│   └── worker_test.go
│
├── docker-compose.yaml
├── Dockerfile.api
├── Dockerfile.worker
├── Makefile
├── go.mod
└── go.sum
```

### Layer Responsibilities

| Layer | Responsibility |
|-------|----------------|
| **domain/** | Entities, value objects, interfaces (contracts) - grouped by aggregate |
| **app/** | Business logic, use cases, processing logic - grouped by feature |
| **infrastructure/** | External systems implementation (queue, providers, storage) |
| **transport/** | HTTP handlers (Echo), worker loops (queue consumers) |

---

## 3. Domain Models

### Notification Aggregate (`domain/notification/`)

```go
// notification.go
type NotificationType string

const (
    TypeEmail NotificationType = "email"
    TypeSMS   NotificationType = "sms"
    TypePush  NotificationType = "push"
)

type Status string

const (
    StatusPending Status = "pending"
    StatusSent    Status = "sent"
    StatusFailed  Status = "failed"
)

type Notification struct {
    ID           string
    Type         NotificationType
    Recipient    Recipient
    TemplateID   string
    TemplateData map[string]any
    Status       Status
    CreatedAt    time.Time
    SentAt       *time.Time
    Error        *string
}
```

```go
// recipient.go
type Recipient struct {
    Email       string
    PhoneNumber string
    DeviceToken string
    UserID      string
}
```

```go
// message.go - queue payload
type Message struct {
    ID              string
    Type            NotificationType
    Recipient       Recipient
    RenderedContent RenderedContent
    Metadata        map[string]string
    CreatedAt       time.Time
    RetryCount      int
}

type RenderedContent struct {
    Subject string            // email only
    Body    string
    Title   string            // push only
    Data    map[string]string // push only
}
```

```go
// publisher.go
type Publisher interface {
    Publish(ctx context.Context, msg *Message) error
}

// validator.go
type Validator interface {
    Validate(n *Notification) error
}
```

### Template Aggregate (`domain/template/`)

```go
// template.go
type Template struct {
    ID        string
    Type      notification.NotificationType
    Name      string
    Subject   string
    Body      string
    IsDefault bool
}

// repository.go
type Repository interface {
    GetByID(ctx context.Context, id string) (*Template, error)
    GetDefault(ctx context.Context, notifType notification.NotificationType) (*Template, error)
    List(ctx context.Context) ([]*Template, error)
}
```

### Provider Interfaces (`domain/provider/`)

```go
// email.go
type EmailSender interface {
    Send(ctx context.Context, to, subject, body string) error
}

// sms.go
type SMSSender interface {
    Send(ctx context.Context, to, message string) error
}

// push.go
type PushSender interface {
    Send(ctx context.Context, token, title, body string, data map[string]string) error
}
```

---

## 4. Application Layer

### Notification Service (`app/notification/service.go`)

```go
type Service struct {
    publisher   notification.Publisher
    templateSvc *template.Service
    validator   notification.Validator
    logger      zerolog.Logger
}

func (s *Service) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
    // 1. Create notification
    // 2. Validate
    // 3. Resolve template
    // 4. Render content
    // 5. Publish to queue
}
```

### Processors (`app/notification/`)

```go
// email_processor.go
type EmailProcessor struct {
    sender provider.EmailSender
    logger zerolog.Logger
}

func (p *EmailProcessor) Process(ctx context.Context, msg *notification.Message) error {
    return p.sender.Send(ctx,
        msg.Recipient.Email,
        msg.RenderedContent.Subject,
        msg.RenderedContent.Body,
    )
}
```

### Template Service (`app/template/service.go`)

```go
type Service struct {
    repo   template.Repository
    logger zerolog.Logger
}

func (s *Service) Resolve(ctx context.Context, templateID string, notifType notification.NotificationType) (*template.Template, error)
func (s *Service) Render(tmpl *template.Template, data map[string]any) (*RenderedContent, error)
func (s *Service) List(ctx context.Context) ([]*template.Template, error)
```

---

## 5. API Design

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/notifications` | Send a notification |
| POST | `/api/v1/notifications/batch` | Send multiple notifications |
| GET | `/api/v1/templates` | List available templates |
| GET | `/api/v1/templates/{id}` | Get template details |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

### Send Notification Request

```json
POST /api/v1/notifications
{
    "type": "email",
    "recipient": {
        "email": "user@example.com",
        "user_id": "user_123"
    },
    "template_id": "welcome",
    "template_data": {
        "name": "John",
        "activation_link": "https://..."
    },
    "metadata": {
        "correlation_id": "order_456"
    }
}
```

### Response

```json
{
    "id": "notif_abc123",
    "status": "pending",
    "created_at": "2025-01-15T10:30:00Z"
}
```

---

## 6. Transport Layer

### HTTP Server (`transport/http/server.go`)

```go
func NewServer(cfg *config.Config, notifSvc *notification.Service, tmplSvc *template.Service, logger zerolog.Logger) *echo.Echo {
    e := echo.New()
    e.HideBanner = true

    // Middleware
    e.Use(middleware.RequestID())
    e.Use(middleware.Recover())
    e.Use(ZerologMiddleware(logger))

    // Health & metrics
    e.GET("/health", handler.HealthCheck)
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

    // API routes
    api := e.Group("/api/v1")
    notifHandler := handler.NewNotificationHandler(notifSvc, logger)
    tmplHandler := handler.NewTemplateHandler(tmplSvc, logger)

    api.POST("/notifications", notifHandler.Send)
    api.POST("/notifications/batch", notifHandler.SendBatch)
    api.GET("/templates", tmplHandler.List)
    api.GET("/templates/:id", tmplHandler.Get)

    return e
}
```

### Worker Consumer (`transport/worker/consumer.go`)

```go
type Consumer struct {
    queue     QueueConsumer
    processor Processor
    logger    zerolog.Logger
}

type Processor interface {
    Process(ctx context.Context, msg *notification.Message) error
}

func (c *Consumer) Run(ctx context.Context) error {
    messages, err := c.queue.Consume(ctx)
    if err != nil {
        return err
    }

    for {
        select {
        case <-ctx.Done():
            c.logger.Info().Msg("worker shutting down")
            return ctx.Err()
        case msg := <-messages:
            if err := c.processor.Process(ctx, msg); err != nil {
                c.handleError(msg, err)
            } else {
                msg.Ack()
            }
        }
    }
}
```

---

## 7. Observability

### Structured Logging (zerolog)

```go
logger := zerolog.New(os.Stdout).With().
    Timestamp().
    Str("service", "notification-service").
    Logger()

// Business events
logger.Info().
    Str("notification_id", id).
    Str("type", string(notifType)).
    Msg("notification created")

// Errors with context
logger.Err(err).
    Str("notification_id", id).
    Msg("failed to send notification")
```

### Metrics (Prometheus)

```go
var (
    notificationsSent = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "notifications_sent_total",
            Help: "Total notifications sent",
        },
        []string{"type", "status"},
    )

    processingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "notification_processing_duration_seconds",
            Help:    "Processing duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"type"},
    )
)
```

---

## 8. Implementation Phases (TDD Approach)

Each step follows: **Write Tests → Implement → Refactor**

### Phase 1: Project Setup
- [ ] Initialize go.mod, Makefile, Docker files
- [ ] Set up linting (golangci-lint)
- [ ] Configure test framework (testify)

### Phase 2: Domain Layer
- [ ] Define notification entity and value objects
- [ ] Define template entity
- [ ] Define provider interfaces
- [ ] Define domain errors

### Phase 3: Validation
- [ ] Write tests for email validation
- [ ] Implement email validator
- [ ] Write tests for phone validation
- [ ] Implement phone validator
- [ ] Write tests for composite validator
- [ ] Implement composite validator

### Phase 4: Template System
- [ ] Write tests for template repository (file-based)
- [ ] Implement file repository
- [ ] Write tests for template service (resolve, render)
- [ ] Implement template service

### Phase 5: Notification Service
- [ ] Write tests for notification service
- [ ] Implement notification service (with mock publisher)

### Phase 6: HTTP Transport
- [ ] Write tests for notification handler
- [ ] Implement notification handler
- [ ] Write tests for template handler
- [ ] Implement template handler
- [ ] Set up Echo server with routes and middleware
- [ ] Implement health check endpoint

### Phase 7: Message Queue
- [ ] Implement RabbitMQ connection
- [ ] Implement publisher
- [ ] Wire publisher to notification service

### Phase 8: Workers
- [ ] Write tests for email processor
- [ ] Implement email processor
- [ ] Write tests for SMS processor
- [ ] Implement SMS processor
- [ ] Write tests for push processor
- [ ] Implement push processor
- [ ] Write tests for consumer loop
- [ ] Implement consumer loop

### Phase 9: Provider Integrations
- [ ] Implement SendGrid email provider
- [ ] Implement Twilio SMS provider
- [ ] Implement FCM push provider

### Phase 10: Observability
- [ ] Add structured logging throughout
- [ ] Add Prometheus metrics
- [ ] Add health checks for dependencies

### Phase 11: Integration Tests & Documentation
- [ ] Write API integration tests (testcontainers)
- [ ] Write worker integration tests (testcontainers)
- [ ] Finalize OpenAPI specification
- [ ] Generate API documentation

---

## 9. Dependencies

```go
github.com/labstack/echo/v4
github.com/rabbitmq/amqp091-go
github.com/go-playground/validator/v10
github.com/spf13/viper
github.com/rs/zerolog
github.com/prometheus/client_golang
github.com/stretchr/testify
github.com/testcontainers/testcontainers-go
github.com/sendgrid/sendgrid-go
github.com/twilio/twilio-go
firebase.google.com/go/v4
```

---

## 10. Open Questions

1. **Database**: Should we persist notification history? (PostgreSQL for audit trail)
2. **Rate Limiting**: Should API have rate limiting per client?
3. **Authentication**: API key based? JWT? OAuth2?
4. **Delivery Status Callbacks**: Webhook support for delivery status updates?

---

## Approval

Please review this plan and confirm to proceed with implementation, or request adjustments.
