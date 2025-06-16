#!/bin/bash

# Start Anvil with pre-funded accounts for OP Stack testing
echo "🔨 Starting Anvil (Local L1 Testnet)..."

# Start anvil with:
# - Deterministic accounts (same every time)
# - 10 ETH per account
# - Chain ID 31337 (Anvil default)
# - Host on all interfaces so K8s can reach it
anvil \
  --host 0.0.0.0 \
  --port 8545 \
  --chain-id 11155111 \
  --accounts 10 \
  --balance 10000 \
  --block-time 2 \
  --gas-limit 30000000 \
  --code-size-limit 50000 \
  --mnemonic "test test test test test test test test test test test junk"

# Note: This will run in foreground. The deployer account will be:
# Address: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
# Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 