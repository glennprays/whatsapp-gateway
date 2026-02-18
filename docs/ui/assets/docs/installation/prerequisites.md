# Prerequisites

## Purpose

This document defines the minimum system and infrastructure requirements for deploying Whatsapp Gateway in development and production environments.

All production deployments should review these requirements carefully before installation.

## Runtime Requirements

Whatsapp Gateway is distributed primarily as a Docker container. Native binary execution is also supported.

Supported environments:

- Linux (recommended for production)
- macOS (development)
- Windows Subsystem for Linux (development)

Production deployment on Linux-based container infrastructure is strongly recommended.

## Go (Manual Build Only)

Minimum required version:

Go 1.25

Required only if:

- Building the binary manually
- Modifying source code
- Contributing to the project

Docker-based deployments do not require Go to be installed on the host system.

## Database

A relational database is required for operational state persistence.

Supported engines:

PostgreSQL  
Minimum version: 16  
Recommended for production deployments.

SQLite  
Suitable for development and lightweight deployments.

The database is mandatory. The gateway does not operate without persistent storage.

## Message Queue (Optional)

RabbitMQ is required only if queue mode is enabled.

Minimum version:

RabbitMQ 3.x

RabbitMQ must be provisioned and managed externally.

If queue mode is disabled:

- RabbitMQ is not required.
- Messages are dispatched immediately.
- Rate limit violations result in request rejection.

## Container Runtime (Recommended)

For Docker-based deployments:

- Docker Engine (latest stable recommended)
- Docker Compose (optional but recommended for multi-service setup)

Container orchestration platforms such as Kubernetes can be used but are not required.

## Network Requirements

The gateway must have:

- Outbound network access to WhatsApp servers
- Connectivity to configured database
- Connectivity to RabbitMQ (if queue mode enabled)
- Connectivity to webhook endpoint

Production deployment should ensure:

- Stable outbound internet connectivity
- Controlled inbound access
- TLS-enabled webhook communication

## Storage Requirements

Persistent storage is required for:

- Database data
- Optional RabbitMQ persistence

Future Support:

Planned support for S3-compatible object storage will enable external media storage. This feature is not yet mandatory but should be considered in long-term infrastructure planning.

## Recommended Production Stack

Minimum recommended production components:

- Linux host
- Docker runtime
- PostgreSQL 16
- RabbitMQ 3.x (if queue mode enabled)
- Reverse proxy (e.g., Nginx) for TLS termination
- Private network deployment

## Resource Considerations

Resource requirements depend on:

- Number of active devices
- Outbound message volume
- Queue usage
- Webhook traffic

Baseline recommendation:

- 1 CPU core minimum
- 512MB–1GB RAM minimum
- Dedicated database instance for production workloads

Higher throughput deployments require proportional resource scaling.
