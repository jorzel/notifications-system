
## Key Principles
1. Simplicity and readability over cleverness
2. Explicit is better than implicit
3. Fail fast with clear error messages
4. Handle all errors explicitly (Go) / use exceptions appropriately (Python)
5. Test-driven development
6. Consistent code formatting and style
7. Comprehensive documentation for public APIs and complex logic
8. Modular design to promote reusability
9. Use version control effectively with meaningful commit messages
10. Domain-Driven Design (DDD) principles where applicable
11. Application logic separated from transport and infrastructure concerns. Use interfaces instead of direct implementation.
12. Use dependency injection for better testability
13. Observability: logging, metrics, tracing
14. Integration tests using testcontainers or similar tools
15. For API use openapi/Swagger for documentation and client generation (and server is possible)


## Critical Constraints
- Performance:
    - API responses must be <200ms
- Security:
    - Never log sensitive data"
    - Keep secrets out of source code
- Compatibility:
    - Go 1.23

## Project Structure
- **Go**:
    - `/cmd`: Main applications
    - `/internal`: Private application and library code
        - `/api`: API definitions
        - `/app`: Application logic
        - `/domain`: Domain models and business logic
        - `/infrastructure`: External systems (DB, messaging, etc.)
        - `/transport`: Incoming transport layer (HTTP, gRPC, etc.)
    - `/configs`: Configuration files
    - `/integration_tests`: Integration tests


## Coding Style Guide

### Follow Standard Go Conventions
- Run `gofmt` on all code
- Use `golangci-lint` for linting
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### Naming
```go
// Exported names: PascalCase
type UserService struct {}
func NewUserService() *UserService {}

// Unexported names: camelCase
type userRepository struct {}
func calculateTotal() int {}

// Acronyms: all caps or all lowercase
type HTTPAPI struct {}  // exported
type httpClient struct {} // unexported

// Constants: PascalCase or SCREAMING_SNAKE_CASE
const MaxRetries = 3
const DEFAULT_TIMEOUT = 30 * time.Second

// Interface names: -er suffix when possible
type Reader interface {}
type UserCreator interface {}
```

### Error Handling
```go
// Always check errors immediately
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Use custom error types for domain errors
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Use errors.Is and errors.As for checking
if errors.Is(err, ErrNotFound) {
    // handle not found
}

var valErr *ValidationError
if errors.As(err, &valErr) {
    // handle validation error
}
```

### Function Design
```go
// Keep functions short and focused
// Accept interfaces, return structs
func ProcessOrder(repo OrderRepository, id string) (*Order, error) {
    // ...
}

// Use functional options for complex constructors
type ServerOption func(*Server)

func WithTimeout(d time.Duration) ServerOption {
    return func(s *Server) {
        s.timeout = d
    }
}

func NewServer(opts ...ServerOption) *Server {
    s := &Server{
        timeout: 30 * time.Second, // default
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

### Context Usage
```go
// Always pass context as first parameter
func FetchUser(ctx context.Context, id string) (*User, error) {
    // Use context for cancellation and deadlines
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case result := <-ch:
        return result, nil
    }
}

// Don't store context in structs (except rare cases)
```

### Concurrency
```go
// Use channels for communication
// Use sync primitives for synchronization
// Always use sync.WaitGroup or errgroup for goroutine coordination

import "golang.org/x/sync/errgroup"

func ProcessBatch(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item // capture loop variable
        g.Go(func() error {
            return processItem(ctx, item)
        })
    }

    return g.Wait()
}
```

## Testing Overview

### Testing Best Practices
- **Write tests first**: Define expected behavior
- **Test behavior, not implementation**
- **Arrange, Act, Assert**: Structure tests clearly
- **Isolate tests**: No shared state, use mocks/fakes
- **Table-driven tests**: Cover multiple scenarios concisely
- **Fast feedback**: Unit tests <5s, integration <30s
- **Clear failures**: Test names explain what broke
- **Realistic data**: No "foo", "bar", "test" values


## Go Testing Standards

### Test Organization
```go
// _test.go files in same package
package service

import "testing"

// Table-driven tests for multiple scenarios
func TestUserService_CreateUser(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
        errType error
    }{
        {
            name:    "creates user with valid email",
            email:   "user@example.com",
            wantErr: false,
        },
        {
            name:    "returns error for invalid email",
            email:   "invalid",
            wantErr: true,
            errType: ErrInvalidEmail,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := NewUserService(newMockRepo())
            user, err := svc.CreateUser(context.Background(), tt.email)

            if (err != nil) != tt.wantErr {
                t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if tt.wantErr && !errors.Is(err, tt.errType) {
                t.Errorf("expected error type %v, got %v", tt.errType, err)
            }

            if !tt.wantErr && user.Email != tt.email {
                t.Errorf("expected email %v, got %v", tt.email, user.Email)
            }
        })
    }
}
```

### Integration Tests
```go
// Place in tests/ directory or use build tags
// +build integration

package tests

func TestCreateOrder_Integration(t *testing.T) {
    // Setup real database
    db := setupTestDB(t)
    defer db.Close()

    repo := repository.NewPostgresOrderRepository(db)
    svc := service.NewOrderService(repo)

    // Test actual behavior
    order, err := svc.CreateOrder(context.Background(), &service.CreateOrderRequest{
        UserID: "user_123",
        Items:  []service.OrderItem{{ProductID: "prod_1", Quantity: 2}},
    })

    require.NoError(t, err)
    assert.NotEmpty(t, order.ID)

    // Verify in database
    found, err := repo.FindByID(context.Background(), order.ID)
    require.NoError(t, err)
    assert.Equal(t, order.ID, found.ID)
}
```

### Test Helpers
```go
// Use test helpers to reduce boilerplate
func newTestUserService(t *testing.T) *UserService {
    t.Helper()
    return NewUserService(newMockRepo())
}

// Use testify require for setup that must succeed
import "github.com/stretchr/testify/require"

func TestSomething(t *testing.T) {
    db := setupDB(t)
    require.NotNil(t, db, "database setup must succeed")

    // Continue with test
}
```

## Observability Guidelines

### Structured Logging

```go
import "github.com/rs/zerolog"

logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

// Log with structured fields
logger.Info().Str("order_id", Order.id).Msg("order created")

// Log errors with context
logger.Err(err).Msg("payment failed")

// Add context to logger
logger = logger.With("service", "order-service", "version", "1.0.0")
```

## Log Levels
- **ERROR**: System failures requiring immediate attention
- **WARN**: Degraded functionality, fallback engaged
- **INFO**: Key business events to track flow (order placed, user registered)
- **DEBUG**: Detailed troubleshooting (not in production)

---

### What to Log

#### ✅ DO Log
```go
// Business events
logger.Info("order created", "order_id", id, "total", total)

// External API calls
logger.Info("payment processed",
    "provider", "stripe",
    "duration_ms", duration,
    "success", true,
)

// Errors with full context
logger.Error("database query failed",
    "error", err,
    "query", "INSERT INTO orders",
    "user_id", userID,
)
```

#### ❌ DON'T Log
- Sensitive data (passwords, tokens, credit cards, PII)
- Inside tight loops (aggregate instead)
- Redundant info already captured in metrics

---

### Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    ordersCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "orders_created_total",
            Help: "Total number of orders created",
        },
        []string{"status"},
    )

    orderDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "order_creation_duration_seconds",
            Help: "Order creation duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"endpoint"},
    )
)
```

### Health Checks

```go
func (s *Server) HealthCheck(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    health := map[string]string{
        "status": "healthy",
    }

    // Check database
    if err := s.db.PingContext(ctx); err != nil {
        health["status"] = "unhealthy"
        health["database"] = "unreachable"
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(health)
}
```

### Dashboards
- Create dashboards in Grafana to visualize key metrics:
  - Request rates, error rates, latencies
  - Business metrics (orders created, revenue)
  - System health (CPU, memory, DB connections)
- Use python libraries like `grafanalib` to define dashboards as code for versioning and reproducibility.