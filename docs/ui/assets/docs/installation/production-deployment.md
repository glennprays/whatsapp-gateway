# Production Deployment

## Purpose

This document provides guidance for deploying Whatsapp Gateway in a production environment.

The goal is to ensure:

- Stability
- Security
- Observability
- Controlled exposure
- Infrastructure resilience

Production deployment must treat the gateway as a critical integration component, not a standalone application.

## Recommended Architecture

Production deployment should follow this topology:

Client Applications  
→ Backend Service (Business Logic Layer)  
→ Whatsapp Gateway  
→ WhatsApp Network  

The gateway should not be directly exposed to end users.

## Infrastructure Components

Minimum recommended production stack:

- Linux host (recommended)
- Docker runtime
- PostgreSQL 16+
- RabbitMQ 3.x (if queue mode enabled)
- Reverse proxy (Nginx or equivalent)
- Private network segmentation

## Network Model

Recommended model:

- Backend service and gateway communicate over private network.
- Database and RabbitMQ are not publicly accessible.
- Only reverse proxy (if used) is externally exposed.
- TLS termination handled at reverse proxy layer.

Example:

Public Internet  
→ Reverse Proxy (HTTPS)  
→ Backend Service  
→ Gateway (private network)

Direct public exposure of the gateway is discouraged unless properly secured.

## Reverse Proxy Configuration

Reverse proxy should provide:

- HTTPS termination
- Request logging
- Rate limiting (optional)
- IP filtering (optional)

Example minimal Nginx configuration:

```nginx
server {
    listen 443 ssl;
    server_name gateway.example.com;

    ssl_certificate /etc/ssl/cert.pem;
    ssl_certificate_key /etc/ssl/key.pem;

    location / {
        proxy_pass http://gateway:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

Always use HTTPS for webhook communication.

## Environment Configuration

Production configuration should:

- Use strong JWT secret
- Use strong webhook HMAC secret
- Store secrets securely (not hardcoded)
- Disable debug-level logging
- Enable queue mode for high throughput systems

Secrets must not be committed to version control.

## Database Configuration

Production recommendations:

- Use PostgreSQL 16+
- Enable regular backups
- Monitor connection pool usage
- Use dedicated database instance
- Apply proper indexing strategy

SQLite is not recommended for production at scale.

## RabbitMQ Configuration (If Enabled)

When queue mode is enabled:

- Deploy RabbitMQ in stable environment
- Enable persistent queues
- Monitor queue depth
- Monitor consumer lag
- Enable proper authentication
- Avoid exposing management UI publicly

RabbitMQ becomes a critical dependency in queue mode.

## Health Monitoring

Production monitoring should include:

- Gateway health endpoint
- WhatsApp login status endpoint
- Database connectivity
- RabbitMQ connectivity (if enabled)
- Webhook failure rate
- Message failure rate

Monitoring systems should alert on:

- WhatsApp session disconnect
- Excessive webhook retries
- Database connection failures
- Queue backlog growth

## Scaling Considerations

Current architecture is single-instance oriented.

Scaling vertically is recommended before horizontal scaling:

- Increase CPU
- Increase memory
- Optimize database

Horizontal scaling requires:

- Shared database
- Centralized queue
- Coordinated session management

Distributed clustering is part of long-term roadmap and not natively supported yet.

## Backup Strategy

Critical components requiring backup:

- PostgreSQL database
- RabbitMQ persistent storage (if queue enabled)

Database backup frequency depends on business requirements.

Loss of database may result in:

- Loss of WhatsApp session state
- Loss of message tracking history

## Security Hardening Checklist

Production deployment should ensure:

- JWT authentication enabled
- HMAC webhook validation enabled
- HTTPS enforced
- Database not publicly accessible
- RabbitMQ not publicly accessible
- Secrets stored securely
- Firewall rules configured
- Access logging enabled

## High Availability

The gateway does not provide built-in HA clustering.

To increase availability:

- Use infrastructure-level restart policies
- Deploy on reliable host
- Use managed database services
- Monitor WhatsApp session health

There is no SLA guarantee.

## Media Storage (Future Planning)

S3 or S3-compatible object storage (AWS S3, MinIO, DigitalOcean Spaces, etc.) is supported and production-ready. Set `STORAGE_PROVIDER=s3` along with the `STORAGE_S3_*` environment variables to enable it.

Future production deployments should consider:

- External object storage
- Secure access credentials
- Lifecycle policies

External S3-compatible object storage is already supported: set `STORAGE_PROVIDER=s3` for S3-compatible storage, or keep the default local storage.

## Operational Responsibility

Operators are responsible for:

- Infrastructure maintenance
- Compliance with WhatsApp policies
- Security configuration
- Monitoring and alerting
- Backup management

The gateway is an integration component, not a managed service.
