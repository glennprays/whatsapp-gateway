# Authentication and Security

## Overview

Whatsapp Gateway is designed as a backend integration component.  
It must not be exposed publicly without proper authentication and network protection.

This document explains:

- Authentication model
- Authorization mechanism
- Webhook security
- Transport security
- Production hardening recommendations

Security is the responsibility of the operator deploying the gateway.

## Authentication Model

The gateway supports two authentication mechanisms:

1. JWT-based authentication (primary)
2. Basic authentication (for Documentation UI and limited scenarios)

JWT authentication is recommended for all production API usage.

## JWT Authentication

### Purpose

JWT (JSON Web Token) is used to authenticate API requests.

Clients must obtain a valid token before accessing protected endpoints.

### Configuration

Relevant environment variables:

- JWT_SECRET
- JWT_TOKEN_DURATION_MINUTES
- JWT_ISSUER

The JWT_SECRET must be changed in production.

### Token Lifecycle

1. Client authenticates.
2. Server generates signed JWT.
3. Client includes token in Authorization header.
4. Gateway validates token signature and expiration.

Header format:

Authorization: Bearer <token>

### Expiration

Token expiration is defined by:

JWT_TOKEN_DURATION_MINUTES

Expired tokens must be renewed.

Shorter durations increase security.

## Basic Authentication

Basic authentication is primarily used for:

- Documentation UI protection
- Internal tools
- Development environments

Configured via:

- DOCUMENTATION_USER
- DOCUMENTATION_PASSWORD

Basic authentication should not replace JWT in production API usage.

On registering a user phone number, you need to need to configured it via `SECRET_KEY` environment variable. This is used by endpoint `/register` to verify the request is coming from a trusted source.

## Webhook Security

### HMAC Signature

All outgoing webhooks are signed using HMAC.

Configured via:

WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY

This key must:

- Be 32+ bytes
- Be randomly generated
- Remain confidential

### Verification Process (Receiver Side)

Webhook receiver should:

1. Read request body.
2. Recompute HMAC using shared secret.
3. Compare signature header.
4. Reject mismatched signatures.

Failure to validate HMAC allows forged requests.

## CORS Configuration

Controlled via:

HTTP_ORIGIN

Default: *

In production, define explicit origins.

Example:

https://your-backend.example.com

Avoid wildcard in public deployments.

## Rate Limiting

Rate limiting protects against:

- Abuse
- Flooding
- Resource exhaustion

Configuration:

- MESSAGE_RATE_LIMIT_PROVIDER
- MESSAGE_RATE_LIMIT_REQUESTS
- MESSAGE_RATE_LIMIT_DURATION_SECONDS

For distributed deployments, use Redis provider.

## Transport Security

The gateway does not manage TLS certificates.

HTTPS must be handled by:

- Reverse proxy
- Load balancer
- Cloud infrastructure

Never expose the gateway over plain HTTP on public networks.

## Network Security

Recommended production setup:

- Gateway accessible only within private network
- Database not publicly accessible
- RabbitMQ not publicly accessible
- Redis not publicly accessible

Firewall rules should restrict all non-required ports.

## Secret Management

Secrets include:

- JWT_SECRET
- SECRET_KEY
- WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY
- Database credentials
- RabbitMQ credentials
- Redis credentials

Best practices:

- Do not commit secrets to version control
- Use environment injection
- Use secret managers (if available)
- Rotate secrets periodically

## Logging Security

Logging configuration:

- LOG_LEVEL
- LOG_OUTPUT
- LOG_FILE_PATH
- LOG_ENABLE_CALLER

In production:

- Avoid debug level
- Avoid logging sensitive payloads
- Protect log files from unauthorized access

Logs may contain metadata that must be treated as sensitive.

## Documentation UI Exposure

Documentation UI is enabled by default.

In production, either:

- Disable ENABLE_DOCUMENTATION
- Restrict access via Basic Auth
- Restrict via network firewall

Publicly exposed Documentation increases attack surface.

## Message Integrity

Message processing integrity depends on:

- Reliable queue configuration (if enabled)
- Retry configuration (QUEUE_MAX_RETRIES)
- Monitoring webhook failures

Operational monitoring is part of security.

## Threat Considerations

Operators should consider:

- Token theft
- Replay attacks
- Brute force attempts
- DoS attempts
- Misconfigured CORS
- Insecure reverse proxy
- Weak secrets
- Unprotected database

The gateway assumes infrastructure-level hardening is applied.

## Security Hardening Checklist

Before production deployment:

- Change all default secrets
- Enforce HTTPS
- Disable debug logging
- Protect Documentation UI
- Restrict CORS
- Protect database and message broker
- Enable rate limiting
- Monitor abnormal request patterns
- Rotate secrets periodically

## Responsibility Disclaimer

This project is:

- Not affiliated with WhatsApp
- Not affiliated with Meta
- Not an official WhatsApp Business API

Operators are fully responsible for:

- Infrastructure security
- Compliance with WhatsApp terms
- Data protection regulations
- Operational monitoring

Security posture depends on deployment discipline.
