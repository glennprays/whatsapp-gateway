I have everything needed. All signatures verified against real source. Here is the blueprint.

---

# Blueprint — Community + group READS (subgroups + community participants)

Scope: the two remaining Phase C group reads (PRD 3 rows 18 & 19). `GET /group` (joined) and `GET /group/info` already shipped and are the exact template. These reuse that machinery end-to-end.

---

## 1. Endpoints + DTOs

Two JWT-protected GETs. Recommend both under a new `/community` namespace (a "subgroup" only exists in a community context; matches PRD rows 18/19). The task's `GET /group/subgroups` also works with zero cost — pick one; I lead with `/community/*`.

| Method + path | Purpose | whatsmeow call |
|---|---|---|
| `GET /community/subgroups?chat=<@g.us>` | List the sub-groups linked under a community | `GetSubGroups` |
| `GET /community/participants?chat=<@g.us>` | List all participants across the community's linked groups | `GetLinkedGroupsParticipants` |

`chat` is the canonical addressing param (deprecated `msisdn` alias accepted for symmetry, but a community is never a bare number — resolver still requires `@g.us`). Both are cached + budgeted server reads → `429` when the per-account read budget is exhausted.

**New DTOs** (append to `domain/whatsapp/group.go`):

```go
// SubGroupItem is one linked group under a community (whatsmeow types.GroupLinkTarget).
type SubGroupItem struct {
    JID               string `json:"jid"`                  // the sub-group's @g.us JID
    Name              string `json:"name,omitempty"`
    IsDefaultSubGroup bool   `json:"is_default_sub_group"` // the community's default/announcement group
}

type SubGroupListResponse struct {
    SubGroups []SubGroupItem `json:"sub_groups"`
    Count     int            `json:"count"`
}

// CommunityParticipantItem is one member across the community's linked groups.
// whatsmeow returns only the primary JID (@lid or @s.whatsapp.net); no PN/LID split.
type CommunityParticipantItem struct {
    JID string `json:"jid"`
}

type CommunityParticipantsResponse struct {
    Participants []CommunityParticipantItem `json:"participants"`
    Count        int                        `json:"count"`
}
```

`SubGroupItem` maps 1:1 from `types.GroupLinkTarget{JID, GroupName.Name, GroupIsDefaultSub.IsDefaultSubGroup}`. `CommunityParticipantItem` is one field — PN/LID enrichment skipped (whatsmeow gives only the primary JID; enriching would need N store lookups over a potentially large cross-group list). → skipped: PN/LID enrichment, add when a caller needs correlation and the LID store lookup is cheap.

---

## 2. Exact whatsmeow calls (verified against source)

**`GetSubGroups`** — `group.go:547-568`:
```go
// GetSubGroups gets the subgroups of the given community.
func (cli *Client) GetSubGroups(ctx context.Context, community types.JID) ([]*types.GroupLinkTarget, error)
```

**`GetLinkedGroupsParticipants`** — `group.go:570-588`:
```go
// GetLinkedGroupsParticipants gets all the participants in the groups of the given community.
func (cli *Client) GetLinkedGroupsParticipants(ctx context.Context, community types.JID) ([]types.JID, error)
```

**Return type `types.GroupLinkTarget`** — `types/group.go:153-157`:
```go
type GroupLinkTarget struct {
    JID JID
    GroupName          // embeds: Name string, NameSetAt time.Time, NameSetBy JID, NameSetByPN JID
    GroupIsDefaultSub  // embeds: IsDefaultSubGroup bool
}
```

**Error sentinels — IMPORTANT divergence from the shipped reads.** Both `GetSubGroups` and `GetLinkedGroupsParticipants` return the **raw** result of `cli.sendGroupIQ(...)` — they do **not** call `wrapIQError(ErrGroupNotFound, …)` the way `getGroupInfo` (`group.go:632/634`) and the invite-link methods do. So on an error they return:
- `*whatsmeow.IQError` (`errors.go:174`) — code from the server's IQ `<error code="…">`. Sentinels for `errors.Is`: `ErrIQBadRequest` (400), `ErrIQNotAuthorized` (401), `ErrIQForbidden` (403), `ErrIQNotFound` (404), `ErrIQNotAcceptable` (406), `ErrIQGone` (410), `ErrIQResourceLimit` (419), `ErrIQRateOverLimit` (429) — `errors.go:183-195`.
- `*whatsmeow.ElementMissingError` (`errors.go:238`) — only if a genuinely malformed response omits the `sub_groups`/`linked_groups_participants` tag entirely (a real community with zero subgroups returns an empty `<sub_groups/>` → empty slice, not this error).

Consequence: the existing `mapWhatsmeowErr` (`client.go:147`) checks `errors.Is(err, whatsmeow.ErrGroupNotFound)` / `ErrNotInGroup` — those are `errors.New` sentinels that a **raw** `*IQError` will **not** match. Without a new branch, a not-a-community / not-found from these two calls falls through to `ErrInternalFailure` (500). Fix = one additive `errors.As(*IQError)` branch (§3).

**HTTP mapping** (via the new `errors.As(*IQError)` branch, applied to whatever code the server returns):

| whatsmeow error | Domain sentinel | HTTP |
|---|---|---|
| `*IQError{Code:400}` (`ErrIQBadRequest`) / 405 / 406 | `ErrBadRequest` | 400 |
| `*IQError{Code:401}` / `{403}` (`ErrIQForbidden`) | `ErrForbidden` | 403 |
| `*IQError{Code:404}` (`ErrIQNotFound`) | `ErrNotFound` | 404 |
| `*IQError{Code:419}` / `{429}` | `ErrTooManyRequests` | 429 |
| `*ElementMissingError`, other | `ErrInternalFailure` | 500 |
| `store.ErrDeviceDeleted` (already handled) | `ErrConflict` | 409 |

⚠ **Unverified per PRD Testing Decisions:** the *exact* IQ code the server returns for "valid `@g.us` group that is NOT a community/parent" is not knowable from source (400 bad-request vs 403 forbidden vs 404 item-not-found are all plausible). The generic `*IQError` switch maps all three sensibly, but the specific code must be verified once against live whatsmeow and pinned. Do **not** ship a substring guess (ADR 0002).

---

## 3. Ordered edit list

No new usecase struct, no new handler struct, no Wire/provider changes, no new file needed — these ride the existing `WhatsappMessageUsecase` + `WhatsappMessageHandler` exactly like `GetGroupInfo`/`ListGroups`.

**3a. `internal/whatsapp/client.go`** — add to the `Client` interface (after line 43, next to `GetGroupInfo`):
```go
ListSubGroups(ctx context.Context, traceID, phoneNumber, communityJID string) ([]waDomain.SubGroupItem, error)
ListCommunityParticipants(ctx context.Context, traceID, phoneNumber, communityJID string) ([]waDomain.CommunityParticipantItem, error)
```
Impl (mirror `GetGroupInfo` `client.go:390-437`): nil-client guard → 404 `ErrClientNotFound`; `!IsLoggedIn()` → 401; `types.ParseJID` + `jid.Server != types.GroupServer` → 400; call `cli.GetSubGroups(ctx, jid)` / `cli.GetLinkedGroupsParticipants(ctx, jid)`; on error `return c.mapWhatsmeowErr(traceID, phoneNumber, err)`; map result slices to DTOs (skip `nil` entries as `ListGroups` does at `client.go:366`). For participants: `items = append(items, waDomain.CommunityParticipantItem{JID: j.String()})`.

**Extend `mapWhatsmeowErr`** (`client.go:147`) — add **after** the `ErrGroupNotFound`/`ErrNotInGroup` block (line 167) and **before** `isRecipientError` (line 170), so wrapped sentinels still win for `getGroupInfo`:
```go
var iqErr *whatsmeow.IQError
if errors.As(err, &iqErr) {
    switch iqErr.Code {
    case 400, 405, 406:
        return errDomain.NewError(errDomain.ErrBadRequest, err)
    case 401, 403:
        return errDomain.NewError(errDomain.ErrForbidden, err)
    case 404:
        return errDomain.NewError(errDomain.ErrNotFound, err)
    case 419, 429:
        return errDomain.NewError(errDomain.ErrTooManyRequests, err)
    }
    // unknown code (0/500/...) falls through to ErrInternalFailure below
}
```
`whatsmeow` is already imported (`client.go:15`); `errors` and `errDomain` already in use. This is the root-cause single seam — it also covers every future group mutation the PRD adds (`mapIQError` in PRD becomes this branch; no separate helper needed → skipped: standalone `mapIQError`, add when IQ mapping diverges from `mapWhatsmeowErr`).

**3b. `internal/whatsapp/manager.go`** — add the two methods to the `Manager` interface (near line 56) and impl as logging pass-throughs (copy `ListGroups`/`GetGroupInfo` at `manager.go:555-577`): call `m.Client.ListSubGroups(...)`, on error `m.Logger.Error(traceID, "Failed to list subgroups", …, MaskedPhoneNumber(phoneNumber), Error(err))`, return.

**3c. `internal/usecase/whatsapp/read_usecase.go`** — first add a shared resolver (dedups the `@g.us` guard now inlined in `GetGroupInfo` at `read_usecase.go:87-94`):
```go
// resolveGroupJID resolves chat and requires an explicit @g.us group JID.
func resolveGroupJID(chat string) (string, error) {
    target, err := resolveChat(chat, "")
    if err != nil {
        return "", err
    }
    if !strings.HasSuffix(target, "@"+groupServerSuffix) {
        return "", errDomain.NewError(errDomain.ErrBadRequest,
            fmt.Errorf("chat must be a group/community JID (@g.us), got %q", target))
    }
    return target, nil
}
```
Refactor `GetGroupInfo` to call it, then add:
```go
func (uc *WhatsappMessageUsecase) ListSubGroups(ctx context.Context, traceID, phoneNumber, chat string) (*waDomain.SubGroupListResponse, error) {
    target, err := resolveGroupJID(chat)
    if err != nil { return nil, err }
    return queryWithBudget(uc, ctx, phoneNumber, "subgroups:"+phoneNumber+":"+target,
        func() (*waDomain.SubGroupListResponse, error) {
            items, err := uc.whatsappManager.ListSubGroups(ctx, traceID, phoneNumber, target)
            if err != nil { return nil, err }
            sort.Slice(items, func(i, j int) bool { return items[i].JID < items[j].JID })
            return &waDomain.SubGroupListResponse{SubGroups: items, Count: len(items)}, nil
        })
}

func (uc *WhatsappMessageUsecase) ListCommunityParticipants(ctx context.Context, traceID, phoneNumber, chat string) (*waDomain.CommunityParticipantsResponse, error) {
    target, err := resolveGroupJID(chat)
    if err != nil { return nil, err }
    return queryWithBudget(uc, ctx, phoneNumber, "communityparts:"+phoneNumber+":"+target,
        func() (*waDomain.CommunityParticipantsResponse, error) {
            items, err := uc.whatsappManager.ListCommunityParticipants(ctx, traceID, phoneNumber, target)
            if err != nil { return nil, err }
            sort.Slice(items, func(i, j int) bool { return items[i].JID < items[j].JID })
            return &waDomain.CommunityParticipantsResponse{Participants: items, Count: len(items)}, nil
        })
}
```
**Cache + budget:** both go through `queryWithBudget` (`query.go:58`) → cache hit is free, cache miss spends one `spendReadBudget` token → 429 when exhausted. **No `spendActionBudget`** (these are reads, not conversation actions). Distinct cache-key prefixes (`subgroups:` / `communityparts:`) keep them isolated per account+JID.

**3d. `internal/handler/whatsapp/whatsapp_message_handler.go`** — add `ListSubGroups` / `ListCommunityParticipants`, verbatim copies of `GetGroupInfo` (`handler:468-482`): `GetTraceID`, `MustGetPhoneNumber`, call usecase with `c.Query("chat")`, `httperror.FromError` on error, `c.Status(200).JSON(resp)`.

**3e. `internal/router/group_routes.go`** — extend `initGroupRoutes` (already called from `router.go:157`, so no `router.go` change):
```go
communityGroup := r.Group("/community")
communityGroup.Use(authMw.JWTAuthentication())
communityGroup.Get("/subgroups", h.WhatsappMessageHandler.ListSubGroups)
communityGroup.Get("/participants", h.WhatsappMessageHandler.ListCommunityParticipants)
```
(If you instead follow the task's `/group/subgroups` example, put that one line on `groupGroup` and only `/participants` on `communityGroup`.)

---

## 4. Migration DDL

**None.** Both are stateless server reads with no persistence. No `internal/whatsapp/migration.go` change.

---

## 5. New config keys

**None.** The shipped group reads (info/joined) already reuse `READ_QUERY_CACHE_TTL_SECONDS` / `READ_QUERY_BUDGET` / `READ_QUERY_WINDOW_SECONDS` via the single `queryCache`+`queryBudget` on `WhatsappMessageUsecase`; these two reads ride the same path. The PRD's proposed `GROUP_INFO_CACHE_TTL_SECONDS` and `GROUP_MANAGEMENT_ENABLED` were never adopted for the shipped reads — do not introduce them for reads alone (YAGNI; they belong to the gated mutation PRD if at all). → skipped: dedicated group-read cache TTL + master toggle, add when the mutation surface lands and needs its own gating.

---

## 6. openapi.yaml delta

Bump `info.version`. Under `paths:` (after `/group/info`, ~line 1134) add:

```yaml
  /community/subgroups:
    get:
      tags: [Community]
      summary: List a community's sub-groups
      description: >
        Lists the groups linked under a community (whatsmeow GetSubGroups, a
        server IQ). chat must be the community's @g.us JID. Short-TTL cached and
        metered by the per-account read budget (429 when exhausted). A chat that
        is a group but not a community returns 400/404.
      parameters:
        - { name: chat, in: query, required: true,
            description: "The community's JID (must be @g.us).",
            schema: { type: string, example: "120363000000000000@g.us" } }
      responses:
        '200': { description: The community's sub-groups.,
                 content: { application/json: { schema: { $ref: '#/components/schemas/SubGroupListResponse' } } } }
        '400': { description: chat missing/not a group JID, or not a community. }
        '403': { description: The account is not a participant/authorized. }
        '404': { description: The community does not exist. }
        '429': { description: Per-account read budget exhausted. }
      security: [ { accountBearerAuth: [] } ]
  /community/participants:
    get:
      tags: [Community]
      summary: List all community participants
      description: >
        Lists all participants across the community's linked groups (whatsmeow
        GetLinkedGroupsParticipants). Returns each participant's primary JID
        (@lid or @s.whatsapp.net). Cached + budgeted like other server reads.
      parameters:
        - { name: chat, in: query, required: true,
            description: "The community's JID (must be @g.us).",
            schema: { type: string, example: "120363000000000000@g.us" } }
      responses:
        '200': { description: The community's participants.,
                 content: { application/json: { schema: { $ref: '#/components/schemas/CommunityParticipantsResponse' } } } }
        '400': { description: chat missing/not a group JID, or not a community. }
        '403': { description: The account is not authorized. }
        '404': { description: The community does not exist. }
        '429': { description: Per-account read budget exhausted. }
      security: [ { accountBearerAuth: [] } ]
```

Under `components.schemas:` (after `GroupInfoResponse`, ~line 516):
```yaml
    SubGroupItem:
      type: object
      properties:
        jid: { type: string, example: "120363111111111111@g.us" }
        name: { type: string, example: "Announcements" }
        is_default_sub_group: { type: boolean, example: true }
    SubGroupListResponse:
      type: object
      properties:
        sub_groups: { type: array, items: { $ref: '#/components/schemas/SubGroupItem' } }
        count: { type: integer, example: 3 }
    CommunityParticipantItem:
      type: object
      properties:
        jid: { type: string, example: "1122334455@lid" }
    CommunityParticipantsResponse:
      type: object
      properties:
        participants: { type: array, items: { $ref: '#/components/schemas/CommunityParticipantItem' } }
        count: { type: integer, example: 42 }
```

---

## 7. Edge cases + partial-failure + gating

- **`chat` is a bare number / user JID / `@lid`** → `resolveGroupJID` rejects 400 before any server call (bare digits resolve to `@s.whatsapp.net`; never silently retargeted).
- **`chat` is a valid `@g.us` group but NOT a community** → raw `*IQError` from whatsmeow → new `errors.As` branch maps to 400 (bad-request) / 403 / 404 per the server code. **Exact code unverified** — pin once against live whatsmeow (PRD Testing Decisions). Because these two calls don't wrap in `ErrGroupNotFound`, this branch is the *only* thing preventing a 500 here.
- **Community exists, zero subgroups** → whatsmeow returns empty `<sub_groups/>` → `[]` → 200 `{sub_groups:[], count:0}`. Not an error (mirrors `ListContacts` never-404-on-empty posture).
- **Malformed server response (missing tag)** → `*ElementMissingError` → falls through to 500 (genuine protocol anomaly; correct).
- **No session / not logged in** → 404 `ErrClientNotFound` / 401 `client not logged in` (client-layer nil guards, identical to `GetGroupInfo`).
- **Read budget exhausted** → 429 from `queryWithBudget` cache-miss path.
- **Device deleted server-side** → `store.ErrDeviceDeleted` → 409 + client eviction (existing `mapWhatsmeowErr` head).
- **Partial failure:** N/A — these are atomic single-IQ reads, no per-item results.
- **Gating:** none. Reads are low ban-risk (PRD: "none (read)") and run under the shared read budget only, not `spendActionBudget` and not behind any default-off flag. No `GROUP_MANAGEMENT_ENABLED` gate for reads.

---

## 8. Unit-test seams (no live client)

- **`resolveGroupJID` / usecase guard** (`internal/usecase/whatsapp/query_test.go`): mirror `TestGetGroupInfo_RejectsNonGroupChat` (`query_test.go:73`) — table over `ListSubGroups`/`ListCommunityParticipants` with a bare number, user JID, `@lid`, device-suffixed JID → all `ErrBadRequest`, never touching the nil manager. `@g.us` passes the guard (will nil-panic past it, so stop at asserting the guard, or inject a stub manager). Refactoring `GetGroupInfo` onto `resolveGroupJID` is covered by the existing test — no regression risk.
- **`mapWhatsmeowErr` IQError mapping** (`internal/whatsapp/client_error_test.go`, alongside `TestIsRecipientError`): construct `&whatsmeow.IQError{Code:400}`, `{Code:403}`, `{Code:404}`, `{Code:419}`, wrap some via `fmt.Errorf("…: %w", …)`, assert `errDomain.Error.ServiceError()` == `ErrBadRequest`/`ErrForbidden`/`ErrNotFound`/`ErrTooManyRequests`. Also assert a `*wrappedIQError` carrying `ErrGroupNotFound` (like `getGroupInfo` returns) still maps to `ErrNotFound` — proves ordering (sentinel branch wins before the generic `*IQError` branch). Pure error mapping, no network.
- **Cache/budget** (`query_test.go`): the new cache-key prefixes ride the already-tested `queryWithBudget` (`TestQueryWithBudget_CachesAfterMiss` / `_ExhaustsBudget` / `_DoesNotCacheErrors`). One added case per read (distinct prefixes don't collide) is enough; the hit-bypasses-budget / miss-spends / error-not-cached invariants are already locked.
- **DTO mapping** (optional, `internal/whatsapp`): given a `[]*types.GroupLinkTarget` with a `nil` entry and a default-sub, assert the mapper skips nil and sets `IsDefaultSubGroup` — but this needs the mapper factored out of the client method to test without a live `cli`; low value, skip unless the mapping grows.

---

**Key load-bearing facts:** `GetSubGroups` (`group.go:548`) and `GetLinkedGroupsParticipants` (`group.go:571`) return **raw** `*whatsmeow.IQError`, unlike `getGroupInfo` which wraps in `ErrGroupNotFound`/`ErrNotInGroup` — so the existing `mapWhatsmeowErr` sentinel checks miss them and a not-a-community would 500 without the new `errors.As(*IQError)` branch. That branch is the one non-trivial code change; everything else is a copy of the shipped `GetGroupInfo` path. No migration, no new config, no Wire changes.