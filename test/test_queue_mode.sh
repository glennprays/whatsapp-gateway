#!/bin/bash

# Test Queue Mode (RabbitMQ Enabled)
# This script tests webhook delivery and trace ID propagation in queue mode

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

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Queue Mode Testing (RabbitMQ Enabled)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Check prerequisites
echo -e "${YELLOW}[1/7] Checking prerequisites...${NC}"

if [ ! -f "auth.txt" ]; then
    echo -e "${RED}❌ auth.txt not found!${NC}"
    echo "Please create auth.txt with your JWT token first."
    exit 1
fi

BEARER_TOKEN=$(cat auth.txt)
echo -e "${GREEN}✅ JWT token loaded${NC}"

# Check RabbitMQ
if ! docker ps | grep -q rabbitmq; then
    echo -e "${RED}❌ RabbitMQ is not running!${NC}"
    echo "Start RabbitMQ with: docker start rabbitmq"
    exit 1
fi
echo -e "${GREEN}✅ RabbitMQ is running${NC}"

# Check gateway is running
if ! curl -s "$GATEWAY_URL/health" > /dev/null; then
    echo -e "${RED}❌ Gateway is not running!${NC}"
    echo "Start the gateway first: make run"
    exit 1
fi
echo -e "${GREEN}✅ Gateway is running${NC}"

# Check webhook server
if ! curl -s "$WEBHOOK_SERVER_URL" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Webhook server not running!${NC}"
    echo "Start webhook server in another terminal: cd test && go run webhook_server.go"
    echo ""
    read -p "Press Enter when webhook server is ready..."
fi
echo -e "${GREEN}✅ Webhook server is ready${NC}"
echo ""

# Step 2: Register webhook
echo -e "${YELLOW}[2/7] Registering webhook URL...${NC}"
WEBHOOK_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/access/webhook" \
    -H "Authorization: Bearer $BEARER_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"url\": \"$WEBHOOK_SERVER_URL\",
        \"hmac_secret\": \"$HMAC_SECRET\"
    }")

if echo "$WEBHOOK_RESPONSE" | grep -q "success"; then
    echo -e "${GREEN}✅ Webhook registered successfully${NC}"
else
    echo -e "${RED}❌ Failed to register webhook${NC}"
    echo "$WEBHOOK_RESPONSE"
    exit 1
fi
echo ""

# Step 3: Send test text message
echo -e "${YELLOW}[3/7] Sending test text message (queue mode)...${NC}"
TRACE_ID="test-trace-text-queue-$(date +%s)"
echo "Trace ID: $TRACE_ID"

TEXT_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/access/message/text" \
    -H "Authorization: Bearer $BEARER_TOKEN" \
    -H "Content-Type: application/json" \
    -H "X-Trace-ID: $TRACE_ID" \
    -d "{
        \"msisdn\": \"$TARGET_PHONE\",
        \"message\": \"Test message from queue mode - $(date)\"
    }")

echo "$TEXT_RESPONSE" | jq '.' 2>/dev/null || echo "$TEXT_RESPONSE"

if echo "$TEXT_RESPONSE" | grep -q "job_id"; then
    JOB_ID=$(echo "$TEXT_RESPONSE" | jq -r '.data.job_id' 2>/dev/null)
    echo -e "${GREEN}✅ Message queued successfully${NC}"
    echo "Job ID: $JOB_ID"
    echo ""
    echo -e "${BLUE}Expected webhooks (check webhook server):${NC}"
    echo "  1. message.queued (immediate)"
    echo "  2. message.sent (after worker processes, ~5-10 seconds)"
    echo ""

    # Wait for processing
    echo -e "${YELLOW}Waiting 10 seconds for worker to process...${NC}"
    for i in {10..1}; do
        echo -ne "  $i seconds remaining...\r"
        sleep 1
    done
    echo ""

    # Check job status
    echo -e "${YELLOW}Checking job status...${NC}"
    JOB_STATUS=$(curl -s "$GATEWAY_URL/access/message/job/$JOB_ID" \
        -H "Authorization: Bearer $BEARER_TOKEN")
    echo "$JOB_STATUS" | jq '.' 2>/dev/null || echo "$JOB_STATUS"

    if echo "$JOB_STATUS" | grep -q "completed"; then
        echo -e "${GREEN}✅ Job completed successfully${NC}"
    else
        echo -e "${YELLOW}⚠️  Job status: $(echo "$JOB_STATUS" | jq -r '.data.status' 2>/dev/null)${NC}"
    fi
else
    echo -e "${RED}❌ Failed to queue message${NC}"
    echo "$TEXT_RESPONSE"
fi
echo ""

# Step 4: Create test image
echo -e "${YELLOW}[4/7] Creating test image...${NC}"
if command -v convert &> /dev/null; then
    convert -size 100x100 xc:blue test-image.png 2>/dev/null
    echo -e "${GREEN}✅ Test image created${NC}"
elif [ ! -f "test-image.png" ]; then
    echo -e "${YELLOW}⚠️  ImageMagick not found, downloading test image...${NC}"
    curl -s -o test-image.png "https://via.placeholder.com/100/0000FF/FFFFFF?text=Test"
    echo -e "${GREEN}✅ Test image downloaded${NC}"
else
    echo -e "${GREEN}✅ Test image already exists${NC}"
fi
echo ""

# Step 5: Send test image message
echo -e "${YELLOW}[5/7] Sending test image message (queue mode)...${NC}"
TRACE_ID="test-trace-image-queue-$(date +%s)"
echo "Trace ID: $TRACE_ID"

IMAGE_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/access/message/image" \
    -H "Authorization: Bearer $BEARER_TOKEN" \
    -H "X-Trace-ID: $TRACE_ID" \
    -F "msisdn=$TARGET_PHONE" \
    -F "caption=Test image from queue mode - $(date)" \
    -F "image=@test-image.png")

echo "$IMAGE_RESPONSE" | jq '.' 2>/dev/null || echo "$IMAGE_RESPONSE"

if echo "$IMAGE_RESPONSE" | grep -q "job_id"; then
    JOB_ID=$(echo "$IMAGE_RESPONSE" | jq -r '.data.job_id' 2>/dev/null)
    echo -e "${GREEN}✅ Image queued successfully${NC}"
    echo "Job ID: $JOB_ID"
    echo ""
    echo -e "${BLUE}Expected webhooks (check webhook server):${NC}"
    echo "  1. message.queued (immediate)"
    echo "  2. message.sent (after worker processes, ~5-10 seconds)"
    echo ""

    # Wait for processing
    echo -e "${YELLOW}Waiting 10 seconds for worker to process...${NC}"
    for i in {10..1}; do
        echo -ne "  $i seconds remaining...\r"
        sleep 1
    done
    echo ""

    # Check job status
    echo -e "${YELLOW}Checking job status...${NC}"
    JOB_STATUS=$(curl -s "$GATEWAY_URL/access/message/job/$JOB_ID" \
        -H "Authorization: Bearer $BEARER_TOKEN")
    echo "$JOB_STATUS" | jq '.' 2>/dev/null || echo "$JOB_STATUS"

    if echo "$JOB_STATUS" | grep -q "completed"; then
        echo -e "${GREEN}✅ Job completed successfully${NC}"
    else
        echo -e "${YELLOW}⚠️  Job status: $(echo "$JOB_STATUS" | jq -r '.data.status' 2>/dev/null)${NC}"
    fi
else
    echo -e "${RED}❌ Failed to queue image${NC}"
    echo "$IMAGE_RESPONSE"
fi
echo ""

# Step 6: Check trace ID in logs
echo -e "${YELLOW}[6/7] Verifying trace ID propagation in logs...${NC}"
echo "Checking for first trace ID: $TRACE_ID"

if [ -f "app.log" ]; then
    TRACE_COUNT=$(grep -c "$TRACE_ID" app.log 2>/dev/null || echo "0")
    if [ "$TRACE_COUNT" -gt 0 ]; then
        echo -e "${GREEN}✅ Found $TRACE_COUNT log entries with trace ID${NC}"
        echo ""
        echo "Sample logs:"
        grep "$TRACE_ID" app.log | head -5 | while read line; do
            echo "  $line"
        done
    else
        echo -e "${YELLOW}⚠️  No logs found with trace ID (logs might be in stdout)${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  app.log not found (logs might be in stdout)${NC}"
fi
echo ""

# Step 7: Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${GREEN}✅ Queue mode testing completed${NC}"
echo ""
echo "What to verify:"
echo "  1. Webhook server received webhooks in correct sequence:"
echo "     - message.queued (immediate)"
echo "     - message.sent (after ~5-10 seconds)"
echo "  2. HMAC signatures were valid (✅ in webhook server)"
echo "  3. Trace IDs appeared in logs throughout the flow"
echo "  4. Job status endpoints returned correct status"
echo "  5. Messages delivered to WhatsApp"
echo ""
echo "Check webhook server output for detailed webhook information."
echo ""
echo -e "${YELLOW}Next: Test direct mode with RabbitMQ disabled${NC}"
echo "Run: ./test_direct_mode.sh"
echo ""
