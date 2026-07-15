# Go SDK

## Overview

The official Go SDK for WhatsApp Gateway provides an ergonomic client wrapper for integrating with the gateway API.

## Features

- Client configuration with customizable options
- Authentication (register, JWT token management)
- WhatsApp login (QR code, pair code)
- Messaging: text, image, audio, video, document, location, poll, and sticker, with canonical `chat` addressing, quoted replies, @-mentions, and `Idempotency-Key` support on every send; plus edit, delete, and react
- Contact & group reads: `ListContacts`, `GetContactInfo`, `GetAvatar` (conditional/ETag fetch), `ListGroups`, `GetGroupInfo`
- Two-way primitives: `MarkRead` (blue ticks) and `SendChatPresence` (typing/recording indicators)
- Full group & community management (create, participants, settings, name/topic/photo, invite links, join requests, sub-group linking)
- Webhook management and HMAC verification, with a unified `ParseWebhook` dispatcher covering the `message.*` and `session.*` lifecycle events
- Separate opt-in `AdminClient` for the operator-only admin plane (session inventory, root liveness/readiness)
- Typed error handling

See the [SDK repository README](https://github.com/glennprays/whatsapp-gateway-sdk-go#readme) for the full method-by-method reference and usage examples.

## Installation

```bash
go get github.com/glennprays/whatsapp-gateway-sdk-go
```

## Documentation

- **GitHub Repository**: [https://github.com/glennprays/whatsapp-gateway-sdk-go](https://github.com/glennprays/whatsapp-gateway-sdk-go)
- **API Reference**: [https://pkg.go.dev/github.com/glennprays/whatsapp-gateway-sdk-go](https://pkg.go.dev/github.com/glennprays/whatsapp-gateway-sdk-go)
