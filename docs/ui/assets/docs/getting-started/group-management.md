# Group & Community Management

Manage WhatsApp groups and communities over REST. This is the **highest-ban-risk**
surface, so the ban-prone bulk vectors now ship **enabled by default**, with outbound pacing and a ban gate supplying the ban-safety (each flag stays a kill-switch you can flip off).

All endpoints are under the API base path, require a JWT, and take the recipient
as `chat` in the body/query. **Every group op requires an explicit `@g.us` JID**:
a bare number or user JID is a `400`; a group op never lands on a user.

## Ban-safety gates

| Env var | Default | Effect |
|---|---|---|
| `GROUP_MANAGEMENT_ENABLED` | `true` | Master toggle. When `false` the whole mutation/invite/join-request/community surface is **unregistered → 404**. Reads stay up. |
| `GROUP_ADD_PARTICIPANTS_ENABLED` | `true` | Gates bulk participant add (`action=add` and add-on-create) → `403` when off. |
| `GROUP_JOIN_VIA_LINK_ENABLED` | `true` | Gates `POST /group/join` (mass-join) → `403` when off. |
| `GROUP_MAX_PARTICIPANTS_PER_REQUEST` | `256` | Caps a batch; over-cap → `400`. `0` disables. |

## Partial-failure semantics

Batch mutations (`participants`, `requests`, `create`) return **HTTP 200 with a
`results[]` array**, never an overall error for one bad member:

- `ok`: applied.
- `invited`: privacy-blocked / non-contact add converted to an invite (`invite.code`, `invite.expires_at`); **not yet a member**.
- `failed`: hard per-participant failure; `code` is the whatsmeow error code.

Removing / promoting / demoting **your own** number is a `400`: use `POST /group/leave`.

## Endpoints

### Group reads (always available)
- `GET /group/`: list joined groups.
- `GET /group/info?chat=<@g.us>`: one group's full detail + roster.

### Group mutations (gated by `GROUP_MANAGEMENT_ENABLED`)
- `POST /group/`: create a group or community (`201`). Body `{name, participants[], is_community?, linked_parent_jid?, is_announce?, is_locked?, is_join_approval_required?}`.
- `POST /group/leave`: leave a group (`{chat}`).
- `POST /group/participants`: `{chat, action, participants[]}`, `action` ∈ `add|remove|promote|demote`.
- `PATCH /group/settings`: `{chat, announce?, locked?}`.
- `PATCH /group/name`: `{chat, name}` (≤25).
- `PATCH /group/topic`: `{chat, topic}` (≤512; empty clears).
- `PUT /group/photo`: multipart `{chat, photo}` (JPEG).
- `DELETE /group/photo`: clear picture (`{chat}` or `?chat=`).
- `GET /group/invite?chat=` / `POST /group/invite/reset`: admin invite link.
- `GET /group/invite/info?code=`: preview without joining (`410` if revoked).
- `POST /group/join`: join via link (**gated by `GROUP_JOIN_VIA_LINK_ENABLED`**).
- `GET /group/requests?chat=` / `POST /group/requests`: list / approve-reject join requests.

### Community
- Reads (always): `GET /community/subgroups?chat=`, `GET /community/participants?chat=`.
- Mutations (gated): `POST /community/subgroups` (`{chat, child_jid}`), `DELETE /community/subgroups?chat=&child=`.

## Error mapping

| Condition | Status |
|---|---|
| Management disabled / group not found | `404` |
| Not a group JID / bad name / over cap / self in remove | `400` |
| Not an admin / add gated / join gated | `403` |
| Already linked / group locked | `409` |
| Invite link revoked | `410` |
| Action budget or server rate limit | `429` |
