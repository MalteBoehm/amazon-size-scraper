#!/bin/bash
# Post-hook: Auto-format Go files after modification
# Performance target: <200ms

set -euo pipefail

# Start timing
START_TIME=$(date +%s%N)

# Parse JSON to get file path
FILE_PATH=""
if command -v jq &> /dev/null; then
    # Try to get file path from tool response or input
    FILE_PATH=$(jq -r '.tool_response.filePath // .tool_input.file_path // empty' 2>/dev/null || echo "")
else
    # Fallback: basic parsing if jq not available
    FILE_PATH=$(grep -o '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4 | head -1)
fi

# Exit if no file path or not a Go file
if [ -z "$FILE_PATH" ] || [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    echo '{"suppressOutput": true}'
    exit 0
fi

# Skip test files from formatting (optional - remove if you want to format tests too)
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    echo '{"suppressOutput": true}'
    exit 0
fi

# Extract service directory
SERVICE_DIR=""
for service in "api-gateway" "content-generation-service" "product-lifecycle-service" "browse-node-service"; do
    if [[ "$FILE_PATH" == *"$service"* ]]; then
        SERVICE_DIR="$service"
        break
    fi
done

# Exit if not in a service directory
if [ -z "$SERVICE_DIR" ]; then
    echo '{"suppressOutput": true}'
    exit 0
fi

# Get relative path within service
REL_PATH=${FILE_PATH#*$SERVICE_DIR/}

# Change to service directory
cd "$SERVICE_DIR" 2>/dev/null || {
    echo '{"suppressOutput": true}'
    exit 0
}

# Format the file (only if tools are available)
FORMATTED=false
ERROR_MSG=""

# Try gofmt
if command -v gofmt &> /dev/null; then
    if gofmt -w "$REL_PATH" 2>/tmp/gofmt_error.log; then
        FORMATTED=true
    else
        ERROR_MSG=$(cat /tmp/gofmt_error.log 2>/dev/null || echo "gofmt failed")
    fi
fi

# Try goimports (if available)
if command -v goimports &> /dev/null && [ "$FORMATTED" = true ]; then
    if ! goimports -w "$REL_PATH" 2>/tmp/goimports_error.log; then
        ERROR_MSG=$(cat /tmp/goimports_error.log 2>/dev/null || echo "goimports failed")
    fi
fi

# Calculate elapsed time
END_TIME=$(date +%s%N)
ELAPSED_MS=$(( (END_TIME - START_TIME) / 1000000 ))

# Return result
if [ -n "$ERROR_MSG" ]; then
    # Non-blocking error - show to user but continue
    echo "{\"error\": \"Format warning: $ERROR_MSG\"}" >&2
    exit 1
else
    # Success - suppress output to avoid noise
    if [ $ELAPSED_MS -gt 200 ]; then
        echo "{\"warning\": \"Formatting took ${ELAPSED_MS}ms (target: <200ms)\"}"
    else
        echo '{"suppressOutput": true}'
    fi
    exit 0
fi