# WhatsApp Gateway — Automation Platform Plan

> **Review drafts — uncommitted. Not commitments.** This directory is the planning + progress-tracking home for turning the gateway into a full WhatsApp automation platform (decision #1). Nothing here is published to an issue tracker. Edit the **Status** cells as work lands — this file is the single tracker.

## How to use this

- **[`GOAL.md`](./GOAL.md)** = the north-star implementation charter (objective, definition of done, guardrails, phase-by-phase checklist). **Pin this** — it's the "refer to one file" doc.
- **This file** = master index + progress tracker + resolved/open decisions + the grilling record.
- **Per-initiative PRDs** (`prd-0` … `prd-6`) = full spec, user stories, edge cases, error semantics, testing seams, per-point review checkboxes, doc surfaces. Each has its own **Progress** table (per endpoint).
- **[`00-sequence.md`](./00-sequence.md)** = the priority-ordered development sequence across everything.
- **[`docs-surface-matrix.md`](./docs-surface-matrix.md)** = every doc surface (openapi, llms.txt, wiki, docs/ui, CLAUDE.md, SDK, MCP) per feature + the master env-key list.
- **[`../adr/`](../adr/)** = the 3 architecture decisions. **[`../../CONTEXT.md`](../../CONTEXT.md)** = the domain glossary (Chat/Account/JID/…).

Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

## Progress tracker

| PRD | Initiative | Seq phase | Ban-risk | Effort | Status |
|---|---|---|---|---|---|
| [0](./prd-0-addressing-foundation.md) | Addressing foundation (`chat` recipient) | A (1st) | none | S | ☑ shipped (8cb509b, f2c600f, af0fb44) |
| [4·B](./prd-4-ops-hardening.md) | Idempotency + health split + graceful shutdown | B | none | M | ☑ shipped — idempotency (0d07469/2685b27), health (ae5eb3f), shutdown (feedd04) |
| [1](./prd-1-read-query-surface.md) | Read/query surface (contacts, groups, profile, avatar) | B | low | M | ☑ shipped — contacts (e4deb71), cache/budget+list-groups (f6591f2), group-info+contact-info (25f89c3), avatar (32309e5); docs 03fd0aa/f6c796d/955f1c9 |
| [2](./prd-2-two-way-primitives.md) | Two-way primitives (read receipts, typing, replies, mentions) | C | low | M | ◐ mark-read + typing + action-cap shipped (5bf97eb/e8ef549/05e2b64); reply_to + mentions next (needs live-verify of reply ContextInfo) |
| [3·reads](./prd-3-group-community-management.md) | Group + community **reads** | C | low | — | ☐ |
| [4·D](./prd-4-ops-hardening.md) | Session-status events → admin inventory + metrics | D | none | M | ☐ |
| [3·mut](./prd-3-group-community-management.md) | Group + community **mutations** (gated) | E | high | L | ⏸ |
| [4·F](./prd-4-ops-hardening.md) | Multi-URL / per-event webhook subscriptions | F (→ #1) | none | L | ⏸ |
| — | Roadmap **#1** webhook event system | after | none | M | ⏸ |
| — | Roadmap **#2** outbound pacing | after | — | M | ⏸ |
| [5](./prd-5-sdk-parity.md) | Go SDK full parity | trails each wave | n/a | L | ☐ |
| [6](./prd-6-mcp-curated.md) | MCP curated subset | trails SDK | low–med | M | ☐ |

### ADRs
| ADR | Decision | Status |
|---|---|---|
| [0001](../adr/0001-chat-canonical-msisdn-alias.md) | `chat` canonical, `msisdn` permanent alias, additive in `/api/v1` | proposed |
| [0002](../adr/0002-whatsmeow-error-sentinels.md) | whatsmeow error mapping via `errors.Is/As` sentinels, not substring | proposed |
| [0003](../adr/0003-send-idempotency-db-backed.md) | Send idempotency DB-backed `(phone,key)`, enqueued-once | proposed |

## Resolved decisions (from the grilling session)

1. **Vision** — full automation platform is the *destination*, delivered sequenced.
2. **Addressing** — unified `chat` (msisdn | PN JID | `@g.us` | `@lid`); `msisdn` permanent alias; `chat` wins. → ADR 0001.
3. **Queue/limits** — reads direct+cached (+ query budget); conversation actions paced through the send path.
4. **Admin plane** — `ADMIN_API_SECRET` bearer; `/admin/*` + `/metrics` 404 when unset.
5. **Replies** — lightweight, caller-supplied context; no durable message store now.
6. **Contacts** — expose local synced store as-is + honest metadata; force-resync deferred.
7. **Sequencing** — automation-platform first; roadmap #1/#2 after; ban-risky bulk ops gated until pacing; new webhook event *types* deferred to #1.
8. **SDK/MCP** — SDK full parity (admin separate); MCP curated safe subset (no group mutations, no admin).
9. **Deliverable** — local PRDs + this tracker, uncommitted; not published.

## Cross-cutting decisions still needing your sign-off

These span multiple PRDs — resolve once here (each PRD's own "Decisions for Review" covers its local ones):

- [ ] **Path convention for a `chat` target** — the read PRD sketched `?chat=` query params; the group PRD sketched `/group/:chat/...` path segments. **Recommend: `chat` as a query param on reads and in the body on actions everywhere** (avoids URL-encoding `@`/`+` in JIDs, matches the `chat` field name). Group sub-resources become `POST /group/participants` etc. with `chat` in the body.
- [ ] **Root vs BasePath** — `/metrics` + `/health/live|ready` at **root** (Prometheus/k8s convention), `/admin/*` at root, keep `/health` under `/api/v1` as alias. Recommend as stated; it drives reverse-proxy + probe config.
- [ ] **`ErrGone` → 410** — add a 410 domain error for revoked invite links (vs reusing 409). Recommend add it — keeps "revoked, get a new link" distinct from "invalid input".
- [ ] **Echo resolved `chat` on send responses** — add the canonical resolved JID alongside `message_id`, in the same wave as the addressing change. Recommend yes.
- [ ] **Interim action pacing** — until roadmap #2, reuse the existing limiter as a best-effort per-account cap on mark-read/typing/react (key `action:`+phone); keep bulk-add/mass-join hard-gated. Recommend yes; document the ceiling.
- [ ] **CI drift guard** — add a ~30-line test asserting every registered route exists in `docs/openapi.yaml` (no codegen today). Recommend yes — highest-leverage doc-quality investment.

## Grilling record (traceability — how we got here)

| # | Question | Decision |
|---|---|---|
| 1 | Strategic posture | Full automation platform |
| 2 | Recipient model | Unified `chat` (msisdn back-compat alias) |
| 3 | Queue/limiter interaction for new endpoints | Split by nature (reads direct+cached, actions paced) |
| 4 | Admin/operator auth | Dedicated `ADMIN_API_SECRET`, 404 when unset |
| 5 | Replies / message store | Lightweight now, durable store deferred |
| 6 | Contacts sync gap | Expose local store as-is + honest metadata |
| 7 | Sequence vs roadmap #1/#2 | Automation-platform first, #1/#2 after (+ mitigations) |
| 8 | SDK vs MCP scope | SDK full; MCP curated (no destructive/admin) |
| 9 | Progress traceability | Local PRDs + this index, uncommitted |

Grounded by two edge-case mining passes over the whatsmeow source + current code (7 + 6 agents), incl. a completeness critic that resolved 7 cross-cutting contradictions and confirmed the real webhook constants (`message.incoming/queued/sent/failed`), the hand-maintained `/api/v1` OpenAPI, and dual-dialect (SQLite+Postgres) migrations.

## What's explicitly NOT in this plan
- **Durable message store / chat history** — its own deferred initiative (replies stay lightweight until it exists).
- **JWT revocation / single-active-token** — declined in `ROADMAP.md` §5 (gateway is backend-internal).
- **Broadcast-list send, status/stories, newsletters/channels, battery/screenshot** — out of scope (low value or impossible on whatsmeow).

## Next step
Review the cross-cutting checkboxes above and each PRD's **Decisions for Review**. Nothing is committed. When you're happy, we start at **Phase A / PRD 0** (addressing) — it unblocks everything and is backward-compatible.
