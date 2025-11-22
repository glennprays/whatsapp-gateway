# Gateway Usage Flow

This guide explains the complete flow for using the WhatsApp Gateway API, from registration to sending messages and receiving webhooks.

## 📋 Overview

The WhatsApp Gateway follows a simple workflow:

1. **Register** - Get a JWT token for accessing the gateway
2. **Login** - Connect to WhatsApp using QR code or pairing code
3. **Configure Webhook** - Set up webhook to receive WhatsApp events
4. **Send/Receive Messages** - Use the API to interact with WhatsApp

## 🔐 Step 1: Register and Get JWT Token

Before using any gateway endpoint, you must first register to obtain a JWT token.

### Endpoint: `POST /api/v1/register`

**Request Body:**
```json
{
  "phone_number": "6281234567890",
  "secret_key": "your_secret_key"
}
```

**Example using cURL:**
```bash
curl -X POST http://localhost:3000/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "phone_number": "6281234567890",
    "secret_key": "my_secret_key_123"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Important Notes:**
- The `phone_number` should be in international format without '+' or spaces (e.g., "6281234567890" for Indonesia)
- The `secret_key` is your authentication secret (configured in the gateway's environment)
- Save the returned JWT token - you'll need it for all subsequent requests
- The token expires after the duration specified in `JWT_DURATION_MINUTES` (default: 60 minutes)

### Using the JWT Token

Include the JWT token in the `Authorization` header for all subsequent API calls:

```bash
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## 📱 Step 2: Login to WhatsApp Account

After registering, you need to authenticate with WhatsApp. There are two methods:

### Method A: QR Code Login

This is the most common method, similar to WhatsApp Web.

#### Endpoint: `POST /api/v1/login/qr_code/{format}`

**Parameters:**
- `format`: Either `json` or `html`

**Example (JSON format):**
```bash
curl -X POST http://localhost:3000/api/v1/login/qr_code/json \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response (JSON):**
```json
{
  "qr_code": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
  "expires_in": 300
}
```

**Example (HTML format):**
```bash
curl -X POST http://localhost:3000/api/v1/login/qr_code/html \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response (HTML):**
```html
<img src='data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...'/>
```

**Steps:**
1. Request a QR code from the endpoint
2. Display the QR code (it's a base64-encoded PNG image)
3. Open WhatsApp on your phone → Settings → Linked Devices → Link a Device
4. Scan the displayed QR code
5. Wait for authentication to complete (the QR code expires after 5 minutes)

### Method B: Pairing Code Login

Alternative method using an 8-digit pairing code.

#### Endpoint: `POST /api/v1/login/pair_code`

**Example:**
```bash
curl -X POST http://localhost:3000/api/v1/login/pair_code \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response:**
```json
{
  "pair_code": "12345678",
  "expires_in": 300
}
```

**Steps:**
1. Request a pairing code from the endpoint
2. Open WhatsApp on your phone → Settings → Linked Devices → Link a Device
3. Select "Link with phone number instead"
4. Enter the 8-digit pairing code
5. Wait for authentication to complete (the code expires after 5 minutes)

### Check Login Status

You can check if you're logged in at any time:

#### Endpoint: `GET /api/v1/login/status`

**Example:**
```bash
curl -X GET http://localhost:3000/api/v1/login/status \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response:**
```json
{
  "authenticated": true
}
```

## 🔔 Step 3: Configure Webhook (Optional but Recommended)

Webhooks allow the gateway to notify your backend when WhatsApp events occur.

### What Events Are Supported?

Currently, the gateway sends webhook notifications for:
- **Incoming messages** - When you receive a message on WhatsApp

**Note:** For detailed information about webhook payload structure and additional events, refer to the [Swagger documentation](../docs/swagger.yaml).

### Register a Webhook

#### Endpoint: `POST /api/v1/webhook`

**Request Body:**
```json
{
  "url": "https://your-backend.com/webhook/whatsapp",
  "hmac_secret": "optional_secret_for_signature_verification"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/v1/webhook \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-backend.com/webhook/whatsapp",
    "hmac_secret": "my_webhook_secret_123"
  }'
```

**Response:**
```json
{
  "success": true
}
```

**Parameters:**
- `url` (required): Your backend endpoint that will receive webhook notifications
- `hmac_secret` (optional): If provided, the gateway will sign webhook payloads with HMAC for verification

### Webhook Payload

When a WhatsApp event occurs, the gateway will send a POST request to your webhook URL:

**Example Incoming Message Event:**
```json
{
  "event": "message",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "message_id": "msg_123456",
    "from": "6281234567890@s.whatsapp.net",
    "text": "Hello from WhatsApp!",
    "timestamp": 1705318200
  }
}
```

### Verifying Webhook Signatures

If you provided an `hmac_secret`, webhook requests will include an `X-Signature` header containing the HMAC signature. Verify it like this:

**Python Example:**
```python
import hmac
import hashlib

def verify_webhook(payload, signature, secret):
    expected = hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)
```

**Node.js Example:**
```javascript
const crypto = require('crypto');

function verifyWebhook(payload, signature, secret) {
  const expected = crypto
    .createHmac('sha256', secret)
    .update(payload)
    .digest('hex');
  return crypto.timingSafeEqual(
    Buffer.from(expected),
    Buffer.from(signature)
  );
}
```

### Managing Webhooks

**Get current webhook:**
```bash
curl -X GET http://localhost:3000/api/v1/webhook \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Delete webhook:**
```bash
curl -X DELETE http://localhost:3000/api/v1/webhook \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 💬 Step 4: Send Messages

Once logged in, you can start sending messages.

### Send Text Message

#### Endpoint: `POST /api/v1/message/text`

**Request Body:**
```json
{
  "msisdn": "6281234567890@s.whatsapp.net",
  "message": "Hello from the gateway!"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/v1/message/text \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "msisdn": "6281234567890@s.whatsapp.net",
    "message": "Hello from the gateway!"
  }'
```

**Response:**
```json
{
  "success": true,
  "message_id": "msg_1234567890"
}
```

**Note:** The `msisdn` format:
- For personal chat: `[phone_number]@s.whatsapp.net` (e.g., "6281234567890@s.whatsapp.net")
- For group chat: `[group_id]@g.us` (e.g., "123456789@g.us")

### Send Image Message

#### Endpoint: `POST /api/v1/message/image`

**Example:**
```bash
curl -X POST http://localhost:3000/api/v1/message/image \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "msisdn=6281234567890@s.whatsapp.net" \
  -F "image=@/path/to/image.jpg" \
  -F "caption=Check out this image!" \
  -F "is_view_once=false"
```

### Other Message Operations

**React to a message:**
```bash
curl -X POST http://localhost:3000/api/v1/message/react \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message_id": "msg_1234567890",
    "msisdn": "6281234567890@s.whatsapp.net",
    "emoji": "👍"
  }'
```

**Edit a message:**
```bash
curl -X PUT http://localhost:3000/api/v1/message \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message_id": "msg_1234567890",
    "msisdn": "6281234567890@s.whatsapp.net",
    "new_message": "Edited message text"
  }'
```

**Delete a message:**
```bash
curl -X DELETE http://localhost:3000/api/v1/message \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message_id": "msg_1234567890",
    "msisdn": "6281234567890@s.whatsapp.net"
  }'
```

## 🔄 Session Management

### Reconnect to Session

If the connection is lost, you can attempt to reconnect:

#### Endpoint: `POST /api/v1/session/reconnect`

```bash
curl -X POST http://localhost:3000/api/v1/session/reconnect \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Logout

To disconnect from WhatsApp:

#### Endpoint: `POST /api/v1/logout`

```bash
curl -X POST http://localhost:3000/api/v1/logout \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Warning:** This will unlink the device from WhatsApp. You'll need to scan a QR code again to reconnect.

## 📊 Complete Workflow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Register                                                  │
│    POST /register                                            │
│    → Get JWT Token                                           │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Login to WhatsApp                                         │
│    POST /login/qr_code/json  OR  POST /login/pair_code      │
│    → Scan QR / Enter Pair Code on Phone                     │
│    → Wait for Authentication                                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Configure Webhook (Optional)                              │
│    POST /webhook                                             │
│    → Receive incoming message notifications                  │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Use WhatsApp Features                                     │
│    • Send messages (POST /message/text)                      │
│    • Send images (POST /message/image)                       │
│    • React to messages (POST /message/react)                 │
│    • Edit messages (PUT /message)                            │
│    • Delete messages (DELETE /message)                       │
│    • Check status (GET /login/status)                        │
└─────────────────────────────────────────────────────────────┘
```

## 📖 Additional Resources

For detailed API specifications and all available endpoints, refer to:
- [Swagger Documentation](../docs/swagger.yaml) - Complete API reference with request/response examples

## ⚠️ Important Notes

1. **Token Management**: JWT tokens expire after the configured duration. Implement token refresh in your backend.
2. **Webhook Reliability**: Ensure your webhook endpoint is always accessible and responds quickly (< 5 seconds).
3. **Rate Limiting**: WhatsApp may rate limit your requests. Implement appropriate backoff strategies.
4. **Phone Number Format**: Always use international format without '+' (e.g., "6281234567890")
5. **Message IDs**: Save message IDs returned from send operations for later reference (editing, deleting, etc.)

## 📚 Next Steps

- [Security Considerations](Security-Considerations.md) - **Important security warnings** (must read!)
- [Environment Variables](Environment-Variables.md) - Configure your gateway
- [Development Guide](Development-Guide.md) - Set up your development environment

---

[← Back to Home](Home.md)
