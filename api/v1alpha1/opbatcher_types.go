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

// OpBatcherSpec defines the desired state of OpBatcher.
type OpBatcherSpec struct {
	// Network Reference
	// +kubebuilder:validation:Required
	OptimismNetworkRef OptimismNetworkRef `json:"optimismNetworkRef"`

	// L2 Sequencer Configuration
	// +kubebuilder:validation:Required
	SequencerRef *SequencerRef `json:"sequencerRef"`

	// Private Key for L1 Transaction Signing
	// +kubebuilder:validation:Required
	PrivateKey SecretKeyRef `json:"privateKey"`

	// Batching Configuration
	// +optional
	Batching *BatchingConfig `json:"batching,omitempty"`

	// Data Availability Configuration
	// +optional
	DataAvailability *DataAvailabilityConfig `json:"dataAvailability,omitempty"`

	// Throttling Configuration
	// +optional
	Throttling *ThrottlingConfig `json:"throttling,omitempty"`

	// L1 Transaction Management
	// +optional
	L1Transaction *L1TransactionConfig `json:"l1Transaction,omitempty"`

	// RPC Configuration
	// +optional
	RPC *RPCConfig `json:"rpc,omitempty"`

	// Metrics Configuration
	// +optional
	Metrics *MetricsConfig `json:"metrics,omitempty"`

	// Resources
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// SequencerRef references an OpNode sequencer instance
type SequencerRef struct {
	// Name of the sequencer OpNode
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the sequencer OpNode (optional, defaults to same namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// BatchingConfig defines configuration for batching
type BatchingConfig struct {
	// Maximum duration for a channel
	// +kubebuilder:default="10m"
	MaxChannelDuration *metav1.Duration `json:"maxChannelDuration,omitempty"`

	// Safety margin for L1 confirmations
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	SubSafetyMargin *int32 `json:"subSafetyMargin,omitempty"`

	// Target size for L1 transactions (bytes)
	// +kubebuilder:validation:Minimum=1000
	// +kubebuilder:default=120000
	TargetL1TxSize *int32 `json:"targetL1TxSize,omitempty"`

	// Target number of frames per transaction
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	TargetNumFrames *int32 `json:"targetNumFrames,omitempty"`

	// Approximate compression ratio (between 0.1 and 1.0, as string)
	// +kubebuilder:default="0.4"
	ApproxComprRatio *string `json:"approxComprRatio,omitempty"`
}

// DataAvailabilityConfig defines data availability settings
type DataAvailabilityConfig struct {
	// Type of data availability
	// +kubebuilder:validation:Enum=blobs;calldata
	// +kubebuilder:default="blobs"
	Type string `json:"type,omitempty"`

	// Maximum blobs per transaction (EIP-4844)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=6
	// +kubebuilder:default=6
	MaxBlobsPerTx *int32 `json:"maxBlobsPerTx,omitempty"`
}

// ThrottlingConfig defines throttling settings
type ThrottlingConfig struct {
	// Enable throttling
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Maximum pending transactions
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	MaxPendingTx *int32 `json:"maxPendingTx,omitempty"`

	// Safety margin for backlog
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	BacklogSafetyMargin *int32 `json:"backlogSafetyMargin,omitempty"`
}

// L1TransactionConfig defines L1 transaction management settings
type L1TransactionConfig struct {
	// Fee limit multiplier for dynamic fees
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	FeeLimitMultiplier *int32 `json:"feeLimitMultiplier,omitempty"`

	// Timeout before resubmitting transaction
	// +kubebuilder:default="48s"
	ResubmissionTimeout *metav1.Duration `json:"resubmissionTimeout,omitempty"`

	// Number of confirmations to wait
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	NumConfirmations *int32 `json:"numConfirmations,omitempty"`

	// Abort threshold for nonce too low errors
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	SafeAbortNonceTooLowCount *int32 `json:"safeAbortNonceTooLowCount,omitempty"`
}

// OpBatcherStatus defines the observed state of OpBatcher.
type OpBatcherStatus struct {
	// Phase represents the current phase of the OpBatcher
	// +optional
	Phase OpBatcherPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the OpBatcher's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// BatcherInfo contains operational information about the batcher
	// +optional
	BatcherInfo *OpBatcherInfo `json:"batcherInfo,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// OpBatcherPhase represents the current phase of the OpBatcher
// +kubebuilder:validation:Enum=Pending;Running;Error;Stopped
type OpBatcherPhase string

const (
	OpBatcherPhasePending OpBatcherPhase = "Pending"
	OpBatcherPhaseRunning OpBatcherPhase = "Running"
	OpBatcherPhaseError   OpBatcherPhase = "Error"
	OpBatcherPhaseStopped OpBatcherPhase = "Stopped"
)

// OpBatcherInfo contains operational information about the batcher
type OpBatcherInfo struct {
	// LastBatchSubmitted contains information about the last submitted batch
	// +optional
	LastBatchSubmitted *BatchInfo `json:"lastBatchSubmitted,omitempty"`

	// PendingBatches is the number of pending batches
	// +optional
	PendingBatches int32 `json:"pendingBatches,omitempty"`

	// TotalBatchesSubmitted is the total number of batches submitted
	// +optional
	TotalBatchesSubmitted int64 `json:"totalBatchesSubmitted,omitempty"`
}

// BatchInfo contains information about a submitted batch
type BatchInfo struct {
	// BlockNumber is the L2 block number of the batch
	// +optional
	BlockNumber int64 `json:"blockNumber,omitempty"`

	// TransactionHash is the L1 transaction hash
	// +optional
	TransactionHash string `json:"transactionHash,omitempty"`

	// Timestamp is when the batch was submitted
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// GasUsed is the gas used by the L1 transaction
	// +optional
	GasUsed int64 `json:"gasUsed,omitempty"`
}

// OpBatcher condition types
const (
	// OpBatcherConditionL1Connected indicates whether the batcher is connected to L1
	OpBatcherConditionL1Connected = "L1Connected"

	// OpBatcherConditionL2Connected indicates whether the batcher is connected to L2 sequencer
	OpBatcherConditionL2Connected = "L2Connected"

	// OpBatcherConditionPrivateKeyLoaded indicates whether the private key is loaded
	OpBatcherConditionPrivateKeyLoaded = "PrivateKeyLoaded"

	// OpBatcherConditionBatching indicates whether the batcher is actively batching
	OpBatcherConditionBatching = "Batching"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Network",type=string,JSONPath=`.spec.optimismNetworkRef.name`
// +kubebuilder:printcolumn:name="Sequencer",type=string,JSONPath=`.spec.sequencerRef.name`
// +kubebuilder:printcolumn:name="Pending Batches",type=integer,JSONPath=`.status.batcherInfo.pendingBatches`
// +kubebuilder:printcolumn:name="Total Batches",type=integer,JSONPath=`.status.batcherInfo.totalBatchesSubmitted`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpBatcher is the Schema for the opbatchers API.
type OpBatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpBatcherSpec   `json:"spec,omitempty"`
	Status OpBatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpBatcherList contains a list of OpBatcher.
type OpBatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpBatcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpBatcher{}, &OpBatcherList{})
}
