#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "🚀 Starting OP Stack Operator End-to-End Test"
echo "=============================================="

# Step 1: Setup Kind cluster
echo ""
echo "Step 1: Setting up Kind cluster..."
"${SCRIPT_DIR}/setup-kind.sh"

# Step 2: Build and load operator image
echo ""
echo "Step 2: Building operator image..."
cd "${PROJECT_ROOT}"
make docker-build IMG=opstack-operator:test

echo "Loading operator image into Kind..."
kind load docker-image opstack-operator:test --name opstack-test

# Step 3: Deploy operator to Kind
echo ""
echo "Step 3: Deploying operator to Kind..."
make install  # Install CRDs
make deploy IMG=opstack-operator:test

# Wait for operator to be ready
echo "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/opstack-operator-controller-manager -n opstack-system

# Step 4: Setup secrets
echo ""
echo "Step 4: Setting up test secrets..."
"${SCRIPT_DIR}/setup-secrets.sh"

# Step 5: Instructions for Anvil
echo ""
echo "Step 5: Ready for Anvil!"
echo "=============================================="
echo "📋 Next steps:"
echo ""
echo "1. In a separate terminal, start Anvil:"
echo "   cd ${PROJECT_ROOT}"
echo "   ./scripts/start-anvil.sh"
echo ""
echo "2. Wait for Anvil to start, then deploy the test chain:"
echo "   kubectl apply -f config/samples/test-opchain.yaml"
echo ""
echo "3. Watch the deployment:"
echo "   kubectl get opchain test-chain -w"
echo "   kubectl get pods -w"
echo ""
echo "4. Check logs:"
echo "   kubectl logs -f deployment/opstack-operator-controller-manager -n opstack-system"
echo ""
echo "✅ Environment is ready!" 