# Client Setup

Configure your MCP client to connect to the WhatsApp Gateway server.

## Overview

The MCP WhatsApp Gateway supports multiple AI clients:

- **Claude Desktop** - Native desktop application
- **Cursor IDE** - AI-powered code editor
- **Open Code (VS Code)** - Microsoft's open-source code editor
- **Claude Code CLI** - Command-line interface for Claude

## Claude Desktop

### Prerequisites

- Claude Desktop installed ([Download](https://claude.ai/download))
- Docker installed and running
- MCP server running (see [Quick Start](/quick-start))

### Configuration File Location

| Platform | Configuration File Path |
|----------|------------------------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%/Claude/claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |

### Configuration

Add the following to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "WAGA_BASE_URL=http://host.docker.internal:3000/api/v1",
        "-e", "WAGA_JWT_TOKEN=your_jwt_token",
        "glennprays/mcp-whatsapp-gateway:latest"
      ]
    }
  }
}
```

### Using a Binary

If you prefer to use the binary instead of Docker:

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "/path/to/mcp-whatsapp-gateway",
      "args": [],
      "env": {
        "WAGA_BASE_URL": "http://localhost:3000/api/v1",
        "WAGA_JWT_TOKEN": "your_jwt_token"
      }
    }
  }
}
```

### Restart Claude Desktop

After updating the configuration, restart Claude Desktop to load the MCP server.

### Testing

Once Claude Desktop restarts, you can test the connection:

```
Use the send_text_message tool to send "Hello from Claude Desktop!" to 6282114759228@s.whatsapp.net
```

## Cursor IDE

### Prerequisites

- Cursor IDE installed ([Download](https://cursor.sh))
- Docker installed and running
- MCP server running (see [Quick Start](/quick-start))

### Configuration File Location

| Platform | Configuration File Path |
|----------|------------------------|
| macOS | `~/Library/Application Support/Cursor/User/globalStorage/mcp_servers.json` |
| Windows | `%APPDATA%/Cursor/User/globalStorage/mcp_servers.json` |
| Linux | `~/.config/Cursor/User/globalStorage/mcp_servers.json` |

### Configuration

Add the following to your MCP servers configuration:

```json
{
  "mcpServers": [
    {
      "name": "whatsapp-gateway",
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "WAGA_BASE_URL=http://host.docker.internal:3000/api/v1",
        "-e", "WAGA_JWT_TOKEN=your_jwt_token",
        "glennprays/mcp-whatsapp-gateway:latest"
      ]
    }
  ]
}
```

### Using a Binary

```json
{
  "mcpServers": [
    {
      "name": "whatsapp-gateway",
      "command": "/path/to/mcp-whatsapp-gateway",
      "args": [],
      "env": {
        "WAGA_BASE_URL": "http://localhost:3000/api/v1",
        "WAGA_JWT_TOKEN": "your_jwt_token"
      }
    }
  ]
}
```

### Restart Cursor IDE

Restart Cursor IDE to load the MCP server configuration.

### Testing

Test the connection in Cursor:

```
Send "Hello from Cursor!" to 6282114759228@s.whatsapp.net
```

## Open Code (VS Code)

### Prerequisites

- Visual Studio Code installed ([Download](https://code.visualstudio.com/))
- Docker installed and running
- MCP server running in HTTP+SSE mode (see [Quick Start](/quick-start))

### MCP Server Setup

First, ensure the MCP server is running with HTTP+SSE transport:

```bash
docker run -d --name whatsapp-gateway-mcp \
  -p 8080:8080 \
  -e MCP_TRANSPORT="http" \
  -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
  -e WAGA_JWT_TOKEN="your_jwt_token" \
  glennprays/mcp-whatsapp-gateway:latest
```

### Configuration File Location

| Platform | Configuration File Path |
|----------|------------------------|
| macOS | `~/Library/Application Support/Code/User/settings.json` |
| Windows | `%APPDATA%/Code/User/settings.json` |
| Linux | `~/.config/Code/User/settings.json` |

### Configuration

Add the MCP server URL to your VS Code settings:

```json
{
  "mcp.servers": {
    "whatsapp-gateway": {
      "url": "http://localhost:8080/mcp",
      "transport": "http"
    }
  }
}
```

### Using MCP Extension

Install the Model Context Protocol extension for VS Code:

1. Open VS Code
2. Go to Extensions (Ctrl+Shift+X)
3. Search for "Model Context Protocol"
4. Install the extension

### Testing

Test the connection:

```
Send "Hello from VS Code!" to 6282114759228@s.whatsapp.net
```

## Claude Code CLI

### Prerequisites

- Claude Code CLI installed ([Documentation](https://docs.anthropic.com/claude-code/overview))
- Docker installed and running
- MCP server running (see [Quick Start](/quick-start))

### Configuration

Claude Code CLI uses a `.mcp.json` file in your project directory or home directory.

#### Project-Specific Configuration

Create `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "WAGA_BASE_URL=http://host.docker.internal:3000/api/v1",
        "-e", "WAGA_JWT_TOKEN=your_jwt_token",
        "glennprays/mcp-whatsapp-gateway:latest"
      ]
    }
  }
}
```

#### Global Configuration

Add to `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "WAGA_BASE_URL=http://host.docker.internal:3000/api/v1",
        "-e", "WAGA_JWT_TOKEN=your_jwt_token",
        "glennprays/mcp-whatsapp-gateway:latest"
      ]
    }
  }
}
```

### Using a Binary

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "/path/to/mcp-whatsapp-gateway",
      "args": [],
      "env": {
        "WAGA_BASE_URL": "http://localhost:3000/api/v1",
        "WAGA_JWT_TOKEN": "your_jwt_token"
      }
    }
  }
}
```

### Testing

Test the connection in Claude Code CLI:

```
Send "Hello from Claude Code CLI!" to 6282114759228@s.whatsapp.net
```

## Troubleshooting

### MCP Server Not Starting

**Problem**: MCP server fails to start or connect

**Solutions**:

1. **Check Docker is running**:
   ```bash
   docker ps
   ```

2. **Verify gateway is accessible**:
   ```bash
   curl http://localhost:3000/api/v1/health
   ```

3. **Test MCP server directly**:
   ```bash
   docker run -i --rm \
     -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1" \
     -e WAGA_JWT_TOKEN="your_jwt_token" \
     glennprays/mcp-whatsapp-gateway:latest
   ```

### Tools Not Available

**Problem**: MCP tools are not available in the client

**Solutions**:

1. **Restart your client application** (Claude Desktop, Cursor, VS Code)
2. **Check configuration file syntax** (valid JSON)
3. **Verify configuration file path** is correct
4. **Check client logs** for error messages

### Connection Errors

**Problem**: "Failed to connect to MCP server"

**Solutions**:

1. **Verify MCP server is running**:
   ```bash
   docker ps | grep whatsapp-gateway-mcp
   ```

2. **Check for port conflicts**:
   ```bash
   lsof -i :8080  # For HTTP+SSE transport
   ```

3. **Test MCP server endpoint**:
   ```bash
   curl http://localhost:8080/mcp
   ```

### Docker Networking Issues

**Problem**: Container cannot reach gateway on host

**Solutions**:

1. **Use host.docker.internal** (macOS/Windows):
   ```bash
   -e WAGA_BASE_URL="http://host.docker.internal:3000/api/v1"
   ```

2. **Use Docker bridge IP** (Linux):
   ```bash
   -e WAGA_BASE_URL="http://172.17.0.1:3000/api/v1"
   ```

3. **Use host network** (Linux only):
   ```bash
   docker run --network host ...
   ```

### JWT Token Issues

**Problem**: "401 Unauthorized" or "Invalid JWT token"

**Solutions**:

1. **Re-register your phone number** with the gateway to get a new token
2. **Verify token is correctly set** in environment variables
3. **Check for extra spaces or quotes** in the token
4. **Test token with curl**:
   ```bash
   curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
        http://localhost:3000/api/v1/health
   ```

## Configuration Examples

### Claude Desktop (Docker)

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "WAGA_BASE_URL=http://host.docker.internal:3000/api/v1",
        "-e", "WAGA_JWT_TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "glennprays/mcp-whatsapp-gateway:latest"
      ]
    }
  }
}
```

### Cursor IDE (Binary)

```json
{
  "mcpServers": [
    {
      "name": "whatsapp-gateway",
      "command": "/usr/local/bin/mcp-whatsapp-gateway",
      "args": [],
      "env": {
        "WAGA_BASE_URL": "http://localhost:3000/api/v1",
        "WAGA_JWT_TOKEN": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "LOG_LEVEL": "debug"
      }
    }
  ]
}
```

### Open Code (HTTP+SSE)

```json
{
  "mcp.servers": {
    "whatsapp-gateway": {
      "url": "http://localhost:8080/mcp",
      "transport": "http"
    }
  }
}
```

### Claude Code CLI (Project-Specific)

```json
{
  "mcpServers": {
    "whatsapp-gateway": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "WAGA_BASE_URL=http://host.docker.internal:3000/api/v1",
        "-e", "WAGA_JWT_TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "glennprays/mcp-whatsapp-gateway:latest"
      ]
    }
  }
}
```

## Next Steps

- [**Tools Reference**](#docs:mcp/tools-reference) - Learn about available MCP tools
- [**Configuration**](#docs:mcp/configuration) - Advanced server configuration
- [**Quick Start**](#docs:mcp/quick-start) - Get started quickly
