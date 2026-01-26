# CLAUDE.md - AI Assistant Context

> Context file for AI assistants working with the WhatsApp Gateway codebase.

## What This Is

A Go-based REST API gateway that abstracts WhatsApp Web complexity. Enables stateless backend applications to integrate with WhatsApp without managing connection states, sessions, or device credentials.

**Purpose**: Separate WhatsApp state management from business logic. Your backend makes HTTP calls; the gateway handles all WhatsApp complexity.

## Tech Stack

- **Language**: Go 1.24.0
- **Web Framework**: Gin (HTTP routing/middleware)
- **WhatsApp Client**: whatsmeow (WhatsApp Web multidevice API)
- **Authentication**: JWT (golang-jwt/v5)
- **Database**: SQLite (dev) / PostgreSQL (prod)
- **Logging**: Logrus
- **Config**: godotenv (.env files)

## Architecture

**Clean Architecture** with three layers:

1. **Presentation** (`cmd/api`, `internal/handler`, `internal/router`, `internal/middleware`)
   - HTTP handlers, routing, middleware
   - Entry point: `cmd/api/main.go` (dependency injection setup)

2. **Domain** (`domain/whatsapp`, `domain/auth`, `domain/error`)
   - Business entities and interfaces
   - No dependencies on infrastructure

3. **Infrastructure** (`internal/whatsapp`, `internal/database`, `pkg/auth`, `pkg/cipherx`)
   - WhatsApp client management, database persistence, utilities

**Key Principle**: `domain/` never imports `internal/` or `pkg/`. `internal/` can import `domain/` and `pkg/`.

## Project Structure

```
cmd/api/main.go          # Entry point, DI container
config/                  # Environment config loading
domain/                  # Business entities & interfaces
  ├── auth/             # JWT claims
  ├── error/            # Custom error types
  └── whatsapp/         # WhatsApp domain models
internal/               # Private application code
  ├── handler/          # HTTP request handlers
  │   ├── auth/        # Registration
  │   └── whatsapp/    # WhatsApp operations
  ├── router/           # Route definitions
  ├── middleware/       # JWT authentication
  ├── whatsapp/         # WhatsApp client management
  ├── database/         # DB connection & migrations
  └── httperror/        # HTTP error responses
pkg/                    # Reusable packages
  ├── auth/            # JWT generation/validation
  └── cipherx/         # AES encryption for HMAC secrets
```

## Why This Design

- **Stateless Backend**: Backend doesn't store WhatsApp sessions or connection state
- **Multi-Device Support**: One gateway instance handles multiple WhatsApp accounts
- **Language Agnostic**: Any HTTP client can integrate (no WhatsApp SDK needed)
- **Scalability**: Backend scales horizontally; gateway manages persistent connections
- **Separation of Concerns**: WhatsApp logic isolated from business logic

## How It Works

### Authentication Flow
1. Backend calls `POST /api/register` with phone number (Basic Auth)
2. Gateway returns JWT token
3. Backend uses JWT for all subsequent API calls
4. Gateway maintains WhatsApp session per phone number

### WhatsApp Connection Flow
1. Call `POST /api/login/qr_code/:format` with JWT → Get QR code
2. Scan QR with WhatsApp mobile app
3. Gateway maintains persistent connection
4. Use `GET /api/login/status` to check connection state

### Message Flow
- **Outbound**: Backend calls API endpoint with JWT
- **Inbound**: Gateway sends webhook to configured URL (HMAC-signed)

## Key Patterns

### Dependency Injection (cmd/api/main.go)
```go
cfg := config.Load()
db := database.Connect(cfg)
repo := whatsapp.NewRepository(db)
manager := whatsapp.NewManager(cfg, repo)
handler := whatsapp.NewHandler(manager)
router := router.SetupRouter(cfg, handler)
router.Run(":" + cfg.Port)
```

### Error Handling
- Domain layer returns custom errors (`domain/error`)
- HTTP layer maps to status codes (`internal/httperror`)
- Pattern: `domainerror.NewValidationError()`, `domainerror.NewInternalError()`

### Context-Based Request Scoping
- JWT middleware extracts phone number from token
- Stores in `context.Context` using `contextkeys.PhoneNumberKey`
- Handlers retrieve phone from context (no parameter passing)

### Repository Pattern
- Interfaces in `domain/` layer
- Implementations in `internal/whatsapp/`
- Database operations abstracted behind repository interface

## Build & Run

```bash
# Development
cp .env.example .env
make run
# OR: go run cmd/api/main.go

# Docker
docker build -t whatsapp-gateway .
docker run -p 3000:3000 --env-file .env whatsapp-gateway

# Testing
go test ./...
```

## Configuration

Key environment variables (`.env.example`):
- `PORT` - HTTP server port (default: 3000)
- `JWT_SECRET` - JWT signing secret (use: `openssl rand -base64 32`)
- `BASIC_AUTH_SECRET_KEY` - Secret for registration endpoint
- `WHATSAPP_DATASTORE_TYPE` - sqlite or postgres
- `WHATSAPP_DATASTORE_URI` - Database connection string
- `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` - 32 hex chars (use: `openssl rand -hex 16`)
- `ENABLE_SWAGGER` - Set to `false` in production

## Security Warnings

### 🚨 CRITICAL: Gateway Must Be Wrapped

**DO NOT expose directly to end users.** Must be behind a backend service that:
- Authenticates users
- Verifies phone number ownership
- Prevents unauthorized registrations
- Implements rate limiting

### JWT Token Sharing Issue

If two JWT tokens are generated for the same phone number, **both can access the WhatsApp session** once either logs in.

**Example**:
1. User A registers phone "123" → Token 1
2. User B registers same phone → Token 2
3. User A logs into WhatsApp
4. ⚠️ User B can also access the session with Token 2

**Mitigation**: Backend must prevent duplicate registrations and verify ownership.

### Production Requirements
- Use PostgreSQL with SSL (not SQLite)
- Strong secrets (≥32 chars, cryptographically random)
- Disable Swagger (`ENABLE_SWAGGER=false`)
- HTTPS via reverse proxy
- Private network/VPC deployment
- Verify webhook HMAC signatures

## API Endpoints

**Public**:
- `POST /api/register` - Register phone, get JWT (Basic Auth)
- `GET /health` - Health check

**Protected** (JWT required):
- `POST /api/login/qr_code/:format` - Generate QR code (png/terminal)
- `POST /api/login/pair_code` - Get pairing code
- `GET /api/login/status` - Check login status
- `POST /api/logout` - Disconnect session
- `POST /api/session/reconnect` - Reconnect existing session
- `GET|POST|DELETE /api/webhook/` - Manage webhook URL

## Adding a New Endpoint

1. Define interface in `domain/whatsapp/manager.go`
2. Implement in `internal/whatsapp/manager.go`
3. Create handler in `internal/handler/whatsapp/`
4. Register route in `internal/router/whatsapp_routes.go`

## Common Issues

- **QR code doesn't appear**: Check database connection and `WHATSAPP_DATASTORE_URI`
- **401 Unauthorized**: Verify JWT token and `JWT_SECRET` consistency
- **Webhook not receiving**: Check URL registration and HMAC key
- **Database locked (SQLite)**: Use PostgreSQL for production

## Additional Documentation

- `wiki/Security-Considerations.md` - Comprehensive security guidelines
- `wiki/Environment-Variables.md` - Full config reference
- `wiki/Gateway-Usage-Flow.md` - Step-by-step usage guide
- `wiki/Development-Guide.md` - Development setup

---

**Last Updated**: 2026-01-26
