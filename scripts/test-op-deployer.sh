#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🧪 Testing op-deployer with Anvil..."

# Test configuration
L1_RPC_URL="http://localhost:8545"
DEPLOYER_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
WORK_DIR="/tmp/op-deployer-test"

# Clean up any previous test
rm -rf "${WORK_DIR}"
mkdir -p "${WORK_DIR}"

echo "Working directory: ${WORK_DIR}"

# Check if Anvil is running
echo "Checking if Anvil is running..."
if ! curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    "${L1_RPC_URL}" > /dev/null; then
    echo "❌ Anvil is not running at ${L1_RPC_URL}"
    echo "Please start Anvil first:"
    echo "  ./scripts/start-anvil.sh"
    exit 1
fi

echo "✅ Anvil is running!"

# Test op-deployer init
echo ""
echo "🔨 Testing op-deployer init..."
cd "${WORK_DIR}"

op-deployer init \
    --l1-chain-id 11155111 \
    --l2-chain-ids 777 \
    --workdir "${WORK_DIR}"

echo "✅ op-deployer init successful!"

# Generate proper intent with test addresses
echo ""
echo "🔧 Generating proper intent with test addresses..."
"${SCRIPT_DIR}/generate-intent.sh" "${WORK_DIR}"

# Check generated files
echo ""
echo "📋 Generated files:"
ls -la "${WORK_DIR}"

if [ -f "${WORK_DIR}/intent.toml" ]; then
    echo ""
    echo "📄 Intent file preview:"
    head -20 "${WORK_DIR}/intent.toml"
else
    echo "⚠️  No intent.toml found"
fi

# Test op-deployer apply (dry run first)
echo ""
echo "🧪 Testing op-deployer apply..."
echo "Deploying L1 contracts to Anvil..."

op-deployer apply \
    --workdir "${WORK_DIR}" \
    --l1-rpc-url "${L1_RPC_URL}" \
    --private-key "${DEPLOYER_KEY}"

echo "✅ op-deployer apply successful!"

# Check deployment artifacts
echo ""
echo "📋 Deployment artifacts:"
ls -la "${WORK_DIR}"

# Test contract inspection
echo ""
echo "🔍 Testing contract inspection..."

if op-deployer inspect genesis --workdir "${WORK_DIR}" 777 > "${WORK_DIR}/genesis.json"; then
    echo "✅ Genesis extraction successful!"
    echo "Genesis file size: $(wc -c < "${WORK_DIR}/genesis.json") bytes"
else
    echo "⚠️  Genesis extraction failed"
fi

if op-deployer inspect rollup --workdir "${WORK_DIR}" 777 > "${WORK_DIR}/rollup.json"; then
    echo "✅ Rollup config extraction successful!"
    echo "Rollup config size: $(wc -c < "${WORK_DIR}/rollup.json") bytes"
else
    echo "⚠️  Rollup config extraction failed"
fi

echo ""
echo "🎉 op-deployer test completed successfully!"
echo "Test artifacts saved in: ${WORK_DIR}" 