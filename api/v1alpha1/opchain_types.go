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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OPChainSpec defines the desired state of OPChain.
type OPChainSpec struct {
	// ChainID is the unique identifier for this L2 chain
	ChainID int64 `json:"chainId"`

	// NetworkName is a human-readable name for the network
	NetworkName string `json:"networkName"`

	// L1 configuration for the parent L1 chain
	L1 L1Config `json:"l1"`

	// Components configuration for OP Stack services
	Components ComponentsConfig `json:"components"`
}

// L1Config defines the L1 chain configuration
type L1Config struct {
	// RpcUrl is the HTTP RPC endpoint for the L1 chain
	RpcUrl string `json:"rpcUrl"`

	// ChainID of the L1 chain (e.g. 1 for Ethereum mainnet, 11155111 for Sepolia)
	ChainID int64 `json:"chainId"`

	// DeployerPrivateKeySecret is the name of the Kubernetes secret containing the L1 deployer private key
	DeployerPrivateKeySecret string `json:"deployerPrivateKeySecret"`

	// ContractAddresses contains pre-deployed L1 contract addresses (optional)
	ContractAddresses *ContractAddresses `json:"contractAddresses,omitempty"`
}

// ComponentsConfig defines the configuration for all OP Stack components
type ComponentsConfig struct {
	// Geth configuration for op-geth (execution client)
	Geth GethConfig `json:"geth"`

	// Node configuration for op-node (rollup coordination)
	Node NodeConfig `json:"node"`

	// Batcher configuration for op-batcher (L2->L1 data submission)
	Batcher BatcherConfig `json:"batcher"`

	// Proposer configuration for op-proposer (L2 output root submission)
	Proposer ProposerConfig `json:"proposer"`
}

// GethConfig defines the configuration for op-geth
type GethConfig struct {
	// Enabled indicates whether op-geth should be deployed
	Enabled bool `json:"enabled"`

	// Image is the container image for op-geth
	Image string `json:"image,omitempty"`

	// Resources defines CPU and memory resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage configuration for persistent data
	Storage StorageConfig `json:"storage,omitempty"`
}

// NodeConfig defines the configuration for op-node
type NodeConfig struct {
	// Enabled indicates whether op-node should be deployed
	Enabled bool `json:"enabled"`

	// Image is the container image for op-node
	Image string `json:"image,omitempty"`

	// Resources defines CPU and memory resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// BatcherConfig defines the configuration for op-batcher
type BatcherConfig struct {
	// Enabled indicates whether op-batcher should be deployed
	Enabled bool `json:"enabled"`

	// Image is the container image for op-batcher
	Image string `json:"image,omitempty"`

	// SignerPrivateKeySecret is the name of the Kubernetes secret containing the batcher's L1 signing key
	SignerPrivateKeySecret string `json:"signerPrivateKeySecret,omitempty"`

	// Resources defines CPU and memory resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ProposerConfig defines the configuration for op-proposer
type ProposerConfig struct {
	// Enabled indicates whether op-proposer should be deployed
	Enabled bool `json:"enabled"`

	// Image is the container image for op-proposer
	Image string `json:"image,omitempty"`

	// SignerPrivateKeySecret is the name of the Kubernetes secret containing the proposer's L1 signing key
	SignerPrivateKeySecret string `json:"signerPrivateKeySecret,omitempty"`

	// Resources defines CPU and memory resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// StorageConfig defines storage configuration
type StorageConfig struct {
	// Size is the requested storage size (e.g. "100Gi")
	Size string `json:"size,omitempty"`

	// StorageClass is the Kubernetes storage class to use
	StorageClass string `json:"storageClass,omitempty"`
}

// ContractAddresses contains the deployed L1 contract addresses
type ContractAddresses struct {
	// OptimismPortal is the address of the OptimismPortal contract
	OptimismPortal string `json:"optimismPortal,omitempty"`

	// L2OutputOracle is the address of the L2OutputOracle contract
	L2OutputOracle string `json:"l2OutputOracle,omitempty"`

	// SystemConfig is the address of the SystemConfig contract
	SystemConfig string `json:"systemConfig,omitempty"`

	// L1CrossDomainMessenger is the address of the L1CrossDomainMessenger contract
	L1CrossDomainMessenger string `json:"l1CrossDomainMessenger,omitempty"`

	// L1StandardBridge is the address of the L1StandardBridge contract
	L1StandardBridge string `json:"l1StandardBridge,omitempty"`

	// DisputeGameFactory is the address of the DisputeGameFactory contract
	DisputeGameFactory string `json:"disputeGameFactory,omitempty"`

	// AddressManager is the address of the AddressManager contract
	AddressManager string `json:"addressManager,omitempty"`
}

// OPChainStatus defines the observed state of OPChain.
type OPChainStatus struct {
	// Phase represents the current deployment phase
	Phase OPChainPhase `json:"phase,omitempty"`

	// Conditions represent the current state conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ContractAddresses contains the deployed L1 contract addresses
	ContractAddresses *ContractAddresses `json:"contractAddresses,omitempty"`

	// ComponentStatus contains status information for each component
	ComponentStatus *ComponentStatus `json:"componentStatus,omitempty"`

	// ObservedGeneration reflects the generation most recently observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// OPChainPhase represents the current phase of the OPChain deployment
type OPChainPhase string

const (
	// OPChainPhasePending indicates the deployment is waiting to start
	OPChainPhasePending OPChainPhase = "Pending"

	// OPChainPhaseDeploying indicates the deployment is in progress
	OPChainPhaseDeploying OPChainPhase = "Deploying"

	// OPChainPhaseRunning indicates all components are deployed and running
	OPChainPhaseRunning OPChainPhase = "Running"

	// OPChainPhaseError indicates an error occurred during deployment
	OPChainPhaseError OPChainPhase = "Error"

	// OPChainPhaseUpgrading indicates the deployment is being upgraded
	OPChainPhaseUpgrading OPChainPhase = "Upgrading"
)

// ComponentStatus contains status information for each OP Stack component
type ComponentStatus struct {
	// Geth status
	Geth *GethStatus `json:"geth,omitempty"`

	// Node status
	Node *NodeStatus `json:"node,omitempty"`

	// Batcher status
	Batcher *BatcherStatus `json:"batcher,omitempty"`

	// Proposer status
	Proposer *ProposerStatus `json:"proposer,omitempty"`
}

// GethStatus represents the status of the op-geth component
type GethStatus struct {
	// Ready indicates if the component is ready
	Ready bool `json:"ready"`

	// SyncHeight is the current sync height
	SyncHeight int64 `json:"syncHeight,omitempty"`

	// Message contains additional status information
	Message string `json:"message,omitempty"`
}

// NodeStatus represents the status of the op-node component
type NodeStatus struct {
	// Ready indicates if the component is ready
	Ready bool `json:"ready"`

	// SafeHeight is the current safe L2 block height
	SafeHeight int64 `json:"safeHeight,omitempty"`

	// UnsafeHeight is the current unsafe L2 block height
	UnsafeHeight int64 `json:"unsafeHeight,omitempty"`

	// Message contains additional status information
	Message string `json:"message,omitempty"`
}

// BatcherStatus represents the status of the op-batcher component
type BatcherStatus struct {
	// Ready indicates if the component is ready
	Ready bool `json:"ready"`

	// LastBatchSubmission is the timestamp of the last batch submission
	LastBatchSubmission *metav1.Time `json:"lastBatchSubmission,omitempty"`

	// Message contains additional status information
	Message string `json:"message,omitempty"`
}

// ProposerStatus represents the status of the op-proposer component
type ProposerStatus struct {
	// Ready indicates if the component is ready
	Ready bool `json:"ready"`

	// LastProposal is the timestamp of the last output root proposal
	LastProposal *metav1.Time `json:"lastProposal,omitempty"`

	// Message contains additional status information
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="ChainID",type="integer",JSONPath=".spec.chainId"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OPChain is the Schema for the opchains API.
type OPChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OPChainSpec   `json:"spec,omitempty"`
	Status OPChainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OPChainList contains a list of OPChain.
type OPChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OPChain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OPChain{}, &OPChainList{})
}
