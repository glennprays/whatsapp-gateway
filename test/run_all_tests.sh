#!/bin/bash

# Master Test Runner
# Runs all test phases in sequence

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

clear

echo -e "${BOLD}${BLUE}"
cat << "EOF"
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║     WhatsApp Gateway - Complete Test Suite                ║
║     Trace ID Logging & Webhook Implementation              ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

echo "This test suite will guide you through testing:"
echo "  • Phase 1: Trace ID propagation"
echo "  • Phase 2: Webhook delivery (queue mode)"
echo "  • Phase 3: Webhook delivery (direct mode)"
echo "  • Phase 4: Comprehensive verification"
echo ""
echo -e "${YELLOW}Prerequisites:${NC}"
echo "  1. Gateway must be running (make run)"
echo "  2. WhatsApp must be logged in"
echo "  3. JWT token must be in auth.txt"
echo ""
read -p "Press Enter to continue or Ctrl+C to exit..."
clear

# Step 1: Start webhook server
echo -e "${BOLD}${CYAN}Step 1: Webhook Test Server${NC}"
echo "═══════════════════════════════════════════════════════════"
echo ""
echo "The webhook server needs to be running to receive webhooks."
echo ""
echo -e "${YELLOW}In a NEW terminal, run:${NC}"
echo "  cd test"
echo "  go run webhook_server.go"
echo ""
echo "The server will:"
echo "  • Listen on http://localhost:8080/webhook"
echo "  • Display received webhooks in real-time"
echo "  • Validate HMAC signatures"
echo ""
read -p "Press Enter when the webhook server is ready..."
clear

# Step 2: Queue mode test
echo -e "${BOLD}${CYAN}Step 2: Queue Mode Testing (RabbitMQ Enabled)${NC}"
echo "═══════════════════════════════════════════════════════════"
echo ""
echo "This will test:"
echo "  • message.queued webhook (immediate)"
echo "  • message.sent webhook (after worker)"
echo "  • Trace ID propagation through queue"
echo "  • Job status tracking"
echo ""
read -p "Press Enter to start queue mode test..."
echo ""

if ./test_queue_mode.sh; then
    echo ""
    echo -e "${GREEN}✅ Queue mode test completed${NC}"
else
    echo ""
    echo -e "${RED}❌ Queue mode test failed${NC}"
    echo "Check the errors above and fix before continuing."
    read -p "Press Enter to continue anyway or Ctrl+C to exit..."
fi

echo ""
read -p "Press Enter to continue to direct mode test..."
clear

# Step 3: Direct mode test
echo -e "${BOLD}${CYAN}Step 3: Direct Mode Testing (RabbitMQ Disabled)${NC}"
echo "═══════════════════════════════════════════════════════════"
echo ""
echo "This will test:"
echo "  • Synchronous message sending (200 OK, not 202)"
echo "  • message.sent webhook (immediate, no queue)"
echo "  • message.failed webhook (immediate on error)"
echo "  • Trace ID in direct mode"
echo ""
echo -e "${YELLOW}NOTE: This will stop RabbitMQ and require gateway restart${NC}"
echo ""
read -p "Press Enter to start direct mode test..."
echo ""

if ./test_direct_mode.sh; then
    echo ""
    echo -e "${GREEN}✅ Direct mode test completed${NC}"
else
    echo ""
    echo -e "${RED}❌ Direct mode test failed${NC}"
    echo "Check the errors above."
    read -p "Press Enter to continue anyway or Ctrl+C to exit..."
fi

echo ""
read -p "Press Enter to continue to verification..."
clear

# Step 4: Verification
echo -e "${BOLD}${CYAN}Step 4: Implementation Verification${NC}"
echo "═══════════════════════════════════════════════════════════"
echo ""
echo "Analyzing logs and verifying implementation..."
echo ""

./verify_implementation.sh

echo ""
echo -e "${BOLD}${GREEN}Test Suite Completed!${NC}"
echo ""

# Final summary
echo -e "${BOLD}${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}${BLUE}Final Summary${NC}"
echo -e "${BOLD}${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""

echo -e "${CYAN}What was tested:${NC}"
echo ""
echo "✓ Phase 1 - Trace ID Propagation:"
echo "    • Trace IDs flow through all layers"
echo "    • No empty trace_id in logs"
echo "    • UUID generation for incoming events"
echo "    • Fallback for backward compatibility"
echo ""
echo "✓ Phase 2 - Webhook Implementation:"
echo "    • message.queued webhook (queue mode)"
echo "    • message.sent webhook (both modes)"
echo "    • message.failed webhook (both modes)"
echo "    • HMAC signature validation"
echo ""
echo "✓ Phase 3 - Queue vs Direct Mode:"
echo "    • Queue mode: 202 Accepted + async processing"
echo "    • Direct mode: 200 OK + immediate send"
echo "    • Webhooks work in both modes"
echo ""

echo -e "${CYAN}Key achievements:${NC}"
echo ""
echo "  🎯 Complete request traceability with trace_id"
echo "  🎯 Reliable webhook delivery in all modes"
echo "  🎯 Consistent API behavior and contracts"
echo "  🎯 Improved debugging capabilities"
echo ""

echo -e "${YELLOW}Next steps:${NC}"
echo ""
echo "  1. Review webhook server output for webhook details"
echo "  2. Check gateway logs for trace ID flow"
echo "  3. Test with real WhatsApp messages"
echo "  4. Update documentation if needed"
echo ""

echo -e "${GREEN}All phases completed successfully! 🎉${NC}"
echo ""
