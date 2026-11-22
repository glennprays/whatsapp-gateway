---
status: proposed
---

# 0001 — `chat` is the canonical recipient; `msisdn` is a permanent back-compat alias

To reach groups (`@g.us`) and LID-only parties (`@lid`) the API needs a recipient that a bare phone number can't express, so we introduce a unified **`chat`** field (bare msisdn | PN JID | group JID | `@lid`) resolved by a single `resolveChat` chokepoint, and keep **`msisdn`** as a permanent alias that resolves to a PN JID (`chat` wins when both are present). The change is **additive within `/api/v1`** — no `/v2` — but it requires relaxing `required: [msisdn]` on the 7 request bodies in `docs/openapi.yaml` (add `chat`, mark `msisdn` deprecated, and state "at least one of `chat`/`msisdn`" in the description since OpenAPI can't express it).

## Considered options

- **New `/v2` with a clean `chat`-only contract** — rejected: forks the surface and forces every existing consumer to migrate for a change that is otherwise backward-compatible.
- **Separate `/message/group/*` endpoints / a `group_jid` field** — rejected: duplicates the entire send surface.
- **Overload `msisdn` to also carry a JID** — rejected: a field named `msisdn` holding a group JID is a lying contract for the SDK/MCP/docs.

## Consequences

- Permanent two-field API debt: `msisdn` can never be dropped without breaking existing consumers.
- The spec asserts a constraint it can't fully encode ("at least one of"); `resolveChat` is the real enforcer.
- Group endpoints must NOT reuse the bare-digit→user normalization (a bare id would silently target a user); they require an explicit `@g.us` via `validateGroupJID`.
- Send responses should echo the resolved canonical `chat` JID alongside `message_id` so callers can persist the canonical form (do it in the same wave to avoid a second SDK/openapi churn).
