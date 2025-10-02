#!/bin/bash
# Test script to demonstrate hooks functionality

echo "🧪 Claude Code Hooks Test Suite"
echo "================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test 1: Import validation (should pass)
echo "Test 1: Valid import"
echo '{"tool_name":"Edit","tool_input":{"file_path":"api-gateway/internal/test.go","new_string":"import \"fmt\""}}' | python3 .claude-hooks/scripts/pre-check-imports.py
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Pass: Valid imports allowed${NC}"
else
    echo -e "${RED}❌ Fail: Valid import blocked${NC}"
fi
echo ""

# Test 2: Cross-service import (should fail)
echo "Test 2: Cross-service import"
echo '{"tool_name":"Edit","tool_input":{"file_path":"api-gateway/internal/test.go","new_string":"import \"tall-affiliate/content-generation-service/internal/worker\""}}' | python3 .claude-hooks/scripts/pre-check-imports.py 2>/dev/null
if [ $? -eq 2 ]; then
    echo -e "${GREEN}✅ Pass: Cross-service import blocked${NC}"
else
    echo -e "${RED}❌ Fail: Cross-service import not detected${NC}"
fi
echo ""

# Test 3: Event validation with cached constants
echo "Test 3: Known event constant"
echo '{"tool_name":"Edit","tool_input":{"file_path":"api-gateway/internal/worker/handler.go","new_string":"event := EventTypeProductCreated"}}' | python3 .claude-hooks/scripts/pre-validate-events.py
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Pass: Known event allowed${NC}"
else
    echo -e "${RED}❌ Fail: Known event blocked${NC}"
fi
echo ""

# Test 4: Unknown event constant
echo "Test 4: Unknown event constant"
echo '{"tool_name":"Edit","tool_input":{"file_path":"api-gateway/internal/worker/handler.go","new_string":"event := EventTypeUnknownEvent"}}' | python3 .claude-hooks/scripts/pre-validate-events.py 2>/dev/null
if [ $? -eq 2 ]; then
    echo -e "${GREEN}✅ Pass: Unknown event blocked${NC}"
else
    echo -e "${RED}❌ Fail: Unknown event not detected${NC}"
fi
echo ""

# Test 5: Go formatting (visual test)
echo "Test 5: Go formatting"
echo '{"tool_response":{"filePath":"api-gateway/test.go"}}' | .claude-hooks/scripts/post-format-go.sh
echo -e "${YELLOW}ℹ️  Note: Format hook runs silently (check output above)${NC}"
echo ""

# Performance summary
echo "================================"
echo "📊 Performance Check:"
echo ""

# Check cache
if [ -f ".claude-hooks/cache/event_constants.json" ]; then
    EVENT_COUNT=$(jq 'length' .claude-hooks/cache/event_constants.json)
    echo -e "${GREEN}✅ Event cache active: $EVENT_COUNT constants cached${NC}"
else
    echo -e "${RED}❌ Event cache missing${NC}"
fi

# Check tools
command -v gofmt &> /dev/null && echo -e "${GREEN}✅ gofmt available${NC}" || echo -e "${YELLOW}⚠️  gofmt missing${NC}"
command -v goimports &> /dev/null && echo -e "${GREEN}✅ goimports available${NC}" || echo -e "${YELLOW}⚠️  goimports missing${NC}"
command -v golangci-lint &> /dev/null && echo -e "${GREEN}✅ golangci-lint available${NC}" || echo -e "${YELLOW}⚠️  golangci-lint missing${NC}"

echo ""
echo "================================"
echo "✨ Hook test complete!"