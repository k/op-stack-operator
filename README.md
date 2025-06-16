# OP Stack Kubernetes Operator

A Kubernetes operator for managing **OP Stack L2 services** (op-geth, op-node, op-batcher, op-proposer) with a focus on **lifecycle management** of pre-configured L2 chains.

## 🎯 **Overview**

The OP Stack Kubernetes Operator simplifies the deployment and management of OP Stack-based L2 services on Kubernetes. It provides a declarative interface through a single Custom Resource Definition (`OPChain`) that manages the lifecycle of L2 components using **pre-deployed L1 contracts and configurations**.

### **Key Features**

- 🚀 **L2 Service Management**: Deploy and manage op-geth, op-node, op-batcher, and op-proposer
- 🔧 **Configuration-Driven**: Uses pre-generated genesis.json and rollup.json from external L1 deployment
- 📊 **Status Reporting**: Comprehensive health monitoring and status tracking  
- 🔒 **Secret Management**: Secure handling of private keys and JWT tokens
- 🎛️ **Lifecycle Management**: Complete CRUD operations with reconciliation loops
- 🏗️ **Cloud-Native**: Built with Kubebuilder following Kubernetes best practices

### **Architecture: Separated Concerns**

```
┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────────┐
│ L1 Deployment       │    │ Kubernetes Operator │    │ L2 Services         │
│ (External)          │───▶│ (This Project)      │───▶│ (Managed)           │
│                     │    │                     │    │                     │
│ • op-deployer       │    │ • Config Loading    │    │ • op-geth           │
│ • Scripts/CI/CD     │    │ • Manifest Gen      │    │ • op-node           │
│ • Manual Setup      │    │ • Lifecycle Mgmt    │    │ • op-batcher        │
│ • Platform Team     │    │ • Status Reporting  │    │ • op-proposer       │
└─────────────────────┘    └─────────────────────┘    └─────────────────────┘
```

## 🚀 **Quick Start**

### **Prerequisites**
- Go version v1.23.0+
- Docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster
- **Pre-deployed L1 contracts** (via op-deployer or other tools)

### **Step 1: Prepare L1 Configuration (External)**

Deploy L1 contracts and extract configuration:

```bash
# Deploy L1 contracts using op-deployer
op-deployer init --l1-chain-id 11155111 --l2-chain-ids 777
op-deployer apply --l1-rpc-url $L1_RPC --private-key $DEPLOYER_KEY

# Extract configuration artifacts
op-deployer inspect genesis 777 > genesis.json
op-deployer inspect rollup 777 > rollup.json

# Generate JWT secret
openssl rand -hex 32 > jwt-secret.txt
```

### **Step 2: Create Kubernetes Secrets**

```bash
# Create configuration secrets
kubectl create secret generic mychain-genesis --from-file=genesis.json
kubectl create secret generic mychain-rollup --from-file=rollup.json
kubectl create secret generic mychain-jwt --from-file=jwt=jwt-secret.txt

# Create private key secrets
kubectl create secret generic batcher-key --from-literal=key=$BATCHER_PRIVATE_KEY
kubectl create secret generic proposer-key --from-literal=key=$PROPOSER_PRIVATE_KEY
```

### **Step 3: Deploy the Operator**

```bash
# Install CRDs
make install

# Deploy the operator
make deploy IMG=<your-registry>/op-stack-operator:latest
```

### **Step 4: Deploy an OPChain**

```yaml
apiVersion: rollup.oplabs.io/v1alpha1
kind: OPChain
metadata:
  name: mychain
  namespace: default
spec:
  # L1 Configuration (from external deployment)
  l1:
    rpcUrl: "https://sepolia.infura.io/v3/YOUR_KEY"
    chainId: 11155111
    contractAddresses:
      optimismPortal: "0x1234567890abcdef..."
      l2OutputOracle: "0xabcdef1234567890..."
      systemConfig: "0x9876543210fedcba..."
  
  # L2 Configuration (from external deployment)  
  l2:
    chainId: 777
    networkName: "mychain"
    genesisConfigSecret: "mychain-genesis"
    rollupConfigSecret: "mychain-rollup"
    jwtSecret: "mychain-jwt"
  
  # Component Configuration (operator manages)
  components:
    geth:
      enabled: true
      image: "ethereum/client-go:v1.13.15"
      storage:
        size: "100Gi"
        storageClass: "fast-ssd"
    
    node:
      enabled: true
      image: "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.9.4"
    
    batcher:
      enabled: true
      signerPrivateKeySecret: "batcher-key"
    
    proposer:
      enabled: true
      signerPrivateKeySecret: "proposer-key"
```

```bash
# Apply the configuration
kubectl apply -f mychain.yaml

# Monitor deployment
kubectl get opchain mychain -w
kubectl get pods -l app.kubernetes.io/instance=mychain -w
```

## 📋 **Configuration Reference**

### **OPChain Specification**

| Field | Description | Required |
|-------|-------------|----------|
| `l1.rpcUrl` | L1 RPC endpoint URL | ✅ |
| `l1.chainId` | L1 chain ID | ✅ |
| `l1.contractAddresses.*` | L1 contract addresses from deployment | ✅ |
| `l2.chainId` | L2 chain ID | ✅ |
| `l2.genesisConfigSecret` | Secret containing genesis.json | ✅ |
| `l2.rollupConfigSecret` | Secret containing rollup.json | ✅ |
| `l2.jwtSecret` | Secret containing JWT token | ✅ |
| `components.*.enabled` | Enable/disable component | ✅ |
| `components.*.image` | Container image | ✅ |
| `components.*.resources` | Resource requests/limits | ❌ |

### **Status Conditions**

| Condition | Description |
|-----------|-------------|
| `ConfigurationValid` | Configuration loaded and validated |
| `L2ServicesReady` | All enabled L2 services are ready |
| `GethReady` | op-geth service is healthy |
| `NodeReady` | op-node service is healthy |
| `BatcherReady` | op-batcher service is healthy |
| `ProposerReady` | op-proposer service is healthy |

## 🛠️ **Development**

### **Local Development**

```bash
# Install dependencies
go mod download

# Generate manifests
make manifests

# Run tests
make test

# Run locally (requires kubeconfig)
make run
```

### **Build and Deploy**

```bash
# Build binary
make build

# Build container image
make docker-build IMG=<your-registry>/op-stack-operator:tag

# Push to registry
make docker-push IMG=<your-registry>/op-stack-operator:tag

# Deploy to cluster
make deploy IMG=<your-registry>/op-stack-operator:tag
```

### **Testing**

```bash
# Unit tests
make test

# Integration tests with Kind
./scripts/e2e-test.sh

# Set up test environment
./scripts/setup-kind.sh
./scripts/start-anvil.sh
```

## 📖 **Documentation**

- **[Architecture](ARCHITECTURE.md)**: Detailed architectural overview
- **[Implementation Notes](IMPLEMENTATION_NOTES.md)**: Implementation details and decisions
- **[Progress](PROGRESS.md)**: Development progress and status

## 🔧 **Management Commands**

### **Installation**

```bash
# Install CRDs
make install

# Deploy operator
make deploy

# Create sample resources
kubectl apply -k config/samples/
```

### **Cleanup**

```bash
# Delete sample resources
kubectl delete -k config/samples/

# Remove operator
make undeploy

# Uninstall CRDs
make uninstall
```

## 🏭 **Production Deployment**

### **Via YAML Bundle**

```bash
# Generate installer
make build-installer IMG=myregistry/op-stack-operator:v1.0.0

# Deploy via installer
kubectl apply -f dist/install.yaml
```

### **Via Helm Chart** (Coming Soon)

```bash
# Install via Helm
helm install op-stack-operator ./dist/chart
```

## 🤝 **Contributing**

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature/amazing-feature`
3. **Commit** your changes: `git commit -m 'Add amazing feature'`
4. **Push** to the branch: `git push origin feature/amazing-feature`
5. **Open** a Pull Request

### **Development Workflow**

1. Update documentation first (if architectural changes)
2. Implement changes with tests
3. Ensure all tests pass: `make test`
4. Validate locally: `make run`
5. Test end-to-end: `./scripts/e2e-test.sh`

## 📄 **License**

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. 