# Environment Variables Reference

## Overview

Whatsapp Gateway is fully configured through environment variables.

There is no mandatory configuration file. All runtime behavior is controlled via environment variables that can be supplied through:

- Docker `-e` flags
- Docker Compose `env_file`
- System environment variables
- Process managers (e.g., systemd)

This document provides a complete reference of all supported configuration options.

## Core Configuration

### ENV

Environment mode.

Type: string  
Default: production  
Options: development, production  

Controls general runtime behavior and logging verbosity.

### PORT

HTTP server port.

Type: string  
Default: 3000  

Defines the internal port used by the gateway.

### BASE_PATH

Base path prefix for all routes.

Type: string  
Default: /  

Useful when running behind a reverse proxy under a subpath.

Example:

/api/v1

### HTTP_ORIGIN

CORS allowed origin.

Type: string  
Default: *  

Set explicitly in production to restrict cross-origin access.

## Documentation Configuration

### ENABLE_DOCUMENTATION

Enable Documentation UI.

Type: boolean  
Default: false  

Should be disabled in hardened production environments.

### DOCUMENTATION_USER

Documentation basic auth username.

Type: string  
Default: user  

### DOCUMENTATION_PASSWORD

Documentation basic auth password.

Type: string  
Default: password  

### Documentation_BASE_PATH

Documentation documentation path.

Type: string  
Default: /docs  

## Authentication Configuration

### JWT_SECRET

JWT signing secret.

Type: string  
Default: secret  

Must be changed in production.

### JWT_TOKEN_DURATION_MINUTES

JWT expiration duration.

Type: integer  
Default: 1440  

Defines how long access tokens remain valid. Clamped to 1..525600 (1 minute..1 year); out-of-range values reset to the 1440 default.

### JWT_ISSUER

JWT issuer identifier.

Type: string  
Default: whatsapp-gateway  

### SECRET_KEY

Basic authentication secret key.

Type: string  
Default: secret  

Should be replaced in production.

## WhatsApp Configuration

### WHATSAPP_DATASTORE_TYPE

Datastore backend type.

Type: string
Default: sqlite
Options: sqlite, postgres

SQLite (modernc.org/sqlite - pure Go) is suitable for development and production.
PostgreSQL is recommended for high-scale production deployments.

### WHATSAPP_DATASTORE_URI

Datastore connection URI.

Type: string  

SQLite default:

file:dbs/whatsapp.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)  

PostgreSQL example:

postgres://user:password@host:5432/dbname?sslmode=disable  

### WHATSMEOW_LOG_LEVEL

Log level for underlying WhatsApp client library.

Type: string  
Default: warn  

Controls verbosity of WhatsApp client internals.

### WHATSAPP_DEVICE_LABEL

Device label shown in WhatsApp linked devices.

Type: string  
Default: WhatsApp Gateway  

### WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY

HMAC secret for webhook signature generation.

Type: string  
Default: 32-character placeholder  

Must be a strong 32-byte secret in production.

## Logging Configuration

### LOG_LEVEL

Application log level.

Type: string  
Default: info  
Options: debug, info, warn, error  

Use info or warn in production.

### LOG_OUTPUT

Log output target.

Type: string  
Default: stdout  
Options: stdout, file  

### LOG_FILE_PATH

Log file path (if LOG_OUTPUT=file).

Type: string  
Default: /var/log/whatsapp-gateway.log  

### LOG_ENABLE_CALLER

Enable caller information in logs.

Type: boolean  
Default: false  

Useful for debugging.

## Rate Limiting

### MESSAGE_RATE_LIMIT_PROVIDER

Rate limit backend.

Type: string  
Default: memory  
Options:
- memory
- redis
- noop

Memory is per-instance.  
Redis allows distributed rate limiting.

### MESSAGE_RATE_LIMIT_REQUESTS

Allowed requests per window.

Type: integer  
Default: 100  

### MESSAGE_RATE_LIMIT_DURATION_SECONDS

Window duration in seconds.

Type: integer  
Default: 60  

### REGISTER_RATE_LIMIT_ENABLED

Toggle per-IP throttling on `POST /register`.

Type: boolean  
Default: true  

Over-budget requests get `429 Too Many Requests` with a `Retry-After` header. The limiter fails open on internal errors.

### REGISTER_RATE_LIMIT_REQUESTS

Maximum registrations per IP per window.

Type: integer  
Default: 5  

### REGISTER_RATE_LIMIT_DURATION_SECONDS

Register throttle window in seconds.

Type: integer  
Default: 60  

## Outbound Pacing

A single in-process **pacer** governs every outbound WhatsApp action (all `POST /message/*` sends, react/edit/delete, mark-read, typing/presence, and all group/community mutations) in both direct and queue modes, in three ordered layers: a **ban gate** (429 while the account is under a WhatsApp temporary ban, auto-resume), a **per-recipient hard cap** (429, never paced), and a **per-account token-bucket pace** (blocks/waits in `pace` mode, or 429s immediately in `reject` mode). This is the **primary** outbound governor; `MESSAGE_RATE_LIMIT_*` is the **fallback**, used only when `OUTBOUND_PACE_ENABLED=false`. The pacer is per-instance (single-node), not distributed: a multi-instance deployment needs an external limiter or the queue for a cluster-wide ceiling.

### OUTBOUND_PACE_ENABLED

Master toggle for the outbound pacer. When `false`, the pacer is bypassed and sends fall back to the reject-only `MESSAGE_RATE_LIMIT_*` limiter.

Type: boolean  
Default: true  

### OUTBOUND_PACE_MODE

`pace` blocks/waits for a token (up to `OUTBOUND_PACE_MAX_WAIT_SECONDS` + jitter) before returning `429`; `reject` never waits: an over-budget call is an immediate `429`.

Type: string  
Default: pace  
Options:
- pace
- reject

### OUTBOUND_PACE_RATE_PER_SECOND

Sustained per-account token-bucket refill rate (tokens per second). Accepts fractional values (e.g. 0.5 = one send every 2s).

Type: float  
Default: 1  

### OUTBOUND_PACE_BURST

Token-bucket burst capacity: how many actions can fire back-to-back before the sustained rate applies.

Type: number  
Default: 5  

### OUTBOUND_PACE_MAX_WAIT_SECONDS

In `pace` mode, the maximum time a call blocks waiting for a token before giving up with `429`.

Type: integer (seconds)  
Default: 30  

### OUTBOUND_PACE_JITTER_MS

Upper bound of random jitter added to the pace wait/deadline, so sends don't align on exact tick boundaries.

Type: integer (milliseconds)  
Default: 250  

### OUTBOUND_PACE_PER_RECIPIENT_REQUESTS

Per-recipient hard cap: more than this many actions to the **same** recipient within `OUTBOUND_PACE_PER_RECIPIENT_WINDOW_SECONDS` is rejected with `429` (never paced/queued).

Type: integer  
Default: 10  

### OUTBOUND_PACE_PER_RECIPIENT_WINDOW_SECONDS

The rolling window over which `OUTBOUND_PACE_PER_RECIPIENT_REQUESTS` is counted.

Type: integer (seconds)  
Default: 60  

### OUTBOUND_PACE_BAN_DEFAULT_HOLD_SECONDS

Fallback hold applied by the ban gate when a temporary ban carries no explicit `ban_expires_at`: outbound actions are `429`d for this long before auto-resuming.

Type: integer (seconds)  
Default: 3600  

## Upload Limits

### MAX_UPLOAD_BYTES

Maximum size of an outbound media upload (image/audio/video/document/sticker), checked before the file is read into memory.

Type: integer (bytes)  
Default: 16777216 (16 MiB)  

Image/sticker/audio/video uploads are also validated against a per-kind MIME allow-list; documents accept any mimetype. PTT voice notes opt out of MIME sniffing (opus/ogg is unidentifiable).

## Read / Query Surface

Server-hitting reads (joined groups, and later profiles/avatars) are short-TTL cached and metered by a per-account budget, so a polling caller can't trip WhatsApp anti-spam. A budget token is spent **only on a cache miss**; cache hits are free. Local-store reads (e.g. `GET /contact/`) are never metered.

### READ_QUERY_CACHE_TTL_SECONDS

How long a server-hitting read is cached before the next call re-fetches.

Type: integer (seconds)  
Default: 300  

### READ_QUERY_BUDGET

Maximum cache-miss reads per account per window before requests get `429 Too Many Requests` (with `Retry-After`).

Type: integer  
Default: 30  

### READ_QUERY_WINDOW_SECONDS

The rolling window over which `READ_QUERY_BUDGET` is counted.

Type: integer (seconds)  
Default: 60  

## Group & Community Management

Group/community **mutations** are the highest-ban-risk surface, gated default-safe. **Reads** stay available regardless of these settings.

### GROUP_MANAGEMENT_ENABLED

Master toggle. When `false` the entire mutation/invite/join-request/community surface is **unregistered → 404** (hidden); reads stay up.

Type: boolean  
Default: true  

### GROUP_ADD_PARTICIPANTS_ENABLED

Gates bulk participant add (`POST /group/participants action=add` and add-on-create) → `403` when off. Now defaults **on**: the outbound pacer + ban gate cover the bulk-add ban risk this gate guarded as an interim measure; set it to `false` to hard-disable bulk add.

Type: boolean  
Default: true  

### GROUP_JOIN_VIA_LINK_ENABLED

Gates `POST /group/join`, the mass-join vector → `403` when off. Now defaults **on**: outbound pacing + the ban gate cover the mass-join ban risk; set it to `false` to hard-disable join-via-link.

Type: boolean  
Default: true  

### GROUP_MAX_PARTICIPANTS_PER_REQUEST

Caps how many participants a single batch may carry; over-cap → `400`. `0` disables the cap.

Type: integer  
Default: 256  

## Send Idempotency

Send endpoints (`/api/message/*`) accept an optional `Idempotency-Key` header. A duplicate key replays the original response (`Idempotent-Replay: true`) instead of sending again; an in-flight duplicate gets `409`; the same key with a different request body gets `422`. Dedup is DB-backed and keyed by the JWT phone number + key, so it survives restarts. In queue mode this guarantees enqueued-once, not delivered-once.

### IDEMPOTENCY_TTL_SECONDS

How long a completed response stays replayable (and the retention bound a background sweeper enforces).

Type: integer (seconds)  
Default: 86400 (24h)  

### IDEMPOTENCY_PENDING_TIMEOUT_SECONDS

If a request crashes after reserving a key but before completing, its row is left `pending`. After this timeout a retry may take the key over instead of getting a `409` forever.

Type: integer (seconds)  
Default: 30  

## Graceful Shutdown

### SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS

Overall bound for cleanly disconnecting all WhatsApp clients on shutdown, before the database is closed. A hung client is skipped so shutdown never blocks past this.

Type: integer (seconds)  
Default: 10  

## Admin / Metrics Plane

An operator-only, cross-tenant plane at the ROOT path (outside `/api/v1`): `GET /admin/sessions`, `GET /admin/sessions/{phone}`, `GET /metrics`. Dark by default: with no `ADMIN_API_SECRET` the routes are unregistered and return `404` (never a `401`). The session inventory is per-instance and phones are masked; metrics are never labelled by phone number.

### ADMIN_API_SECRET

Bearer secret for `/admin/*` and `/metrics`. Empty disables the whole plane. When set, requests send `Authorization: Bearer <secret>` (constant-time compare).

Type: string  
Default: "" (disabled)  

### METRICS_ENABLED

Expose `GET /metrics` (hand-rolled Prometheus text: `whatsapp_gateway_messages_total`, `whatsapp_gateway_webhook_deliveries_total`, `whatsapp_gateway_sessions{state}`). Still requires `ADMIN_API_SECRET` to be reachable.

Type: boolean  
Default: false  

## Direct-mode Webhook Retry

Direct-mode status webhooks are delivered asynchronously with bounded exponential backoff on a detached context (best-effort; queue mode keeps RabbitMQ retry).

### WEBHOOK_MAX_RETRIES

Maximum retry attempts (beyond the first) for a direct-mode webhook delivery. Capped at 10.

Type: integer  
Default: 3  

### WEBHOOK_RETRY_BACKOFF_SECONDS

Base backoff (seconds) for the exponential retry schedule.

Type: integer (seconds)  
Default: 2  

## RabbitMQ Configuration

### RABBITMQ_ENABLED

Enable queue mode.

Type: boolean  
Default: false  

When enabled, outgoing messages are processed asynchronously.

### RABBITMQ_URL

RabbitMQ connection URI.

Type: string  
Default: amqp://user:user@localhost:5672/  

### RABBITMQ_CONNECTION_NAME

Connection name identifier.

Type: string  
Default: whatsapp-gateway  

### RABBITMQ_PREFETCH_COUNT

Consumer prefetch count.

Type: integer  
Default: 5  

Controls number of unacknowledged messages per worker.

## Redis Configuration

### REDIS_ENABLED

Enable Redis usage.

Type: boolean  
Default: false  

Required when using Redis-based rate limiting.

### REDIS_URI

Redis connection URI.

Type: string  
Default: redis://localhost:6379/0  

## Worker Pool Configuration

### WORKER_INCOMING_EVENTS

Number of workers handling incoming WhatsApp events.

Type: integer  
Default: 5  

### WORKER_WEBHOOK_DELIVERY

Number of workers delivering webhooks.

Type: integer  
Default: 10  

### WORKER_OUTGOING_MESSAGES

Number of workers processing outgoing messages.

Type: integer  
Default: 3  

Increasing worker count increases concurrency but also resource usage.

## Queue Retry Configuration

### QUEUE_MAX_RETRIES

Maximum retry attempts for failed queue messages.

Type: integer  
Default: 3  

Messages exceeding retry limit are marked as failed.

## Status Webhook Configuration

### WEBHOOK_STATUS_EVENTS_ENABLED

Enable status event webhooks.

Type: boolean  
Default: true  

### WEBHOOK_STATUS_EVENTS

Deprecated: comma-separated list of status events. No longer applied as a delivery filter: which events fire is controlled per-subscription via `POST /webhook` (`events[]`). Retained only so existing `.env` files still parse.

Type: string  
Default: message.sent,message.failed  

Defines which message lifecycle events trigger webhooks.

## Storage Configuration

### STORAGE_PROVIDER

Storage backend provider.

Type: string
Default: local
Options:
- local: Filesystem-based storage (production-ready, full data control)
- s3: S3/S3-compatible object storage (AWS S3, MinIO, DigitalOcean Spaces, etc.)

Both providers are production-ready. Choose based on infrastructure needs.

### STORAGE_S3_ENDPOINT

S3/S3-compatible service endpoint.

Type: string
Default: s3.amazonaws.com

AWS S3: s3.amazonaws.com
MinIO: localhost:9000
DigitalOcean Spaces: nyc3.digitaloceanspaces.com

### STORAGE_S3_ACCESS_KEY_ID

S3 access key ID.

Type: string
Default: ""

Required for S3 provider. Can be omitted for local provider.

### STORAGE_S3_SECRET_ACCESS_KEY

S3 secret access key.

Type: string
Default: ""

Required for S3 provider. Can be omitted for local provider.

### STORAGE_S3_REGION

S3 region.

Type: string
Default: us-east-1

Required for S3 provider.

### STORAGE_S3_BUCKET

S3 bucket name.

Type: string
Default: whatsapp-gateway

Required for S3 provider. Bucket must exist or be creatable.

### STORAGE_S3_USE_SSL

Use SSL/TLS for S3 connections.

Type: boolean
Default: true

Should be true for production. May be false for local MinIO testing.

### STORAGE_S3_PRESIGNED_URL_EXPIRY_SECONDS

Presigned URL expiration time in seconds.

Type: integer
Default: 86400 (24h)

Maximum validity of presigned URLs for accessing S3 files. URLs are returned in webhooks and automatically expire after this duration.

### STORAGE_LOCAL_PATH

Local filesystem storage path.

Type: string
Default: ./storage

For production with Docker, use a persistent volume or bind mount.
Example: /var/lib/whatsapp-gateway/storage

### STORAGE_BASE_URL

Base URL for public file access (local provider only).

Type: string
Default: ""

Optional. Used when serving local files via a web server.

Example: https://example.com/storage

### STORAGE_API_PATH

Path for serving files directly from the gateway.

Type: string
Default: /storage

Used for direct file serving via gateway HTTP endpoints.

Example: /storage

## Production Recommendations

For production deployments:

- Change all default secrets
- Use PostgreSQL instead of SQLite
- Enable Redis for distributed rate limiting if horizontally scaled
- Enable RabbitMQ for high throughput systems
- Disable Documentation or protect it properly
- Set LOG_LEVEL to info or warn
- Use strong HMAC secret (32+ random bytes)
- Restrict HTTP_ORIGIN
- Use persistent storage for local provider in production (Docker volumes)
- Enable SSL for S3 provider in production
- Use appropriate presigned URL expiry based on security requirements

Environment configuration directly impacts system reliability and security.  
Defaults are intended for development convenience, not hardened production.
