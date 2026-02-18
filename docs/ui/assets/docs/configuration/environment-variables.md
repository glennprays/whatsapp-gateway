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

Environment configuration directly impacts system reliability and security.  
Defaults are intended for development convenience, not hardened production.
