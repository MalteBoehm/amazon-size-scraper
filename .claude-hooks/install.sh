#!/bin/bash
# Claude Code Hooks Installation Script
# Installs and configures hooks for the Tall Affiliate project

set -euo pipefail

echo "🚀 Installing Claude Code Hooks for Tall Affiliate..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Check prerequisites
echo "📋 Checking prerequisites..."

# Check for Python 3
if ! command -v python3 &> /dev/null; then
    echo -e "${RED}❌ Python 3 is required but not installed${NC}"
    exit 1
fi

# Check for jq (optional but recommended)
if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}⚠️  jq is not installed (recommended for better performance)${NC}"
    echo "   Install with: brew install jq (macOS) or apt-get install jq (Linux)"
fi

# Check for Go tools
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}⚠️  Go is not installed - some hooks will be limited${NC}"
fi

if ! command -v gofmt &> /dev/null; then
    echo -e "${YELLOW}⚠️  gofmt not found - auto-formatting will be disabled${NC}"
fi

if ! command -v goimports &> /dev/null; then
    echo -e "${YELLOW}⚠️  goimports not found - import sorting will be disabled${NC}"
    echo "   Install with: go install golang.org/x/tools/cmd/goimports@latest"
fi

if ! command -v golangci-lint &> /dev/null; then
    echo -e "${YELLOW}⚠️  golangci-lint not found - linting will be disabled${NC}"
    echo "   Install from: https://golangci-lint.run/usage/install/"
fi

# Step 2: Create directories
echo "📁 Creating directories..."
mkdir -p .claude-hooks/{scripts,cache}

# Step 3: Make scripts executable
echo "🔧 Setting permissions..."
chmod +x .claude-hooks/scripts/*.py 2>/dev/null || true
chmod +x .claude-hooks/scripts/*.sh 2>/dev/null || true

# Step 4: Initialize cache
echo "💾 Initializing event constants cache..."
python3 << 'EOF'
import json
import re
from pathlib import Path

def collect_event_constants():
    """Collect all event constants from the codebase"""
    events = {}
    
    # Check tall-affiliate-common for event definitions
    events_path = Path("tall-affiliate-common/pkg/events")
    if events_path.exists():
        for go_file in events_path.glob("*.go"):
            content = go_file.read_text()
            # Extract EventType constants
            pattern = r'EventType(\w+)\s*=\s*"([^"]+)"'
            for match in re.finditer(pattern, content):
                events[match.group(1)] = match.group(2)
    
    # Also check constants.go
    constants_path = Path("tall-affiliate-common/pkg/constants/constants.go")
    if constants_path.exists():
        content = constants_path.read_text()
        pattern = r'EventType(\w+)\s*=\s*"([^"]+)"'
        for match in re.finditer(pattern, content):
            events[match.group(1)] = match.group(2)
    
    return events

try:
    events = collect_event_constants()
    cache_path = Path(".claude-hooks/cache/event_constants.json")
    cache_path.parent.mkdir(parents=True, exist_ok=True)
    
    with open(cache_path, 'w') as f:
        json.dump(events, f, indent=2)
    
    print(f"✅ Cached {len(events)} event constants")
except Exception as e:
    print(f"⚠️  Could not initialize cache: {e}")
EOF

# Step 5: Check current Claude settings
echo ""
echo "📝 Checking Claude settings..."

SETTINGS_FILE=".claude/settings.local.json"
if [ -f "$SETTINGS_FILE" ]; then
    echo -e "${YELLOW}⚠️  Found existing $SETTINGS_FILE${NC}"
    echo "   Hooks configuration should be added manually to preserve existing settings"
else
    echo "   No existing settings found"
fi

# Step 6: Generate hook configuration
echo ""
echo "🔧 Generating hooks configuration..."

cat << 'EOF' > .claude-hooks/hooks-config.json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "python3 .claude-hooks/scripts/pre-check-imports.py"
          }
        ]
      },
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "python3 .claude-hooks/scripts/pre-validate-events.py"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": ".claude-hooks/scripts/post-format-go.sh"
          }
        ]
      },
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "python3 .claude-hooks/scripts/post-test-affected.py"
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "python3 .claude-hooks/scripts/stop-quality-report.py"
          }
        ]
      }
    ]
  }
}
EOF

# Step 7: Test hooks
echo ""
echo "🧪 Testing hooks..."

# Test import validation hook
if python3 .claude-hooks/scripts/pre-check-imports.py << 'EOF' 2>/dev/null
{
  "tool_name": "Edit",
  "tool_input": {
    "file_path": "api-gateway/test.go",
    "new_string": "import \"testing\""
  }
}
EOF
then
    echo "✅ Import validation hook: OK"
else
    echo -e "${YELLOW}⚠️  Import validation hook test failed${NC}"
fi

# Step 8: Instructions
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ Claude Code Hooks installed successfully!${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📋 Next steps:"
echo ""
echo "1. Add the hooks configuration to your Claude settings:"
echo "   - Copy the configuration from: .claude-hooks/hooks-config.json"
echo "   - Add it to: $SETTINGS_FILE"
echo ""
echo "2. Or use the /hooks command in Claude Code:"
echo "   - Type: /hooks"
echo "   - Add each hook type manually"
echo ""
echo "3. Restart Claude Code for the hooks to take effect"
echo ""
echo "📚 Hook Summary:"
echo "   • Pre-checks: Import validation, Event consistency"
echo "   • Post-actions: Auto-format Go, Run affected tests"
echo "   • Stop: Quality report with lint & coverage"
echo ""
echo "💡 Tips:"
echo "   • Hooks run automatically - no manual intervention needed"
echo "   • Exit code 2 blocks actions with Claude feedback"
echo "   • Check .claude-hooks/cache/ for cached data"
echo "   • Logs appear in transcript mode (Ctrl+R)"
echo ""
echo "🔍 Troubleshooting:"
echo "   • Ensure Python 3 is in PATH"
echo "   • Install missing Go tools for full functionality"
echo "   • Check file permissions if hooks don't run"
echo "   • Use --debug flag in Claude for detailed logs"
echo ""