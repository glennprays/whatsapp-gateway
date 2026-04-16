# MCP Tools Reference

Complete reference for all MCP tools exposed by the WhatsApp Gateway server.

## Overview

The MCP WhatsApp Gateway exposes tools organized into three categories:

1. **Messaging Tools** - Send, edit, delete, and react to messages
2. **Connection Tools** - Check gateway status and health
3. **Webhook Tools** - Manage webhook configuration

## Messaging Tools

### send_text_message

Send a text message to a WhatsApp contact or group.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `message` | string | Yes | Text message content |

**Recipient Format:**

- **Individual**: `{phone}@s.whatsapp.net`
  - Example: `6281234567890@s.whatsapp.net`
  - Note: Use country code without `+` or `00`
- **Group**: `{group_id}@g.us`
  - Example: `120363xxxxx@g.us`

**Returns:**

```json
{
  "message_id": "3EB0xxxxxxxxxxxxx",
  "status": "sent",
  "to": "6281234567890@s.whatsapp.net"
}
```

**Example:**

```
Send "Hello from Claude!" to 6281234567890@s.whatsapp.net
```

### send_image_message

Send an image message to a WhatsApp contact or group.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `image_url` | string | Yes | Public URL of the image to send |
| `caption` | string | No | Image caption/description |
| `view_once` | boolean | No | Whether image should be view-once (default: false) |

**Recipient Format:** Same as `send_text_message`

**Returns:**

```json
{
  "message_id": "3EB0xxxxxxxxxxxxx",
  "status": "sent",
  "to": "6281234567890@s.whatsapp.net",
  "type": "image"
}
```

**Example:**

```
Send image from https://example.com/photo.jpg to 6281234567890@s.whatsapp.net with caption "Check this out!"
```

### edit_message

Edit a previously sent message.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `message_id` | string | Yes | ID of the message to edit |
| `new_message` | string | Yes | New message content |

**Recipient Format:** Same as `send_text_message`

**Returns:**

```json
{
  "status": "success",
  "message_id": "3EB0xxxxxxxxxxxxx"
}
```

**Example:**

```
Edit message 3EB0xxxxxxxxxxxxx for 6281234567890@s.whatsapp.net to say "Updated message"
```

### delete_message

Delete a previously sent message.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `message_id` | string | Yes | ID of the message to delete |

**Recipient Format:** Same as `send_text_message`

**Returns:**

```json
{
  "status": "success",
  "message_id": "3EB0xxxxxxxxxxxxx"
}
```

**Example:**

```
Delete message 3EB0xxxxxxxxxxxxx for 6281234567890@s.whatsapp.net
```

### react_to_message

React to a message with an emoji.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `message_id` | string | Yes | ID of the message to react to |
| `emoji` | string | Yes | Emoji reaction |

**Recipient Format:** Same as `send_text_message`

**Supported Emojis:** Any standard Unicode emoji

**Returns:**

```json
{
  "status": "success",
  "message_id": "3EB0xxxxxxxxxxxxx",
  "emoji": "👍"
}
```

**Example:**

```
React to message 3EB0xxxxxxxxxxxxx for 6281234567890@s.whatsapp.net with emoji 👍
```

## Connection Tools

### check_connection_status

Check if the WhatsApp session is active and authenticated.

**Parameters:** None

**Returns:**

```json
{
  "authenticated": true,
  "phone_number": "6281234567890",
  "connected": true,
  "connection_status": "connected"
}
```

**Status Values:**

- `authenticated`: Whether the session has a valid JWT token
- `phone_number`: The registered phone number
- `connected`: Whether WhatsApp connection is active
- `connection_status`: Detailed status (connected, connecting, disconnected)

**Example:**

```
Check the WhatsApp connection status
```

### check_health

Check if the WhatsApp Gateway service is reachable and healthy.

**Parameters:** None

**Returns:**

```json
{
  "status": "healthy",
  "gateway_version": "1.0.0",
  "timestamp": "2024-01-15T10:30:45Z"
}
```

**Status Values:**

- `healthy`: Gateway is operating normally
- `unhealthy`: Gateway has issues (check logs)

**Example:**

```
Check the gateway health status
```

## Webhook Tools

### get_webhook

Get the currently registered webhook URL and configuration.

**Parameters:** None

**Returns:**

```json
{
  "webhook_url": "https://example.com/webhook",
  "registered": true,
  "hmac_enabled": true
}
```

**Example:**

```
Get the current webhook configuration
```

### register_webhook

Register a webhook URL to receive incoming message notifications.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | Webhook URL to register |
| `hmac_secret` | string | No | HMAC secret for signature verification |

**Returns:**

```json
{
  "status": "success",
  "webhook_url": "https://example.com/webhook",
  "hmac_enabled": true
}
```

**Example:**

```
Register webhook URL https://example.com/webhook with HMAC secret my_secret_key
```

**Webhook Payload Example:**

```json
{
  "event": "message.received",
  "data": {
    "message_id": "3EB0xxxxxxxxxxxxx",
    "from": "6289876543210@s.whatsapp.net",
    "message": "Hello!",
    "timestamp": "2024-01-15T10:30:45Z"
  },
  "signature": "sha256=..."
}
```

### delete_webhook

Remove the registered webhook configuration.

**Parameters:** None

**Returns:**

```json
{
  "status": "success",
  "message": "Webhook deleted successfully"
}
```

**Example:**

```
Delete the current webhook configuration
```

## Error Handling

All tools return structured error messages when operations fail.

### Common Error Responses

**401 Unauthorized:**

```json
{
  "error": "JWT token is invalid or expired. Re-register the phone number against the gateway to obtain a new token."
}
```

**Solution**: Re-register your phone number with the gateway to get a new JWT token.

**403 Forbidden:**

```json
{
  "error": "Session may be disconnected. Run check_connection_status to verify."
}
```

**Solution**: Use `check_connection_status` to verify your WhatsApp session and reconnect if needed.

**500 Internal Server Error:**

```json
{
  "error": "Internal server error. Check gateway logs for details.",
  "trace_id": "abc123-def456"
}
```

**Solution**: Check the WhatsApp Gateway logs and include the trace ID when reporting issues.

## Tool Usage Patterns

### Send and React Workflow

```
1. Send text message to 6281234567890@s.whatsapp.net
2. React to message [message_id] with emoji 👍
```

### Check Before Send

```
1. Check the WhatsApp connection status
2. If connected, send "Hello!" to 6281234567890@s.whatsapp.net
```

### Webhook Setup

```
1. Register webhook URL https://example.com/webhook with HMAC secret my_secret
2. Get the current webhook configuration to verify
```

## Tips

1. **Always use JID format** for recipient addresses (include `@s.whatsapp.net` or `@g.us`)
2. **Use international format** for phone numbers (country code without `+` or `00`)
3. **Check connection status** before sending important messages
4. **Use trace IDs** from error responses when debugging issues
5. **Test with `check_health`** first to verify gateway connectivity

## Next Steps

- [**Client Setup**](/client-setup) - Configure your MCP client
- [**Configuration**](/configuration) - Server configuration options
