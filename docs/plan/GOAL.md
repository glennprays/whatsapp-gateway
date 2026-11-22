# 🎯 GOAL — WhatsApp Gateway → Full Automation Platform

> **This is the north-star / implementation charter.** If you read one file, read this. It states the objective, what "done" means, the guardrails you must not break, and a phase-by-phase checklist. Each phase links to its PRD for the fine detail.
> _Review draft — uncommitted. Not a commitment._ Detail lives in the sibling PRDs; progress is tracked in [`README.md`](./README.md).

> **🚀 EXECUTION PUSH (2026-07-14):** directive is to **complete implementation of ALL phases** (C tail → D → E → F → roadmap #1 → #2), sequenced, each landed green + committed with full doc surfaces. Cross-repo SDK/MCP trail as their own repos. Approach: parallel blueprint/grounding pass, then sequential implementation with adversarial review. Status per phase tracked below and in [`README.md`](./README.md).
>
> **Shipped so far:** Phase A ✅ · Phase B ✅ (reads + idempotency + health + shutdown) · Phase C ✅ (mark-read, typing, action-cap, reply/mentions on **all** sends, community reads) · Phase D ✅ (session-status events + eviction, `/admin/sessions`, `/metrics`, direct-mode webhook retry — af7cbd1…21aaa7b, openapi 0.21.0) · Phase E ✅ (16 group/community mutation endpoints, gated off by default, partial-failure semantics, `ErrGone`→410 — 21aaa7b…2284b74, openapi 0.22.0; adversarial review: 5 lenses, 0 confirmed findings) · Phase F ✅ + Roadmap #1 ✅ (multi-URL/per-event webhook subscriptions + generic dispatcher + `session.*` events — ad52e1b…72f2f1c, openapi 0.24.0; adversarial review: 7 confirmed findings all fixed in dced0e0/fda7c68/9e33170) · **Roadmap #2 ✅** (outbound pacer: ban gate + per-recipient cap + per-account token bucket over all outbound ops; lifts the Phase E gates — 43beaf4…72cb70d, openapi 0.25.0; adversarial review: 6 confirmed findings fixed in 5c1f53a/b3c197f/72cb70d). **Gateway phases A→F + roadmap #1/#2 all shipped.** Cross-repo **SDK full parity** (16 commits) + **MCP curated subset** (7 commits, 26 tools) also shipped & contract-reviewed — all locally green, pending an outward tag+push to release. **Entire plan implemented.**

---

## The objective

Turn the gateway from a **notification sender + validator** into a **full WhatsApp automation platform**: a backend can read the WhatsApp graph it messages into (contacts, groups, profiles, avatars), hold two-way conversations (read receipts, typing, replies, mentions), manage groups and communities, and run it as production-grade infrastructure (admin visibility, metrics, idempotent sends, clean shutdown) — all behind the existing per-Account JWT, staying backward-compatible, and **without getting numbers banned**.

Delivered **sequenced** (Phases A→F), then the two pre-existing roadmap initiatives (#1 webhook events, #2 pacing).

## Definition of Done (the whole objective)

- [ ] Every tenant can address **any Chat** (1:1 or group) via a unified `chat` field; `msisdn` still works everywhere.
- [ ] Read surface live: list contacts, contact info, avatar, list groups, group info — cached + budgeted so polling can't trip anti-spam.
- [ ] Two-way primitives live: mark-read, typing, replies, mentions.
- [ ] Group + community management live, with ban-risky bulk ops **gated off by default** until pacing exists.
- [ ] Ops-grade: admin session inventory, Prometheus `/metrics`, send idempotency, graceful multi-account shutdown, health/readiness split, direct-mode webhook retry parity.
- [ ] **SDK** (`whatsapp-gateway-sdk-go`) at full tenant parity; **MCP** (`mcp-whatsapp-gateway`) at the curated safe subset.
- [ ] Every shipped endpoint is reflected in **all** its doc surfaces (openapi, llms.txt, wiki, `.env.example`, CLAUDE.md, docs/ui, CHANGELOG) and covered by the tests it can have.
- [ ] Roadmap **#1** (webhook events) and **#2** (pacing) land after, lifting the interim gates.

---

## Guardrails — non-negotiable (violating any of these is a bug)

1. **Backward compatible.** `msisdn` is a permanent alias; additive within `/api/v1`; **no `/v2`**. Existing consumers must not break. → [ADR 0001](../adr/0001-chat-canonical-msisdn-alias.md)
2. **Ban-safety first.** Bulk participant-add, add-on-create, and mass-join stay behind enable-flags (default **off**) until #2 pacing. Conversation actions (mark-read/typing/react) run under an **interim per-account cap** meanwhile. Global online presence is default-off, caller-driven, **no gateway heartbeat**.
3. **Reads are cheap by default.** Local-store reads (contacts) uncapped; the 4 server-hitting reads are short-TTL cached + per-account **query budget** (spent only on cache miss). Never proxy raw whatsmeow calls unbudgeted.
4. **Error mapping via exported sentinels** (`errors.Is`/`errors.As` + `IQError.Code`), never substring. Verify each error once against live whatsmeow, then lock to the sentinel. → [ADR 0002](../adr/0002-whatsmeow-error-sentinels.md)
5. **Idempotency is DB-backed** `(phone, key)`, checked before the queue/direct fork; replays the stored response; guarantees **enqueued-once, not delivered-once** in queue mode (document it). → [ADR 0003](../adr/0003-send-idempotency-db-backed.md)
6. **Admin plane is dark by default.** `/admin/*` + `/metrics` return **404** when `ADMIN_API_SECRET` is unset; 401 on bad bearer when set; constant-time compare. A tenant must never see another tenant.
7. **Metrics never label by phone** (cardinality explosion). Aggregate counters + one `sessions{state}` gauge; per-account detail lives in `/admin/sessions`.
8. **Don't normalize phone numbers.** Gateway expects E.164 digits; leading-zero/national-format is the wrapping backend's job (no libphonenumber). Group endpoints require an explicit `@g.us` (bare digits mean a *user*).
9. **Every migration is dual-dialect** (SQLite + Postgres) and **data-preserving**.
10. **New webhook event *types* are deferred to roadmap #1.** Phases here ship request/response only; Phase D persists session-status internally (no new webhook emitted) — that code is what #1 §3.4 builds on.
11. **Docs + cross-repo are part of "done."** A route that exists but isn't in `openapi.yaml`/SDK/MCP is not shipped.

---

## Locked decisions (quick ref)

| # | Decision |
|---|---|
| 1 | Full automation platform, sequenced |
| 2 | Unified `chat` recipient; `msisdn` permanent alias; `chat` wins |
| 3 | Reads direct+cached (+ query budget); actions paced through the send path |
| 4 | `ADMIN_API_SECRET` bearer; admin/metrics 404 when unset |
| 5 | Lightweight replies; durable message store deferred |
| 6 | Contacts: local synced store as-is + honest metadata |
| 7 | Automation first → #1 → #2; ban-risky bulk ops gated |
| 8 | SDK full parity; MCP curated (no group mutations, no admin) |
| 9 | Local uncommitted PRDs + tracker |

## Lock these BEFORE starting Phase A
(cross-cutting; recommendations in [`README.md`](./README.md#cross-cutting-decisions-still-needing-your-sign-off))
- [ ] Path convention (`?chat=` query vs `/group/:chat` path) — recommend `chat` as query/body param everywhere.
- [ ] Root vs `/api/v1` for `/metrics` + `/health/live|ready` + `/admin/*` — recommend root; keep `/health` alias under BasePath.
- [ ] Add `ErrGone`→410 for revoked invite links — recommend yes.
- [ ] Echo resolved `chat` on send responses — recommend yes (same wave as Phase A).
- [ ] Interim action pacing (reuse existing limiter) — recommend yes.
- [ ] CI guard: every route ⊆ `openapi.yaml` — recommend yes.

---

## Build order — phase checklists

Track detailed per-endpoint status in each PRD's own Progress table; use these as the milestone checklist.

### Phase A — Addressing foundation · [PRD 0](./prd-0-addressing-foundation.md) · ban-risk none · unblocks everything ✅ SHIPPED (commits 8cb509b, f2c600f, af0fb44)
- [x] `resolveChat(chat, msisdn)` resolver: `chat` wins; digits-only user for pn/lid/group; lowercase server; strip device/agent suffix (via `types.ParseJID`); drop `broadcast` from send targets.
- [x] Add `chat` field to every send + `/contact/check` request; drop `msisdn` `required`.
- [x] Echo resolved canonical `chat` in send responses.
- [~] Migrate `isRecipientError` to sentinels — broadcast now rejected at the resolver (no more 500); **group-not-member sentinel mapping deferred** (needs one live-whatsmeow verification per ADR 0002, not CI-testable).
- [x] `openapi.yaml`: remove `msisdn` from `required`, add `chat`, deprecate `msisdn`, version → 0.12.0.
- **Exit:** ✅ resolver unit tests green (incl. group JID, device-strip, precedence, broadcast-reject); ✅ spec validates; ✅ msisdn-only unchanged. (End-to-end group send not CI-testable — resolver produces the group JID; whatsmeow receives it unchanged.)

### Phase B — Early low-risk wins · [PRD 4 subset](./prd-4-ops-hardening.md) + [PRD 1](./prd-1-read-query-surface.md) · ban-risk none/low
Ops subset (no dependency on A — can start immediately):
- [x] `idempotency_keys` table (dual-dialect migration) + `Idempotency-Key` header, insert-first concurrency (`ON CONFLICT DO NOTHING`), replay stored response, 409 pending / 422 body-mismatch. — commit 0d07469. Enforced as HTTP middleware (before the queue/direct fork; captures the exact response snapshot). Sweeper bounds growth.
- [x] `/health/live` (always 200) + `/health/ready` (DB+queue only, **not** session health); keep `/health` alias. — commit ae5eb3f
- [x] Graceful `DisconnectAll(timeout)` on shutdown — after HTTP stop, **before** DB close. — commit feedd04
Read/query (depends on A):
- [x] Shared TTL cache + per-account query budget infra. — commit f6591f2 (`internal/usecase/whatsapp/query.go`)
- [x] `GET` contacts (local store, paginated, honest metadata, never 404 empty). — commit e4deb71
- [x] `GET` joined-groups (cached + budgeted, lightweight summaries). — commit f6591f2
- [x] `GET` contact-info + group-info (cached + budgeted). — commit 25f89c3
- [x] `GET` avatar (tri-state 404/403/200 + ETag/304, user or group). — commit 32309e5
- [x] Sentinel mappings: `ErrGroupNotFound`→404, `ErrNotInGroup`→403, `ErrProfilePictureNotSet`→404, `ErrProfilePictureUnauthorized`→403. — commit 25f89c3
- [x] Config: `READ_QUERY_*` ×3, `SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS`. (Skipped `CONTACT_LIST_MAX_LIMIT` — const `maxContactPageLimit`=500 is enough; `IDEMPOTENCY_*` land with idempotency.)
- **Exit:** ✅ reads served from cache for free / budgeted on miss (429+Retry-After); ✅ readiness stays green when all sessions are logged out; ✅ idempotent retry returns the original result (409 in-flight / 422 body-mismatch). **Phase B complete.**

### Phase C — Interaction + group reads · [PRD 2](./prd-2-two-way-primitives.md) + [PRD 3 reads](./prd-3-group-community-management.md) · ban-risk low
- [x] `POST /message/read` — exactly one receipt type (panic guarded); `sender` required for group chats. — commit 5bf97eb
- [x] `POST /chat/presence` — composing/recording(=composing+audio)/paused. — commit 5bf97eb. (Auto-pause timer skipped — WhatsApp auto-expires typing client-side; `CHAT_PRESENCE_AUTO_PAUSE_SECONDS` deferred as low value.)
- [x] Interim action cap wired on mark-read/typing (5bf97eb) + react (e8ef549) — reuse limiter, key `action:`+phone.
- [x] `reply_to` + `mentions` on **all** sends — text (8e25a7d/2494435) + 7 media/other sends (f98d815…4e6883a). Shared `MessageContext` (resolved JIDs) through direct + queue; client builds `ContextInfo` (storeless quote per #5); text→`ExtendedTextMessage`, media/poll set inner `.ContextInfo` (poll: `PollCreationMessage.ContextInfo` only, secret preserved).
- [x] Group community reads — `GET /community/subgroups` + `GET /community/participants` (d9c6f74/d03079f), cached+budgeted; added `errors.As(*IQError)` mapping branch for raw IQ errors.
- **Exit:** blue-ticks ✅, typing ✅, action cap ✅, replies + mentions in 1:1 **and** groups ✅, community reads ✅. **Phase C complete.**

### Phase D — Operator visibility · [PRD 4 rest](./prd-4-ops-hardening.md) · ban-risk none ✅ SHIPPED (commits af7cbd1…21aaa7b)
- [x] `HandleEvent` cases: `LoggedOut`/`TemporaryBan`/`ConnectFailure`/`StreamReplaced` → persist status to dual-dialect `session_status` (no FK, outlives device-row delete) + evict dead clients (**no webhook** — that's #1's job). Transient `ConnectFailure` records `disconnected` without eviction. — 1a05923
- [x] `GET /admin/sessions` (+ `/:phone`): merge `ClientStore` + `GetAllDevices` + persisted status; honest connected/logged-out/banned/never-paired; per-instance `hostname`. — d59e12b
- [x] Prometheus `/metrics`: hand-rolled (no new dep), aggregate counters (`messages_total`, `webhook_deliveries_total`) + `sessions{state}` scrape-gauge; **never** phone labels. — 7ab186a
- [x] Direct-mode webhook retry parity (async `SendAsync` on detached `context.Background()` + bounded backoff, semaphore drop-on-full). — 1b0b785
- [x] Config: `ADMIN_API_SECRET`, `METRICS_ENABLED`, `WEBHOOK_MAX_RETRIES`, `WEBHOOK_RETRY_BACKOFF_SECONDS`.
- **Exit:** ✅ admin lists true session states (bearer-gated, 404-when-unset, constant-time); ✅ `/metrics` scrapes clean aggregate series (phone-never-a-label guarded by test); ✅ direct-mode webhooks retry instead of silently dropping. openapi 0.21.0. **Phase D complete.**

### Phase E — Group/community mutations (GATED) · [PRD 3](./prd-3-group-community-management.md) · ban-risk high ✅ SHIPPED (commits 8f584bd…2284b74)
- [x] create / leave / participants(add·remove·promote·demote) / settings / name / topic / photo / invite(get·reset·join·preview) / requests(list·approve·reject) / community(link·unlink). 16 endpoints as methods on the existing `WhatsappMessageUsecase`/`Handler` receivers (zero Wire changes).
- [x] Per-participant `ParticipantResult` (ok / invited / failed, invite code+RFC3339 expiry) — partial failure is 200, not an error.
- [x] Non-admin mutation → 403 (IQ 401/403 both → `ErrForbidden`); self-JID in remove/promote/demote → 400 (PN guard in usecase + LID guard in client, → `/leave`); JPEG-only photo (`ErrInvalidImageFormat`→400); name ≤25 / topic ≤512; revoked invite → **410 `ErrGone`**. Every op requires an explicit `@g.us` via `resolveGroupJID`.
- [x] Error mapping = one coherent edit: `mapIQError` (400/405/406→400, 401/403→403, 404→404, 409/423→409, 410→Gone, 419/429→429) + named sentinels ordered before it; `errors.Is`/`errors.As`, never substring.
- [x] Gates default off: `GROUP_ADD_PARTICIPANTS_ENABLED` (blocks bulk add + add-on-create, 403 in usecase pre-manager), `GROUP_JOIN_VIA_LINK_ENABLED` (blocks `/group/join`, 403), `GROUP_MAX_PARTICIPANTS_PER_REQUEST`=256; `GROUP_MANAGEMENT_ENABLED` (default true) unregisters the whole mutation surface → 404 when off, reads stay up.
- [x] Config: `GROUP_*` ×4. Docs: openapi 0.22.0, llms.txt, `.env.example`, wiki (new `Group-Management.md` + Security/Env), docs/ui, CHANGELOG.
- **Exit:** ✅ mutations work with partial-success semantics; ✅ every ban-risky path off by default and documented; ✅ 16 unit tests (gates/resolvers/participant-mapper/IQ-mapping); ✅ adversarial review (5 lenses × refute-verify) surfaced **0 confirmed findings**. **Phase E complete.** _Verify-once-live: exact IQ codes for non-admin (401 vs 403) / over-limit (419 vs 429) / per-participant error integers — all currently map safely; pin against a live group before relying on `result.code`._

### Phase F — Multi-URL webhooks · [PRD 4 feature 8](./prd-4-ops-hardening.md) ✅ SHIPPED (commits ad52e1b…9da3e40, openapi 0.23.0)
- [x] `device_webhook_subscriptions(jid,url,hmac,events[])` with a **data-preserving** dual-dialect migration off single-row `device_webhooks`. Backfill is **run-once** (persistent `schema_seed_markers` gate, not table-empty — never resurrects a deleted webhook) and **no FK** (subscriptions survive whatsmeow's logout device-delete + re-pair).
- [x] Per-subscription event filter validated against the fixed catalog; rewritten `GET|POST|DELETE /api/webhook` (back-compat: `.url`=first sub, `has_hmac` never leaks the secret, no-body DELETE=all, empty events=all).
- **Exit:** ✅ every operator's existing webhook survives the migration; ✅ N subscriptions with event filters.

### Roadmap #1 — webhook event system ✅ SHIPPED (commits 8c5e732…72f2f1c, openapi 0.24.0)
- [x] Generic flat-envelope dispatcher replacing all 6 dispatch sites (per-subscription fan-out, `EventMatches` filter, status-family `WEBHOOK_STATUS_EVENTS_ENABLED` kill-switch preserved; `message.incoming`/`session.*` ungated). `WEBHOOK_STATUS_EVENTS` deprecated.
- [x] `session.*` event types (logged_out/banned/connect_failure/connected/disconnected/replaced) emitted from Phase D's lifecycle switch (persistence + eviction retained).
- **Adversarial review (5 lenses × refute-verify): 7 confirmed findings → all fixed** (commits dced0e0, fda7c68, 9e33170): queue-mode fan-out dedup collision (CRITICAL), migration resurrection (HIGH), logout-webhook FK-cascade race (HIGH), `Store.ID` TOCTOU nil-deref (MEDIUM), `has_hmac` legacy false-positive (LOW). Re-verified green.

### Roadmap #2 — outbound pacing ✅ SHIPPED (commits 43beaf4…72cb70d, openapi 0.25.0)
- [x] Single shared in-process pacer governs **every** outbound op (all `/message/*`, react/edit/delete, mark-read, typing, all group/community mutations) in both direct + queue modes: ban gate (read-through of persisted `session_status`, auto-resume) → per-recipient hard cap → per-account token-bucket pace (`pace` blocks to MaxWait+jitter, `reject` immediate). `OUTBOUND_PACE_*` config; `OUTBOUND_PACE_ENABLED=false` falls back to legacy `MESSAGE_RATE_LIMIT_*`.
- [x] Replaces the Phase C interim action cap (`spendActionBudget` deleted) and lifts the Phase E gates (`GROUP_ADD_PARTICIPANTS_ENABLED`/`GROUP_JOIN_VIA_LINK_ENABLED` default → **true**).
- [x] Docs across all surfaces (openapi 0.25.0 + 429/Retry-After on the 8 sends, llms.txt, both wikis, CLAUDE.md, docs/ui, README, CHANGELOG) — commit 5a19bbc.
- **Adversarial review (5 lenses × 3-skeptic refute panel): 13 raised, 6 confirmed (7 refuted) → fixed** (5c1f53a, b3c197f, 72cb70d): media sends bypassed the pacer in direct mode (HIGH), bulk group-add charged n=len so any add >burst was permanently 429'd (HIGH×2 + MED, now paced as one op + n>burst fail-fast guard), ban-expiry TZ mis-round-trip on non-UTC Postgres (HIGH, now UTC writes), recipient-slot over-count on account timeout (LOW, documented). Re-verified green (14 pkgs).

---

## Cross-repo obligations (trail the gateway)

**SDK — full parity** ([PRD 5](./prd-5-sdk-parity.md)) ✅ SHIPPED (`whatsapp-gateway-sdk-go` dev, commits 7266e76…b12b47d, 16 commits, 204+ tests):
- [x] Send surface: `Chat`/resolved-`chat` on all sends via `WithChat`; `WithReply`/`WithMentions`; `WithIdempotencyKey`; (React `SenderMsisdn` + media/CheckContact already existed).
- [x] Reads (`ListContacts`/`GetContactInfo`/`GetAvatar`/`ListGroups`/`GetGroupInfo`); two-way (`MarkRead`/`SendChatPresence`); group+community methods; admin as a **separate opt-in `AdminClient`**; `ParseWebhook` + `session.*` events.
- **Adversarial contract review (5 lenses × 3-skeptic refute panel): 6 confirmed → fixed** (SDK da9cb15/8331987/a13c890 + gateway 7a397ed): media-idempotency random-boundary → 422 (deterministic boundary), `message.failed` error reason + incoming `addressing_mode` dropped, and the gateway handlers emitting `{Status,Message}` vs the documented `{error,code}` (fixed gateway-side + SDK tolerant parse).

**MCP — curated safe subset** ([PRD 6](./prd-6-mcp-curated.md)) ✅ SHIPPED (`mcp-whatsapp-gateway` dev, 7 commits, 26 tools, green):
- [x] Messaging (media, react/edit/delete, `chat` addressing, reply/mentions), read/query tools, `mark_read`/`send_typing` (documented under the outbound pacer). Consumes SDK v0.7.0 via a git-ignored `go.work` (SDK not yet tagged).
- [x] **Excluded** (locked by `manifest_test.go`): group mutations, community, admin/metrics.

**Release (outward — needs user go-ahead):** SDK + MCP are implemented and locally green but NOT tagged/pushed. To release: tag+push `whatsapp-gateway-sdk-go` `v0.7.0`, then the MCP's committed `require v0.7.0` resolves without the local `go.work`. Gateway `dev` also has the error-shape + pacing-review fixes to push.

---

## Per-feature "Definition of Done" (apply to every endpoint)

1. [ ] Code + Wire regen if a provider changed (`wire gen ./internal/infrastructure`).
2. [ ] Unit tests (highest seam: usecase / resolver / error-sentinel mapping / cache-budget). Live-only paths: verify once, lock to sentinel.
3. [ ] `docs/openapi.yaml` path + schemas (+ bump `info.version`). *(`openapi.go` just loads it.)*
4. [ ] `llms.txt` entry.
5. [ ] `wiki/Environment-Variables.md` + `.env.example` + `CLAUDE.md` config list (any new env keys — see [docs-surface-matrix](./docs-surface-matrix.md)).
6. [ ] `docs/ui/assets/docs/*` page.
7. [ ] `wiki/Security-Considerations.md` if it carries a ban/security caveat.
8. [ ] `CHANGELOG.md`.
9. [ ] SDK method (+ MCP tool if in the curated subset).
10. [ ] Commit small + focused per concern (never one bulk commit), Conventional Commits.

---

## Progress & references
- **Live tracker:** [`README.md`](./README.md) — per-PRD + per-endpoint status.
- **Sequence rationale:** [`00-sequence.md`](./00-sequence.md).
- **Doc obligations + env-key master list:** [`docs-surface-matrix.md`](./docs-surface-matrix.md).
- **Decisions of record:** [`../adr/`](../adr/) · **Glossary:** [`../../CONTEXT.md`](../../CONTEXT.md) · **Roadmap:** [`../ROADMAP.md`](../ROADMAP.md) §10.

**Start here:** lock the six cross-cutting decisions above → begin **Phase A / PRD 0**.
