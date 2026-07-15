# Feature Matrix

## Overview

This section provides a structured summary of supported capabilities in the current version of Whatsapp Gateway.

The matrix is intended to give solution architects and integrators a clear view of functional coverage.

## Core Messaging Capabilities

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Send Text Message              | Yes       | Via REST API |
| Send Image Media               | Yes       | View-once supported |
| Send Audio / Voice Note        | Yes       | `is_ptt` renders the voice-note bubble |
| Send Video                     | Yes       | Caption, GIF flag, view-once |
| Send Document / File           | Yes       | Any mimetype; caption + file name |
| Send Location / Poll / Sticker | Yes       | Static location, polls, stickers |
| React / Edit / Delete          | Yes       | Message actions |
| Mark as Read (blue ticks)      | Yes       | `POST /message/read`; `sender` required for groups; outbound pacer |
| Typing Indicator               | Yes       | `POST /chat/presence` (composing / recording / paused); outbound pacer |
| Recipient Validation           | Yes       | `GET /contact/check` (IsOnWhatsApp) |
| Receive Incoming Text          | Yes       | Delivered via webhook |
| Receive Incoming Media         | Yes       | Image/audio/video/document/sticker/contact/location/poll |
| Delivery Status Tracking       | Yes       | Status updates available via webhook |
| Message Retry (Outbound)       | Yes       | Controlled by queue mode and retry policy |

## Directory & Read Surface

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| List Contacts                  | Yes       | `GET /contact/` — locally-synced address book, paginated (`limit`/`offset`); local read, never 404 on empty |
| Contact Profile Info           | Yes       | `GET /contact/info?chat=` — status/picture-id/verified-name/device-count; server read, cached + budgeted |
| Avatar / Profile Picture       | Yes       | `GET /contact/avatar?chat=` — user or group picture URL+id; `ETag`/`If-None-Match` → `304`; `404` none / `403` hidden |
| List Joined Groups             | Yes       | `GET /group/` — lightweight group summaries; server read, short-TTL cached + per-account read budget |
| Group Detail + Roster          | Yes       | `GET /group/info?chat=<@g.us>` — full detail + participants; `403` if not a member, `404` if absent |
| Community Sub-groups           | Yes       | `GET /community/subgroups?chat=<@g.us>` — a community's linked groups; empty list if none; server read, cached + budgeted |
| Community Participants         | Yes       | `GET /community/participants?chat=<@g.us>` — all participants across the community's linked groups; server read, cached + budgeted |
| Read Query Budget              | Yes       | Server-hitting reads metered per account (`READ_QUERY_*`); cache hits are free, `429` when the budget is spent |

## Group & Community Management

All mutations require an explicit `@g.us` JID and are gated by `GROUP_MANAGEMENT_ENABLED` (routes unregistered → `404` when off). Batch mutations return `200` with per-participant `results[]` (partial success). See [Group & Community Management](getting-started/group-management) for the full guide.

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Create Group / Community       | Yes       | `POST /group/` — add-on-create gated by `GROUP_ADD_PARTICIPANTS_ENABLED` |
| Leave Group                    | Yes       | `POST /group/leave` — allowed for non-admins |
| Participants add/remove/promote/demote | Yes | `POST /group/participants` — `add` gated (`403` off); `200` partial success; self → `400` |
| Group Settings                 | Yes       | `PATCH /group/settings` — announce / locked |
| Group Name / Topic             | Yes       | `PATCH /group/name` (≤25), `PATCH /group/topic` (≤512) |
| Group Photo                    | Yes       | `PUT /group/photo` (multipart JPEG) / `DELETE /group/photo` |
| Invite Link                    | Yes       | `GET /group/invite`, `POST /group/invite/reset`, `GET /group/invite/info?code=` (preview; `410` if revoked) |
| Join via Link                  | Yes       | `POST /group/join` — **gated by `GROUP_JOIN_VIA_LINK_ENABLED` (403 off)** |
| Join Requests                  | Yes       | `GET`/`POST /group/requests` — list / approve-reject (partial success) |
| Community Link / Unlink        | Yes       | `POST`/`DELETE /community/subgroups` — link/unlink a sub-group |

## Rate Limiting & Queue

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Outbound Rate Limiting         | Yes       | Per-phone message pacing |
| Register Rate Limiting         | Yes       | Per-IP throttle on `/register` (5/min default) |
| Media Upload Cap               | Yes       | `MAX_UPLOAD_BYTES` (16 MiB default) + per-kind MIME allow-lists |
| Queue Mode (RabbitMQ)          | Optional  | Toggle-based activation |
| Immediate Reject on Limit      | Yes       | When queue mode disabled |
| Buffered Dispatch              | Yes       | When queue mode enabled |
| Configurable Max Retry         | Yes       | For webhook delivery |
| Send Idempotency               | Yes       | Optional `Idempotency-Key` header on `/message/*`; DB-backed replay (`409` in-flight, `422` body mismatch); enqueued-once in queue mode |

## Database Support

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| PostgreSQL                     | Yes       | Recommended for production |
| SQLite                         | Yes       | Suitable for development or lightweight deployment |
| Session Persistence            | Yes       | Stored in database |
| Message Status Persistence     | Yes       | Stored in database |

## Security

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| JWT Authentication             | Yes       | Required for protected endpoints |
| Webhook HMAC Verification      | Yes       | Client must validate signature |
| Internal Network Deployment    | Recommended | Best practice model |
| External Exposure              | Supported | Requires proper authentication |

## Operational Capabilities

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Health Endpoint                | Yes       | Server operational check |
| WhatsApp Login Status Endpoint | Yes       | Device connection state |
| Webhook Retry Mechanism        | Yes       | Configurable retry behavior |
| SLA Guarantee                  | No        | No formal SLA provided |

## Deployment Model

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Docker Deployment              | Yes       | Primary distribution method |
| Native Binary Build            | Yes       | Manual build required |
| Horizontal Scaling             | Planned   | Future roadmap |
| Distributed Cluster            | Planned   | Long-term direction |
| Multi-Channel Messaging        | No        | WhatsApp only |

## Contribution Model

- Open-source project
- External contributions accepted
- All pull requests require review
- Architectural consistency is enforced
