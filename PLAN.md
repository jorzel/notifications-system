# Notifications System - Implementation Plan

## Overview

A scalable, microservice-ready notification system built in Go that supports multiple notification channels (email, SMS, push) with template-based messaging, asynchronous processing via message queues, and clean architecture principles.

**Design target: spiky load.** The system is sized for bursty traffic — e.g. bulk campaigns causing 10–100x spikes over baseline. Three consequences shape the design:

1. **The queue is the shock absorber.** The API does minimal work (validate + publish) so it can accept bursts quickly; delivery drains at whatever rate providers allow.
2. **Priority lanes.** Queues are split by channel × priority class (`transactional` vs `bulk`) so a marketing blast never delays an OTP or password reset. Transactional queues have dedicated workers and a tight latency SLO.
3. **Workers render and rate-limit.** Queue messages carry `template_id + data`, not rendered content — messages stay small under burst, the API hot path stays cheap (<200ms), and template fixes apply to in-flight messages. Workers rate-limit calls to providers so spikes are smoothed before hitting SendGrid/Twilio/FCM, and autoscale on queue depth (e.g. KEDA).

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
│  • Template existence check                                                   │
│  • Notification event publishing (template_id + data, no rendering)          │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                  Message Queue (RabbitMQ, channel × priority)                │
│  ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐    │
│  │ email.transactional │ │  sms.transactional  │ │ push.transactional  │    │
│  │ email.bulk          │ │  sms.bulk           │ │ push.bulk           │    │
│  └─────────────────────┘ └─────────────────────┘ └─────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                    │                 │                 │
                    ▼                 ▼                 ▼
         ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
         │Email Workers │  │ SMS Workers  │  │ Push Workers │
         │ (autoscaled  │  │ (autoscaled  │  │ (autoscaled  │
         │ on q. depth) │  │ on q. depth) │  │ on q. depth) │
         └──────────────┘  └──────────────┘  └──────────────┘
           render template, rate-limit provider calls
                    │                 │                 │
                    ▼                 ▼                 ▼
         ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
         │   SendGrid   │  │    Twilio    │  │     FCM      │
         │   (SMTP)     │  │              │  │    (APNs)    │
         └──────────────┘  └──────────────┘  └──────────────┘
```

### Key Components

1. **API Server**: Stateless REST API that validates requests, checks template existence, and publishes notification events to message queues. Does no rendering — the hot path stays cheap so bursts are accepted quickly.
2. **Message Queue**: RabbitMQ for reliable, asynchronous delivery. One queue per channel × priority class (`transactional` / `bulk`) so high-volume bulk sends never delay latency-sensitive transactional notifications.
3. **Workers**: Independent processor services that consume from queues, resolve and render templates, rate-limit provider calls, and deliver notifications via third-party providers. Scaled on queue depth (e.g. KEDA); transactional queues get dedicated workers.

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
│   │   ├── openapi.yaml        # OpenAPI 3 spec — source of truth, written first
│   │   ├── generate.go         # go:generate directive for oapi-codegen
│   │   └── server.gen.go       # generated: Echo ServerInterface + request/response types
│   │
│   ├── domain/
│   │   ├── notification/           # Notification entity
│   │   │   ├── notification.go     # Notification entity
│   │   │   ├── recipient.go        # Recipient value object
│   │   │   ├── message.go          # NotificationMessage (queue payload)
│   │   │   ├── errors.go           # Notification-specific errors, ValidationError
│   │   │   ├── publisher.go        # Publisher interface
│   │   │   └── mocks/              # generated mocks (uber-go/mock, via make generate)
│   │   │
│   │   ├── template/               # Template entity
│   │   │   ├── template.go         # Template entity with Render() method
│   │   │   ├── errors.go           # Template-specific errors
│   │   │   └── repository.go       # TemplateRepository interface
│   │   │
│   │   └── provider/               # Provider value objects and interfaces
│   │       ├── email.go            # EmailAddress, EmailMessage, EmailSender
│   │       ├── email_test.go
│   │       ├── sms.go              # PhoneNumber, SMSMessage, SMSSender
│   │       ├── sms_test.go
│   │       ├── push.go             # DeviceToken, PushMessage, PushSender
│   │       └── push_test.go
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
│   │       ├── service.go          # TemplateService (resolve, list)
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
│   └── transport/
│       ├── http/
│       │   ├── handler/
│       │   │   ├── notification.go
│       │   │   ├── notification_test.go
│       │   │   ├── template.go
│       │   │   ├── template_test.go
│       │   │   └── health.go
│       │   ├── middleware/
│       │   │   ├── logging.go
│       │   │   └── recovery.go
│       │   └── server.go       # request/response types come generated from the spec
│       │
│       └── worker/
│           ├── consumer.go
│           └── consumer_test.go
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
│           └── default.txt
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

### Notification Entity (`domain/notification/`)

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

// Priority selects the queue lane. Bulk sends (campaigns) must never
// delay transactional notifications (OTP, password reset).
type Priority string

const (
    PriorityTransactional Priority = "transactional"
    PriorityBulk          Priority = "bulk"
)

type Notification struct {
    ID           string
    Type         NotificationType
    Priority     Priority
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
// Carries the template reference, NOT rendered content. Rendering happens
// at the worker: messages stay small under burst, the API hot path stays
// cheap, and template fixes apply to in-flight messages.
type Message struct {
    ID           string
    Type         NotificationType
    Priority     Priority
    Recipient    Recipient
    TemplateID   string
    TemplateData map[string]any
    Metadata     map[string]string
    CreatedAt    time.Time
    RetryCount   int
}

// RenderedContent is produced by Template.Render at the worker and
// consumed by processors. It never travels over the queue.
// Fields usage by notification type:
//   - Email: Subject (subject line), Body (HTML content)
//   - SMS:   Body (text message), Subject and Data ignored
//   - Push:  Subject (title), Body (body text), Data (extra payload)
type RenderedContent struct {
    Subject string
    Body    string
    Data    map[string]string
}
```

```go
// publisher.go
type Publisher interface {
    Publish(ctx context.Context, msg *Message) error
}
```

Validation happens in the constructor — there is no separate Validator. `New`
returns an error for an unknown type or priority, or a recipient that doesn't
match the channel (format checked via the provider value objects). This keeps
the entity always-valid, the same principle as the provider value objects.
Every violation is wrapped in `*ValidationError` (with the offending field),
which the transport layer maps to HTTP 400 via `errors.As`:

```go
// notification.go
func New(notifType Type, priority Priority, recipient Recipient,
    templateID string, templateData map[string]any) (*Notification, error)

// errors.go
type ValidationError struct {
    Field string // e.g. "type", "recipient.email"
    Err   error
}
```

### Template Entity (`domain/template/`)

```go
// template.go
type Template struct {
    ID        string
    Type      notification.Type
    Name      string
    Subject   string  // Go template for email subject / push title
    Body      string  // Go template for body (html/template for email, text/template for SMS/push)
    IsDefault bool
}

// Render processes the template with data and returns rendered content
func (t *Template) Render(data map[string]any) (*notification.RenderedContent, error)

// repository.go
// Templates are scoped by notification type: the same ID (e.g. "default")
// may exist independently for each channel.
type Repository interface {
    Get(ctx context.Context, notificationType notification.Type, id string) (*Template, error)
    GetDefault(ctx context.Context, notificationType notification.Type) (*Template, error)
    List(ctx context.Context) ([]*Template, error)
}
```

### Provider Value Objects and Interfaces (`domain/provider/`)

Provider value objects encapsulate validation. You cannot create an invalid message.

```go
// email.go - Value objects with self-validation
type EmailAddress struct { value string }
func NewEmailAddress(email string) (EmailAddress, error)  // validates RFC 5322

type EmailMessage struct { to EmailAddress; subject, body string }
func NewEmailMessage(to EmailAddress, subject, body string) (EmailMessage, error)

type EmailSender interface {
    Send(ctx context.Context, message EmailMessage) error
}

// sms.go - Phone number normalized to E.164
type PhoneNumber struct { value string }
func NewPhoneNumber(number string) (PhoneNumber, error)  // normalizes & validates E.164

type SMSMessage struct { to PhoneNumber; text string }
func NewSMSMessage(to PhoneNumber, text string) (SMSMessage, error)

type SMSSender interface {
    Send(ctx context.Context, message SMSMessage) error
}

// push.go - Device token validation
type DeviceToken struct { value string }
func NewDeviceToken(token string) (DeviceToken, error)

type PushMessage struct { to DeviceToken; title, body string; data map[string]string }
func NewPushMessage(to DeviceToken, title, body string, data map[string]string) (PushMessage, error)

type PushSender interface {
    Send(ctx context.Context, message PushMessage) error
}
```

---

## 4. Application Layer

### Notification Service (`app/notification/service.go`)

```go
type Service struct {
    publisher   notification.Publisher
    templateSvc *template.Service
}

func (s *Service) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
    // 1. Default priority to transactional if unset
    // 2. Create notification (constructor validates type, priority, recipient)
    // 3. Resolve template and stamp its ID (no rendering — that happens at the worker)
    // 4. Publish to queue (routing key: <type>.<priority>)
}
```

Structured logging is added across services in Phase 10.

```go
```

### Processors (`app/notification/`)

Processors resolve and render the template, create validated provider messages, and delegate to sender interfaces. Provider calls go through a rate limiter so traffic spikes are smoothed before hitting external APIs.

```go
// email_processor.go
type EmailProcessor struct {
    templateSvc *template.Service
    sender      provider.EmailSender
    limiter     *rate.Limiter // golang.org/x/time/rate, per-provider limit
    logger      zerolog.Logger
}

func (p *EmailProcessor) Process(ctx context.Context, msg *notification.Message) error {
    // Resolve and render template (rendering happens at the worker)
    tmpl, err := p.templateSvc.Resolve(ctx, msg.TemplateID, msg.Type)
    if err != nil {
        return err
    }

    content, err := tmpl.Render(msg.TemplateData)
    if err != nil {
        return err
    }

    // Create validated EmailAddress (validation happens here)
    emailAddr, err := provider.NewEmailAddress(msg.Recipient.Email)
    if err != nil {
        return err
    }

    // Create validated EmailMessage
    emailMsg, err := provider.NewEmailMessage(emailAddr, content.Subject, content.Body)
    if err != nil {
        return err
    }

    // Smooth bursts before they hit the provider's rate limits
    if err := p.limiter.Wait(ctx); err != nil {
        return err
    }

    return p.sender.Send(ctx, emailMsg)
}
```

### Template Service (`app/template/service.go`)

Template rendering is on the Template entity itself. Service handles resolution and listing.

```go
type Service struct {
    repo template.Repository
}

func (s *Service) Resolve(ctx context.Context, templateID string, notificationType notification.Type) (*template.Template, error)
func (s *Service) List(ctx context.Context) ([]*template.Template, error)

// Note: Rendering is done via Template.Render(data) method on the entity
```

---

## 5. API Design

**Spec-first.** `internal/api/openapi.yaml` is written before any handler code and is the single source of truth for the HTTP API. From it, [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) generates the Echo `ServerInterface` and all request/response types (`make generate`); handlers are implemented against the generated interface, and middleware validates incoming requests against the spec. Clients (other microservices) generate their own clients from the same file. The spec, the server, and the docs cannot drift apart.

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/notifications` | Send a notification |
| POST | `/api/v1/notifications/batch` | Send multiple notifications |
| GET | `/api/v1/templates` | List available templates (optional `?type=` filter) |
| GET | `/api/v1/templates/{type}/{id}` | Get template details (IDs are type-scoped: `default` exists per channel) |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

### Send Notification Request

```json
POST /api/v1/notifications
{
    "type": "email",
    "priority": "transactional",
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

`priority` is optional and defaults to `transactional`. Bulk senders (campaigns) must set `"priority": "bulk"` so they are routed to the bulk lane and cannot delay transactional traffic.

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

    // API routes: handlers implement the generated api.ServerInterface,
    // routes are registered from the spec, requests validated against it
    apiGroup := e.Group("/api/v1")
    apiGroup.Use(oapimiddleware.OapiRequestValidator(spec))
    api.RegisterHandlersWithBaseURL(e, handler.New(notifSvc, tmplSvc, logger), "/api/v1")

    return e
}
```

### Error Mapping (domain → HTTP)

Handlers return domain errors as-is; a single error mapper in `transport/http`
(Echo `HTTPErrorHandler`) translates them. The response body shape is the
`ErrorResponse` schema in the OpenAPI spec (`code`, `message`, `field`).

| Domain error | Detected via | HTTP |
|---|---|---|
| `*notification.ValidationError` | `errors.As` | 400 (body includes `field`) |
| `template.ErrTemplateNotFound`, `ErrNoDefaultTemplate` | `errors.Is` | 400 on `POST /notifications` (bad reference in body), 404 on `GET /templates/{type}/{id}` |
| anything else | fallback | 500, generic message — details only in logs |

Malformed JSON, missing required fields, and enum violations never reach the
domain: the oapi-codegen validation middleware rejects them with 400 based on
the spec. Domain validation covers what the schema can't express (RFC 5322,
E.164, recipient-matches-channel).

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
- [x] Initialize go.mod, Makefile, Docker files
- [x] Set up linting (golangci-lint)
- [x] Configure test framework (testify)

### Phase 2: Domain Layer
- [x] Define notification entity and value objects
- [x] Define template entity with Render() method
- [x] Define provider value objects and interfaces
- [x] Define domain errors

### Phase 3: Validation (in provider value objects)
- [x] Write tests for EmailAddress and EmailMessage validation
- [x] Implement email validation (RFC 5322 with net/mail)
- [x] Write tests for PhoneNumber and SMSMessage validation
- [x] Implement phone validation (E.164 with normalization)
- [x] Write tests for DeviceToken and PushMessage validation
- [x] Implement device token validation

### Phase 4: Template System
- [x] Write tests for template repository (file-based)
- [x] Implement file repository (directory per type, optional YAML front matter for name/subject)
- [x] Write tests for template service (resolve with default fallback, list)
- [x] Implement template service

### Phase 5: Notification Service
- [x] Move validation into the Notification constructor (always-valid entity, `ValidationError` for HTTP 400 mapping); drop the Validator interface
- [x] Write tests for notification service (validate, template check, publish — no rendering)
- [x] Implement notification service (mock publisher generated with uber-go/mock via `make generate`)

### Phase 6: HTTP Transport (spec-first)
- [ ] Write OpenAPI spec (`internal/api/openapi.yaml`): all endpoints, request/response schemas (incl. `priority` enum defaulting to `transactional`), error responses; template paths are type-scoped (`/templates/{type}/{id}`, list filterable by `?type=`)
- [ ] Add Makefile `generate` target running oapi-codegen on the spec (server interface + types into `internal/api/server.gen.go`, via `go:generate`); commit generated code
- [ ] Write tests for notification handler
- [ ] Implement notification handler (implements generated `ServerInterface`)
- [ ] Write tests for template handler
- [ ] Implement template handler
- [ ] Set up Echo server: routes registered from generated code, spec validation middleware, logging/recovery middleware
- [ ] Implement health check endpoint

### Phase 7: Message Queue
- [ ] Implement RabbitMQ connection
- [ ] Declare channel × priority topology (`<type>.<transactional|bulk>` queues)
- [ ] Implement publisher (routing key from notification type + priority)
- [ ] Wire publisher to notification service

### Phase 8: Workers
- [ ] Write tests for email processor (resolve, render, validate, rate-limit, send)
- [ ] Implement email processor
- [ ] Write tests for SMS processor
- [ ] Implement SMS processor
- [ ] Write tests for push processor
- [ ] Implement push processor
- [ ] Write tests for consumer loop
- [ ] Implement consumer loop
- [ ] Per-provider rate limiting (golang.org/x/time/rate), limits from config

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
- [ ] Serve the OpenAPI spec (Swagger UI or `/openapi.yaml` endpoint) — the spec itself already exists from Phase 6

---

## 9. Dependencies

```go
github.com/labstack/echo/v4
github.com/oapi-codegen/oapi-codegen/v2   // codegen tool, invoked via make generate
github.com/oapi-codegen/runtime           // runtime helpers for generated code
github.com/oapi-codegen/echo-middleware   // request validation against the spec
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
golang.org/x/time
```

---

## 10. Open Questions

1. **Database**: Should we persist notification history? (PostgreSQL for audit trail)
2. **API Rate Limiting**: Should the API have rate limiting per client? (Provider-side rate limiting in workers is already part of the design.)
3. **Authentication**: API key based? JWT? OAuth2?
4. **Delivery Status Callbacks**: Webhook support for delivery status updates?

---

## Approval

Please review this plan and confirm to proceed with implementation, or request adjustments.
