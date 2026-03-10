# Feature Matrix

## Overview

This section provides a structured summary of supported capabilities in the current version of Whatsapp Gateway.

The matrix is intended to give solution architects and integrators a clear view of functional coverage.

## Core Messaging Capabilities

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Send Text Message              | Yes       | Via REST API |
| Send Image Media               | Yes       | Media support currently limited to images |
| Receive Incoming Text          | Yes       | Delivered via webhook |
| Receive Incoming Media         | Yes       | Image media supported |
| Delivery Status Tracking       | Yes       | Status updates available via webhook |
| Message Retry (Outbound)       | Yes       | Controlled by queue mode and retry policy |

## Rate Limiting & Queue

| Capability                     | Supported | Notes |
|--------------------------------|-----------|-------|
| Outbound Rate Limiting         | Yes       | Enforced per gateway instance |
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
