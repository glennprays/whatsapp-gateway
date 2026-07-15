# MCP Tools Reference

Complete reference for all MCP tools exposed by the WhatsApp Gateway server.

## Overview

The MCP WhatsApp Gateway exposes a **curated read-and-messaging subset** of the gateway API (26 tools), organized into these categories:

1. **Messaging Tools** - Send (text/image/audio/video/document/location/poll/sticker), edit, delete, and react to messages, with canonical `chat` addressing and optional reply/mention threading
2. **Contact & Group Read Tools** - List contacts, look up profiles and avatars, list groups, and read one group's roster
3. **Conversation Tools** - Mark messages read and set the typing indicator
4. **Connection Tools** - Check gateway status, health, and whether a number is on WhatsApp
5. **Webhook Tools** - Manage webhook configuration

### Not exposed (excluded by design)

This MCP deliberately exposes only read and messaging capabilities. It does **not** expose:

- **Group/community mutations** - create, leave, participants, settings, name, topic, photo, invite, join, requests
- **Community operations** - sub-group linking/unlinking, community participants
- **Admin plane** - operator session inventory (`/admin/sessions`)
- **Metrics** - the Prometheus `/metrics` endpoint

The underlying gateway and Go SDK support all of these, but keeping them out of the MCP prevents an autonomous agent from performing destructive or account-wide actions. To perform those operations, call the gateway's REST API directly from a trusted backend.

## Messaging Tools

### Common send arguments

Every send tool below accepts canonical **chat addressing** plus optional reply/mention threading, in addition to its tool-specific fields:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes* | Canonical recipient: a bare number, a user JID (`@s.whatsapp.net`), a group JID (`@g.us`), or a `@lid`. Preferred; wins over `to` when both are set. |
| `to` | string | Yes* | Back-compat recipient alias. |
| `reply_to_id` | string | No | Quote an existing message by id. |
| `reply_to_sender` | string | No | Author JID/number of the quoted message. |
| `reply_to_text` | string | No | Caller-supplied preview of the quoted text (the gateway is storeless and does not look it up). |
| `mentions` | array of strings | No | Numbers/JIDs to @-tag in the message. |

\* Either `chat` or `to` is required.

### send_text_message

Send a text message to a WhatsApp contact or group.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes* | Canonical recipient (preferred): see [Common send arguments](#common-send-arguments) |
| `to` | string | Yes* | Back-compat recipient alias. *Either `chat` or `to` is required. |
| `message` | string | Yes | Text message content |

Reply/mention fields (`reply_to_id`, `reply_to_sender`, `reply_to_text`, `mentions`) from [Common send arguments](#common-send-arguments) apply here too.

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

### send_audio_message

Send an audio message. `is_ptt=true` renders a push-to-talk voice-note bubble (opus/ogg); `false` sends a playable audio-file card.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `audio_url` | string | Yes | Public URL of the audio file to send |
| `is_ptt` | boolean | No | Render as a voice-note bubble (default: false) |
| `view_once` | boolean | No | Whether audio should be view-once (default: false) |

**Returns:** Same shape as `send_image_message`, with `"type": "audio"`.

**Example:**

```
Send a voice note from https://example.com/note.ogg to 6281234567890@s.whatsapp.net
```

### send_video_message

Send a video message with an optional caption.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `video_url` | string | Yes | Public URL of the video file to send |
| `caption` | string | No | Video caption |
| `is_gif` | boolean | No | Toggle GIF-like rendering (default: false) |
| `view_once` | boolean | No | Whether video should be view-once (default: false) |

**Returns:** Same shape as `send_image_message`, with `"type": "video"`.

### send_document_message

Send a document/file of any mimetype with a visible file name and optional caption.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient address in JID format |
| `document_url` | string | Yes | Public URL of the document to send |
| `file_name` | string | No | Visible file name (defaults to the uploaded filename, then "file") |
| `caption` | string | No | Document caption |

**Returns:** Same shape as `send_image_message`, with `"type": "document"`.

### check_contact

Validate whether a number is registered on WhatsApp before sending (uses `IsOnWhatsApp`).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `msisdn` | string | Yes | Phone number or JID to validate |

**Returns:**

```json
{
  "query": "6281234567890",
  "jid": "6281234567890@s.whatsapp.net",
  "is_on_whatsapp": true,
  "verified_name": null
}
```

**Example:**

```
Check if 6281234567890 is on WhatsApp
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

## Contact & Group Read Tools

### list_contacts

List the account's locally-synced WhatsApp contacts. Reads the local address book; an empty result is normal, never an error.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | integer | No | Page size. Gateway default 100, max 500. |
| `offset` | integer | No | Pagination offset. |

**Returns:** `contacts[]` (`jid`, `push_name`, `full_name`, `first_name`, `business_name`), `count`, `total`, and an optional `note`.

### get_contact_info

Look up one contact's WhatsApp profile.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes | Canonical recipient: a number, user JID, or `@lid`. |

**Returns:** `jid`, `status`, `picture_id`, `verified_name`, `device_count`, `lid`.

### get_avatar

Get a chat's profile picture URL (user or group). Soft failures are surfaced as results, not errors.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes | Canonical recipient (user or `@g.us` group). |
| `preview` | boolean | No | Request the low-res thumbnail instead of full resolution. |

**Returns:** `jid`, `available` (boolean). When available: `url` (time-limited CDN link), `id` (ETag), `type` (`image`/`preview`). When not: `available=false` with `reason` `not_set` (404, no picture) or `hidden` (403, privacy).

### list_groups

List the account's joined groups as lightweight summaries (no participant roster; use `get_group_info` for one group's full detail).

**Parameters:** None

**Returns:** `groups[]` (`jid`, `name`, `topic`, `owner_jid`, `participant_count`, `is_announce`, `is_locked`, `is_community`) and `count`. Server-hitting read subject to a per-account budget (`429` when exhausted).

### get_group_info

Get one group's full detail plus its participant roster.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes | A group JID (`@g.us`). The account must be a member. |

**Returns:** Group detail plus `participants[]` (`jid`, `phone_number`, `lid`, `is_admin`, `is_super_admin`). `403` if not a member, `404` if absent.

## Conversation Tools

> These are conversation-affecting **outbound** actions. They are governed by the gateway's outbound pacer (per-account pace + per-recipient cap); over-budget calls are paced or rejected with `429`.

### mark_read

Mark one or more messages in a chat as read (blue ticks).

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes | Canonical recipient. |
| `message_ids` | array of strings | Yes | Message IDs to mark read. |
| `sender` | string | No | Message author's JID/number: required for group chats. |

**Returns:** Success status.

### send_typing

Set the typing indicator in a chat.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat` | string | Yes | Canonical recipient. |
| `state` | string | Yes | One of `composing` (typing…), `recording` (recording audio…), or `paused` (cleared). |

**Returns:** Success status and the applied state.

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

- [**Client Setup**](#docs:mcp/client-setup) - Configure your MCP client
- [**Configuration**](#docs:mcp/configuration) - Server configuration options
