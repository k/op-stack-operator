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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ethereum/go-ethereum/ethclient"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/discovery"
	"github.com/ethereum-optimism/op-stack-operator/pkg/utils"
)

const (
	OptimismNetworkFinalizer = "optimismnetwork.optimism.io/finalizer"
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
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OptimismNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OptimismNetwork instance
	var optimismNetwork optimismv1alpha1.OptimismNetwork
	if err := r.Get(ctx, req.NamespacedName, &optimismNetwork); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return without error
			logger.Info("OptimismNetwork resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request
		logger.Error(err, "Failed to get OptimismNetwork")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if optimismNetwork.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &optimismNetwork)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&optimismNetwork, OptimismNetworkFinalizer) {
		controllerutil.AddFinalizer(&optimismNetwork, OptimismNetworkFinalizer)
		if err := r.Update(ctx, &optimismNetwork); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed
	if optimismNetwork.Status.Phase == "" {
		optimismNetwork.Status.Phase = "Pending"
		optimismNetwork.Status.NetworkInfo = &optimismv1alpha1.NetworkInfo{
			DeploymentTimestamp: metav1.Now(),
		}
	}

	// Set observed generation
	optimismNetwork.Status.ObservedGeneration = optimismNetwork.Generation

	// Validate configuration
	if err := r.validateConfiguration(ctx, &optimismNetwork); err != nil {
		logger.Error(err, "Configuration validation failed")
		utils.SetConditionFalse(&optimismNetwork.Status.Conditions,
			utils.ConditionConfigurationValid, utils.ReasonInvalidConfiguration, err.Error())
		optimismNetwork.Status.Phase = "Error"
		return r.updateStatus(ctx, &optimismNetwork, ctrl.Result{RequeueAfter: time.Minute * 5})
	}

	utils.SetConditionTrue(&optimismNetwork.Status.Conditions,
		utils.ConditionConfigurationValid, utils.ReasonValidConfiguration, "Network configuration is valid")

	// Test L1 connectivity
	if err := r.testL1Connectivity(ctx, &optimismNetwork); err != nil {
		logger.Error(err, "L1 connectivity test failed")
		utils.SetConditionFalse(&optimismNetwork.Status.Conditions,
			utils.ConditionL1Connected, utils.ReasonRPCEndpointUnreachable, err.Error())
	} else {
		utils.SetConditionTrue(&optimismNetwork.Status.Conditions,
			utils.ConditionL1Connected, utils.ReasonRPCEndpointReachable, "L1 RPC endpoint is responsive")
	}

	// Test L2 connectivity if L2 RPC URL is provided
	if optimismNetwork.Spec.L2RpcUrl != "" {
		if err := r.testL2Connectivity(ctx, &optimismNetwork); err != nil {
			logger.Error(err, "L2 connectivity test failed")
			utils.SetConditionFalse(&optimismNetwork.Status.Conditions,
				utils.ConditionL2Connected, utils.ReasonRPCEndpointUnreachable, err.Error())
		} else {
			utils.SetConditionTrue(&optimismNetwork.Status.Conditions,
				utils.ConditionL2Connected, utils.ReasonRPCEndpointReachable, "L2 RPC endpoint is responsive")
		}
	}

	// Discover contract addresses
	if err := r.discoverContractAddresses(ctx, &optimismNetwork); err != nil {
		logger.Error(err, "Contract address discovery failed")
		utils.SetConditionFalse(&optimismNetwork.Status.Conditions,
			utils.ConditionContractsDiscovered, utils.ReasonDiscoveryFailed, err.Error())
	} else {
		utils.SetConditionTrue(&optimismNetwork.Status.Conditions,
			utils.ConditionContractsDiscovered, utils.ReasonAddressesResolved, "All contract addresses discovered successfully")
	}

	// Generate and manage ConfigMaps
	if err := r.reconcileConfigMaps(ctx, &optimismNetwork); err != nil {
		logger.Error(err, "Failed to reconcile ConfigMaps")
		return r.updateStatus(ctx, &optimismNetwork, ctrl.Result{RequeueAfter: time.Minute * 2})
	}

	// Update phase based on conditions
	r.updatePhase(&optimismNetwork)

	// Update status and set next reconcile
	optimismNetwork.Status.NetworkInfo.LastUpdated = metav1.Now()
	return r.updateStatus(ctx, &optimismNetwork, ctrl.Result{RequeueAfter: time.Minute * 10})
}

// validateConfiguration validates the OptimismNetwork configuration
func (r *OptimismNetworkReconciler) validateConfiguration(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	// Validate required fields
	if network.Spec.ChainID == 0 {
		return fmt.Errorf("chainID is required")
	}
	if network.Spec.L1ChainID == 0 {
		return fmt.Errorf("l1ChainID is required")
	}
	if network.Spec.L1RpcUrl == "" {
		return fmt.Errorf("l1RpcUrl is required")
	}

	// Validate chain ID relationships
	if network.Spec.ChainID == network.Spec.L1ChainID {
		return fmt.Errorf("chainID and l1ChainID cannot be the same")
	}

	// Validate configuration sources if provided
	if network.Spec.RollupConfig != nil {
		if err := r.validateConfigSource(ctx, network.Spec.RollupConfig, network.Namespace); err != nil {
			return fmt.Errorf("invalid rollupConfig: %w", err)
		}
	}
	if network.Spec.L2Genesis != nil {
		if err := r.validateConfigSource(ctx, network.Spec.L2Genesis, network.Namespace); err != nil {
			return fmt.Errorf("invalid l2Genesis: %w", err)
		}
	}

	return nil
}

// validateConfigSource validates a configuration source
func (r *OptimismNetworkReconciler) validateConfigSource(ctx context.Context, source *optimismv1alpha1.ConfigSource, namespace string) error {
	if source.ConfigMapRef != nil {
		// Verify ConfigMap exists
		var configMap corev1.ConfigMap
		configMapKey := client.ObjectKey{
			Name:      source.ConfigMapRef.Name,
			Namespace: namespace,
		}
		if err := r.Get(ctx, configMapKey, &configMap); err != nil {
			return fmt.Errorf("configMap %s not found: %w", source.ConfigMapRef.Name, err)
		}
		// Verify key exists
		if source.ConfigMapRef.Key != "" {
			if _, exists := configMap.Data[source.ConfigMapRef.Key]; !exists {
				return fmt.Errorf("key %s not found in configMap %s", source.ConfigMapRef.Key, source.ConfigMapRef.Name)
			}
		}
	}
	return nil
}

// testL1Connectivity tests connectivity to the L1 RPC endpoint
func (r *OptimismNetworkReconciler) testL1Connectivity(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	client, err := ethclient.Dial(network.Spec.L1RpcUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}
	defer client.Close()

	// Test basic connectivity by getting the chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L1 chain ID: %w", err)
	}

	// Verify chain ID matches expected
	if chainID.Int64() != network.Spec.L1ChainID {
		return fmt.Errorf("L1 chain ID mismatch: expected %d, got %d", network.Spec.L1ChainID, chainID.Int64())
	}

	return nil
}

// testL2Connectivity tests connectivity to the L2 RPC endpoint
func (r *OptimismNetworkReconciler) testL2Connectivity(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	client, err := ethclient.Dial(network.Spec.L2RpcUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L2 RPC: %w", err)
	}
	defer client.Close()

	// Test basic connectivity by getting the chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L2 chain ID: %w", err)
	}

	// Verify chain ID matches expected
	if chainID.Int64() != network.Spec.ChainID {
		return fmt.Errorf("L2 chain ID mismatch: expected %d, got %d", network.Spec.ChainID, chainID.Int64())
	}

	return nil
}

// discoverContractAddresses discovers and caches contract addresses
func (r *OptimismNetworkReconciler) discoverContractAddresses(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	addresses, err := r.DiscoveryService.DiscoverContracts(ctx, network)
	if err != nil {
		return fmt.Errorf("failed to discover contract addresses: %w", err)
	}

	// Update status with discovered addresses
	if network.Status.NetworkInfo == nil {
		network.Status.NetworkInfo = &optimismv1alpha1.NetworkInfo{}
	}
	network.Status.NetworkInfo.DiscoveredContracts = addresses

	return nil
}

// reconcileConfigMaps manages ConfigMaps for rollup config and genesis data
func (r *OptimismNetworkReconciler) reconcileConfigMaps(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	// Generate rollup config ConfigMap if needed
	if network.Spec.RollupConfig != nil && network.Spec.RollupConfig.AutoDiscover {
		if err := r.generateRollupConfigMap(ctx, network); err != nil {
			return fmt.Errorf("failed to generate rollup config: %w", err)
		}
	}

	// Generate genesis ConfigMap if needed
	if network.Spec.L2Genesis != nil && network.Spec.L2Genesis.AutoDiscover {
		if err := r.generateGenesisConfigMap(ctx, network); err != nil {
			return fmt.Errorf("failed to generate genesis config: %w", err)
		}
	}

	return nil
}

// generateRollupConfigMap generates a ConfigMap with rollup configuration
func (r *OptimismNetworkReconciler) generateRollupConfigMap(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	// For now, create a placeholder ConfigMap
	// In a full implementation, this would fetch the actual rollup config from L2
	configMapName := fmt.Sprintf("%s-rollup-config", network.Name)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: network.Namespace,
		},
		Data: map[string]string{
			"rollup.json": `{
				"genesis": {
					"l1": {
						"hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
						"number": 0
					},
					"l2": {
						"hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
						"number": 0
					},
					"l2_time": 0
				},
				"block_time": 2,
				"max_sequencer_drift": 600,
				"seq_window_size": 3600,
				"channel_timeout": 300
			}`,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(network, configMap, r.Scheme); err != nil {
		return err
	}

	// Create or update ConfigMap
	if err := r.Client.Patch(ctx, configMap, client.Apply, client.ForceOwnership, client.FieldOwner("optimism-network-controller")); err != nil {
		return err
	}

	return nil
}

// generateGenesisConfigMap generates a ConfigMap with L2 genesis configuration
func (r *OptimismNetworkReconciler) generateGenesisConfigMap(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) error {
	// For now, create a placeholder ConfigMap
	// In a full implementation, this would fetch the actual genesis from L2
	configMapName := fmt.Sprintf("%s-genesis", network.Name)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: network.Namespace,
		},
		Data: map[string]string{
			"genesis.json": `{
				"config": {
					"chainId": ` + fmt.Sprintf("%d", network.Spec.ChainID) + `,
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
					"londonBlock": 0,
					"arrowGlacierBlock": 0,
					"grayGlacierBlock": 0,
					"mergeNetsplitBlock": 0,
					"bedrockBlock": 0,
					"regolithTime": 0,
					"canyonTime": 0,
					"optimism": {
						"eip1559Elasticity": 6,
						"eip1559Denominator": 50
					}
				},
				"alloc": {},
				"difficulty": "0x1",
				"gasLimit": "0x1c9c380"
			}`,
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(network, configMap, r.Scheme); err != nil {
		return err
	}

	// Create or update ConfigMap
	if err := r.Client.Patch(ctx, configMap, client.Apply, client.ForceOwnership, client.FieldOwner("optimism-network-controller")); err != nil {
		return err
	}

	return nil
}

// updatePhase updates the network phase based on conditions
func (r *OptimismNetworkReconciler) updatePhase(network *optimismv1alpha1.OptimismNetwork) {
	// Check if all critical conditions are True
	if utils.IsConditionTrue(network.Status.Conditions, utils.ConditionConfigurationValid) &&
		utils.IsConditionTrue(network.Status.Conditions, utils.ConditionL1Connected) &&
		utils.IsConditionTrue(network.Status.Conditions, utils.ConditionContractsDiscovered) {
		network.Status.Phase = "Ready"
	} else {
		// Check if any condition is False (error state)
		for _, condition := range network.Status.Conditions {
			if condition.Status == metav1.ConditionFalse {
				network.Status.Phase = "Error"
				return
			}
		}
		// If no conditions are False but not all are True, we're still pending
		network.Status.Phase = "Pending"
	}
}

// updateStatus updates the OptimismNetwork status
func (r *OptimismNetworkReconciler) updateStatus(ctx context.Context, network *optimismv1alpha1.OptimismNetwork, result ctrl.Result) (ctrl.Result, error) {
	if err := r.Status().Update(ctx, network); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update OptimismNetwork status")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	return result, nil
}

// handleDeletion handles the deletion of OptimismNetwork resources
func (r *OptimismNetworkReconciler) handleDeletion(ctx context.Context, network *optimismv1alpha1.OptimismNetwork) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling OptimismNetwork deletion")

	// Clean up any external resources here if necessary
	// For now, we rely on owner references to clean up ConfigMaps

	// Remove finalizer
	controllerutil.RemoveFinalizer(network, OptimismNetworkFinalizer)
	if err := r.Update(ctx, network); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OptimismNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize discovery service if not provided
	if r.DiscoveryService == nil {
		r.DiscoveryService = discovery.NewContractDiscoveryService(24 * time.Hour)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&optimismv1alpha1.OptimismNetwork{}).
		Owns(&corev1.ConfigMap{}).
		Named("optimismnetwork").
		Complete(r)
}
