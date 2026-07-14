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
| Read Query Budget              | Yes       | Server-hitting reads metered per account (`READ_QUERY_*`); cache hits are free, `429` when the budget is spent |

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
