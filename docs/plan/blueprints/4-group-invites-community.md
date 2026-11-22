I have everything needed. Here is the blueprint.

---

# Blueprint — Phase E: Group Invites + Community Mutations (gated)

## 0. Grounding notes that change the plan (read first)

- **The PRD's "new `WhatsappGroupUsecase` + `WhatsappGroupHandler` + Wire providers" is not the lazy path and contradicts the shipped code.** Group *reads* (`ListGroups`, `GetGroupInfo`) already live as methods on `WhatsappMessageUsecase` / `WhatsappMessageHandler`, routed in `internal/router/group_routes.go`. That struct already carries everything these endpoints need: `queryCache`, `queryBudget`, `limiter` (→ `spendActionBudget`), `config`, `whatsappManager`. **Recommendation: add methods to the existing usecase/handler (new files `group_usecase.go` / `whatsapp_group_handler.go` in the same packages), touch zero Wire files.** This matches the shipped precedent and is a much smaller diff. The rest of the blueprint assumes this. *(Want the PRD's separate-struct version? It's ~4 extra files + a `wire gen`; say so.)*
- **Routing style: follow the shipped flat `/group/info?chat=` convention, not the PRD's `/group/:chat/invite` path-param sketch.** The shipped group/contact endpoints all take `chat` as a query param (GET) or body field (POST). Mixing `:chat` segments in would be inconsistent. Blueprint uses flat routes.
- **No migration.** This feature group has zero DB tables. Section 4 is empty by design.
- **Cache TTL: reuse the existing `queryCache` (`READ_QUERY_CACHE_TTL_SECONDS`), do NOT add `GROUP_INFO_CACHE_TTL_SECONDS` or a second `ttlCache`.** ponytail: one cache already exists and is wired; a second config knob + cache instance for a 30s-vs-300s difference isn't worth it. Note left in §5.
- **Config keys the PRD lists but that belong to the *other* feature group (participants/create):** `GROUP_ADD_PARTICIPANTS_ENABLED`, `GROUP_MAX_PARTICIPANTS_PER_REQUEST`. Not added here.

---

## 1. Endpoints + DTOs

All under JWT auth (`authMw.JWTAuthentication()`). All gated by `GROUP_MANAGEMENT_ENABLED` (→ **404** when off, master toggle "hidden entirely").

| Method + Path | whatsmeow call | Budget/gate |
|---|---|---|
| `GET /group/invite?chat=` | `GetGroupInviteLink(reset=false)` | `spendActionBudget` (no cache — link can be reset out-of-band) |
| `POST /group/invite/reset` | `GetGroupInviteLink(reset=true)` | `spendActionBudget` |
| `GET /group/invite/info?code=` | `GetGroupInfoFromLink` | `queryWithBudget` (cache key by code) |
| `POST /group/join` | `JoinGroupWithLink` | **`GROUP_JOIN_VIA_LINK_ENABLED` gate (403 when off)** + `spendActionBudget` |
| `GET /group/requests?chat=` | `GetGroupRequestParticipants` | `queryWithBudget` (cache key by group JID) |
| `POST /group/requests` | `UpdateGroupRequestParticipants` | `spendActionBudget`, per-participant results |
| `POST /community/subgroups` | `LinkGroup` | `spendActionBudget` |
| `DELETE /community/subgroups?chat=&child=` | `UnlinkGroup` | `spendActionBudget` |

### DTOs (add to `domain/whatsapp/group.go`)

```go
// Invite link
type GroupInviteLinkResponse struct {
    Chat       string `json:"chat"`        // resolved @g.us JID
    InviteLink string `json:"invite_link"` // https://chat.whatsapp.com/<code>
}
type GroupInviteResetRequest struct {
    Chat   string `json:"chat"`             // @g.us (required)
    Msisdn string `json:"msisdn,omitempty"` // deprecated alias
}

// Join by link
type JoinGroupRequest struct {
    Code string `json:"code"` // full chat.whatsapp.com link OR bare code (required)
}
type JoinGroupResponse struct {
    GroupJID string `json:"group_jid"` // JID whatsmeow returned (group or approval-request)
}

// Invite preview -> reuse existing GroupInfoResponse

// Join requests (list)
type GroupJoinRequestItem struct {
    JID         string `json:"jid"`
    RequestedAt string `json:"requested_at,omitempty"` // RFC3339
}
type GroupJoinRequestsResponse struct {
    Chat     string                 `json:"chat"`
    Requests []GroupJoinRequestItem `json:"requests"`
    Count    int                    `json:"count"`
}

// Join requests (approve/reject)
type GroupJoinRequestsActionRequest struct {
    Chat         string   `json:"chat"`               // @g.us (required)
    Msisdn       string   `json:"msisdn,omitempty"`
    Action       string   `json:"action"`            // "approve" | "reject"
    Participants []string `json:"participants"`      // JIDs/numbers (required, non-empty)
}
type GroupJoinRequestsActionResponse struct {
    Chat    string              `json:"chat"`
    Action  string              `json:"action"`
    Results []ParticipantResult `json:"results"`
}

// Canonical per-participant result (shared with the participants feature group;
// this feature only sets JID/Status/Code — Invite is for add-participant).
type ParticipantResult struct {
    JID    string             `json:"jid"`
    LID    string             `json:"lid,omitempty"`
    Status string             `json:"status"`          // "ok" | "failed" | "invited"
    Code   int                `json:"code,omitempty"`  // IQ item code when failed
    Invite *ParticipantInvite `json:"invite,omitempty"`
}
type ParticipantInvite struct {
    Code      string `json:"code"`
    ExpiresAt string `json:"expires_at,omitempty"`
}

// Community
type CommunityLinkRequest struct {
    Chat     string `json:"chat"`      // parent community @g.us (required)
    Msisdn   string `json:"msisdn,omitempty"`
    ChildJID string `json:"child_jid"` // subgroup @g.us (required)
}
```

---

## 2. Exact whatsmeow signatures + sentinels + HTTP mapping

All from `go.mau.fi/whatsmeow@v0.0.0-20260516102357-8d3700152a69/group.go` unless noted. Sentinels from `errors.go`.

**`GetGroupInviteLink`** — `group.go:393`
```go
func (cli *Client) GetGroupInviteLink(ctx context.Context, jid types.JID, reset bool) (string, error)
```
Returns `InviteLinkPrefix + code` (const `InviteLinkPrefix` = `https://chat.whatsapp.com/`). Sentinels it emits (each wraps an IQError via `wrapIQError`, so both `errors.Is(sentinel)` and `errors.As(*IQError)` match):
- `ErrGroupInviteLinkUnauthorized` (`errors.go:84`, wraps IQ 401) → **403** ("not authorized to view this group's invite link" — caller IS authenticated).
- `ErrGroupNotFound` (`errors.go:88`, IQ 404) → **404**.
- `ErrNotInGroup` (`errors.go:86`, IQ 403) → **403**.

**`JoinGroupWithLink`** — `group.go:478`
```go
func (cli *Client) JoinGroupWithLink(ctx context.Context, code string) (types.JID, error)
```
Strips `InviteLinkPrefix` itself → pass caller's raw string. Returns the membership-approval-request JID if present, else the group JID (both `@g.us`; **whatsmeow does NOT expose which branch** — see §7 join corner). Sentinels:
- `ErrInviteLinkRevoked` (`errors.go:92`, IQ 410) → **410 ErrGone**.
- `ErrInviteLinkInvalid` (`errors.go:90`, IQ 406) → **400**.

**`GetGroupInfoFromLink`** — `group.go:457`
```go
func (cli *Client) GetGroupInfoFromLink(ctx context.Context, code string) (*types.GroupInfo, error)
```
Strips prefix itself. Same two sentinels: `ErrInviteLinkRevoked`→410, `ErrInviteLinkInvalid`→400. (Do NOT use `GetGroupInfoFromInvite` at `group.go:418` — that is for invite *messages*, not `chat.whatsapp.com` links.)

**`GetGroupRequestParticipants`** — `group.go:225`
```go
func (cli *Client) GetGroupRequestParticipants(ctx context.Context, jid types.JID) ([]types.GroupParticipantRequest, error)
```
`GroupParticipantRequest{JID, RequestedAt}` (`types/group.go:165`). No named sentinels — falls through to `mapIQError`: IQ 403→403 (not admin), 404→404.

**`UpdateGroupRequestParticipants`** — `group.go:255`
```go
func (cli *Client) UpdateGroupRequestParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action ParticipantRequestChange) ([]types.GroupParticipant, error)
```
Action consts (`group.go:250-251`, type `ParticipantRequestChange`):
```go
ParticipantChangeApprove ParticipantRequestChange = "approve"
ParticipantChangeReject  ParticipantRequestChange = "reject"
```
Returns `[]types.GroupParticipant` (`types/group.go:103`) — each carries `.JID`, `.LID`, `.Error int` (per-item IQ code), `.AddRequest *GroupParticipantAddRequest`. approve/reject won't set `AddRequest`; map `Error==0`→`ok`, else `failed` with `Code=Error`. Whole-request sentinels via `mapIQError`.

**`LinkGroup`** — `group.go:152`
```go
func (cli *Client) LinkGroup(ctx context.Context, parent, child types.JID) error
```
**`UnlinkGroup`** — `group.go:137`
```go
func (cli *Client) UnlinkGroup(ctx context.Context, parent, child types.JID) error
```
No named sentinels → raw IQError through `mapIQError`: 403→403 (not admin on parent/child), 400→400, 409→409 (already linked / is itself a parent).

**IQError** (`errors.go:174`): `type IQError struct { Code int; Text string; ErrorNode, RawNode *waBinary.Node }`. Named IQ sentinels: 400/401/403/404/405/406/410/419/423/429/500/503/530 at `errors.go:183-195`.

### `mapIQError` (new helper in `internal/whatsapp/client.go`)
```go
func mapIQError(err error) (error, bool) {
    var iq *whatsmeow.IQError
    if !errors.As(err, &iq) {
        return nil, false
    }
    switch iq.Code {
    case 400, 405, 406:
        return errDomain.NewError(errDomain.ErrBadRequest, err), true
    case 401, 403: // authenticated caller lacks a *group role* → 403, never 401
        return errDomain.NewError(errDomain.ErrForbidden, err), true
    case 404:
        return errDomain.NewError(errDomain.ErrNotFound, err), true
    case 409, 423:
        return errDomain.NewError(errDomain.ErrConflict, err), true
    case 410:
        return errDomain.NewError(errDomain.ErrGone, err), true
    case 419, 429:
        return errDomain.NewError(errDomain.ErrTooManyRequests, err), true
    default:
        return errDomain.NewError(errDomain.ErrInternalFailure, err), true
    }
}
```
**Wire it into the existing `mapWhatsmeowErr`** (client.go:147), inserting the named invite sentinels + the `mapIQError` fallback **before** the generic `isRecipientError`/`ErrInternalFailure` tail. Order matters (named sentinels first, for the specific human message — the status is the same either way):
```go
// after the existing ErrGroupNotFound / ErrNotInGroup block:
if errors.Is(err, whatsmeow.ErrInviteLinkRevoked) {
    return errDomain.NewError(errDomain.ErrGone, err)
}
if errors.Is(err, whatsmeow.ErrInviteLinkInvalid) {
    return errDomain.NewError(errDomain.ErrBadRequest, err)
}
if errors.Is(err, whatsmeow.ErrGroupInviteLinkUnauthorized) {
    return errDomain.NewError(errDomain.ErrForbidden, err)
}
if mapped, ok := mapIQError(err); ok {
    return mapped
}
```

---

## 3. Ordered edit list

**3.1 `domain/error/error.go`** — add sentinel:
```go
ErrGone = errors.New("gone")
```

**3.2 `internal/httperror/httperror.go`** — add case in the `switch svcErr`:
```go
case errDomain.ErrGone:
    apiError.Status = http.StatusGone
```

**3.3 `internal/whatsapp/client.go`** — add 7 methods to the `Client` interface + impls, plus `mapIQError` (§2) and fold sentinels into `mapWhatsmeowErr`. Also extract the `*types.GroupInfo → *waDomain.GroupInfoResponse` mapping currently inlined in `GetGroupInfo` (client.go:410-436) into a reusable helper `toGroupInfoResponse(g *types.GroupInfo) *waDomain.GroupInfoResponse` so the preview path reuses it.

Interface additions:
```go
GetGroupInviteLink(ctx context.Context, traceID, phoneNumber, groupJID string, reset bool) (string, error)
JoinGroupWithLink(ctx context.Context, traceID, phoneNumber, code string) (string, error)
GetGroupInfoFromLink(ctx context.Context, traceID, phoneNumber, code string) (*waDomain.GroupInfoResponse, error)
GetGroupRequestParticipants(ctx context.Context, traceID, phoneNumber, groupJID string) ([]waDomain.GroupJoinRequestItem, error)
UpdateGroupRequestParticipants(ctx context.Context, traceID, phoneNumber, groupJID string, participants []string, approve bool) ([]waDomain.ParticipantResult, error)
LinkSubGroup(ctx context.Context, traceID, phoneNumber, parentJID, childJID string) error
UnlinkSubGroup(ctx context.Context, traceID, phoneNumber, parentJID, childJID string) error
```
Impl shape (mirror the shipped `GetGroupInfo` at client.go:390 — nil-guard→404, `!IsLoggedIn`→401, `types.ParseJID` per JID with `Server != types.GroupServer`→400 for group JIDs, call whatsmeow, `mapWhatsmeowErr` on error). For `UpdateGroupRequestParticipants`: parse each participant JID, pick `whatsmeow.ParticipantChangeApprove`/`ParticipantChangeReject` from the `approve` bool, then map `[]types.GroupParticipant`→`[]waDomain.ParticipantResult` (`Error==0`→`ok`, else `failed`+`Code`). For `GetGroupInfoFromLink`/`JoinGroupWithLink`: **no ParseJID** — pass `code` straight through (whatsmeow strips the prefix); just the nil/login guards.

**3.4 `internal/whatsapp/manager.go`** — add the same 7 methods to the `Manager` interface + logging pass-throughs (copy the `GetGroupInfo` manager wrapper at manager.go:567).

**3.5 `internal/usecase/whatsapp/group_usecase.go` (NEW)** — methods on `*WhatsappMessageUsecase`:
- `resolveGroupChat(chat, msisdn string) (string, error)` helper: `resolveChat` then require `strings.HasSuffix(target, "@"+groupServerSuffix)` else `ErrBadRequest` (reuses the exact guard from `GetGroupInfo`, read_usecase.go:91). `groupServerSuffix` const already exists (read_usecase.go:16).
- `requireGroupManagement() error`: `if !uc.config.GroupManagementEnabled { return errDomain.NewError(errDomain.ErrNotFound, errors.New("group management is disabled")) }`.
- `GetGroupInviteLink` / `ResetGroupInviteLink`: `requireGroupManagement` → `resolveGroupChat` → `spendActionBudget` → `manager.GetGroupInviteLink(..., reset)`.
- `JoinGroup(req)`: `requireGroupManagement` → **if `!uc.config.GroupJoinViaLinkEnabled` return `ErrForbidden`** ("joining via link is disabled") → trim/validate `req.Code` non-empty → `spendActionBudget` → `manager.JoinGroupWithLink`.
- `GetGroupInviteInfo(code)`: `requireGroupManagement` → validate non-empty → `queryWithBudget(uc, ctx, phone, "inviteinfo:"+phone+":"+code, fn)`.
- `ListJoinRequests(chat, msisdn)`: `requireGroupManagement` → `resolveGroupChat` → `queryWithBudget(... "joinreqs:"+phone+":"+target ...)`.
- `UpdateJoinRequests(req)`: `requireGroupManagement` → `resolveGroupChat` → validate `action ∈ {approve, reject}` (→400) → **dedupe + reject empty** participants (→400) → resolve each participant via `resolveChat` → `spendActionBudget` → `manager.UpdateGroupRequestParticipants` → assemble response (partial success = 200).
- `LinkSubGroup(req)` / `UnlinkSubGroup(req)`: `requireGroupManagement` → `resolveGroupChat(parent)` + `resolveGroupChat(child)` → `spendActionBudget` → manager call.

**3.6 `internal/handler/whatsapp/whatsapp_group_handler.go` (NEW)** — methods on `*WhatsappMessageHandler` (same struct). Copy the shipped pattern (whatsapp_message_handler.go:32): `middleware.GetTraceID`, `utils.MustGetPhoneNumber`, `c.BodyParser`/`c.Query`, call usecase, `httperror.FromError`, `c.Status(...).JSON(...)`. GETs read `c.Query("chat")`/`c.Query("code")`/`c.Query("child")`; POST/DELETE use `BodyParser`. `POST /group/join` → 200 with `JoinGroupResponse`; approve/reject → 200 with results even when mixed.

**3.7 `internal/router/group_routes.go`** — append inside `initGroupRoutes`:
```go
groupGroup.Get("/invite", h.WhatsappMessageHandler.GetGroupInviteLink)
groupGroup.Post("/invite/reset", h.WhatsappMessageHandler.ResetGroupInviteLink)
groupGroup.Get("/invite/info", h.WhatsappMessageHandler.GetGroupInviteInfo)
groupGroup.Post("/join", h.WhatsappMessageHandler.JoinGroup)
groupGroup.Get("/requests", h.WhatsappMessageHandler.ListJoinRequests)
groupGroup.Post("/requests", h.WhatsappMessageHandler.UpdateJoinRequests)
```
(Fiber routes `/invite` and `/invite/info` and `/invite/reset` as distinct paths — no collision.)

**3.8 `internal/router/community_routes.go` (NEW)** + register in `router.go`:
```go
func initCommunityRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
    g := r.Group("/community")
    g.Use(authMw.JWTAuthentication())
    g.Post("/subgroups", h.WhatsappMessageHandler.LinkSubGroup)
    g.Delete("/subgroups", h.WhatsappMessageHandler.UnlinkSubGroup)
}
```
Add `initCommunityRoutes(api, h, authMw)` after `initGroupRoutes(...)` in `SetupRouter` (router.go:157).

**3.9 `config/config.go`** — §5. **No Wire changes** (usecase/handler structs unchanged; only new methods).

---

## 4. Migration DDL

**None.** No new tables, columns, or persisted state. (The `$1`/dual-dialect/ON-CONFLICT conventions are irrelevant to this feature group.)

---

## 5. New config keys

Add to the `Config` struct (`config/config.go`, alongside the `MaxUploadBytes`/read-query block):
```go
// Group & community management
GroupManagementEnabled  bool `mapstructure:"GROUP_MANAGEMENT_ENABLED" default:"true"`
GroupJoinViaLinkEnabled bool `mapstructure:"GROUP_JOIN_VIA_LINK_ENABLED" default:"false"`
```
| Key | Type | Default | Effect |
|---|---|---|---|
| `GROUP_MANAGEMENT_ENABLED` | bool | `true` | Master toggle; every group/community endpoint in this group returns **404** when false. |
| `GROUP_JOIN_VIA_LINK_ENABLED` | bool | `false` | Gates the mass-join vector `POST /group/join`; **403** when off. |

**Deliberately NOT added:** `GROUP_INFO_CACHE_TTL_SECONDS` (reuse `READ_QUERY_CACHE_TTL_SECONDS`), `GROUP_ADD_PARTICIPANTS_ENABLED` / `GROUP_MAX_PARTICIPANTS_PER_REQUEST` (belong to the participants/create feature group). Mirror the two new keys into `.env.example`, `wiki/Environment-Variables.md`, `CLAUDE.md` Configuration list.

---

## 6. openapi.yaml delta (bump `info.version` 0.18.0 → 0.19.0)

Add a reusable 410 note and these paths (style copied from the shipped `/group/info` block at line 1100). Abbreviated:

```yaml
/group/invite:
  get:
    tags: [Group]
    summary: Get a group's invite link (admin only)
    parameters:
      - { name: chat, in: query, required: true, schema: { type: string, example: "1203...@g.us" } }
    responses:
      '200': { description: Invite link., content: { application/json: { schema: { $ref: '#/components/schemas/GroupInviteLinkResponse' } } } }
      '400': { description: chat is missing or not a group JID. }
      '403': { description: Account is not an admin, or not a participant. }
      '404': { description: Group management disabled, or group not found. }
      '429': { description: Action budget exhausted. }
    security: [ { accountBearerAuth: [] } ]
/group/invite/reset:
  post:
    tags: [Group]
    summary: Revoke and regenerate the invite link
    requestBody: { required: true, content: { application/json: { schema: { $ref: '#/components/schemas/GroupInviteResetRequest' } } } }
    responses: { '200': { ... GroupInviteLinkResponse }, '403': {...}, '404': {...}, '429': {...} }
/group/invite/info:
  get:
    tags: [Group]
    summary: Preview a group from an invite link (no join)
    parameters: [ { name: code, in: query, required: true, schema: { type: string } } ]
    responses:
      '200': { description: Group preview., content: { application/json: { schema: { $ref: '#/components/schemas/GroupInfoResponse' } } } }
      '400': { description: Invite link malformed/invalid. }
      '410': { description: Invite link revoked — request a fresh link. }
      '429': { description: Read budget exhausted. }
/group/join:
  post:
    tags: [Group]
    summary: Join a group via invite link (gated)
    requestBody: { required: true, content: { application/json: { schema: { $ref: '#/components/schemas/JoinGroupRequest' } } } }
    responses:
      '200': { ... JoinGroupResponse }
      '400': { description: Invite link malformed/invalid. }
      '403': { description: Joining via link is disabled (GROUP_JOIN_VIA_LINK_ENABLED=false). }
      '404': { description: Group management disabled. }
      '410': { description: Invite link revoked. }
/group/requests:
  get: { tags: [Group], summary: List pending join requests, parameters: [chat], responses: { '200': GroupJoinRequestsResponse, '403': not admin, '404': ..., '429': ... } }
  post: { tags: [Group], summary: Approve/reject join requests, requestBody: GroupJoinRequestsActionRequest, responses: { '200': GroupJoinRequestsActionResponse (partial success), '400': bad action/empty, '403': not admin, '404': ... } }
/community/subgroups:
  post: { tags: [Community], summary: Link a subgroup, requestBody: CommunityLinkRequest, responses: { '200': {}, '400': ..., '403': not admin on parent/child, '409': already linked, '404': ... } }
  delete: { tags: [Community], summary: Unlink a subgroup, parameters: [chat, child], responses: { '200': {}, '403': ..., '404': ... } }
```
Add schemas: `GroupInviteLinkResponse`, `GroupInviteResetRequest`, `JoinGroupRequest`, `JoinGroupResponse`, `GroupJoinRequestItem`, `GroupJoinRequestsResponse`, `GroupJoinRequestsActionRequest`, `GroupJoinRequestsActionResponse`, `ParticipantResult`, `ParticipantInvite`, `CommunityLinkRequest` (field lists = §1).

---

## 7. Edge cases, partial-failure, gating

| Case | Handling |
|---|---|
| **`GROUP_MANAGEMENT_ENABLED=false`** | `requireGroupManagement()` → **404** on every endpoint (hidden entirely). Checked first, before any resolve/budget. |
| **`GROUP_JOIN_VIA_LINK_ENABLED=false`** (mass-join gate, default off) | `POST /group/join` → **403** "joining via link is disabled". This is the ban-risky path; stays dark until pacing (#2). |
| **Get/reset invite link, not admin** | `ErrGroupInviteLinkUnauthorized` (wraps IQ 401) → **403** (authenticated caller, group-role issue — never 401). `ErrNotInGroup`/IQ 403 → 403; `ErrGroupNotFound`/IQ 404 → 404. |
| **Join/preview revoked link** | `ErrInviteLinkRevoked` (IQ 410) → **410 ErrGone** "invite link revoked". |
| **Join/preview malformed/invalid link** | `ErrInviteLinkInvalid` (IQ 406) → **400** "invite link is not valid". Kept distinct from 410 so caller tells retry-with-new-link from fix-input. |
| **Join idempotency / already-member / approval-pending** | whatsmeow returns a bare `@g.us` JID and **does not expose which branch** (group vs `membership_approval_request`). *Corner cut:* return `{group_jid}` only — no `already_member`/`membership_pending` flag. `// ponytail: whatsmeow collapses group vs approval-request into one JID; add a flag if it ever exposes the branch.` |
| **Approve/reject: some JIDs not pending / bad** | Per-item `GroupParticipant.Error != 0` → `ParticipantResult{status:"failed", code:Error}`; overall **200 partial success**, never fail the batch for one item. |
| **Approve/reject: empty or duplicate participants** | **400** on empty; **dedupe** before the server call so `results` map 1:1. |
| **Approve/reject: invalid `action`** | **400** (must be `approve`/`reject`). |
| **Group addressed by bare number** | `resolveGroupChat` requires `@g.us` → **400**; a group op never silently lands on a user JID. |
| **Community link without admin on parent/child** | raw IQ 403 → **403**. Already-linked / child-is-itself-a-parent → IQ 400/409 → **400/409** via `mapIQError`. |
| **Server resource/rate limit** | IQ 419/429 → **429** via `mapIQError`. Local `spendActionBudget` exhaustion → **429** (`ErrTooManyRequests`). |
| **No session / not logged in** | client-layer nil-guard → **404** (`ErrClientNotFound`); `!IsLoggedIn()` → **401**. |

Gated (default-off) paths in this group: **`POST /group/join`** only (`GROUP_JOIN_VIA_LINK_ENABLED`). Everything else (invite get/reset, preview, requests list/approve/reject, community link/unlink) ships enabled under the master toggle.

---

## 8. Unit-test seams (no live client — guards fire before the nil manager)

Reuse `newTestUsecase` (query_test.go:15) — a `WhatsappMessageUsecase` with only cache+budget. Set `.config` (the tests need `GroupManagementEnabled`/`GroupJoinViaLinkEnabled`, so extend the helper or build `&config.Config{...}` inline) and `.limiter` for `spendActionBudget`.

**`group_usecase_test.go` (usecase level, nil manager):**
- `requireGroupManagement`: every method returns **404** when `GroupManagementEnabled=false`.
- `JoinGroup`: **403** when `GroupJoinViaLinkEnabled=false`; **400** when `code` empty.
- `resolveGroupChat` via `GetGroupInviteLink`/`ListJoinRequests`/`LinkSubGroup`: bare number and user JID → **400**; `@g.us` (mixed case server) accepted + lowercased (mirror `TestGetGroupInfo_RejectsNonGroupChat`, query_test.go:73).
- `UpdateJoinRequests`: invalid `action` → 400; empty participants → 400; **duplicate participants deduped** (assert resolved slice length).
- `LinkSubGroup`/`UnlinkSubGroup`: non-`@g.us` parent OR child → 400.

**`client_test.go` (or `map_test.go`) in `internal/whatsapp` — pure mapping, no client:**
- `mapIQError`: construct `&whatsmeow.IQError{Code: c}` for c ∈ {400,401,403,404,406,409,410,419} and assert the domain sentinel (esp. **410→ErrGone**, **401→ErrForbidden** not Unauthorized).
- `mapWhatsmeowErr`: `errors.Is` on `whatsmeow.ErrInviteLinkRevoked` → `ErrGone`; `ErrInviteLinkInvalid` → `ErrBadRequest`; `ErrGroupInviteLinkUnauthorized` → `ErrForbidden`. (These wrap IQErrors, so this also confirms the named-sentinel-before-generic ordering.)
- Per-participant mapping: `[]types.GroupParticipant{{JID:.., Error:0}, {JID:.., Error:404}}` → `[]ParticipantResult{{status:"ok"},{status:"failed",code:404}}`.

**Cache/budget:** already covered by the shipped `query_test.go` (hit bypasses budget, miss spends, errors not cached) — the new cached reads (`inviteinfo:`, `joinreqs:`) inherit it for free; no new cache tests needed.

**Not CI-testable (needs live WhatsApp):** the actual IQ codes for revoked/invalid/not-admin. Per the PRD "verify once, then lock to the exported sentinel" — the sentinels are already exported (`errors.go:84-92`), so the mapping is pinned; no substring guesses.

---

### Skipped / deferred (say the word to add)
- `already_member`/`membership_pending` on join → skipped: whatsmeow doesn't expose the branch. Add when it does.
- Separate `WhatsappGroupUsecase`/handler/Wire providers → skipped: shipped code already extends the message usecase for group reads. Add if you want strict per-feature struct separation.
- `GROUP_INFO_CACHE_TTL_SECONDS` + second cache → skipped: reuse `READ_QUERY_CACHE_TTL_SECONDS`. Add when group reads need a materially different TTL than other reads.