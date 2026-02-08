# WhatsApp Gateway Testing

This directory contains testing utilities for the WhatsApp Gateway.

## Webhook Test Server

A simple HTTP server to receive and display webhook notifications from the gateway.

### Usage

1. **Start the webhook server:**
   ```bash
   cd test
   go run webhook_server.go
   ```

   The server will listen on `http://localhost:8080/webhook` with HMAC secret `test-secret-123`.

2. **Register the webhook URL with the gateway:**
   ```bash
   # Make sure you have a valid JWT token in auth.txt
   BEARER_TOKEN=$(cat auth.txt)

   curl -X POST http://localhost:3004/access/webhook \
     -H "Authorization: Bearer $BEARER_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "url": "http://localhost:8080/webhook",
       "hmac_secret": "test-secret-123"
     }'
   ```

3. **Send test messages:**
   ```bash
   # Test text message
   curl -X POST http://localhost:3004/access/message/text \
     -H "Authorization: Bearer $BEARER_TOKEN" \
     -H "Content-Type: application/json" \
     -H "X-Trace-ID: test-trace-001" \
     -d '{
       "msisdn": "6285155487630",
       "message": "Test message"
     }'

   # Test image message
   curl -X POST http://localhost:3004/access/message/image \
     -H "Authorization: Bearer $BEARER_TOKEN" \
     -H "X-Trace-ID: test-trace-002" \
     -F "msisdn=6285155487630" \
     -F "caption=Test image" \
     -F "image=@test-image.png"
   ```

4. **Expected webhooks:**

   **Queue Mode (RabbitMQ enabled):**
   - `message.queued` - Sent immediately after queuing
   - `message.sent` - Sent after worker processes successfully
   - `message.failed` - Sent if worker fails

   **Direct Mode (RabbitMQ disabled):**
   - `message.sent` - Sent immediately after successful send
   - `message.failed` - Sent immediately if send fails

   **Incoming Messages:**
   - Webhook sent when messages are received
   - Contains message content, sender info, etc.

### Webhook Output

The server displays:
- ✅ Timestamp of webhook receipt
- ✅ Event type (message.queued, message.sent, message.failed, etc.)
- ✅ Full JSON payload (pretty-printed)
- ✅ HMAC signature validation status

Example output:
```
================================================================================
[15:04:05] 📨 Webhook Received
================================================================================
Event: message.queued
{
  "event": "message.queued",
  "job_id": "abc123...",
  "to": "6285155487630",
  "phone_number": "6281234567890",
  "timestamp": 1704729845
}

HMAC Signature: sha256=abc123...
✅ HMAC Valid
================================================================================
```

## Testing Scenarios

### Test 1: Queue Mode with RabbitMQ

1. Ensure RabbitMQ is running:
   ```bash
   docker ps | grep rabbitmq
   ```

2. Set `.env`:
   ```
   RABBITMQ_ENABLED=true
   WEBHOOK_STATUS_EVENTS_ENABLED=true
   WEBHOOK_STATUS_EVENTS=message.sent,message.failed,message.queued
   ```

3. Send message and verify webhook sequence:
   - message.queued (immediate)
   - message.sent (after ~few seconds)

### Test 2: Direct Mode without RabbitMQ

1. Stop RabbitMQ:
   ```bash
   docker stop rabbitmq
   ```

2. Set `.env`:
   ```
   RABBITMQ_ENABLED=false
   WEBHOOK_STATUS_EVENTS_ENABLED=true
   WEBHOOK_STATUS_EVENTS=message.sent,message.failed
   ```

3. Send message and verify:
   - message.sent (immediate, synchronous)

### Test 3: Trace ID Propagation

1. Send message with custom trace ID:
   ```bash
   curl -X POST http://localhost:3004/access/message/text \
     -H "Authorization: Bearer $BEARER_TOKEN" \
     -H "Content-Type: application/json" \
     -H "X-Trace-ID: custom-trace-123" \
     -d '{"msisdn": "6285155487630", "message": "Test"}'
   ```

2. Check gateway logs - should show `custom-trace-123` throughout the request flow

### Test 4: Incoming Messages

1. Send a WhatsApp message TO your gateway number
2. Verify webhook received with message content

## Test Image

Create a test image for image message testing:
```bash
# Using ImageMagick
convert -size 100x100 xc:blue test-image.png

# Or download
curl -o test-image.png https://via.placeholder.com/100/0000FF/FFFFFF?text=Test
```

## Troubleshooting

**Webhook not received:**
- Check if webhook URL is registered: `GET /access/webhook`
- Check if status events are enabled in `.env`
- Verify gateway logs for webhook sending errors

**HMAC validation fails:**
- Ensure HMAC secret matches in both webhook server and registration
- Check `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` in `.env`

**Queue mode not working:**
- Verify RabbitMQ is running and accessible
- Check `RABBITMQ_URL` in `.env`
- Look for queue health errors in gateway logs
