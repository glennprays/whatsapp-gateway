# Component Overview

## Purpose

This document describes each architectural component of Whatsapp Gateway in detail, including its responsibilities, boundaries, and interaction patterns.

The goal is to provide a precise understanding of how internal components collaborate to deliver messaging functionality.

## HTTP API Layer

The HTTP API layer exposes RESTful endpoints used by backend systems.

Responsibilities:

- Accept inbound HTTP requests
- Validate JWT authentication
- Perform request validation and schema enforcement
- Invoke message processing logic
- Return structured JSON responses
- Expose health and status endpoints

Characteristics:

- Stateless request handling
- No session memory stored in the HTTP layer
- Deterministic response behavior

All persistent state changes are delegated to the database layer.

## Authentication Layer

Authentication is enforced using JWT.

Responsibilities:

- Validate token signature
- Validate token claims
- Reject unauthorized requests

The gateway does not manage user authentication flows. It assumes a trusted backend service issues or manages valid JWT tokens.

## Rate Limiter

The rate limiter controls outbound message throughput.

Responsibilities:

- Enforce sending limits
- Prevent excessive dispatch to WhatsApp
- Provide predictable backpressure behavior

Operational Modes:

Queue Enabled:
- Messages exceeding immediate dispatch capacity are buffered.
- Controlled release ensures compliance with configured rate.

Queue Disabled:
- Outbound requests are paced in-process by the outbound pacer (enabled by default), which blocks up to a configured maximum wait (OUTBOUND_PACE_MAX_WAIT_SECONDS, default 30s) before returning a 429.
- Immediate rejection occurs only when the pacer runs in reject mode or is disabled, in which case it falls back to the legacy per-message reject limiter.

Rate limiting is applied at the gateway instance level.

## Message Processing Engine

The message processing engine coordinates outbound and inbound message lifecycle handling.

Responsibilities:

- Validate outbound message payload
- Persist message metadata
- Determine dispatch strategy (queue or direct)
- Track delivery state
- Emit webhook events
- Handle retry policies

The engine operates using Go concurrency primitives within a single service process.

## Queue Integration (Optional)

RabbitMQ integration enables asynchronous outbound processing.

Responsibilities:

- Publish outbound message tasks
- Consume queued tasks via worker routines
- Coordinate retry logic
- Smooth rate spikes

RabbitMQ must be externally provisioned and managed.

If disabled, the system bypasses this component entirely.

## WhatsApp Session Manager

The session manager is responsible for maintaining an authenticated connection to WhatsApp via the WhatsApp Web multi-device protocol.

Responsibilities:

- QR-based login handling
- Persistent session storage
- Connection lifecycle management
- Automatic reconnection
- Receiving inbound messages and events
- Reporting device login status

Session credentials and state are stored in the database to survive restarts.

The gateway maintains device session state internally and exposes status through dedicated endpoints.

## Webhook Dispatcher

The webhook dispatcher delivers outbound events to backend systems.

Responsibilities:

- Send HTTP POST requests to configured webhook endpoint
- Sign payload using HMAC
- Implement retry mechanism
- Respect configured maximum retry limit

Webhook dispatch is asynchronous relative to API request handling.

The gateway does not guarantee webhook delivery beyond configured retry attempts.

## Database Layer

The database layer is mandatory.

Supported engines:

- PostgreSQL (recommended for production)
- SQLite (development or lightweight deployments)

Responsibilities:

- Persist WhatsApp session state
- Store outbound message metadata
- Track delivery status
- Inbound message metadata is NOT database-backed: received messages are held only in a per-account in-memory ring buffer (INCOMING_MESSAGE_BUFFER_SIZE, default 100), served via GET /api/message/incoming, and lost on restart
- Maintain operational records

The database is the source of truth for system state.

## Health & Status Endpoints

Operational visibility is provided through:

- Server health endpoint
- WhatsApp login status endpoint

These endpoints allow infrastructure monitoring systems to detect:

- Application readiness
- WhatsApp session connectivity
- Basic operational state

No advanced metrics system is built-in by default.

## Internal Concurrency Model

The gateway runs as a single service process.

Concurrency is achieved through:

- Goroutines
- Channels
- Internal worker routines

There are no separate microservices in the default architecture.

Queue consumers operate within the same runtime process.

## Current Architectural Limitations

- Single-instance architecture by default
- No distributed session coordination
- No cross-instance rate limit synchronization
- No cluster-aware message dispatch

These limitations are acknowledged and documented as part of the long-term roadmap.
