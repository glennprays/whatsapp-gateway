# Group & Community Management

Manage WhatsApp groups and communities over REST. This surface is the
**highest-ban-risk** part of the gateway, so the ban-prone bulk vectors ship
**gated OFF by default** — read the [Ban-safety gates](#ban-safety-gates) section
before enabling anything.

> All endpoints are under the API base path (`/api/...`), require a JWT, and take
> the recipient as `chat` in the body/query (the deprecated `msisdn` alias is
> still accepted; `chat` wins). **Every group op requires an explicit `@g.us`
> JID** — a bare number or user JID is a `400`; a group op never lands on a user.

## Ban-safety gates

| Env var | Default | Effect |
|---|---|---|
| `GROUP_MANAGEMENT_ENABLED` | `true` | Master toggle. When `false` the entire mutation/invite/join-request/community surface is **unregistered → 404**. Reads stay up. |
| `GROUP_ADD_PARTICIPANTS_ENABLED` | `false` | Gates **bulk participant add** (`action=add` and add-on-create) → `403` when off. Other actions stay enabled. |
| `GROUP_JOIN_VIA_LINK_ENABLED` | `false` | Gates `POST /group/join` (mass-join) → `403` when off. |
| `GROUP_MAX_PARTICIPANTS_PER_REQUEST` | `256` | Caps a batch; over-cap → `400`. `0` disables the cap. |

## Partial-failure semantics

Batch mutations (`participants`, `requests` approve/reject, and `create`) return
**HTTP 200 with a `results[]` array**, never an overall error for one bad member.
Each entry:

| `status` | Meaning |
|---|---|
| `ok` | Applied. |
| `invited` | Privacy-blocked / non-contact add — converted to an invite (`invite.code`, `invite.expires_at`). **Not yet a member.** |
| `failed` | Hard per-participant failure (e.g. promoting a non-member); `code` is the whatsmeow error code. |

Removing / promoting / demoting **your own** number is a `400` — use `POST /group/leave`.

## Group endpoints

### Reads (always available)
- `GET /group/` — list joined groups (cached + per-account read budget).
- `GET /group/info?chat=<@g.us>` — one group's full detail + participant roster.

### Mutations (gated by `GROUP_MANAGEMENT_ENABLED`)
- `POST /group/` — create a group or community. Body: `{name, participants[], is_community?, linked_parent_jid?, is_announce?, is_locked?, is_join_approval_required?}`. `201`. Add-on-create is gated by `GROUP_ADD_PARTICIPANTS_ENABLED`.
- `POST /group/leave` — leave a group. Body: `{chat}`. Allowed for non-admins.
- `POST /group/participants` — Body: `{chat, action, participants[]}`, `action` ∈ `add|remove|promote|demote`. `add` is gated. `200` partial success.
- `PATCH /group/settings` — Body: `{chat, announce?, locked?}` (at least one). `announce` = only admins can send; `locked` = only admins can edit info.
- `PATCH /group/name` — Body: `{chat, name}`, name ≤ 25 chars.
- `PATCH /group/topic` — Body: `{chat, topic}`, topic ≤ 512 chars (empty clears it).
- `PUT /group/photo` — `multipart/form-data` `{chat, photo}`. Photo must be a JPEG (size-capped by `MAX_UPLOAD_BYTES`).
- `DELETE /group/photo` — clears the picture. `{chat}` in body or `?chat=`.

### Invite links
- `GET /group/invite?chat=<@g.us>` — get the invite link (admin only). Not cached.
- `POST /group/invite/reset` — Body: `{chat}`. Revoke + regenerate the link.
- `GET /group/invite/info?code=` — preview a group from an invite link/code **without joining**. `410` if the link was revoked, `400` if malformed.
- `POST /group/join` — Body: `{code}`. Join via link. **Gated by `GROUP_JOIN_VIA_LINK_ENABLED` (403 when off).** The returned JID is either the group or a membership-approval request (whatsmeow does not expose which).

### Join requests (approval-required groups)
- `GET /group/requests?chat=<@g.us>` — list pending join requests (admin only; cached + budgeted).
- `POST /group/requests` — Body: `{chat, action, participants[]}`, `action` ∈ `approve|reject`. `200` partial success.

## Community endpoints

### Reads (always available)
- `GET /community/subgroups?chat=<@g.us>` — list a community's linked sub-groups.
- `GET /community/participants?chat=<@g.us>` — all participants across linked groups.

### Mutations (gated by `GROUP_MANAGEMENT_ENABLED`)
- `POST /community/subgroups` — Body: `{chat, child_jid}`. Link a sub-group under a parent community (admin on both). `409` if already linked / the child is itself a community.
- `DELETE /community/subgroups?chat=&child=` — unlink a sub-group.

## Error mapping

| Condition | Status |
|---|---|
| Management disabled / group not found | `404` |
| Not a group JID / bad name / over cap / self in remove | `400` |
| Not an admin / add gated / join gated | `403` |
| Already linked / group locked | `409` |
| Invite link revoked | `410` |
| Action budget or server rate limit | `429` |

The exact server IQ codes for non-admin (401 vs 403) and over-limit (419 vs 429)
both map to `403` / `429` respectively, so the behaviour is stable either way.

See also: [Environment Variables](Environment-Variables), [Security Considerations](Security-Considerations).
