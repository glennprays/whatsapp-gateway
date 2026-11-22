# Gateway Roadmap & Cross-Repo Update Plan

> Status snapshot after **v0.11.0** (shipped 2026-07-05). This is a working planning doc, not a commitment. Captures the two architectural initiatives agreed with the maintainer, the one deliberately declined, and the SDK / MCP / docs work that must follow any gateway change.

---

## 1. Where we are after v0.11.0

v0.11.0 moved the gateway from a "notification sender" toward a general-purpose integration:

- **New message types**: `POST /message/audio` (voice notes via `is_ptt`), `/message/video` (`is_gif`), `/message/document` (`file_name` + `caption`)
- **New endpoint**: `GET /contact/check` (`IsOnWhatsApp` validation)
- **Fixes**: poll votes now decryptable (`BuildPollCreation`), reactions attribute correctly (`BuildReaction` + optional `sender_msisdn`), `@lid` no longer leaks into webhook `from` (new `addressing_mode` field), JWT duration overflow clamped
- **Hardening**: outbound media size cap (`MAX_UPLOAD_BYTES`) + per-kind MIME allow-lists, per-IP `/register` rate limit

What's **still** missing — the subject of this doc — is **feedback and pacing**: the gateway is deaf to most whatsmeow events (no delivery/read, no ban/session signals), and its send limiter rejects instead of pacing (the pattern that gets numbers banned).

---

## 2. The three architectural points — decisions

| # | Initiative | Decision | Rationale |
|---|---|---|---|
| 1 | **Webhook event system** — generalize delivery so delivery/read/played receipts, session lifecycle, and bans reach the backend | **DO** | Highest user-facing value; closes the outbound status loop and enables #2's ban gating |
| 2 | **Outbound pacing** — token-bucket + per-recipient cap + jitter, pace-not-reject, ban-aware | **DO** | Directly prevents the #1 cause of WhatsApp number bans |
| 3 | **JWT `jti` + single-active-token + revocation** | **DECLINE** | See §5 — the gateway is a backend-internal service by design; auth is layered above |

---

## 3. Initiative #1 — Webhook Event System

### 3.1 Current state (the problem)

`internal/whatsapp/handler.go:75` `HandleEvent` switches on `evt.(type)`, handles **only `*events.Message`** (forwarded as `message.incoming`), and the `default:` branch silently drops **every other event**. The delivery plumbing is hardcoded to that one shape — `buildWebhookPayload` / `deliverWebhook` only know how to serialize a `*events.Message` (handler.go:180, mirrored in `internal/queue/handlers/incoming_event_handler.go:88` for queue mode).

Result: the backend learns **nothing** about:

| whatsmeow event | What the backend never learns | Impact |
|---|---|---|
| `events.Receipt` (`Delivered`/`Read`/`Played`) | Whether an **outbound** message was delivered / read / a voice note was played | `message.sent` today only means "server accepted." No way to tell "sent but undelivered" from "read." **#1 sender-side gap.** |
| `events.LoggedOut` | Device was unpaired — session is dead | Backend keeps sending into a dead session until it 404s |
| `events.TemporaryBan` (`Expire`, codes 101–106) | **WhatsApp banned the number** for a window | Continuing to send during a ban → escalation to a *permanent* ban. **Dangerous.** |
| `events.Connected`/`Disconnected`/`StreamReplaced` | Connection up/down / conflicting login | Backend discovers state only via polling `/login/status` or send failures |
| `events.CallOffer` / `CallTerminate` | Incoming voice/video call | Bots usually want to auto-reject or notify |
| `events.UndecryptableMessage` | A real message arrived but couldn't be decrypted | Sender's message silently lost |
| `events.PairSuccess` / `QRScannedWithoutMultidevice` | Pairing completed / "enable multidevice" hint needed | Event-driven pairing confirmation |

### 3.2 Target design

Generalize the delivery path to a **generic envelope**:

```jsonc
{
  "event": "message.delivered",          // discriminator (already present on message.incoming today)
  "phone_number": "628...",
  "jid": "628...@s.whatsapp.net",        // account the event concerns
  "timestamp": 1735900000,
  "payload": { ... }                      // event-specific
}
```

Then add `case` branches in `HandleEvent` mapping each whatsmeow event → its webhook. `message.incoming` keeps its **current** payload (moved into `payload`) so the wire shape is preserved.

**Proposed event catalog (phase 1):**

| `event` | whatsmeow source | `payload` sketch |
|---|---|---|
| `message.incoming` *(existing)* | `*events.Message` | unchanged |
| `message.delivered` | `events.Receipt` (`Delivered`) | `{message_ids[], chat, from}` |
| `message.read` | `events.Receipt` (`Read`) | `{message_ids[], chat, from}` |
| `message.played` | `events.Receipt` (`Played`) | `{message_ids[], chat, from}` — view-once opened / voice note played |
| `session.logged_out` | `events.LoggedOut` | `{reason}` — also evicts client from ClientStore |
| `session.banned` | `events.TemporaryBan` | `{code, reason, expires_in}` — also gates the limiter (see #2) |
| `session.connected` / `disconnected` / `replaced` | `Connected`/`Disconnected`/`StreamReplaced` | `{}` |
| `call.offer` | `events.CallOffer` | `{call_id, from}` |
| `message.undecryptable` | `events.UndecryptableMessage` | `{message_id, from, reason}` |

### 3.3 Backward compatibility — **yes, if done additively**

| Aspect | Changes? |
|---|---|
| `message.incoming` payload shape | **No** — same fields, just nested under `payload` (or kept flat; TBD — flat is more backward-compatible) |
| HMAC signing (`X-Webhook-Signature`, `sha256=…`) | **No** |
| New `event` values | **Added** — new payload shapes for new event types, same webhook URL |

**The one break risk:** a consumer that parses *every* webhook through the incoming-message parser. `message.delivered` won't unmarshal into an `IncomingWebhookPayload`. Fix = dispatch on `event` before parsing — which the SDK should own (§6).

**Recommendation:** keep `message.incoming` fields **flat at top level** (not nested under `payload`) so existing consumers need zero changes. Only new event types carry event-specific fields.

### 3.4 Reliability tie-ins (not just notify)

The receipt + lifecycle events also enable **correctness** work, not just webhooks:
- On `events.Receipt`: resolve `message_ids` back to job IDs and update `JobRepository` so `GET /message/job/:id` can eventually return the full `queued → sent → delivered → read` chain.
- On `events.LoggedOut`: evict the dead client from `ClientStore` (it stays in the map today).
- On `events.TemporaryBan`: gate the limiter (see #2).

### 3.5 Implementation plan

1. Refactor `buildWebhookPayload`/`deliverWebhook` to take a generic `(event, payload)` instead of `*events.Message`. ~M.
2. Add a `case` per event in `HandleEvent` (each ~S). Start with `Receipt` + `LoggedOut` + `TemporaryBan` (highest value).
3. Wire receipt → `JobRepository` status update.
4. Update `domain/queue` if a queue-mode event-forwarding path is needed (queue mode currently only serializes `events.Message`).
5. **Cross-repo:** ship unified SDK dispatcher (§6) + MCP event surface (§7) in the same release window.

**Effort:** M for the refactor; each event S. The receipt→job linkage is the meatiest piece.

---

## 4. Initiative #2 — Outbound Pacing (ban avoidance)

### 4.1 Current state

The limiter **is** applied on both paths (correcting an earlier note): the queue worker (`internal/queue/handlers/outgoing_message_handler.go:63`) and the direct-mode usecase (`internal/usecase/whatsapp/message_usecase.go:214/670/743`). But three properties make it a ban magnet:

1. **Reject, not pace.** Over budget → `429` (caller retries). It doesn't slow the caller to a safe rate.
2. **Fixed window, per-phone only** (`pkg/ratelimiter/memory.go`). 100 msgs/window keyed by sender phone → a full 100-msg burst is allowed at a window edge. WhatsApp's anti-spam trips on exactly these bursts.
3. **No per-recipient cap.** The single biggest ban trigger is many messages to **non-contacts** / repeat messages to the same recipient. Keying only on sender phone means "100 messages to 100 strangers in 5s" passes.

### 4.2 Target design

Swap the algorithm for a **token bucket** (or leaky bucket):

- **Steady refill** (e.g. 1 token/sec) + small burst capacity → sends naturally space out instead of clockwork.
- **Per-account AND per-recipient keying** → cap both aggregate throughput and per-recipient frequency.
- **Jitter** on the inter-message gap → traffic isn't perfectly periodic.
- **Pace, don't reject:** when no token is available, *wait* for one (within a deadline) instead of `429`. The caller sees latency, not a failure. (Optionally configurable: `pacing_mode = pace | reject`.)
- **Ban-aware:** when `events.TemporaryBan` fires (#1), drain the bucket to zero and hold until `Expire` elapses, then auto-resume. Persist `banned_until` so a restart doesn't re-flood.

### 4.3 Why it's "architectural"

It changes **observable throughput/latency**: a paced send blocks ~1s instead of failing fast; queue depth grows under load (correct — a deep queue is better than a banned number). It also needs:
- The per-recipient dimension threaded through the limiter key.
- The direct-mode action paths (react/edit/delete) sharing the same pacing.
- `TemporaryBan` integration from #1.

### 4.4 Implementation plan

1. Add a token-bucket implementation to `pkg/ratelimiter` (alongside the existing memory/redis backends).
2. Introduce a **pacing wrapper** that `Allow()`-blocks-up-to-deadline instead of returning 429.
3. Re-key sends to `(account, recipient)`; add a secondary per-account aggregate cap.
4. Add `banned_until` state (in-memory + persisted) gated by #1's `TemporaryBan` event; auto-resume on expiry; expose admin force-resume.
5. Apply uniformly to all send + action paths (text/image/audio/video/document/sticker/poll/react/edit/delete).

**Effort:** M. Mostly contained to `pkg/ratelimiter` + send paths + the ban gate.

---

## 5. Declined — #3 JWT revocation (rationale + the one doc note)

**Decision: not doing this on the gateway.** The gateway is intentionally a **backend-internal integration service** (private network, behind the wrapping backend's auth) — not a front-facing API. Layering token revocation / single-active-token / ownership at the gateway would duplicate the backend's job and add friction for the "flexible pipe" use case. This is already documented in the gateway's security section.

**One residual worth a doc sentence (not code):** because logout tears down only the WhatsApp session (not the JWT), a token stays valid until natural expiry. The *wrapping backend* should mint **short-lived** tokens (or rotate on logout) for sensitive deployments. This is a backend-side recommendation consistent with "the gateway is not the security boundary." → goes into the docs update (§8).

---

## 6. SDK update requirements (`whatsapp-gateway-sdk-go`)

The SDK lags the gateway. After v0.11.0 and through #1/#2:

### 6.1 New client methods (mirror v0.11.0 endpoints)
Existing: `SendText`, `SendImage`, `SendLocation`, `SendPoll`, `SendSticker`.
**Add:**
- `SendAudio(ctx, msisdn, audio io.Reader, isPTT, isViewOnce)` → `POST /message/audio`
- `SendVideo(ctx, msisdn, video io.Reader, caption, isGif, isViewOnce)` → `POST /message/video`
- `SendDocument(ctx, msisdn, doc io.Reader, fileName, caption)` → `POST /message/document`
- `CheckContact(ctx, msisdn)` → `GET /contact/check`

### 6.2 Reaction change
`React` / message-react request gains an optional `SenderMsisdn` field (matches the gateway's new `sender_msisdn`).

### 6.3 Webhook dispatcher (the structural change — needed for #1)
Today the SDK has **two separate** parse methods (`webhook.go:80` `ParseIncomingWebhook`, `:108` `ParseOutgoingWebhook`) and the caller picks which. There is **no** "parse any webhook" path. **Add a unified dispatcher:**

```go
type WebhookEvent interface{ Event() string }

// ParseWebhook verifies the signature, reads `event`, and unmarshals into the
// matching typed payload. Existing message.incoming + outgoing still parse.
func (v *WebhookVerifier) ParseWebhook(body []byte, sig string) (WebhookEvent, error)
```

This is what makes #1's new events usable. Consumers bump the SDK, switch to `ParseWebhook`, and add `switch` cases for the events they care about. Non-breaking for `message.incoming`.

### 6.4 Release
Bump the SDK, update `CHANGELOG.md`, tag alongside the gateway release that ships #1.

---

## 7. MCP update requirements (`mcp-whatsapp-gateway`)

The MCP server (`internal/tools/messaging.go`) currently exposes: `send_text_message`, `send_image_message`, `send_location_message`, `send_poll_message`, `send_sticker_message`.

### 7.1 New tools (mirror v0.11.0)
**Add:**
- `send_audio_message` — args: `msisdn`, `audio` (path/bytes), `is_ptt`, `is_view_once`
- `send_video_message` — args: `msisdn`, `video`, `caption`, `is_gif`, `is_view_once`
- `send_document_message` — args: `msisdn`, `document`, `file_name`, `caption`
- `check_contact` — args: `msisdn`; returns jid / is_on_whatsapp / verified_name

### 7.2 Gap (pre-existing, worth noting)
The MCP server has **no** `react`/`edit`/`delete` tools despite the gateway supporting them. Consider adding in the same pass.

### 7.3 Depends on SDK
The MCP server is a consumer of `whatsapp-gateway-sdk-go`, so §6.1 must land first.

---

## 8. Docs update checklist

After v0.11.0 (do now) and again when #1/#2 ship:

### 8.1 Gateway `docs/openapi.yaml`
- Add paths: `/message/audio`, `/message/video`, `/message/document`, `/contact/check`
- Add request/response schemas for the new types (audio/video/document/contact-check)
- Update the reaction schema with optional `sender_msisdn`
- Add `addressing_mode` to the incoming-message webhook schema
- Note new error responses: 429 on `/register` and on media size/MIME violations

### 8.2 Gateway `wiki/`
- `Environment-Variables.md`: add `MAX_UPLOAD_BYTES`, `REGISTER_RATE_LIMIT_ENABLED`, `REGISTER_RATE_LIMIT_REQUESTS`, `REGISTER_RATE_LIMIT_DURATION_SECONDS`
- `Gateway-Usage-Flow.md` / a new `Webhooks.md`: document the `addressing_mode` field; add the §5 token-lifetime note (short-lived tokens recommended)
- **Fix (flagged by the gap audit):** the webhook-verification example must use header `X-Webhook-Signature` (not `X-Signature`) with the `sha256=` prefix, signing the **raw** JSON body
- When #1 ships: a full **webhook event catalog** section

### 8.3 `CLAUDE.md` (gateway)
- Update the **API Endpoints** section with the four new endpoints
- Add the new config knobs to the Configuration section

### 8.4 SDK `README.md` + `CHANGELOG.md`
- New methods (§6.1), reaction field, the unified `ParseWebhook` dispatcher + examples
- When #1 ships: the event catalog with Go examples per event

### 8.5 MCP `README.md`
- Document the new tools (§7.1)

---

## 9. Sequencing recommendation

> **Superseded for the automation work (2026-07-14).** A scoping/grilling session added a third, larger initiative — the **automation platform** — and reprioritized: automation-platform first, **then** #1, **then** #2. See §10 and [`docs/plan/`](./plan/README.md) for the full sequence. The order below still holds *within* the #1/#2 window.


1. **Docs update for v0.11.0** (§8) — do now while changes are fresh. *Uncommitted draft pending.*
2. **SDK + MCP additions for v0.11.0 endpoints** (§6.1, §7.1) — so all four surfaces stay in sync before starting #1.
3. **Initiative #1 (webhook events)** — highest value; includes the unified SDK dispatcher (§6.3), the receipt→job linkage, and the small inbound-parse gaps (reactions, contacts-array, live-location, reply/quoted context). Ship together.
4. **Initiative #2 (pacing)** — builds on #1's `TemporaryBan` signal.
5. **Decline #3** — keep the doc note only.

---

## 10. Automation-Platform Initiative (scoped 2026-07-14)

A grilling + edge-case-mining session scoped a third, larger initiative: turn the gateway from a "notification sender + validator" into a **full WhatsApp automation platform** — the read/query surface (contacts, groups, profiles, avatars), two-way primitives (read receipts, typing, replies, mentions), group + community management, and ops/gateway-grade hardening (admin session inventory, Prometheus metrics, send idempotency, graceful shutdown, health/readiness split, webhook retry parity).

**Full detail + progress tracking lives in [`docs/plan/`](./plan/README.md)** — 7 PRDs (local, uncommitted review drafts), a development sequence, a docs-surface matrix, and 3 ADRs. Domain glossary: [`CONTEXT.md`](../CONTEXT.md).

### The 9 decisions taken
1. **Vision** — full automation platform, delivered *sequenced* (not all at once).
2. **Addressing** — unified `chat` recipient (msisdn | PN JID | `@g.us` | `@lid`); `msisdn` a permanent back-compat alias. → [ADR 0001](./adr/0001-chat-canonical-msisdn-alias.md).
3. **Queue/limits** — reads direct+cached (+ per-account query budget); conversation actions paced through the send path.
4. **Admin plane** — `ADMIN_API_SECRET` bearer; `/admin/*` + `/metrics` 404 when unset.
5. **Replies** — lightweight, caller-supplied context; durable message store deferred.
6. **Contacts** — expose local synced store as-is + honest metadata; force-resync deferred.
7. **Sequencing** — automation-platform first, **then** #1, **then** #2. Ban-risky bulk ops (participant-add, mass-join) gated until #2 pacing; conversation actions use an interim per-account cap. **New webhook event types deferred to #1's envelope window.**
8. **SDK/MCP** — SDK full parity (admin separate); MCP curated safe subset (no group mutations, no admin).
9. **Deliverable** — local PRDs + tracker, uncommitted.

### Ties into #1 and #2
- The ops "session-status event tracking" (Phase D) adds the `events.LoggedOut/TemporaryBan/ConnectFailure/StreamReplaced` cases to `HandleEvent` — **this is the code #1 §3.4 needs.** #1 later only adds webhook *emission* on top; one owner for the switch.
- The interim per-account action cap and the bulk-op gates are **replaced/lifted by #2 pacing** when it lands.
- Multi-URL / per-event webhook subscriptions (ops feature 8) overlap #1's generic envelope and are **deferred into the #1 window** to avoid a double schema migration.

---

*Last updated: 2026-07-14 (automation-platform initiative scoped; see §10 + `docs/plan/`). Living document — update as initiatives land.*
