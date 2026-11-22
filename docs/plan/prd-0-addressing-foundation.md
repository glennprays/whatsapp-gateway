# PRD 0 — Addressing Foundation — the unified `chat` recipient

> **Status:** ☑ Shipped (commits 8cb509b, f2c600f, af0fb44) — group-not-member error-sentinel mapping deferred (needs one live verification, per ADR 0002)  ·  **Sequence position:** 1 (unblocks everything)  ·  **Ban-risk:** none
> **Depends on:** none   ·   **Effort rollup:** S
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | Deliverable (endpoint / change) | whatsmeow call | Effort | Ban-risk | Status |
|---|---|---|---|---|---|
| 1 | Add unified `chat` field + keep `msisdn` as back-compat alias on all Send*/Message*/ContactCheck request structs | gateway-owned | S | none | ☐ |
| 2 | `resolveChat(chat, msisdn)` — evolve `validateRecipient` into the canonical resolver with precedence (chat wins, msisdn fallback) | gateway-owned (feeds `types.ParseJID`) | S | none | ☐ |
| 3 | Strip device/agent suffix (`ToNonAD`) for `s.whatsapp.net` and `@lid` recipients | `types.ParseJID` + `StripDeviceIDFromJID` | S | none | ☐ |
| 4 | Lowercase server + tighten allowlist: digits-only user for pn/lid/group, drop `broadcast` from send targets | gateway-owned | S | none | ☐ |
| 5 | Broaden send-error mapping (`isRecipientError`) for group-not-member + broadcast-unsupported so they 4xx not 500 | maps `cli.SendMessage` errors in `mapWhatsmeowErr` | S | none | ☐ |

## Problem Statement

Backend integrators today can only address a WhatsApp recipient by `msisdn` (a bare phone number), and every message body marks `msisdn` as `required`. That model cannot express the addressing forms an automation platform must handle: group JIDs (`@g.us`), PN JIDs (`@s.whatsapp.net`), and LID participants (`@lid`) surfaced by incoming-message senders and `reply_to.participant`. It also leaks whatsmeow implementation errors: a device-suffixed JID (which is exactly what an incoming sender JID looks like) fails late with `ErrRecipientADJID`, a group the account isn't in or a broadcast list falls through to a 500, and a garbage or uppercased server reaches whatsmeow before anyone validates it. There is no single, canonical recipient concept, so every later area (reads, two-way, group management) would have to invent its own.

## Solution

Introduce one canonical recipient field, **`chat`**, accepted on every request that currently takes `msisdn`. `chat` accepts a bare msisdn, a PN JID (`@s.whatsapp.net`), a group JID (`@g.us`), or a `@lid`. `msisdn` stays forever as a **back-compat alias** resolving to a PN JID; when both are present, `chat` wins. All resolution funnels through one usecase-layer chokepoint — the existing `validateRecipient` evolved into `resolveChat(chat, msisdn)` — which normalizes, validates against a tightened server allowlist, strips device/agent suffixes, and rejects bad input early with a clean 400 instead of letting whatsmeow fail late. The change is additive within `/api/v1` (no `/v2`): the OpenAPI spec relaxes `required: [msisdn]`, adds `chat`, and marks `msisdn` deprecated. This is the foundation every other automation-platform area resolves recipients through.

## User Stories

1. As a backend integrator, I want to address a message by group JID (`123@g.us`), so that I can send into groups, not just to individual numbers.
2. As a backend integrator, I want to pass a PN JID (`628...@s.whatsapp.net`) directly, so that I can round-trip the exact JID I received on an incoming webhook without converting it back to a bare number.
3. As a backend integrator, I want to keep sending `msisdn` on existing calls with zero code changes, so that upgrading the gateway never breaks my current integration.
4. As a backend integrator, I want a single documented precedence rule (chat wins, msisdn is the fallback alias), so that I know exactly which field is honored when I send both.
5. As a support-agent backend, I want to reply to a LID-only group participant surfaced by `reply_to.participant` via `@lid`, so that I can respond to people whose PN alias I don't hold — accepting that `@lid` delivery is best-effort.
6. As a backend integrator, I want a device/agent-suffixed sender JID (`628...:2@s.whatsapp.net`) to just work when I feed it straight back as a recipient, so that I don't have to strip the suffix myself before every reply.
7. As a backend integrator, I want a bad recipient (unknown server, non-numeric user, all-punctuation input) rejected with a clear early 400, so that I get a meaningful error instead of a late 500 from deep inside whatsmeow.
8. As a backend integrator, I want a case-variant server (`@S.WHATSAPP.NET`) accepted, so that a canonical-but-uppercased JID isn't falsely rejected.
9. As a backend integrator, I want `+`, spaces, dashes and parentheses in an msisdn tolerated, so that lightly-formatted numbers still resolve (existing behavior, preserved).
10. As a backend integrator, I want an unambiguous error when I supply neither `chat` nor `msisdn`, so that I know a recipient is required — the message now reads "chat (or msisdn) is required".
11. As a backend integrator, I want a group send to a group I'm not a member of to return a 4xx I can act on, so that it doesn't surface as an opaque 500.
12. As a backend integrator, I want broadcast-list sends rejected up front (until broadcast-send is a real feature), so that I don't get a 500 from an unsupported target.
13. As a backend integrator, I want to note-to-self by using my own number as `chat`, so that no special self-send handling gets in my way.
14. As a backend integrator, I want the gateway to NOT silently reinterpret a leading-zero / national-format number, so that I'm never surprised by a message delivered to the wrong number — the gateway expects E.164 and this responsibility stays with my wrapping backend.
15. As an AI assistant (MCP), I want one uniform `chat` recipient across every messaging tool, so that I don't need per-tool addressing logic.
16. As an operator, I want the resolver to reject known-bad targets before they hit whatsmeow, so that logs show clean 400s instead of a stream of 500s that look like gateway faults.
17. As a backend integrator, I want (open, see Decisions for Review) the resolved canonical `chat` JID echoed back alongside `message_id`, so that I can learn and persist the canonical form the platform is built around.

## Implementation Decisions

**The chokepoint.** `validateRecipient` (single existing resolver, called at ~11 sites before the queue-vs-direct branch) becomes `resolveChat(chat, msisdn)`. Because the resolved JID flows into both the queue job's `To` field and the direct manager call, resolving once here covers both paths. Every `validateRecipient(req.Msisdn)` call-site swaps to `resolveChat(req.Chat, req.Msisdn)`.

**Precedence.** `chat` wins whenever non-empty; `msisdn` is used only as the fallback. Requiredness is no longer a struct binding — it's enforced inside `resolveChat` (400 when both empty). This keeps `msisdn` a true alias, not a competing required field.

**Normalization + validation, in order:** trim → pick `chat` else `msisdn` → if it contains `@`, lowercase the server and look it up in the allowlist `{s.whatsapp.net, lid, g.us}` (`broadcast` dropped); else treat as a bare msisdn (strip `+` and non-digits → PN JID). For pn/lid/group, require a **digits-only user** (rejects `12ab34@s.whatsapp.net` early). Strip any device/agent suffix to non-AD. A bare input that reduces to zero digits → 400.

**Error mapping.** `resolveChat` handles shape errors (early 400). Delivery-time errors from `cli.SendMessage` still map in `mapWhatsmeowErr`; broaden `isRecipientError` to also catch group-not-member and (if broadcast is ever kept) broadcast-unsupported so they become 4xx. Exact whatsmeow strings are open questions — see Decisions for Review; the ADR direction (project-wide) is to pin these to exported sentinels via `errors.Is/As`, not substring, so they're unit-testable.

**Request contract (added field, every Send*/Message* body + ContactCheck):**
- `chat` (string, json/form/query `chat`) — canonical recipient; bare msisdn | `@s.whatsapp.net` | `@g.us` | `@lid`.
- `msisdn` (string, retained, **`binding:"required"` dropped**, marked deprecated) — back-compat alias resolving to a PN JID.
- At least one of `chat` / `msisdn` required (OpenAPI can't express "at least one of" — stated in description, enforced in `resolveChat`).
- `MessageReactionRequest.sender_msisdn` doc updated to note it also accepts the `chat` forms (React resolves twice: the recipient and the sender).

**Send response (open, recommended):** add resolved canonical `chat` JID alongside `message_id`, done in this same wave so SDK/OpenAPI churn is one release.

**Config additions:** none for this area.

### Integration seams

| File | What changes |
|---|---|
| `internal/usecase/whatsapp/message_usecase.go` | THE integration point. `validateRecipient` (line 91) → `resolveChat(chat, msisdn)` with precedence, digits-only user check, server lowercasing, device-suffix strip, broadcast dropped. Swap all ~11 call-sites (SendText/Image/Audio/Video/Document/Location/Poll/Sticker + React ×2 + Delete + Edit + CheckNumber). |
| `domain/whatsapp/message.go` | Add `Chat string` (json/form `chat`) to every Send*Request/Message*Request; drop `binding:"required"` on `Msisdn`; update `SenderMsisdn` doc on `MessageReactionRequest`. |
| `domain/whatsapp/contact.go` | Add `Chat string` (json/query/form `chat`) to `ContactCheckRequest`; drop `binding:required` on `Msisdn`. |
| `internal/handler/whatsapp/whatsapp_message_handler.go` | `c.BodyParser` auto-binds the new `chat` tag — no per-handler change for JSON/multipart. EXCEPT the contact-check manual guard at line 418 (`if req.Msisdn == ""`) must also accept `req.Chat`. |
| `internal/whatsapp/client.go` | `types.ParseJID(to)` stays as-is (resolver produced a clean JID). Broaden `isRecipientError` (line 162) to match group-not-member / broadcast-unsupported → 4xx via `mapWhatsmeowErr`. |
| `internal/whatsapp/helper.go` | Reuse `StripDeviceIDFromJID` (line 32) inside the resolver — no new helper. `WhatsappDecomposeJID` (msisdn-extraction, different direction) left untouched. |

## Edge Cases

| Scenario | Risk | Handling |
|---|---|---|
| Both `chat` and `msisdn` supplied and differ (`chat=123@g.us`, `msisdn=628...`) | confusing-UX | `chat` wins; `msisdn` ignored whenever `chat` non-empty. Do NOT 400 (defeats the alias). Document precedence; optionally warn-log when both present and differ. |
| Neither supplied (both empty/whitespace) | none | Already rejected; change message to "chat (or msisdn) is required" → 400. |
| Garbage JID, unknown server (`foo@bar`, `x@example.com`) | correctness | Resolver rejects early (server not in allowlist) → 400 "invalid recipient". Better than whatsmeow's late `ErrUnknownServer`. |
| PN/LID JID with non-numeric user (`12ab34@s.whatsapp.net`) | correctness | GAP today (passes through, fails late). Require digits-only user for pn/lid; else 400 early. (Group user is numeric too — one shared rule covers all three.) |
| JID carries device/agent suffix (`628...:2@s.whatsapp.net`, natural from incoming sender / `reply_to.participant`) | correctness | GAP today: whatsmeow rejects at send with `ErrRecipientADJID`. Strip device+agent to non-AD in the resolver so the send succeeds. |
| Group JID for a group the account isn't in (`99999@g.us`) | correctness | Do NOT pre-check membership (GetGroupInfo = server round-trip, burns query budget). Let send fail, map the IQ error to 403/400. Needs exact error string (open). |
| Raw `@lid` (LID-only participant from `reply_to`, no PN alias) | correctness | Allow pass-through — required to reply to LID-only participants. whatsmeow attempts the send; delivery best-effort (works with an existing signal session / PN mapping, may fail cold). Map failure to 400/422; document not-guaranteed-dialable. |
| Self-send: own number, or `status@broadcast` | none | No special handling. whatsmeow supports note-to-self. Do not add a self-guard. |
| msisdn with `+`, spaces, dashes, parens (`+62 812-3456-7890`) | none | Already handled — strip `+` and non-digits. Keep current `strings.Map` behavior. |
| msisdn leading zero / `00` intl prefix (`0812...`, `006281...`) | correctness | Do NOT normalize (needs libphonenumber — YAGNI, impossible without caller country). Gateway expects E.164; leading zeros preserved as-is and WILL address the wrong number — the wrapping backend's responsibility. Document loudly. |
| Case-variant server (`@S.WHATSAPP.NET`, `@G.US`) | confusing-UX | Lowercase server before allowlist lookup. User is numeric so lowercasing the whole string is safe. |
| Non-status broadcast list (`123@broadcast`) | correctness | `broadcast` currently allowed but whatsmeow returns `ErrBroadcastListUnsupported` → unmatched → 500. Drop `broadcast` from the send allowlist (reject at resolver → 400) until broadcast-send is deliberately built. |

## Error Semantics

| Condition | Response |
|---|---|
| Both `chat` and `msisdn` empty/whitespace | 400 `{"error":"chat (or msisdn) is required"}` (ErrBadRequest) |
| Unknown/unsupported server, empty user, or non-numeric user for pn/lid/group | 400 `{"error":"invalid recipient \"<value>\""}` (ErrBadRequest), early in resolver |
| Bare input reducing to zero digits (`+++`, `abc`) | 400 `{"error":"invalid recipient \"<value>\""}` (ErrBadRequest) |
| Device-suffixed JID reaching whatsmeow unstripped | whatsmeow `ErrRecipientADJID` (contains "recipient" → `isRecipientError` → 400 today). After resolver strips device, send succeeds (preferred). |
| Valid-shaped JID whatsmeow rejects as `ErrUnknownServer` | 400 (already mapped via `isRecipientError` "unknown server"); resolver allowlist normally pre-empts this |
| Group send to a group the account isn't in | Currently likely 500 (unmapped). Recommend 403 (or 400) — depends on exact whatsmeow error (open). |
| Send to unroutable/cold `@lid` (no session/PN mapping) | whatsmeow encryption/send error → recommend 400 or 422 "recipient not reachable"; needs error-string confirmation |
| Non-status broadcast list send | `ErrBroadcastListUnsupported` currently → 500. Fix by rejecting at resolver → 400 (or matching the string → 400). |
| Client not registered / not logged in (unchanged) | 404 ErrClientNotFound / 401 "client not logged in" — existing behavior in `client.go` |

## Testing Decisions

Highest-seam-first. `resolveChat` is the marquee unit-test seam — pure, no deps, table-driven:
- precedence (chat wins), msisdn fallback, both-empty → 400
- digits-only user enforcement for pn/lid/group
- server lowercasing + allowlist (accept `s.whatsapp.net`/`lid`/`g.us`, reject unknown + `broadcast`)
- device/agent-suffix strip to non-AD
- zero-digit / all-punctuation rejects

Error mapping (`mapWhatsmeowErr` / `isRecipientError`) is testable **only if pinned to exported sentinels** — construct whatsmeow sentinel/`IQError` values and assert domain-error + status via `errors.Is/As`. **ADR: error mapping uses `errors.Is/As` sentinels, NOT substring** (the existing `isRecipientError` substring style is the trap — silently wrong on any whatsmeow text change, and un-unit-testable against real error values).

Not CI-testable (no live WhatsApp): the "needs the exact error string" cases — group-not-member, cold-`@lid` unroutable, broadcast-unsupported. **Verify once against the live whatsmeow build in a manual harness, then lock each to its exported sentinel** so CI pins it. Do not ship substring guesses. The usecase layer (Manager/Client interfaces) is the mocking seam for send-path tests; the testcontainers RabbitMQ harness is unaffected by this area (resolution happens before the queue/direct fork).

## Decisions for Review

- [ ] **Conflict policy when `chat` and `msisdn` differ:** silently prefer `chat` (true alias semantics) vs 400 on mismatch. **Recommend: prefer `chat`, optionally warn-log the mismatch.** 400 would break the back-compat alias.
- [ ] **Group-not-member send error mapping:** exact whatsmeow error type/string when `SendMessage` targets a group the account isn't in (note: `ErrNotInGroup` is only for group-info getters, not send). **Recommend: verify once live, map to 403, lock to a sentinel.**
- [ ] **Cold `@lid` routability:** is sending to a bare/cold `@lid` (from `reply_to`, no prior session) reliably routable, or should the API require a PN alias alongside `@lid`? **Recommend: allow with a documented best-effort caveat; verify once live to confirm allow/warn/reject.**
- [ ] **`broadcast` / `status@broadcast` as a send target:** keep now or drop until broadcast-send is a built feature. **Recommend: drop from the send allowlist (currently maps to a 500).**
- [ ] **National-format normalization:** confirm the gateway will NOT normalize national-format / leading-zero / `00`-prefixed numbers (caller must pass E.164), to avoid a libphonenumber dependency. **Recommend: confirm — no normalization, document loudly.**
- [ ] **Echo resolved `chat` in send responses:** add the resolved canonical `chat` JID alongside `message_id` so callers can persist the canonical form. **Recommend: yes, in this same wave (one release of SDK/OpenAPI churn).**

## Doc Surfaces

- [ ] `docs/openapi.yaml` — remove `msisdn` from `required` on the 7 request bodies, add `chat` to every Send*/Message*/ContactCheck body, mark `msisdn` deprecated, state "chat wins; msisdn is a back-compat alias; at least one required" in the description; add resolved-`chat` to send responses if approved. (`docs/openapi.go` is only the loader/server-URL injector — no change.)
- [ ] `llms.txt` — add a "chat vs msisdn addressing" note (precedence, accepted forms, `@lid` best-effort).
- [ ] `wiki/Environment-Variables.md` + `.env.example` + `CLAUDE.md` config list — **no new env keys for this area.**
- [ ] `wiki/Security-Considerations.md` — note `@lid` non-guaranteed dialability and the "caller must pass E.164 / leading zeros address the wrong number" responsibility (fits the "gateway must be wrapped" model).
- [ ] `CLAUDE.md` "API Endpoints" — note the `chat` field is accepted everywhere `msisdn` was.
- [ ] `docs/ui/assets/docs/*` — refresh the messaging pages to show `chat`.
- [ ] `CHANGELOG.md` — additive `chat` recipient + `msisdn` deprecation; OpenAPI version bump.
- [ ] SDK (`whatsapp-gateway-sdk-go`) — full parity: add `chat` to every method (and React's second recipient field, coordinated with the pending reaction change so it's ONE release).
- [ ] MCP (`mcp-whatsapp-gateway`) — curated subset: expose `chat` on the messaging tools.

## Out of Scope

- **Group management, reads, two-way primitives, ops** — those are their own PRDs; this PRD only builds the shared resolver + `chat` field they all consume.
- **Broadcast-list send** — deferred until deliberately built; dropped from the allowlist now.
- **Phone-number normalization (libphonenumber)** — explicitly excluded; caller passes E.164.
- **Durable message store** — deferred initiative; `reply_to` fidelity is bounded by caller-supplied fields, not stored here.
- **Pacing / query budget** — roadmap #2; this area is read-free and adds no server round-trips (no membership pre-check).
- **New webhook event types** — roadmap #1 envelope window.

## Dependencies & Sequencing

- **Before:** nothing. This is sequence position 1, unconditional and backward-compatible, ships without a flag.
- **After (unblocked by this):** every other automation-platform area resolves recipients through `resolveChat` — two-way primitives (participant/`reply_to.participant` JID resolution), group management (which additionally requires `validateGroupJID` since bare-digit→user is wrong for groups), and reads. Land this first so no area invents its own addressing.
- **Coordinate:** the resolved-`chat` echo + React second-recipient field with the SDK so the OpenAPI/SDK churn is a single release, not two.
