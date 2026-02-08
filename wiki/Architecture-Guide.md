# Architecture Guide

This guide explains the architectural design of the WhatsApp Gateway, including its layered architecture, dependency injection system, and key design patterns.

## Overview

The WhatsApp Gateway follows **Clean Architecture** principles with a clear separation of concerns across four distinct layers. This design ensures maintainability, testability, and flexibility.

## Architecture Layers

```
┌─────────────────────────────────────────┐
│         HTTP Request (Client)           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   1. PRESENTATION LAYER                 │
│   (Handler, Router, Middleware)         │
│   - HTTP request parsing                │
│   - Response formatting                 │
│   - Status code mapping                 │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   2. USE CASE LAYER                     │
│   (Business Logic)                      │
│   - Validation                          │
│   - Business rules                      │
│   - Orchestration                       │
│   - Queue management                    │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   3. DOMAIN LAYER                       │
│   (Entities, Interfaces)                │
│   - Core business models                │
│   - Domain services                     │
│   - Business interfaces                 │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   4. INFRASTRUCTURE LAYER               │
│   (WhatsApp, Database, Queue)           │
│   - External services                   │
│   - Database operations                 │
│   - Message queues                      │
└─────────────────────────────────────────┘
```

### 1. Presentation Layer

**Location**: `internal/handler`, `internal/router`, `internal/middleware`

**Responsibility**: HTTP concerns only
- Parse incoming HTTP requests
- Extract parameters and request bodies
- Format responses (JSON)
- Map errors to HTTP status codes
- Authentication/authorization middleware

**Example**:
```go
func (h *AuthHandler) Register(c *fiber.Ctx) error {
    // Parse request
    var req authDomain.RegistrationRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
    }

    // Call usecase
    response, err := h.authUsecase.Register(traceID, req)
    if err != nil {
        return c.Status(httpErr.Status).JSON(httpErr)
    }

    // Return response
    return c.Status(201).JSON(response)
}
```

### 2. Use Case Layer

**Location**: `internal/usecase`

**Responsibility**: Business logic orchestration
- Validate input data
- Enforce business rules
- Coordinate between domain services
- Handle queue operations
- Log business events
- Error handling and wrapping

**Structure**:
```
internal/usecase/
├── auth/
│   ├── auth_usecase.go       # Registration logic
│   └── provider.go
└── whatsapp/
    ├── auth_usecase.go       # WhatsApp authentication
    ├── webhook_usecase.go    # Webhook management
    ├── message_usecase.go    # Message operations
    └── provider.go
```

**Example**:
```go
func (uc *AuthUsecase) Register(traceID string, req Request) (*Response, error) {
    // Validate secret key
    if req.SecretKey != uc.config.BasicAuthSecretKey {
        uc.logger.Warn(traceID, "Invalid secret key")
        return nil, errDomain.NewError(errDomain.ErrForbidden, ...)
    }

    // Generate token
    token, err := uc.jwtManager.GenerateTokens(req.PhoneNumber)
    if err != nil {
        uc.logger.Error(traceID, "Failed to generate token")
        return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
    }

    uc.logger.Info(traceID, "User registered successfully")
    return &Response{Token: token}, nil
}
```

### 3. Domain Layer

**Location**: `domain/`

**Responsibility**: Core business models and contracts
- Define business entities
- Define interfaces for repositories and services
- Custom error types
- Domain-specific types

**Key Principle**: Domain layer has **NO dependencies** on other layers.

**Structure**:
```
domain/
├── auth/           # JWT claims, auth models
├── error/          # Custom error types
├── queue/          # Queue interfaces
└── whatsapp/       # WhatsApp business models
```

### 4. Infrastructure Layer

**Location**: `internal/whatsapp`, `internal/database`, `pkg/`

**Responsibility**: External integrations
- WhatsApp client management (whatsmeow)
- Database operations
- Message queue (RabbitMQ/Direct)
- External API calls
- File system operations

## Dependency Flow

```
Handler → Usecase → Domain Service → Infrastructure

Direction of dependency: OUTER → INNER
Direction of control: INNER ← OUTER
```

**Key Rules**:
- ✅ Handlers depend on Usecases
- ✅ Usecases depend on Domain interfaces
- ✅ Infrastructure implements Domain interfaces
- ❌ Domain NEVER depends on Infrastructure
- ❌ Domain NEVER depends on Use Cases

## Dependency Injection (Wire)

The application uses [Google Wire](https://github.com/google/wire) for compile-time dependency injection.

### Provider Organization

Each package owns its construction logic via `provider.go` files:

```
config/provider.go                    # Config & Logger
internal/database/provider.go         # Database
pkg/auth/provider.go                  # JWT Manager
internal/usecase/auth/provider.go     # Auth Usecase
internal/handler/auth/provider.go     # Auth Handler
... (15 packages total)
```

**Example Provider**:
```go
// pkg/auth/provider.go
package auth

func ProvideJWTManager(cfg *config.Config) *JWTManager {
    return NewJWTManager(cfg.JwtSecret, cfg.JwtIssuer, cfg.GetJwtDuration())
}
```

### Wire Configuration

**Location**: `internal/infrastructure/wire.go`

```go
func InitializeApp() (*App, func(), error) {
    wire.Build(
        // Configuration & Logging
        config.ProvideConfig,
        config.ProvideLogger,

        // Database
        database.ProvideDatabase,

        // Infrastructure
        cipherx.ProvideCipher,
        pkgQueue.ProvideMessageQueue,

        // Usecases
        auth_usecase.ProvideAuthUsecase,
        whatsapp_usecase.ProvideWhatsappAuthUsecase,

        // Handlers
        auth_handler.ProvideAuthHandler,
        whatsapp_handler.ProvideWhatsappAuthHandler,

        // Middleware & Router
        middleware.ProvideAuthMiddleware,
        router.ProvideRouter,

        wire.Struct(new(App), "FiberApp", "Config", "Logger"),
    )
    return nil, nil, nil
}
```

After modifying providers: `wire gen ./internal/infrastructure`

## Key Design Patterns

### 1. Repository Pattern

**Purpose**: Abstract data access

**Location**:
- Interface: `domain/whatsapp/repository.go`
- Implementation: `internal/whatsapp/repository.go`

```go
// Domain (interface)
type WhatsAppRepository interface {
    GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error)
    SaveWebhookURL(ctx context.Context, phoneNumber, url string) error
}

// Infrastructure (implementation)
type whatsappRepository struct {
    db *sql.DB
}
```

### 2. Manager Pattern

**Purpose**: Orchestrate domain operations

**Location**: `internal/whatsapp/manager.go`

Manages WhatsApp client lifecycle, sessions, and operations.

### 3. Middleware Pattern

**Purpose**: Cross-cutting concerns

**Examples**:
- JWT Authentication (`internal/middleware/auth.go`)
- Request Tracing (`internal/middleware/trace.go`)
- Rate Limiting (future)

### 4. Queue Pattern

**Purpose**: Async message processing

**Location**: `pkg/queue/`, `internal/queue/`

Supports:
- RabbitMQ (production)
- Direct processing (development/fallback)

## Message Flow Example

### Sending a Text Message

```
1. Client → POST /api/messages/text
2. Handler extracts request data
3. Handler → Usecase.SendTextMessage()
4. Usecase checks queue health
5. If queue available:
   - Create job
   - Publish to queue
   - Return job ID
6. If queue unavailable:
   - Call Manager.SendTextMessage()
   - Return message ID
7. Handler formats response
8. Client ← JSON response
```

## Testing Strategy

### Unit Tests

**Handlers**: Mock usecases
```go
mockUsecase := &MockAuthUsecase{}
mockUsecase.On("Register", ...).Return(response, nil)
handler := NewAuthHandler(mockUsecase, logger)
```

**Usecases**: Mock managers
```go
mockManager := &MockWhatsappManager{}
usecase := NewAuthUsecase(cfg, jwtMgr, mockManager, logger)
```

**Infrastructure**: Use test databases or mocks

### Integration Tests

Test complete flows:
- Registration → Login → Send Message
- Queue publish → Worker consume → WhatsApp send

## Adding New Features

### Example: Add "Send Video" Endpoint

**1. Domain Model** (`domain/whatsapp/message.go`):
```go
type SendVideoRequest struct {
    Msisdn  string `json:"msisdn"`
    Caption string `json:"caption"`
}
```

**2. Domain Interface** (`domain/whatsapp/manager.go`):
```go
SendVideoMessage(ctx context.Context, phoneNumber, to string, video []byte) (string, error)
```

**3. Manager Implementation** (`internal/whatsapp/manager.go`):
```go
func (m *Manager) SendVideoMessage(...) (string, error) {
    // WhatsApp video send logic
}
```

**4. Usecase** (`internal/usecase/whatsapp/message_usecase.go`):
```go
func (uc *WhatsappMessageUsecase) SendVideoMessage(...) (*Response, error) {
    // Queue or direct send logic
}
```

**5. Handler** (`internal/handler/whatsapp/whatsapp_message_handler.go`):
```go
func (h *WhatsappMessageHandler) SendVideoMessage(c *fiber.Ctx) error {
    // Parse request, call usecase, return response
}
```

**6. Router** (`internal/router/whatsapp_routes.go`):
```go
api.Post("/messages/video", handler.SendVideoMessage)
```

**7. Wire**: Run `wire gen ./internal/infrastructure`

## Performance Considerations

### 1. Connection Pooling
- Database connection pool configured in `internal/database/`
- WhatsApp client reuse per phone number

### 2. Queue Processing
- Worker pools for parallel message processing
- Configurable concurrency levels

### 3. Caching
- JWT validation results cached
- Consider Redis for session cache (future enhancement)

## Security Architecture

### Authentication Flow
```
Client → JWT Token → Middleware → Context
         (phone number extracted)
```

### Webhook Security
```
Gateway → HMAC Sign → Webhook Request → Backend
Backend validates HMAC before processing
```

See [Security Considerations](Security-Considerations.md) for details.

## Monitoring & Observability

### Logging
- Structured logging with trace IDs
- Log levels: debug, info, warn, error, fatal
- Request/response logging

### Metrics (Future)
- Message throughput
- Queue depth
- Connection health
- Error rates

### Tracing
- Each request gets unique trace ID
- Trace ID propagated through all layers
- Useful for debugging and correlation

## Benefits of This Architecture

✅ **Maintainability**: Clear separation makes code easy to understand
✅ **Testability**: Each layer can be tested independently
✅ **Flexibility**: Easy to swap implementations (e.g., database, queue)
✅ **Scalability**: Stateless design allows horizontal scaling
✅ **Reusability**: Business logic independent of HTTP layer
✅ **Clean Code**: Following SOLID principles

## Common Pitfalls to Avoid

❌ **Don't** put business logic in handlers
❌ **Don't** let domain depend on infrastructure
❌ **Don't** skip the usecase layer
❌ **Don't** tightly couple to framework
❌ **Don't** mix concerns across layers

## Further Reading

- [Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Google Wire Documentation](https://github.com/google/wire/blob/main/docs/guide.md)
- [Dependency Inversion Principle](https://en.wikipedia.org/wiki/Dependency_inversion_principle)

---

[← Back to Home](Home.md)
