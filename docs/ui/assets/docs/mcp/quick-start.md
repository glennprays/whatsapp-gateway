# Quick Start

Get the MCP WhatsApp Gateway server running in minutes using Docker.

## Prerequisites

Before you begin, ensure you have:

- **Docker installed** on your system ([Download Docker](https://www.docker.com/products/docker-desktop))
- **Running WhatsApp Gateway** instance ([Setup Guide](https://waga.glennprays.com))
- **JWT Token** from your gateway registration ([Get Token](/getting-started/introduction))

## Step 1: Pull the Docker Image

```bash
docker pull glennprays/mcp-whatsapp-gateway:latest
```

## Step 2: Run the MCP Server

### For Claude Desktop / Cursor / Claude Code CLI (stdio transport)

```bash
docker run -i --rm \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

**What this does:**
- Runs the MCP server in interactive mode (`-i`)
- Removes the container when it exits (`--rm`)
- Sets the gateway URL to `host.docker.internal` (Docker alias for your host)
- Passes your JWT token for authentication

### For Web-Based Clients (HTTP+SSE transport)

```bash
docker run -d --name whatsapp-gateway-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT="http" \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

**What this does:**
- Runs the MCP server in detached mode (`-d`)
- Names the container `whatsapp-gateway-mcp`
- Exposes port 8080 for HTTP+SSE connections
- Sets the transport mode to HTTP

### For Production (with authentication)

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

## Step 3: Configure Your MCP Client

Once the MCP server is running, configure your AI client:

### Claude Desktop

See [Client Setup - Claude Desktop](#docs:mcp/client-setup)

### Cursor IDE

See [Client Setup - Cursor IDE](#docs:mcp/client-setup)

### Open Code (VS Code)

See [Client Setup - Open Code](#docs:mcp/client-setup)

### Claude Code CLI

See [Client Setup - Claude Code CLI](#docs:mcp/client-setup)

## Step 4: Test the Connection

### Test stdio Transport

If running in stdio mode, the server will start and wait for MCP protocol messages. Configure your client and try sending a test message:

```
Use the send_text_message tool to send a message to +1234567890
```

### Test HTTP+SSE Transport

If running in HTTP+SSE mode, verify the server is accessible:

```bash
curl http://localhost:8080/mcp
```

You should see the MCP server endpoint respond.

## Troubleshooting

### Gateway Connection Failed

**Problem**: "Failed to initialize gateway client" or "Gateway health check failed"

**Solutions**:

1. **Verify Gateway is Running**:
   ```bash
   curl http://localhost:3000/api/v1/health
   ```

2. **Check WAGA_BASE_URL**:
   - Local development: `http://localhost:3000/api/v1`
   - Docker: Use `host.docker.internal` on Mac/Windows
   - Remote: Ensure firewall allows connections

3. **Validate JWT Token**:
   ```bash
   curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
        http://localhost:3000/api/v1/health
   ```

### 401 Unauthorized

**Problem**: "JWT token is invalid or expired"

**Solution**: Re-register your phone number with the gateway to get a new token.

### 403 Forbidden

**Problem**: "Session may be disconnected"

**Solution**: Your WhatsApp session may have disconnected. Use the `check_connection_status` tool to verify and reconnect if needed.

### Docker Networking Issues

**Problem**: Container cannot reach host services

**Solution**: Use the appropriate host address:
- **macOS/Windows**: `host.docker.internal`
- **Linux**: Use host's IP address or `172.17.0.1` (default Docker bridge)

```bash
# Example for macOS/Windows
docker run -i --rm \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

## Next Steps

- [**Configuration**](#docs:mcp/configuration) - Detailed configuration options and environment variables
- [**Tools Reference**](#docs:mcp/tools-reference) - Complete list of available MCP tools
- [**Client Setup**](#docs:mcp/client-setup) - Configure your MCP client in detail
