# PRD 1 — Read / Query Surface — contacts, groups, profile, avatar

> **Status:** Not started  ·  **Sequence position:** 3  ·  **Ban-risk:** low
> **Depends on:** PRD 0   ·   **Effort rollup:** M
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | Deliverable (endpoint / change) | whatsmeow call | Effort | Ban-risk | Status |
|---|---|---|---|---|---|
| 1 | `GET /contact/?limit=&offset=` — list contacts (local synced store, paginated) | `Store.Contacts.GetAllContacts(ctx)` | S | none | ☐ |
| 2 | `GET /contact/info?chat=&refresh=` — contact/user info (status, verified name, picture id, device count, LID) merged with local names | `GetUserInfo(ctx,[]JID)` + `Store.Contacts.GetContact(ctx,jid)` | M | low | ☐ |
| 3 | `GET /contact/avatar?chat=&preview=&existing_id=&is_community=` — avatar URL, existing-ID diffing | `GetProfilePictureInfo(ctx,jid,params)` | M | low | ☐ |
| 4 | `GET /group/?limit=&offset=&refresh=` — list joined groups (cap-noted) | `GetJoinedGroups(ctx)` | M | low | ☐ |
| 5 | `GET /group/info?chat=&refresh=` — group info (subject, topic, participants, flags) | `GetGroupInfo(ctx,groupJID)` | M | low | ☐ |
| 6 | Read-query TTL cache + per-account query budget (shared infra for #2–#5) | gateway-owned; `ratelimiter.Limiter.AllowN(ctx,"query:"+phone,1)` | M | none | ☐ |

## Problem Statement

A backend integrator can send messages through the gateway, but cannot **read** the WhatsApp graph it is messaging into. To build anything richer than fire-and-forget sends — a contact directory, a "which groups am I in" view, a profile card, an avatar thumbnail, a business-vs-personal badge — the integrator has to either scrape it out of inbound webhooks or keep asking "is this number even on WhatsApp". The four questions they actually ask ("who are my contacts", "tell me about this JID", "what's their avatar", "what groups am I in / what's in this group") each map to a whatsmeow call the gateway already holds a client for, but there's no HTTP surface for them. The naive fix — proxy every call straight through — is a **ban vector**: usync and group-IQ hammering from a polling backend is exactly the traffic WhatsApp flags. So the read surface has to exist *and* be cheap by default.

## Solution

Expose the read graph as five REST endpoints under the existing `/api/v1` base, split by address space (`/contact/*`, `/group/*`). Serve **direct + cached**:

- The **contacts list** is a pure local-store read — WhatsApp's own synced address book, already on disk. Uncapped, no network, no budget. Returned with honest metadata (`count`, `synced`, "reflects locally synced state, may be incomplete until history sync completes"). Never 404 on empty.
- The **four server-hitting reads** (contact info, avatar, joined groups, group info) go through one shared **short-TTL in-memory cache** + a lightweight **per-account query budget** consumed only on a cache *miss*. Normal polling rides the cache for free; `refresh=true` and cold misses spend budget; budget exhausted → 429 + `Retry-After`. This is the decision-#3 "reads served direct + cached" contract, made concrete.

No enrichment fan-out: the contacts list stays strictly local-store (no per-entry avatar/presence — that's N server hits and a budget blow-up). Reads never mutate, so ban-risk is low and the whole area ships early per the sequencing decision.

## User Stories

1. As a **backend integrator**, I want `GET /contact/` to return my account's synced contacts (paginated), so that I can render a contact directory without re-deriving it from inbound message webhooks.
2. As a **backend integrator**, I want the contacts response to carry `{count, synced, note}` metadata, so that I can tell an empty list caused by "no contacts" apart from one caused by "history sync hasn't finished yet".
3. As a **backend integrator**, I want the contacts list to **never 404 on empty**, so that a freshly-paired account returns `200 {items:[], count:0}` and my code doesn't branch on a false error.
4. As a **backend integrator**, I want `GET /contact/info?chat=` to return status text, verified business name, picture id, device count and LID merged with my local saved name, so that I can build a profile card from one call.
5. As a **support-agent backend**, I want contact info on a **WhatsApp Business** number to include `verified_name` and `is_business:true`, so that agents can see they're talking to a verified business.
6. As a **backend integrator**, I want `GET /contact/avatar?chat=` to return the avatar URL (preview or full), so that I can show a thumbnail without proxying image bytes through my own service.
7. As a **backend integrator**, I want to pass `existing_id=` and get `200 {unchanged:true, id}` (plus an `ETag`) when the picture hasn't changed, so that I can cache avatars and skip re-downloading unchanged images.
8. As a **backend integrator**, I want a target with **no avatar** to return `404 {has_avatar:false}` and a **privacy-hidden** avatar to return `403 {reason:"privacy"}`, so that "no picture" and "not allowed to see the picture" are distinguishable rather than both 500ing.
9. As a **backend integrator**, I want to pass `is_community=true` for a community's parent JID, so that fetching a community photo doesn't get mis-reported as a privacy 403.
10. As a **backend integrator**, I want `GET /group/` to list the groups my account participates in, so that I can populate a group picker.
11. As a **backend integrator**, I want the joined-groups response to note WhatsApp's ~500-group participating cap, so that I understand a truncated list is a platform limit, not a gateway bug.
12. As a **backend integrator**, I want `GET /group/info?chat=` to return subject, topic, participants and flags, so that I can show a group detail view.
13. As a **backend integrator**, I want `GET /group/info` on a group I've **left** to return `403 "not a participant of this group"` and an unknown-but-well-formed JID to return `404 "group not found"`, so that "you left" and "doesn't exist" are separate outcomes.
14. As a **backend integrator**, I want a malformed or wrong-address-space `chat` (e.g. `@g.us` sent to `/contact/info`) rejected with `400` **before any network call**, so that my typo doesn't burn query budget.
15. As a **backend integrator**, I want repeated polling of the same read to be served from cache for free, so that a dashboard refreshing every few seconds doesn't rate-limit itself.
16. As a **backend integrator**, I want `refresh=true` to force a fresh fetch (spending budget), so that I can bypass the cache when I know something changed.
17. As a **backend integrator**, I want a cache-miss past the per-account budget to return `429 + Retry-After`, so that my client backs off instead of hammering WhatsApp into a ban.
18. As a **backend integrator**, I want an `@lid`-only chat surfaced honestly with `addressing_mode:"lid"`, so that I don't treat a non-dialable LID as a phone number.
19. As an **operator**, I want the per-account query budget to be tunable via env (`READ_QUERY_BUDGET_*`) and the cache toggleable/tunable (`READ_QUERY_CACHE_*`), so that I can trade freshness against WhatsApp-side load per deployment.
20. As an **operator**, I want a read against a session whose store isn't ready yet to return `409 "session store not ready"` instead of panicking, so that a mid-startup node degrades cleanly.
21. As an **operator**, I want a device deleted server-side mid-read to route through the existing `mapWhatsmeowErr` (client eviction + `409 re-pair`), so that stale sessions self-heal the same way sends already do.
22. As an **AI assistant (MCP)**, I want all five reads exposed as read-only MCP tools, so that an assistant can answer "who's in this group / what's this contact's status / show me their avatar" without any mutation risk.
23. As a **backend integrator**, I want the send response to echo the resolved canonical `chat` JID (landing with the addressing change), so that the read and write surfaces speak the same identifiers.

## Implementation Decisions

**Shape.** Five thin Fiber handlers → new read methods on the existing `WhatsappMessageUsecase` (reuses its injected manager + limiter + logger + config — no new provider) → 5 new methods on the `Manager` interface (thin logging wrappers) → 5 new methods on the `Client` interface, each following the established `clients.Get`-nil→404 / `!IsLoggedIn()`→401 / call / `mapWhatsmeowErr` pattern.

**Cache + budget (deliverable #6).** One ~50-line TTL cache: `map[string]entry{val any; expiresAt time.Time}` guarded by `sync.RWMutex`, keyed `phone|op|arg`. A `getOrFetch(key, ttl, budgetKey, fetchFn)` helper: cache hit → return; miss → `limiter.AllowN(ctx,"query:"+phone,1)`; allowed → fetch, store, return; denied → 429. The contacts list does **not** go through this (local-store, uncapped). Avatars use a separate longer TTL. Cache stampede (two concurrent misses, both spend budget) is **accepted for v1** — `ponytail:` no singleflight; upgrade path is `golang.org/x/sync/singleflight` keyed by cache key if duplicate server hits become measurable.

**Address-space enforcement.** After `validateRecipient` normalizes `chat`, assert the server: contact endpoints require `s.whatsapp.net` | `lid`; group endpoints require `g.us`; `status@broadcast` rejected on contact info/avatar. Mismatch → 400 before any network. (Reuses the PRD-0 addressing resolver; groups need the `@g.us` requirement since bare digits normalize to a *user* JID.)

**Contact-info merge.** `GetUserInfo` (usync) for status / verified name / picture id / device count, merged with `Store.Contacts.GetContact` for the locally-saved name. `VerifiedName` populated ⇒ `is_business:true`.

**Avatar.** `GetProfilePictureInfo` with `{Preview, ExistingID, IsCommunity}`. Return the time-limited WhatsApp CDN URL as-is (no proxy/re-host). Tri-state: not-set→404 `{has_avatar:false}`, privacy→403 `{reason:"privacy"}`, unchanged-vs-`existing_id`→200 `{unchanged:true,id}` + `ETag`. whatsmeow returning `(nil,nil)` for unchanged must NOT be treated as an error.

**Pagination.** Contacts and joined-groups paginate **client-side over the fetched slice** (no server-side paging exists in whatsmeow). Default limit 100, clamp to `CONTACT_LIST_MAX_LIMIT` (500) — matches the existing `maxIncomingLimit`-style clamp. Joined-groups list is truncated at WhatsApp's ~500 participating-groups cap; add a `cap_note` to metadata.

### Contract sketches (field lists)

- **ContactListResponse** — `items[]{jid, msisdn?, name, addressing_mode}`, `count`, `metadata{synced, note}`
- **ContactInfoResponse** — `chat, msisdn?, name, status, verified_name?, is_business, picture_id?, device_count, addressing_mode`
- **AvatarResponse** — `chat, url?, id, type(preview|full), unchanged?, has_avatar` (+ `ETag` header; 403 body `{reason:"privacy"}`)
- **GroupListResponse** — `items[]{jid, subject, is_announce?, participant_count}`, `count`, `metadata{cap_note}`
- **GroupInfoResponse** — `jid, subject, topic?, owner?, is_announce, is_locked, participants[]{jid, is_admin, is_super_admin}`, `metadata`

### Config additions

| Key | Purpose | Default |
|---|---|---|
| `READ_QUERY_CACHE_ENABLED` | Toggle short-TTL cache for the 4 server-hitting reads. Off = always hit network (still budgeted). | `true` |
| `READ_QUERY_CACHE_TTL_SECONDS` | TTL for group-list, group-info, user-info cache entries. | `60` |
| `READ_QUERY_AVATAR_CACHE_TTL_SECONDS` | Separate (longer) TTL for avatar lookups (change rarely, support existing-ID diffing). | `600` |
| `READ_QUERY_BUDGET_REQUESTS` | Per-account server-hitting query budget (tokens); consumed only on cache MISS. | `30` |
| `READ_QUERY_BUDGET_DURATION_SECONDS` | Window for the per-account query budget. | `60` |
| `CONTACT_LIST_MAX_LIMIT` | Upper clamp for contacts-list pagination limit (default page size 100). | `500` |

### Integration seams

| File | What |
|---|---|
| `internal/whatsapp/client.go` | Add 5 `Client` methods + impl (`ListContacts`, `GetContactInfo`, `GetAvatar`, `ListJoinedGroups`, `GetGroupInfo`), each `clients.Get`-nil→404 / `!IsLoggedIn()`→401 / call / `mapWhatsmeowErr`. |
| `internal/whatsapp/client.go` (`mapWhatsmeowErr`) | `errors.Is`-map `ErrGroupNotFound`→404, `ErrNotInGroup`→403, `ErrProfilePictureNotSet`→404, `ErrProfilePictureUnauthorized`→403. |
| `internal/whatsapp/manager.go` | Same 5 methods on `Manager` + thin logging wrappers (mirror `CheckNumber`/`GetIncomingMessages`). |
| `internal/usecase/whatsapp/read_usecase.go` (new) | Read methods on `WhatsappMessageUsecase`: `validateRecipient`, address-space assert, cache+budget for server-hitting reads, clamp pagination, build DTOs + metadata. |
| `internal/usecase/whatsapp/readcache.go` (new) | ~50-line TTL cache + `getOrFetch(key,ttl,budgetKey,fetchFn)`; field on `WhatsappMessageUsecase`, built in `NewWhatsappMessageUsecase`. |
| `domain/whatsapp/contact.go` & `domain/whatsapp/group.go` (new) | JSON-tagged response DTOs, no infra imports. |
| `internal/handler/whatsapp/whatsapp_read_handler.go` (new, or extend message handler) | 5 handlers: `QueryParser`, call usecase, `httperror.FromError`; set `ETag` on avatar; `utils.MustGetPhoneNumber` + `middleware.GetTraceID`. |
| `internal/router/contact_routes.go` + `internal/router/group_routes.go` (new) | Register `GET /contact/`, `/contact/info`, `/contact/avatar` under `contactGroup`; new `initGroupRoutes` for `GET /group/`, `/group/info` (JWT); call from `SetupRouter`. |
| `config/config.go` | Add the `READ_QUERY_*` / `CONTACT_LIST_MAX_LIMIT` fields. |
| `internal/infrastructure/wire.go` | No new provider if methods hang off existing usecase/handler. If a dedicated query limiter is chosen, add `ProvideQueryLimiter` + run `wire gen ./internal/infrastructure`. |

## Edge Cases

| Scenario | Risk | Handling |
|---|---|---|
| Fresh QR pair — `GetAllContacts` empty/sparse (history sync incomplete) | confusing-UX | 200 empty/partial + `{count, synced:true, note:"reflects locally synced state; may be incomplete until history sync completes"}`. Never 404. |
| `GetJoinedGroups` truncated at WhatsApp's ~500 participating cap | correctness | Return what the single IQ yields + metadata `cap_note` ("groups beyond ~500 omitted"). Paginate client-side over the fetched slice only. |
| `GetGroupInfo` on a group the account has **left** | confusing-UX | `ErrNotInGroup` → 403 `"not a participant of this group"`. Distinct from 404. |
| Invalid/non-existent group JID or malformed `chat` | correctness | `validateRecipient` rejects → 400 before network. Well-formed-but-unknown → `ErrGroupNotFound` → 404. |
| Target has no avatar set | confusing-UX | `ErrProfilePictureNotSet` → 404 `{has_avatar:false}` (not 500/empty-200). |
| Avatar hidden by target's privacy | confusing-UX | `ErrProfilePictureUnauthorized` → 403 `{reason:"privacy"}`. Must differ from not-set (404). |
| Avatar `existing_id` supplied, picture unchanged | none | whatsmeow returns `(nil,nil)` → 200 `{unchanged:true, id}` + `ETag`. Not an error. |
| Community parent JID to avatar without `is_community` | confusing-UX | Bare call 401 → misleading "privacy" 403. Expose documented `is_community=true` so community-photo callers pass it. (See open question re auto-retry.) |
| Contact info for a WhatsApp **Business** number | none | `VerifiedName` populated → include `verified_name` + `is_business:true` (as `CheckNumber` already does). |
| `chat` address space mismatched to endpoint (`@g.us`→/contact, `@s.whatsapp.net`→/group) | correctness | After `validateRecipient`, assert server; else 400. Reject `status@broadcast` on contact info/avatar. |
| Large contacts list (thousands) in one response | confusing-UX | `limit`/`offset` paging, default 100, clamp 500; total count in metadata. |
| Session store not ready — `Store.Contacts` nil | correctness | Nil-guard `GetAllContacts`/`GetContact`; nil → 409 `"session store not ready"` (no panic). |
| Per-account query budget exhausted on cache MISS | ban | `AllowN` denied → 429 + `Retry-After` from `Result.RetryAfter`. Cache HITs bypass budget; `refresh=true` still consumes. |
| Cache stampede — two concurrent misses, both hit network + spend budget | none | Accept for v1 (`ponytail:` no singleflight). Upgrade: `x/sync/singleflight` keyed by cache key. |
| `@lid`-only chat with no resolvable phone | confusing-UX | `GetUserInfo`/`GetProfilePictureInfo` accept `@lid`; surface it honestly + `addressing_mode:"lid"` so callers don't dial it. |
| Device deleted server-side mid-read (`store.ErrDeviceDeleted`) | correctness | Route through existing `mapWhatsmeowErr` → evict client, 409 re-pair. Reused unchanged. |

## Error Semantics

| Condition | Response |
|---|---|
| No in-memory client for phone (never registered / evicted) | 404 `ErrNotFound {error:"client not found"}` |
| Client present but not logged in | 401 `ErrUnauthorized {error:"client not logged in"}` |
| Session store not ready (`Store.Contacts` nil) | 409 `ErrConflict {error:"session store not ready"}` |
| Empty/blank `chat`, malformed JID, or wrong address space | 400 `ErrBadRequest {error:"invalid recipient ..."}` |
| Group not found (well-formed unknown JID) | 404 `ErrNotFound {error:"group not found"}` |
| Not a participant / account left the group | 403 `ErrForbidden {error:"not a participant of this group"}` |
| Avatar not set on target | 404 `ErrNotFound {has_avatar:false}` |
| Avatar hidden by target's privacy | 403 `ErrForbidden {reason:"privacy"}` |
| Avatar unchanged vs supplied `existing_id` | 200 `{unchanged:true, id}` + `ETag` (see open question on 304) |
| Per-account query budget exhausted (cache miss only) | 429 `ErrTooManyRequests` + `Retry-After` from `Result.RetryAfter` |
| Contacts/groups list served | 200 `{items:[...], count, metadata:{synced:true, note, cap_note?}}` |
| Device deleted server-side mid-call | 409 `ErrConflict {error:"session deleted, re-pair required"}` (via `mapWhatsmeowErr`) |
| Other whatsmeow / IQ failure | 500 `ErrInternalFailure` |

## Testing Decisions

Highest seam first — the **usecase layer** carries all the branching (address-space assert, cache/budget gating, pagination clamp, metadata) and is fully **unit-testable** against a faked `Manager` interface + a real `ratelimiter.Limiter`:

- `validateRecipient` + address-space enforcement: table-driven unit tests (bare digits, PN JID, group JID, `@lid`, `status@broadcast`, cross-space mismatch) → assert 400 vs pass with no network.
- **Cache + budget** (`readcache.go`): unit test hit-bypasses-budget, miss-spends-budget, `refresh=true`-forces-miss, exhaustion→429+Retry-After, TTL expiry re-fetches. This is the load-bearing ban mitigation — test it directly.
- Pagination clamp (default 100, max 500, offset past end → empty) and metadata assembly (`synced`, `note`, `cap_note`): unit.
- Avatar tri-state branch selection (404 not-set / 403 privacy / 200 unchanged) is unit-testable at the usecase from faked `Client` returns, including the `(nil,nil)`-is-not-an-error case.

**Not CI-testable** (no live WhatsApp): the actual `GetUserInfo`/`GetJoinedGroups`/`GetGroupInfo`/`GetProfilePictureInfo` IQ round-trips. Strategy: verify each whatsmeow error once against a live account, then **lock the mapping to the exported sentinel** — `mapWhatsmeowErr` matches `whatsmeow.ErrGroupNotFound` / `ErrNotInGroup` / `ErrProfilePictureNotSet` / `ErrProfilePictureUnauthorized` via `errors.Is/As`, **never substring** (per the error-mapping ADR). The sentinel-mapping table itself is unit-testable with wrapped errors. No new testcontainers surface (the RabbitMQ harness is unrelated to reads).

## Decisions for Review

- [ ] **Query-budget wiring** — recommend a **dedicated in-memory limiter** built from `READ_QUERY_BUDGET_*` (memory-only per-instance ceiling; keeps read budget independent of the send limit). Alternative was reusing the message limiter with a `"query:"` key namespace (zero wiring but couples read budget to send sizing).
- [ ] **Avatar "unchanged" transport** — recommend **200 `{unchanged:true}` + `ETag`** driven by the `existing_id` query param (simplest for non-browser backend callers). Alternative: true HTTP `304 Not Modified` via `If-None-Match` (cleaner for HTTP caches, but callers must send the header).
- [ ] **Avatar delivery** — recommend **return the time-limited WhatsApp CDN URL as-is**, no proxy/re-host. Download-and-re-host is deferred (ties to the storage/media pipeline).
- [ ] **Community photos** — recommend **require explicit `is_community=true`** (documented). Alternative auto-retry with `IsCommunity=true` on a 401 costs one extra IQ and spends budget.
- [ ] **`GetContactInfo` scope** — recommend **single `chat` only** for curated v1. Comma-separated batch (one usync, cap N) adds surface; defer.
- [ ] **Cache invalidation beyond TTL** — recommend **`refresh=true` (budget-consuming) is sufficient now**; event-driven invalidation (group.change / avatar-change) deferred to the roadmap #1 webhook-events window.
- [ ] **MCP curated subset** — recommend **expose all five reads** (list contacts, contact info, avatar, list groups, group info) — all read-only, none mutate, all fair game per decision #8.
- [ ] **Contacts-list enrichment** — recommend **strictly local-store-only** (no per-entry avatar/presence). Enrichment is N+1 network + budget blow-up.

## Doc Surfaces

- [ ] `docs/openapi.yaml` — add the 5 read paths under `/api/v1`; keep additive (no `/v2`). (`docs/openapi.go` only loads/serves the hand-maintained spec.)
- [ ] `llms.txt` — add the read/query endpoints.
- [ ] `wiki/Environment-Variables.md` + `.env.example` + `CLAUDE.md` config list — document new keys: `READ_QUERY_CACHE_ENABLED`, `READ_QUERY_CACHE_TTL_SECONDS`, `READ_QUERY_AVATAR_CACHE_TTL_SECONDS`, `READ_QUERY_BUDGET_REQUESTS`, `READ_QUERY_BUDGET_DURATION_SECONDS`, `CONTACT_LIST_MAX_LIMIT`.
- [ ] `docs/ui/assets/docs/*` — add/extend the contacts + groups read pages.
- [ ] `CHANGELOG.md` — new read/query surface entry.
- [ ] SDK (`whatsapp-gateway-sdk-go`) — full tenant parity: 5 read methods (per decision #8).
- [ ] MCP (`mcp-whatsapp-gateway`) — expose the 5 reads as read-only tools (curated safe subset; no mutations).

## Out of Scope

- **Contacts-list enrichment** (per-entry avatar/presence) — N+1 server hits; stays local-store-only.
- **Force-resync of the contact store** — deferred (decision #6); we expose the synced store as-is.
- **Avatar re-hosting / media pipeline** — return CDN URL only; download+store lives with the storage/media initiative.
- **Batch contact-info** (comma-separated JIDs) — single `chat` in v1.
- **Event-driven cache invalidation** (group.change / avatar-change webhooks) — deferred to roadmap **#1** (webhook events); these are net-new event *types*.
- **Server-side pagination** — whatsmeow has none; we paginate client-side over the fetched slice.
- **Durable message store** — unrelated deferred initiative.

## Dependencies & Sequencing

- **Depends on PRD 0** (addressing) — this area consumes the unified `chat` resolver (`validateRecipient` / address-space rules) and the echoed-`chat` convention. Group endpoints specifically need the `@g.us` requirement that PRD 0's resolver work establishes.
- **Ships early** in the automation-first wave (reads/queries are the low-ban mitigation baked into decision #7) — before roadmap **#1** (webhook events) and **#2** (pacing).
- **Feeds #1** — event-driven cache invalidation for these reads is intentionally deferred into the #1 webhook-events envelope window (avoid designing invalidation twice).
- The per-account query budget is a **best-effort interim ceiling** reusing the existing limiter; it is superseded by the real pacer from **#2** when that lands.
