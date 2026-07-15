# Design Principles

## Purpose

This document defines the core engineering principles guiding the design and evolution of Whatsapp Gateway. These principles serve as architectural constraints and decision-making references for contributors and maintainers.

They ensure long-term consistency, predictability, and production readiness.

## 1. Separation of Concerns

Business logic must remain outside the gateway.

The gateway is responsible exclusively for:

- WhatsApp session lifecycle management
- Message transport
- Delivery tracking
- Rate limiting
- Queue orchestration (optional)
- Webhook dispatching

All domain-specific logic, user management, and workflow orchestration must be implemented in the integrating backend service.

## 2. Stateless API Layer

The HTTP API layer is designed to remain stateless.

All persistent state is stored in the database or managed through the WhatsApp session mechanism. This allows:

- Predictable behavior
- Future horizontal scaling
- Clear request-response semantics

Each API request is treated as a new operation and is not implicitly deduplicated.

## 3. Explicit Rate Control

Outbound communication must respect WhatsApp rate constraints.

The system enforces rate limiting through:

- Controlled pacing in direct dispatch mode (requests are briefly blocked, then rejected with a 429 only when pacing limits are exceeded)
- Controlled dispatch when queue mode is enabled

Rate handling must be deterministic and observable.

## 4. Optional Queue-Driven Processing

RabbitMQ integration is optional but architecturally supported.

When enabled:

- Outbound messages are buffered
- Dispatch is controlled
- Retry behavior is centralized

When disabled:

- The gateway operates in direct dispatch mode
- Backpressure is enforced by the outbound pacer, which paces (blocks) requests by default and rejects them (HTTP 429) only after the configured max-wait deadline

This design allows flexible deployment models without altering the API contract.

## 5. Database-Backed State Persistence

Operational state is persisted in a database.

This includes:

- WhatsApp session information
- Device status
- Message metadata
- Delivery tracking

PostgreSQL is recommended for production-grade reliability.
SQLite is suitable for development or lightweight environments.

## 6. Secure by Default

Security mechanisms are built into the core architecture:

- JWT-based API authentication
- HMAC-based webhook signature verification
- No implicit trust for external systems
- Explicit separation between backend and gateway

The gateway should not rely solely on network trust.

## 7. Operational Observability

The system must provide visibility into:

- Server health status
- WhatsApp login state
- Message processing state
- Retry behavior

Operational transparency reduces debugging time and improves system reliability.

## 8. Production-Oriented Simplicity

The gateway is designed to be:

- Lightweight
- Deterministic
- Container-friendly
- Infrastructure-compatible

Complexity is introduced only when operationally justified.

## 9. Single Responsibility Scope

The gateway focuses exclusively on WhatsApp integration.

It does not aim to:

- Become a multi-channel messaging abstraction
- Replace business orchestration systems
- Provide frontend application features

Scope discipline ensures long-term maintainability.

## 10. Evolution Toward Distributed Architecture

While currently optimized for single-instance deployment, the architecture is structured to evolve toward:

- Horizontal scalability
- Distributed worker models
- Cluster-ready database usage
- MCP integration

Future changes must preserve backward compatibility at the API level whenever possible.
