# Reverse Proxy Setup

## Purpose

This document explains how to configure a reverse proxy in front of Whatsapp Gateway.

A reverse proxy is recommended in production environments to provide:

- HTTPS termination
- Network isolation
- Centralized access control
- Rate limiting
- Observability
- Improved security posture

The gateway itself does not manage TLS certificates.

## Why Use a Reverse Proxy

Directly exposing the gateway container or binary to the public internet is not recommended.

A reverse proxy allows you to:

- Terminate TLS securely
- Restrict allowed HTTP methods
- Apply IP filtering
- Enforce request size limits
- Enable structured logging
- Integrate with centralized monitoring

## Deployment Topology

Recommended structure:

Public Internet  
→ Reverse Proxy (HTTPS)  
→ Whatsapp Gateway (private network)  

The gateway should listen on an internal port (default: 3000).

The reverse proxy forwards requests securely.

## Nginx Example Configuration

Minimal HTTPS configuration:

```nginx
server {
    listen 443 ssl;
    server_name gateway.example.com;

    ssl_certificate /etc/ssl/certs/fullchain.pem;
    ssl_certificate_key /etc/ssl/private/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;

    location / {
        proxy_pass http://whatsapp-gateway:3000;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

If running via Docker Compose, ensure both services share the same Docker network.

## HTTP to HTTPS Redirect

To enforce secure traffic:

```nginx
server {
    listen 80;
    server_name gateway.example.com;
    return 301 https://$host$request_uri;
}
```

This prevents plaintext communication.

## Rate Limiting (Optional)

To mitigate abuse:

```nginx
limit_req_zone $binary_remote_addr zone=gateway_limit:10m rate=10r/s;

server {
    location / {
        limit_req zone=gateway_limit burst=20 nodelay;
        proxy_pass http://whatsapp-gateway:3000;
    }
}
```

Rate limiting strategy depends on workload characteristics.

## Access Control (Optional)

To restrict access by IP:

```nginx
location / {
    allow 192.168.1.0/24;
    deny all;
    proxy_pass http://whatsapp-gateway:3000;
}
```

This is recommended if the gateway is intended only for internal services.

## Header Security

Recommended security headers:

```nginx
add_header X-Content-Type-Options nosniff;
add_header X-Frame-Options DENY;
add_header X-XSS-Protection "1; mode=block";
```

These headers reduce attack surface.

## Webhook Considerations

If the gateway sends outbound webhooks:

- Ensure outbound traffic is allowed.
- Webhook endpoints should also use HTTPS.
- Webhook servers should validate HMAC signature.

Reverse proxy does not interfere with outbound webhook dispatching.

## Logging

Enable access logs for:

- Auditing
- Monitoring
- Debugging request failures

Example:

```nginx
access_log /var/log/nginx/gateway_access.log;
error_log /var/log/nginx/gateway_error.log;
```

Logs should be rotated and monitored.

## Alternative Reverse Proxies

Other supported options:

- Traefik
- Caddy
- HAProxy
- Cloud load balancers

The gateway is reverse-proxy agnostic as long as HTTP forwarding is correct.

## Docker Compose Example

Example integration with Nginx:

```yaml
version: "3.9"

services:
  gateway:
    image: glennprays/whatsapp-gateway
    env_file:
      - .env
    expose:
      - "3000"

  nginx:
    image: nginx:stable
    ports:
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf
      - ./certs:/etc/ssl
    depends_on:
      - gateway
```

In this setup:

- Gateway is not directly exposed.
- Only Nginx publishes ports.
- Both services share a Docker network.

## Security Recommendations

Production deployments should:

- Use modern TLS versions only
- Avoid exposing internal ports
- Use strong cipher suites
- Monitor unusual traffic spikes
- Protect against brute force attempts
- Regularly update proxy image

Reverse proxy configuration is part of infrastructure responsibility.

## Summary

Reverse proxy is strongly recommended for:

- Production deployments
- Public-facing APIs
- Secure webhook integrations
- Controlled network exposure

It enhances security, observability, and operational stability without modifying gateway behavior.
