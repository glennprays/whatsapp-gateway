# Changelog

All notable changes to this project are documented here. Versions follow
[Semantic Versioning](https://semver.org/).

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
