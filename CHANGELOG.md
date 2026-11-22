# Changelog

All notable changes to this project are documented here. Versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.16.0] - 2026-11-22

### Added
- **Webhook dead letters (direct mode)** — a direct-mode webhook delivery that exhausts `WEBHOOK_MAX_RETRIES` (or arrives while the sender's concurrency slots are full) is no longer dropped: the envelope (`{url, hmac_secret, payload}`) is persisted AES-GCM-encrypted in the `webhook_dead_letters` table for 7 days (`deadLetterRetentionSeconds`), keyed by a deterministic hash so a given event stores once. Only fixed classifications (`delivery_failed`, `semaphore_full`, `invalid_payload`) are ever stored as `last_error` — transport error text never reaches the DB or admin responses. New metric `whatsapp_gateway_webhook_dead_letter_misses_total{reason}` counts persistence failures (delivery NOT retained).
- **Admin dead-letter plane** — `GET /admin/webhook-dlq?limit=&offset=` (1–100, default 50; metadata only: id, timestamps, retry count, classification — never URL, secret, payload, or error text) and `POST /admin/webhook-dlq/{id}/replay` (at-least-once: 404 missing/expired, 409 while another replay holds the 300s lease, 502 on delivery failure with the row retained, 200 `{status:"delivered"}` + row deleted on 2xx). Same `ADMIN_API_SECRET` bearer as the rest of the admin plane.
- **Unified incoming webhook payloads** — direct mode (`handler.deliverWebhook`) and queue mode (`IncomingEventHandler`) now build payloads through one shared builder (`BuildWebhookPayload` in `internal/whatsapp/incoming_payload.go`), guaranteeing byte-identical payloads for the same event across modes: `ContactMessage` handling, media error fields, nil-message handling and JID resolution all have one authority. Media is downloaded at most once per event, before subscription fan-out.
- **Incoming media download deadline** — `INCOMING_MEDIA_DOWNLOAD_TIMEOUT_SECONDS` (default 5; `<= 0` keeps the historical 5s) bounds each incoming-media webhook download in both modes, so a hung WhatsApp CDN fetch can't wedge a worker.
- **Marketing landing page** — `docs/ui/landing.html`: a dark "signal" design system landing page (console hero with copyable quickstart, spec band, animated flow diagram, capabilities bento, endpoint table, agents/operate/ship sections) sharing tokens with the docs console. On the public site it renders at `/` via `cmd/docs-gen`; on a self-hosted server it now serves at the root route (see Changed).
- **Runtime landing route** — when `ENABLE_DOCUMENTATION=true` the gateway now serves the landing page at `GET /`, its favicon/og assets at `GET /assets/*`, and root aliases `GET /llms.txt` + `GET /openapi.yaml` (mirroring the static-site layout the landing links to). Unauthenticated by design (front door); `/docs/` deep links land on the session-authed console.
- **Docs-site restructure (public site)** — `scripts/build-docs-pages.sh` + `cmd/docs-gen` now render the landing at `_site/index.html` and the docs console under `_site/docs/` with its assets at `_site/docs/assets/`; root paths (`/llms.txt`, `/openapi.yaml`, favicons) stay stable. Template render + deep-link guard tests (`cmd/docs-gen`) keep landing links and `nav.json` from drifting.
- **Project planning docs** — `CONTEXT.md` (domain glossary), `docs/ROADMAP.md`, `docs/adr/` (accepted decisions: chat resolver, DB-backed idempotency, whatsmeow error sentinels) and `docs/plan/` (PRDs, blueprints, goal) are now tracked; the changelog's roadmap references resolve in-repo.

### Fixed
- **`.env.example` shipped a phantom register secret** — it listed `BASIC_AUTH_SECRET_KEY`, a name the config never reads (`SECRET_KEY` is the real key), so a copy-paste deployment silently ran the `/register` Basic Auth with the default `secret`. The example now uses `SECRET_KEY`; wiki docs renamed to match. Production-secret validation (`validateProductionSecrets`) is unchanged.
- **`.env.example` / config parity** — documented every previously missing var (`DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFE_MINS`, `REDIS_ENABLED`/`REDIS_URI`, `INCOMING_MESSAGE_BUFFER_SIZE`, `WHATSMEOW_LOG_LEVEL`); the two sets now match exactly.
- **Dead config removed** — `HTTP_ORIGIN` was loaded into `config.Config` but never read anywhere (no CORS middleware exists); it is removed from the schema so operators don't believe setting it enables CORS.

### Changed
- Docs content: webhook dead-letter + DLQ replay reference (`wiki/Environment-Variables.md`), incoming media timeout, MCP entry points (`docs/ui/assets/docs/mcp/introduction.md`, getting-started intro, `llms.txt`).
- whatsmeow upgraded to `v0.0.0-20261116172437` (Go 1.26 toolchain; Dockerfile `golang:1.26`; `foreign_keys(1)` added to the default SQLite pragmas — the new sqlstore refuses schema upgrades without it).

## [0.15.3] - 2026-08-25

### Fixed
- `connect_timeout` is now injected only into URL-form postgres DSNs (keyword-form DSNs got a broken parameter).

## [0.15.2] - 2026-08-25

### Fixed
- Postgres DSNs gain TCP keepalives and a connect timeout, so a dead DB endpoint fails fast instead of hanging startup.

## [0.15.1] - 2026-08-25

### Added
- `SEND_TIMEOUT_SECONDS` — hard deadline for every outbound WhatsApp round-trip at the whatsmeow boundary (expiry → `504`); bounds zombie-socket hangs.
- `GET /debug/pprof/*` (goroutine/heap profiles) behind `ADMIN_API_SECRET`.
- `/main health` probe subcommand for scratch-image healthchecks.

## [0.15.0] - 2026-08-02

### Added
- **`session.reachout_timelocked` webhook** — whatsmeow `NotifyAccountReachoutTimelock` is surfaced as a webhook (`is_active`, `enforcement_type`, `time_enforcement_ends`); recorded to `session_status` but the client is never evicted and the pacer's ban gate is not fed. `openapi.yaml` → `0.26.0`.

### Fixed
- WhatsApp send-nack `463` (`NackCallerReachoutTimelocked`) now maps to `429` instead of `500`, so callers learn it's enforcement, not a gateway glitch.

## [0.14.0] - 2026-07-30

### Added
- Running server version surfaced in the docs header badge and `/health`.
- Trusted-proxy support (`TRUSTED_PROXIES`/`PROXY_HEADER`): the client IP is read from the proxy header only for allowlisted hops, so a spoofed `X-Forwarded-For` can't poison the per-IP register limiter.

### Changed
- whatsmeow bumped to 2026-07-22.
- Docs: auth schemes disambiguated, login status clarified; llms.txt uses canonical `chat`.

### Removed
- Dead pacer `Drain` method; documented HTTP 463 handling.

## [0.13.0] - 2026-07-15

### Changed (docs console)
- Docs engine unified into a single source (`pkg/docsgen`) shared by the runtime and the static-site generator.
- Full-text search palette, mermaid diagram rendering (+ fullscreen pan/zoom viewer), design tokens with light/dark theme and focus a11y, on-page TOC rail, code copy buttons + syntax highlighting.
- Fixed: root-served asset fetches resolve under a base path; mobile content scrolls under the inactive sidebar overlay.

## [0.12.0] - 2026-07-15

### Added
- **Outbound pacing (roadmap #2)** — a single shared in-process **pacer** (`pkg/ratelimiter.Pacer`) governs every outbound WhatsApp action — all `POST /message/*` sends, react, edit, delete, mark-read, typing/presence, and all group/community mutations — in both direct and queue modes, in three ordered layers: (1) a **ban gate** — while the account is under a WhatsApp temporary ban the action is rejected with `429` until the ban expires (auto-resume; `OUTBOUND_PACE_BAN_DEFAULT_HOLD_SECONDS` default 3600 when no explicit `ban_expires_at` is known); (2) a **per-recipient hard cap** — more than `OUTBOUND_PACE_PER_RECIPIENT_REQUESTS` (default 10) actions to one recipient inside `OUTBOUND_PACE_PER_RECIPIENT_WINDOW_SECONDS` (default 60) reject with `429` (repeat sends to one recipient is the top ban trigger); (3) a **per-account token bucket** (`OUTBOUND_PACE_RATE_PER_SECOND`/`OUTBOUND_PACE_BURST`, default 1/s burst 5). `OUTBOUND_PACE_MODE=pace` blocks up to `OUTBOUND_PACE_MAX_WAIT_SECONDS`+`OUTBOUND_PACE_JITTER_MS` (default 30s/250ms) then 429s; `reject` 429s immediately. In-memory/per-instance; `OUTBOUND_PACE_ENABLED=false` falls back to the legacy `MESSAGE_RATE_LIMIT_*` behavior.
- **Generic event dispatcher + `session.*` webhooks (roadmap #1)** — every webhook dispatch site fans out through one subscription-aware primitive (`GetWebhookSubscriptions` + `EventMatches`), and six lifecycle events are emitted from the whatsmeow event switch: `session.logged_out`, `session.banned`, `session.connect_failure`, `session.connected`, `session.disconnected`, `session.replaced` (flat envelope: `event`/`phone_number`/`jid`/`timestamp`).
- **Multi-URL / per-event webhook subscriptions (Phase F)** — multiple webhook URLs per account, each with an optional `events` filter (empty = all). `GET /webhook` keeps the top-level `url` for back-compat and adds `subscriptions[]` (`has_hmac`; the secret is never returned). `POST /webhook` upserts one subscription (unknown event → 400); `DELETE /webhook` with no body clears all, `{url}` deletes one. Storage: `device_webhook_subscriptions` (backfilled once from legacy `device_webhooks`).
- **Group & community mutations (Phase E)** — the highest-ban-risk feature group, shipped default-safe: create group/community, leave, participants add/remove/promote/demote (batch, 200 with per-participant `results[]`), settings/name/topic/photo, invite get/reset/info, join via link, join-request list/approve-reject, community sub-group link/unlink + participants reads. Gates: `GROUP_MANAGEMENT_ENABLED` (master, unregisters routes), `GROUP_ADD_PARTICIPANTS_ENABLED`, `GROUP_JOIN_VIA_LINK_ENABLED`, `GROUP_MAX_PARTICIPANTS_PER_REQUEST`.
- **Operator visibility (Phase D)** — admin plane at the ROOT path, dark by default: `GET /admin/sessions` (+ `/{phone}`) merged from live clients + device store + persisted `session_status` into honest states (`connected`/`disconnected`/`never_paired`/`logged_out`/`banned`, masked phones, `instance`); `GET /metrics` (hand-rolled Prometheus text, no phone labels); gated by `ADMIN_API_SECRET` (unset ⇒ routes never registered, 404).
- **Session lifecycle tracking** — whatsmeow `LoggedOut`/`TemporaryBan`/`ConnectFailure`/`StreamReplaced` persist to `session_status` and evict dead in-memory clients (transient `ConnectFailure` is recorded but not evicted, so auto-reconnect recovers).
- **Direct-mode webhook retry parity** — status webhooks deliver asynchronously with bounded exponential backoff on a detached context (`WEBHOOK_MAX_RETRIES` default 3, `WEBHOOK_RETRY_BACKOFF_SECONDS` default 2), semaphore-bounded, drop-on-full.
- **Replies & mentions on every send** — `reply_to_id`/`reply_to_sender`/`reply_to_text` + `mentions[]` on all `POST /api/message/*` (send `mentions` as repeated form fields on multipart).
- **`POST /api/message/read`** — mark read (blue ticks); `sender` required for group chats; exactly one receipt type used.
- **`POST /api/chat/presence`** — typing indicator (`composing`/`recording`/`paused`).
- **Send idempotency** — optional `Idempotency-Key` header on `/api/message/*`, DB-backed per `(phone_number, key)`: duplicates replay verbatim (`Idempotent-Replay: true`), in-flight → `409`, different body → `422`. Queue mode: enqueued-once, not delivered-once. (`IDEMPOTENCY_TTL_SECONDS`, `IDEMPOTENCY_PENDING_TIMEOUT_SECONDS`.)
- **Unified `chat` recipient** — bare number, `@s.whatsapp.net`, `@g.us`, or `@lid` accepted on every send/message/contact endpoint (`msisdn` stays as a deprecated alias; `chat` wins). Responses echo the resolved JID. First step of the automation-platform plan (`docs/plan/`).
- **Graceful multi-account shutdown** — all whatsmeow clients disconnect cleanly (bounded by `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS`, default 10) before the DB closes.
- **Liveness/readiness probes** — `GET /health/live` (always 200, process-only) and `GET /health/ready` (503 when DB/enabled queue down; not coupled to WhatsApp session health).
- **`GET /api/contact/`** — locally-synced contacts, paginated (default 100, max 500). Pure local read, never 404 on empty.
- **`GET /api/group/`** and **`GET /api/group/info?chat=`** — joined-group summaries and one group's full detail + roster; short-TTL cached, metered by the per-account read budget (`429` when exhausted).
- **`GET /api/contact/info?chat=`** and **`GET /api/contact/avatar?chat=`** — profile lookup and profile picture (time-limited CDN URL; id doubles as an `ETag` → `304` via `If-None-Match`). Tri-state: 404 no picture / 403 hidden / 200 otherwise.
- **`GET /api/community/subgroups?chat=`** and **`GET /api/community/participants?chat=`** — community sub-group and cross-linked participant reads, same cache + budget.
- **Read/query cache + per-account budget** infra (`READ_QUERY_CACHE_TTL_SECONDS`, `READ_QUERY_BUDGET`, `READ_QUERY_WINDOW_SECONDS`) shared by all server-hitting reads.
- whatsmeow error sentinels mapped via `errors.Is`: `ErrGroupNotFound`/`ErrProfilePictureNotSet` → 404, `ErrNotInGroup`/`ErrProfilePictureUnauthorized` → 403.

### Fixed
- API errors serialize as the documented `{error, code}` shape.
- Webhook queue delivery deduped per-destination so fan-out reaches every subscription; session JID resolved from a cache to survive the logout race; subscription backfill runs once and outlives device deletes.
- Bulk group ops paced as one op and fail fast when n > burst; image/audio/video/document sends paced in direct mode; session-status timestamps stored in UTC for a correct ban gate.

### Changed
- `msisdn` deprecated in favor of `chat`; recipient resolution funnels through `resolveChat` (rejects `broadcast`/unknown servers with 400 early).
- `openapi.yaml`/`llms.txt` document the full surface above (spec version → `0.25.0` over the cycle).

## [0.11.1] - 2026-07-05

### Docs (no code change; refreshes the in-image OpenAPI spec + docs UI site)
- `openapi.yaml`: `/message/audio`, `/message/video`, `/message/document`, `/contact/check` paths + schemas; `addressing_mode` on the incoming webhook; `sender_msisdn` on react; `429` on `/register`.
- `.env.example`: Rate Limiting + Upload Limits sections.
- `README.md`, `llms.txt`: v0.11 endpoint/config parity.
- `docs/ui` site: feature-matrix, environment-variables, webhook-flow (fixed webhook verification: `X-Webhook-Signature` + `sha256=` prefix + raw body), important-security, MCP tools-reference.
- `wiki/`: restored detailed pages in-repo (source for `sync-wiki`).

## [0.11.0] - 2026-07-05

### Added
- `POST /message/audio` (voice notes via `is_ptt`; view-once).
- `POST /message/video` (caption, `is_gif`, view-once).
- `POST /message/document` (`file_name`, caption wrapped for reliable render).
- `GET /contact/check` — `IsOnWhatsApp` recipient validation; warms the LID↔PN cache.
- Outbound media size cap (`MAX_UPLOAD_BYTES`) + per-kind MIME allow-lists.
- Per-IP rate limit on `POST /register` (`REGISTER_RATE_LIMIT_*`).

### Fixed
- Polls now decryptable (`BuildPollCreation` injects MessageSecret — polls were write-only).
- Reactions attribute correctly (`BuildReaction` + optional `sender_msisdn`).
- `@lid` no longer leaks into webhook `from` (prefer `SenderAlt`; new `addressing_mode` field).
- `JWT_TOKEN_DURATION_MINUTES` overflow clamped to 1..525600.
