# PRD 6 — MCP Curated Subset (`mcp-whatsapp-gateway`)

> **Status:** Not started  ·  **Sequence position:** trails PRD 1–2 (needs SDK reads + two-way)  ·  **Ban-risk:** low–medium (AI agents spam actions)
> **Depends on:** PRD 5 SDK (MCP consumes the SDK), PRD 1, PRD 2  ·  **Effort rollup:** M
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | MCP tool | Category | Exposed? | Status |
|---|---|---|---|---|
| 1 | `send_audio_message`, `send_video_message`, `send_document_message`, `check_contact` (v0.11 backfill) | messaging | ✅ yes | ☐ |
| 2 | `react_to_message`, `edit_message`, `delete_message` (pre-existing gateway gap) | messaging | ✅ yes | ☐ |
| 3 | `chat` addressing on all send tools (address groups too) | messaging | ✅ yes | ☐ |
| 4 | `list_contacts`, `get_contact_info`, `get_avatar` | read/query | ✅ yes | ☐ |
| 5 | `list_groups`, `get_group_info` | read/query | ✅ yes | ☐ |
| 6 | `mark_read`, `send_typing` (chat presence) | two-way | ✅ yes (⚠ interim-pace) | ☐ |
| 7 | `reply` / `mentions` args on send tools | two-way | ✅ yes | ☐ |
| 8 | Group **mutations** (create/leave/participants/settings/invite/community) | group mgmt | ❌ **excluded** | ⏸ |
| 9 | Admin (sessions/metrics) | operator | ❌ **excluded** | ⏸ |

## Problem Statement

An AI assistant using the MCP server can only send five message types. It can't look up the contacts or groups it needs to address, can't mark messages read or show typing, and can't send the newer media types — so it can't act as a competent WhatsApp agent. At the same time, exposing destructive group management or operator inspection to an autonomous model is dangerous.

## Solution

Expose a **curated, read-and-message-safe subset** (decision #8): all messaging (incl. the v0.11 media types, react/edit/delete, and `chat` addressing), the read/query surface, and the two-way primitives (mark-read, typing, reply, mentions). **Exclude** group-management mutations and the admin plane so an AI agent cannot make destructive group/community changes or inspect other tenants. Group-mutation tools can be added later behind explicit opt-in.

## User Stories

1. As an AI assistant, I want `list_contacts`/`list_groups`, so that I can discover who and where I can message.
2. As an AI assistant, I want `get_contact_info`/`get_group_info`/`get_avatar`, so that I can resolve a name or JID before acting.
3. As an AI assistant, I want a `chat` argument on send tools, so that I can message a group, not just a phone number.
4. As an AI assistant, I want `send_audio/video/document` and `check_contact`, so that I have the full v0.11 messaging surface.
5. As an AI assistant, I want `react_to_message/edit_message/delete_message`, so that I can manage messages I've sent or received.
6. As an AI assistant, I want `mark_read` and `send_typing`, so that conversations feel human.
7. As an AI assistant, I want `reply` and `mentions` on sends, so that I can thread and tag in group chats.
8. As a gateway operator, I want group mutations and admin endpoints **absent** from the MCP surface, so that an autonomous model can never remove members, dissolve a community, or read another tenant's sessions.

## Implementation Decisions

- **Curated subset only** (decision #8). Group **reads** are fair game (read-only); group **mutations** and admin are excluded from the tool manifest entirely (not merely permission-gated), so the model can't discover them.
- **MCP consumes the SDK** (roadmap §7.3) — so the SDK methods (PRD 5) must land first for each tool.
- **Interim-pace warning** (cross-cutting): `mark_read`/`send_typing` are conversation-affecting actions, and AI agents are the actors most likely to spam them. Until pacing (#2) lands they inherit the gateway's interim per-account action cap; document the ceiling in the tool descriptions.
- Tools mirror the gateway's `chat` addressing and error semantics; surface partial/soft failures (e.g. avatar-not-set 404) as tool results, not exceptions.

### Integration seams
- MCP tool definitions in the MCP repo's `internal/tools/messaging.go` (+ new `read.go`, `presence.go`).
- Each tool wraps the corresponding SDK method (PRD 5).

## Testing Decisions
- Highest seam: MCP tool handlers against a mock SDK/gateway (fixture-driven), mirroring existing MCP tool tests.
- Assert the tool manifest **excludes** group mutations + admin (a guard test that the exposed tool set matches the curated allowlist).
- No live WhatsApp needed.

## Decisions for Review
- [ ] **Which reads to expose** — proposal: `list_contacts`, `get_contact_info`, `get_avatar`, `list_groups`, `get_group_info` (all read-only). Confirm none of these leak more than the tenant already sees.
- [ ] **Later opt-in for group mutations** — keep fully excluded, or add behind an explicit server-config flag (default off) in a later release? Recommend: excluded now, revisit after pacing.
- [ ] **Interim-pace disclosure** — confirm documenting the best-effort action cap in `mark_read`/`send_typing` tool descriptions is sufficient until #2.

## Doc Surfaces
- [ ] MCP `README.md` + `docs/ui/assets/docs/mcp/tools-reference.md` — new tools; explicitly state the exclusions.
- [ ] `llms.txt` MCP section.
- [ ] MCP `CHANGELOG.md`.

## Out of Scope
- Group-management mutations, community flows, admin/session inventory, `/metrics` (excluded by decision #8).
- Anything requiring the SDK method to exist before its wave (blocked on PRD 5).

## Dependencies & Sequencing
Trails PRD 5 (SDK) which trails the gateway. Practically: MCP messaging + read/query tools land after the SDK Wave 1 + Wave 3; mark-read/typing after SDK Wave 4.
