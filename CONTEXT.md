# WhatsApp Gateway — Context

Glossary for the WhatsApp Gateway: a backend-internal service that abstracts WhatsApp Web (whatsmeow) behind a REST API so stateless backends can integrate without managing sessions or device state. This file is a glossary only — no implementation details.

## Language

### Identity & addressing

**Account**:
The WhatsApp identity the gateway manages on a backend's behalf — one paired device / one whatsmeow client, keyed by its phone number. The *sender* of outbound messages. Distinct from the recipient.
_Avoid_: User, session (session is the connection state of an Account, not the Account itself)

**Chat**:
The canonical recipient / conversation target of a message — either a 1:1 chat (a phone number / PN JID) or a group (a `@g.us` JID). Requests express it as the `chat` field.
_Avoid_: To, recipient, destination, msisdn (msisdn is only one way to express a 1:1 Chat)

**MSISDN**:
An international phone number in bare digits (e.g. `628123…`). A convenience input that resolves to a PN JID; a way to name a 1:1 Chat, not a general recipient. Also the key by which an Account is registered.
_Avoid_: Number, phone (when precision matters)

**JID**:
WhatsApp's addressing identifier, `user@server`. Three servers matter here: `@s.whatsapp.net` (PN — a phone-number address), `@g.us` (a group), `@lid` (a LID).
_Avoid_: Address, id (too generic)

**LID**:
An opaque privacy identifier (`@lid`) for a party whose phone number is not exposed. May be unresolvable to a PN; consumers must not treat a LID as a callable phone number.

**Addressing mode**:
A tag on inbound identities — `"pn"` (a callable phone-number address was resolved) or `"lid"` (only an opaque LID was available). Already present on inbound webhooks.

### Groups & community

**Group**:
A multi-party Chat (`@g.us` JID). The Account participates in it and may or may not be an admin of it; admin status gates every group mutation.

**Community**:
A parent container that links multiple Groups (subgroups) under one umbrella. A Community is itself a Group-shaped entity (`@g.us`) flagged as a parent.

**Participant result**:
The per-member outcome of a participant operation — `ok` (applied), `invited` (WhatsApp converted an add into an invite because the target blocks adds / isn't a contact; not yet a member), or `failed` (with a code). A participant operation returns 200 with an array of these; partial failure is not a request failure.

### Reliability & operator plane

**Query budget**:
A lightweight per-Account rate allowance for the four server-hitting reads (contact info, avatar, joined groups, group info), consumed only on a cache miss. Distinct from send pacing.

**Idempotency key**:
A caller-supplied `Idempotency-Key` on a send, scoped `(Account, key)`, that makes a retried send return the original result instead of sending twice. Guarantees enqueued-once, not delivered-once, in queue mode.

**Operator plane**:
The admin surface (`/admin/*`, `/metrics`) authenticated by `ADMIN_API_SECRET` and separate from the per-Account tenant JWT. A tenant must never see another tenant.
_Avoid_: Admin API (fine informally), management API
