# WhatsApp Gateway 
A modern, scalable WhatsApp Gateway built with Go and [whatsmeow](https://github.com/tulir/whatsmeow) that handles all the WhatsApp complexity for you. No more state management headaches in your backend—let the gateway do the heavy lifting!

## What's This All About?
Ever tried integrating WhatsApp into your backend and got tangled up managing connection states, session data, and all that WhatsApp jazz? Yeah, we've been there too. That's why we built this gateway.

**The core idea is simple:** Keep your backend stateless and focused on business logic. This gateway takes care of all WhatsApp-related state management, authentication, and message handling. Your backend just needs to make HTTP calls and handle webhooks. Easy peasy.

## Why Use This?
- **Stateless Backend** - Your application doesn't need to worry about WhatsApp sessions, connection states, or device management. The gateway handles it all.
- **Easy Scalability** - Since your backend stays stateless, you can scale it horizontally without worrying about WhatsApp session distribution.
- **Simple Integration** - Just REST API calls and webhooks. No need to learn WhatsApp's complex protocols.
- **Multi-Device Support** - Handle multiple WhatsApp accounts/devices from a single gateway instance.
- **Built on whatsmeow** - Uses the reliable [whatsmeow](https://github.com/tulir/whatsmeow) library for WhatsApp Web's multidevice API.
- **Secure** - JWT authentication, webhook HMAC encryption, and proper credential management.

## How It Works
The gateway sits between your backend and WhatsApp, maintaining all the persistent connections and state. Your backend just sends REST requests and receives webhook notifications.

## Quick Start

### Prerequisites
- Go 1.25 or higher
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

3. Generate Wire dependency injection code:
```bash
make generate
```

4. Run the application:
```bash
make run
# Or: go run cmd/api/main.go
```

Or using Docker:
```bash
docker build -t whatsapp-gateway .

# For testing (ephemeral data):
docker run -p 3000:3000 --env-file .env whatsapp-gateway

# For persistent data (recommended):
docker run -p 3000:3000 \
  -v whatsapp-data:/dbs \
  --env-file .env \
  whatsapp-gateway
```

### Development Commands
```bash
# Generate Wire DI code (required after modifying wire.go)
make generate

# Run the application
make run

# Build the binary
go build -o api ./cmd/api
```

### Basic Usage
1. **Register a WhatsApp account** and get a QR code for scanning
2. **Scan the QR code** with your WhatsApp mobile app
3. **Start sending messages** through the REST API
4. **Receive messages** via configured webhooks
That's it! Check the API documentation at `/docs` (if Documentation is enabled) for detailed endpoint information.

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

### Observability
- **Request Tracing** - UUID-based trace IDs track requests end-to-end
- **Structured Logging** - JSON logs with contextual information
- **Log Aggregation Ready** - Compatible with Fluent Bit, Promtail, Vector, etc.

## Request Tracing

The gateway supports request tracing via the `X-Trace-ID` header:

- **Client-provided Trace ID**: Include `X-Trace-ID` header in your request with a valid UUID
- **Auto-generated Trace ID**: If no header is provided or if the UUID is invalid, a new UUID is generated automatically
- **Response Header**: The trace ID is always returned in the `X-Trace-ID` response header
- **Log Correlation**: All logs for a request include the same trace ID for easy correlation

Example:
```bash
# With custom trace ID
curl -H "X-Trace-ID: 123e4567-e89b-12d3-a456-426614174000" \
     http://localhost:3000/api/health

# Without trace ID (auto-generated)
curl http://localhost:3000/api/health
```

## Configuration
Key configuration options in `.env`:

### Server Configuration
- `PORT` - Server port (default: 3000)
- `ENV` - Environment: development/staging/production (default: development)

### Logging Configuration
- `LOG_LEVEL` - Log level: debug/info/warn/error/fatal (default: debug)
- `LOG_OUTPUT` - Output destination: stdout/file (default: stdout)
- `LOG_FILE_PATH` - Log file path when output is file (default: /var/log/whatsapp-gateway.log)
- `LOG_ENABLE_CALLER` - Enable caller info in logs for debugging (default: true)

### WhatsApp Configuration
- `WHATSAPP_DATASTORE_TYPE` - Database type (sqlite/postgres)
- `WHATSAPP_DATASTORE_URI` - Database connection string

### Security Configuration
- `JWT_SECRET` - Secret for JWT token generation
- `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` - Master key for webhook HMAC encryption on DB

See `.env.example` for all available options.

## Documentation

For detailed guides, API documentation, architecture explanations, and more, check out our [**Wiki**](https://github.com/glennprays/whatsapp-gateway/wiki)

### 📚 Wiki Pages

- **[Development Guide](wiki/Development-Guide.md)** - How to run in development mode and build with Docker
- **[Environment Variables](wiki/Environment-Variables.md)** - Complete configuration reference
- **[Gateway Usage Flow](wiki/Gateway-Usage-Flow.md)** - Step-by-step usage guide
- **[Security Considerations](wiki/Security-Considerations.md)** - Important security warnings and best practices 

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

- **Language:** Go 1.25
- **Dependency Injection:** [Wire](https://github.com/google/wire) - Compile-time DI
- **Logging:** Custom structured logger with trace ID support
- **WhatsApp Library:** [whatsmeow](https://github.com/tulir/whatsmeow)
- **Web Framework:** Gin
- **Database:** SQLite / PostgreSQL
- **Authentication:** JWT
- **API Documentation:** Swagger/OpenAPI

### Architecture Patterns
- **Wire DI**: Automated dependency injection with compile-time code generation
- **Clean Architecture**: Separation of domain, infrastructure, and presentation layers
- **Structured Logging**: JSON logs with trace IDs for request correlation

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Acknowledgments

- Built with [whatsmeow](https://github.com/tulir/whatsmeow) - A fantastic Go library for WhatsApp Web's multidevice API
- Inspired by the need for simpler WhatsApp integrations

---

**Need help?** Check the [Wiki](https://github.com/glennprays/whatsapp-gateway/wiki) or open an issue!
