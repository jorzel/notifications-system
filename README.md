# notifications-system

A notification system built in Go that delivers email, SMS, and push notifications through third-party providers (SendGrid, Twilio, FCM), with template-based messaging and asynchronous processing via a message queue.

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

`Publisher`, `EmailSender`, `SMSSender`, `PushSender`, and `TemplateRepository` are interfaces defined in the domain layer; RabbitMQ, SendGrid, Twilio, FCM, and file storage are implementations in `infrastructure/`. This keeps business logic testable without external systems — and it is what makes the alternative architectures below a contained swap rather than a rewrite.

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

## Project status

Implementation in progress — Phases 1–3 of [PLAN.md](PLAN.md) are complete (project setup, domain layer, provider value-object validation). Next up: template system, notification service, HTTP transport, queue, and workers.
