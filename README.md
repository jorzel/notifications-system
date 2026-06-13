# notifications-system

A notification system built in Go that delivers email, SMS, and push notifications through third-party providers (SMTP relay, Twilio, FCM), with template-based messaging and asynchronous processing via a message queue.

See [PLAN.md](PLAN.md) for the full implementation plan.

## Architecture

```
Clients ──▶ API Server ──▶ RabbitMQ (channel × priority queues) ──▶ Workers ──▶ Providers
            validate +      email.transactional / email.bulk         render,     SendGrid,
            publish only    sms.transactional   / sms.bulk           rate-limit, Twilio,
                            push.transactional  / push.bulk          send        FCM
```

- **API server** — stateless REST API. Validates the request, checks the template exists, and publishes an event to the queue. It does *not* render templates or call providers.
- **RabbitMQ** — one queue per notification channel × priority class.
- **Workers** — consume from queues, resolve and render templates, build validated provider messages, rate-limit outbound calls, and send via the provider.

The code follows a clean architecture layout (`domain` / `app` / `infrastructure` / `transport`): all infrastructure (queue, providers, template storage) sits behind interfaces defined in the domain layer.

## How publishing works

The publish path lives in `internal/infrastructure/rabbitmq/` and implements the domain's `notification.Publisher` port. Four concepts and how they relate:

```
notification ─▶ RoutingKey("email","bulk") = "email.bulk"
                          │
   Publisher.Publish ─────┤ publishes to the exchange with that routing key,
   (over a Channel)       │ then waits for the broker's confirm
                          ▼
        direct exchange "notifications"
                          │  binding key == routing key (exact match)
                          ▼
                  queue "email.bulk"  ──▶ consumed by email workers
```

- **Connection** (`connection.go`) — a single TCP connection to the broker, holding one AMQP **channel**. Channels are the lightweight virtual connections over which all AMQP commands travel.
- **Channel** — an AMQP channel is *not* safe for concurrent use, so the publisher serializes sends behind a mutex and puts the channel into **confirm mode** at startup. (One confirming channel handles thousands of publishes/sec; a channel pool is the scaling step if the API ever outgrows it.)
- **Topology** (`topology.go`) — declared idempotently at startup: one durable **direct exchange** (`notifications`) and one durable queue per channel × priority, each bound to the exchange. `DeclareTopology` is safe to call from both the publisher and (later) the consumer.
- **RoutingKey** (`topology.go`) — `"<type>.<priority>"`, derived from the notification. The publisher publishes to the exchange with this key; a direct exchange delivers to the queue whose binding key matches exactly. The queue name equals its routing key, so `email.bulk` notifications land in the `email.bulk` queue and nowhere else.

**Reliability — publisher confirms.** `Publish` returns success only after the broker acknowledges the message, so a notification is never silently lost between API and broker. A nack, or no confirm within a bounded **publish timeout**, becomes an error → the API responds 500 and the client can retry. The timeout covers the whole operation *including the channel-lock wait*, so a hung broker can't stall the API publish path — queued publishes fail fast instead of piling up.

## How delivery works (workers)

A worker consumes one lane, renders the template, builds a validated provider message, rate-limits the call, and sends. Two design decisions shape it:

**One lane per worker process.** A worker serves exactly one `<type>.<priority>` lane, selected by config (`WORKER_TYPE` × `WORKER_PRIORITY`). Deploy one process per lane (up to six). This is what makes the priority split real at runtime: transactional lanes get dedicated capacity and each lane scales independently on its own queue depth (more replicas = more throughput), so a bulk campaign can't starve OTP delivery even under load. The alternative — one process draining both priorities of a channel — would let bulk and transactional share (and contend for) the same workers.

**Bounded retry, then dead-letter.** Processing failures are classified:

- *Permanent* (`PermanentError`) — invalid recipient, missing template, render error. Retrying can't help, so the message is logged and dropped (acked).
- *Transient* — a send failure. The message is re-published to its lane with an incremented retry count, up to `WORKER_MAX_RETRIES`. Once exhausted it's parked in a **dead-letter queue** (`notifications.dead`) for inspection/replay rather than lost or looping forever.

If the re-publish or dead-letter itself fails, the delivery is nacked for broker redelivery — so a message is never dropped on a broker hiccup. (Retries are immediate; spacing them with backoff is a noted enhancement.)

The pipeline is broker-agnostic above the adapter: the consumer depends on a small `Delivery` (Body/Ack/Nack) and `Publisher` interface, with the RabbitMQ specifics confined to `infrastructure/rabbitmq`. Rate limiting (`golang.org/x/time/rate`) smooths bursts before they hit the provider, turning an absorbed spike into a steady outbound rate instead of a wall of 429s.

## Design decisions

The design targets a **spiky load profile**: a mild baseline of transactional traffic (password resets, OTPs, order confirmations) punctuated by bursts 10–100x larger (bulk campaigns, fan-out events). That assumption drives the decisions below.

### 1. Asynchronous delivery through a queue

The API accepts a notification and returns immediately with status `pending`; delivery happens later. Under a spike, the queue is the shock absorber — the API keeps accepting requests at burst rate while workers drain at whatever rate the providers allow. Calling providers synchronously from the API would instead tie API latency and availability to the slowest external dependency exactly when traffic peaks.

### 2. Queues split by channel × priority class

Queues are split not only by channel (email/SMS/push, since they scale and fail independently) but also by priority class: `transactional` vs `bulk`. This is the most important spiky-load decision. With a single email queue, a 500k-recipient campaign enqueued ahead of a password-reset OTP delays that OTP by however long the campaign takes to drain. With separate lanes, transactional queues have dedicated workers and stay near-empty regardless of what bulk is doing. Priority is part of the API contract (`"priority": "bulk"`), defaulting to `transactional`.

### 3. Template rendering happens in the workers, not the API

Queue messages carry `template_id + template_data`, not rendered content. Three reasons:

- **Small messages under burst.** Rendered HTML bodies multiplied by a campaign-sized spike would bloat the broker; template references are tiny.
- **Cheap API hot path.** The API does validation and a publish — nothing CPU-bound — which protects the <200ms response budget while absorbing bursts.
- **Fixable in flight.** If a template is broken, fixing it applies to messages still in the queue. Pre-rendered messages would be wrong forever.

The trade-off: workers need access to the template repository, and a rendering failure is detected at delivery time rather than at the API. The API mitigates the latter by checking template existence before publishing.

### 4. Workers rate-limit providers and autoscale on queue depth

A spike absorbed by the queue must not be replayed at full speed into SendGrid or Twilio — that just converts queue depth into provider 429s and retry storms. Each worker rate-limits outbound calls per provider (`golang.org/x/time/rate`). Worker replicas scale on queue depth (e.g. KEDA in Kubernetes), so capacity follows the spike instead of being permanently provisioned for it.

### 5. Infrastructure behind domain interfaces

`Publisher`, `EmailSender`, `SMSSender`, `PushSender`, and `TemplateRepository` are interfaces defined in the domain layer; RabbitMQ, SMTP, Twilio, FCM, and file storage are implementations in `infrastructure/` (named by technology — `infrastructure/rabbitmq`, `infrastructure/provider`, …). This keeps business logic testable without external systems — and it is what makes the alternative architectures below a contained swap rather than a rewrite.

## How the design would differ under other load profiles

The spiky-load assumption is load-bearing. Had we assumed a different traffic shape, several decisions would flip.

### Mild, stable traffic (tens to hundreds of notifications per day)

The honest assessment: the current architecture would be over-built. A RabbitMQ cluster, three autoscaled worker deployments, and an API server are significant operational surface for a trickle of messages. A leaner design:

- **Postgres as the queue, no broker.** A transactional outbox table drained with `SELECT ... FOR UPDATE SKIP LOCKED` comfortably handles hundreds of messages per second — orders of magnitude above this profile. The notification record and its "queue entry" commit in one transaction, eliminating the dual-write problem RabbitMQ designs have to work around.
- **Single binary.** API and a worker goroutine pool in one process. One deployment, one datastore, no broker to operate.
- **Audit trail for free.** Persisting notification history (an open question in the current plan) falls out naturally — the outbox *is* the history table.
- **No priority lanes, no autoscaling.** With no bursts, there is nothing to absorb and nothing for bulk traffic to starve.

Because the application core only sees the `Publisher` interface, this is an `infrastructure/` and deployment change — the domain and app layers are identical, and the system could graduate to a broker later without touching them.

### Heavy constant load (sustained thousands per second)

The opposite direction: the bottlenecks move and some choices stop scaling.

- **Broker.** Deep RabbitMQ classic queues degrade; sustained high throughput favors Kafka or RabbitMQ streams — partitioned consumption (e.g. by recipient), consumer groups, batched reads, and replay for failed sends instead of per-message requeue.
- **Providers become the real ceiling.** At this volume the constraint is SendGrid/Twilio rate limits, not internal throughput. Batched provider APIs, connection pooling, and circuit breakers become mandatory rather than nice-to-have, and provider quota management becomes a first-class concern.
- **Persistence must be asynchronous.** A synchronous database write per notification becomes its own scaling problem; status tracking shifts to batched/async writes and provider delivery webhooks, with metrics carrying the operational picture.
- **Idempotency and deduplication** become critical: at millions of messages per day, redelivery duplicates that are a rounding error at low volume turn into a steady stream of double-sent notifications.
- **Capacity planning replaces autoscaling** as the primary tool. With a constant load there is no idle trough to scale down into; the fleet is sized for the steady state, and autoscaling only covers failures and deploys.

Two decisions survive all three profiles unchanged: rendering in the workers (better or neutral everywhere), and infrastructure behind domain interfaces (which is what keeps the profile choice revisable).

## Running locally

The common dev loop runs the broker in Docker and the app on the host:

```bash
make infra        # start RabbitMQ (Docker); management UI at http://localhost:15672 (guest/guest)
make run-api      # start the API on :8080 (publishes to the broker)
WORKER_TYPE=email WORKER_PRIORITY=transactional make run-worker   # drain one lane
make smoke        # POST a sample notification → 202
make infra-down   # stop the stack
```

A posted notification is accepted (202), routed to its `<type>.<priority>` queue, and drained by the matching worker. The full stack — broker, API, and all six per-lane workers — runs in Docker with `make docker-up`.

> **Delivery uses stub senders until Phase 9.** The workers consume, render, and run the full pipeline, but the provider senders (SMTP/Twilio/FCM) are no-op stubs for now — the worker's `notification delivered` log shows the lane draining. Real outbound sending arrives with the provider integrations.

## Testing

Tests are split into two tiers so the everyday loop stays fast and Docker-free, while real-infrastructure tests run on demand.

```
make test              # unit tests — no Docker, milliseconds
make test-integration  # integration tests — testcontainers, needs Docker
```

**Unit tests** (the default `go test ./...`) cover the domain (entities, value objects, constructor validation), the application services (publisher mocked at the port, real template service over an in-memory repo), and the full HTTP stack via `httptest` (routing + spec validation + handlers + error mapping). Only the architectural boundary — the `Publisher` port — is mocked; internal collaborators are real.

**Integration tests** are guarded by the `//go:build integration` tag, so they never run in the unit loop. Each spins up the real dependency with [testcontainers](https://golang.testcontainers.org/) and exercises the actual adapter — no fakes. The RabbitMQ publisher test (`internal/infrastructure/rabbitmq/publisher_integration_test.go`) starts a real broker container and asserts the two properties that only a real broker can prove:

- **Lane routing** — a published notification arrives on its `<type>.<priority>` queue, intact after a JSON round-trip.
- **Priority isolation** — a `bulk` send never leaks into the `transactional` lane. This is the system's central design claim (campaigns can't delay OTPs), so it is verified against a live broker, not asserted in prose.

This is the same shape the Phase 11 end-to-end suite will scale up: API → real RabbitMQ → real consumer → provider double, across every channel × priority, with Mailpit standing in for SMTP. See the Testing Strategy section of [PLAN.md](PLAN.md) for the full plan.

## Project status

Implementation in progress — Phases 1–8 of [PLAN.md](PLAN.md) are complete:

| # | Phase | Highlights |
|---|-------|-----------|
| 1 | Project setup | go modules, Makefile, Docker, CI tooling |
| 2 | Domain layer | notification/template/provider entities, value objects, ports |
| 3 | Validation | provider value objects (RFC 5322 email, E.164 phone, device token) |
| 4 | Template system | file repository (YAML front matter), template service |
| 5 | Notification service | constructor validation, `ValidationError`, resolve + publish |
| 6 | HTTP transport | spec-first OpenAPI + oapi-codegen, error mapping, request logging |
| 7 | Message queue | RabbitMQ adapter: direct exchange, type×priority queues, confirming publisher |
| 8 | Workers | per-lane consumer, renderer + processors, rate limiting, bounded-retry → dead-letter |

Next up: **Phase 9 — provider integrations** (SMTP/Twilio/FCM behind the sender interfaces, replacing the stub senders), then observability and the end-to-end suite.
