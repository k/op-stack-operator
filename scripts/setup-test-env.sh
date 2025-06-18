#!/bin/bash

# OP Stack Operator - Test Environment Setup Script
# This script helps developers set up their testing environment securely

set -e

echo "🔧 Setting up OP Stack Operator test environment..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create test config directory
mkdir -p test/config

# Check if env.local already exists
if [ -f "test/config/env.local" ]; then
    echo -e "${YELLOW}⚠️  test/config/env.local already exists${NC}"
    read -p "Do you want to overwrite it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Skipping environment setup"
        exit 0
    fi
fi

# Copy example file
cp test/config/env.example test/config/env.local

echo -e "${GREEN}✅ Created test/config/env.local from example${NC}"
echo

# Prompt for API key
echo -e "${BLUE}🔑 Setting up API keys...${NC}"
echo "You'll need to get API keys from RPC providers:"
echo "  • Alchemy: https://dashboard.alchemy.com/"
echo "  • Infura: https://infura.io/"
echo "  • QuickNode: https://www.quicknode.com/"
echo

read -p "Enter your Alchemy Sepolia API key (or press Enter to skip): " alchemy_key

if [ ! -z "$alchemy_key" ]; then
    # Replace the placeholder in env.local
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/YOUR-API-KEY-HERE/${alchemy_key}/g" test/config/env.local
    else
        # Linux
        sed -i "s/YOUR-API-KEY-HERE/${alchemy_key}/g" test/config/env.local
    fi
    echo -e "${GREEN}✅ Updated env.local with your API key${NC}"
fi

echo
echo -e "${BLUE}📝 Next steps:${NC}"
echo "1. Edit test/config/env.local to customize your configuration"
echo "2. Export environment variables:"
echo "   ${YELLOW}export \$(cat test/config/env.local | xargs)${NC}"
echo "3. Run integration tests:"
echo "   ${YELLOW}make test-integration${NC}"
echo
echo -e "${GREEN}🚀 Environment setup complete!${NC}"

# Show security reminder
echo
echo -e "${RED}🔒 SECURITY REMINDER:${NC}"
echo "• Never commit env.local files to version control"
echo "• Keep your API keys secure and rotate them regularly"
echo "• Use separate API keys for development and production" 