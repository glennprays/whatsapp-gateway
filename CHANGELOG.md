# Changelog

All notable changes to this project are documented here. Versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- **Unified `chat` recipient** on every send/message/contact-check request. `chat` accepts a bare phone number, a user JID (`@s.whatsapp.net`), a group JID (`@g.us`), or a `@lid`, so groups are now addressable. Send responses echo the resolved canonical `chat` JID. First step of the automation-platform plan (`docs/plan/`).
- **Graceful multi-account shutdown**: all whatsmeow clients are now disconnected cleanly on shutdown (before the DB closes), concurrently and bounded by `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS` (default 10), so deploys no longer drop sockets abruptly.
- **Liveness/readiness probes** at the root path: `GET /health/live` (always `200`, process-only) and `GET /health/ready` (`503` when the DB or an enabled queue is down; deliberately not coupled to WhatsApp session health). The existing `/health` under the API base path is unchanged.
- **`GET /api/contact/`** — list the account's locally-synced contacts, paginated via `limit`/`offset` (default 100, max 500), with `count`/`total` and a synced-state note. A pure local-store read (no network); an empty/partial list right after pairing is normal, not an error. (openapi/llms doc entry lands with the rest of the read/query family.)

### Changed
- `msisdn` is now a **deprecated back-compat alias** for `chat` (still fully supported; `chat` wins when both are set). Recipient resolution funnels through `resolveChat`, which strips device/agent JID suffixes, lowercases the server, requires a digits-only user, and rejects `broadcast`/unknown servers early with a `400` instead of a late `500`.
- `openapi.yaml`: `chat` added to all request bodies + send responses; `msisdn` marked deprecated and dropped from `required`; version → `0.12.0`.

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
