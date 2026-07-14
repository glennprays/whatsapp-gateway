# Changelog

All notable changes to this project are documented here. Versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- **`POST /api/message/read`** — mark messages as read (blue ticks) for a chat: `{chat, message_ids[], sender?}`. `sender` (the message author) is required for group chats. Exactly one whatsmeow receipt type is used, avoiding its multi-type panic.
- **`POST /api/chat/presence`** — typing indicator: `{chat, state}` where `state` is `composing` (typing…), `recording` (voice note; sent as composing+audio), or `paused`. The indicator auto-expires client-side, so an explicit `paused` is optional.
- **Interim per-account action cap** on mark-read / typing (and reusable for react) — reuses the existing limiter under an `action:` key so conversation actions can't be spammed until roadmap #2 pacing lands. Over-budget returns `429`.
- **Send idempotency** via an optional `Idempotency-Key` request header on `/api/message/*`. Backed by a DB table keyed `(phone_number, key)` (phone from the JWT, never the body), checked before the queue/direct fork so it covers both send modes and survives restarts. A duplicate replays the original response verbatim with `Idempotent-Replay: true`; an in-flight duplicate gets `409`; reusing a key with a **different** body gets `422`. In queue mode this guarantees enqueued-once, **not** delivered-once. Retention/behaviour tuned by `IDEMPOTENCY_TTL_SECONDS` (default 24h) and `IDEMPOTENCY_PENDING_TIMEOUT_SECONDS` (default 30s); a background sweeper bounds table growth.
- **Unified `chat` recipient** on every send/message/contact-check request. `chat` accepts a bare phone number, a user JID (`@s.whatsapp.net`), a group JID (`@g.us`), or a `@lid`, so groups are now addressable. Send responses echo the resolved canonical `chat` JID. First step of the automation-platform plan (`docs/plan/`).
- **Graceful multi-account shutdown**: all whatsmeow clients are now disconnected cleanly on shutdown (before the DB closes), concurrently and bounded by `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS` (default 10), so deploys no longer drop sockets abruptly.
- **Liveness/readiness probes** at the root path: `GET /health/live` (always `200`, process-only) and `GET /health/ready` (`503` when the DB or an enabled queue is down; deliberately not coupled to WhatsApp session health). The existing `/health` under the API base path is unchanged.
- **`GET /api/contact/`** — list the account's locally-synced contacts, paginated via `limit`/`offset` (default 100, max 500), with `count`/`total` and a synced-state note. A pure local-store read (no network); an empty/partial list right after pairing is normal, not an error.
- **`GET /api/group/`** — list the account's joined groups (lightweight summaries: jid, name, topic, owner, participant count, announce/locked/community flags). Hits the WhatsApp server, so it is short-TTL cached and metered by a per-account **read budget** (`READ_QUERY_*`); repeat polls are served from cache, and an exhausted budget returns `429`.
- **Read/query cache + per-account budget** infra (`READ_QUERY_CACHE_TTL_SECONDS`, `READ_QUERY_BUDGET`, `READ_QUERY_WINDOW_SECONDS`) shared by all server-hitting reads so polling can't trip anti-spam.
- **`GET /api/group/info?chat=<@g.us>`** — full detail of one group incl. the participant roster (jid/phone/lid + admin flags). Requires a group JID and account membership. Cached + budgeted.
- **`GET /api/contact/info?chat=`** — server-side profile lookup for one user (status text, current picture id, verified business name, linked-device count, lid). Cached + budgeted.
- **whatsmeow error sentinels mapped via `errors.Is`** (not substring): `ErrGroupNotFound`/`ErrProfilePictureNotSet` → `404`, `ErrNotInGroup`/`ErrProfilePictureUnauthorized` → `403`.
- **`GET /api/contact/avatar?chat=`** — a chat's (user or group) profile picture: a time-limited CDN URL plus its id. `?preview=true` for the thumbnail. Tri-state — `404` (no picture), `403` (hidden from you), `200` otherwise. The id doubles as an `ETag`: send it back via `If-None-Match` to get `304 Not Modified` when unchanged (the freshness check bypasses the cache but still spends read budget). Cached + budgeted like other server reads.

### Changed
- `msisdn` is now a **deprecated back-compat alias** for `chat` (still fully supported; `chat` wins when both are set). Recipient resolution funnels through `resolveChat`, which strips device/agent JID suffixes, lowercases the server, requires a digits-only user, and rejects `broadcast`/unknown servers early with a `400` instead of a late `500`.
- `openapi.yaml` / `llms.txt`: `chat` added to all request bodies + send responses; `msisdn` marked deprecated and dropped from `required`; read endpoints documented (`GET /contact/`, `GET /contact/info`, `GET /contact/avatar`, `GET /group/`, `GET /group/info`) with their schemas; `Idempotency-Key` header + `409`/`422` responses added to every `POST /message/*` send (reusable `IdempotencyKeyHeader` parameter); `Contact`/`Group` tags added; version → `0.16.0`.

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
