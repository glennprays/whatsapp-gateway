# Docs-Surface Matrix

> _Review draft — uncommitted._ Every feature's "done" includes its doc surfaces. This matrix is the single checklist so nothing drifts. Track per-PRD progress in each PRD's own **Doc Surfaces** section; this is the cross-cutting rollup.

## Surfaces (what each is)

| Surface | What it is | Notes |
|---|---|---|
| `docs/openapi.yaml` | Hand-maintained spec (1203 lines, **no codegen**), `servers: /api/v1` | **`docs/openapi.go` is only the loader** that injects the server URL + serves the YAML — content lives in the YAML. Bump `info.version` per release. |
| `llms.txt` | Curated 18K LLM-facing summary | Add each endpoint family + the `chat` vs `msisdn` note + admin-plane note. |
| `wiki/Environment-Variables.md` | Full config reference | Mirror every new env key (see master list below). |
| `.env.example` | Sample env (60 keys today) | Add every new key with a sane default + comment. |
| `CLAUDE.md` | AI-assistant context | Update **API Endpoints** list + **Configuration** list; add a doc-surface step to the "Adding a New Endpoint" recipe. |
| `docs/ui/assets/docs/*` | Rendered docs site | Pages below. |
| `wiki/Security-Considerations.md` | Security guide | Admin-404 plane, ban-risk gates, `@lid` non-dialability, idempotency enqueued-once. |
| `CHANGELOG.md` | Release log | Per release. |
| SDK repo | `whatsapp-gateway-sdk-go` | Per PRD 5. |
| MCP repo | `mcp-whatsapp-gateway` | Per PRD 6. |

## Per-initiative → surface matrix

| Initiative | openapi.yaml | llms.txt | env docs (wiki + .env.example + CLAUDE.md) | docs/ui page(s) | Security wiki | CHANGELOG | SDK | MCP |
|---|---|---|---|---|---|---|---|---|
| **PRD 0 Addressing** | remove `msisdn` from `required` on 7 bodies; add `chat`; deprecate `msisdn`; add resolved-`chat` to send responses | `chat` addressing note | — | getting-started/feature-matrix, system-boundary | `@lid` non-dialable; E.164-only note | ✓ | Wave 1 | ✓ (chat arg) |
| **PRD 1 Read/Query** | 5 new GET paths + schemas; 403/404/409/429 responses | read/query family | READ_QUERY_* ×5, CONTACT_LIST_MAX_LIMIT | new "Read & Query" page; feature-matrix | budget/ban note | ✓ | Wave 3 | ✓ |
| **PRD 2 Two-Way** | `/message/read`, `/chat/presence`; `reply_to`+`mentions` on sends | two-way family | CHAT_PRESENCE_AUTO_PAUSE_SECONDS | new "Two-Way" page; feature-matrix | read-receipt privacy + played-leak note | ✓ | Wave 4 | ✓ (mark-read/typing, interim-pace note) |
| **PRD 3 Group/Community** | ~19 group/community paths + schemas; 403/410/422 | group family | GROUP_* ×4 | new "Groups & Community" page; feature-matrix | bulk-add/mass-join gates | ✓ | Waves 5–7 | ❌ mutations excluded (state exclusion) |
| **PRD 4 Ops** | `/admin/sessions`, `/health/live|ready`, `Idempotency-Key`/`Idempotent-Replay` headers; `/metrics` (note root path) | admin-plane note | ADMIN_API_SECRET, METRICS_ENABLED, IDEMPOTENCY_* ×2, SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS, WEBHOOK_MAX_RETRIES, WEBHOOK_RETRY_BACKOFF_SECONDS | new "Operations & Metrics" page; installation/production-deployment (probes, scrape); reverse-proxy-setup (root paths) | admin-404 plane; idempotency enqueued-once | ✓ | Wave 6 (admin module) | ❌ admin excluded |

## Master new-env-key list (single source for wiki + .env.example + CLAUDE.md + config.go)

| Key | Default | PRD |
|---|---|---|
| `READ_QUERY_CACHE_ENABLED` | `true` | 1 |
| `READ_QUERY_CACHE_TTL_SECONDS` | `60` | 1 |
| `READ_QUERY_AVATAR_CACHE_TTL_SECONDS` | `600` | 1 |
| `READ_QUERY_BUDGET_REQUESTS` | `30` | 1 |
| `READ_QUERY_BUDGET_DURATION_SECONDS` | `60` | 1 |
| `CONTACT_LIST_MAX_LIMIT` | `500` | 1 |
| `CHAT_PRESENCE_AUTO_PAUSE_SECONDS` | `0` | 2 |
| `GROUP_MANAGEMENT_ENABLED` | `true` | 3 |
| `GROUP_ADD_PARTICIPANTS_ENABLED` | `false` | 3 |
| `GROUP_MAX_PARTICIPANTS_PER_REQUEST` | `256` | 3 |
| `GROUP_INFO_CACHE_TTL_SECONDS` | `30` | 3 |
| `ADMIN_API_SECRET` | `""` (empty ⇒ admin plane 404) | 4 |
| `METRICS_ENABLED` | `true` | 4 |
| `IDEMPOTENCY_ENABLED` | `true` | 4 |
| `IDEMPOTENCY_TTL_HOURS` | `24` | 4 |
| `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS` | `10` | 4 |
| `WEBHOOK_MAX_RETRIES` | `3` | 4 |
| `WEBHOOK_RETRY_BACKOFF_SECONDS` | `2` | 4 |

## Recommended CI guard (cheap drift catch)
A ~30-line test that walks registered Fiber routes and asserts each path appears in `docs/openapi.yaml`. Catches the "route exists but spec doesn't" drift for all ~35 new endpoints — the highest-leverage doc-quality investment given there is no codegen.
