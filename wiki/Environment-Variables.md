# Environment Variables Configuration

This page provides a comprehensive reference for all environment variables used to configure the WhatsApp Gateway.

## Configuration Overview

The WhatsApp Gateway uses environment variables for configuration. Copy `.env.example` to `.env` and customize the values according to your needs.

```bash
cp .env.example .env
```

## Configuration Sections

### Server Configuration

#### `PORT`
- **Description**: The port on which the HTTP server will listen
- **Type**: Integer
- **Default**: `3000`
- **Example**: `PORT=3000`
- **Note**: If running in Docker, ensure this matches the exposed port in your Docker configuration

#### `BASIC_AUTH_SECRET_KEY`
- **Description**: Secret key for basic authentication (if used)
- **Type**: String
- **Default**: `secret`
- **Example**: `BASIC_AUTH_SECRET_KEY=my_secure_secret_key_123`
- **Security**: Change this in production environments

---

### HTTP Configuration

#### `BASE_PATH`
- **Description**: **Dynamic base path** for all API endpoints. This allows you to mount the API at any path prefix.
- **Type**: String
- **Default**: `/api`
- **Example**: `BASE_PATH=/api` or `BASE_PATH=/gateway` or `BASE_PATH=/whatsapp`
- **Usage**: If set to `/api`, all endpoints will be accessible at `http://localhost:3000/api/v1/*`
- **Note**: This is dynamic and can be changed to match your infrastructure needs (e.g., if using a reverse proxy or API gateway)

---

### Swagger Documentation Configuration

#### `ENABLE_SWAGGER`
- **Description**: Enable or disable the Swagger UI documentation interface
- **Type**: Boolean
- **Default**: `true`
- **Example**: `ENABLE_SWAGGER=true`
- **Note**: Set to `false` in production for security

#### `SWAGGER_USER`
- **Description**: **Username required to access the Swagger documentation**
- **Type**: String
- **Default**: `secret`
- **Example**: `SWAGGER_USER=admin`
- **Security**: The Swagger docs are protected by basic authentication. You must provide this username to access `/docs`

#### `SWAGGER_PASSWORD`
- **Description**: **Password required to access the Swagger documentation**
- **Type**: String
- **Default**: `secret`
- **Example**: `SWAGGER_PASSWORD=secure_password_123`
- **Security**: The Swagger docs are protected by basic authentication. You must provide this password to access `/docs`

#### `SWAGGER_BASE_PATH`
- **Description**: **Dynamic base path** for the Swagger documentation interface
- **Type**: String
- **Default**: `/docs`
- **Example**: `SWAGGER_BASE_PATH=/docs` or `SWAGGER_BASE_PATH=/api-docs`
- **Usage**: Access Swagger UI at `http://localhost:3000/docs` (or your configured path)
- **Note**: This path is dynamic and can be customized to match your documentation URL structure

**⚠️ Important**: The Swagger documentation requires both username and password for access. When accessing the docs at the configured base path, you'll be prompted for credentials.

---

### JWT Configuration

#### `JWT_SECRET`
- **Description**: Secret key used to sign and verify JWT tokens
- **Type**: String
- **Default**: `secret`
- **Example**: `JWT_SECRET=your_very_secure_random_string_here`
- **Security**: **CRITICAL** - Use a long, random, and secure secret in production
- **Note**: Changing this will invalidate all existing tokens

#### `JWT_TOKEN_DURATION_MINUTES`
- **Description**: Duration (in minutes) for which the JWT token remains valid
- **Type**: Integer
- **Default**: `1440` (24 hours)
- **Example**: `JWT_TOKEN_DURATION_MINUTES=1440` (24 hours)
- **Clamping**: Clamped to `1..525600` (1 minute..1 year). Out-of-range values (e.g. an accidental `1000000000000000000`, which would overflow `time.Duration`) reset to the `1440` default.
- **Note**: Balance security (shorter duration) with user convenience (longer duration). The gateway does **not** revoke tokens on logout; mint short-lived tokens from your wrapping backend for sensitive deployments.

#### `JWT_ISSUER`
- **Description**: The issuer claim for JWT tokens (identifies who issued the token)
- **Type**: String
- **Default**: `whatsapp-gateway`
- **Example**: `JWT_ISSUER=whatsapp-gateway`
- **Note**: This is a standard JWT claim used for token validation

**JWT Overview**: This gateway uses industry-standard JSON Web Tokens (JWT) for authentication. When you register via the `/register` endpoint, you receive a JWT token that must be included in the `Authorization: Bearer <token>` header for all subsequent API requests. The JWT contains claims about the authenticated phone number and is signed with the `JWT_SECRET` to prevent tampering.

---

### WhatsApp Configuration

#### `WHATSAPP_DATASTORE_TYPE`
- **Description**: Type of database to use for storing WhatsApp session data and device information
- **Type**: String (Enum)
- **Allowed Values**: 
  - `sqlite` or `sqlite3` - Use SQLite database (file-based, good for development and small deployments)
  - `postgres` - Use PostgreSQL database (recommended for production)
- **Default**: `sqlite`
- **Example**: 
  ```
  WHATSAPP_DATASTORE_TYPE=sqlite3
  # or
  WHATSAPP_DATASTORE_TYPE=postgres
  ```
- **Note**: The value for SQLite can be either `"sqlite"` or `"sqlite3"` - both are accepted

#### `WHATSAPP_DATASTORE_URI`
- **Description**: Connection string/URI for the selected datastore
- **Type**: String (Connection URI)
- **Examples**:
  
  **For SQLite:**
  ```
  WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)
  ```
  - `file:dbs/whatsapp.db` - Path to the SQLite database file
  - `?_pragma=foreign_keys(1)` - Enables foreign key constraints
  
  **For PostgreSQL:**
  ```
  WHATSAPP_DATASTORE_URI=postgresql://username:password@localhost:5432/whatsapp_gateway?sslmode=disable
  # or
  WHATSAPP_DATASTORE_URI=postgres://username:password@localhost:5432/whatsapp_gateway?sslmode=require
  ```
  - Replace `username`, `password`, `localhost`, `5432`, and `whatsapp_gateway` with your PostgreSQL credentials
  - `sslmode=disable` for local development, `sslmode=require` for production

- **Note**: Ensure the directory exists for SQLite, or the database exists for PostgreSQL

#### `WHATSAPP_DEVICE_LABEL`
- **Description**: Label/name for the WhatsApp device that will appear in WhatsApp's "Linked Devices" section
- **Type**: String
- **Default**: `"Whatsapp Gateway"`
- **Example**: `WHATSAPP_DEVICE_LABEL="My Company WhatsApp Gateway"`
- **Note**: This helps identify the gateway when viewing connected devices in WhatsApp

#### `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY`
- **Description**: **Master encryption key used to encrypt the HMAC secrets for device webhooks before storing them in the database**
- **Type**: String (Hexadecimal, must be 32 characters representing 16 bytes)
- **Default**: `0123456789abcdef0123456789abcdef`
- **Example**: `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6`
- **Security**: **CRITICAL** - This key is used to encrypt HMAC secrets before they are stored in the database
- **Purpose**: 
  - When you register a webhook with an `hmac_secret`, this secret is encrypted using this master key before being stored
  - The encrypted HMAC secret is then used to sign webhook payloads sent to your backend
  - This ensures that even if the database is compromised, the actual HMAC secrets remain protected
- **Note**: 
  - Must be exactly 32 hexadecimal characters (16 bytes for AES-128 encryption)
  - Use a cryptographically secure random string
  - Changing this key will make existing encrypted webhook secrets unreadable
  - Generate using: `openssl rand -hex 16`

**Webhook HMAC Flow**:
1. You register a webhook URL with an optional `hmac_secret`
2. The gateway encrypts this `hmac_secret` with the `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` and stores it
3. When a WhatsApp event occurs (e.g., incoming message), the gateway:
   - Decrypts the stored HMAC secret
   - Signs the webhook payload with the decrypted secret
   - Sends the payload with the HMAC signature to your backend
4. Your backend can verify the signature using the original `hmac_secret` you provided

---

### Rate Limiting Configuration

The `/register` endpoint is throttled per client IP to prevent brute-forcing of `BASIC_AUTH_SECRET_KEY` and mass token minting. The limiter is in-process (memory) so it works without Redis.

#### `REGISTER_RATE_LIMIT_ENABLED`
- **Description**: Toggle per-IP throttling on `POST /register`
- **Type**: Boolean
- **Default**: `true`
- **Example**: `REGISTER_RATE_LIMIT_ENABLED=true`

#### `REGISTER_RATE_LIMIT_REQUESTS`
- **Description**: Maximum registrations per IP per window
- **Type**: Integer
- **Default**: `5`
- **Example**: `REGISTER_RATE_LIMIT_REQUESTS=10`

#### `REGISTER_RATE_LIMIT_DURATION_SECONDS`
- **Description**: Rolling window length in seconds
- **Type**: Integer
- **Default**: `60`
- **Example**: `REGISTER_RATE_LIMIT_DURATION_SECONDS=60`
- **Over-budget response**: `429 Too Many Requests` with a `Retry-After` header (seconds). The limiter fails open on internal errors.

> Outbound **message** sends are gated separately by the existing `MESSAGE_RATE_LIMIT_*` variables.

---

### Upload Limits Configuration

#### `MAX_UPLOAD_BYTES`
- **Description**: Maximum size of an outbound media upload (image/audio/video/document/sticker), checked **before** the file is read into memory or base64-encoded into a queue job. Protects against OOM under concurrent large uploads.
- **Type**: Integer (bytes)
- **Default**: `16777216` (16 MiB)
- **Example**: `MAX_UPLOAD_BYTES=33554432` (32 MiB)
- **MIME allow-lists**: In addition to the size cap, `image`/`sticker`/`audio`/`video` uploads are validated against a per-kind MIME allow-list. Disguised payloads (e.g. a binary sent as an image) are rejected with `400`. Documents accept any mimetype. PTT voice notes opt out of MIME sniffing (opus/ogg is unidentifiable) and default to `audio/ogg; codecs=opus` server-side.

---

### Read / Query Surface Configuration

Server-hitting reads (joined groups; later profiles/avatars) are short-TTL **cached** and metered by a **per-account budget** so a polling caller cannot trip WhatsApp anti-spam. A budget token is spent **only on a cache miss** — cache hits are free. Local-store reads (e.g. `GET /contact/`) are never metered.

#### `READ_QUERY_CACHE_TTL_SECONDS`
- **Description**: How long a server-hitting read stays cached before the next call re-fetches from WhatsApp.
- **Type**: Integer (seconds)
- **Default**: `300`

#### `READ_QUERY_BUDGET`
- **Description**: Maximum cache-miss reads per account per window before requests receive `429 Too Many Requests` (with `Retry-After`).
- **Type**: Integer
- **Default**: `30`

#### `READ_QUERY_WINDOW_SECONDS`
- **Description**: The rolling window over which `READ_QUERY_BUDGET` is counted.
- **Type**: Integer (seconds)
- **Default**: `60`

---

### Group & Community Management Configuration

Group/community **mutations** are the highest-ban-risk surface. They are gated by a master toggle plus two default-off gates over the specific bulk/mass vectors. Group/community **reads** (`GET /group/`, `GET /group/info`, `GET /community/subgroups`, `GET /community/participants`) are always available regardless of these settings.

#### `GROUP_MANAGEMENT_ENABLED`
- **Description**: Master toggle for the entire mutation surface (create/leave/participants/settings/name/topic/photo, invite links, join-via-link, join-requests, community link/unlink). When `false` those routes are **never registered**, so the whole surface returns `404` (hidden entirely); reads stay up.
- **Type**: Boolean
- **Default**: `true`

#### `GROUP_ADD_PARTICIPANTS_ENABLED`
- **Description**: Gates **bulk participant add** — `POST /group/participants` with `action=add`, and adding participants at create time (`POST /group/` with a non-empty `participants`). When `false` those requests return `403` (checked in the usecase, before any server call). remove/promote/demote/settings/name/topic/photo/leave stay enabled. Adding people you have no prior relationship with is the classic ban trigger, so this defaults off.
- **Type**: Boolean
- **Default**: `false`

#### `GROUP_JOIN_VIA_LINK_ENABLED`
- **Description**: Gates `POST /group/join` — the mass-join vector. When `false` it returns `403`. Defaults off until outbound pacing lands.
- **Type**: Boolean
- **Default**: `false`

#### `GROUP_MAX_PARTICIPANTS_PER_REQUEST`
- **Description**: Caps how many participants a single batch (add/remove/promote/demote, approve/reject) may carry; an over-cap request is `400` before the server call. `0` disables the cap.
- **Type**: Integer
- **Default**: `256`

---

### Send Idempotency Configuration

Send endpoints (`/api/message/*`) accept an optional `Idempotency-Key` header. A duplicate key **replays** the original response (with `Idempotent-Replay: true`) rather than sending again; an in-flight duplicate returns `409`; reusing a key with a **different** request body returns `422`. Dedup is DB-backed and keyed by the **JWT phone number + key** (never the body — one Account cannot spoof another's namespace), so it survives restarts and is shared across instances. In queue mode this is **enqueued-once, not delivered-once**.

#### `IDEMPOTENCY_TTL_SECONDS`
- **Description**: How long a completed response stays replayable; also the retention bound a background sweeper enforces on the `idempotency_keys` table.
- **Type**: Integer (seconds)
- **Default**: `86400` (24h)

#### `IDEMPOTENCY_PENDING_TIMEOUT_SECONDS`
- **Description**: If a request crashes after reserving a key but before completing, its row stays `pending`. After this timeout a retry may take the key over instead of receiving `409` indefinitely.
- **Type**: Integer (seconds)
- **Default**: `30`

---

### Graceful Shutdown Configuration

#### `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS`
- **Description**: Overall bound for cleanly disconnecting all WhatsApp clients on shutdown, **before** the database is closed. A hung client is skipped so shutdown never blocks past this bound.
- **Type**: Integer (seconds)
- **Default**: `10`

---

### Admin / Metrics Plane Configuration

An **operator-only** admin plane lives at the **ROOT path** (outside `/api/v1`): `GET /admin/sessions`, `GET /admin/sessions/{phone}`, and `GET /metrics`. It is **dark by default** — with no `ADMIN_API_SECRET` the routes are never registered and return `404` (never a `401` that would confirm the plane exists). The plane is cross-tenant by design (operator visibility) and the inventory is **per-instance**: a device with no live client on this node may be live on another.

#### `ADMIN_API_SECRET`
- **Description**: Bearer secret for `/admin/*` and `/metrics`. Empty disables (unregisters) the whole plane. When set, requests must send `Authorization: Bearer <secret>`, compared in constant time (`crypto/subtle`).
- **Type**: String
- **Default**: `""` (disabled)

#### `METRICS_ENABLED`
- **Description**: Exposes `GET /metrics` (hand-rolled Prometheus text exposition). Still requires `ADMIN_API_SECRET` to be reachable (same bearer-gated plane). Series: `whatsapp_gateway_messages_total{type,mode,result}`, `whatsapp_gateway_webhook_deliveries_total{result,mode,event}`, `whatsapp_gateway_sessions{state}` (gauge). A phone number is **never** a metric label (cardinality).
- **Type**: Boolean
- **Default**: `false`

---

### Webhook Subscriptions & Status Events

Each account can register **multiple** webhook subscriptions, each with an
optional per-subscription `events` filter (`POST /webhook` with `events: [...]`).
An empty/omitted filter receives **all** events. Event catalog:
`message.incoming`, `message.queued`, `message.sent`, `message.failed`.
`GET /webhook` returns the legacy top-level `url` (first subscription) plus a
`subscriptions[]` array (`url`, `events`, `has_hmac`); the HMAC secret is never
returned. `DELETE /webhook` with no body clears all subscriptions; with
`{"url": "..."}` it removes one.

#### `WEBHOOK_STATUS_EVENTS_ENABLED`
- **Description**: Master kill-switch for the `message.queued/sent/failed` status family. When `false`, no status webhook fires regardless of per-subscription filters. `message.incoming` is not gated by this flag.
- **Type**: Boolean
- **Default**: `true`

#### `WEBHOOK_STATUS_EVENTS`
- **Description**: **Deprecated** — superseded by the per-subscription `events` filter. Still read for back-compat, but a subscription registered with an empty filter now receives all events. Prefer setting `events` per URL.
- **Type**: Comma-separated string
- **Default**: `message.sent,message.failed`

---

### Direct-mode Webhook Retry Configuration

Direct-mode status webhooks (`message.queued` / `message.sent` / `message.failed`) are delivered asynchronously with bounded exponential backoff on a detached context, so a retry outlives the HTTP request. Best-effort (drop-on-full, no DLQ); durable delivery requires RabbitMQ. Queue mode keeps its own RabbitMQ-level retry and is not double-retried.

#### `WEBHOOK_MAX_RETRIES`
- **Description**: Maximum retry attempts (beyond the first) for a direct-mode webhook delivery. Capped internally at 10.
- **Type**: Integer
- **Default**: `3`

#### `WEBHOOK_RETRY_BACKOFF_SECONDS`
- **Description**: Base backoff for the exponential retry schedule (`base`, `base×2`, `base×4`, …).
- **Type**: Integer (seconds)
- **Default**: `2`

---

## Security Best Practices

1. **Never commit `.env` files**: Always use `.env.example` as a template
2. **Use strong secrets**: Generate cryptographically secure random strings for all secret keys
3. **Protect Swagger docs**: Use strong credentials for `SWAGGER_USER` and `SWAGGER_PASSWORD`
4. **Disable Swagger in production**: Set `ENABLE_SWAGGER=false` in production
5. **Use PostgreSQL in production**: SQLite is good for development, but PostgreSQL is recommended for production
6. **Secure your database**: Ensure proper authentication and network security for your database
7. **Rotate secrets regularly**: Periodically update JWT secrets and encryption keys (note: this will invalidate existing tokens/data)

## Generating Secure Secrets

Use these commands to generate secure random strings:

**For JWT_SECRET and BASIC_AUTH_SECRET_KEY:**
```bash
openssl rand -base64 32
```

**For WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY (32 hex characters):**
```bash
openssl rand -hex 16
```

## Example Configuration Files

### Development (.env.development)
```env
PORT=3000
BASIC_AUTH_SECRET_KEY=dev_secret
BASE_PATH=/api
ENABLE_SWAGGER=true
SWAGGER_USER=admin
SWAGGER_PASSWORD=admin
SWAGGER_BASE_PATH=/docs
JWT_SECRET=dev_jwt_secret_key
JWT_DURATION_MINUTES=60
JWT_ISSUER=whatsapp-gateway-dev
WHATSAPP_DATASTORE_TYPE=sqlite3
WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)
WHATSAPP_DEVICE_LABEL="Development Gateway"
WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=0123456789abcdef0123456789abcdef
```

### Production (.env.production)
```env
PORT=3000
BASIC_AUTH_SECRET_KEY=<strong-random-secret>
BASE_PATH=/api
ENABLE_SWAGGER=false
SWAGGER_USER=<secure-username>
SWAGGER_PASSWORD=<strong-password>
SWAGGER_BASE_PATH=/docs
JWT_SECRET=<strong-random-jwt-secret>
JWT_DURATION_MINUTES=1440
JWT_ISSUER=whatsapp-gateway-prod
WHATSAPP_DATASTORE_TYPE=postgres
WHATSAPP_DATASTORE_URI=postgresql://user:pass@db-host:5432/whatsapp?sslmode=require
WHATSAPP_DEVICE_LABEL="Production WhatsApp Gateway"
WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=<secure-random-hex-32-chars>
```

## Next Steps

- [Development Guide](Development-Guide.md) - Learn how to run the gateway
- [Gateway Usage Flow](Gateway-Usage-Flow.md) - Understand how to use the API
- [Security Considerations](Security-Considerations.md) - Important security information

---

[← Back to Home](Home.md)
