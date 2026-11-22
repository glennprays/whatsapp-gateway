I have everything I need. Reads (`ListGroups`, `GetGroupInfo`) already ship on `WhatsappMessageUsecase`/`WhatsappMessageHandler` with routes in `internal/router/group_routes.go`. Here is the blueprint.

---

# Blueprint — Phase E Group Mutations (CORE, gated)

Scope (this feature group only): **create group, leave, participants add/remove/promote/demote, settings announce/locked, set name, set topic, set/remove photo.** Invite links, join-via-link, join-requests, and community link/unlink are *out of scope* here (separate PRD-3 items 10-19).

## 0. Key architecture decision (deviation from PRD, ponytail-justified)

The PRD proposes a new `group_usecase.go` + `whatsapp_group_handler.go` + `ProvideWhatsappGroupUsecase/Handler` + `wire.go` edits. **Skip all of that.** The group *reads* already live as methods on the existing `WhatsappMessageUsecase` (`read_usecase.go`) and `WhatsappMessageHandler`, and `resolveChat` / `spendActionBudget` / `uc.config` / `queryWithBudget` are all already on that receiver. Add the mutation methods as **new files on the same receivers** (`internal/usecase/whatsapp/group_usecase.go`, `internal/handler/whatsapp/whatsapp_group_handler.go`) exactly like `read_usecase.go`/`interaction_usecase.go` do. Result: **zero `provider.go`, zero `wire.go`, zero `handler.go` struct changes, no `wire gen`.** Fragmenting group logic into a parallel usecase would only add a constructor, a provider, and a Wire entry for no behavioral benefit.

Addressing: mutations take `chat` in the **body/form** (not a `:chat` path param) to match the shipped message endpoints (`POST /message/text` with body `chat`) and reuse `resolveChat`. This diverges from the PRD's `/group/:chat/...` sketch; the body-`chat` shape is the codebase-consistent one.

---

## 1. Endpoints + DTOs

All under `/group`, all behind `authMw.JWTAuthentication()`, all gated at route-registration by `GROUP_MANAGEMENT_ENABLED` (routes not registered → 404 when off). Reads stay registered unconditionally.

| Method | Path | Usecase | Notes |
|---|---|---|---|
| POST | `/group` | `CreateGroup` | 201; add-on-create gated |
| POST | `/group/leave` | `LeaveGroup` | 200; allowed for non-admins |
| POST | `/group/participants` | `UpdateGroupParticipants` | 200 partial-success; `action=add` gated |
| PATCH | `/group/settings` | `SetGroupSettings` | 200; announce/locked |
| PATCH | `/group/name` | `SetGroupName` | 200 |
| PATCH | `/group/topic` | `SetGroupTopic` | 200 |
| PUT | `/group/photo` | `SetGroupPhoto` | 200; multipart JPEG |
| DELETE | `/group/photo` | `RemoveGroupPhoto` | 200; body/query `chat` |

**DTOs** (add to existing `domain/whatsapp/group.go`; `time` import needed):

```go
// Recipient wrapper reused by leave / photo-delete (body {chat} or ?chat=).
type GroupTargetRequest struct {
    Chat   string `json:"chat"`
    Msisdn string `json:"msisdn"` // deprecated alias; chat wins
}

type CreateGroupRequest struct {
    Name                   string   `json:"name"`
    Participants           []string `json:"participants"`             // may be empty → group of just the account
    IsCommunity            bool     `json:"is_community,omitempty"`   // ReqCreateGroup.IsParent
    LinkedParentJID        string   `json:"linked_parent_jid,omitempty"` // create inside a community
    IsAnnounce             bool     `json:"is_announce,omitempty"`
    IsLocked               bool     `json:"is_locked,omitempty"`
    IsJoinApprovalRequired bool     `json:"is_join_approval_required,omitempty"`
}
type CreateGroupResponse struct {
    GroupJID  string              `json:"group_jid"`
    GroupInfo *GroupInfoResponse  `json:"group_info"`
    Results   []ParticipantResult `json:"results"`
}

type GroupParticipantsRequest struct {
    Chat         string   `json:"chat"`
    Msisdn       string   `json:"msisdn"`
    Action       string   `json:"action"` // add|remove|promote|demote
    Participants []string `json:"participants"`
}
type GroupParticipantsResponse struct {
    GroupJID string              `json:"group_jid"`
    Action   string              `json:"action"`
    Results  []ParticipantResult `json:"results"`
}

// Canonical per-participant outcome. Partial failure => overall 200.
type ParticipantResult struct {
    JID    string            `json:"jid"`
    LID    string            `json:"lid,omitempty"`
    Status string            `json:"status"`         // ok|invited|failed
    Code   int               `json:"code,omitempty"` // whatsmeow participant error code
    Invite *ParticipantInvite `json:"invite,omitempty"`
}
type ParticipantInvite struct {
    Code      string    `json:"code"`
    ExpiresAt time.Time `json:"expires_at"`
}

// Pointer fields = "only change what was supplied".
type GroupSettingsRequest struct {
    Chat     string `json:"chat"`
    Msisdn   string `json:"msisdn"`
    Announce *bool  `json:"announce,omitempty"` // only admins can send
    Locked   *bool  `json:"locked,omitempty"`   // only admins can edit info
}
type GroupSettingsResponse struct {
    GroupJID string   `json:"group_jid"`
    Applied  []string `json:"applied"` // e.g. ["announce","locked"]
}

type SetGroupNameRequest struct {
    Chat, Msisdn, Name string
}
type SetGroupTopicRequest struct {
    Chat, Msisdn, Topic string
}
type GroupPhotoResponse struct {
    PictureID string `json:"picture_id,omitempty"`
    Removed   bool   `json:"removed,omitempty"`
}
```

Photo upload is `multipart/form-data` (form field `photo` + `chat`), not JSON — mirrors `SendImageMessage`.

---

## 2. whatsmeow calls — real signatures + error mapping

All quotes from `.../whatsmeow@v0.0.0-20260516102357-8d3700152a69/group.go` and `types/group.go`, `errors.go`.

```go
// group.go:32  (request struct for CreateGroup)
type ReqCreateGroup struct {
    Name         string          // ≤25 chars or server 406
    Participants []types.JID     // own JID added implicitly
    CreateKey    types.MessageID
    types.GroupEphemeral            // IsEphemeral, DisappearingTimer
    types.GroupAnnounce            // IsAnnounce
    types.GroupLocked             // IsLocked
    types.GroupMembershipApprovalMode // IsJoinApprovalRequired
    types.GroupParent             // IsParent  (set true => community)
    types.GroupLinkedParent       // LinkedParentJID (create inside community)
}
// group.go:56
func (cli *Client) CreateGroup(ctx context.Context, req ReqCreateGroup) (*types.GroupInfo, error)

// group.go:168
func (cli *Client) LeaveGroup(ctx context.Context, jid types.JID) error

// group.go:179-186
type ParticipantChange string
const (
    ParticipantChangeAdd     ParticipantChange = "add"
    ParticipantChangeRemove  ParticipantChange = "remove"
    ParticipantChangePromote ParticipantChange = "promote"
    ParticipantChangeDemote  ParticipantChange = "demote"
)
// group.go:189
func (cli *Client) UpdateGroupParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action ParticipantChange) ([]types.GroupParticipant, error)

// group.go:292  (avatar nil => remove; returns "remove"; else pictureID)
func (cli *Client) SetGroupPhoto(ctx context.Context, jid types.JID, avatar []byte) (string, error)

// group.go:324
func (cli *Client) SetGroupName(ctx context.Context, jid types.JID, name string) error
// group.go:337  (pass previousID="" newID="" => whatsmeow fetches prev TopicID + generates new ID itself)
func (cli *Client) SetGroupTopic(ctx context.Context, jid types.JID, previousID, newID, topic string) error
// group.go:371
func (cli *Client) SetGroupLocked(ctx context.Context, jid types.JID, locked bool) error
// group.go:381
func (cli *Client) SetGroupAnnounce(ctx context.Context, jid types.JID, announce bool) error

// (settings extensions if you widen scope)
// group.go:1018
func (cli *Client) SetGroupJoinApprovalMode(ctx context.Context, jid types.JID, mode bool) error
// group.go:1039
func (cli *Client) SetGroupMemberAddMode(ctx context.Context, jid types.JID, mode types.GroupMemberAddMode) error
```

**Per-participant result type** (`types/group.go:103-125`, populated by `parseParticipant` `group.go:672-699`):
```go
type GroupParticipant struct {
    JID, PhoneNumber, LID types.JID
    IsAdmin, IsSuperAdmin bool
    DisplayName string
    Error       int                        // 0 => ok; !=0 => add failed for this member
    AddRequest  *GroupParticipantAddRequest // present => converted to an invite
}
type GroupParticipantAddRequest struct { Code string; Expiration time.Time }
```
Map each returned participant → `ParticipantResult`:
- `Error==0` → `status:"ok"`
- `Error!=0 && AddRequest!=nil` → `status:"invited"`, `code:Error`, `invite:{AddRequest.Code, AddRequest.Expiration}` (privacy-blocked / non-contact; **not yet a member**)
- `Error!=0 && AddRequest==nil` → `status:"failed"`, `code:Error`

**Error sentinels & HTTP mapping.** Critical nuance: unlike `GetGroupInfo`/`GetGroupInviteLink`, the **mutation** calls (`CreateGroup`, `LeaveGroup`, `UpdateGroupParticipants`, `SetGroupName/Topic/Locked/Announce`) do **not** wrap their IQ errors into named sentinels — they return the **raw `*whatsmeow.IQError`** from `sendGroupIQ`. So `errors.Is(err, whatsmeow.ErrGroupNotFound)` will **not** fire for these; you must match on `IQError.Code` via `errors.As`. `SetGroupPhoto` is the one exception: it wraps `ErrInvalidImageFormat` on IQ-406 (`group.go:308-309`).

```go
// errors.go:174-196
type IQError struct { Code int; Text string; ErrorNode, RawNode *waBinary.Node }
ErrIQBadRequest   = &IQError{Code:400,...}
ErrIQNotAuthorized= &IQError{Code:401,...}
ErrIQForbidden    = &IQError{Code:403,...}
ErrIQNotFound     = &IQError{Code:404,...}
ErrIQNotAcceptable= &IQError{Code:406,...}
ErrIQGone         = &IQError{Code:410,...}
ErrIQResourceLimit= &IQError{Code:419,...}
ErrIQLocked       = &IQError{Code:423,...}
ErrIQRateOverLimit= &IQError{Code:429,...}
// errors.go:98  (SetGroupPhoto wraps this on 406)
ErrInvalidImageFormat = errors.New("the given data is not a valid image")
```

| whatsmeow error | Domain sentinel | HTTP |
|---|---|---|
| `IQError.Code == 401` (not-authorized / not admin) | `ErrForbidden` | 403 "account is not an admin of this group" |
| `IQError.Code == 403` (forbidden) | `ErrForbidden` | 403 |
| `IQError.Code == 404` (item-not-found) | `ErrNotFound` | 404 "that group does not exist" |
| `IQError.Code == 400` (bad-request) | `ErrBadRequest` | 400 (server text) |
| `IQError.Code == 406` (not-acceptable) | `ErrBadRequest` | 400 (name>25 / bad photo) |
| `IQError.Code == 419` / `429` | `ErrTooManyRequests` | 429 "rate limited" |
| `IQError.Code == 423` (locked) | `ErrConflict` | 409 |
| `errors.Is(ErrInvalidImageFormat)` | `ErrBadRequest` | 400 "group photo must be a JPEG" |
| `store.ErrDeviceDeleted` | `ErrConflict` (evicts client) | 409 — already handled in `mapWhatsmeowErr` |
| default | `ErrInternalFailure` | 500 |

Per-participant `Error` codes (403 privacy → invited, 404/400 → failed) are **not** overall errors — they arrive on a `nil`-error `UpdateGroupParticipants`/`CreateGroup` return.

---

## 3. Ordered edit list

### 3.1 `internal/whatsapp/client.go`
Extend `mapWhatsmeowErr` (root-cause: one place, all mutation callers route through it) — add **before** the final 500 return, after the existing sentinel checks:
```go
if errors.Is(err, whatsmeow.ErrInvalidImageFormat) {
    return errDomain.NewError(errDomain.ErrBadRequest, errors.New("group photo must be a JPEG"))
}
var iq *whatsmeow.IQError
if errors.As(err, &iq) {
    switch iq.Code {
    case 401, 403: return errDomain.NewError(errDomain.ErrForbidden, err)
    case 404:      return errDomain.NewError(errDomain.ErrNotFound, err)
    case 406, 400: return errDomain.NewError(errDomain.ErrBadRequest, err)
    case 419, 429: return errDomain.NewError(errDomain.ErrTooManyRequests, err)
    case 423:      return errDomain.NewError(errDomain.ErrConflict, err)
    }
}
```
Add to the `Client` interface + impl (thin wrappers; each does `clients.Get` nil→404, `IsLoggedIn`→401, `types.ParseJID`, then the whatsmeow call, then `mapWhatsmeowErr`). Return domain DTOs so whatsmeow types never leak upward (mirrors `GetGroupInfo`):
```go
CreateGroup(ctx, traceID, phoneNumber string, name string, participantJIDs []string, isCommunity bool, linkedParentJID string, isAnnounce, isLocked, isJoinApproval bool) (*waDomain.CreateGroupResponse, error)
LeaveGroup(ctx, traceID, phoneNumber, groupJID string) error
UpdateGroupParticipants(ctx, traceID, phoneNumber, groupJID, action string, participantJIDs []string) ([]waDomain.ParticipantResult, error)
SetGroupAnnounce(ctx, traceID, phoneNumber, groupJID string, announce bool) error
SetGroupLocked(ctx, traceID, phoneNumber, groupJID string, locked bool) error
SetGroupName(ctx, traceID, phoneNumber, groupJID, name string) error
SetGroupTopic(ctx, traceID, phoneNumber, groupJID, topic string) error
SetGroupPhoto(ctx, traceID, phoneNumber, groupJID string, photo []byte) (string, error) // nil photo => remove, returns "remove"
```
Impl details:
- `CreateGroup`: build `whatsmeow.ReqCreateGroup{Name:name, Participants:jids, GroupParent:types.GroupParent{IsParent:isCommunity}, GroupLinkedParent:types.GroupLinkedParent{LinkedParentJID:parentJID}, GroupAnnounce:types.GroupAnnounce{IsAnnounce:...}, GroupLocked:{IsLocked:...}, GroupMembershipApprovalMode:{IsJoinApprovalRequired:...}}`. `info.JID.String()` → GroupJID; map `info.Participants` → `[]ParticipantResult` via the shared mapper; reuse the existing `GetGroupInfo` participant→`GroupInfoResponse` projection for `group_info`.
- `UpdateGroupParticipants`: `action` string → `whatsmeow.ParticipantChange(action)`; map returned `[]types.GroupParticipant` → `[]ParticipantResult`.
- `SetGroupTopic`: call with `previousID="", newID="", topic` — whatsmeow auto-fetches prev topic ID (note: this costs an extra internal `GetGroupInfo` IQ; acceptable).
- **Self-LID defensive guard** (belt-and-suspenders for the LID form the usecase can't see): in `UpdateGroupParticipants`, for `remove/promote/demote`, if a parsed JID equals `cli.Store.LID` (`store.Device.LID`, `store/store.go:227`) or `*cli.Store.ID`, return `ErrBadRequest` "use /group/leave to remove yourself".
- Add a shared unexported mapper `participantResults(ps []types.GroupParticipant) []waDomain.ParticipantResult`.

### 3.2 `internal/whatsapp/manager.go`
Mirror all 8 methods on the `Manager` interface + impl as logging pass-throughs (existing per-method style; `m.Logger.Info/Error` around `m.Client.X(...)`).

### 3.3 `internal/usecase/whatsapp/group_usecase.go` (NEW, methods on `*WhatsappMessageUsecase`)
Helpers first:
```go
const (maxGroupNameLen = 25; maxGroupTopicLen = 512)

// require an explicit @g.us after normalization.
func resolveGroupJID(chat, msisdn string) (string, error) {
    t, err := resolveChat(chat, msisdn)
    if err != nil { return "", err }
    if !strings.HasSuffix(t, "@"+groupServerSuffix) {
        return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("chat must be a group (@g.us), got %q", t))
    }
    return t, nil
}

// normalize + dedupe + empty-check + self-PN guard + cap. ownPhone is the account number.
func (uc *WhatsappMessageUsecase) resolveParticipants(list []string, ownPhone string, allowEmpty, selfGuard bool) ([]string, error) {
    seen := map[string]bool{}; out := []string{}
    for _, p := range list {
        jid, err := resolveChat(p, "")          // bare number → @s.whatsapp.net; validates JID
        if err != nil { return nil, err }
        if strings.HasSuffix(jid, "@"+groupServerSuffix) {
            return nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("participant must be a user, not a group: %q", p))
        }
        if selfGuard && strings.SplitN(jid, "@", 2)[0] == ownPhone { // PN self-guard
            return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("use POST /group/leave to remove yourself"))
        }
        if !seen[jid] { seen[jid] = true; out = append(out, jid) }
    }
    if len(out) == 0 && !allowEmpty {
        return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("participants is required"))
    }
    if uc.config.GroupMaxParticipantsPerRequest > 0 && len(out) > uc.config.GroupMaxParticipantsPerRequest {
        return nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("too many participants (max %d)", uc.config.GroupMaxParticipantsPerRequest))
    }
    return out, nil
}
```
Methods (each runs `uc.spendActionBudget(ctx, phoneNumber)` **before** the manager call — the interim send-path pacer):
- `CreateGroup`: validate `Name` non-empty & ≤25; `resolveParticipants(req.Participants, phoneNumber, allowEmpty=true, selfGuard=false)`; **if `len(participants)>0 && !cfg.GroupAddParticipantsEnabled` → 403 "adding participants is disabled"**; resolve `LinkedParentJID` via `resolveGroupJID` when set; spend budget; call manager.
- `UpdateGroupParticipants`: `resolveGroupJID`; validate `action ∈ {add,remove,promote,demote}` (else 400); `selfGuard := action != "add"`; `resolveParticipants(..., allowEmpty=false, selfGuard)`; **if `action=="add" && !cfg.GroupAddParticipantsEnabled` → 403**; spend budget; call manager; wrap results.
- `LeaveGroup`: `resolveGroupJID`; spend budget; call.
- `SetGroupSettings`: `resolveGroupJID`; require at least one of `Announce/Locked` non-nil (else 400); spend budget; call `SetGroupAnnounce`/`SetGroupLocked` for each supplied flag; append to `Applied`. (Multiple IQs; stop on first error and return it.)
- `SetGroupName`: `resolveGroupJID`; `len(Name)>25 → 400`; empty name → 400; spend budget; call.
- `SetGroupTopic`: `resolveGroupJID`; `len(Topic)>512 → 400` (empty topic is valid = clears it); spend budget; call.
- `SetGroupPhoto(chat, file)`: `resolveGroupJID`; `uc.validateMediaSize(file.Size)`; read file; `http.DetectContentType(head)=="image/jpeg"` else 400 "group photo must be a JPEG"; spend budget; call manager (returns pictureID).
- `RemoveGroupPhoto(chat)`: `resolveGroupJID`; spend budget; `manager.SetGroupPhoto(..., nil)`.

Cache/budget: **mutations use `spendActionBudget` only** (no `queryWithBudget`, no `queryCache`). The read cache/`READ_QUERY_*` budget applies only to the already-shipped reads.

### 3.4 `internal/handler/whatsapp/whatsapp_group_handler.go` (NEW, methods on `*WhatsappMessageHandler`)
Same skeleton as existing handlers (`GetTraceID`, `MustGetPhoneNumber`, `BodyParser`, `httperror.FromError`). Statuses: `CreateGroup`→201, others→200. Photo handler: `c.FormValue("chat")` + `c.FormFile("photo")` (mirror `SendImageMessage` lines 65-112). `RemoveGroupPhoto`: `BodyParser` of `GroupTargetRequest`, fall back to `c.Query("chat")`.

### 3.5 `internal/router/group_routes.go` (EDIT)
Pass `cfg` in and gate mutations:
```go
func initGroupRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware, cfg *config.Config) {
    g := r.Group("/group"); g.Use(authMw.JWTAuthentication())
    g.Get("/", h.WhatsappMessageHandler.ListGroups)      // existing read
    g.Get("/info", h.WhatsappMessageHandler.GetGroupInfo) // existing read
    if !cfg.GroupManagementEnabled { return }             // mutations dark => 404
    g.Post("/", h.WhatsappMessageHandler.CreateGroup)
    g.Post("/leave", h.WhatsappMessageHandler.LeaveGroup)
    g.Post("/participants", h.WhatsappMessageHandler.UpdateGroupParticipants)
    g.Patch("/settings", h.WhatsappMessageHandler.SetGroupSettings)
    g.Patch("/name", h.WhatsappMessageHandler.SetGroupName)
    g.Patch("/topic", h.WhatsappMessageHandler.SetGroupTopic)
    g.Put("/photo", h.WhatsappMessageHandler.SetGroupPhoto)
    g.Delete("/photo", h.WhatsappMessageHandler.RemoveGroupPhoto)
}
```
`internal/router/router.go:157` → `initGroupRoutes(api, h, authMw, cfg)`.

**No Wire/provider/handler.go changes.**

---

## 4. Migration DDL
**None.** Group mutations are stateless (no gateway-owned tables; whatsmeow's own store already migrated via `container.Upgrade`). `internal/whatsapp/migration.go` untouched.

---

## 5. Config keys (`config/config.go` + `.env.example`)
```go
// Group management (PRD-3, phase E). Ban-risky bulk paths default OFF until pacing (#2).
GroupManagementEnabled         bool `mapstructure:"GROUP_MANAGEMENT_ENABLED" default:"true"`
GroupAddParticipantsEnabled    bool `mapstructure:"GROUP_ADD_PARTICIPANTS_ENABLED" default:"false"`
GroupMaxParticipantsPerRequest int  `mapstructure:"GROUP_MAX_PARTICIPANTS_PER_REQUEST" default:"256"`
```
`GROUP_INFO_CACHE_TTL_SECONDS` from the PRD is a **reads** concern; reads already use the generic `READ_QUERY_CACHE_TTL_SECONDS` cache — **do not add it** for this mutations feature group. `.env.example` gains the three keys above with the ban-safety comment.

---

## 6. openapi.yaml delta (bump version)
Add paths (all `security: [BearerAuth]`), request/response bodies referencing new schemas:
- `POST /group` → 201 `CreateGroupResponse`; 400/401/403/404/409/429.
- `POST /group/leave` → 200; body `GroupTargetRequest`.
- `POST /group/participants` → 200 `GroupParticipantsResponse`; body `GroupParticipantsRequest` (`action` enum `[add,remove,promote,demote]`); 403 when add gated/non-admin, 400 over cap/empty/self.
- `PATCH /group/settings` → 200 `GroupSettingsResponse`; body `GroupSettingsRequest`.
- `PATCH /group/name` → 200; body `{chat,name}`; 400 when name>25.
- `PATCH /group/topic` → 200; body `{chat,topic}`.
- `PUT /group/photo` → 200 `GroupPhotoResponse`; `multipart/form-data` `{chat,photo(binary)}`; 400 non-JPEG/oversize.
- `DELETE /group/photo` → 200 `{removed:true}`.

New schemas: `CreateGroupRequest`, `CreateGroupResponse`, `GroupParticipantsRequest`, `GroupParticipantsResponse`, `ParticipantResult` (`status` enum `[ok,invited,failed]`, `invite` = `{code,expires_at}`), `GroupSettingsRequest`, `GroupSettingsResponse`, `SetGroupNameRequest`, `SetGroupTopicRequest`, `GroupTargetRequest`, `GroupPhotoResponse`. Add reusable `429` response. Also update `docs/ui/assets/docs/*`, `llms.txt` (endpoint family + "group ops require an explicit `@g.us`; bulk add & mass-join gated" note), `wiki/Group-Management.md` (new), `wiki/Environment-Variables.md`, `wiki/Security-Considerations.md` (ban gates default-off), `CLAUDE.md` API list + Configuration list, `CHANGELOG.md`.

---

## 7. Edge cases, partial-failure, gating

| Case | Handling |
|---|---|
| Non-admin mutation | server IQ 401/403 → `ErrForbidden` 403 "account is not an admin of this group". `LeaveGroup` & reads stay allowed. |
| Add privacy-blocked/non-contact | participant returned with `Error!=0 + AddRequest` → `status:"invited"` + invite; overall **200**. Not yet a member. |
| Promote/demote a non-member | participant `Error` (404/400) → `status:"failed"` + code; overall **200**. Never fail the batch. |
| Caller lists own number in remove/promote/demote | usecase PN self-guard → 400 "use POST /group/leave to remove yourself". LID form caught defensively in Client via `cli.Store.LID`. |
| Over cap / huge list | usecase 400 before server (`GROUP_MAX_PARTICIPANTS_PER_REQUEST`). Server overflow IQ 419 → 429. |
| `action=add` (and add-on-create) while `GROUP_ADD_PARTICIPANTS_ENABLED=false` | **403 "adding participants is disabled"** in usecase, before server. remove/promote/demote/settings/name/topic/photo/leave stay enabled. |
| Whole `/group` surface with `GROUP_MANAGEMENT_ENABLED=false` | mutation routes not registered → **404** (reads stay up). |
| Non-JPEG / oversize photo | usecase pre-check `image/jpeg` + `MaxUploadBytes` → 400; server `ErrInvalidImageFormat` (IQ 406) also → 400. |
| Name>25 / topic>512 | usecase 400 (explicit limit) before server 406. |
| Bare number / user JID as group | `resolveGroupJID` requires `@g.us` → 400; never retargets a user. |
| Empty participants (non-create) / duplicates | 400 on empty (participants endpoint); dedupe so results map 1:1. Create allows empty (group of just the account). |
| Create with unreachable members | `CreateGroup` still 201; `results[]` carry per-member invited/failed. |
| Settings with no flags | 400 "no settings supplied". Multi-flag: applied sequentially; first IQ error returned (partial-apply possible — documented, best-effort). |

**Gated paths (default-off):** `POST /group/participants action=add` and add-on-create (`GROUP_ADD_PARTICIPANTS_ENABLED=false`); entire mutation surface (`GROUP_MANAGEMENT_ENABLED`, though default true). Mass-join / invite links are not in this feature group.

---

## 8. Unit-test seams (no live client — `uc := &WhatsappMessageUsecase{config: &config.Config{...}}`)
Follow the existing `interaction_usecase_test.go` pattern (assert 400/403 fires **before** the nil manager is touched):
- **`resolveGroupJID`** (table): accepts `@g.us` (lowercased server, device suffix stripped); rejects bare digits (would become `@s.whatsapp.net`), user JID, `@lid`, empty.
- **`resolveParticipants`**: bare number → `@s.whatsapp.net`; dedupes; empty→400 when `allowEmpty=false`; group JID participant→400; PN self-guard rejects `ownPhone` for `selfGuard=true`, allows it for add; `>GroupMaxParticipantsPerRequest`→400.
- **Add-gate**: `UpdateGroupParticipants{action:"add"}` with `GroupAddParticipantsEnabled=false`→403; with `true` proceeds past the gate (nil-manager panic acceptable, or assert it got past). Same for `CreateGroup` with non-empty participants.
- **`participantResults` mapper** (pure, in `internal/whatsapp`): construct `[]types.GroupParticipant` with `Error==0` / `Error=403 + AddRequest` / `Error=404, AddRequest=nil` → assert `ok` / `invited`(+invite code,expires) / `failed`(+code); assert a mixed slice yields all three (the "mixed outcome is one response" contract).
- **`mapWhatsmeowErr` IQ mapping** (in `internal/whatsapp`): feed `&whatsmeow.IQError{Code:401/403/404/406/419/423}` and `whatsmeow.ErrInvalidImageFormat`; assert the domain sentinel each maps to. Possible only because mapping is `errors.As`/`errors.Is` on `IQError.Code` + sentinels (per ADR-0002), never substring.
- **Name/topic limits & settings**: name>25→400; topic>512→400; empty settings→400.
- Ban-safety limiter wiring has no live surface — these gate/cap/self-guard unit tests are the only guardrail (per PRD Testing Decisions). The exact live IQ codes (non-admin 401 vs 403, over-cap 419) should be verified once in a manual harness, then pinned.

**Uncertain / verify-once-live:** whether server returns IQ **401 vs 403** for a non-admin mutation (both map to 403, so safe either way); whether over-limit is **419 vs 429** (both → 429). The per-participant `Error` integer codes (403 privacy-invite vs 404 not-found) are inferred from `parseParticipant` (`group.go:687-697`) reading the raw `error` attr — confirm the exact integers against a live add before locking test expectations.