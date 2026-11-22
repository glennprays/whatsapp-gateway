---
status: accepted
---

> **Implementation note (2026-07-14, commit 0d07469):** enforced as HTTP middleware on `/api/message/*` (after JWT auth, before the handler — hence before the queue/direct fork), which lets it capture and replay the exact `http_status`+body snapshot the design calls for. Insert-first uses `INSERT ... ON CONFLICT (phone_number, idempotency_key) DO NOTHING` + `RowsAffected()` (portable across SQLite/Postgres, no driver-specific error parsing). A `pending` row left by a crashed request is taken over after `IDEMPOTENCY_PENDING_TIMEOUT_SECONDS`; a background sweeper deletes rows past `IDEMPOTENCY_TTL_SECONDS`.

# 0003 — Send idempotency is DB-backed, keyed `(phone_number, key)`, replays a stored response, and guarantees enqueued-once not delivered-once

Send endpoints accept a client `Idempotency-Key` header. We dedupe with a new DB-backed `idempotency_keys` table keyed `UNIQUE(phone_number, key)` (phone taken from the JWT, never the body, so one Account can't spoof another's namespace), checked in the usecase **before the queue/direct fork** so it covers both modes and survives restart. On replay we return the **stored original response snapshot** (`http_status` + body) verbatim with `Idempotent-Replay: true`.

## Considered options

- **Reuse the existing in-memory `DedupCache`** — rejected: it's lost on restart, single-instance, and keyed by internal `JobID` (useless as a client-facing key); direct mode creates no job record at all.
- **Key globally on `key` alone** — rejected: collides across tenants.

## Consequences

- Concurrency is **insert-first**: `INSERT ... status=pending` guarded by the unique constraint. Winner proceeds; a loser that hits the constraint returns `409` if the row is still pending, or replays the stored response if completed — the send never runs twice.
- Key reuse with a **different** request body (SHA-256 of the canonicalized request mismatches) → `422`, never a silent re-send or stale result.
- In queue mode the stored replay is `202 + job_id`, i.e. this guarantees **exactly-once enqueue, not exactly-once delivery** — a re-enqueued job can still fail or deliver twice downstream. This boundary must be documented in the header contract; it is inherent to queue mode and is NOT to be "fixed" by rebuilding an outbox in direct mode.
- New dual-dialect (SQLite + Postgres) migration; a TTL sweeper (or lazy delete-on-read) bounds table growth.
- The `idempotency_keys` table is the one piece of send state that is cross-instance-safe (unlike `DedupCache`).
