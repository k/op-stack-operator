#!/bin/bash

set -e

echo "🔑 Setting up private key secrets for testing..."

# These are the well-known Anvil test accounts
# DO NOT USE THESE IN PRODUCTION - THEY ARE PUBLIC KEYS!

# Deployer private key (first Anvil account)
DEPLOYER_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

# Batcher private key (second Anvil account) 
BATCHER_KEY="0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

# Proposer private key (third Anvil account)
PROPOSER_KEY="0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"

echo "Creating deployer-key secret..."
kubectl create secret generic deployer-key \
  --from-literal=key="${DEPLOYER_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Creating batcher-key secret..."
kubectl create secret generic batcher-key \
  --from-literal=key="${BATCHER_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Creating proposer-key secret..."
kubectl create secret generic proposer-key \
  --from-literal=key="${PROPOSER_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✅ All secrets created successfully!"

# Display the addresses for reference
echo ""
echo "📍 Test Account Addresses:"
echo "Deployer:  0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
echo "Batcher:   0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
echo "Proposer:  0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC" 