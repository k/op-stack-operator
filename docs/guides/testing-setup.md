# Testing Setup - Secure API Key Management

This guide explains how to set up your development environment for testing the OP Stack Operator while keeping API keys secure.

## 🔒 Security First

**Never commit API keys to version control!** This repository follows security best practices for handling sensitive credentials.

## Quick Start

### 1. Automated Setup (Recommended)

Run the setup script to configure your test environment:

```bash
make setup-test-env
```

This will:

- Create `test/config/env.local` from the example template
- Prompt you to enter your API keys securely
- Set up the proper file permissions

### 2. Manual Setup

If you prefer manual setup:

```bash
# Copy the example file
cp test/config/env.example test/config/env.local

# Edit with your API keys
vi test/config/env.local
```

### 3. Export Environment Variables

Before running tests, export the environment variables:

```bash
# Load environment variables
export $(cat test/config/env.local | xargs)

# Or source them (if using bash)
set -a; source test/config/env.local; set +a
```

## 🧪 Running Tests

### Unit Tests (No API Keys Required)

```bash
make test-unit
```

### Integration Tests (Requires API Keys)

```bash
# With environment file
make test-integration-with-env

# Or with exported variables
make test-integration
```

### End-to-End Tests

```bash
make test-e2e
```

## 🌐 RPC Provider Options

### Alchemy (Recommended for Testing)

```bash
TEST_L1_RPC_URL="https://eth-sepolia.g.alchemy.com/v2/YOUR-API-KEY"
```

- **Free tier**: 300M compute units/month
- **Sepolia support**: ✅ Full support
- **Rate limits**: Generous for testing
- **Sign up**: https://dashboard.alchemy.com/

### Infura

```bash
TEST_L1_RPC_URL="https://sepolia.infura.io/v3/YOUR-PROJECT-ID"
```

- **Free tier**: 100K requests/day
- **Sepolia support**: ✅ Full support
- **Rate limits**: 10 requests/second
- **Sign up**: https://infura.io/

### QuickNode

```bash
TEST_L1_RPC_URL="https://YOUR-ENDPOINT.sepolia.quiknode.pro/YOUR-TOKEN/"
```

- **Free tier**: 5M API credits/month
- **Sepolia support**: ✅ Full support
- **Rate limits**: Configurable
- **Sign up**: https://www.quicknode.com/

### Public Endpoints (No API Key Required)

```bash
# Ankr (rate limited)
TEST_L1_RPC_URL="https://rpc.ankr.com/eth_sepolia"

# BlastAPI (rate limited)
TEST_L1_RPC_URL="https://eth-sepolia.public.blastapi.io"
```

⚠️ **Warning**: Public endpoints have strict rate limits and may not be suitable for extensive testing.

## 🏗️ CI/CD Setup

### GitHub Actions

1. **Set Repository Secrets**:

   - Go to your repository Settings → Secrets and variables → Actions
   - Add these secrets:
     - `TEST_L1_RPC_URL`: Your Alchemy/Infura endpoint
     - `TEST_L1_BEACON_URL`: Your beacon chain endpoint (optional)

2. **Workflow Configuration**:
   The CI workflow automatically:
   - Skips integration tests if no API key is provided
   - Runs security scans to prevent accidental key commits
   - Uses secrets securely without exposing them in logs

### Local Development

```bash
# Check your current configuration
env | grep TEST_

# Test API key connectivity
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
  $TEST_L1_RPC_URL
```

## 🔧 Troubleshooting

### Common Issues

**1. Tests Skip with "No API Key" Message**

```
Skip: Skipping integration tests - no TEST_L1_RPC_URL environment variable set
```

**Solution**: Make sure your environment variables are exported:

```bash
export $(cat test/config/env.local | xargs)
echo $TEST_L1_RPC_URL  # Should show your URL
```

**2. Rate Limit Errors**

```
Error: too many requests
```

**Solutions**:

- Use a paid API key with higher limits
- Add delays between test runs
- Use multiple API keys for different test suites

**3. Network Timeout Errors**

```
Error: context deadline exceeded
```

**Solutions**:

- Check your internet connection
- Verify the RPC endpoint is accessible
- Increase timeout values in `test/config/env.local`

### Debugging

Enable debug logging for more detailed output:

```bash
# Set debug level
export LOG_LEVEL=debug

# Run tests with verbose output
make test-integration V=1
```

## 🛡️ Security Best Practices

### DO ✅

- Use separate API keys for development and production
- Rotate API keys regularly
- Set up rate limiting and usage monitoring
- Use the minimum required permissions
- Keep `env.local` files in `.gitignore`

### DON'T ❌

- Commit API keys to version control
- Share API keys in chat or email
- Use production API keys for testing
- Hardcode credentials in source code
- Leave API keys in terminal history

### Git Hooks (Optional)

Add a pre-commit hook to prevent accidental key commits:

```bash
# Create pre-commit hook
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
if git diff --cached --name-only | xargs grep -l "alchemy.com/v2/" 2>/dev/null; then
    echo "❌ Prevented commit: API key detected!"
    echo "Remove hardcoded API keys before committing."
    exit 1
fi
EOF

chmod +x .git/hooks/pre-commit
```

## 📚 Additional Resources

- [Alchemy Documentation](https://docs.alchemy.com/)
- [Infura Documentation](https://docs.infura.io/)
- [QuickNode Documentation](https://www.quicknode.com/docs/)
- [Ethereum JSON-RPC Specification](https://ethereum.github.io/execution-apis/api-documentation/)
