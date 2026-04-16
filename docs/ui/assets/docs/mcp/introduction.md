# MCP WhatsApp Gateway

## Overview

The **MCP WhatsApp Gateway** is a Model Context Protocol (MCP) server that acts as a bridge between AI agents and the WhatsApp Gateway. It allows AI assistants like Claude, Cursor, and Claude Code to send WhatsApp messages, manage webhooks, and check connection status through a pre-authenticated JWT session.

## What is MCP?

The **Model Context Protocol (MCP)** is an open protocol that enables AI agents to interact with external systems through standardized tools. Think of MCP as a universal translator that allows AI assistants to:

- Send messages to WhatsApp contacts and groups
- Manage WhatsApp webhooks for incoming messages
- Check WhatsApp connection status
- Edit, delete, and react to sent messages

## Architecture

```
┌─────────────────────┐     MCP Protocol      ┌──────────────────────┐     HTTP/JWT     ┌─────────────────────┐
│  AI Agent           │ ←────────────────────→ │  MCP WhatsApp        │ ←───────────────→ │  WhatsApp           │
│  (Claude/Cursor     │   (stdio or HTTP+SSE)   │  Gateway Server      │   (REST API)      │  Gateway            │
│   Claude Code)      │                         │                      │                   │  (waga)             │
└─────────────────────┘                         └──────────────────────┘                   └─────────────────────┘
```

### Data Flow

1. **AI Agent** calls MCP tools (e.g., `send_text_message`)
2. **MCP Server** receives the tool call and validates input
3. **Gateway Client** makes HTTP request to WhatsApp Gateway with JWT authentication
4. **WhatsApp Gateway** processes the request and interacts with WhatsApp
5. **Response** flows back through the chain to the AI agent

### Key Points

- The **WhatsApp Gateway runs as a separate service** that you need to deploy
- This MCP server connects to it via **HTTP using JWT authentication**
- The gateway handles all WhatsApp-specific logic and protocol details
- This MCP server only provides the MCP protocol interface for AI agents

## Features

### Messaging Tools

- **Send text messages** to WhatsApp contacts and groups
- **Send image messages** with optional captions
- **Edit messages** to correct typos or update content
- **Delete messages** to remove sent messages
- **React to messages** with emoji reactions

### Connection Management

- **Check connection status** to verify WhatsApp session is active
- **Health monitoring** to ensure gateway service is reachable

### Webhook Management

- **Get webhook configuration** to view current webhook URL
- **Register webhooks** to receive incoming message notifications
- **Delete webhooks** to remove webhook configuration

## Transport Support

The MCP WhatsApp Gateway supports two transport modes:

### stdio Transport (Default)

Best for: **Claude Desktop, Cursor, Claude Code CLI**

- Uses standard input/output for communication
- Runs as a subprocess of the AI agent
- Simple configuration with environment variables

### HTTP+SSE Transport

Best for: **Web-based MCP clients, Open Code**

- Runs as a standalone HTTP server
- Uses Server-Sent Events (SSE) for real-time updates
- Supports Basic Authentication in production mode
- Can be accessed by multiple clients simultaneously

## Why Use MCP WhatsApp Gateway?

### For AI Agents

- **Pre-authenticated**: No need to handle QR codes or pairing flows
- **Simple API**: Clean tool interfaces for common WhatsApp operations
- **Type-safe**: Input validation and structured responses
- **Traceability**: All operations include trace IDs for debugging

### For Developers

- **Separation of Concerns**: AI logic separate from WhatsApp protocol
- **Scalability**: MCP server and Gateway can be deployed independently
- **Security**: JWT tokens provide secure authentication
- **Flexibility**: Multiple AI agents can connect to the same gateway

## Prerequisites

Before using the MCP WhatsApp Gateway, you need:

1. **Running WhatsApp Gateway instance**
   - Deploy the gateway service (Docker, binary, or cloud)
   - Ensure it's accessible via HTTP/HTTPS

2. **JWT Token from Gateway**
   - Register your phone number with the gateway
   - Obtain a JWT token for authentication
   - This token is used by the MCP server to authenticate with the gateway

3. **Gateway Configuration**
   - Set `WAGA_BASE_URL` to point to your gateway instance
   - Example: `http://localhost:3000/api/v1` (local development)
   - Example: `https://waga.example.com/api/v1` (production)

## Next Steps

- [**Quick Start**](#docs:mcp/quick-start) - Get up and running with Docker in minutes
- [**Configuration**](#docs:mcp/configuration) - Detailed configuration options and environment variables
- [**Tools Reference**](#docs:mcp/tools-reference) - Complete list of available MCP tools
- [**Client Setup**](#docs:mcp/client-setup) - Configure your MCP client (Claude, Cursor, Open Code, etc.)
