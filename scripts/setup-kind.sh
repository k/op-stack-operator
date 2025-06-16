#!/bin/bash

set -e

CLUSTER_NAME="opstack-test"

echo "🐋 Setting up Kind cluster for OP Stack testing..."

# Check if cluster already exists
if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "⚠️  Cluster '${CLUSTER_NAME}' already exists. Deleting it first..."
    kind delete cluster --name "${CLUSTER_NAME}"
fi

# Create kind cluster with extra port mappings for our services
cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  # op-geth RPC
  - containerPort: 30545
    hostPort: 8545
    protocol: TCP
  # op-geth WebSocket
  - containerPort: 30546
    hostPort: 8546
    protocol: TCP
  # op-node RPC
  - containerPort: 30547
    hostPort: 8547
    protocol: TCP
EOF

echo "✅ Kind cluster '${CLUSTER_NAME}' created successfully!"

# Wait for cluster to be ready
echo "⏳ Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s

echo "🎯 Cluster is ready! Use: kubectl get nodes" 