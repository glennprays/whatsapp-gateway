# PRD 4 — Ops / Gateway-Grade Hardening

> **Status:** Not started  ·  **Sequence position:** 2 (idempotency/health/shutdown) & 6 (admin/metrics/session-status) & deferred (multi-URL webhooks → #1 window)  ·  **Ban-risk:** none
> **Depends on:** none   ·   **Effort rollup:** L
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | Deliverable (endpoint / change) | whatsmeow call | Effort | Ban-risk | Status |
|---|---|---|---|---|---|
| 1 | Admin session inventory — `GET /admin/sessions` (+ `GET /admin/sessions/:phone`) | `container.GetAllDevices` + per-client `IsConnected`/`IsLoggedIn` | M | none | ☐ |
| 2 | Session-status event tracking (banned / logged-out / connect-failure) | `events.LoggedOut/TemporaryBan/ConnectFailure/StreamReplaced` cases in `HandleEvent` (no webhook emitted) | M | none | ☐ |
| 3 | Prometheus `GET /metrics` (aggregate counters + session-state gauge) | gateway-owned (`prometheus/client_golang`) | M | none | ☐ |
| 4 | Send idempotency — `Idempotency-Key` header on `POST /message/*` | gateway-owned; DB-backed, checked before queue/direct fork | L | none | ☐ |
| 5 | Graceful multi-account shutdown (Disconnect N clients) | range `ClientStore.GetAll` → `client.Disconnect` under bounded WG + timeout | S | none | ☐ |
| 6 | Health + readiness split — `GET /health/live` + `GET /health/ready` (keep `/health` alias) | gateway-owned (`db.PingContext`, `queue.IsHealthy`) | S | none | ☐ |
| 7 | Direct-mode webhook retry parity (bounded backoff, async) | gateway-owned; retry loop in `WebhookSender` on a goroutine | M | none | ☐ |
| 8 | Multi-URL webhooks + per-subscription event filter | gateway-owned; `device_webhook_subscriptions(jid,url,hmac,events[])` | L | none | ⏸ |

## Problem Statement

A backend integrator running this gateway in production today is flying blind and unsafe:

- **No fleet visibility.** whatsmeow deletes the device row on `events.LoggedOut`, and a ban is only observable via `events.TemporaryBan`/`ConnectFailure` — none of which `HandleEvent` handles. A dead or banned session looks identical to a healthy one. There is no way to enumerate accounts and their state.
- **No metrics.** There is no scrape endpoint; message throughput, webhook delivery success, queue depth, and per-state session counts are invisible to Prometheus/Grafana.
- **Sends aren't idempotent.** A client retry-on-timeout or double-submit sends the message twice. The existing in-memory `DedupCache` is keyed by internal `JobID`, lost on restart, single-instance, and absent in direct mode — useless as a client-facing idempotency store.
- **Shutdown is unclean.** On SIGTERM there is no step to disconnect whatsmeow clients before the shared DB closes, so session state may not flush.
- **Health is ambiguous.** A single `/health` conflates liveness and readiness; there is no clean k8s liveness/readiness split, and readiness risks being coupled to session health — which would drain a node that can still serve `/register` and QR pairing.
- **Webhooks are single-URL and direct-mode-lossy.** One URL per device, no per-event filtering; direct-mode status webhooks are a single inline 5s attempt that stalls the response and silently drops on any transient receiver blip.

## Solution

Add a gateway-grade ops plane, all additive within `/api/v1` (no `/v2`):

- A dedicated **admin plane** behind `ADMIN_API_SECRET` (bearer, constant-time compare). `/admin/*` and `/metrics` **return 404 when the secret is unset** (routes never registered) and 401 on bad bearer when set (decision #4).
- **Session-status event tracking** first (it makes inventory honest): new `HandleEvent` cases persist `{connected, disconnected, never_paired, logged_out, banned, ban_expires_at}` per phone and evict dead clients. No webhook is emitted (decision #7 defers new event *types*); this IS the code roadmap #1 §3.4 needs — one owner writes the switch now, #1 later only adds emission.
- **`GET /admin/sessions`** merges the in-memory client set + `container.GetAllDevices` + persisted status into an honest, per-instance inventory (phones masked, source + `last_seen` labelled).
- **Prometheus `/metrics`** with aggregate counters and a single `sessions{state=...}` gauge — never labelled by phone.
- **DB-backed send idempotency** on the `Idempotency-Key` header, keyed `(phone_number, key)`, checked before the queue/direct fork so it covers both modes and survives restart; replays the original stored response verbatim. Guarantees **enqueued-once, NOT delivered-once** in queue mode (documented loudly).
- **Graceful multi-account shutdown** disconnecting N clients concurrently under a timeout, after HTTP stop but before wire cleanup closes the DB.
- **Health/readiness split** — liveness always 200; readiness = DB + queue only, decoupled from session health.
- **Direct-mode webhook retry parity** — fire status webhooks on a goroutine with bounded exponential backoff (best-effort; durability stays queue mode).
- **Multi-URL / per-event webhook subscriptions** are plumbed but **deferred into the #1 envelope window** to avoid migrating `device_webhooks` twice.

## User Stories

1. As an **operator**, I want `GET /admin/sessions` to list every account this instance holds with its derived state, so I can see the fleet without shelling into the DB.
2. As an **operator**, I want `GET /admin/sessions/:phone` for a single account, so I can drill into one tenant during an incident.
3. As an **operator**, I want banned and logged-out accounts distinguished from healthy ones, so a dead session doesn't masquerade as connected.
4. As an **operator**, I want each inventory entry labelled with its source (in-memory vs store) and `last_seen`, so I can tell a confirmed state from a heuristic.
5. As an **operator**, I want `never_paired` derived (in-memory client with `Store.ID==nil`), so I can see accounts registered but never QR-paired.
6. As an **operator**, I want session-status changes (logged-out / temporary-ban / connect-failure / stream-replaced) persisted, so inventory and the metrics gauge reflect reality even after whatsmeow drops the device row.
7. As an **operator**, I want a dead client evicted on `events.LoggedOut`, so it stops appearing as a live socket.
8. As an **operator**, I want `GET /metrics` in Prometheus format, so I can graph and alert in Grafana.
9. As an **operator**, I want message throughput as `messages_total{type,mode,result}`, so I can see send volume and failure rate at a glance.
10. As an **operator**, I want `webhook_deliveries_total{result,mode,event}`, so I can catch a failing webhook receiver.
11. As an **operator**, I want a single `sessions{state=...}` gauge, so I can alert on connected-count dropping without per-account cardinality.
12. As an **operator**, I want metrics to **never** be labelled by phone number, so an unbounded tenant count can't OOM Prometheus.
13. As an **operator**, I want `/admin/*` and `/metrics` to return 404 when `ADMIN_API_SECRET` is unset, so the mere existence of an admin plane doesn't leak.
14. As an **operator**, I want a wrong/missing bearer to return 401 (secret set) with a constant-time compare, so the admin plane isn't brute-forceable via timing.
15. As an **operator**, I want `/metrics` toggleable via `METRICS_ENABLED` independently of the admin API, so I can turn off scraping without disabling `/admin`.
16. As a **backend integrator**, I want to send an `Idempotency-Key` header on `POST /message/*`, so a retry-on-timeout doesn't send the message twice.
17. As a **backend integrator**, I want the idempotency key scoped `(phone_number, key)` from my JWT, so I can't collide with — or spoof — another tenant's namespace.
18. As a **backend integrator**, I want a replay of a completed request to return the **original** status + body (200+message_id direct / 202+job_id queue) with `Idempotent-Replay: true`, so retries are transparent even if the mode changed between calls.
19. As a **backend integrator**, I want a concurrent duplicate (still in-flight) to get 409 rather than a second send, so a double-click never double-sends.
20. As a **backend integrator**, I want an `Idempotency-Key` reused with a *different* payload to be rejected 422, so a client bug can't silently send the wrong thing or return a stale result.
21. As a **backend integrator**, I want idempotency to survive a gateway restart, so an in-flight retry after a deploy still de-dups.
22. As a **backend integrator**, I want the header contract to state **enqueued-once, not delivered-once** in queue mode, so I don't wrongly assume exactly-once delivery.
23. As a **backend integrator**, I want a clean `GET /health/live` that is always 200, so my k8s liveness probe only restarts a truly hung process.
24. As a **backend integrator**, I want `GET /health/ready` to reflect DB + queue health only, so a node with all sessions logged-out still serves `/register` and QR pairing instead of being drained.
25. As a **backend integrator**, I want the existing `/health` kept as a readiness alias, so nothing that already points at it breaks.
26. As an **operator**, I want graceful shutdown to disconnect all whatsmeow clients concurrently under `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS`, so state flushes before the process exits.
27. As an **operator**, I want a hung `Disconnect` logged-and-skipped rather than blocking, so shutdown never runs past the SIGTERM grace period into SIGKILL.
28. As an **operator**, I want client disconnect to run after HTTP stop but before the DB closes, so whatsmeow can flush to the shared store.
29. As a **backend integrator**, I want direct-mode status webhooks fired async with bounded retry, so a slow receiver doesn't stall my API response and a transient blip doesn't permanently lose `message.sent`/`message.failed`.
30. As a **backend integrator**, I want direct-mode webhook retry documented as best-effort/in-memory, so I know durable delivery means enabling RabbitMQ.
31. As a **backend integrator (future / #1 window)**, I want to register multiple webhook URLs with per-subscription event filters, so different consumers get different event streams.
32. As a **backend integrator**, I want subscribing to an event outside the fixed catalog rejected 400, so the surface is ready when #1 expands the catalog without inventing events now.
33. As an **operator**, I want inventory and metrics documented as **per-instance** with a hostname/instance field, so I understand a whatsmeow session is pinned to the process holding its socket.
34. As an **operator**, I want the idempotency table called out as the one cross-instance-safe record, so I know it (unlike `DedupCache`) works across a horizontally-scaled fleet.

## Implementation Decisions

### Admin plane & routing
- New `internal/middleware/admin.go`: bearer middleware, constant-time compare against `cfg.AdminAPISecret`. Guards `/admin/*` and `/metrics`.
- **Route mounting (decision to record):** `/metrics` and `/health/live|ready` live at **ROOT** (Prometheus/k8s convention). `/admin` mounts at **root**. Existing `/health` stays under `cfg.BasePath` (`/api/v1`) as a readiness alias.
- When `ADMIN_API_SECRET` is empty, the router does **not register** `/admin/*` or `/metrics` at all → Fiber default 404 (no hint the plane exists). When set: 401 on bad/missing bearer.

### Session-status tracking (prerequisite for honest inventory)
- New `HandleEvent` cases: `events.LoggedOut`, `events.TemporaryBan`, `events.ConnectFailure`, `events.StreamReplaced`. Persist status + timestamp to a small status table/column keyed by phone. **No webhook emitted** (decision #7). One owner for the switch — ops writes the cases; #1 later only adds emission.
- Derived states: `never_paired` = in-memory client with `Store.ID==nil` (no device row); `logged_out` = device row vanished while client lingers; `banned` = last `TemporaryBan`/`ConnectFailure`. Each entry labelled `source` (in-memory vs store) and `last_seen`.

### Admin session inventory
- `manager.SessionInventory()` merges `ClientStore.GetAll` + `container.GetAllDevices` + persisted status. Handler masks phones. Response includes an instance/hostname field (per-instance honesty).
- `GET /admin/sessions` → 200 with list (empty list + instance id when zero sessions, not an error); `GET /admin/sessions/:phone` for one.

### Prometheus metrics
- New `internal/metrics/metrics.go`: own registry + collectors. Instrument ~4 hot paths: send path, webhook deliveries, queue depth, session-state gauge.
- Counters: `messages_total{type,mode,result}`, `webhook_deliveries_total{result,mode,event}`. Gauge: `sessions{state="connected|disconnected|banned|logged_out|never_paired"}`. **Never** a phone label.

### Send idempotency (DB-backed, cross-mode)
- New `internal/idempotency` package + `idempotency_keys` table: `UNIQUE(phone_number, key)`, columns `status`, `request_hash` (SHA-256 of canonicalized body), `http_status`, `response_snapshot`, `created_at`. Dual-dialect (SQLite + Postgres) migration.
- Checked in the usecase at the **top of each `Send*`** method, before the queue/direct fork (helper wired into ~10 send methods). Insert-first concurrency: `INSERT ... status=pending` guarded by the unique constraint. Winner proceeds and updates the row with the final `{http_status, response_snapshot}`; loser on constraint violation → pending ⇒ 409, completed ⇒ replay snapshot + `Idempotent-Replay: true`, `request_hash` mismatch ⇒ 422.
- Do **not** reuse `DedupCache` (in-memory, `JobID`-keyed, queue-only) or `message_jobs`.
- **Contract:** guarantees enqueued-once, not delivered-once in queue mode. Document in header contract + wiki.
- Retention/cleanup governed by `IDEMPOTENCY_TTL_HOURS` (see Decisions for Review on sweeper vs lazy).

### Graceful shutdown
- New step in `internal/infrastructure/shutdown.go` inserted between `shutdownHTTPServer` and wire `cleanup()`. `manager.DisconnectAll(timeout)` ranges `ClientStore.GetAll`, disconnects each on a bounded `WaitGroup` under a single `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS` context; log-and-continue on timeout. Runs after HTTP stop, before DB close.

### Health/readiness split
- Two thin handlers reusing existing checks. `GET /health/live` → always 200. `GET /health/ready` → 200 when DB reachable (`db.PingContext`) and queue healthy (`queue.IsHealthy`, if RabbitMQ enabled); else 503 with detail. Readiness is **not** coupled to session state.

### Direct-mode webhook retry parity
- `internal/whatsapp/webhook_sender.go`: bounded exponential-backoff retry loop (`WEBHOOK_MAX_RETRIES`, `WEBHOOK_RETRY_BACKOFF_SECONDS`). Fire direct-mode status webhooks on a goroutine, mirroring the incoming `asyncDeliverWebhook`, so they don't block the HTTP response. Keep the existing SSRF-guarded dialer. Best-effort in-memory — no DLQ in direct mode.

### Multi-URL / per-event subscriptions (DEFERRED to #1 window)
- New `device_webhook_subscriptions(jid, url, hmac, events[])` replacing single-row `device_webhooks`, with a **data-preserving, dual-dialect** migration (copy each existing URL → one subscription with the full fixed event set). `GET/POST/DELETE /webhook` rewritten to manage N subscriptions. Per-subscription `events[]` validated against the **actual fixed catalog** `{message.incoming, message.queued, message.sent, message.failed}`; unknown → 400. Deferred to avoid migrating twice against #1's envelope change.

### Config additions

| Key | Purpose | Default |
|---|---|---|
| `ADMIN_API_SECRET` | Bearer for `/admin/*` and `/metrics`; whole surface 404 when unset (decision #4) | `""` (empty = admin plane disabled/404) |
| `METRICS_ENABLED` | Toggle Prometheus `/metrics` independently (still needs `ADMIN_API_SECRET` to be reachable) | `true` |
| `IDEMPOTENCY_ENABLED` | Toggle DB-backed send idempotency on the `Idempotency-Key` header | `true` |
| `IDEMPOTENCY_TTL_HOURS` | Retention/replay window before cleanup | `24` |
| `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS` | Overall bound for disconnecting all clients on shutdown | `10` |
| `WEBHOOK_MAX_RETRIES` | Direct-mode webhook retry attempts (parity with queue mode) | `3` |
| `WEBHOOK_RETRY_BACKOFF_SECONDS` | Base for exponential backoff between direct-mode retries | `2` |

### Request/response contract sketches
- **Send with idempotency** — request header `Idempotency-Key: <opaque>`; response on replay adds header `Idempotent-Replay: true` and returns the stored `{http_status, body}`.
- **`GET /admin/sessions`** → `{ instance: <hostname>, sessions: [ { phone_masked, state, source, last_seen, ban_expires_at? } ] }`.
- **`GET /health/ready`** → 200 `{ status:"ready" }` / 503 `{ status:"not_ready", database, queue }`.
- **`POST /webhook`** (deferred) → `{ url, events: [<subset of fixed catalog>] }` per subscription.

### Integration seams

| File | What |
|---|---|
| `internal/router/router.go` | Conditionally mount `/admin` group, `/metrics`, `/health/live`, `/health/ready` only when `ADMIN_API_SECRET` set; keep `/health` as readiness alias |
| `internal/middleware/admin.go` (new) | Admin bearer-auth middleware; constant-time compare vs `cfg.AdminAPISecret`; used by `/admin/*` and `/metrics` |
| `internal/handler/admin/session_handler.go` (new) | `GET /admin/sessions` inventory handler; calls `manager.SessionInventory()` |
| `internal/whatsapp/manager.go` | Add `SessionInventory()` (merge `ClientStore.GetAll` + `container.GetAllDevices` + persisted status) and `DisconnectAll(timeout)` |
| `internal/whatsapp/handler.go` | Add `events.LoggedOut/TemporaryBan/ConnectFailure/StreamReplaced` cases persisting per-session status |
| `internal/infrastructure/shutdown.go` | Insert client-disconnect step between `shutdownHTTPServer` and `cleanup` (before wire cleanup closes DB) |
| `internal/usecase/whatsapp/message_usecase.go` | Idempotency check helper at top of each `Send*` (before queue/direct fork); store + replay original response |
| `internal/idempotency/repository.go` + migration (new) | `idempotency_keys` table `UNIQUE(phone_number, key)` with status, request_hash, http_status, response_snapshot, created_at; TTL cleanup |
| `internal/whatsapp/webhook_sender.go` | Bounded exponential-backoff retry loop (config-driven) for direct-mode parity; keep SSRF dialer |
| `internal/metrics/metrics.go` (new) + `internal/whatsapp/repository.go` | Prometheus registry + collectors; replace single-row `device_webhooks` with `device_webhook_subscriptions(jid,url,hmac,events[])`; new migration |
| `config/config.go` | Add new env fields (admin secret, idempotency, shutdown timeout, webhook retry, metrics toggle) |

## Edge Cases

| Scenario | Risk | Handling |
|---|---|---|
| Dead session looks healthy: whatsmeow deletes the device row on `LoggedOut`; ban only visible via `TemporaryBan`/`ConnectFailure` — none handled today | correctness | Ship session-status tracking first; persist `{connected, disconnected, never_paired, logged_out, banned, ban_expires_at}` keyed by phone; label each entry with `source` + `last_seen` |
| Prometheus per-account (`phone_number`) labels → unbounded time series | data-loss | Never label by phone; aggregate counters + single `sessions{state=...}` gauge; per-account detail in `/admin/sessions` |
| Same `Idempotency-Key` value from two different tenants | correctness | Composite `UNIQUE(phone_number, key)`; phone taken from JWT context, never the body, so no cross-account spoof |
| Idempotency must survive restart AND cover both modes; `DedupCache` is in-memory/`JobID`-keyed/queue-only | correctness | New DB-backed `idempotency_keys` table, checked at usecase entry before the queue/direct fork; don't reuse `DedupCache` or `message_jobs` |
| Replay must return the original response shape (202+job_id queue / 200+message_id direct) | correctness | Store `{http_status, response_snapshot}` and replay verbatim with `Idempotent-Replay: true`; don't re-derive from current mode (mode may have changed) |
| Concurrent duplicates arrive before the first completes (double-click / retry-on-timeout) | correctness | Insert-first with `status=pending` under the unique constraint; winner proceeds, loser → pending ⇒ 409, completed ⇒ replay; never send twice |
| `Idempotency-Key` reused with a different body | correctness | Compare stored SHA-256 of canonicalized request; mismatch → 422; never silently send or return stale result |
| A hung `Disconnect` on one account blocks shutdown past the SIGTERM grace period | data-loss | Disconnect concurrently under one timeout ctx; log-and-continue on timeout; run after HTTP stop but before wire `cleanup` closes the DB |
| Readiness coupled to session health drains a node whose accounts are all logged-out/unpaired but can still serve `/register` + QR | confusing-UX | Do not couple; readiness = DB + queue only; liveness always 200; session health surfaced via `/admin/sessions` + gauge (alerts without evicting the pod) |
| Direct-mode status webhook is a single inline 5s attempt — slow receiver stalls the response; a blip permanently loses `sent`/`failed` | data-loss | Fire async on a goroutine with bounded backoff retry; document best-effort/in-memory (durability = enable RabbitMQ; don't rebuild the outbox in direct mode) |
| Multi-URL + per-event subscriptions vs the deferred event catalog (#7) | confusing-UX | Ship plumbing (N subs, `events[]` filter) but keep the vocabulary the current fixed set; validate against it and reject unknown values |
| Horizontal scaling: session pinned to its process; inventory + gauge are node-local | confusing-UX | Document inventory/metrics as per-instance; include instance/hostname field; call out the idempotency table as the one cross-instance-safe piece |
| `/admin/*` or `/metrics` must 404 (not 401) when `ADMIN_API_SECRET` unset, else the plane's existence leaks | security | When empty, don't register the routes (Fiber default 404); when set, missing/wrong bearer → 401; constant-time compare |

## Error Semantics

| Condition | Response |
|---|---|
| Any `/admin/*` or `/metrics` request when `ADMIN_API_SECRET` unset | 404 Not Found (routes never registered; no hint the plane exists) |
| `/admin/*` or `/metrics` with missing/wrong bearer (secret set) | 401 Unauthorized, `{"error":"unauthorized"}` |
| `Idempotency-Key` replay of a COMPLETED request | Original `http_status` (200 direct / 202 queue) + original body snapshot + header `Idempotent-Replay: true` |
| `Idempotency-Key` still in-flight (pending) | 409 Conflict, `{"error":"request with this idempotency key is still processing"}` |
| `Idempotency-Key` reused with a different body (hash mismatch) | 422 Unprocessable Entity, `{"error":"idempotency key reused with a different payload"}` |
| `GET /health/ready` with DB unreachable or (RabbitMQ enabled and) queue down | 503 Service Unavailable, `{status:"not_ready", database/queue detail}` |
| `GET /health/live` | 200 OK always (process serving); reflects neither DB, queue, nor session state |
| `GET /admin/sessions` when node holds zero sessions | 200 OK, empty list + instance identifier (not an error) |
| Direct-mode webhook exhausts `WEBHOOK_MAX_RETRIES` | No HTTP effect (async); logged at error with full context; no DLQ in direct mode (documented best-effort) |
| `POST /webhook` subscribing to an event not in the fixed catalog | 400 Bad Request, `{"error":"unknown event type"}` (guards the deferred #1 catalog) |
| Graceful shutdown: a client `Disconnect` exceeds the timeout | Logged and skipped; shutdown proceeds (never blocks past grace period, never fails the process) |

## Testing Decisions

Highest-seam-first. Most of this area is gateway-owned, so it is unusually unit-testable (unlike the whatsmeow send/read paths in sibling PRDs).

- **Idempotency (usecase layer + repository) — the one correctness path needing a real concurrency test.** Unit: `request_hash` canonicalization + mismatch → 422; replay returns the stored status/body verbatim. Concurrency: insert-first winner/loser with concurrent goroutines against the unique constraint (SQLite in-memory or the existing pg testcontainer). Persistence: table survives a simulated restart.
- **`resolveChat`-adjacent / admin auth (middleware seam):** unit-test the constant-time compare (set/unset/wrong), and that routes are unregistered → 404 when secret empty.
- **Health handlers:** unit-test liveness always-200 and readiness 200/503 by stubbing the DB ping + `queue.IsHealthy`.
- **Session-status derivation (`SessionInventory`):** unit-test the merge of in-memory clients + device rows + persisted status → derived state (`never_paired`/`logged_out`/`banned`), stubbing `ClientStore.GetAll` / `GetAllDevices`.
- **Webhook retry (`WebhookSender`):** unit-test backoff attempts and give-up against an httptest server; assert async (non-blocking) fire.
- **Multi-URL subscriptions (deferred):** extend the testcontainers RabbitMQ harness for the repo round-trip + event-filter enforcement + the `device_webhooks → subscriptions` data-preserving migration on BOTH SQLite and Postgres.
- **`HandleEvent` session-status cases:** whatsmeow events (`LoggedOut/TemporaryBan/ConnectFailure/StreamReplaced`) are **not** integration-testable in CI (no live WhatsApp). Verify once against the live build in a manual harness, then lock the mapping to exported sentinels. **ADR:** error/event mapping uses `errors.Is/As` sentinels (+ `IQError.Code`), NOT substring matching — the existing `isRecipientError` substring pattern is the trap; sentinels are the only thing that makes this unit-testable against constructed error values.

## Decisions for Review

- [ ] **Metrics scrape auth** — reuse `ADMIN_API_SECRET` as the scrape bearer (Prometheus supports `bearer_token` in scrape config) rather than a separate `METRICS_API_SECRET`. *Recommend: reuse* — simplest, one secret to rotate; confirm the ops team's scrape setup accepts a bearer.
- [ ] **Readiness never fails on all-sessions-down** — decouple readiness from session health by default. *Recommend: decouple.* If some deployment genuinely wants "no live session = drain me", gate that behind an explicit opt-in flag rather than the default.
- [ ] **Internal session-status tracking with no webhook** — add `TemporaryBan/LoggedOut/ConnectFailure/StreamReplaced` handling + a small status store now, emitting **no** webhook (decision #7 defers new event *types*). *Recommend: yes* — this is the code #1 §3.4 needs; #1 adds emission on top.
- [ ] **Idempotency replay body storage** — store the full response snapshot (incl. recipient/job_id) vs storing only status+message_id/job_id and reconstructing a minimal body. *Recommend: store the full snapshot* — cheap, and guarantees byte-exact replay across a mode change; TTL bounds the storage.
- [ ] **Multi-URL / per-event webhooks (feature 8)** — defer the whole feature into the #1 envelope window vs ship the multi-URL plumbing now with today's fixed event set. *Recommend: defer* — heaviest item, overlaps #1's catalog, and shipping now means a second data migration off `device_webhooks`.
- [ ] **Per-instance inventory/metrics honesty** — expose per-instance data with a hostname field rather than attempting cross-instance aggregation (no shared registry today). *Recommend: per-instance + hostname field.*
- [ ] **Idempotency key cleanup** — background sweeper goroutine vs lazy delete-on-read past TTL. *Recommend: lazy delete-on-read* (simpler, no extra goroutine); add a sweeper only if stale rows measurably accumulate.
- [ ] **Direct-mode send history** — also write a `message_jobs` row in direct mode (so `/admin` has send history in both modes) vs treat the `idempotency_keys` table as the single cross-mode record. *Recommend: idempotency table is the single record* for now — don't grow direct-mode writes; revisit if `/admin` needs full send history.

## Doc Surfaces

- [ ] `docs/openapi.yaml` — add `/admin/sessions` (+ `:phone`), `/metrics`, `/health/live`, `/health/ready`; add `Idempotency-Key` request header + `Idempotent-Replay` response header on `POST /message/*`; add 409/422/503 responses; (deferred) multi-URL `/webhook` schema. (`docs/openapi.go` only loads/serves the spec + injects the server URL — no codegen.)
- [ ] `llms.txt` — add the admin endpoint family + an admin-plane / idempotency note.
- [ ] `wiki/Environment-Variables.md` + `.env.example` + `CLAUDE.md` Configuration list — new keys: `ADMIN_API_SECRET`, `METRICS_ENABLED`, `IDEMPOTENCY_ENABLED`, `IDEMPOTENCY_TTL_HOURS`, `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS`, `WEBHOOK_MAX_RETRIES`, `WEBHOOK_RETRY_BACKOFF_SECONDS`.
- [ ] `wiki/Security-Considerations.md` — document the `ADMIN_API_SECRET`-404 plane, and the idempotency **enqueued-once, not delivered-once** semantics.
- [ ] `CLAUDE.md` "API Endpoints" list — add the admin + health-split endpoints.
- [ ] `docs/ui/assets/docs/*` — reference-page refresh for admin/metrics/health.
- [ ] `CHANGELOG.md` + openapi version bump.
- [ ] **SDK** (`whatsapp-gateway-sdk-go`) — full parity: `Idempotency-Key` support on send methods, admin session-inventory as a separate opt-in module (decision #8).
- [ ] **MCP** (`mcp-whatsapp-gateway`) — curated subset: **exclude** admin entirely (no admin tools), per decision #8.

## Out of Scope

- **Multi-URL / per-event webhook subscriptions (feature 8)** — plumbed here but **deferred into the roadmap #1 (webhook events) envelope window** to avoid a double schema migration off `device_webhooks`.
- **New webhook event *types*** (`session.*`, `message.delivered/read/played`, `call.offer`) — deferred to #1; this PRD writes the `HandleEvent` cases but emits no webhook (decision #7).
- **Real per-account action pacer** — roadmap #2; the interim crude action cap lives in the two-way/group PRDs.
- **Durable message store / send history in direct mode** — deferred initiative; idempotency table is the only cross-mode record here.
- **Cross-instance aggregation of inventory/metrics** — no shared registry; each instance is authoritative for its own sessions only.

## Dependencies & Sequencing

- **Depends on:** none. Idempotency, health-split, and graceful-shutdown have no dependency on the addressing foundation (PRD 1) and are best landed **early** — before automation drives up send volume and clients bake in retry logic (sequence position **2**).
- Session-status tracking → admin inventory → Prometheus metrics form the position-**6** block; they need the `HandleEvent` cases, whose ownership must be coordinated with roadmap #1 §3.4 (ops writes the switch now; #1 only adds emission).
- **Multi-URL webhooks (feature 8)** is deferred into the roadmap #1 envelope window.
- Roadmap #1 (webhook events) and #2 (pacing) land AFTER the automation platform, per decision #7.
