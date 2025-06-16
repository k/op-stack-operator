# OP Stack Kubernetes Operator — `OPChain`

## 🎯 Goal

Create a Kubernetes Operator using **Kubebuilder** to manage OP Stack L2 services (op-geth, op-node, op-batcher, op-proposer) for deploying and managing L2 chains via a single custom resource (`OPChain`). The operator focuses on **L2 service lifecycle management** using **pre-deployed L1 contracts and configurations**.

---

## 📐 Architecture Overview

### **Separation of Concerns**

```
┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────────┐
│ L1 Deployment       │    │ Kubernetes Operator │    │ L2 Services         │
│ (External)          │───▶│ (This Project)      │───▶│ (Managed)           │
│                     │    │                     │    │                     │
│ • op-deployer       │    │ • Config Loading    │    │ • op-geth           │
│ • Scripts           │    │ • Manifest Gen      │    │ • op-node           │
│ • CI/CD             │    │ • Lifecycle Mgmt    │    │ • op-batcher        │
│ • Manual Setup      │    │ • Status Reporting  │    │ • op-proposer       │
└─────────────────────┘    └─────────────────────┘    └─────────────────────┘
```

### Custom Resource Definition (CRD): `OPChain`

```yaml
apiVersion: rollup.oplabs.io/v1alpha1
kind: OPChain
metadata:
  name: mychain
spec:
  # L1 Configuration (from external deployment)
  l1:
    rpcUrl: https://sepolia.infura.io/v3/abc...
    chainId: 11155111
    contractAddresses:
      optimismPortal: 0x1234...
      l2OutputOracle: 0x5678...
      systemConfig: 0x9abc...
      batcher: 0xbdef...
      proposer: 0xcdef...
  
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
      storage: 100Gi
    node:
      enabled: true
      image: "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.9.4"
      metricsEnabled: true
    batcher:
      enabled: true
      signerPrivateKeySecret: l1-batcher-key
    proposer:
      enabled: true
      signerPrivateKeySecret: l1-proposer-key
```

### Operator Responsibilities
- **Configuration Loading**: Load genesis.json, rollup.json, and JWT secrets from Kubernetes secrets
- **L2 Service Management**: Generate and apply Kubernetes manifests for OP Stack L2 components
- **Lifecycle Management**: Install, update, delete L2 services with full reconciliation loop
- **Status Reporting**: Track L2 service health, sync status, and component readiness
- **Multi-role Support**: Deploy sequencer, batcher, proposer, and verifier configurations

⸻

## 🔨 Implementation Plan

### 📦 1. Update API Definition

Update the `OPChain` CRD to reflect the new architecture:

```bash
# Update api/v1alpha1/opchain_types.go
# - Remove L1 deployment fields
# - Add L2 configuration secrets
# - Add L1 contract addresses
# - Update status conditions
```

⸻

### 🏗️ 2. Configuration Management Strategy

**Approach**: External L1 deployment + Kubernetes secrets

- **L1 Deployment**: External tools (op-deployer, scripts, CI/CD)
- **Configuration Storage**: Kubernetes secrets for genesis.json, rollup.json, JWT
- **Operator Input**: Pre-generated configurations and contract addresses
- **No Dependencies**: No embedded op-deployer or L1 integration

**Structure**:
```
pkg/config/
├── loader.go              # Load configs from secrets
├── validator.go           # Validate configurations
├── types.go               # Configuration data structures
└── utils.go               # Helper functions
```

⸻

### 🧠 3. Updated Reconciliation Logic

1. **Detect** creation/update of OPChain objects
2. **Load** configuration from Kubernetes secrets (genesis.json, rollup.json, JWT)
3. **Validate** configuration and contract addresses
4. **Generate** L2 component manifests from templates
5. **Apply** manifests using controller-runtime client
6. **Track** L2 service health and sync status
7. **Report** status back to OPChain resource

⸻

### 🔐 4. Enhanced Secret Management

- **Configuration Secrets**: Genesis and rollup configs from external deployment
- **L1 Private Keys**: Reference existing Kubernetes Secrets for batcher/proposer
- **JWT Tokens**: Shared secrets between op-geth and op-node
- **TLS Certificates**: Optional cert-manager integration
- **External Secrets**: Support for Vault, AWS Secrets Manager, etc.

⸻

### 📊 5. Focused Status Reporting

- **Deployment Phase**: Pending, ConfigLoading, Deploying, Running, Error, Upgrading
- **L2 Service Health**: Individual component readiness and health
- **Sync Status**: L2 block heights, safe/unsafe block numbers
- **Configuration Status**: Config loading and validation status
- **Event Emission**: Kubernetes events for important state changes

⸻

### 🧪 6. Simplified Testing Strategy

- **Unit Tests**: Configuration loading, manifest generation, controller logic
- **Integration Tests**: L2 service deployment with mock configurations
- **E2E Tests**: Complete L2 chain deployment scenarios
- **Mock Configs**: Pre-generated genesis.json and rollup.json for testing

⸻

### 🎛️ 7. Enhanced Features

- **Multi-tenancy**: Namespace isolation and resource quotas
- **High Availability**: Multi-replica configurations where applicable
- **Upgrades**: Rolling updates with configuration compatibility checks
- **Backup/Restore**: L2 data backup strategies
- **Monitoring**: Comprehensive metrics and health checks
- **Autoscaling**: Resource-based scaling where appropriate

⸻

## 🚀 **Deployment Workflow**

### **Phase 1: L1 Deployment (External)**

```bash
# 1. Deploy L1 contracts (external process)
op-deployer init --l1-chain-id 11155111 --l2-chain-ids 777
op-deployer apply --l1-rpc-url $L1_RPC --private-key $DEPLOYER_KEY

# 2. Extract configuration artifacts
op-deployer inspect genesis 777 > genesis.json
op-deployer inspect rollup 777 > rollup.json

# 3. Generate shared secrets
openssl rand -hex 32 > jwt-secret.txt

# 4. Create Kubernetes secrets
kubectl create secret generic mychain-genesis --from-file=genesis.json
kubectl create secret generic mychain-rollup --from-file=rollup.json
kubectl create secret generic mychain-jwt --from-file=jwt=jwt-secret.txt
kubectl create secret generic batcher-key --from-literal=key=$BATCHER_PRIVATE_KEY
kubectl create secret generic proposer-key --from-literal=key=$PROPOSER_PRIVATE_KEY
```

### **Phase 2: L2 Service Deployment (Operator)**

```bash
# 5. Deploy OPChain resource
kubectl apply -f mychain-opchain.yaml

# 6. Monitor deployment
kubectl get opchain mychain -w
kubectl get pods -l app.kubernetes.io/instance=mychain -w
```

## 🎯 **Benefits of This Approach**

### **1. Clear Separation of Concerns**
- **L1 Platform Team**: Manages contract deployment, provides configs
- **L2 Application Team**: Manages L2 services via operator

### **2. Simplified Testing**
- No complex op-deployer integration in tests
- Standard Kubernetes testing patterns
- Clear input/output boundaries

### **3. Better Operational Model**
- L1 contracts deployed via existing DevOps practices
- L2 services managed via GitOps and Kubernetes native tools
- Different teams can own different aspects

### **4. Reduced Complexity**
- Focused operator responsibility (L2 services only)
- No embedded external tools or complex integrations
- Standard Kubernetes patterns throughout

### **5. Deployment Flexibility**
- Support for any L1 deployment method
- Pre-existing L1 contracts can be used
- Multiple L2 chains can share L1 infrastructure

## 📋 **Project Structure Updates**

```
opstack-operator/
├── api/v1alpha1/
│   ├── opchain_types.go          # Updated CRD schema (L2-focused)
│   └── groupversion_info.go
├── controllers/
│   └── opchain_controller.go     # Simplified reconciliation logic
├── pkg/
│   ├── config/
│   │   ├── loader.go             # Configuration loading from secrets
│   │   ├── validator.go          # Configuration validation
│   │   ├── types.go              # Configuration data structures
│   │   └── utils.go              # Helper functions
│   ├── manifests/
│   │   ├── templates/            # L2 component templates
│   │   │   ├── op-geth/
│   │   │   ├── op-node/
│   │   │   ├── op-batcher/
│   │   │   └── op-proposer/
│   │   ├── generator.go          # Template rendering engine
│   │   ├── types.go              # Template data structures
│   │   └── utils.go              # Helper functions
│   └── status/
│       ├── reporter.go           # Status reporting logic
│       └── health.go             # Health checking
├── config/
│   ├── samples/                  # Updated sample OPChain resources
│   └── ...                       # Standard Kubebuilder configs
├── scripts/
│   ├── deploy-l1.sh              # Example L1 deployment script
│   ├── setup-configs.sh          # Example config preparation
│   └── e2e-test.sh               # Simplified E2E testing
└── docs/
    ├── deployment-guide.md       # Updated deployment documentation
    └── configuration-guide.md    # Configuration management guide
```

⸻

## 🧩 **Implementation Steps**

1. **Update CRD schema** with new L2-focused fields
2. **Remove op-deployer integration** and related complexity
3. **Implement configuration loading** from Kubernetes secrets
4. **Update manifest templates** to use loaded configurations
5. **Simplify controller logic** to focus on L2 service management
6. **Add comprehensive unit tests** for new architecture
7. **Update documentation** and examples
8. **Create E2E tests** with mock configurations

Ready to implement the simplified, focused architecture! 🚀