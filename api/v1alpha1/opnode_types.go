/*
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
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpNodeSpec defines the desired state of OpNode
type OpNodeSpec struct {
	// OptimismNetworkRef references the OptimismNetwork for this node
	OptimismNetworkRef OptimismNetworkRef `json:"optimismNetworkRef"`

	// NodeType specifies whether this is a sequencer or replica node
	// +kubebuilder:validation:Enum=sequencer;replica
	NodeType string `json:"nodeType"`

	// SequencerRef references the sequencer OpNode (only for replica nodes)
	// This field is optional when L2RpcUrl is set for external sequencer connections
	SequencerRef *SequencerReference `json:"sequencerRef,omitempty"`

	// L2RpcUrl is the external L2 RPC URL for connecting to an external sequencer
	// This is typically used for replica nodes connecting to external networks (e.g., Sepolia)
	// When set, SequencerRef is optional
	L2RpcUrl string `json:"l2RpcUrl,omitempty"`

	// OpNode configuration
	OpNode OpNodeConfig `json:"opNode,omitempty"`

	// OpGeth configuration
	OpGeth OpGethConfig `json:"opGeth,omitempty"`

	// Resources defines resource requirements for the components
	Resources *OpNodeResources `json:"resources,omitempty"`

	// Service configuration
	Service *ServiceConfig `json:"service,omitempty"`
}

// OptimismNetworkRef references an OptimismNetwork resource
type OptimismNetworkRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// SequencerReference defines a reference to the sequencer OpNode
type SequencerReference struct {
	// Name of the sequencer OpNode
	Name string `json:"name"`
	// Namespace of the sequencer OpNode (optional, defaults to same namespace)
	Namespace string `json:"namespace,omitempty"`
}

// OpNodeConfig defines op-node specific configuration
type OpNodeConfig struct {
	// Sync configuration
	SyncMode string `json:"syncMode,omitempty"` // execution-layer, consensus-layer

	// P2P configuration
	P2P *P2PConfig `json:"p2p,omitempty"`

	// RPC configuration
	RPC *RPCConfig `json:"rpc,omitempty"`

	// Sequencer-specific configuration
	Sequencer *SequencerConfig `json:"sequencer,omitempty"`

	// Engine API configuration (communication with op-geth)
	Engine *EngineConfig `json:"engine,omitempty"`
}

// P2PConfig defines P2P networking configuration
type P2PConfig struct {
	Enabled    bool  `json:"enabled,omitempty"`
	ListenPort int32 `json:"listenPort,omitempty"`

	// Discovery configuration
	Discovery *P2PDiscoveryConfig `json:"discovery,omitempty"`

	// Static peer configuration
	Static []string `json:"static,omitempty"`

	// Peer scoring
	PeerScoring *P2PScoringConfig `json:"peerScoring,omitempty"`

	// Bandwidth limit
	BandwidthLimit string `json:"bandwidthLimit,omitempty"`

	// P2P private key management
	PrivateKey *SecretKeyRef `json:"privateKey,omitempty"`
}

// P2PDiscoveryConfig defines P2P discovery settings
type P2PDiscoveryConfig struct {
	Enabled   bool     `json:"enabled,omitempty"`
	Bootnodes []string `json:"bootnodes,omitempty"`
}

// P2PScoringConfig defines P2P peer scoring settings
type P2PScoringConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

// RPCConfig defines RPC server configuration
type RPCConfig struct {
	Enabled     bool        `json:"enabled,omitempty"`
	Host        string      `json:"host,omitempty"`
	Port        int32       `json:"port,omitempty"`
	EnableAdmin bool        `json:"enableAdmin,omitempty"`
	CORS        *CORSConfig `json:"cors,omitempty"`
}

// CORSConfig defines CORS settings
type CORSConfig struct {
	Origins []string `json:"origins,omitempty"`
	Methods []string `json:"methods,omitempty"`
}

// SequencerConfig defines sequencer-specific settings
type SequencerConfig struct {
	Enabled       bool   `json:"enabled,omitempty"`
	BlockTime     string `json:"blockTime,omitempty"`
	MaxTxPerBlock int32  `json:"maxTxPerBlock,omitempty"`
}

// EngineConfig defines Engine API configuration
type EngineConfig struct {
	JWTSecret *SecretKeyRef `json:"jwtSecret,omitempty"`
	Endpoint  string        `json:"endpoint,omitempty"`
}

// SecretKeyRef references a secret for key material
type SecretKeyRef struct {
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
	Generate  bool                      `json:"generate,omitempty"`
}

// OpGethConfig defines op-geth specific configuration
type OpGethConfig struct {
	// Network must match OptimismNetwork
	Network string `json:"network,omitempty"`

	// Data directory and storage
	DataDir string         `json:"dataDir,omitempty"`
	Storage *StorageConfig `json:"storage,omitempty"`

	// Sync configuration
	SyncMode    string `json:"syncMode,omitempty"`    // snap, full
	GCMode      string `json:"gcMode,omitempty"`      // full, archive
	StateScheme string `json:"stateScheme,omitempty"` // path, hash

	// Database configuration
	Cache    int32  `json:"cache,omitempty"`    // Cache size in MB
	DBEngine string `json:"dbEngine,omitempty"` // pebble, leveldb

	// Networking configuration
	Networking *GethNetworkingConfig `json:"networking,omitempty"`

	// Transaction pool configuration
	TxPool *TxPoolConfig `json:"txpool,omitempty"`

	// Rollup-specific configuration
	Rollup *RollupConfig `json:"rollup,omitempty"`
}

// StorageConfig defines persistent storage settings
type StorageConfig struct {
	Size         resource.Quantity `json:"size,omitempty"`
	StorageClass string            `json:"storageClass,omitempty"`
	AccessMode   string            `json:"accessMode,omitempty"`
}

// GethNetworkingConfig defines geth networking settings
type GethNetworkingConfig struct {
	HTTP    *HTTPConfig    `json:"http,omitempty"`
	WS      *WSConfig      `json:"ws,omitempty"`
	AuthRPC *AuthRPCConfig `json:"authrpc,omitempty"`
	P2P     *GethP2PConfig `json:"p2p,omitempty"`
}

// HTTPConfig defines HTTP RPC settings
type HTTPConfig struct {
	Enabled bool        `json:"enabled,omitempty"`
	Host    string      `json:"host,omitempty"`
	Port    int32       `json:"port,omitempty"`
	APIs    []string    `json:"apis,omitempty"`
	CORS    *CORSConfig `json:"cors,omitempty"`
}

// WSConfig defines WebSocket RPC settings
type WSConfig struct {
	Enabled bool     `json:"enabled,omitempty"`
	Host    string   `json:"host,omitempty"`
	Port    int32    `json:"port,omitempty"`
	APIs    []string `json:"apis,omitempty"`
	Origins []string `json:"origins,omitempty"`
}

// AuthRPCConfig defines authenticated RPC settings
type AuthRPCConfig struct {
	Host string   `json:"host,omitempty"`
	Port int32    `json:"port,omitempty"`
	APIs []string `json:"apis,omitempty"`
}

// GethP2PConfig defines geth P2P settings
type GethP2PConfig struct {
	Port        int32    `json:"port,omitempty"`
	MaxPeers    int32    `json:"maxPeers,omitempty"`
	NoDiscovery bool     `json:"noDiscovery,omitempty"`
	NetRestrict string   `json:"netRestrict,omitempty"`
	Static      []string `json:"static,omitempty"`
}

// TxPoolConfig defines transaction pool settings
type TxPoolConfig struct {
	Locals         []string `json:"locals,omitempty"`
	NoLocals       bool     `json:"noLocals,omitempty"`
	Journal        string   `json:"journal,omitempty"`
	JournalRemotes bool     `json:"journalRemotes,omitempty"`
	Lifetime       string   `json:"lifetime,omitempty"`
	PriceBump      int32    `json:"priceBump,omitempty"`

	// Pool limits
	AccountSlots int32 `json:"accountSlots,omitempty"`
	GlobalSlots  int32 `json:"globalSlots,omitempty"`
	AccountQueue int32 `json:"accountQueue,omitempty"`
	GlobalQueue  int32 `json:"globalQueue,omitempty"`
}

// RollupConfig defines rollup-specific settings
type RollupConfig struct {
	DisableTxPoolGossip bool `json:"disableTxPoolGossip,omitempty"`
	ComputePendingBlock bool `json:"computePendingBlock,omitempty"`
}

// OpNodeResources defines resource requirements for OpNode components
type OpNodeResources struct {
	OpNode *corev1.ResourceRequirements `json:"opNode,omitempty"`
	OpGeth *corev1.ResourceRequirements `json:"opGeth,omitempty"`
}

// ServiceConfig defines Kubernetes service configuration
type ServiceConfig struct {
	Type        corev1.ServiceType  `json:"type,omitempty"`
	Annotations map[string]string   `json:"annotations,omitempty"`
	Ports       []ServicePortConfig `json:"ports,omitempty"`
}

// ServicePortConfig defines a service port
type ServicePortConfig struct {
	Name       string             `json:"name"`
	Port       int32              `json:"port"`
	TargetPort intstr.IntOrString `json:"targetPort,omitempty"`
	Protocol   corev1.Protocol    `json:"protocol,omitempty"`
}

// OpNodeStatus defines the observed state of OpNode
type OpNodeStatus struct {
	// Phase represents the overall state of the OpNode
	Phase string `json:"phase,omitempty"` // Pending, Initializing, Running, Error, Stopped

	// Conditions represent detailed status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// NodeInfo contains operational information about the node
	NodeInfo *NodeInfo `json:"nodeInfo,omitempty"`
}

// NodeInfo contains operational information about the running node
type NodeInfo struct {
	// Chain head information
	ChainHead *ChainHeadInfo `json:"chainHead,omitempty"`

	// Sync status
	SyncStatus *SyncStatusInfo `json:"syncStatus,omitempty"`

	// P2P information
	PeerCount int32 `json:"peerCount,omitempty"`

	// Engine API connectivity
	EngineConnected bool `json:"engineConnected,omitempty"`
}

// ChainHeadInfo contains information about the current chain head
type ChainHeadInfo struct {
	BlockNumber int64       `json:"blockNumber,omitempty"`
	BlockHash   string      `json:"blockHash,omitempty"`
	Timestamp   metav1.Time `json:"timestamp,omitempty"`
}

// SyncStatusInfo contains sync status information
type SyncStatusInfo struct {
	CurrentBlock int64 `json:"currentBlock,omitempty"`
	HighestBlock int64 `json:"highestBlock,omitempty"`
	Syncing      bool  `json:"syncing,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.nodeType`
// +kubebuilder:printcolumn:name="Network",type=string,JSONPath=`.spec.optimismNetworkRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Peers",type=integer,JSONPath=`.status.nodeInfo.peerCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpNode is the Schema for the opnodes API
type OpNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpNodeSpec   `json:"spec,omitempty"`
	Status OpNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpNodeList contains a list of OpNode
type OpNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpNode{}, &OpNodeList{})
}
