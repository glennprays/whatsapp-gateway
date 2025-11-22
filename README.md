# WhatsApp Gateway 🚀

A modern, scalable WhatsApp Gateway built with Go and [whatsmeow](https://github.com/tulir/whatsmeow) that handles all the WhatsApp complexity for you. No more state management headaches in your backend—let the gateway do the heavy lifting!

## What's This All About?

Ever tried integrating WhatsApp into your backend and got tangled up managing connection states, session data, and all that WhatsApp jazz? Yeah, we've been there too. That's why we built this gateway.

**The core idea is simple:** Keep your backend stateless and focused on business logic. This gateway takes care of all WhatsApp-related state management, authentication, and message handling. Your backend just needs to make HTTP calls and handle webhooks. Easy peasy.

## Why Use This?

- **🎯 Stateless Backend** - Your application doesn't need to worry about WhatsApp sessions, connection states, or device management. The gateway handles it all.
- **📈 Easy Scalability** - Since your backend stays stateless, you can scale it horizontally without worrying about WhatsApp session distribution.
- **🔌 Simple Integration** - Just REST API calls and webhooks. No need to learn WhatsApp's complex protocols.
- **📱 Multi-Device Support** - Handle multiple WhatsApp accounts/devices from a single gateway instance.
- **🛠️ Built on whatsmeow** - Uses the reliable [whatsmeow](https://github.com/tulir/whatsmeow) library for WhatsApp Web's multidevice API.
- **🔒 Secure** - JWT authentication, webhook HMAC encryption, and proper credential management.

## How It Works

```
┌─────────────┐         ┌──────────────────┐         ┌──────────────┐
│   Your      │ ◄─────► │  WhatsApp        │ ◄─────► │  WhatsApp    │
│   Backend   │  REST   │  Gateway         │ WebSocket│  Servers     │
│             │  APIs   │  (Stateful)      │         │              │
└─────────────┘         └──────────────────┘         └──────────────┘
       │                         │
       │                         │
       └─────── Webhooks ────────┘
```

The gateway sits between your backend and WhatsApp, maintaining all the persistent connections and state. Your backend just sends REST requests and receives webhook notifications.

## Quick Start

### Prerequisites

- Go 1.24 or higher
- SQLite or PostgreSQL (for storing WhatsApp session data)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/glennprays/whatsapp-gateway.git
cd whatsapp-gateway
```

2. Copy the example environment file and configure it:
```bash
cp .env.example .env
# Edit .env with your preferred settings
```

3. Run the application:
```bash
go run cmd/api/main.go
```

Or using Docker:
```bash
docker build -t whatsapp-gateway .
docker run -p 3000:3000 --env-file .env whatsapp-gateway
```

### Basic Usage

1. **Register a WhatsApp account** and get a QR code for scanning
2. **Scan the QR code** with your WhatsApp mobile app
3. **Start sending messages** through the REST API
4. **Receive messages** via configured webhooks

That's it! Check the API documentation at `/docs` (if Swagger is enabled) for detailed endpoint information.

## Features

### Authentication
- QR Code login
- Pairing code login
- JWT-based API authentication
- Basic auth support

### Messaging
- Send text messages
- Receive messages via webhooks
- Connection status monitoring
- Session management

### Management
- Multi-device support
- Webhook configuration
- Auto-reconnection handling
- Graceful shutdown

## Configuration

Key configuration options in `.env`:

- `PORT` - Server port (default: 3000)
- `WHATSAPP_DATASTORE_TYPE` - Database type (sqlite/postgres)
- `WHATSAPP_DATASTORE_URI` - Database connection string
- `JWT_SECRET` - Secret for JWT token generation
- `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` - Master key for webhook HMAC

See `.env.example` for all available options.

## Documentation

For detailed guides, API documentation, architecture explanations, and more, check out our [**Wiki**](https://github.com/glennprays/whatsapp-gateway/wiki) 📚

## Architecture Benefits

### For Your Backend
- **No WhatsApp SDK required** - Just standard HTTP clients
- **Language agnostic** - Any language that can make HTTP requests works
- **Simplified deployment** - No need to package WhatsApp libraries with your app
- **Better separation of concerns** - WhatsApp logic lives in the gateway

### For DevOps
- **Independent scaling** - Scale the gateway and backend independently
- **Easier monitoring** - All WhatsApp metrics in one place
- **Centralized updates** - Update WhatsApp integration without touching the backend
- **Resource optimization** - Run one gateway for multiple backend instances

## Tech Stack

- **Language:** Go 1.24
- **WhatsApp Library:** [whatsmeow](https://github.com/tulir/whatsmeow)
- **Web Framework:** Gin
- **Database:** SQLite / PostgreSQL
- **Authentication:** JWT
- **API Documentation:** Swagger/OpenAPI

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Acknowledgments

- Built with [whatsmeow](https://github.com/tulir/whatsmeow) - A fantastic Go library for WhatsApp Web's multidevice API
- Inspired by the need for simpler WhatsApp integrations

---

**Need help?** Check the [Wiki](https://github.com/glennprays/whatsapp-gateway/wiki) or open an issue!
