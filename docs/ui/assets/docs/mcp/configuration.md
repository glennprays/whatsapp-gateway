# Configuration

Complete configuration reference for the MCP WhatsApp Gateway server.

## Environment Variables

All configuration is done via environment variables. Set these when running the Docker container or binary.

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `WAGA_BASE_URL` | Your WhatsApp Gateway API URL | `http://localhost:3000/api/v1` |
| `WAGA_JWT_TOKEN` | JWT token from gateway registration | `eyJhbGciOiJIUzI1NiIs...` |

### Optional Variables

#### Application Settings

| Variable | Description | Default | Options |
|----------|-------------|---------|---------|
| `APP_ENV` | Application environment | `development` | `development`, `production` |
| `LOG_LEVEL` | Logging verbosity | `info` | `debug`, `info`, `warn`, `error` |

#### Transport Settings

| Variable | Description | Default | Options |
|----------|-------------|---------|---------|
| `MCP_TRANSPORT` | Transport protocol | `stdio` | `stdio`, `http` |
| `MCP_PORT` | HTTP+SSE port (when `MCP_TRANSPORT=http`) | `8080` | Any valid port |

#### Production HTTP+SSE Only

| Variable | Description | Required When |
|----------|-------------|---------------|
| `MCP_BASIC_AUTH_USER` | Basic auth username | `APP_ENV=production` and `MCP_TRANSPORT=http` |
| `MCP_BASIC_AUTH_PASSWORD` | Basic auth password | `APP_ENV=production` and `MCP_TRANSPORT=http` |

## Transport Options

### stdio Transport (Default)

Best for: **Claude Desktop, Cursor, Claude Code CLI**

**Characteristics:**
- Uses standard input/output for communication
- Runs as a subprocess of the AI agent
- Simple configuration with environment variables
- Single client connection

**Configuration:**
```bash
MCP_TRANSPORT="stdio"  # Optional, this is the default
WAGA_BASE_URL="http://localhost:3000/api/v1"
WAGA_JWT_TOKEN="your_jwt_token"
```

**Running with stdio:**
```bash
# Docker
docker run -i --rm \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest

# Binary
WAGA_BASE_URL="http://localhost:3000/api/v1" \
WAGA_JWT_TOKEN="your_jwt_token" \
./mcp-whatsapp-gateway
```

### HTTP+SSE Transport

Best for: **Web-based MCP clients, Open Code (VS Code)**

**Characteristics:**
- Runs as a standalone HTTP server
- Uses Server-Sent Events (SSE) for real-time updates
- Supports multiple concurrent clients
- Can be secured with Basic Authentication

**Development Configuration** (no authentication):
```bash
MCP_TRANSPORT="http"
MCP_PORT="8080"
APP_ENV="development"
WAGA_BASE_URL="http://localhost:3000/api/v1"
WAGA_JWT_TOKEN="your_jwt_token"
```

**Production Configuration** (with authentication):
```bash
MCP_TRANSPORT="http"
MCP_PORT="8080"
APP_ENV="production"
MCP_BASIC_AUTH_USER="admin"
MCP_BASIC_AUTH_PASSWORD="secure_password"
WAGA_BASE_URL="https://your-gateway.com/api/v1"
WAGA_JWT_TOKEN="your_jwt_token"
```

**Running with HTTP+SSE:**
```bash
# Docker (Development)
docker run -d --name whatsapp-gateway-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT="http" \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest

# Docker (Production)
docker run -d --name whatsapp-gateway-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT="http" \
  -e APP_ENV="production" \
  -e MCP_BASIC_AUTH_USER="admin" \
  -e MCP_BASIC_AUTH_PASSWORD="secure_password" \
  -e WAGA_BASE_URL="https://your-gateway.com/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  --restart unless-stopped \
  glennprays/mcp-whatsapp-gateway:latest
```

## Production Deployment

### Security Best Practices

1. **Use HTTPS** for your WhatsApp Gateway URL
2. **Set Strong Passwords** for Basic Authentication
3. **Use APP_ENV=production** to enable security features
4. **Rotate JWT tokens** regularly
5. **Monitor logs** for suspicious activity

### Docker Compose Example

```yaml
version: '3.8'

services:
  whatsapp-gateway-mcp:
    image: glennprays/mcp-whatsapp-gateway:latest
    container_name: whatsapp-gateway-mcp
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      # Application Settings
      APP_ENV: "production"
      LOG_LEVEL: "info"

      # Transport Settings
      MCP_TRANSPORT: "http"
      MCP_PORT: "8080"

      # Authentication
      MCP_BASIC_AUTH_USER: "${MCP_BASIC_AUTH_USER}"
      MCP_BASIC_AUTH_PASSWORD: "${MCP_BASIC_AUTH_PASSWORD}"

      # Gateway Configuration
      WAGA_BASE_URL: "${WAGA_BASE_URL}"
      WAGA_JWT_TOKEN: "${WAGA_JWT_TOKEN}"
```

### Environment File (.env)

```bash
# MCP Server Settings
APP_ENV=production
LOG_LEVEL=info
MCP_TRANSPORT=http
MCP_PORT=8080

# Basic Authentication
MCP_BASIC_AUTH_USER=admin
MCP_BASIC_AUTH_PASSWORD=your_secure_password_here

# Gateway Configuration
WAGA_BASE_URL=https://your-gateway.com/api/v1
WAGA_JWT_TOKEN=your_jwt_token_here
```

### Running with Docker Compose

```bash
# Create .env file
cat > .env << EOF
APP_ENV=production
MCP_TRANSPORT=http
MCP_PORT=8080
MCP_BASIC_AUTH_USER=admin
MCP_BASIC_AUTH_PASSWORD=your_secure_password
WAGA_BASE_URL=https://your-gateway.com/api/v1
WAGA_JWT_TOKEN=your_jwt_token
EOF

# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

## Gateway URL Configuration

### Local Development

```bash
# Running gateway on localhost
WAGA_BASE_URL="http://localhost:3000/api/v1"
```

### Docker (Gateway on Host)

```bash
# macOS/Windows - Use host.docker.internal
WAGA_BASE_URL="http://host.docker.internal:3000/api/v1"

# Linux - Use Docker bridge IP
WAGA_BASE_URL="http://172.17.0.1:3000/api/v1"
```

### Remote Gateway

```bash
# Production gateway with HTTPS
WAGA_BASE_URL="https://waga.example.com/api/v1"

# Behind reverse proxy
WAGA_BASE_URL="https://api.example.com/whatsapp/api/v1"
```

## Logging

### Log Levels

| Level | Description | Use Case |
|-------|-------------|----------|
| `debug` | Detailed debugging information | Development |
| `info` | General informational messages | Production (default) |
| `warn` | Warning messages | Production |
| `error` | Error messages | Production |

### Viewing Logs

```bash
# Docker - View logs from running container
docker logs whatsapp-gateway-mcp

# Docker - Follow logs in real-time
docker logs -f whatsapp-gateway-mcp

# Docker - View last 100 lines
docker logs --tail 100 whatsapp-gateway-mcp

# Docker Compose
docker-compose logs -f whatsapp-gateway-mcp
```

### Trace IDs

All operations include a trace ID for debugging. When reporting issues, include the trace ID from your logs.

```
[INFO] 2024-01-15T10:30:45Z trace_id=abc123-def456 Sending text message to +1234567890
```

## Configuration Examples

### Claude Desktop (stdio)

```bash
docker run -i --rm \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

### Cursor IDE (stdio)

```bash
docker run -i --rm \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

### Open Code / VS Code (HTTP+SSE)

```bash
docker run -d --name whatsapp-gateway-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT="http" \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

### Production (HTTP+SSE with Auth)

```bash
docker run -d --name whatsapp-gateway-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT="http" \
  -e APP_ENV="production" \
  -e MCP_BASIC_AUTH_USER="admin" \
  -e MCP_BASIC_AUTH_PASSWORD="secure_password" \
  -e WAGA_BASE_URL="https://your-gateway.com/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  --restart unless-stopped \
  glennprays/mcp-whatsapp-gateway:latest
```

## Next Steps

- [**Tools Reference**](#docs:mcp/tools-reference) - Complete list of available MCP tools
- [**Client Setup**](#docs:mcp/client-setup) - Configure your MCP client
