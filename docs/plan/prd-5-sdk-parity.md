# PRD 5 — Go SDK Full Parity (`whatsapp-gateway-sdk-go`)

> **Status:** Not started  ·  **Sequence position:** follows each gateway wave (SDK trails the endpoint that ships it)  ·  **Ban-risk:** n/a
> **Depends on:** PRD 0–4 (mirrors their endpoints)  ·  **Effort rollup:** L (spread across waves)
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | SDK change | Mirrors | Wave | Status |
|---|---|---|---|---|
| 1 | Add `Chat` to every send/action request; keep `Msisdn` alias; add resolved `chat` to responses | PRD 0 | 1 | ☐ |
| 2 | `React` gains optional `SenderMsisdn` (fold in the pending §6.2 reaction change — one release) | existing + PRD 0 | 1 | ☐ |
| 3 | `SendAudio`, `SendVideo`, `SendDocument`, `CheckContact` (v0.11 backfill) | v0.11 | 1 | ☐ |
| 4 | `Idempotency-Key` support on send methods | PRD 4 | 1 | ☐ |
| 5 | `ListContacts`, `GetContactInfo`, `GetAvatar`, `ListGroups`, `GetGroupInfo` | PRD 1 | 3 | ☐ |
| 6 | `MarkRead`, `SendChatPresence` (typing), `reply_to`/`mentions` on sends, optional global presence | PRD 2 | 4 | ☐ |
| 7 | Group mgmt: create/leave/participants/settings/name/topic/photo/invite(get,reset,join,preview)/requests | PRD 3 | 5–7 | ☐ |
| 8 | Community: link/unlink/subgroups/participants | PRD 3 | 7 | ☐ |
| 9 | Admin as a **separate opt-in module** (`admin` sub-client): `Sessions`, health/ready | PRD 4 | 6 | ☐ |
| 10 | Unified `ParseWebhook` dispatcher (prereq for roadmap #1; ship early, do not gate the above on it) | roadmap #1 §6.3 | any | ☐ |

## Problem Statement

Backend integrators using the Go SDK can only call the v0.11 surface (`SendText/Image/Location/Poll/Sticker`). As the gateway becomes an automation platform, every new endpoint that isn't mirrored forces those teams to hand-roll HTTP calls, defeating the SDK's purpose and drifting from the gateway contract.

## Solution

The SDK mirrors the **full tenant surface** with typed methods, in lockstep waves that trail each gateway release. Admin lives in a separate opt-in module so operator concerns don't leak into the tenant client. Backward compatibility is preserved: `Msisdn` keeps working; `Chat` is additive.

## User Stories

1. As a backend integrator, I want a `Chat` field on every send method, so that I can address groups and LID parties with the same typed call I use for phone numbers.
2. As a backend integrator, I want `Msisdn` to keep compiling, so that upgrading the SDK doesn't break my existing code.
3. As a backend integrator, I want the resolved canonical `chat` echoed on send responses, so that I can persist and reuse it without re-resolving.
4. As a backend integrator, I want `SendAudio/SendVideo/SendDocument/CheckContact`, so that I stop hand-rolling the v0.11 endpoints the SDK never caught up to.
5. As a backend integrator, I want to pass an `Idempotency-Key`, so that a retried send after a timeout doesn't double-send.
6. As a support-agent backend, I want `ListContacts/ListGroups/GetGroupInfo/GetContactInfo/GetAvatar`, so that I can resolve who I'm talking to before messaging.
7. As a support-agent backend, I want `MarkRead` and `SendChatPresence`, so that my agent UI shows blue ticks and typing like a human operator.
8. As a support-agent backend, I want `reply_to` and `mentions` on sends, so that I can thread replies and tag members.
9. As an automation integrator, I want typed group and community management methods, so that I can onboard and moderate groups programmatically.
10. As an operator, I want a separate admin sub-client for session inventory and health, so that operator tooling is clearly separated from tenant code and gated by the admin secret.
11. As an SDK consumer, I want a unified `ParseWebhook` dispatcher, so that I can handle new event types with a single switch once roadmap #1 lands.

## Implementation Decisions

- **Full tenant parity** (decision #8): every tenant endpoint from PRD 0–4 gets a typed method. Method names follow the existing `SendX`/verb-noun style.
- **`Chat` additive, `Msisdn` alias** (ADR 0001): request structs gain `Chat`; `Msisdn` stays; responses gain the resolved `chat`. One release absorbs the reaction `SenderMsisdn` change (§6.2) to avoid two churns.
- **Admin is a separate module** — `client.Admin` (or a distinct constructor taking `ADMIN_API_SECRET`), never mixed into the tenant client, so tenants can't accidentally call operator endpoints.
- **Waves, not a big bang** — the SDK change for a feature ships with (or immediately after) the gateway release that ships that feature. Each row's Wave column maps to the sequence in `docs/plan/README.md`.
- **`ParseWebhook`** (roadmap #1 §6.3) is a prerequisite for the deferred webhook-event work but is decoupled here — ship it whenever, don't block parity on it.

### Integration seams
- Request/response structs per method family (mirror the gateway domain DTOs).
- `React` request: add `SenderMsisdn` (coordinate with the already-pending §6.2 change so it's one release).
- Webhook parsing: add `ParseWebhook(body, sig) (WebhookEvent, error)` alongside the existing `ParseIncomingWebhook`/`ParseOutgoingWebhook` (non-breaking).

## Testing Decisions

- Highest seam: the SDK's HTTP client against a mock gateway server (table-driven request/response fixtures) — mirror existing SDK tests.
- Assert `Chat`/`Msisdn` precedence and that responses decode the new `chat` echo.
- Assert `ParseWebhook` dispatches the fixed catalog (`message.incoming/queued/sent/failed`) and errors on unknown events until #1 expands it.
- No live WhatsApp needed — the SDK tests the gateway contract, not whatsmeow.

## Decisions for Review
- [ ] **Admin surface shape** — a `client.Admin` sub-client on the same client vs a wholly separate `adminclient` package. Recommend: separate constructor taking the admin secret, exposed as `client.Admin` only when configured.
- [ ] **Ship `ParseWebhook` now or with roadmap #1** — Recommend now (non-breaking, unblocks consumers early).
- [ ] **One reaction release** — confirm folding the pending `SenderMsisdn` (§6.2) into Wave 1 rather than shipping it separately first.

## Doc Surfaces
- [ ] SDK `README.md` — new method families + `chat` addressing note + admin module.
- [ ] SDK `CHANGELOG.md` — per wave.
- [ ] Gateway `docs/ui/assets/docs/sdk/go.md` + `llms.txt` SDK section — reference the new methods.
- [ ] Tag alongside the gateway release that ships each wave.

## Out of Scope
- Non-Go SDKs.
- The webhook **event catalog** payloads (roadmap #1) — only the `ParseWebhook` plumbing is in scope here.
- Durable message store / history methods (deferred initiative).

## Dependencies & Sequencing
Trails the gateway: each SDK wave ships with or just after the gateway wave it mirrors. Wave 1 (addressing + idempotency + v0.11 backfill) can start as soon as PRD 0 + the idempotency slice of PRD 4 land.
