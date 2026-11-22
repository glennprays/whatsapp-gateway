# PRD 2 — Two-Way Conversation Primitives — read receipts, typing, replies, mentions

> **Status:** Not started  ·  **Sequence position:** 4  ·  **Ban-risk:** low (interim action cap)
> **Depends on:** PRD 0   ·   **Effort rollup:** M
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | Deliverable (endpoint / change) | whatsmeow call | Effort | Ban-risk | Status |
|---|---|---|---|---|---|
| 1 | Mark as read / send read receipt — `POST /api/message/read` | `MarkRead(ctx, ids, ts, chat, sender, receiptType)` | S | low | ☐ |
| 2 | Chat presence: typing / recording / paused — `POST /api/chat/presence` | `SendChatPresence(ctx, jid, presence, media)` | S | low | ☐ |
| 3 | Reply (`reply_to`) as optional field on all send endpoints | `waE2E.ContextInfo{StanzaID,Participant,QuotedMessage}` + `SendMessage` | M | low | ☐ |
| 4 | Mentions as optional field on text/caption sends | `waE2E.ContextInfo.MentionedJID` + `SendMessage` | S | low | ☐ |
| 5 | Global online presence toggle — `POST /api/chat/presence?scope=global` | `SendPresence(ctx, available\|unavailable)` | S | low | ☐ |

## Problem Statement

Today the gateway is fire-only: a backend can push a text/media message but cannot participate in a conversation the way a human client does. A backend integrator building a support inbox, a bot, or an AI assistant on top of the gateway has no way to:

- **Acknowledge** an incoming message (mark it read / send a read or played receipt), so the sender never sees blue ticks and the account looks like a bot that ignores you.
- **Signal activity** (typing / recording indicator), so replies appear abruptly with no "…is typing" cue — the single biggest tell that a chat is automated.
- **Thread a reply** to a specific earlier message, so every answer lands as a fresh message with no quoted context.
- **Mention** a participant in a group so they get pinged.

These are the primitives that make an automated chat feel like a real conversation. Without them the platform can send but cannot converse.

## Solution

Add four conversation primitives, all reusing the existing send path and its guards (get client → `IsLoggedIn` → `ParseJID` → `mapWhatsmeowErr`), so they inherit addressing (PRD 0 `resolveChat`) and error semantics for free:

- **`POST /api/message/read`** — map a request `type` (`read`|`played`) to exactly one whatsmeow `ReceiptType` and call `MarkRead`. Group receipts require `sender`.
- **`POST /api/chat/presence`** — map `state` (`composing`/`typing`/`recording`/`paused`) to `SendChatPresence`; `recording` = composing + media=audio. Optional server-side auto-`paused` timer so callers make one call, not two.
- **`reply_to` field** on every `/api/message/*` send — inject `ContextInfo{StanzaID,Participant,QuotedMessage}` onto the outgoing sub-message; text sends switch `Conversation` → `ExtendedTextMessage`.
- **`mentions` field** on text/caption sends — populate `ContextInfo.MentionedJID` and validate that each mention's user-part appears as an `@`-token in the text.

Replies are **lightweight** (resolved decision #5): the caller supplies `reply_to`; there is **no durable message store**, so preview fidelity is bounded by the optional `quoted_text`/`quoted_type` the caller passes. These are all conversation-affecting **actions** (decision #3) — they route through the same send path and, until the pacer (roadmap #2) lands, a crude per-account **action cap** reusing the existing limiter (key `action:`+phone).

## User Stories

1. As a **support-agent backend**, I want to mark an incoming customer message as read, so the customer sees blue ticks and knows their message was seen.
2. As a **backend integrator**, I want to send a `played` receipt on a voice note, so the sender sees the mic-icon-played state after my bot transcribes it.
3. As a **backend integrator**, I want a single receipt-type field validated to exactly one value, so I never trip whatsmeow's panic on more than one receipt type.
4. As a **support-agent backend**, I want group mark-read to require the original `sender`, so the receipt is attributed to the right participant instead of being silently dropped.
5. As a **backend integrator**, I want mark-read on a message my node never saw to return `200` best-effort, so I don't have to maintain a durable message store just to acknowledge.
6. As an **AI assistant (MCP)**, I want a `typing` indicator before I answer, so the conversation feels human and my reply doesn't appear out of nowhere.
7. As an **AI assistant (MCP)**, I want a `recording` indicator before I send a voice note, so the recipient sees the audio cue rather than the text cue.
8. As a **backend integrator**, I want to send `paused` to clear the indicator immediately, so a cancelled reply doesn't leave a stuck "typing…" on the peer's screen.
9. As a **backend integrator**, I want an optional server-side auto-pause timer, so I can fire one `composing` call and let the gateway clear it, instead of scheduling a second call myself.
10. As a **backend integrator**, I want to reply-thread to a specific message, so my answer quotes the message it responds to.
11. As a **support-agent backend**, I want group replies to carry the quoted author (`participant`), so the reply threads and attributes correctly in the group.
12. As a **backend integrator**, I want to pass `quoted_text`/`quoted_type`, so the reply shows a minimal preview even though the gateway has no durable copy of the original.
13. As a **backend integrator**, I want reply-threading to work with just `StanzaID`+`participant` when I have no quoted body, so the thread link still forms even if the preview is sparse.
14. As a **backend integrator**, I want to mention participants in a group message, so the mentioned users get pinged.
15. As a **backend integrator**, I want strict mention validation (400 if the `@`-token is missing from my text), so I stay in control of where the mention renders and never send an unhighlighted mention.
16. As an **AI assistant (MCP)**, I want mark-read and typing exposed as safe tools, so I can converse naturally — while group-management mutations and admin stay out of my reach (decision #8).
17. As an **operator**, I want typing/mark-read spam bounded by an interim per-account action cap, so an over-eager AI agent can't hammer receipt/chatstate nodes and raise the account's ban profile before the real pacer lands.
18. As a **backend integrator**, I want to edit or delete a message that is itself a reply by targeting its own `message_id`, so I don't need special logic for replies.
19. As a **backend integrator**, I want mark-read on a text message with a `played` type documented as harmless-but-meaningless, so I know `played` only matters for voice/view-once.
20. As a **backend integrator**, I want the resolved canonical `chat` echoed on send responses, so I can persist the canonical Chat form the platform is built around (shared with PRD 0's addressing wave).
21. As an **operator**, I want global online-presence to be caller-driven and default-off with no gateway heartbeat, so the account doesn't acquire an "always online" behavioral fingerprint.

## Implementation Decisions

**Modules & contracts.** All four primitives extend the existing Manager/Client interfaces and reuse the react/edit/delete pattern (get client, `IsLoggedIn` guard, `ParseJID`, `mapWhatsmeowErr`). A new `buildContextInfo(replyTo, mentions)` helper produces the `waE2E.ContextInfo` shared by replies and mentions.

**Receipt-type mapping (critical).** whatsmeow **panics on more than one receipt type**. The usecase maps the request `type` string to **exactly one** `types.ReceiptType` (`read` → `ReceiptTypeRead`, `played` → `ReceiptTypePlayed`); unknown or multiple → `400` before the call. Never forward more than one vararg.

**Presence mapping.** There is no separate `recording` state: `recording` → `SendChatPresence(composing, ChatPresenceMediaAudio)`; `typing`/`composing` → `media=text`; `paused` clears both. Optional `CHAT_PRESENCE_AUTO_PAUSE_SECONDS` schedules a server-side `paused` after a `composing`/`recording`.

**Reply plumbing.** `Send*` gains an optional `*MessageContext`; `buildContextInfo` injects `ContextInfo` onto the populated sub-message. `SendTextMessage` switches `Conversation` → `ExtendedTextMessage{Text, ContextInfo}` when context is present. In groups, `reply_to.participant` is **required** (quoted author); in 1:1 it defaults to peer/self. With no durable store, `QuotedMessage` is synthesized from `quoted_text`/`quoted_type` when supplied, else omitted.

**Mentions.** Populate `ContextInfo.MentionedJID` and validate each mention's JID user-part appears as an `@`-token in the text; `400` listing any missing. Token uses the **resolved participant JID's user-part** (so a `@lid` participant matches its lid user-part, not a PN).

**Interim pacing (ban mitigation).** Per decision #3 these actions should route through the send-path pacer, but the pacer (#2) is deferred. Interim: go **direct** (like react/edit/delete) with a crude per-account cap reusing the existing limiter, key `action:`+phone. Documented as a best-effort ceiling, not a delivery guarantee.

### Request/response contract sketches

- `MarkReadRequest` — `{ chat, message_ids[], sender?, type ("read"|"played", default read), timestamp? }`
- `ChatPresenceRequest` — `{ chat, state ("composing"|"typing"|"recording"|"paused"), duration_ms? }`; global scope via `?scope=global` or `{ scope: "global", available: bool }`
- `Send*Request` additions — `reply_to { message_id, participant?, quoted_text?, quoted_type? }`, `mentions []string`
- **Success responses** — mark-read / presence: `200 {success:true}` (receipt/chatstate nodes carry no `message_id`); reply/mention send: `200 {message_id}` direct or `202 {job_id}` queued — identical to a normal send of that type. Send responses also echo the resolved canonical `chat` (shared with PRD 0's addressing wave).

### Config additions

| Key | Purpose | Default |
|---|---|---|
| `CHAT_PRESENCE_AUTO_PAUSE_SECONDS` | Optional server-side timer that auto-sends `paused` N seconds after a `composing`/`recording`, so callers avoid a second call. `0` disables (caller sends `paused` explicitly). | `0` |

### Integration seams

| File | What |
|---|---|
| `internal/whatsapp/client.go` | Add `MarkRead` + `SendChatPresence` client methods (Client interface + impl) following the react/edit/delete pattern. Add `buildContextInfo(replyTo, mentions)`. Refactor `Send*` to accept an optional `*MessageContext` and inject `ContextInfo`; `SendTextMessage` switches `Conversation` → `ExtendedTextMessage`. |
| `internal/whatsapp/manager.go` | Extend Manager interface + thin wrappers: `MarkRead(ctx,traceID,phone,chat,sender,ids,type,ts)`, `SendChatPresence(ctx,traceID,phone,chat,state,media,durationMs)`, plus optional context param on existing `Send*`. |
| `domain/whatsapp/message.go` | New `MarkReadRequest`, `ChatPresenceRequest`; add `ReplyTo{MessageID,Participant,QuotedText,QuotedType}` and `Mentions []string` to `Send*Request`. `chat` canonical, `msisdn` back-compat alias (decision #2). |
| `internal/usecase/whatsapp/message_usecase.go` | New `MarkRead`/`SendChatPresence` methods + validation: reuse `validateRecipient` for chat/sender/mention JIDs; group→sender/participant required; receipt-type string → single `types.ReceiptType` (panic guard); mention `@`-token presence check; recording→composing+audio. |
| `internal/handler/whatsapp/whatsapp_message_handler.go` | Add MarkRead + Presence handlers (BodyParser, GetTraceID, MustGetPhoneNumber, httperror mapping). Parse `reply_to`/`mentions` in existing send handlers and thread into usecase calls. |
| `internal/router/message_routes.go` | Register `POST /message/read`. Add a `/chat` route group under JWT for `POST /chat/presence` (+ optional global presence). |
| `domain/queue/*.go` (`OutgoingMessageJob`) | When mark-read/typing later route through the queue+pacing envelope (#2), add `ReplyTo`/`Mentions` to the job + an action-type discriminator so workers dispatch through the same paced path. |

## Edge Cases

| Scenario | Risk | Handling |
|---|---|---|
| Group mark-read with no `sender`/`participant` | correctness | `MarkRead` only attaches participant when sender is set + chat is a group; without it the receipt is mis-attributed/ignored. Require `sender` (400) when chat ends `@g.us`/is a lid group; optional for 1:1. |
| `>1` receipt type passed to `MarkRead` | correctness | whatsmeow **panics** on >1. Map request `type` to exactly one (`read`\|`played`); reject unknown/multiple at usecase (400). Never forward more than one vararg. |
| Mark-read a `message_id` we never received | confusing-UX | No durable store to verify; whatsmeow sends the node blindly and the server no-ops. Accept best-effort, return 200, document "read receipt is fire-and-forget, not confirmed". Optionally cross-check the in-memory incoming buffer. |
| Sending account has read-receipts privacy OFF | confusing-UX | `MarkRead` silently downgrades read→read-self (no blue ticks). Not detectable per-call; return 200 and document blue-tick visibility depends on the account's own privacy. |
| `played` receipt while read-receipts privacy OFF | security | whatsmeow does **not** downgrade played→played-self (open TODO), so a played receipt leaks that you opened a voice/view-once even with privacy off. Optionally check `GetPrivacySettings` and refuse/downgrade `played` when privacy is None. |
| `played` receipt on a plain text message | none | Harmless but semantically wrong. Pass through; document `played` is only meaningful for voice notes (PTT) and view-once media. |
| Typing started but never stopped | confusing-UX | WhatsApp auto-clears composing after a few seconds; indicator lingers until then. Support `paused` for immediate stop; offer optional server-side auto-pause timer rather than forcing two calls. |
| `recording` vs `composing` indicator | correctness | No separate recording state — it's composing + media=audio. Map `recording` → `SendChatPresence(composing, ChatPresenceMediaAudio)`; `typing`/`composing` → media=text; `paused` clears both. |
| Group `reply_to` missing `participant` | correctness | Quoted author must be in `ContextInfo.Participant` or the reply won't thread/attribute. Require `reply_to.participant` for groups; optional in 1:1. |
| `reply_to` with no quoted content | confusing-UX | No durable store to rebuild `QuotedMessage`; clients may render a blank preview. Accept optional `quoted_text`/`quoted_type` to synthesize a minimal `QuotedMessage`; without it, send just `StanzaID`+`Participant` (thread links, preview sparse). |
| Mentions listed but `@<user>` token absent from text | confusing-UX | Highlight requires BOTH `MentionedJID` set AND the literal `@<user-part>` in text. Validate each mention's user-part appears as an `@`-token; 400 listing the missing ones (caller controls placement) rather than sending an unhighlighted mention. |
| Mention JID is a `@lid` / non-participant | correctness | The token must use the JID's user-part; a PN token won't match a lid participant and vice-versa. Use the resolved participant JID's user-part for the token; skip membership check in 1:1. lid↔pn token rendering is an open question. |
| Edit/delete a message that is itself a reply | none | Edit/delete/revoke target the reply's own `message_id`, not the quoted original — no special logic. Deleting the quoted original just leaves a dangling preview; document it. |
| Batch mark-read spanning different senders | correctness | `MarkRead` attributes ALL ids to the single `sender` param. Document one-sender-per-call; either reject mixed batches or accept an array of `{sender, message_ids}` groups and loop. |
| Spamming typing/mark-read at high frequency | ban | Low individual risk, high frequency. Per decision #3 these are conversation-affecting actions that must pace through the send path; go direct for now (like react/edit/delete) behind the interim per-account action cap, slot into the pacing envelope (#2) when it lands. |

## Error Semantics

| Condition | Response |
|---|---|
| chat/recipient missing, empty, or JID with unknown server | `400 {error}` via `validateRecipient` (reuses existing "invalid recipient" mapping) |
| group mark-read or group `reply_to` without sender/participant | `400 "sender/participant is required for group chats"` |
| mark-read with empty `message_ids` | `400 "message_ids is required"` (pre-empt whatsmeow's "no message IDs specified") |
| receipt type not in `{read,played}` or more than one supplied | `400 "invalid receipt type"` — never forward >1 to `MarkRead` (it panics) |
| typing state not in `{composing,typing,recording,paused}` | `400 "invalid presence state"` |
| mentions supplied but `@<user>` token absent from text | `400 "mention <jid> not referenced in text"` |
| client not registered in the in-memory map | `404 client not found` (`ErrClientNotFound`) |
| client registered but not logged in | `401 "client not logged in"` |
| session deleted by server (`store.ErrDeviceDeleted`) | `409 re-pair required` (`ErrClientSessionDeleted`, via `mapWhatsmeowErr`) |
| mark-read or typing success | `200 {success:true}` — receipt/chatstate nodes, no `message_id` to return |
| reply/mention send success | `200 {message_id}` direct or `202 {job_id}` queued — identical to a normal send of that type |

## Testing Decisions

Highest-seam-first, unit-testable at the **usecase layer** and Manager/Client interfaces without a live WhatsApp:

- **Receipt-type mapping** — table-driven, assert exactly one `types.ReceiptType` produced; reject unknown/multiple. This is the critical panic guard (whatsmeow panics on >1) and MUST be unit-tested.
- **Presence-state map** — `recording`→composing+audio, `typing`/`composing`→text, `paused` clears both; reject unknown state.
- **Mention `@`-token validation** — user-part present-in-text passes, missing → 400 listing missing JIDs; lid user-part vs pn user-part token selection.
- **Group required-field guards** — group mark-read requires `sender`; group `reply_to` requires `participant`; 1:1 optional. Reuses `resolveChat` (PRD 0) for JID resolution.
- **buildContextInfo** — reply-only, mention-only, both; text send switches `Conversation`→`ExtendedTextMessage` when context present.
- **Error mapping** — construct whatsmeow sentinel errors and assert domain-error+status via `errors.Is/As`. Per the error-mapping ADR, use **exported sentinels, not substring matching** — substring can't be unit-tested against real error values.

**Not CI-testable (no live WhatsApp):** actual `MarkRead`/`SendChatPresence`/receipt-privacy-downgrade behavior and the "played not downgraded" leak. Only one testcontainers test exists today (RabbitMQ). Verify these **once against the live whatsmeow build** in a manual harness, then lock each to an exported sentinel so CI pins it going forward — do not ship substring guesses. Add a unit test proving the interim action-cap limiter is wired on typing/mark-read.

## Decisions for Review

- [ ] **Route mark-read/typing direct or through the paced envelope now?** Decision #3 says same send-path + pacing, but the pacer (#2) is deferred. **Recommend:** go **direct** now (like react/edit/delete) behind a crude per-account action cap (reuse existing limiter, key `action:`+phone) + a TODO to route through the paced envelope when #2 lands.
- [ ] **Global online presence (`SendPresence` available/unavailable) — own endpoint + gateway heartbeat, or caller-driven?** It flips active-vs-inactive delivery receipts and "online" visibility and makes typing look natural. **Recommend:** expose via `?scope=global` (or `POST /api/presence`), **default-off, single-shot, NO gateway-managed heartbeat** — a permanent "always online" is a behavioral fingerprint / ban signal (per critique).
- [ ] **Mentions — strict validation vs auto-append tokens the caller didn't place?** **Recommend:** **strict** (400 when the `@<user>` token is missing) to keep the caller in control of placement.
- [ ] **`@lid` vs `@pn` mention/reply token rendering — which user-part goes in the `@` token and `MentionedJID` for a hidden-lid group participant?** **Recommend:** use the **resolved participant JID's user-part** (lid token for a lid participant, pn for pn); flag for live verification since client highlight behavior isn't CI-testable.
- [ ] **Synthesize `QuotedMessage` from `quoted_text`/`quoted_type`, or require the full original body?** **Recommend:** **synthesize a minimal preview** from optional `quoted_text`/`quoted_type` (decision #5: no durable store); without it, send just `StanzaID`+`Participant` (thread links, preview sparse).
- [ ] **Block/downgrade a `played` receipt when the account's read-receipt privacy is off?** whatsmeow does not auto-downgrade played→played-self, leaking that a voice/view-once was opened. **Recommend:** check `GetPrivacySettings` and **refuse/downgrade `played` when privacy is None** (close the leak); document the behavior.
- [ ] **Batch mark-read across multiple senders — reject mixed batches or accept grouped `{sender, message_ids}`?** **Recommend:** accept an **array of `{sender, message_ids}` groups** and loop `MarkRead` per group internally (one whatsmeow call per sender); reject a flat mixed batch.
- [ ] **Endpoint placement — new `/chat/*` group or keep under `/message/*`?** **Recommend:** put presence under a new **`/chat/*`** group (aligns with the canonical "Chat" term, decision #2); keep mark-read under `/message/read` since it acts on messages.

## Doc Surfaces

- [ ] `docs/openapi.yaml` — add `POST /api/message/read`, `POST /api/chat/presence` (+ `scope=global`), and the `reply_to`/`mentions` fields on all `/api/message/*` send bodies; add the echoed resolved `chat` to send responses. (`docs/openapi.go` is only the loader — no codegen.)
- [ ] `llms.txt` — add the two-way primitives family (mark-read, presence, replies, mentions) + the "chat vs msisdn addressing" note.
- [ ] `wiki/Environment-Variables.md` + `.env.example` + `CLAUDE.md` config list — add new key: **`CHAT_PRESENCE_AUTO_PAUSE_SECONDS`** (default `0`).
- [ ] `wiki/Security-Considerations.md` — note the interim best-effort action cap (no real pacer yet), the `played`-receipt privacy leak, and global-presence-as-fingerprint (default-off, no heartbeat).
- [ ] `docs/ui/assets/docs/*` — reference page(s) for mark-read, presence/typing, replies, mentions.
- [ ] `CLAUDE.md` "API Endpoints" list — add `POST /api/message/read`, `POST /api/chat/presence`, and note `reply_to`/`mentions` on send endpoints.
- [ ] `CHANGELOG.md` + openapi.yaml version bump.
- [ ] SDK (`whatsapp-gateway-sdk-go`) — full parity (decision #8): `MarkRead`, `SendChatPresence`, and `reply_to`/`mentions` on every send method.
- [ ] MCP (`mcp-whatsapp-gateway`) — curated subset (decision #8): expose **mark-read + typing** tools; flag the interim-pacing ban risk on them. No group mutations, no admin.

## Out of Scope

- **Durable message store** — replies stay lightweight (decision #5); no server-side rebuild of `QuotedMessage`. A durable store is a separate deferred initiative.
- **Real send-path pacer** — roadmap #2. This PRD ships only the crude interim per-account action cap.
- **New webhook event types** (e.g. `message.read`/`message.played` inbound events) — deferred to the roadmap #1 envelope window; this PRD ships request/response only.
- **Gateway-managed online-presence heartbeat** — explicitly excluded (ban-signal); global presence is single-shot caller-driven.

## Dependencies & Sequencing

- **Depends on PRD 0 (addressing foundation)** — `resolveChat` + `chat` field on request structs + participant JID resolution are prerequisites for `reply_to.participant`, mention token user-parts, and group sender attribution. This PRD sits at **sequence position 4**, after addressing (0), idempotency/health (1 subset), and read/query infra (3) per the dependency order.
- **Ships with the interim action cap** — reuse the existing message/query limiter (key `action:`+phone) until roadmap #2's pacer replaces it.
- **Echoing resolved `chat`** on send responses is shared with PRD 0's addressing wave — do it in one release of SDK/openapi churn, not two.
- **After:** roadmap #1 (webhook events — adds inbound read/played event emission) and #2 (pacing — replaces the interim cap), per decision #7.
