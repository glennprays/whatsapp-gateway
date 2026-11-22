# PRD 3 — Group & Community Management

> **Status:** Not started  ·  **Sequence position:** 5 (reads) & 7 (mutations, gated)  ·  **Ban-risk:** high (bulk-add/mass-join gated)
> **Depends on:** PRD 0 (addressing foundation), PRD 1 (read/query infra)   ·   **Effort rollup:** L
> _Review draft — uncommitted. Not a commitment. Part of the WhatsApp Gateway automation-platform plan; see `docs/plan/README.md`._

## Progress
Status legend: ☐ Not started · ◐ In progress · ☑ Shipped · ⏸ Gated/deferred

| # | Deliverable (endpoint / change) | whatsmeow call | Effort | Ban-risk | Status |
|---|---|---|---|---|---|
| 1 | `POST /group` — create group / community | `CreateGroup(ctx, ReqCreateGroup{...})` | M | high (add-on-create gated) | ☐ |
| 2 | `GET /group/:chat` — group info (admin-status source) | `GetGroupInfo(ctx, jid)` | S | none (read) | ☐ |
| 3 | `GET /group` — list joined groups | `GetJoinedGroups(ctx)` | S | none (read) | ☐ |
| 4 | `POST /group/:chat/leave` — leave group | `LeaveGroup(ctx, jid)` | S | none | ☐ |
| 5 | `POST /group/:chat/participants` — add/remove/promote/demote | `UpdateGroupParticipants(ctx, jid, changes, action)` | M | high (add gated) | ☐ |
| 6 | `PATCH /group/:chat/name` — set subject | `SetGroupName(ctx, jid, name)` | S | low | ☐ |
| 7 | `PATCH /group/:chat/topic` — set topic/description | `SetGroupTopic(ctx, jid, "", "", topic)` | S | low | ☐ |
| 8 | `PUT /group/:chat/photo` (DELETE removes) — set/remove photo | `SetGroupPhoto(ctx, jid, jpeg\|nil)` | M | low | ☐ |
| 9 | `PATCH /group/:chat/settings` — announce/locked/join-approval/member-add-mode | `SetGroupAnnounce / SetGroupLocked / SetGroupJoinApprovalMode / SetGroupMemberAddMode` | M | low | ☐ |
| 10 | `GET /group/:chat/invite` — get invite link | `GetGroupInviteLink(ctx, jid, false)` | S | low | ☐ |
| 11 | `POST /group/:chat/invite/reset` — revoke/regenerate link | `GetGroupInviteLink(ctx, jid, true)` | S | low | ☐ |
| 12 | `POST /group/join` — join via invite link | `JoinGroupWithLink(ctx, code)` | S | high (mass-join gated) | ☐ |
| 13 | `GET /group/invite/info?code=` — preview from link (no join) | `GetGroupInfoFromLink(ctx, code)` | S | none (read) | ☐ |
| 14 | `GET /group/:chat/requests` — list pending join requests | `GetGroupRequestParticipants(ctx, jid)` | S | none (read) | ☐ |
| 15 | `POST /group/:chat/requests` — approve/reject join requests | `UpdateGroupRequestParticipants(ctx, jid, jids, action)` | S | low | ☐ |
| 16 | `POST /community/:chat/subgroups` — link subgroup | `LinkGroup(ctx, parent, child)` | S | low | ☐ |
| 17 | `DELETE /community/:chat/subgroups/:child` — unlink subgroup | `UnlinkGroup(ctx, parent, child)` | S | low | ☐ |
| 18 | `GET /community/:chat/subgroups` — list subgroups | `GetSubGroups(ctx, community)` | S | none (read) | ☐ |
| 19 | `GET /community/:chat/participants` — list all community participants | `GetLinkedGroupsParticipants(ctx, community)` | S | none (read) | ☐ |

## Problem Statement

A backend integrator building automation on top of the gateway can send messages to a group only if the group already exists and the account already belongs to it. Everything upstream of that — creating groups and communities, managing membership, moderating join requests, editing subject/topic/photo/settings, and reading who is in a group — is invisible. To do any of it, the integrator would have to embed whatsmeow directly, exactly the coupling this gateway exists to remove. There is also no safe way to expose group operations: bulk participant-adds and mass-joins via invite link are the two highest ban-risk actions on WhatsApp, and today nothing gates them.

## Solution

Expose the full whatsmeow group + community surface as thin, JWT-scoped REST endpoints under `/group` and `/community`, all resolving the group through the unified **Chat** addressing model (PRD 0) but requiring an explicit `@g.us` JID so a bare number can never silently target a user. Reads (info, joined, subgroups, community participants) reuse PRD 1's short-TTL cache + per-account query budget. Mutations run through the shared send-path pacing hook. The two ban vectors — bulk add-participants (and add-on-create) and join-via-link at scale — are hard-gated behind default-off enable-flags with a per-request participant cap, so operators can ship the binary with group management live but the ban-risky paths dark until outbound pacing (roadmap #2) lands. Per-participant operations return partial-success arrays (`ok`/`invited`/`failed`), never failing the whole batch for one bad member. Group-change webhook event *types* are deferred to the Initiative #1 envelope window; this PRD ships request/response only.

## User Stories

1. As a backend integrator, I want to create a group with an initial participant list, so that my product can spin up conversations programmatically.
2. As a backend integrator, I want to create a community (parent group) and create/link subgroups under it, so that I can model multi-channel spaces.
3. As a backend integrator, I want create to return per-participant results, so that I know which invitees became members, which were only invited, and which failed — without the create call failing outright.
4. As a backend integrator, I want a group created with zero reachable participants to still succeed (just my account), so that provisioning is not blocked by unreachable numbers.
5. As a support-agent backend, I want to read a group's info including my account's admin status, so that I can decide whether an agent may perform admin actions before attempting them.
6. As a backend integrator, I want to list the groups my account has joined, so that I can reconcile local state with WhatsApp.
7. As a backend integrator, I want to leave a group even as a non-admin, so that an account can exit a conversation it no longer needs.
8. As a support-agent backend, I want to add, remove, promote, and demote participants in one endpoint keyed by an `action`, so that moderation is a single call shape.
9. As a support-agent backend, I want adding a privacy-blocked or non-contact number to return that participant as `invited` with the invite code, so that I can surface "invitation sent" instead of a hard failure.
10. As a backend integrator, I want the gateway to reject my own account JID in a remove/promote/demote request with a pointer to `/leave`, so that I never accidentally evict myself through the participants endpoint.
11. As an operator, I want bulk participant-adds gated behind a default-off flag and a per-request cap, so that my accounts are not banned by an over-eager automation before pacing exists.
12. As a backend integrator, I want to set a group's name and get a clear 400 when it exceeds 25 characters, so that I fail fast on the client instead of decoding a server 406.
13. As a backend integrator, I want to set a group's topic/description without tracking the previous topic ID, so that the gateway handles the whatsmeow bookkeeping.
14. As a backend integrator, I want to set or remove a group photo by uploading a JPEG, so that I can brand a group; and I want a clear 400 if I upload a PNG/WebP/GIF or exceed the size cap.
15. As a support-agent backend, I want to toggle group settings (announce-only, locked, join-approval, member-add-mode) in one call supplying only the flags I want to change, so that moderation policy is scriptable.
16. As a backend integrator, I want to fetch a group's invite link, and get a 403 (not 401) when my account lacks the admin role, so that an authenticated caller understands it is a group-permission issue.
17. As a backend integrator, I want to revoke/reset an invite link, so that I can rotate access.
18. As a backend integrator, I want to join a group via an invite link and have it be idempotent (already-member returns 200 with an `already_member` flag), so that retries are safe.
19. As an operator, I want mass-join via invite link gated like bulk-add, so that this comparably ban-risky vector is not open by default.
20. As a backend integrator, I want to preview a group from an invite link without joining, so that I can show the user what they are about to join.
21. As a backend integrator, I want a revoked invite link to return 410 Gone and a malformed one to return 400, so that I can tell "get a fresh link" apart from "fix the input".
22. As a support-agent backend, I want to list pending join requests, so that a moderator UI can show who is waiting.
23. As a support-agent backend, I want to approve or reject join requests with per-participant results, so that batch moderation reports each outcome.
24. As a backend integrator, I want to link and unlink subgroups into a community, and get a 403 when I lack admin on the parent or child, so that community structure is manageable.
25. As a backend integrator, I want to list a community's subgroups and all its participants, so that I can render the full community membership.
26. As an AI assistant (MCP), I want group **reads** (list groups, group info) exposed, but I explicitly do NOT get group mutations, so that an agent cannot restructure or spam groups (decision #8).
27. As any caller, I want a group addressed by a bare number to be rejected with a clear 400, so that a group op never silently lands on a user JID.
28. As any caller, I want empty or duplicate participant lists rejected/deduped, so that results map 1:1 to what I sent.
29. As an operator, I want the whole `/group` + `/community` surface behind a master toggle that returns 404 when off, so that I can ship the binary with group ops dark.

## Implementation Decisions

**Addressing.** All `:chat` params and body `chat` fields resolve through the unified resolver (PRD 0) but pass through a **`validateGroupJID`** that requires an explicit `@g.us` server. Bare digits (which the base resolver normalizes to `@s.whatsapp.net`) are rejected for group endpoints — never auto-retargeted to a user. Lowercase the server before any allowlist check.

**Reads vs mutations (decision #3).** The four server-hitting reads (group info, joined groups, subgroups, community participants) go through PRD 1's short-TTL in-memory cache keyed by account+JID (`GROUP_INFO_CACHE_TTL_SECONDS`) and consume the per-account query budget only on a cache miss. Mutations (create, participants, name/topic/photo/settings, link/unlink, join, request-actions) route through the shared send-path pacing hook.

**Ban gating (decision #7).** Bulk add (via `POST .../participants action=add` and add-on-create) is gated behind `GROUP_ADD_PARTICIPANTS_ENABLED` (default false) and capped at `GROUP_MAX_PARTICIPANTS_PER_REQUEST` (400 over the cap, before the server call). Join-via-link (`POST /group/join`) is the comparably risky mass-join vector and is gated too (see Decisions for Review for which flag). Remove/promote/demote/settings/links stay enabled.

**Per-participant results.** Create, participants, and request-actions return a `results[]` of the canonical `ParticipantResult{jid, lid?, status(ok|invited|failed), code?, invite{code, expires_at}?}`. A participant converted to an invite (privacy-blocked/non-contact) is `status:"invited"` with the `AddRequest` payload; a non-member promote/demote is `status:"failed"` with the code. Mixed outcome is **200 partial success, not an error**.

**Error mapping.** Add a `mapIQError` helper: `errors.As(err, *whatsmeow.IQError)` → domain error keyed by `.Code`, alongside the existing `mapWhatsmeowErr`/`ErrDeviceDeleted` handling and whatsmeow's typed invite-link sentinels. Per the error-mapping ADR, map via `errors.Is/As` sentinels and `IQError.Code`, **not** substring matching.

### Request/response contract sketches

- **Create** (`POST /group`) req: `{name, participants[], is_community?, linked_parent_jid?, is_announce?, is_locked?, is_join_approval_required?}` → 201 `{group_jid, group_info, results[]}`.
- **Participants** (`POST /group/:chat/participants`) req: `{action: add|remove|promote|demote, participants[]}` → 200 `{group_jid, action, results[]}`.
- **Settings** (`PATCH /group/:chat/settings`) req: any subset of `{announce?, locked?, join_approval?, member_add_mode?}` → 200 `{group_info}`.
- **Photo** (`PUT /group/:chat/photo`): multipart JPEG; `DELETE` removes → 200 `{picture_id}` / `{removed:true}`.
- **Join** (`POST /group/join`) req: `{code}` → 200 `{group_jid, already_member?, membership_approval_request?}`.
- **Invite preview** (`GET /group/invite/info?code=`) → 200 `{group_info}` / 410 revoked / 400 invalid.
- **Requests actions** (`POST /group/:chat/requests`) req: `{action: approve|reject, participants[]}` → 200 `{results[]}`.
- **Community link** (`POST /community/:chat/subgroups`) req: `{child_jid}` → 200.

### Config additions

| Key | Purpose | Default |
|---|---|---|
| `GROUP_MANAGEMENT_ENABLED` | Master toggle for the whole `/group` + `/community` surface; endpoints 404 when false so operators can ship the binary but keep group ops dark | `true` |
| `GROUP_ADD_PARTICIPANTS_ENABLED` | Gate the ban-risky bulk add-participant path (and add-on-create) until outbound pacing lands (decision #7); remove/promote/demote/settings/links stay enabled | `false` |
| `GROUP_MAX_PARTICIPANTS_PER_REQUEST` | Hard cap on participants accepted in one add/create/promote/demote request; 400 over the cap to bound ban blast radius and stay under WhatsApp's group size limit | `256` |
| `GROUP_INFO_CACHE_TTL_SECONDS` | Short-TTL cache for server-hitting group reads (info, joined, subgroups, community participants) so admin-status checks and repeated reads don't burn the query budget (decision #3) | `30` |

### Integration seams

| File | What |
|---|---|
| `domain/whatsapp/group.go` (new) | Request/response DTOs: `CreateGroupRequest`, `GroupParticipantsRequest{action,participants[]}`, `GroupSettingsRequest`, `GroupPhoto` (multipart), `JoinRequest`, `JoinRequestsActionRequest`, `CommunityLinkRequest`; canonical `ParticipantResult{jid,lid?,status,code?,invite{code,expires_at}?}` and `GroupInfoResponse` mapped from `types.GroupInfo`. |
| `internal/whatsapp/client.go` | ~19 group methods on the Client interface + impl as thin wrappers over `cli.CreateGroup/UpdateGroupParticipants/etc.`, each doing `ParseJID`, `IsLoggedIn` guard, and the new `mapIQError` helper. |
| `internal/whatsapp/manager.go` | Mirror the same methods on the Manager interface as logging pass-throughs (existing per-method style). |
| `internal/whatsapp/helper.go` | `validateGroupJID` (require `@g.us`) + `normalizeParticipant` (reuse `validateRecipient` digit-normalization) + `types.GroupParticipant.Error/AddRequest` → `ParticipantResult.status` mapping. |
| `internal/usecase/whatsapp/group_usecase.go` (new) | Validation, `GROUP_MAX_PARTICIPANTS_PER_REQUEST` cap, `GROUP_ADD_PARTICIPANTS_ENABLED` gating, self-JID guard, per-participant result assembly, short-TTL read cache (`GROUP_INFO_CACHE_TTL_SECONDS`), shared send-path pacing hook. |
| `internal/usecase/whatsapp/provider.go` + `internal/handler/whatsapp/provider.go` | `ProvideWhatsappGroupUsecase` / `ProvideWhatsappGroupHandler` following the existing message provider signatures. |
| `internal/handler/whatsapp/whatsapp_group_handler.go` (new) + `internal/handler/handler.go` | HTTP handlers (`BodyParser`, `GetTraceID`, `MustGetPhoneNumber`, `httperror.FromError`) + new `WhatsappGroupHandler` field on the Handler struct and `NewHandler` signature. |
| `internal/router/group_routes.go` (new) + `internal/router/router.go` | `initGroupRoutes` registering `/group` and `/community` behind `authMw.JWTAuthentication()`; called from `SetupRouter` alongside `initMessageRoutes`. |
| `internal/infrastructure/wire.go` (+ `wire_gen.go`) | Add the new usecase/handler providers to `wire.Build`; run `wire gen ./internal/infrastructure`. |
| `config/config.go` | Add the `GROUP_*` knobs (management toggle, add-gate, per-request cap, read cache TTL). |
| `domain/error/error.go` + `internal/httperror/httperror.go` | Optionally add `ErrGone` → 410 for revoked invite links; otherwise reuse 400/409. |

## Edge Cases

| Scenario | Risk | Handling |
|---|---|---|
| Non-admin calls add/remove/promote/demote, set-name/topic/photo, or a settings toggle | confusing-UX | Server returns IQ 403/401 for the whole request. `errors.As(*whatsmeow.IQError)` → 403 "account is not an admin of this group". Reads and `LeaveGroup` stay allowed. Optional fast-fail from cached `GetGroupInfo.IsAdmin`, but mapping the server error is the lazy correct path. |
| Add a privacy-blocked or non-contact participant | confusing-UX | whatsmeow returns that participant with `Error=403` + `AddRequest{Code,Expiration}`. Surface as `status:"invited"` with the invite payload; participant is NOT yet a member. Overall 200 (partial success). |
| Promote/demote a JID not in the group | correctness | Per-participant `Error` (404 item-not-found / 400). Map to `status:"failed"` with the code; overall 200. Never fail the whole batch for one bad member. |
| Caller lists own account JID in remove/promote/demote | confusing-UX | Resolve the account's own PN+LID and reject that entry with 400 "use POST /group/:chat/leave to remove yourself" (or 400 for self promote/demote). Prevents accidental self-eviction. |
| Add/create exceeds WhatsApp's group size limit (~1024) or is a huge bulk list | ban | Enforce `GROUP_MAX_PARTICIPANTS_PER_REQUEST` in usecase → 400 before the server. Server overflow → IQ 419 resource-limit → 429. Bulk add is the primary ban vector: gate behind `GROUP_ADD_PARTICIPANTS_ENABLED` and route through the send-path pacer. |
| Request invite link while not admin | confusing-UX | `ErrGroupInviteLinkUnauthorized` (wraps IQ 401) → 403 "not authorized to view this group's invite link" (403 not 401 — caller IS authenticated; this is a group-role permission). |
| Join via link a group already joined | confusing-UX | `JoinGroupWithLink` is effectively idempotent — returns the group JID (or a membership-approval-request JID). Return 200; on server conflict surface 200 with `already_member` flag, not an error. |
| Join/preview a revoked vs malformed/expired link | confusing-UX | `ErrInviteLinkRevoked` (IQ 410) → 410 "invite link revoked" (add `ErrGone`); `ErrInviteLinkInvalid` (IQ 406) → 400 "invite link is not valid". Keep distinct so callers can retry-with-new-link vs fix-input. |
| SetGroupPhoto given non-JPEG (PNG/WebP/GIF) | correctness | Pre-validate detected MIME == `image/jpeg` (mirror `mediaMimeAllowlists`) → 400 before upload. Also handle server `ErrInvalidImageFormat` (IQ 406) → 400 "group photo must be a JPEG". Enforce `MaxUploadBytes`. |
| Link/unlink subgroup without admin on parent and/or child | confusing-UX | `LinkGroup`/`UnlinkGroup` return raw IQ 403 → 403. Linking a group already owned by another community, or a group that is itself a parent, errors server-side (IQ 400/409) → map 400/409 with server text. |
| Create with some/all participants unreachable/non-existent/blocking | correctness | `CreateGroup` still succeeds; `Participants[]` carry per-member `Error`/`AddRequest`. Return 201 with the created group AND the same per-participant results. Zero valid members (just the account) is still created — not an error. |
| Group addressed by bare numeric ID instead of full `@g.us` JID | correctness | `validateRecipient` normalizes bare digits to `@s.whatsapp.net` (WRONG for a group). `validateGroupJID` requires explicit `@g.us` → 400 otherwise. Never let a group op silently target a user JID. |
| Empty or duplicate participant list | confusing-UX | 400 on empty; dedupe repeated JIDs before sending so results map 1:1. |
| Group name > 25 chars or over-long topic | confusing-UX | Pre-validate name ≤25 (WhatsApp hard limit → IQ 406) and topic length → 400 with an explicit limit message rather than a confusing server 406. |

## Error Semantics

| Condition | Response |
|---|---|
| Chat/group JID missing or not a valid `@g.us` JID | 400 `{message: "invalid group JID"}` (`validateGroupJID` before any server call) |
| No WhatsApp session for the account (client not in ClientStore) | 404 `{message: "no active WhatsApp session for this number; link via QR or pair code"}` |
| Session exists but not logged in | 401 `{message: "client not logged in"}` |
| Not a group/community admin for a mutation (IQ 403/401) | 403 `{message: "account is not an admin of this group"}` |
| Group does not exist (`ErrGroupNotFound` / IQ 404) | 404 `{message: "that group does not exist"}` |
| Not a participant of the group (`ErrNotInGroup` / IQ 403 on info) | 403 `{message: "you're not participating in that group"}` |
| Invite link revoked (`ErrInviteLinkRevoked` / IQ 410) | 410 `{message: "invite link revoked"}` (add `ErrGone`; or 409 if not adding 410) |
| Invite link malformed/invalid (`ErrInviteLinkInvalid` / IQ 406) | 400 `{message: "invite link is not valid"}` |
| Group photo not JPEG (pre-check or `ErrInvalidImageFormat` / IQ 406) | 400 `{message: "group photo must be a JPEG"}` |
| Participant count over cap, or name > 25 chars | 400 `{message: <explicit limit>}` |
| Bulk add path while `GROUP_ADD_PARTICIPANTS_ENABLED=false` | 403 `{message: "adding participants is disabled"}` (or 404 if hidden entirely) |
| Participant op with mixed outcome (some invited/failed) | 200 `{group_jid, action, results:[{jid,status,code?,invite?}]}` — partial failure is NOT an error |
| Server resource/rate limit (IQ 419 / 429) or local send-pace budget exhausted | 429 `{message: "rate limited"}` (becomes a paced block, not a reject, once #2 pacing lands) |

## Testing Decisions

Highest-seam-first; whatsmeow group/IQ calls are **not** CI-integration-testable (no live WhatsApp).

- **`validateGroupJID` + `normalizeParticipant` (helper.go)** — table-driven unit tests: require `@g.us`; reject bare digits, user JID, device/agent suffix; lowercase server; dedupe + empty-list rejection. No deps.
- **Self-JID guard + cap gating (usecase)** — unit tests that a request containing the account's own PN/LID is rejected 400; that `> GROUP_MAX_PARTICIPANTS_PER_REQUEST` rejects 400; that `action=add` (and add-on-create) rejects when `GROUP_ADD_PARTICIPANTS_ENABLED=false`, and that mass-join rejects when its gate is off. Ban-safety has no live surface, so these limiter-wiring unit tests are the only guardrail.
- **Per-participant result mapping (`mapIQError` + `GroupParticipant.Error/AddRequest` → `ParticipantResult`)** — construct whatsmeow `IQError` and `AddRequest` values and assert `status`/`code`/`invite` and the mixed-outcome 200. Possible only because error mapping uses `errors.Is/As` sentinels + `IQError.Code` (per the ADR), not substring matching.
- **TTL read-cache** — reuse PRD 1's cache tests: hit bypasses budget, miss consumes it, expiry re-fetches.
- **"Verify once against live whatsmeow, then lock to an exported sentinel"** — the exact IQ codes/sentinels for: non-admin mutation (403/401), group-not-found (404), not-a-member (403), invite revoked (410) vs invalid (406), non-JPEG photo (406), resource-limit (419). Verify each once in a manual harness, then pin to the exported sentinel so CI locks it. Do **not** ship substring guesses.

## Decisions for Review

- [ ] **Group addressing** — Require an explicit `@g.us` JID for all group endpoints (via `validateGroupJID`); do **not** auto-append `@g.us` to a bare 18-digit id. *Recommended:* require explicit `@g.us`. The unified resolver defaults bare digits to a user JID, and a silently-mistargeted group op is worse than a 400.
- [ ] **Read budget source** — Do server-hitting group reads share the message send limiter, or use PRD 1's dedicated query budget? *Recommended:* reuse PRD 1's query budget + short-TTL cache; interim cache-only with no separate limiter until PRD 1 lands. (Depends on PRD 1.)
- [ ] **Create endpoint shape** — Fold create-community and create-group-inside-community into `POST /group` via `is_community` / `linked_parent_jid` flags, or split into dedicated endpoints? *Recommended:* fold into one `POST /group`.
- [ ] **`ErrGone` → 410** — Add `ErrGone` → 410 for revoked invite links, or reuse 400/409? *Recommended:* add `ErrGone` → 410; the retry-with-new-link vs fix-input distinction is worth the small `httperror` addition.
- [ ] **Self-removal** — Block self in the participants endpoint with a 400 redirect to `/leave`, or silently alias remove-self to `LeaveGroup`? *Recommended:* block with 400 redirect — explicit, avoids surprising self-eviction.
- [ ] **Admin pre-check** — Pre-check admin status from a cached `GetGroupInfo` before mutations, or rely purely on mapping the server 403? *Recommended:* rely on mapping the server 403 (one fewer round-trip); fast-fail from cache only when the info is already cached.
- [ ] **Group-change webhooks deferred** — Confirm `events.GroupInfo`/`JoinedGroup` (added/removed, name/settings changed) stay deferred to the Initiative #1 envelope per decision #7. *Recommended:* yes — ship request/response only, no new webhook shapes here.
- [ ] **`member_add_mode=all_member_add`** — When the account is a non-admin member and the group allows member adds, allow the add (server permits) or restrict adds to admins regardless? *Recommended:* allow what the server permits; still subject to `GROUP_ADD_PARTICIPANTS_ENABLED`.
- [ ] **Participant identity in results/community lists** — Return PN only, LID only, or both? *Recommended:* return PN as primary + LID when present (both), so callers can correlate; document the linkage-leak note.
- [ ] **Disappearing-message timer + CreateKey dedup on create** — Expose `GroupEphemeral` and `CreateKey` on create now, or defer? *Recommended:* defer both to keep the create payload lean; add later.
- [ ] **Mass-join gate (from critique)** — Join-via-link is a comparably ban-risky mass-join vector and must be gated too. *Recommended:* gate `POST /group/join` behind a dedicated `GROUP_JOIN_VIA_LINK_ENABLED` (default false) rather than overloading `GROUP_ADD_PARTICIPANTS_ENABLED`, since join and add are genuinely different actions callers may want independently. **Approving this adds a 5th env key** to Doc Surfaces.

## Doc Surfaces

- [ ] `docs/openapi.yaml` — hand-author the ~19 new `/group` + `/community` endpoints + schemas (`ParticipantResult`, `GroupInfoResponse`, request bodies); add 410/429 responses (`openapi.go` is only the loader/server-URL injector).
- [ ] `llms.txt` — add the group/community endpoint family + the "group JID required (`@g.us`)" addressing note.
- [ ] `wiki/Environment-Variables.md` + `.env.example` + `CLAUDE.md` Configuration list — new env keys: `GROUP_MANAGEMENT_ENABLED`, `GROUP_ADD_PARTICIPANTS_ENABLED`, `GROUP_MAX_PARTICIPANTS_PER_REQUEST`, `GROUP_INFO_CACHE_TTL_SECONDS` (+ `GROUP_JOIN_VIA_LINK_ENABLED` if the mass-join-gate decision is approved).
- [ ] `wiki/Security-Considerations.md` — document the bulk-add + mass-join ban gates and their default-off posture.
- [ ] New `wiki/Group-Management.md` usage page (create/manage/moderate + community linking).
- [ ] `CLAUDE.md` "API Endpoints" list — enumerate the new `/group` + `/community` routes.
- [ ] `docs/ui/assets/docs/*` — group/community reference page(s).
- [ ] `CHANGELOG.md` + openapi.yaml version bump.
- [ ] SDK (`whatsapp-gateway-sdk-go`) — full parity: add the ~19 group/community methods with `chat` addressing (decision #8).
- [ ] MCP (`mcp-whatsapp-gateway`) — curated subset: expose group **reads** only (list groups, group info); **exclude** all group/community mutations (decision #8).

## Out of Scope

- **Group-change webhook event types** (`message`-style `group.*` / added-removed / settings-changed) — deferred to the Initiative #1 envelope window; this PRD ships request/response only.
- **Outbound pacing** for group mutations — the real pacer is roadmap #2; the interim is the crude shared send-path cap + the enable-flags. Bulk-add and mass-join stay hard-gated until #2 lands.
- **Durable message store** — no group-message history/backfill here (separate deferred initiative).
- **Force group-metadata resync / presence keep-alive** — not part of this PRD.

## Dependencies & Sequencing

- **Before (blocking):**
  - **PRD 0 (addressing foundation)** — `resolveChat` + the group-JID resolver (`validateGroupJID`) + participant JID normalization must exist first.
  - **PRD 1 (read/query infra)** — the short-TTL cache + per-account query budget that group reads reuse.
- **Sequencing within this PRD (per the dependency order):**
  - **Position 5 — group READS** (info, joined, subgroups, community participants): reuse PRD 1's cache+budget; low ban-risk; ship with or just after PRD 1.
  - **Position 7 — group/community MUTATIONS**: depend on the PRD 0 group-JID resolver + the reads; bulk-add and mass-join gated behind enable-flags until roadmap #2 pacing.
- **After:** Initiative #1 (webhook events) adds `group.*` event emission on top of these endpoints; Initiative #2 (pacing) replaces the interim send-path cap with the real pacer.
