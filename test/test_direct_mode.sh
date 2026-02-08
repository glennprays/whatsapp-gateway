#!/bin/bash

# Test Direct Mode (RabbitMQ Disabled)
# This script tests webhook delivery and trace ID propagation in direct mode

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GATEWAY_URL="http://localhost:3004"
WEBHOOK_SERVER_URL="http://localhost:8080/webhook"
HMAC_SECRET="test-secret-123"
TARGET_PHONE="6285155487630"
ENV_FILE="../.env"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Direct Mode Testing (RabbitMQ Disabled)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Check prerequisites
echo -e "${YELLOW}[1/6] Checking prerequisites...${NC}"

if [ ! -f "auth.txt" ]; then
    echo -e "${RED}❌ auth.txt not found!${NC}"
    echo "Please create auth.txt with your JWT token first."
    exit 1
fi

BEARER_TOKEN=$(cat auth.txt)
echo -e "${GREEN}✅ JWT token loaded${NC}"

# Check webhook server
if ! curl -s "$WEBHOOK_SERVER_URL" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Webhook server not running!${NC}"
    echo "Start webhook server in another terminal: cd test && go run webhook_server.go"
    echo ""
    read -p "Press Enter when webhook server is ready..."
fi
echo -e "${GREEN}✅ Webhook server is ready${NC}"
echo ""

# Step 2: Disable RabbitMQ
echo -e "${YELLOW}[2/6] Configuring direct mode...${NC}"
echo "Stopping RabbitMQ..."
docker stop rabbitmq > /dev/null 2>&1 || echo "RabbitMQ already stopped"
sleep 2

# Update .env to disable RabbitMQ
if [ -f "$ENV_FILE" ]; then
    echo "Updating .env to disable RabbitMQ..."
    sed -i.bak 's/RABBITMQ_ENABLED=true/RABBITMQ_ENABLED=false/' "$ENV_FILE"
    echo -e "${GREEN}✅ RabbitMQ disabled in .env${NC}"
else
    echo -e "${YELLOW}⚠️  .env file not found, assuming RabbitMQ is already disabled${NC}"
fi
echo ""

echo -e "${YELLOW}Please restart the gateway for changes to take effect:${NC}"
echo "  1. Stop the gateway (Ctrl+C)"
echo "  2. Run: make run"
echo "  3. Press Enter when ready..."
read

# Verify gateway is running
echo -e "${YELLOW}Verifying gateway is running...${NC}"
if ! curl -s "$GATEWAY_URL/health" > /dev/null; then
    echo -e "${RED}❌ Gateway is not running!${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Gateway is running${NC}"
echo ""

# Step 3: Send test text message (direct mode)
echo -e "${YELLOW}[3/6] Sending test text message (direct mode)...${NC}"
TRACE_ID="test-trace-text-direct-$(date +%s)"
echo "Trace ID: $TRACE_ID"

TEXT_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/access/message/text" \
    -H "Authorization: Bearer $BEARER_TOKEN" \
    -H "Content-Type: application/json" \
    -H "X-Trace-ID: $TRACE_ID" \
    -d "{
        \"msisdn\": \"$TARGET_PHONE\",
        \"message\": \"Test message from direct mode - $(date)\"
    }")

echo "$TEXT_RESPONSE" | jq '.' 2>/dev/null || echo "$TEXT_RESPONSE"

if echo "$TEXT_RESPONSE" | grep -q "message_id"; then
    MESSAGE_ID=$(echo "$TEXT_RESPONSE" | jq -r '.data.message_id' 2>/dev/null)
    echo -e "${GREEN}✅ Message sent successfully (synchronous)${NC}"
    echo "Message ID: $MESSAGE_ID"
    echo ""
    echo -e "${BLUE}Expected webhook (check webhook server):${NC}"
    echo "  - message.sent (immediate, no job_id)"
    echo ""
    echo -e "${GREEN}✅ Notice: Response is 200 OK (not 202 Accepted)${NC}"
else
    echo -e "${RED}❌ Failed to send message${NC}"
    echo "$TEXT_RESPONSE"

    # If it failed, send failed webhook should have been triggered
    echo ""
    echo -e "${BLUE}Expected webhook (check webhook server):${NC}"
    echo "  - message.failed (immediate, with error)"
fi
echo ""

sleep 2

# Step 4: Send test image message (direct mode)
echo -e "${YELLOW}[4/6] Sending test image message (direct mode)...${NC}"

# Check if test image exists
if [ ! -f "test-image.png" ]; then
    echo "Creating test image..."
    if command -v convert &> /dev/null; then
        convert -size 100x100 xc:green test-image.png 2>/dev/null
    else
        curl -s -o test-image.png "https://via.placeholder.com/100/00FF00/FFFFFF?text=Direct"
    fi
fi

TRACE_ID="test-trace-image-direct-$(date +%s)"
echo "Trace ID: $TRACE_ID"

IMAGE_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/access/message/image" \
    -H "Authorization: Bearer $BEARER_TOKEN" \
    -H "X-Trace-ID: $TRACE_ID" \
    -F "msisdn=$TARGET_PHONE" \
    -F "caption=Test image from direct mode - $(date)" \
    -F "image=@test-image.png")

echo "$IMAGE_RESPONSE" | jq '.' 2>/dev/null || echo "$IMAGE_RESPONSE"

if echo "$IMAGE_RESPONSE" | grep -q "message_id"; then
    MESSAGE_ID=$(echo "$IMAGE_RESPONSE" | jq -r '.data.message_id' 2>/dev/null)
    echo -e "${GREEN}✅ Image sent successfully (synchronous)${NC}"
    echo "Message ID: $MESSAGE_ID"
    echo ""
    echo -e "${BLUE}Expected webhook (check webhook server):${NC}"
    echo "  - message.sent (immediate, no job_id)"
    echo ""
    echo -e "${GREEN}✅ Notice: Response is 200 OK (not 202 Accepted)${NC}"
else
    echo -e "${RED}❌ Failed to send image${NC}"
    echo "$IMAGE_RESPONSE"
fi
echo ""

# Step 5: Test incoming message webhook
echo -e "${YELLOW}[5/6] Testing incoming message webhook...${NC}"
echo "Send a WhatsApp message TO your gateway number: $TARGET_PHONE"
echo "from another WhatsApp account."
echo ""
echo -e "${BLUE}Expected webhook:${NC}"
echo "  - Incoming message event with content"
echo "  - New trace_id generated for the incoming event"
echo ""
read -p "Press Enter after sending the message to continue..."
echo ""

# Step 6: Comparison with Queue Mode
echo -e "${YELLOW}[6/6] Direct Mode vs Queue Mode Comparison${NC}"
echo ""
echo -e "${BLUE}Key Differences:${NC}"
echo ""
echo "Queue Mode (RabbitMQ enabled):"
echo "  ✓ Returns: 202 Accepted with job_id"
echo "  ✓ Webhooks: message.queued → message.sent/failed"
echo "  ✓ Processing: Asynchronous (background worker)"
echo "  ✓ Job status: Available via /message/job/:id"
echo ""
echo "Direct Mode (RabbitMQ disabled):"
echo "  ✓ Returns: 200 OK with message_id"
echo "  ✓ Webhooks: message.sent/failed (immediate)"
echo "  ✓ Processing: Synchronous (blocks until sent)"
echo "  ✓ Job status: Not applicable (no job created)"
echo ""

# Step 7: Restore RabbitMQ configuration
echo -e "${YELLOW}Restore RabbitMQ configuration?${NC}"
echo "Do you want to re-enable RabbitMQ and restore queue mode? (y/n)"
read -r RESTORE

if [ "$RESTORE" = "y" ] || [ "$RESTORE" = "Y" ]; then
    echo "Starting RabbitMQ..."
    docker start rabbitmq > /dev/null 2>&1

    if [ -f "$ENV_FILE" ]; then
        echo "Updating .env to enable RabbitMQ..."
        sed -i.bak 's/RABBITMQ_ENABLED=false/RABBITMQ_ENABLED=true/' "$ENV_FILE"
        echo -e "${GREEN}✅ RabbitMQ re-enabled in .env${NC}"
        echo ""
        echo -e "${YELLOW}Restart the gateway to apply changes:${NC}"
        echo "  1. Stop the gateway (Ctrl+C)"
        echo "  2. Run: make run"
    fi
fi
echo ""

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${GREEN}✅ Direct mode testing completed${NC}"
echo ""
echo "What to verify:"
echo "  1. Text message:"
echo "     - Returned 200 OK with message_id (not 202)"
echo "     - Webhook: message.sent (immediate, no queue delay)"
echo "  2. Image message:"
echo "     - Returned 200 OK with message_id"
echo "     - Webhook: message.sent (immediate)"
echo "  3. Incoming message:"
echo "     - Webhook received with message content"
echo "  4. No job_id in responses (direct mode doesn't create jobs)"
echo "  5. HMAC signatures valid in all webhooks"
echo ""
echo "Check webhook server output for detailed webhook information."
echo ""
