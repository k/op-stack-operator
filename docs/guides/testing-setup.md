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
