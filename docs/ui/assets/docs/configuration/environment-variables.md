# Environment Variables Reference

## Overview

Whatsapp Gateway is fully configured through environment variables.

There is no mandatory configuration file. All runtime behavior is controlled via environment variables that can be supplied through:

- Docker `-e` flags
- Docker Compose `env_file`
- System environment variables
- Process managers (e.g., systemd)

This document provides a complete reference of all supported configuration options.

## Core Configuration

### ENV

Environment mode.

Type: string  
Default: development  
Options: development, production  

Controls general runtime behavior and logging verbosity.

### PORT

HTTP server port.

Type: string  
Default: 3000  

Defines the internal port used by the gateway.

### BASE_PATH

Base path prefix for all routes.

Type: string  
Default: /  

Useful when running behind a reverse proxy under a subpath.

Example:

/api/v1

### HTTP_ORIGIN

CORS allowed origin.

Type: string  
Default: *  

Set explicitly in production to restrict cross-origin access.

## Documentation Configuration

### ENABLE_DOCUMENTATION

Enable Documentation UI.

Type: boolean  
Default: true  

Should be disabled in hardened production environments.

### DOCUMENTATION_USER

Documentation basic auth username.

Type: string  
Default: user  

### DOCUMENTATION_PASSWORD

Documentation basic auth password.

Type: string  
Default: password  

### Documentation_BASE_PATH

Documentation documentation path.

Type: string  
Default: /docs  

## Authentication Configuration

### JWT_SECRET

JWT signing secret.

Type: string  
Default: secret  

Must be changed in production.

### JWT_TOKEN_DURATION_MINUTES

JWT expiration duration.

Type: integer  
Default: 1440  

Defines how long access tokens remain valid.

### JWT_ISSUER

JWT issuer identifier.

Type: string  
Default: whatsapp-gateway  

### SECRET_KEY

Basic authentication secret key.

Type: string  
Default: secret  

Should be replaced in production.

## WhatsApp Configuration

### WHATSAPP_DATASTORE_TYPE

Datastore backend type.

Type: string  
Default: sqlite3  
Options: sqlite3, postgres  

SQLite is suitable for development.  
PostgreSQL is recommended for production.

### WHATSAPP_DATASTORE_URI

Datastore connection URI.

Type: string  

SQLite default:

file:dbs/whatsapp.db?_foreign_keys=on  

PostgreSQL example:

postgres://user:password@host:5432/dbname?sslmode=disable  

### WHATSMEOW_LOG_LEVEL

Log level for underlying WhatsApp client library.

Type: string  
Default: warn  

Controls verbosity of WhatsApp client internals.

### WHATSAPP_DEVICE_LABEL

Device label shown in WhatsApp linked devices.

Type: string  
Default: WhatsApp Gateway  

### WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY

HMAC secret for webhook signature generation.

Type: string  
Default: 32-character placeholder  

Must be a strong 32-byte secret in production.

## Logging Configuration

### LOG_LEVEL

Application log level.

Type: string  
Default: debug  
Options: debug, info, warn, error  

Use info or warn in production.

### LOG_OUTPUT

Log output target.

Type: string  
Default: stdout  
Options: stdout, file  

### LOG_FILE_PATH

Log file path (if LOG_OUTPUT=file).

Type: string  
Default: /var/log/whatsapp-gateway.log  

### LOG_ENABLE_CALLER

Enable caller information in logs.

Type: boolean  
Default: false  

Useful for debugging.

## Rate Limiting

### MESSAGE_RATE_LIMIT_PROVIDER

Rate limit backend.

Type: string  
Default: memory  
Options:
- memory
- redis
- noop

Memory is per-instance.  
Redis allows distributed rate limiting.

### MESSAGE_RATE_LIMIT_REQUESTS

Allowed requests per window.

Type: integer  
Default: 100  

### MESSAGE_RATE_LIMIT_DURATION_SECONDS

Window duration in seconds.

Type: integer  
Default: 60  

## RabbitMQ Configuration

### RABBITMQ_ENABLED

Enable queue mode.

Type: boolean  
Default: false  

When enabled, outgoing messages are processed asynchronously.

### RABBITMQ_URL

RabbitMQ connection URI.

Type: string  
Default: amqp://user:user@localhost:5672/  

### RABBITMQ_CONNECTION_NAME

Connection name identifier.

Type: string  
Default: whatsapp-gateway  

### RABBITMQ_PREFETCH_COUNT

Consumer prefetch count.

Type: integer  
Default: 5  

Controls number of unacknowledged messages per worker.

## Redis Configuration

### REDIS_ENABLED

Enable Redis usage.

Type: boolean  
Default: false  

Required when using Redis-based rate limiting.

### REDIS_URI

Redis connection URI.

Type: string  
Default: redis://localhost:6379/0  

## Worker Pool Configuration

### WORKER_INCOMING_EVENTS

Number of workers handling incoming WhatsApp events.

Type: integer  
Default: 5  

### WORKER_WEBHOOK_DELIVERY

Number of workers delivering webhooks.

Type: integer  
Default: 10  

### WORKER_OUTGOING_MESSAGES

Number of workers processing outgoing messages.

Type: integer  
Default: 3  

Increasing worker count increases concurrency but also resource usage.

## Queue Retry Configuration

### QUEUE_MAX_RETRIES

Maximum retry attempts for failed queue messages.

Type: integer  
Default: 3  

Messages exceeding retry limit are marked as failed.

## Status Webhook Configuration

### WEBHOOK_STATUS_EVENTS_ENABLED

Enable status event webhooks.

Type: boolean  
Default: true  

### WEBHOOK_STATUS_EVENTS

Comma-separated list of status events.

Type: string  
Default: message.sent,message.failed  

Defines which message lifecycle events trigger webhooks.

## Storage Configuration

### STORAGE_PROVIDER

Storage backend provider.

Type: string
Default: local
Options:
- local: Filesystem-based storage (production-ready, full data control)
- s3: S3/S3-compatible object storage (AWS S3, MinIO, DigitalOcean Spaces, etc.)

Both providers are production-ready. Choose based on infrastructure needs.

### STORAGE_S3_ENDPOINT

S3/S3-compatible service endpoint.

Type: string
Default: s3.amazonaws.com

AWS S3: s3.amazonaws.com
MinIO: localhost:9000
DigitalOcean Spaces: nyc3.digitaloceanspaces.com

### STORAGE_S3_ACCESS_KEY_ID

S3 access key ID.

Type: string
Default: ""

Required for S3 provider. Can be omitted for local provider.

### STORAGE_S3_SECRET_ACCESS_KEY

S3 secret access key.

Type: string
Default: ""

Required for S3 provider. Can be omitted for local provider.

### STORAGE_S3_REGION

S3 region.

Type: string
Default: us-east-1

Required for S3 provider.

### STORAGE_S3_BUCKET

S3 bucket name.

Type: string
Default: whatsapp-gateway

Required for S3 provider. Bucket must exist or be creatable.

### STORAGE_S3_USE_SSL

Use SSL/TLS for S3 connections.

Type: boolean
Default: true

Should be true for production. May be false for local MinIO testing.

### STORAGE_S3_PRESIGNED_URL_EXPIRY_SECONDS

Presigned URL expiration time in seconds.

Type: integer
Default: 3600

Maximum validity of presigned URLs for accessing S3 files. URLs are returned in webhooks and automatically expire after this duration.

### STORAGE_LOCAL_PATH

Local filesystem storage path.

Type: string
Default: ./storage

For production with Docker, use a persistent volume or bind mount.
Example: /var/lib/whatsapp-gateway/storage

### STORAGE_BASE_URL

Base URL for public file access (local provider only).

Type: string
Default: ""

Optional. Used when serving local files via a web server.

Example: https://example.com/storage

### STORAGE_API_PATH

Path for serving files directly from the gateway.

Type: string
Default: /storage

Used for direct file serving via gateway HTTP endpoints.

Example: /storage

## Production Recommendations

For production deployments:

- Change all default secrets
- Use PostgreSQL instead of SQLite
- Enable Redis for distributed rate limiting if horizontally scaled
- Enable RabbitMQ for high throughput systems
- Disable Documentation or protect it properly
- Set LOG_LEVEL to info or warn
- Use strong HMAC secret (32+ random bytes)
- Restrict HTTP_ORIGIN
- Use persistent storage for local provider in production (Docker volumes)
- Enable SSL for S3 provider in production
- Use appropriate presigned URL expiry based on security requirements

Environment configuration directly impacts system reliability and security.  
Defaults are intended for development convenience, not hardened production.
