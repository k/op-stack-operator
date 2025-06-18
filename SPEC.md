# OP Stack Kubernetes Operator - Comprehensive Specification

## Executive Summary

This document specifies a Kubernetes operator for managing OP Stack components, supporting both public node operators and chain operators. The operator manages the lifecycle of consensus layer clients (op-node + op-geth), and chain operation services (op-batcher, op-proposer, op-challenger) through a set of Custom Resource Definitions (CRDs) and controllers.

## Architecture Overview

### Design Principles

1. **Separation of Concerns**: Each OP Stack component has its own CRD and controller
2. **Configuration Inheritance**: Shared configurations are managed centrally via `OptimismNetwork`
3. **Operational Flexibility**: Support both public node operators and chain operators
4. **Security First**: Proper secret management and network isolation
5. **Kubernetes Native**: Leverage native Kubernetes patterns and best practices

### Component Relationships

```
OptimismNetwork (Central Config)
├── OpNode (Consensus Layer - Sequencer or Replica)
├── OpBatcher (Chain Operations - L2 to L1 batch submission)
├── OpProposer (Chain Operations - Output root proposals)
└── OpChallenger (Chain Operations - Dispute resolution)
```

## Custom Resource Definitions (CRDs)

### 1. OptimismNetwork CRD

**Purpose**: Central configuration resource that defines network-wide parameters shared across all components.

#### Spec Schema

```yaml
apiVersion: optimism.io/v1alpha1
kind: OptimismNetwork
metadata:
  name: op-mainnet
  namespace: optimism-system
spec:
  # Network Configuration
  networkName: "op-mainnet" # Optional: well-known network name
  chainID: 10 # L2 Chain ID
  l1ChainID: 1 # L1 Chain ID (Ethereum mainnet = 1)

  # RPC Endpoints
  l1RpcUrl: "https://eth-mainnet.alchemyapi.io/v2/YOUR-API-KEY"
  l1BeaconUrl: "https://eth-beacon.example.com"
  l2RpcUrl: "http://op-geth:8545" # Internal L2 RPC (for components)

  # L1 RPC Configuration
  l1RpcKind: "alchemy" # alchemy, quicknode, infura, basic_http, etc.
  l1RpcRateLimit: 0 # requests per second, 0 = disabled
  l1RpcTimeout: "10s"

  # Network-specific Configuration Files
  rollupConfig:
    # Option 1: Inline configuration
    inline: |
      {
        "genesis": { ... },
        "block_time": 2,
        "seq_window_size": 3600
      }
    # Option 2: Reference to ConfigMap
    configMapRef:
      name: "op-mainnet-rollup-config"
      key: "rollup.json"
    # Option 3: Auto-discovery (default) - controller fetches from L2
    autoDiscover: true

  l2Genesis:
    # Option 1: Inline configuration
    inline: |
      {
        "config": { ... },
        "alloc": { ... }
      }
    # Option 2: Reference to ConfigMap
    configMapRef:
      name: "op-mainnet-genesis"
      key: "genesis.json"
    # Option 3: Auto-discovery (default) - controller fetches from L2
    autoDiscover: true

  # Contract Address Discovery (optional - will be auto-discovered if not provided)
  contractAddresses:
    # L1 Contract Addresses
    systemConfigAddr: "0x229047fed2591dbec1eF1118d64F7aF3dB9EB290" # Optional: helps discovery
    l2OutputOracleAddr: "" # Auto-discovered from SystemConfig or registry
    disputeGameFactoryAddr: "" # Auto-discovered from SystemConfig or registry
    optimismPortalAddr: "" # Auto-discovered from SystemConfig or registry

    # Discovery configuration
    discoveryMethod: "auto" # auto, superchain-registry, well-known, manual
    cacheTimeout: "24h" # How long to cache discovered addresses

  # Shared Configuration
  sharedConfig:
    # Logging
    logging:
      level: "info" # trace, debug, info, warn, error
      format: "logfmt" # logfmt, json
      color: false

    # Metrics
    metrics:
      enabled: true
      port: 7300
      path: "/metrics"

    # Resource Defaults
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"
      limits:
        cpu: "1000m"
        memory: "2Gi"

    # Security
    security:
      runAsNonRoot: true
      runAsUser: 1000
      fsGroup: 1000
      seccompProfile:
        type: "RuntimeDefault"

status:
  phase: "Ready" # Pending, Ready, Error
  conditions:
    - type: "ConfigurationValid"
      status: "True"
      reason: "ValidConfiguration"
      message: "Network configuration is valid"
    - type: "ContractsDiscovered"
      status: "True"
      reason: "AddressesResolved"
      message: "All contract addresses discovered successfully"
    - type: "L1Connected"
      status: "True"
      reason: "RPCEndpointReachable"
      message: "L1 RPC endpoint is responsive"
    - type: "L2Connected"
      status: "True"
      reason: "RPCEndpointReachable"
      message: "L2 RPC endpoint is responsive"

  observedGeneration: 1
  networkInfo:
    deploymentTimestamp: "2024-01-15T10:00:00Z"
    lastUpdated: "2024-01-15T10:00:00Z"

    # Discovered contract addresses (populated by controller)
    discoveredContracts:
      l2OutputOracleAddr: "0xdfe97868233d1aa22e815a266982f2cf17685a27"
      disputeGameFactoryAddr: "0xe5965Ab5962eDc7477C8520243A95517CD252fA9"
      optimismPortalAddr: "0xbEb5Fc579115071764c7423A4f12eDde41f106Ed"
      systemConfigAddr: "0x229047fed2591dbec1eF1118d64F7aF3dB9EB290"
      lastDiscoveryTime: "2024-01-15T10:00:00Z"
      discoveryMethod: "system-config" # system-config, superchain-registry, well-known
```

#### Controller Responsibilities

- Validate network configuration and L1/L2 connectivity
- **Discover and cache contract addresses from L1/L2 chains**
- Generate and manage ConfigMaps for rollup config and genesis data
- Create default JWT secrets if not provided
- Ensure consistency of shared parameters across dependent components
- Monitor L1/L2 RPC endpoint health

#### Contract Address Discovery

The OptimismNetwork controller automatically discovers contract addresses by querying the L1 and L2 chains:

```go
type NetworkContractAddresses struct {
    // L1 Contracts (discovered from L1 chain)
    L2OutputOracleAddr         string `json:"l2OutputOracleAddr"`
    DisputeGameFactoryAddr     string `json:"disputeGameFactoryAddr"`
    OptimismPortalAddr         string `json:"optimismPortalAddr"`
    SystemConfigAddr           string `json:"systemConfigAddr"`
    L1CrossDomainMessengerAddr string `json:"l1CrossDomainMessengerAddr"`
    L1StandardBridgeAddr       string `json:"l1StandardBridgeAddr"`

    // L2 Contracts (discovered from L2 chain or computed)
    L2CrossDomainMessengerAddr string `json:"l2CrossDomainMessengerAddr"`
    L2StandardBridgeAddr       string `json:"l2StandardBridgeAddr"`
    L2ToL1MessagePasserAddr    string `json:"l2ToL1MessagePasserAddr"`
}

func (r *OptimismNetworkReconciler) discoverContractAddresses(ctx context.Context, network *OptimismNetwork) (*NetworkContractAddresses, error) {
    addresses := &NetworkContractAddresses{}

    // Connect to L1 RPC
    l1Client, err := ethclient.Dial(network.Spec.L1RpcUrl)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to L1 RPC: %w", err)
    }
    defer l1Client.Close()

    // Method 1: Query SystemConfig contract for other contract addresses
    if network.Spec.SystemConfigAddr != "" {
        systemConfig, err := r.getSystemConfigContract(l1Client, network.Spec.SystemConfigAddr)
        if err == nil {
            addresses.L2OutputOracleAddr = systemConfig.L2OutputOracle()
            addresses.DisputeGameFactoryAddr = systemConfig.DisputeGameFactory()
            addresses.OptimismPortalAddr = systemConfig.OptimismPortal()
        }
    }

    // Method 2: Use Superchain Registry (future enhancement)
    if addresses.L2OutputOracleAddr == "" {
        registryAddresses, err := r.querySuperchainRegistry(network.Spec.ChainID)
        if err == nil {
            addresses = registryAddresses
        }
    }

    // Method 3: Use well-known addresses for official networks
    if addresses.L2OutputOracleAddr == "" {
        wellKnown := r.getWellKnownAddresses(network.Spec.NetworkName, network.Spec.ChainID)
        if wellKnown != nil {
            addresses = wellKnown
        }
    }

    // Connect to L2 and verify/discover L2 contracts
    if network.Spec.L2RpcUrl != "" {
        l2Client, err := ethclient.Dial(network.Spec.L2RpcUrl)
        if err == nil {
            defer l2Client.Close()
            r.discoverL2Contracts(l2Client, addresses)
        }
    }

    return addresses, nil
}

// Well-known contract addresses for official networks
func (r *OptimismNetworkReconciler) getWellKnownAddresses(networkName string, chainID int64) *NetworkContractAddresses {
    switch {
    case networkName == "op-mainnet" || chainID == 10:
        return &NetworkContractAddresses{
            L2OutputOracleAddr:     "0xdfe97868233d1aa22e815a266982f2cf17685a27",
            DisputeGameFactoryAddr: "0xe5965Ab5962eDc7477C8520243A95517CD252fA9",
            OptimismPortalAddr:     "0xbEb5Fc579115071764c7423A4f12eDde41f106Ed",
            SystemConfigAddr:       "0x229047fed2591dbec1eF1118d64F7aF3dB9EB290",
            // ... other addresses
        }
    case networkName == "op-sepolia" || chainID == 11155420:
        return &NetworkContractAddresses{
            L2OutputOracleAddr:     "0x90E9c4f8a994a250F6aEfd61CAFb4F2e895D458F",
            DisputeGameFactoryAddr: "0x05F9613aDB30026FFd634f38e5C4dFd30a197Fa1",
            // ... other addresses
        }
    case networkName == "base-mainnet" || chainID == 8453:
        return &NetworkContractAddresses{
            L2OutputOracleAddr:     "0x56315b90c40730925ec5485cf004d835058518A0",
            DisputeGameFactoryAddr: "0x43edB88C4B80fDD2AdFF2412A7BebF9dF42cB40e",
            // ... other addresses
        }
    default:
        return nil
    }
}
```

---

### 2. OpNode CRD

**Purpose**: Manages op-node (consensus layer) paired with op-geth (execution layer). Supports both sequencer and replica configurations.

#### Spec Schema

```yaml
apiVersion: optimism.io/v1alpha1
kind: OpNode
metadata:
  name: op-mainnet-sequencer
  namespace: optimism-system
spec:
  # Network Reference
  optimismNetworkRef:
    name: "op-mainnet"
    namespace: "optimism-system"

  # Node Type
  nodeType: "sequencer" # sequencer, replica

  # op-node Configuration
  opNode:
    # Sync Configuration
    syncMode: "execution-layer" # execution-layer, consensus-layer

    # P2P Configuration
    p2p:
      enabled: true
      listenPort: 9003
      discovery:
        enabled: true # Set to false for sequencer isolation
        bootnodes:
          - "enr://..."
      static:
        - "16Uiu2HAm..." # Static peer list for sequencer isolation
      peerScoring:
        enabled: true
      bandwidthLimit: "10MB"

      # P2P Key Management
      privateKey:
        # Option 1: Reference existing secret
        secretRef:
          name: "op-node-p2p-key"
          key: "private-key"
        # Option 2: Auto-generate (default)
        generate: true

    # RPC Configuration
    rpc:
      enabled: true
      host: "0.0.0.0"
      port: 9545
      enableAdmin: false # Set to true for sequencer
      cors:
        origins: ["*"]
        methods: ["GET", "POST"]

    # Sequencer-specific Configuration
    sequencer:
      enabled: false # Set to true for sequencer nodes
      blockTime: "2s"
      maxTxPerBlock: 1000

    # Engine API Configuration (communication with op-geth)
    engine:
      jwtSecret:
        # Option 1: Reference existing secret
        secretRef:
          name: "engine-jwt-secret"
          key: "jwt"
        # Option 2: Auto-generate shared secret for op-node + op-geth (default)
        generate: true
      endpoint: "http://127.0.0.1:8551" # Same pod communication

  # op-geth Configuration
  opGeth:
    # Initialization
    network: "op-mainnet" # Must match OptimismNetwork

    # Data Directory and Storage
    dataDir: "/data/geth"
    storage:
      size: "1Ti"
      storageClass: "fast-ssd" # Override default storage class
      accessMode: "ReadWriteOnce"

    # Sync Configuration
    syncMode: "snap" # snap, full
    gcMode: "full" # full, archive
    stateScheme: "path" # path, hash

    # Database Configuration
    cache: 4096 # Cache size in MB
    dbEngine: "pebble" # pebble, leveldb

    # Network Configuration
    networking:
      http:
        enabled: true
        host: "0.0.0.0"
        port: 8545
        apis: ["web3", "eth", "net", "debug"]
        cors:
          origins: ["*"]
          methods: ["GET", "POST"]

      ws:
        enabled: true
        host: "0.0.0.0"
        port: 8546
        apis: ["web3", "eth", "net"]
        origins: ["*"]

      authrpc:
        host: "127.0.0.1"
        port: 8551
        apis: ["engine", "eth"]

      p2p:
        port: 30303
        maxPeers: 50
        noDiscovery: false # Set to true for sequencer isolation
        netRestrict: "" # "10.0.0.0/8" for internal networks
        static: [] # Static peer list

    # Transaction Pool Configuration
    txpool:
      locals: []
      noLocals: true
      journal: "transactions.rlp"
      journalRemotes: false
      lifetime: "1h"
      priceBump: 10

      # Pool limits
      accountSlots: 16
      globalSlots: 5120
      accountQueue: 64
      globalQueue: 1024

    # Rollup-specific Configuration
    rollup:
      disableTxPoolGossip: false
      computePendingBlock: false

  # Resource Configuration (adjust based on node type and storage requirements)
  resources:
    opNode:
      requests:
        cpu: "500m"
        memory: "1Gi"
      limits:
        cpu: "2000m"
        memory: "4Gi"

    opGeth:
      # Default for full nodes - archive nodes need significantly more
      requests:
        cpu: "2000m" # Increased for better performance
        memory: "8Gi" # Increased for state caching
      limits:
        cpu: "8000m" # Increased for archive node support
        memory: "32Gi" # Increased for large state size

  # Service Configuration
  service:
    type: "ClusterIP" # ClusterIP, NodePort, LoadBalancer
    annotations: {}

    ports:
      # op-geth ports
      - name: "geth-http"
        port: 8545
        targetPort: 8545
        protocol: "TCP"
      - name: "geth-ws"
        port: 8546
        targetPort: 8546
        protocol: "TCP"
      - name: "geth-p2p"
        port: 30303
        targetPort: 30303
        protocol: "TCP"

      # op-node ports
      - name: "node-rpc"
        port: 9545
        targetPort: 9545
        protocol: "TCP"
      - name: "node-p2p"
        port: 9003
        targetPort: 9003
        protocol: "TCP"

status:
  phase: "Running" # Pending, Initializing, Running, Error, Stopped
  conditions:
    - type: "InitializationComplete"
      status: "True"
      reason: "GenesisInitialized"
      message: "op-geth genesis block initialized"
    - type: "P2PConnected"
      status: "True"
      reason: "PeersConnected"
      message: "Connected to 5 peers"
    - type: "Syncing"
      status: "False"
      reason: "FullySynced"
      message: "Node is fully synced with L1"

  nodeInfo:
    chainHead:
      blockNumber: 12345678
      blockHash: "0xabc123..."
      timestamp: "2024-01-15T10:30:00Z"

    syncStatus:
      currentBlock: 12345678
      highestBlock: 12345678
      syncing: false # Simplified sync status

    peerCount: 5
    engineConnected: true

  observedGeneration: 1
```

#### Controller Responsibilities

- Initialize op-geth with genesis data or network flags
- Generate and manage JWT secrets for engine API communication
- Create StatefulSet for op-geth (persistent data) and Deployment for op-node (stateless)
- Manage P2P key generation and storage
- Configure service discovery and networking
- Handle rolling updates and configuration changes
- Monitor sync status and peer connectivity

---

### 3. OpBatcher CRD

**Purpose**: Manages op-batcher instances responsible for submitting L2 transaction batches to L1.

#### Spec Schema

```yaml
apiVersion: optimism.io/v1alpha1
kind: OpBatcher
metadata:
  name: op-mainnet-batcher
  namespace: optimism-system
spec:
  # Network Reference
  optimismNetworkRef:
    name: "op-mainnet"
    namespace: "optimism-system"

  # L2 Sequencer Configuration
  sequencerRef:
    name: "op-mainnet-sequencer" # Reference to OpNode instance
    namespace: "optimism-system" # Optional, defaults to same namespace

  # Private Key for L1 Transaction Signing
  privateKey:
    secretRef:
      name: "batcher-private-key"
      key: "private-key"

  # Batching Configuration
  batching:
    maxChannelDuration: "10m" # Maximum duration for a channel
    subSafetyMargin: "10" # Safety margin for L1 confirmations
    targetL1TxSize: "120000" # Target size for L1 transactions (bytes)
    targetNumFrames: "1" # Target number of frames per transaction
    approxComprRatio: "0.4" # Approximate compression ratio

  # Data Availability Configuration
  dataAvailability:
    type: "blobs" # blobs, calldata
    maxBlobsPerTx: "6" # Maximum blobs per transaction (EIP-4844)

  # Throttling Configuration
  throttling:
    enabled: true
    maxPendingTx: "10" # Maximum pending transactions
    backlogSafetyMargin: "10" # Safety margin for backlog

  # L1 Transaction Management
  l1Transaction:
    feeLimitMultiplier: "5" # Fee limit multiplier for dynamic fees
    resubmissionTimeout: "48s" # Timeout before resubmitting transaction
    numConfirmations: "10" # Number of confirmations to wait
    safeAbortNonceTooLowCount: "3" # Abort threshold for nonce too low errors

  # RPC Configuration
  rpc:
    enabled: true
    host: "127.0.0.1"
    port: 8548
    enableAdmin: true

  # Metrics Configuration
  metrics:
    enabled: true
    host: "0.0.0.0"
    port: 7300

  # Resources
  resources:
    requests:
      cpu: "100m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "2Gi"

status:
  phase: "Running" # Pending, Running, Error, Stopped
  conditions:
    - type: "L1Connected"
      status: "True"
      reason: "ConnectionEstablished"
      message: "Connected to L1 RPC endpoint"
    - type: "L2Connected"
      status: "True"
      reason: "SequencerReachable"
      message: "Connected to L2 sequencer"
    - type: "PrivateKeyLoaded"
      status: "True"
      reason: "SecretFound"
      message: "Private key loaded from secret"

  batcherInfo:
    lastBatchSubmitted:
      blockNumber: 12345678
      transactionHash: "0xdef456..."
      timestamp: "2024-01-15T10:25:00Z"
      gasUsed: 21000

    pendingBatches: 2
    totalBatchesSubmitted: 5432

  observedGeneration: 1
```

#### Controller Responsibilities

- Create and manage Deployment for op-batcher instances
- Validate private key secret exists and is properly formatted
- Configure L1 and L2 RPC connections
- Monitor batch submission status and L1 transaction confirmations
- Handle fee management and transaction resubmission logic
- Ensure high availability during configuration updates

---

### 4. OpProposer CRD

**Purpose**: Manages op-proposer instances that submit L2 output root proposals to L1.

#### Spec Schema

```yaml
apiVersion: optimism.io/v1alpha1
kind: OpProposer
metadata:
  name: op-mainnet-proposer
  namespace: optimism-system
spec:
  # Network Reference
  optimismNetworkRef:
    name: "op-mainnet"
    namespace: "optimism-system"

  # L2 Output Oracle Configuration (address auto-discovered from OptimismNetwork)
  l2OutputOracleAddr: "" # Leave empty - populated from network.status.discoveredContracts

  # Private Key for L1 Transaction Signing
  privateKey:
    secretRef:
      name: "proposer-private-key"
      key: "private-key"

  # Proposal Configuration
  proposal:
    pollInterval: "12s" # Interval between output root proposals
    allowNonFinalized: false # Allow proposing non-finalized L2 state (testnets only)
    outputInterval: "1800s" # How often outputs are proposed (30 minutes)

  # Dispute Game Configuration (for Fault Proof chains - addresses auto-discovered)
  disputeGame:
    factoryAddr: "" # Auto-discovered from OptimismNetwork
    gameType: "0" # Fault proof game type

  # L1 Transaction Management
  l1Transaction:
    feeLimitMultiplier: "5"
    resubmissionTimeout: "48s"
    numConfirmations: "5"
    safeAbortNonceTooLowCount: "3"

  # RPC Configuration
  rpc:
    enabled: true
    host: "127.0.0.1"
    port: 8560
    enableAdmin: true

  # Metrics Configuration
  metrics:
    enabled: true
    host: "0.0.0.0"
    port: 7300

  # Resources
  resources:
    requests:
      cpu: "100m"
      memory: "256Mi"
    limits:
      cpu: "500m"
      memory: "1Gi"

status:
  phase: "Running"
  conditions:
    - type: "L1Connected"
      status: "True"
      reason: "OracleContractReachable"
      message: "Connected to L2OutputOracle contract"
    - type: "L2Connected"
      status: "True"
      reason: "OutputRootAccessible"
      message: "Can fetch L2 output roots"
    - type: "PrivateKeyLoaded"
      status: "True"
      reason: "SecretFound"
      message: "Private key loaded from secret"

  proposerInfo:
    lastProposalSubmitted:
      outputRoot: "0xabc123..."
      l2BlockNumber: 12345678
      transactionHash: "0xdef456..."
      timestamp: "2024-01-15T10:20:00Z"

    totalProposalsSubmitted: 1234
    nextProposalDue: "2024-01-15T10:50:00Z"

  observedGeneration: 1
```

#### Controller Responsibilities

- Create and manage Deployment for op-proposer instances
- Validate L2OutputOracle contract accessibility
- Configure proposal timing and dispute game parameters
- Monitor proposal submission status and handle resubmissions
- Manage private key rotation and security
- Handle upgrades from output oracle to dispute game factory

---

### 5. OpChallenger CRD

**Purpose**: Manages op-challenger instances that monitor and participate in dispute games.

#### Spec Schema

```yaml
apiVersion: optimism.io/v1alpha1
kind: OpChallenger
metadata:
  name: op-mainnet-challenger
  namespace: optimism-system
spec:
  # Network Reference
  optimismNetworkRef:
    name: "op-mainnet"
    namespace: "optimism-system"

  # Private Key for L1 Transaction Signing
  privateKey:
    secretRef:
      name: "challenger-private-key"
      key: "private-key"

  # Dispute Game Configuration (addresses auto-discovered from OptimismNetwork)
  disputeGame:
    factoryAddr: "" # Auto-discovered from OptimismNetwork
    gameAllowlist: [] # Empty = monitor all games

  # Fault Proof Configuration
  faultProof:
    traceType: "cannon" # cannon, alphabet (for testing)

    # Cannon-specific Configuration
    cannon:
      server: "http://cannon-server:8080"
      prestate: "0xdeadbeef..."
      rollupConfigPath: "/config/rollup.json"
      l2GenesisPath: "/config/genesis.json"

  # Data Directory (for persistent challenger state)
  dataDir: "/data/challenger"
  storage:
    size: "100Gi"
    storageClass: "standard"
    accessMode: "ReadWriteOnce"

  # Monitoring Configuration
  monitoring:
    interval: "1m" # How often to check for new games
    numConfirmations: "5" # L1 confirmations before acting
    maxGames: "100" # Maximum concurrent games to monitor

  # RPC Configuration
  rpc:
    enabled: true
    host: "127.0.0.1"
    port: 8545

  # Metrics Configuration
  metrics:
    enabled: true
    host: "0.0.0.0"
    port: 7300

  # Resources
  resources:
    requests:
      cpu: "200m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "4Gi"

status:
  phase: "Running"
  conditions:
    - type: "L1Connected"
      status: "True"
      reason: "DisputeGameFactoryReachable"
      message: "Connected to DisputeGameFactory contract"
    - type: "PrivateKeyLoaded"
      status: "True"
      reason: "SecretFound"
      message: "Private key loaded from secret"
    - type: "MonitoringActive"
      status: "True"
      reason: "GamesBeingMonitored"
      message: "Monitoring 3 active dispute games"

  challengerInfo:
    activeGames: 3
    totalChallengesMade: 15
    totalGamesResolved: 42

    lastChallenge:
      gameAddr: "0x4567890123456789012345678901234567890123"
      transactionHash: "0xghi789..."
      timestamp: "2024-01-15T10:15:00Z"

  observedGeneration: 1
```

#### Controller Responsibilities

- Create and manage StatefulSet for op-challenger (needs persistent storage)
- Generate and manage persistent volumes for challenger data
- Configure fault proof system (Cannon) integration
- Monitor dispute game factory for new games
- Handle dynamic Job creation for op-program execution during disputes
- Manage challenger state persistence and recovery

## Controller Implementation Architecture

### 1. Controller Structure

Each CRD has its own dedicated controller following the Kubebuilder pattern:

```go
// controllers/
├── optimismnetwork_controller.go
├── opnode_controller.go
├── opbatcher_controller.go
├── opproposer_controller.go
├── opchallenger_controller.go
└── common/
    ├── config.go          // Shared configuration utilities
    ├── secrets.go         // Secret management utilities
    ├── resources.go       // Resource creation utilities
    └── status.go          // Status update utilities
```

### 2. Reconciliation Logic

#### Common Reconciliation Pattern

```go
const (
    OpNodeFinalizer      = "opnode.optimism.io/finalizer"
    OpBatcherFinalizer   = "opbatcher.optimism.io/finalizer"
    OpProposerFinalizer  = "opproposer.optimism.io/finalizer"
    OpChallengerFinalizer = "opchallenger.optimism.io/finalizer"
)

func (r *OpNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the resource
    var opNode optimismv1alpha1.OpNode
    if err := r.Get(ctx, req.NamespacedName, &opNode); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. Handle deletion with finalizers
    if opNode.DeletionTimestamp != nil {
        return r.handleDeletion(ctx, &opNode)
    }

    // 3. Add finalizer if not present
    if !controllerutil.ContainsFinalizer(&opNode, OpNodeFinalizer) {
        controllerutil.AddFinalizer(&opNode, OpNodeFinalizer)
        return ctrl.Result{}, r.Update(ctx, &opNode)
    }

    // 4. Fetch referenced OptimismNetwork
    network, err := r.fetchOptimismNetwork(ctx, &opNode)
    if err != nil {
        return ctrl.Result{}, err
    }

    // 5. Generate configuration
    config, err := r.generateConfiguration(&opNode, network)
    if err != nil {
        return ctrl.Result{}, err
    }

    // 6. Manage secrets (JWT, P2P keys)
    if err := r.reconcileSecrets(ctx, &opNode, config); err != nil {
        return ctrl.Result{}, err
    }

    // 7. Manage persistent volumes
    if err := r.reconcilePersistentVolumes(ctx, &opNode); err != nil {
        return ctrl.Result{}, err
    }

    // 8. Manage ConfigMaps
    if err := r.reconcileConfigMaps(ctx, &opNode, config); err != nil {
        return ctrl.Result{}, err
    }

    // 9. Manage workloads (StatefulSet/Deployment)
    if err := r.reconcileWorkloads(ctx, &opNode, config); err != nil {
        return ctrl.Result{}, err
    }

    // 10. Manage services
    if err := r.reconcileServices(ctx, &opNode); err != nil {
        return ctrl.Result{}, err
    }

    // 11. Update status
    return r.updateStatus(ctx, &opNode)
}
```

### 5. Configuration Management

#### Contract Address Discovery and Configuration Generation

The operator uses a multi-tiered approach to discover contract addresses and generate configurations:

```go
type ContractDiscoveryService struct {
    l1Client     *ethclient.Client
    l2Client     *ethclient.Client
    cache        map[string]*NetworkContractAddresses
    cacheTimeout time.Duration
}

func (c *ContractDiscoveryService) DiscoverContracts(ctx context.Context, network *OptimismNetwork) (*NetworkContractAddresses, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("%s-%d", network.Spec.NetworkName, network.Spec.ChainID)
    if cached, exists := c.cache[cacheKey]; exists && !c.isCacheExpired(cached) {
        return cached, nil
    }

    var addresses *NetworkContractAddresses
    var err error

    switch network.Spec.ContractAddresses.DiscoveryMethod {
    case "auto":
        addresses, err = c.autoDiscoverContracts(ctx, network)
    case "superchain-registry":
        addresses, err = c.discoverFromSuperchainRegistry(network.Spec.ChainID)
    case "well-known":
        addresses = c.getWellKnownAddresses(network.Spec.NetworkName, network.Spec.ChainID)
    case "manual":
        addresses = &network.Spec.ContractAddresses.NetworkContractAddresses
    default:
        return nil, fmt.Errorf("unknown discovery method: %s", network.Spec.ContractAddresses.DiscoveryMethod)
    }

    if err != nil {
        return nil, fmt.Errorf("failed to discover contracts: %w", err)
    }

    // Cache the result
    addresses.LastDiscoveryTime = time.Now()
    c.cache[cacheKey] = addresses

    return addresses, nil
}

func (c *ContractDiscoveryService) autoDiscoverContracts(ctx context.Context, network *OptimismNetwork) (*NetworkContractAddresses, error) {
    addresses := &NetworkContractAddresses{}

    // Strategy 1: Query SystemConfig contract
    if network.Spec.ContractAddresses.SystemConfigAddr != "" {
        systemConfig, err := c.querySystemConfig(ctx, network.Spec.ContractAddresses.SystemConfigAddr)
        if err == nil {
            addresses.L2OutputOracleAddr = systemConfig.L2OutputOracle().Hex()
            addresses.DisputeGameFactoryAddr = systemConfig.DisputeGameFactory().Hex()
            addresses.OptimismPortalAddr = systemConfig.OptimismPortal().Hex()
            addresses.DiscoveryMethod = "system-config"
            return addresses, nil
        }
    }

    // Strategy 2: Query L2 predeploys (always at known addresses)
    if c.l2Client != nil {
        l2Addresses, err := c.queryL2Predeploys(ctx)
        if err == nil {
            addresses.L2CrossDomainMessengerAddr = l2Addresses.L2CrossDomainMessengerAddr
            addresses.L2StandardBridgeAddr = l2Addresses.L2StandardBridgeAddr
            addresses.L2ToL1MessagePasserAddr = l2Addresses.L2ToL1MessagePasserAddr
        }
    }

    // Strategy 3: Query Superchain Registry as fallback
    registryAddresses, err := c.discoverFromSuperchainRegistry(network.Spec.ChainID)
    if err == nil {
        // Merge any missing addresses from registry
        c.mergeAddresses(addresses, registryAddresses)
        addresses.DiscoveryMethod = "superchain-registry"
        return addresses, nil
    }

    // Strategy 4: Fall back to well-known addresses
    wellKnownAddresses := c.getWellKnownAddresses(network.Spec.NetworkName, network.Spec.ChainID)
    if wellKnownAddresses != nil {
        c.mergeAddresses(addresses, wellKnownAddresses)
        addresses.DiscoveryMethod = "well-known"
        return addresses, nil
    }

    return nil, fmt.Errorf("unable to discover contract addresses for network %s (chain ID: %d)",
        network.Spec.NetworkName, network.Spec.ChainID)
}

// Query L2 predeploy contracts (always at fixed addresses)
func (c *ContractDiscoveryService) queryL2Predeploys(ctx context.Context) (*NetworkContractAddresses, error) {
    addresses := &NetworkContractAddresses{}

    // L2 predeploy addresses are standardized across all OP Stack chains
    const (
        L2CrossDomainMessengerAddr = "0x4200000000000000000000000000000000000007"
        L2StandardBridgeAddr       = "0x4200000000000000000000000000000000000010"
        L2ToL1MessagePasserAddr    = "0x4200000000000000000000000000000000000016"
    )

    // Verify these contracts exist on the L2
    for addr, name := range map[string]string{
        L2CrossDomainMessengerAddr: "L2CrossDomainMessenger",
        L2StandardBridgeAddr:       "L2StandardBridge",
        L2ToL1MessagePasserAddr:    "L2ToL1MessagePasser",
    } {
        code, err := c.l2Client.CodeAt(ctx, common.HexToAddress(addr), nil)
        if err != nil || len(code) == 0 {
            return nil, fmt.Errorf("predeploy contract %s not found at %s", name, addr)
        }
    }

    addresses.L2CrossDomainMessengerAddr = L2CrossDomainMessengerAddr
    addresses.L2StandardBridgeAddr = L2StandardBridgeAddr
    addresses.L2ToL1MessagePasserAddr = L2ToL1MessagePasserAddr

    return addresses, nil
}
```

#### Configuration Inheritance Pattern

```go
type ComponentConfig struct {
    // Inherited from OptimismNetwork
    L1RpcUrl      string
    L1BeaconUrl   string
    NetworkName   string
    ChainID       int64

    // Component-specific
    ComponentSpec interface{}

    // Computed values
    JWTSecret     string
    ConfigMaps    map[string]string
    ServiceRefs   map[string]string  // Computed service references
}

func (r *OpBatcherReconciler) generateConfiguration(opBatcher *OpBatcher, network *OptimismNetwork) (*ComponentConfig, error) {
    config := &ComponentConfig{
        L1RpcUrl:    network.Spec.L1RpcUrl,
        L1BeaconUrl: network.Spec.L1BeaconUrl,
        NetworkName: network.Spec.NetworkName,
        ChainID:     network.Spec.ChainID,
    }

    // Merge component-specific configuration
    config.ComponentSpec = opBatcher.Spec

    // Resolve service references
    if opBatcher.Spec.SequencerRef != nil {
        serviceName := r.computeServiceName(opBatcher.Spec.SequencerRef)
        config.ServiceRefs = map[string]string{
            "sequencer": fmt.Sprintf("http://%s:8545", serviceName),
        }
    }

    // Generate derived configuration
    config.JWTSecret = r.generateOrGetJWTSecret()
    config.ConfigMaps = r.generateConfigMaps(opBatcher, network)

    return config, nil
}
```

### 6. Workload Management

#### Container Co-location Strategy

**Design Decision**: op-node and op-geth run in the same pod for simplified networking and shared volume access. This enables:

- Direct localhost communication for Engine API (no network latency)
- Shared JWT secret via mounted volume
- Simplified service discovery
- Atomic pod lifecycle management

#### StatefulSet for Stateful Components (op-geth, op-challenger)

```go
func (r *OpNodeReconciler) createStatefulSet(opNode *OpNode, config *ComponentConfig) *appsv1.StatefulSet {
    return &appsv1.StatefulSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:      opNode.Name + "-geth",
            Namespace: opNode.Namespace,
        },
        Spec: appsv1.StatefulSetSpec{
            Replicas: int32Ptr(1),
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "app":       "op-geth",
                    "instance":  opNode.Name,
                },
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "app":       "op-geth",
                        "instance":  opNode.Name,
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        r.createOpGethContainer(opNode, config),
                        r.createOpNodeContainer(opNode, config),
                    },
                    Volumes: r.createVolumes(opNode, config),
                },
            },
            VolumeClaimTemplates: r.createVolumeClaimTemplates(opNode),
        },
    }
}
```

#### Deployment for Stateless Components (op-batcher, op-proposer)

```go
func (r *OpBatcherReconciler) createDeployment(opBatcher *OpBatcher, config *ComponentConfig) *appsv1.Deployment {
    return &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      opBatcher.Name,
            Namespace: opBatcher.Namespace,
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: int32Ptr(1),
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "app":      "op-batcher",
                    "instance": opBatcher.Name,
                },
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "app":      "op-batcher",
                        "instance": opBatcher.Name,
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        r.createOpBatcherContainer(opBatcher, config),
                    },
                },
            },
        },
    }
}
```

## Security Considerations

### 1. Secret Management

- **JWT Tokens**: Auto-generated 256-bit hex secrets for engine API communication
- **Private Keys**: Store in Kubernetes Secrets with proper RBAC
- **P2P Keys**: Auto-generated Ed25519 keys for node identity
- **Encryption**: All secrets encrypted at rest via Kubernetes

### 2. Network Security

- **Sequencer Isolation**: Disable P2P discovery, use static peer lists
- **Internal Communication**: Use ClusterIP services by default
- **RPC Security**: Admin endpoints restricted to localhost by default

### 3. Pod Security

- **Security Context**: Run as non-root user (uid 1000)
- **Seccomp**: Runtime default seccomp profile
- **Capabilities**: Drop all, add only necessary capabilities
- **Read-only Root**: Where possible, use read-only root filesystems

## Monitoring and Observability

### 1. Metrics Exposure

All components expose Prometheus-compatible metrics on `/metrics` endpoint:

- **op-node**: Chain head, sync status, peer count, RPC metrics
- **op-geth**: Block processing, transaction pool, P2P metrics
- **op-batcher**: Batch submission rate, L1 gas usage, queue depth
- **op-proposer**: Proposal frequency, L1 transaction status
- **op-challenger**: Active games, challenge success rate

### 2. Health Checks

Kubernetes-native health checks via HTTP endpoints:

- **Liveness Probe**: Component is running and responsive
- **Readiness Probe**: Component is ready to serve traffic
- **Startup Probe**: Component has completed initialization

### 3. Status Reporting

Rich status information in CRD status fields:

- **Phase**: High-level component state (Pending, Running, Error)
- **Conditions**: Detailed condition status with reasons and messages
- **Operational Metrics**: Block numbers, sync status, peer counts

## Future Enhancements

### Phase 2: Advanced Features

1. **Superchain Registry Integration**

   - Automatic network configuration discovery
   - Standardized chain parameter management
   - Cross-chain configuration validation

2. **High Availability**

   - Multi-replica sequencer setups with op-conductor
   - Leader election for batcher/proposer components
   - Automatic failover and recovery

3. **Advanced Networking**
   - Service mesh integration (Istio, Linkerd)
   - Ingress controller integration
   - Load balancer configuration for RPC endpoints

### Phase 3: Operational Excellence

1. **Backup and Recovery**

   - Automated chain data snapshots
   - Point-in-time recovery mechanisms
   - Cross-cluster backup replication

2. **Auto-scaling**

   - Horizontal pod autoscaling for replica nodes
   - Vertical pod autoscaling based on chain growth
   - Dynamic resource allocation

3. **Interop Support**
   - Cross-chain communication management
   - Multi-chain sequencer coordination
   - Dependency tracking between chains

### Phase 4: Ecosystem Integration

1. **External Service Integration**

   - proxyd for RPC load balancing
   - Blob archiver for data availability
   - Chain monitoring tools (Monitorism)

2. **Alternative Execution Clients**

   - Support for Reth execution client
   - Support for Erigon execution client
   - Client switching and migration tools

3. **Developer Experience**
   - Helm charts for easy deployment
   - CLI tools for operator management
   - Integration with existing DevOps workflows

## Implementation Roadmap

### Milestone 1: Core CRDs and Controllers (8-10 weeks)

- [ ] OptimismNetwork CRD and controller
- [ ] OpNode CRD and controller (sequencer + replica)
- [ ] Basic secret and configuration management
- [ ] Unit tests and integration tests

### Milestone 2: Chain Operations (6-8 weeks)

- [ ] OpBatcher CRD and controller
- [ ] OpProposer CRD and controller
- [ ] OpChallenger CRD and controller
- [ ] End-to-end testing with local devnet

### Milestone 3: Production Readiness (4-6 weeks)

- [ ] Security hardening and RBAC
- [ ] Comprehensive monitoring and alerting
- [ ] Documentation and examples
- [ ] Performance testing and optimization

### Milestone 4: Advanced Features (8-12 weeks)

- [ ] Superchain registry integration
- [ ] High availability features
- [ ] Backup and recovery mechanisms

## Conclusion

This OP Stack Kubernetes operator provides a comprehensive solution for managing both public node operations and chain operations in a Kubernetes environment. The design emphasizes security, operational simplicity, and Kubernetes-native patterns while providing a solid foundation for future enhancements.

The operator enables users to:

- Deploy complete OP Stack chains with minimal configuration
- Manage both sequencer and replica node deployments
- Handle chain operation services (batcher, proposer, challenger)
- Maintain proper security and isolation
- Monitor and observe system health
- Upgrade and maintain deployments safely

This specification provides the foundation for building a production-ready operator that can scale from single-node test deployments to multi-chain production environments.
