# WhatsApp Gateway Wiki

Welcome to the WhatsApp Gateway documentation! This wiki provides comprehensive guides for setting up, configuring, and using the WhatsApp Gateway.

## Documentation Pages

### Getting Started
- **[Development Guide](Development-Guide.md)** - Learn how to run the gateway in development mode and build with Docker
- **[Environment Variables](Environment-Variables.md)** - Complete reference for all configuration options

### Usage Guides
- **[Gateway Usage Flow](Gateway-Usage-Flow.md)** - Step-by-step guide on how to use the gateway API
- **[Group & Community Management](Group-Management.md)** - Manage groups and communities; ban-safety gates and partial-failure semantics
- **[Security Considerations](Security-Considerations.md)** - Important security warnings and best practices

## Quick Links

- [GitHub Repository](https://github.com/glennprays/whatsapp-gateway)
- [API Documentation (Swagger)](../docs/swagger.yaml)
- [Report Issues](https://github.com/glennprays/whatsapp-gateway/issues)

## Overview

The WhatsApp Gateway is a modern, scalable solution built with Go that handles all WhatsApp complexity for you. It keeps your backend stateless and focused on business logic while the gateway manages all WhatsApp-related state, authentication, and message handling.

### Key Features

- **Stateless Backend** - No need to manage WhatsApp sessions in your application
- **Easy Integration** - Simple REST API and webhook-based architecture
- **Multi-Device Support** - Handle multiple WhatsApp accounts from a single gateway
- **Secure** - JWT authentication with webhook HMAC encryption
- **Built on whatsmeow** - Reliable WhatsApp Web multidevice API implementation

## Need Help?

If you need assistance or have questions:
1. Check the relevant documentation pages above
2. Review the [Swagger API documentation](../docs/swagger.yaml)
3. Open an [issue on GitHub](https://github.com/glennprays/whatsapp-gateway/issues)

---

**Note:** This gateway should always be wrapped by a proper backend service and should not be directly exposed to end users. See [Security Considerations](Security-Considerations.md) for more details.
