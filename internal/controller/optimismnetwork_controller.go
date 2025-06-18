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

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ethereum/go-ethereum/ethclient"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/discovery"
	"github.com/ethereum-optimism/op-stack-operator/pkg/utils"
)

// OptimismNetworkFinalizer is the finalizer for OptimismNetwork resources
const OptimismNetworkFinalizer = "optimismnetwork.optimism.io/finalizer"

// Phase constants for OptimismNetwork status
const (
	PhaseError = "Error"
	PhaseReady = "Ready"
)

// OptimismNetworkReconciler reconciles an OptimismNetwork object
type OptimismNetworkReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	DiscoveryService *discovery.ContractDiscoveryService
}

// +kubebuilder:rbac:groups=optimism.optimism.io,resources=optimismnetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=optimismnetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=optimismnetworks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OptimismNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OptimismNetwork instance
	var network optimismv1alpha1.OptimismNetwork
	if err := r.Get(ctx, req.NamespacedName, &network); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch OptimismNetwork")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if network.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &network)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&network, OptimismNetworkFinalizer) {
		controllerutil.AddFinalizer(&network, OptimismNetworkFinalizer)
		if err := r.Update(ctx, &network); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate configuration
	if err := r.validateConfiguration(&network); err != nil {
		utils.SetCondition(&network.Status.Conditions, "ConfigurationValid", metav1.ConditionFalse, "InvalidConfiguration", err.Error())
		network.Status.Phase = PhaseError
		if statusErr := r.Status().Update(ctx, &network); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	utils.SetCondition(&network.Status.Conditions, "ConfigurationValid", metav1.ConditionTrue, "ValidConfiguration", "Network configuration is valid")

	// Test L1 connectivity
	if err := r.testL1Connectivity(ctx, &network); err != nil {
		utils.SetCondition(&network.Status.Conditions, "L1Connected", metav1.ConditionFalse, "L1ConnectionFailed", fmt.Sprintf("Failed to connect to L1: %v", err))
		network.Status.Phase = PhaseError
		if statusErr := r.Status().Update(ctx, &network); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	utils.SetCondition(&network.Status.Conditions, "L1Connected", metav1.ConditionTrue, "L1ConnectionSuccess", "Successfully connected to L1 RPC endpoint")

	// Discover contract addresses
	addresses, err := r.discoverContractAddresses(ctx, &network)
	if err != nil {
		utils.SetCondition(&network.Status.Conditions, "ContractsDiscovered", metav1.ConditionFalse, "DiscoveryFailed", fmt.Sprintf("Failed to discover contracts: %v", err))
		network.Status.Phase = PhaseError
		if statusErr := r.Status().Update(ctx, &network); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	utils.SetCondition(&network.Status.Conditions, "ContractsDiscovered", metav1.ConditionTrue, "AddressesResolved", fmt.Sprintf("Contract addresses discovered via %s", addresses.DiscoveryMethod))

	// Update network info in status
	if network.Status.NetworkInfo == nil {
		network.Status.NetworkInfo = &optimismv1alpha1.NetworkInfo{}
	}
	network.Status.NetworkInfo.DiscoveredContracts = addresses
	network.Status.NetworkInfo.LastUpdated = metav1.Now()

	// Create ConfigMaps for rollup config and genesis
	if err := r.reconcileConfigMaps(ctx, &network, addresses); err != nil {
		logger.Error(err, "failed to reconcile ConfigMaps")
		network.Status.Phase = PhaseError
		if statusErr := r.Status().Update(ctx, &network); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	// Update final status
	network.Status.Phase = PhaseReady
	network.Status.ObservedGeneration = network.Generation

	if err := r.Status().Update(ctx, &network); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("OptimismNetwork reconciled successfully", "name", network.Name, "phase", network.Status.Phase)
	return ctrl.Result{RequeueAfter: time.Hour}, nil // Re-check contract addresses periodically
}

// validateConfiguration validates the OptimismNetwork configuration
func (r *OptimismNetworkReconciler) validateConfiguration(network *optimismv1alpha1.OptimismNetwork) error {
	// Check required fields
	if network.Spec.ChainID == 0 {
		return fmt.Errorf("chainID is required")
	}
	if network.Spec.L1ChainID == 0 {
		return fmt.Errorf("l1ChainID is required")
	}
	if network.Spec.L1RpcUrl == "" {
		return fmt.Errorf("l1RpcUrl is required")
	}

	// Validate chain ID relationship
	if network.Spec.ChainID == network.Spec.L1ChainID {
		return fmt.Errorf("chainID cannot be the same as l1ChainID")
	}

	// Validate configuration sources (if specified)
	if err := r.validateConfigSource(network.Spec.RollupConfig, "rollupConfig"); err != nil {
		return err
	}
	if err := r.validateConfigSource(network.Spec.L2Genesis, "l2Genesis"); err != nil {
		return err
	}

	return nil
}

// validateConfigSource validates a configuration source
func (r *OptimismNetworkReconciler) validateConfigSource(source *optimismv1alpha1.ConfigSource, fieldName string) error {
	if source == nil {
		return nil
	}

	sourceCount := 0
	if source.Inline != "" {
		sourceCount++
	}
	if source.ConfigMapRef != nil {
		sourceCount++
	}
	if source.AutoDiscover {
		sourceCount++
	}

	if sourceCount > 1 {
		return fmt.Errorf("%s: only one of inline, configMapRef, or autoDiscover can be specified", fieldName)
	}

	return nil
}

// testL1Connectivity tests connectivity to the L1 RPC endpoint
func (r *OptimismNetworkReconciler) testL1Connectivity(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	// Create L1 client with timeout
	timeout := 10 * time.Second
	if network.Spec.L1RpcTimeout != 0 {
		timeout = network.Spec.L1RpcTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := ethclient.DialContext(ctx, network.Spec.L1RpcUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}
	defer client.Close()

	// Get chain ID to verify connection and configuration
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L1 chain ID: %w", err)
	}

	if chainID.Int64() != network.Spec.L1ChainID {
		return fmt.Errorf("L1 chain ID mismatch: expected %d, got %d", network.Spec.L1ChainID, chainID.Int64())
	}

	return nil
}

// discoverContractAddresses discovers and caches contract addresses
func (r *OptimismNetworkReconciler) discoverContractAddresses(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) (*optimismv1alpha1.NetworkContractAddresses, error) {
	if r.DiscoveryService == nil {
		r.DiscoveryService = discovery.NewContractDiscoveryService(24 * time.Hour)
	}

	addresses, err := r.DiscoveryService.DiscoverContracts(ctx, network)
	if err != nil {
		return nil, err
	}

	return addresses, nil
}

// reconcileConfigMaps manages ConfigMaps for rollup config and genesis data
func (r *OptimismNetworkReconciler) reconcileConfigMaps(ctx context.Context, network *optimismv1alpha1.OptimismNetwork, addresses *optimismv1alpha1.NetworkContractAddresses) error {
	// Create ConfigMap for rollup configuration
	if network.Spec.RollupConfig != nil && network.Spec.RollupConfig.AutoDiscover {
		rollupConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      network.Name + "-rollup-config",
				Namespace: network.Namespace,
			},
			Data: map[string]string{
				"rollup.json": r.generateRollupConfig(network, addresses),
			},
		}

		if err := controllerutil.SetControllerReference(network, rollupConfigMap, r.Scheme); err != nil {
			return err
		}

		if err := r.createOrUpdateConfigMap(ctx, rollupConfigMap); err != nil {
			return fmt.Errorf("failed to create rollup config map: %w", err)
		}
	}

	// Create ConfigMap for L2 genesis
	if network.Spec.L2Genesis != nil && network.Spec.L2Genesis.AutoDiscover {
		genesisConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      network.Name + "-genesis",
				Namespace: network.Namespace,
			},
			Data: map[string]string{
				"genesis.json": r.generateGenesisConfig(network, addresses),
			},
		}

		if err := controllerutil.SetControllerReference(network, genesisConfigMap, r.Scheme); err != nil {
			return err
		}

		if err := r.createOrUpdateConfigMap(ctx, genesisConfigMap); err != nil {
			return fmt.Errorf("failed to create genesis config map: %w", err)
		}
	}

	return nil
}

// generateRollupConfig generates a ConfigMap with rollup configuration
func (r *OptimismNetworkReconciler) generateRollupConfig(network *optimismv1alpha1.OptimismNetwork, addresses *optimismv1alpha1.NetworkContractAddresses) string {
	// Generate a basic rollup configuration
	// In a real implementation, this would create a proper rollup.json structure
	return fmt.Sprintf(`{
	"genesis": {
		"l1": {
			"hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"number": 0
		},
		"l2": {
			"hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"number": 0
		},
		"l2_time": 0,
		"system_config": {
			"batcherAddr": "0x0000000000000000000000000000000000000000",
			"overhead": "0x00000000000000000000000000000000000000000000000000000000000000bc",
			"scalar": "0x00000000000000000000000000000000000000000000000000000000000a6fe0",
			"gasLimit": 30000000
		}
	},
	"block_time": 2,
	"max_sequencer_drift": 600,
	"seq_window_size": 3600,
	"channel_timeout": 300,
	"l1_chain_id": %d,
	"l2_chain_id": %d,
	"regolith_time": 0,
	"canyon_time": 0,
	"delta_time": 0,
	"ecotone_time": 0,
	"fjord_time": 0,
	"granite_time": 0,
	"holocene_time": 0,
	"batch_inbox_address": "0xff00000000000000000000000000000000000000",
	"deposit_contract_address": "%s",
	"l1_system_config_address": "%s"
}`, network.Spec.L1ChainID, network.Spec.ChainID,
		addresses.OptimismPortalAddr, addresses.SystemConfigAddr)
}

// generateGenesisConfig generates a ConfigMap with L2 genesis configuration
func (r *OptimismNetworkReconciler) generateGenesisConfig(network *optimismv1alpha1.OptimismNetwork, _ *optimismv1alpha1.NetworkContractAddresses) string {
	// Generate a basic L2 genesis configuration
	// In a real implementation, this would create a proper genesis.json structure
	return fmt.Sprintf(`{
	"config": {
		"chainId": %d,
		"homesteadBlock": 0,
		"eip150Block": 0,
		"eip155Block": 0,
		"eip158Block": 0,
		"byzantiumBlock": 0,
		"constantinopleBlock": 0,
		"petersburgBlock": 0,
		"istanbulBlock": 0,
		"muirGlacierBlock": 0,
		"berlinBlock": 0,
		"londonBlock": 4,
		"arrowGlacierBlock": 4,
		"grayGlacierBlock": 4,
		"mergeNetsplitBlock": 4,
		"shanghaiTime": 4,
		"cancunTime": 4,
		"terminalTotalDifficulty": 0,
		"terminalTotalDifficultyPassed": true,
		"optimism": {
			"eip1559Elasticity": 6,
			"eip1559Denominator": 50,
			"eip1559DenominatorCanyon": 250
		}
	},
	"nonce": "0x0",
	"timestamp": "0x4",
	"extraData": "0x",
	"gasLimit": "0x1c9c380",
	"difficulty": "0x0",
	"mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	"coinbase": "0x4200000000000000000000000000000000000011",
	"alloc": {},
	"number": "0x0",
	"gasUsed": "0x0",
	"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	"baseFeePerGas": "0x3b9aca00",
	"excessBlobGas": "0x0",
	"blobGasUsed": "0x0"
}`, network.Spec.ChainID)
}

// createOrUpdateConfigMap creates or updates a ConfigMap
func (r *OptimismNetworkReconciler) createOrUpdateConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	var existing corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, configMap)
		}
		return err
	}

	// Update existing ConfigMap
	existing.Data = configMap.Data
	return r.Update(ctx, &existing)
}

// handleDeletion handles the deletion of OptimismNetwork resources
func (r *OptimismNetworkReconciler) handleDeletion(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Clean up any resources created by this network
	// For now, we rely on owner references to clean up ConfigMaps

	// Remove finalizer
	controllerutil.RemoveFinalizer(network, OptimismNetworkFinalizer)
	if err := r.Update(ctx, network); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("OptimismNetwork deleted successfully", "name", network.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OptimismNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&optimismv1alpha1.OptimismNetwork{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
