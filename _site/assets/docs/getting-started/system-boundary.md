# System Boundary

## Purpose
This document defines the architectural and operational boundaries of the Whatsapp Gateway. It clarifies responsibilities, integration expectations, and system limitations to ensure predictable deployment and integration behavior.

Clearly defining system boundaries prevents architectural misuse and reduces operational risk.

## Architectural Position

Whatsapp Gateway acts as an intermediary service between:

- A backend application (business logic layer)
- The WhatsApp network (via WhatsApp Web multi-device protocol)

The gateway is not a full application server and does not implement business workflows. It is a transport and session management layer.

## Responsibilities of the Gateway

The gateway is responsible for:

- Managing WhatsApp device sessions
- Maintaining authenticated connection to WhatsApp
- Sending outbound messages (text and supported media)
- Receiving inbound messages and media
- Enforcing outbound rate limits
- Optional queue-based outbound processing (RabbitMQ)
- Tracking message delivery status
- Persisting session and operational state in the database
- Delivering webhook events to backend systems
- Exposing health and device status endpoints

## Responsibilities of the Backend System

The backend system that integrates with the gateway is responsible for:

- Business logic and application workflows
- Authorization and user-level access control
- Data persistence beyond gateway operational needs
- Webhook endpoint implementation
- Webhook signature validation (HMAC verification)
- Idempotency management (if required by the business domain)
- Long-term message archival (if required)

The gateway does not replace a backend application.

## What the Gateway Is Not

The gateway does not:

- Provide a frontend user interface
- Implement end-user authentication flows
- Guarantee message delivery by WhatsApp
- Provide SLA guarantees
- Replace application-level retry or idempotency logic
- Serve as a multi-channel messaging abstraction layer

It is exclusively designed for WhatsApp integration.

## Deployment Boundary

Recommended deployment model:
Client Application 
    |
    ▼
Backend Service (Business Logic)
    |
    ▼
WhatsApp Gateway (Session Management & Transport)
    |
    ▼
WhatsApp Network

### Recommended Network Model

- Deploy the gateway within a private network.
- Expose only the backend service to public traffic.
- Protect gateway endpoints with JWT authentication.
- Validate webhook payload integrity using HMAC signature verification.

External exposure is technically possible but should be carefully controlled.

## Persistence Boundary

The gateway uses a database for:

- WhatsApp session persistence
- Device state
- Message status tracking
- Operational metadata

Supported databases:

- PostgreSQL (recommended for production)
- SQLite (development or lightweight deployment)

The database is considered a required component of the system.

## Queue Boundary (Optional Component)

RabbitMQ integration is optional.

When enabled:

- Outbound messages are queued.
- Rate limiting is enforced through controlled dispatch.
- Automatic retry behavior is supported.

When disabled:

- Outbound requests are processed immediately.
- If rate limits are exceeded, the request is rejected.
- No broker-based buffering occurs.

## Trust Boundary

Security assumptions:

- The gateway trusts authenticated backend services.
- Webhook receivers must verify request signatures.
- JWT authentication must be enforced for all protected endpoints.
- Secrets (JWT signing key, HMAC key, database credentials) must be securely managed.

The gateway should not be treated as a public-facing service without proper authentication and network isolation.

## Operational Boundary

The gateway provides:

- Server health endpoint
- WhatsApp logged-in status endpoint
- Controlled retry mechanism for webhook delivery

The gateway does not provide:

- SLA commitments
- High-availability guarantees (single-instance default architecture)
- Distributed clustering (future roadmap)

## Future Architectural Direction

While currently designed for single-instance deployment, the long-term roadmap includes:

- Horizontal scalability
- Distributed processing architecture
- Improved worker isolation
- MCP integration support

Current documentation reflects the present architecture and should not assume distributed behavior unless explicitly stated.
