# CLAUDE.md - AI Assistant Context for WhatsApp Gateway

> This file provides comprehensive context about the WhatsApp Gateway project for AI assistants (like Claude, ChatGPT, etc.) to understand the codebase structure, architecture, and conventions.

## Project Overview

**WhatsApp Gateway** is a production-ready, scalable REST API gateway built in Go that abstracts WhatsApp Web's complexity. It enables stateless backend applications to integrate with WhatsApp without managing connection states, sessions, or device credentials.

### Core Value Proposition
- **Stateless Backend**: Your application makes HTTP calls; the gateway handles all WhatsApp state
- **Multi-Device Support**: Manage multiple WhatsApp accounts from a single gateway instance
- **Security-First**: JWT authentication, HMAC webhook encryption, proper credential management
- **Language Agnostic**: Any HTTP-capable language can integrate (no WhatsApp SDK required)
- **Built on whatsmeow**: Uses the reliable [whatsmeow](https://github.com/tulir/whatsmeow) library

### Key Features
- QR Code & Pairing Code authentication
- Send/receive text messages via REST API
- Webhook-based event notifications (incoming messages, connection status)
- Session management (reconnect, logout, status checking)
- SQLite & PostgreSQL support
- Docker deployment ready
- Swagger/OpenAPI documentation

---

## Architecture

### Design Pattern: Clean Architecture (Go Standard)

The project follows **Clean Architecture** principles with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation Layer                        │
│  (cmd/api, internal/handler, internal/router, middleware)  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                     Domain Layer                             │
│        (domain/whatsapp, domain/auth, domain/error)         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                        │
│   (internal/whatsapp, internal/database, pkg/auth, pkg/*)   │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
whatsapp-gateway/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point, DI setup
├── config/                          # Configuration management
├── domain/                          # Business entities & interfaces
│   ├── auth/                       # Auth domain models
│   ├── error/                      # Custom error types
│   └── whatsapp/                   # WhatsApp domain models & interfaces
├── internal/                        # Private application code
│   ├── constant/                   # Application constants
│   ├── contextkeys/                # Context key definitions
│   ├── database/                   # Database connection & migrations
│   ├── handler/                    # HTTP request handlers
│   │   ├── auth/                  # Registration handlers
│   │   └── whatsapp/              # WhatsApp operation handlers
│   ├── httperror/                  # HTTP error response utilities
│   ├── middleware/                 # Gin middleware (JWT auth)
│   ├── router/                     # Route definitions
│   │   ├── router.go              # Main router setup
│   │   ├── whatsapp_routes.go     # WhatsApp endpoints
│   │   ├── webhook_routes.go      # Webhook management
│   │   └── swagger_routes.go      # API documentation
│   ├── utils/                      # Internal utilities
│   └── whatsapp/                   # WhatsApp client management
├── pkg/                            # Public/reusable packages
│   ├── auth/                      # JWT generation & validation
│   └── cipherx/                   # AES encryption for HMAC secrets
├── dbs/                            # SQLite database files (gitignored)
├── docs/                           # API documentation (Swagger)
├── wiki/                           # Comprehensive documentation
│   ├── Development-Guide.md
│   ├── Environment-Variables.md
│   ├── Gateway-Usage-Flow.md
│   └── Security-Considerations.md
├── .env.example                    # Environment configuration template
├── Dockerfile                      # Container deployment
├── Makefile                        # Build automation
├── go.mod                          # Go module dependencies
└── README.md                       # Project overview
```

---

## Technology Stack

### Core Dependencies
| Package | Purpose | Version |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | HTTP web framework & routing | v1.10.1 |
| `go.mau.fi/whatsmeow` | WhatsApp Web API client | v0.0.0-20251116104239 |
| `github.com/golang-jwt/jwt/v5` | JWT authentication | v5.3.0 |
| `github.com/mattn/go-sqlite3` | SQLite database driver | v1.14.32 |
| `github.com/lib/pq` | PostgreSQL driver | v1.10.9 |
| `github.com/sirupsen/logrus` | Structured logging | v1.9.3 |
| `github.com/skip2/go-qrcode` | QR code generation | v0.0.0-20200617195104 |
| `google.golang.org/protobuf` | Protocol buffers (whatsmeow) | v1.36.10 |
| `github.com/joho/godotenv` | Environment variable loading | v1.5.1 |

### Database Support
- **SQLite**: Default, file-based (`file:dbs/whatsapp.db`)
- **PostgreSQL**: Production-recommended with SSL support

### Go Version
- **Required**: Go 1.24.0 or higher

---

## API Endpoints

### Authentication
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/api/register` | Basic Auth | Register phone number, returns JWT token |

**Basic Auth**: Uses `BASIC_AUTH_SECRET_KEY` from environment

### WhatsApp Operations
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/api/login/qr_code/:format` | JWT | Generate QR code (png/terminal) |
| `POST` | `/api/login/pair_code` | JWT | Request pairing code |
| `GET` | `/api/login/status` | JWT | Check login status |
| `POST` | `/api/logout` | JWT | Disconnect WhatsApp session |
| `POST` | `/api/session/reconnect` | JWT | Reconnect existing session |

### Webhook Management
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/webhook/` | JWT | Get current webhook URL |
| `POST` | `/api/webhook/` | JWT | Set webhook URL |
| `DELETE` | `/api/webhook/` | JWT | Remove webhook |

### Health & Documentation
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/health` | None | Health check endpoint |
| `GET` | `/docs/*` | Basic Auth | Swagger UI (if `ENABLE_SWAGGER=true`) |

---

## Configuration

### Environment Variables

See `.env.example` for the complete configuration template. Key variables:

#### Server Configuration
```bash
PORT=3000                          # HTTP server port
BASE_PATH=/api                     # API base path prefix
```

#### Authentication
```bash
BASIC_AUTH_SECRET_KEY=secret       # Secret for registration endpoint
JWT_SECRET=secret                  # JWT signing secret (use strong random value)
JWT_DURATION_MINUTES=60            # Token expiration time
JWT_ISSUER=whatsapp-gateway        # JWT issuer claim
```

#### WhatsApp Configuration
```bash
WHATSAPP_DATASTORE_TYPE=sqlite                              # sqlite or postgres
WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)
WHATSAPP_DEVICE_LABEL="Whatsapp Gateway"                    # Device name shown in WhatsApp
WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=0123456789abcdef0123456789abcdef
```

**CRITICAL**: `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` must be 32 hex characters (16 bytes). Generate with: `openssl rand -hex 16`

#### Swagger (Development Only)
```bash
ENABLE_SWAGGER=true               # Enable API documentation
SWAGGER_USER=secret               # Swagger UI username
SWAGGER_PASSWORD=secret           # Swagger UI password
SWAGGER_BASE_PATH=/docs           # Documentation path
```

**Production**: Set `ENABLE_SWAGGER=false`

---

## Authentication Flow

### 1. Registration (Get JWT Token)
```http
POST /api/register
Authorization: Basic <BASIC_AUTH_SECRET_KEY>
Content-Type: application/json

{
  "phoneNumber": "6281234567890"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresAt": "2024-01-26T16:20:00Z"
}
```

### 2. Use JWT for All API Calls
```http
GET /api/login/status
Authorization: Bearer <JWT_TOKEN>
```

### JWT Token Structure
```json
{
  "phoneNumber": "6281234567890",
  "iss": "whatsapp-gateway",
  "sub": "whatsapp-gateway-auth",
  "exp": 1706285200
}
```

---

## Code Patterns & Conventions

### 1. Dependency Injection (cmd/api/main.go)
```go
// Main orchestrates dependency creation
func main() {
    // 1. Load configuration
    cfg := config.Load()
    
    // 2. Initialize infrastructure
    db := database.Connect(cfg)
    
    // 3. Create repositories
    whatsappRepo := whatsapp.NewRepository(db)
    
    // 4. Create domain services
    whatsappManager := whatsapp.NewManager(cfg, whatsappRepo)
    
    // 5. Inject into handlers
    whatsappHandler := whatsapp.NewHandler(whatsappManager)
    
    // 6. Setup router
    router := router.SetupRouter(cfg, whatsappHandler)
    
    // 7. Start server
    router.Run(":" + cfg.Port)
}
```

### 2. Error Handling Pattern
```go
// Domain layer returns custom errors
func (m *Manager) Login(ctx context.Context, phone string) error {
    if !m.isValidPhone(phone) {
        return domainerror.NewValidationError("invalid phone number")
    }
    
    if err := m.whatsappClient.Connect(); err != nil {
        return domainerror.NewInternalError("connection failed", err)
    }
    
    return nil
}

// Handler maps domain errors to HTTP responses
func (h *Handler) Login(c *gin.Context) {
    err := h.manager.Login(c.Request.Context(), phoneNumber)
    if err != nil {
        httperror.HandleError(c, err) // Maps to appropriate HTTP status
        return
    }
    
    c.JSON(http.StatusOK, response)
}
```

### 3. Middleware Chain (internal/router/router.go)
```go
func SetupRouter(cfg *config.Config, handlers *handler.Handlers) *gin.Engine {
    router := gin.Default()
    
    // Public routes
    router.POST("/api/register", handlers.Auth.Register)
    router.GET("/health", healthCheck)
    
    // Protected routes (JWT required)
    api := router.Group("/api")
    api.Use(middleware.JWTAuthentication(cfg.JWT))
    {
        api.POST("/login/qr_code/:format", handlers.WhatsApp.GetQRCode)
        api.POST("/login/pair_code", handlers.WhatsApp.GetPairCode)
        api.GET("/login/status", handlers.WhatsApp.GetStatus)
        // ... more routes
    }
    
    return router
}
```

### 4. Context-Based Request Scoping
```go
// Middleware extracts phone from JWT and stores in context
func JWTAuthentication(jwtConfig config.JWTConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        claims := extractJWTClaims(c)
        
        ctx := context.WithValue(c.Request.Context(), 
            contextkeys.PhoneNumberKey, claims.PhoneNumber)
        
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}

// Handler retrieves phone from context
func (h *Handler) GetStatus(c *gin.Context) {
    phone := c.Request.Context().Value(contextkeys.PhoneNumberKey).(string)
    status := h.manager.GetConnectionStatus(phone)
    c.JSON(200, status)
}
```

### 5. Repository Pattern (internal/whatsapp/)
```go
// Repository interface (domain layer)
type Repository interface {
    SaveSession(ctx context.Context, phone string, session *Session) error
    GetSession(ctx context.Context, phone string) (*Session, error)
    DeleteSession(ctx context.Context, phone string) error
}

// Implementation (infrastructure layer)
type repository struct {
    db *sql.DB
}

func (r *repository) SaveSession(ctx context.Context, phone string, session *Session) error {
    query := "INSERT INTO sessions (phone, data) VALUES (?, ?)"
    _, err := r.db.ExecContext(ctx, query, phone, session.Data)
    return err
}
```

---

## Security Considerations

### 🚨 CRITICAL WARNING

**DO NOT expose this gateway directly to end users or the public internet.**

The gateway **MUST** be wrapped by a proper backend service that implements:
- User authentication & authorization
- Phone number ownership verification
- Access control (prevent unauthorized phone number registration)
- Rate limiting
- Audit logging

### JWT Token Vulnerability

**Security Issue**: If two JWT tokens are generated for the same phone number, both can access the WhatsApp session once either logs in.

**Example**:
1. User A registers `"6281234567890"` → Token 1
2. User B registers same number → Token 2
3. User A logs into WhatsApp
4. ⚠️ User B can also access the session with Token 2

**Mitigation**: Your backend MUST:
- Verify users own their phone numbers before registration
- Prevent duplicate registrations
- Track token-to-user mappings
- Implement proper session lifecycle management

See `wiki/Security-Considerations.md` for comprehensive security guidelines.

### Best Practices
1. **Strong Secrets**: Use cryptographically random values (≥32 chars)
   ```bash
   openssl rand -base64 32  # JWT_SECRET
   openssl rand -hex 16     # HMAC_ENCRYPTION_MASTER_KEY
   ```

2. **Database Security**: 
   - Use PostgreSQL with SSL in production
   - Encrypt database at rest
   - Restrict network access

3. **Network Isolation**:
   - Deploy in private VPC/network
   - Use reverse proxy (Nginx) with HTTPS
   - Enable firewall rules

4. **Webhook Security**:
   - Always verify HMAC signatures
   - Use HTTPS webhook URLs
   - Implement replay attack protection

5. **Production Deployment**:
   - Disable Swagger (`ENABLE_SWAGGER=false`)
   - Use PostgreSQL instead of SQLite
   - Enable comprehensive logging & monitoring
   - Implement security audits

---

## Development Workflow

### Running Locally
```bash
# 1. Clone repository
git clone https://github.com/glennprays/whatsapp-gateway.git
cd whatsapp-gateway

# 2. Setup environment
cp .env.example .env
# Edit .env with your configuration

# 3. Run application
make run
# OR
go run cmd/api/main.go

# Server starts on http://localhost:3000
```

### Using Docker
```bash
# Build image
docker build -t whatsapp-gateway .

# Run container
docker run -p 3000:3000 --env-file .env whatsapp-gateway
```

### Testing the Gateway

1. **Register a phone number**:
```bash
curl -X POST http://localhost:3000/api/register \
  -H "Content-Type: application/json" \
  -H "Authorization: Basic secret" \
  -d '{"phoneNumber": "6281234567890"}'
```

2. **Get QR code** (terminal format):
```bash
curl -X POST http://localhost:3000/api/login/qr_code/terminal \
  -H "Authorization: Bearer <JWT_TOKEN>"
```

3. **Scan QR code** with WhatsApp mobile app

4. **Check status**:
```bash
curl http://localhost:3000/api/login/status \
  -H "Authorization: Bearer <JWT_TOKEN>"
```

---

## Common Tasks

### Adding a New Endpoint

1. **Define domain interface** (`domain/whatsapp/manager.go`):
```go
type Manager interface {
    SendMessage(ctx context.Context, phone, recipient, message string) error
}
```

2. **Implement in service** (`internal/whatsapp/manager.go`):
```go
func (m *manager) SendMessage(ctx context.Context, phone, recipient, message string) error {
    client := m.getClient(phone)
    return client.SendText(recipient, message)
}
```

3. **Create handler** (`internal/handler/whatsapp/send_message.go`):
```go
func (h *Handler) SendMessage(c *gin.Context) {
    phone := c.Request.Context().Value(contextkeys.PhoneNumberKey).(string)
    
    var req struct {
        Recipient string `json:"recipient" binding:"required"`
        Message   string `json:"message" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        httperror.HandleError(c, domainerror.NewValidationError(err.Error()))
        return
    }
    
    if err := h.manager.SendMessage(c.Request.Context(), phone, req.Recipient, req.Message); err != nil {
        httperror.HandleError(c, err)
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "sent"})
}
```

4. **Register route** (`internal/router/whatsapp_routes.go`):
```go
func SetupWhatsAppRoutes(router *gin.RouterGroup, handler *whatsapp.Handler) {
    router.POST("/message/send", handler.SendMessage)
}
```

### Database Migration

The gateway uses `whatsmeow`'s built-in schema management. On startup, it automatically:
1. Creates necessary tables if they don't exist
2. Handles schema upgrades transparently

Custom migrations (if needed) should be added to `internal/database/migrations.go`.

### Adding Custom Middleware

```go
// internal/middleware/custom.go
func RateLimiting() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !checkRateLimit(c.ClientIP()) {
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
            return
        }
        c.Next()
    }
}

// Apply in router
api.Use(middleware.RateLimiting())
```

---

## Troubleshooting

### Connection Issues
- **Symptom**: QR code doesn't appear
- **Check**: Database connection, `WHATSAPP_DATASTORE_URI` configuration
- **Solution**: Ensure database is accessible and path exists

### Authentication Failures
- **Symptom**: 401 Unauthorized on protected endpoints
- **Check**: JWT token validity, `JWT_SECRET` consistency
- **Solution**: Re-register to get a new token, verify secret matches

### Webhook Not Receiving Events
- **Symptom**: No webhook calls on incoming messages
- **Check**: Webhook URL registered, HMAC key configured
- **Solution**: Verify webhook URL is accessible, check logs for delivery errors

### Database Locked (SQLite)
- **Symptom**: "database is locked" errors
- **Solution**: Use PostgreSQL for production, or ensure no concurrent writes to SQLite

---

## Testing

### Unit Tests
```bash
go test ./...
```

### Integration Tests
```bash
# Requires running gateway
go test -tags=integration ./internal/...
```

### Manual API Testing
Use the Swagger UI at `http://localhost:3000/docs` (if enabled) or import the OpenAPI spec into Postman/Insomnia.

---

## Deployment

### Production Checklist
- [ ] Use PostgreSQL with SSL
- [ ] Set strong, unique secrets (JWT, HMAC, database)
- [ ] Disable Swagger (`ENABLE_SWAGGER=false`)
- [ ] Enable HTTPS via reverse proxy
- [ ] Configure firewall rules (restrict to backend only)
- [ ] Setup monitoring & logging
- [ ] Implement backend wrapper with access control
- [ ] Regular backups of session database
- [ ] Security audit completed

### Docker Compose Example
```yaml
version: '3.8'
services:
  gateway:
    build: .
    ports:
      - "3000:3000"
    environment:
      - PORT=3000
      - WHATSAPP_DATASTORE_TYPE=postgres
      - WHATSAPP_DATASTORE_URI=postgresql://user:pass@db:5432/whatsapp
      - JWT_SECRET=${JWT_SECRET}
      - BASIC_AUTH_SECRET_KEY=${BASIC_AUTH_SECRET_KEY}
      - WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=${HMAC_KEY}
    depends_on:
      - db
  
  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
      - POSTGRES_DB=whatsapp
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

---

## Contributing

When contributing to this project:

1. **Follow Go conventions**: 
   - `gofmt` formatting
   - Exported identifiers have comments
   - Package names are lowercase, single-word

2. **Maintain architecture boundaries**:
   - `domain/` should not import `internal/` or `pkg/`
   - `internal/` can import `domain/` and `pkg/`
   - Avoid circular dependencies

3. **Write tests**:
   - Unit tests for business logic
   - Integration tests for API endpoints

4. **Update documentation**:
   - Add comments for public APIs
   - Update wiki for significant changes
   - Keep this CLAUDE.md file in sync

5. **Security-first mindset**:
   - Never commit secrets
   - Validate all user input
   - Use parameterized queries (prevent SQL injection)
   - Log security events

---

## Key Files Reference

### Entry Point
- `cmd/api/main.go` - Application bootstrap, DI container

### Configuration
- `.env.example` - Configuration template
- `config/` - Environment variable loading & validation

### Domain Models
- `domain/whatsapp/manager.go` - WhatsApp service interface
- `domain/auth/jwt.go` - JWT claims structure
- `domain/error/error.go` - Custom error types

### HTTP Layer
- `internal/router/router.go` - Main router setup
- `internal/handler/whatsapp/` - WhatsApp operation handlers
- `internal/handler/auth/` - Registration handlers
- `internal/middleware/jwt.go` - JWT authentication middleware

### Business Logic
- `internal/whatsapp/manager.go` - WhatsApp operations implementation
- `internal/whatsapp/repository.go` - Session persistence

### Utilities
- `pkg/auth/jwt.go` - JWT token generation & parsing
- `pkg/cipherx/cipher.go` - AES encryption for HMAC secrets

---

## Additional Resources

- **Wiki**: https://github.com/glennprays/whatsapp-gateway/wiki
- **whatsmeow Documentation**: https://pkg.go.dev/go.mau.fi/whatsmeow
- **Gin Web Framework**: https://gin-gonic.com/docs/
- **Go Best Practices**: https://go.dev/doc/effective_go

---

## Contact & Support

- **Issues**: https://github.com/glennprays/whatsapp-gateway/issues
- **Discussions**: https://github.com/glennprays/whatsapp-gateway/discussions

---

## License

See [LICENSE](LICENSE) file for details.

---

**Last Updated**: 2026-01-26  
**Project Version**: 1.0.0  
**Go Version**: 1.24.0
