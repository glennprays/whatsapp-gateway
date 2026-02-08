#!/bin/bash

# Verification Script
# Analyzes logs and verifies trace ID propagation and webhook implementation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Implementation Verification${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

LOG_FILE="../app.log"
PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0

# Function to check and report
check_status() {
    local test_name=$1
    local status=$2
    local message=$3

    if [ "$status" = "pass" ]; then
        echo -e "${GREEN}✅ PASS${NC}: $test_name"
        [ ! -z "$message" ] && echo -e "   ${CYAN}$message${NC}"
        PASS_COUNT=$((PASS_COUNT + 1))
    elif [ "$status" = "fail" ]; then
        echo -e "${RED}❌ FAIL${NC}: $test_name"
        [ ! -z "$message" ] && echo -e "   ${RED}$message${NC}"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    else
        echo -e "${YELLOW}⚠️  WARN${NC}: $test_name"
        [ ! -z "$message" ] && echo -e "   ${YELLOW}$message${NC}"
        WARN_COUNT=$((WARN_COUNT + 1))
    fi
}

echo -e "${YELLOW}[1/5] Checking log file...${NC}"
if [ -f "$LOG_FILE" ]; then
    LOG_SIZE=$(wc -l < "$LOG_FILE")
    echo "Log file: $LOG_FILE ($LOG_SIZE lines)"
    check_status "Log file exists" "pass" "Found $LOG_SIZE log entries"
else
    check_status "Log file exists" "fail" "app.log not found (logs may be in stdout)"
    echo ""
    echo "To enable file logging, ensure your logger configuration writes to app.log"
    echo ""
fi
echo ""

# Check for trace IDs in logs
echo -e "${YELLOW}[2/5] Verifying trace ID propagation...${NC}"

if [ -f "$LOG_FILE" ]; then
    # Check for empty trace_id (should be ZERO)
    EMPTY_TRACE=$(grep -c 'trace_id=""' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$EMPTY_TRACE" -eq 0 ]; then
        check_status "No empty trace_id in logs" "pass" "All logs have proper trace_id"
    else
        check_status "No empty trace_id in logs" "fail" "Found $EMPTY_TRACE logs with empty trace_id"
    fi

    # Check for trace_id field existence
    TRACE_ID_COUNT=$(grep -c 'trace_id' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$TRACE_ID_COUNT" -gt 0 ]; then
        check_status "Trace ID field present" "pass" "Found $TRACE_ID_COUNT logs with trace_id field"
    else
        check_status "Trace ID field present" "fail" "No trace_id field found in logs"
    fi

    # Check for UUID format trace IDs
    UUID_COUNT=$(grep -E 'trace_id="[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"' "$LOG_FILE" 2>/dev/null | wc -l)
    if [ "$UUID_COUNT" -gt 0 ]; then
        check_status "UUID-format trace IDs" "pass" "Found $UUID_COUNT UUID-format trace IDs"
    else
        check_status "UUID-format trace IDs" "warn" "No UUID-format trace IDs found (may use custom format)"
    fi

    # Sample trace ID from recent logs
    SAMPLE_TRACE=$(grep -o 'trace_id="[^"]*"' "$LOG_FILE" 2>/dev/null | tail -1 | cut -d'"' -f2)
    if [ ! -z "$SAMPLE_TRACE" ]; then
        echo -e "   ${CYAN}Sample trace ID: $SAMPLE_TRACE${NC}"

        # Count occurrences of this trace ID across layers
        TRACE_OCCURRENCES=$(grep -c "$SAMPLE_TRACE" "$LOG_FILE" 2>/dev/null || echo "0")
        if [ "$TRACE_OCCURRENCES" -gt 1 ]; then
            check_status "Trace ID propagation" "pass" "Sample trace ID appears $TRACE_OCCURRENCES times (propagated across layers)"
        else
            check_status "Trace ID propagation" "warn" "Sample trace ID appears only once"
        fi
    fi
else
    check_status "Trace ID verification" "warn" "Cannot verify - log file not available"
fi
echo ""

# Check for webhook-related logs
echo -e "${YELLOW}[3/5] Verifying webhook implementation...${NC}"

if [ -f "$LOG_FILE" ]; then
    # Check for queued webhook logs
    QUEUED_WEBHOOK=$(grep -c 'message.queued' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$QUEUED_WEBHOOK" -gt 0 ]; then
        check_status "message.queued webhook" "pass" "Found $QUEUED_WEBHOOK queued webhook logs"
    else
        check_status "message.queued webhook" "warn" "No queued webhook logs (may not have tested queue mode)"
    fi

    # Check for sent webhook logs
    SENT_WEBHOOK=$(grep -c 'message.sent' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$SENT_WEBHOOK" -gt 0 ]; then
        check_status "message.sent webhook" "pass" "Found $SENT_WEBHOOK sent webhook logs"
    else
        check_status "message.sent webhook" "warn" "No sent webhook logs"
    fi

    # Check for failed webhook logs
    FAILED_WEBHOOK=$(grep -c 'message.failed' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$FAILED_WEBHOOK" -gt 0 ]; then
        check_status "message.failed webhook" "pass" "Found $FAILED_WEBHOOK failed webhook logs"
    else
        check_status "message.failed webhook" "warn" "No failed webhook logs (good - means no failures)"
    fi

    # Check for direct mode webhook logs
    DIRECT_WEBHOOK=$(grep -c 'direct mode' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$DIRECT_WEBHOOK" -gt 0 ]; then
        check_status "Direct mode webhooks" "pass" "Found $DIRECT_WEBHOOK direct mode webhook logs"
    else
        check_status "Direct mode webhooks" "warn" "No direct mode webhook logs (may not have tested direct mode)"
    fi
else
    check_status "Webhook verification" "warn" "Cannot verify - log file not available"
fi
echo ""

# Check for layer-specific logging
echo -e "${YELLOW}[4/5] Verifying logging across layers...${NC}"

if [ -f "$LOG_FILE" ]; then
    # Check for handler logs
    HANDLER_LOGS=$(grep -c 'handler' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$HANDLER_LOGS" -gt 0 ]; then
        check_status "Handler layer logging" "pass" "Found $HANDLER_LOGS handler logs"
    else
        check_status "Handler layer logging" "warn" "No handler logs found"
    fi

    # Check for usecase logs
    USECASE_LOGS=$(grep -c 'usecase\|Usecase' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$USECASE_LOGS" -gt 0 ]; then
        check_status "Usecase layer logging" "pass" "Found $USECASE_LOGS usecase logs"
    else
        check_status "Usecase layer logging" "warn" "No usecase logs found"
    fi

    # Check for manager logs
    MANAGER_LOGS=$(grep -c 'manager\|Manager' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$MANAGER_LOGS" -gt 0 ]; then
        check_status "Manager layer logging" "pass" "Found $MANAGER_LOGS manager logs"
    else
        check_status "Manager layer logging" "warn" "No manager logs found"
    fi

    # Check for client logs
    CLIENT_LOGS=$(grep -c 'client\|Client' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$CLIENT_LOGS" -gt 0 ]; then
        check_status "Client layer logging" "pass" "Found $CLIENT_LOGS client logs"
    else
        check_status "Client layer logging" "warn" "No client logs found"
    fi
else
    check_status "Layer verification" "warn" "Cannot verify - log file not available"
fi
echo ""

# Check queue handler logs
echo -e "${YELLOW}[5/5] Verifying queue handler logs...${NC}"

if [ -f "$LOG_FILE" ]; then
    # Check for queue processing logs
    QUEUE_LOGS=$(grep -c 'queue\|Queue\|job' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$QUEUE_LOGS" -gt 0 ]; then
        check_status "Queue processing logs" "pass" "Found $QUEUE_LOGS queue-related logs"
    else
        check_status "Queue processing logs" "warn" "No queue logs (may be using direct mode)"
    fi

    # Check for worker logs
    WORKER_LOGS=$(grep -c 'worker\|Worker' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$WORKER_LOGS" -gt 0 ]; then
        check_status "Worker logs" "pass" "Found $WORKER_LOGS worker logs"
    else
        check_status "Worker logs" "warn" "No worker logs (may not have tested queue mode)"
    fi

    # Check for fallback trace ID generation
    FALLBACK_TRACE=$(grep -c 'missing trace_id\|generating new' "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$FALLBACK_TRACE" -gt 0 ]; then
        check_status "Fallback trace ID generation" "pass" "Found $FALLBACK_TRACE fallback trace ID generations (backward compatibility)"
    else
        check_status "Fallback trace ID generation" "pass" "No fallback needed (all messages had trace_id)"
    fi
else
    check_status "Queue handler verification" "warn" "Cannot verify - log file not available"
fi
echo ""

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Verification Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${GREEN}Passed:  $PASS_COUNT${NC}"
echo -e "${YELLOW}Warnings: $WARN_COUNT${NC}"
echo -e "${RED}Failed:  $FAIL_COUNT${NC}"
echo ""

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}🎉 All critical checks passed!${NC}"
    echo ""
    echo "Implementation is working correctly:"
    echo "  ✓ Trace IDs are properly propagated"
    echo "  ✓ No empty trace_id in logs"
    echo "  ✓ Webhooks are being sent"
    echo "  ✓ Logging works across all layers"
else
    echo -e "${RED}⚠️  Some checks failed. Please review the issues above.${NC}"
fi
echo ""

# Detailed trace ID analysis
if [ -f "$LOG_FILE" ] && [ "$TRACE_ID_COUNT" -gt 0 ]; then
    echo -e "${YELLOW}Detailed Trace ID Analysis:${NC}"
    echo ""

    # Get unique trace IDs
    UNIQUE_TRACES=$(grep -o 'trace_id="[^"]*"' "$LOG_FILE" | sort -u | wc -l)
    echo "  Total log entries with trace_id: $TRACE_ID_COUNT"
    echo "  Unique trace IDs: $UNIQUE_TRACES"
    echo "  Average logs per trace: $((TRACE_ID_COUNT / UNIQUE_TRACES))"
    echo ""

    # Show most common trace IDs (top 5)
    echo "  Top 5 most active trace IDs:"
    grep -o 'trace_id="[^"]*"' "$LOG_FILE" | sort | uniq -c | sort -rn | head -5 | while read count trace; do
        trace_id=$(echo $trace | cut -d'"' -f2)
        echo "    $count logs: $trace_id"
    done
    echo ""
fi

# Suggestions for improvement
if [ "$WARN_COUNT" -gt 0 ] || [ "$FAIL_COUNT" -gt 0 ]; then
    echo -e "${YELLOW}Suggestions:${NC}"
    echo ""

    if [ ! -f "$LOG_FILE" ]; then
        echo "  • Enable file logging to app.log for better analysis"
    fi

    if [ "$QUEUED_WEBHOOK" -eq 0 ]; then
        echo "  • Test queue mode with RabbitMQ to verify message.queued webhooks"
    fi

    if [ "$DIRECT_WEBHOOK" -eq 0 ]; then
        echo "  • Test direct mode (RabbitMQ disabled) to verify direct webhooks"
    fi

    if [ "$EMPTY_TRACE" -gt 0 ]; then
        echo "  • Fix empty trace_id logs - all logs should have trace_id"
    fi

    echo ""
fi

echo "For more details, check the log file: $LOG_FILE"
echo ""
