# OP Stack Operator - Repository Setup Plan

## Overview

This document outlines the step-by-step plan for setting up the `op-stack-operator` repository using Kubebuilder, the standard tool for building Kubernetes operators in Go.

## Prerequisites

- Go 1.23+ installed
- Docker installed
- kubectl installed
- kind installed (for local testing)
- Kubebuilder v4.0+ installed

## Phase 1: Initial Project Setup

### Step 1: Initialize Kubebuilder Project

```bash
# Create new directory
mkdir op-stack-operator
cd op-stack-operator

# Initialize kubebuilder project
kubebuilder init \
  --domain optimism.io \
  --repo github.com/ethereum-optimism/op-stack-operator \
  --project-name op-stack-operator \
  --project-version v1alpha1

# This creates the basic project structure:
# ├── Dockerfile
# ├── Makefile
# ├── PROJECT
# ├── README.md
# ├── cmd/
# │   └── main.go
# ├── config/
# │   ├── default/
# │   ├── manager/
# │   ├── prometheus/
# │   └── rbac/
# ├── go.mod
# ├── go.sum
# └── internal/
#     └── controller/
```

### Step 2: Create API Groups and CRDs

```bash
# Create OptimismNetwork CRD (foundational)
kubebuilder create api \
  --group optimism \
  --version v1alpha1 \
  --kind OptimismNetwork \
  --resource \
  --controller

# Create OpNode CRD
kubebuilder create api \
  --group optimism \
  --version v1alpha1 \
  --kind OpNode \
  --resource \
  --controller

# Create OpBatcher CRD
kubebuilder create api \
  --group optimism \
  --version v1alpha1 \
  --kind OpBatcher \
  --resource \
  --controller

# Create OpProposer CRD
kubebuilder create api \
  --group optimism \
  --version v1alpha1 \
  --kind OpProposer \
  --resource \
  --controller

# Create OpChallenger CRD
kubebuilder create api \
  --group optimism \
  --version v1alpha1 \
  --kind OpChallenger \
  --resource \
  --controller
```

This creates:

```
api/v1alpha1/
├── groupversion_info.go
├── optimismnetwork_types.go
├── opnode_types.go
├── opbatcher_types.go
├── opproposer_types.go
├── opchallenger_types.go
└── zz_generated.deepcopy.go

internal/controller/
├── optimismnetwork_controller.go
├── opnode_controller.go
├── opbatcher_controller.go
├── opproposer_controller.go
├── opchallenger_controller.go
└── suite_test.go
```

### Step 3: Customize Project Structure

Add additional directories for our specific needs:

```bash
# Create additional directories
mkdir -p {pkg,examples,helm,docs}
mkdir -p pkg/{discovery,config,utils,resources}
mkdir -p examples/{basic,production,testnet}
mkdir -p docs/{api,guides,troubleshooting}
mkdir -p helm/op-stack-operator
mkdir -p test/{e2e,integration}
```

Final structure:

```
op-stack-operator/
├── api/v1alpha1/              # CRD type definitions
├── cmd/                       # Main application
├── config/                    # Kubernetes manifests
│   ├── crd/                   # CRD manifests
│   ├── default/               # Default kustomization
│   ├── manager/               # Manager deployment
│   ├── prometheus/            # Monitoring
│   ├── rbac/                  # RBAC manifests
│   └── samples/               # Sample CRs
├── docs/                      # Documentation
│   ├── api/                   # API documentation
│   ├── guides/                # User guides
│   └── troubleshooting/       # Troubleshooting guides
├── examples/                  # Complete examples
│   ├── basic/                 # Basic deployments
│   ├── production/            # Production configurations
│   └── testnet/               # Testnet examples
├── helm/                      # Helm charts
│   └── op-stack-operator/
├── internal/controller/       # Controller implementations
├── pkg/                       # Shared packages
│   ├── discovery/             # Contract address discovery
│   ├── config/                # Configuration management
│   ├── utils/                 # Shared utilities
│   └── resources/             # Resource creation helpers
├── test/                      # Tests
│   ├── e2e/                   # End-to-end tests
│   └── integration/           # Integration tests
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## Phase 2: Dependency Management

### Step 4: Add Required Dependencies

```bash
# Add Ethereum client dependencies
go get github.com/ethereum/go-ethereum@latest

# Note: OP Stack components will be used as container images, not Go imports
# The Optimism monorepo has complex internal dependencies that make
# direct Go module imports challenging. We'll use official container images instead.

# Add Kubernetes and controller dependencies
go get sigs.k8s.io/controller-runtime@latest
go get k8s.io/apimachinery@latest
go get k8s.io/client-go@latest

# Add additional utilities
go get github.com/prometheus/client_golang@latest
go get github.com/stretchr/testify@latest
go get github.com/onsi/ginkgo/v2@latest
go get github.com/onsi/gomega@latest

# Update go.mod
go mod tidy
```

### Step 4b: Define Container Image Strategy and Version Compatibility

```go
// pkg/config/images.go
package config

import "fmt"

const (
    // Official Optimism container registry
    OptimismRegistry = "us-docker.pkg.dev/oplabs-tools-artifacts/images"

    // Current stable versions (January 2025)
    DefaultOpStackVersion = "v1.9.5"
    DefaultOpGethVersion  = "v1.101511.0" // Based on geth v1.15.11
)

// Container images with default versions
var DefaultImages = ImageConfig{
    OpGeth:      fmt.Sprintf("%s/op-geth:%s", OptimismRegistry, DefaultOpGethVersion),
    OpNode:      fmt.Sprintf("%s/op-node:%s", OptimismRegistry, DefaultOpStackVersion),
    OpBatcher:   fmt.Sprintf("%s/op-batcher:%s", OptimismRegistry, DefaultOpStackVersion),
    OpProposer:  fmt.Sprintf("%s/op-proposer:%s", OptimismRegistry, DefaultOpStackVersion),
    OpChallenger: fmt.Sprintf("%s/op-challenger:%s", OptimismRegistry, DefaultOpStackVersion),
}

// ImageConfig allows overriding default images and supports version compatibility
type ImageConfig struct {
    OpGeth      string `json:"opGeth,omitempty"`
    OpNode      string `json:"opNode,omitempty"`
    OpBatcher   string `json:"opBatcher,omitempty"`
    OpProposer  string `json:"opProposer,omitempty"`
    OpChallenger string `json:"opChallenger,omitempty"`
}

// VersionCompatibilityMatrix defines known compatible version combinations
// These should be updated based on testing and official releases
// These are outdated just use what is already in the repo
var VersionCompatibilityMatrix = map[string]VersionSet{
    // Stable production versions - verified January 2025
    "stable-v1.13": {
        OpNode:       "v1.13.3",     // Latest stable op-node
        OpGeth:       "v1.101511.0", // Based on geth v1.15.11
        OpBatcher:    "v1.12.0",     // Compatible with Isthmus
        OpProposer:   "v1.10.0",     // Latest stable op-proposer
        OpChallenger: "v1.5.1",      // Latest challenger version
    },
}

// VersionSet represents a compatible set of component versions
type VersionSet struct {
    OpNode      string `json:"opNode"`
    OpGeth      string `json:"opGeth"`
    OpBatcher   string `json:"opBatcher"`
    OpProposer  string `json:"opProposer"`
    OpChallenger string `json:"opChallenger"`
}

// ValidateImageCompatibility checks if the provided image versions are compatible
func (ic *ImageConfig) ValidateImageCompatibility() error {
    // Extract versions from image tags and validate compatibility
    // Implementation would parse image tags and check against compatibility matrix
    return nil
}

// GetCompatibleOpGethVersion returns the compatible op-geth version for a given OP Stack version
func GetCompatibleOpGethVersion(opStackVersion string) (string, error) {
    // Parse major.minor from opStackVersion (e.g., "v1.9.5" -> "v1.9")
    // Look up in VersionCompatibilityMatrix
    // Return compatible op-geth version
    return "", nil
}
```

### Step 5: Configure Build System

Update `Makefile` to include our specific needs:

```makefile
# Add to existing Makefile

##@ Development

.PHONY: generate-all
generate-all: manifests generate fmt vet ## Generate all code and manifests

.PHONY: test-unit
test-unit: manifests generate fmt vet envtest ## Run unit tests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: test-integration
test-integration: manifests generate fmt vet ## Run integration tests
	go test ./test/integration/... -v

.PHONY: test-e2e
test-e2e: manifests generate fmt vet ## Run e2e tests
	go test ./test/e2e/... -v

##@ Deployment

.PHONY: kind-load
kind-load: docker-build ## Load image into kind cluster for testing
	kind load docker-image controller:latest

.PHONY: deploy-samples
deploy-samples: ## Deploy sample configurations
	kubectl apply -f config/samples/

##@ Examples

.PHONY: examples-basic
examples-basic: ## Deploy basic example
	kubectl apply -f examples/basic/

.PHONY: examples-production
examples-production: ## Deploy production example
	kubectl apply -f examples/production/
```

## Phase 3: Core Implementation

### Step 6: Implement Type Definitions

Start with the foundational `OptimismNetwork` type:

```go
// api/v1alpha1/optimismnetwork_types.go
type OptimismNetworkSpec struct {
    // Network Configuration
    NetworkName string `json:"networkName,omitempty"`
    ChainID     int64  `json:"chainID"`
    L1ChainID   int64  `json:"l1ChainID"`

    // RPC Endpoints
    L1RpcUrl    string `json:"l1RpcUrl"`
    L1BeaconUrl string `json:"l1BeaconUrl,omitempty"`
    L2RpcUrl    string `json:"l2RpcUrl,omitempty"`

    // L1 RPC Configuration
    L1RpcKind      string        `json:"l1RpcKind,omitempty"`
    L1RpcRateLimit int           `json:"l1RpcRateLimit,omitempty"`
    L1RpcTimeout   time.Duration `json:"l1RpcTimeout,omitempty"`

    // Network-specific Configuration Files
    RollupConfig *ConfigSource `json:"rollupConfig,omitempty"`
    L2Genesis    *ConfigSource `json:"l2Genesis,omitempty"`

    // Contract Address Discovery
    ContractAddresses *ContractAddressConfig `json:"contractAddresses,omitempty"`

    // Shared Configuration
    SharedConfig *SharedConfig `json:"sharedConfig,omitempty"`
}

type ConfigSource struct {
    Inline       string                `json:"inline,omitempty"`
    ConfigMapRef *corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
    AutoDiscover bool                  `json:"autoDiscover,omitempty"`
}

// Additional types as per spec...
```

### Step 7: Implement Core Controllers

Priority order based on dependencies:

1. **OptimismNetwork Controller** (foundational)
2. **OpNode Controller** (needed by others)
3. **OpBatcher Controller**
4. **OpProposer Controller**
5. **OpChallenger Controller**

### Step 8: Create Shared Packages

```go
// pkg/discovery/discovery.go
type ContractDiscoveryService struct {
    l1Client     *ethclient.Client
    l2Client     *ethclient.Client
    cache        map[string]*NetworkContractAddresses
    cacheTimeout time.Duration
}

// pkg/config/config.go
type ComponentConfig struct {
    L1RpcUrl      string
    L1BeaconUrl   string
    NetworkName   string
    ChainID       int64
    ComponentSpec interface{}
    JWTSecret     string
    ConfigMaps    map[string]string
    ServiceRefs   map[string]string
}

// pkg/resources/workloads.go
func CreateStatefulSet(name, namespace string, spec StatefulSetSpec) *appsv1.StatefulSet
func CreateDeployment(name, namespace string, spec DeploymentSpec) *appsv1.Deployment
func CreateService(name, namespace string, spec ServiceSpec) *corev1.Service
```

## Phase 4: Testing Infrastructure

### Step 9: Setup Testing Framework

```bash
# Create test configurations
mkdir -p test/testdata/{configs,manifests}

# Create test utilities
touch test/utils/test_helpers.go
touch test/utils/kind_cluster.go
touch test/utils/mock_ethereum.go
```

### Step 10: Integration Tests

```go
// test/integration/optimismnetwork_test.go
func TestOptimismNetworkController(t *testing.T) {
    // Test network configuration validation
    // Test contract address discovery
    // Test ConfigMap generation
}

// test/integration/opnode_test.go
func TestOpNodeController(t *testing.T) {
    // Test StatefulSet creation
    // Test Service creation
    // Test Secret generation
}
```

### Step 11: E2E Tests

```go
// test/e2e/full_stack_test.go
func TestFullStackDeployment(t *testing.T) {
    // Deploy OptimismNetwork
    // Deploy OpNode (sequencer)
    // Deploy OpBatcher
    // Deploy OpProposer
    // Verify connectivity and functionality
}
```

## Phase 5: Documentation and Examples

### Step 12: Create Examples

```yaml
# examples/basic/optimism-network.yaml
apiVersion: optimism.io/v1alpha1
kind: OptimismNetwork
metadata:
  name: op-sepolia
  namespace: optimism-system
spec:
  networkName: "op-sepolia"
  chainID: 11155420
  l1ChainID: 11155111
  l1RpcUrl: "https://sepolia.infura.io/v3/YOUR-API-KEY"
  sharedConfig:
    logging:
      level: "info"
    metrics:
      enabled: true

# examples/basic/op-node-replica.yaml
apiVersion: optimism.io/v1alpha1
kind: OpNode
metadata:
  name: op-sepolia-replica
  namespace: optimism-system
spec:
  optimismNetworkRef:
    name: "op-sepolia"
    namespace: "optimism-system"
  nodeType: "replica"
  # ... rest of configuration
```

### Step 13: Create Helm Chart

```bash
# Initialize Helm chart
cd helm
helm create op-stack-operator

# Customize for operator deployment
# - Manager deployment
# - CRD installation
# - RBAC configuration
# - Service monitor for metrics
```

### Step 14: Documentation

```markdown
# docs/guides/getting-started.md

# docs/guides/configuration.md

# docs/guides/monitoring.md

# docs/api/optimismnetwork.md

# docs/api/opnode.md

# docs/troubleshooting/common-issues.md
```

## Phase 6: CI/CD and Release

### Step 15: GitHub Actions

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test-unit
      - run: make test-integration

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
      - uses: helm/kind-action@v1
      - run: make test-e2e

# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make docker-build
      - run: make docker-push
      - run: helm package helm/op-stack-operator
```

## Development Workflow

### Daily Development Process

1. **Start with tests**: Write tests for new features first
2. **Implement incrementally**: One CRD/controller at a time
3. **Validate continuously**: Run `make test-unit` frequently
4. **Integration testing**: Use `make test-integration` for component interaction
5. **Local validation**: Use kind cluster for end-to-end testing

### Testing Strategy

1. **Unit Tests**: Controller logic, configuration generation, validation
2. **Integration Tests**: Kubernetes API interactions, resource creation
3. **E2E Tests**: Full workflow testing with real components
4. **Performance Tests**: Resource usage, scalability testing

### Release Process

1. **Version tagging**: Semantic versioning (v0.1.0, v0.2.0, etc.)
2. **Container images**: Multi-arch images pushed to registry
3. **Helm releases**: Packaged charts for easy deployment
4. **Documentation**: Release notes with migration guides

## Success Criteria

- [ ] All 5 CRDs implemented with full spec compliance
- [ ] Comprehensive test coverage (>80%)
- [ ] Working examples for common use cases
- [ ] Complete API documentation
- [ ] Helm chart for easy deployment
- [ ] CI/CD pipeline with automated testing
- [ ] Container images built and published
- [ ] Integration with existing OP Stack components validated

## Version Management Strategy

The operator uses a conservative approach to version management:

### **Version Selection Principles**

- **Stable Releases**: Default to stable, production-tested versions
- **Component Independence**: Allow per-component version overrides
- **Compatibility Validation**: Check versions against known compatibility matrix
- **Flexible Configuration**: Support custom image specifications

### **Implementation Approach**

1. **Default Stable Versions**: Use well-tested stable releases as defaults
2. **Override Capability**: Allow users to specify custom versions when needed
3. **Validation Warnings**: Warn about untested version combinations
4. **Manual Updates**: Version matrix updated through testing and validation

## Additional Considerations

### Security and RBAC

```yaml
# config/rbac/role.yaml - The operator will need extensive permissions:
rules:
  - apiGroups: [""]
    resources: ["secrets", "configmaps", "services", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["optimism.io"]
    resources: ["*"]
    verbs: ["*"]
```

### Error Handling Strategy

```go
// pkg/utils/retry.go
func RetryWithBackoff(operation func() error, maxRetries int) error {
    // Implement exponential backoff for external API calls
    // Essential for contract address discovery and L1/L2 connectivity
}
```

### Monitoring Integration

```yaml
# config/prometheus/monitors.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: op-stack-operator-metrics
spec:
  selector:
    matchLabels:
      app: op-stack-operator
  endpoints:
    - port: metrics
```

## Timeline Estimate (Revised)

- **Phase 1-2**: 2-3 weeks (Project setup, dependency strategy, container image research)
- **Phase 3**: 10-12 weeks (Core implementation - more complex than initially estimated)
- **Phase 4**: 3-4 weeks (Testing infrastructure with real OP Stack components)
- **Phase 5**: 2-3 weeks (Documentation and examples)
- **Phase 6**: 1-2 weeks (CI/CD setup)

**Total**: 18-24 weeks for complete, production-ready implementation

### Critical Success Factors

1. **Container Image Compatibility**: Ensure we use the correct, compatible OP Stack images
2. **Network Configuration**: Proper handling of L1/L2 connectivity and discovery
3. **Secret Management**: Secure handling of private keys and JWT tokens
4. **Resource Management**: Proper StatefulSet and PVC handling for persistent data
5. **Error Recovery**: Robust error handling for external dependencies

This plan provides a structured approach to building a production-ready Kubernetes operator that follows best practices and integrates well with the existing OP Stack ecosystem.
