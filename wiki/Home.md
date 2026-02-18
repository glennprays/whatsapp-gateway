# Whatsapp Gateway Wiki

## Overview

Whatsapp Gateway is a backend service that enables WhatsApp Web protocol integration through a REST API.

This project:

- Is built in Go
- Uses the WhatsApp Web protocol
- Is powered internally by the `whatsmeow` library
- Is not affiliated with WhatsApp or Meta
- Is intended for backend automation use cases

## Documentation

The complete documentation is available inside the repository:

- Introduction
- Installation (Docker & Binary)
- Production Deployment
- Reverse Proxy Setup
- Environment Configuration
- Authentication & Security
- Architecture
- API Reference

For full technical documentation, refer to the [docs directory](https://github.com/glennprays/whatsapp-gateway/tree/main/docs/ui/assets/docs) in the main repository.
On running gateway, the API documentation is also available at the `/docs` endpoint for better accessibility.

## Quick Start

Recommended deployment method:

Docker run:

```bash
docker run -d \
  --name whatsapp-gateway \
  -p 9000:3000 \
  --env-file .env \
  glennprays/whatsapp-gateway
```

Basic steps:

1. Create `.env` file.
2. Configure database and secrets.
3. Run via Docker or Docker Compose.
4. Scan QR code.
5. Start sending messages via API.

> For example `.env` file and detailed instructions, refer to the [.env.example](https://github.com/glennprays/whatsapp-gateway/blob/main/.env.example)

For detailed setup, refer to the Installation documentation.

## System Requirements

Minimum:

- Go 1.25 (for binary build)
- PostgreSQL 16+ (recommended)
- RabbitMQ 3.x (optional)
- Redis (optional, for distributed rate limiting)

Linux is recommended for production environments.

## Project Philosophy

This project is designed to be:

- Lightweight
- Infrastructure-agnostic
- Backend-focused
- Configurable
- Production-aware

It is not intended to replace the official WhatsApp Business API.

## Contribution

External contributions are welcome.

All pull requests require review before merging.

When contributing:

- Follow existing project structure
- Write clear commit messages
- Keep changes focused
- Avoid breaking API compatibility unless necessary

## Support & Responsibility

This project:

- Does not provide SLA
- Does not guarantee WhatsApp account safety
- Depends on WhatsApp Web protocol stability

Operators are responsible for:

- Infrastructure security
- Compliance with WhatsApp policies
- Monitoring and operational reliability

## License

Refer to the repository license file.
