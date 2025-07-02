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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/resources"
	"github.com/ethereum-optimism/op-stack-operator/pkg/utils"
)

// OpNodeFinalizer is the finalizer for OpNode resources
const OpNodeFinalizer = "opnode.optimism.io/finalizer"

// Phase constants for OpNode status
const (
	OpNodePhasePending      = "Pending"
	OpNodePhaseInitializing = "Initializing"
	OpNodePhaseRunning      = "Running"
	OpNodePhaseError        = "Error"
	OpNodePhaseStopped      = "Stopped"
)

// OpNodeReconciler reconciles an OpNode object
type OpNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opnodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opnodes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets;configmaps;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpNode instance
	var opNode optimismv1alpha1.OpNode
	if err := r.Get(ctx, req.NamespacedName, &opNode); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch OpNode")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if opNode.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &opNode)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&opNode, OpNodeFinalizer) {
		controllerutil.AddFinalizer(&opNode, OpNodeFinalizer)
		if err := r.Update(ctx, &opNode); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate configuration
	if err := r.validateConfiguration(&opNode); err != nil {
		utils.SetCondition(&opNode.Status.Conditions, "ConfigurationValid", metav1.ConditionFalse, "InvalidConfiguration", err.Error())
		opNode.Status.Phase = OpNodePhaseError
		// Update status with retry and return
		opNode.Status.ObservedGeneration = opNode.Generation
		if statusErr := r.updateStatusWithRetry(ctx, &opNode); statusErr != nil {
			logger.Error(statusErr, "failed to update status after validation error")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	utils.SetCondition(&opNode.Status.Conditions, "ConfigurationValid", metav1.ConditionTrue, "ValidConfiguration", "OpNode configuration is valid")

	// Fetch referenced OptimismNetwork
	network, err := r.fetchOptimismNetwork(ctx, &opNode)
	if err != nil {
		utils.SetCondition(&opNode.Status.Conditions, "NetworkReference", metav1.ConditionFalse, "NetworkNotFound", fmt.Sprintf("Failed to fetch OptimismNetwork: %v", err))
		opNode.Status.Phase = OpNodePhaseError
		// Update status with retry and return
		opNode.Status.ObservedGeneration = opNode.Generation
		if statusErr := r.updateStatusWithRetry(ctx, &opNode); statusErr != nil {
			logger.Error(statusErr, "failed to update status after network fetch error")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	utils.SetCondition(&opNode.Status.Conditions, "NetworkReference", metav1.ConditionTrue, "NetworkFound", "OptimismNetwork reference resolved successfully")

	// All core resources are reconciled once the network is ready
	if network.Status.Phase != PhaseReady {
		// Network not yet ready: update status and requeue
		utils.SetCondition(&opNode.Status.Conditions, "NetworkReady", metav1.ConditionFalse, "NetworkNotReady", "OptimismNetwork is not ready")
		opNode.Status.Phase = OpNodePhasePending
		opNode.Status.ObservedGeneration = opNode.Generation
		if err := r.updateStatusWithRetry(ctx, &opNode); err != nil {
			logger.Error(err, "failed to update status for network pending")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	// Network is ready
	utils.SetCondition(&opNode.Status.Conditions, "NetworkReady", metav1.ConditionTrue, "NetworkReady", "OptimismNetwork is ready")
	opNode.Status.Phase = OpNodePhaseInitializing

	// 1) Reconcile secrets
	if err := r.reconcileSecrets(ctx, &opNode); err != nil {
		utils.SetCondition(&opNode.Status.Conditions, "SecretsReady", metav1.ConditionFalse, "SecretReconciliationFailed", fmt.Sprintf("Failed to reconcile secrets: %v", err))
		opNode.Status.Phase = OpNodePhaseError
		goto updateStatus
	}
	utils.SetCondition(&opNode.Status.Conditions, "SecretsReady", metav1.ConditionTrue, "SecretsReconciled", "All required secrets are ready")

	// 2) Reconcile StatefulSet
	if err := r.reconcileStatefulSet(ctx, &opNode, network); err != nil {
		utils.SetCondition(&opNode.Status.Conditions, "StatefulSetReady", metav1.ConditionFalse, "StatefulSetReconciliationFailed", fmt.Sprintf("Failed to reconcile StatefulSet: %v", err))
		opNode.Status.Phase = OpNodePhaseError
		goto updateStatus
	}
	utils.SetCondition(&opNode.Status.Conditions, "StatefulSetReady", metav1.ConditionTrue, "StatefulSetReconciled", "StatefulSet is ready")

	// 3) Reconcile Service
	if err := r.reconcileService(ctx, &opNode, network); err != nil {
		utils.SetCondition(&opNode.Status.Conditions, "ServiceReady", metav1.ConditionFalse, "ServiceReconciliationFailed", fmt.Sprintf("Failed to reconcile Service: %v", err))
		opNode.Status.Phase = OpNodePhaseError
		goto updateStatus
	}
	utils.SetCondition(&opNode.Status.Conditions, "ServiceReady", metav1.ConditionTrue, "ServiceReconciled", "Service is ready")

	// 4) All done
	r.updateNodeStatus(ctx, &opNode)
	opNode.Status.Phase = OpNodePhaseRunning

updateStatus:
	// Consolidated status update
	opNode.Status.ObservedGeneration = opNode.Generation
	if err := r.updateStatusWithRetry(ctx, &opNode); err != nil {
		logger.Error(err, "failed to update status")
	}
	// Decide requeue interval
	var requeueAfter time.Duration
	switch opNode.Status.Phase {
	case OpNodePhaseError:
		requeueAfter = time.Minute * 2
	case OpNodePhasePending, OpNodePhaseInitializing:
		requeueAfter = time.Minute
	case OpNodePhaseRunning:
		requeueAfter = time.Minute * 5
	default:
		requeueAfter = time.Minute
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// validateConfiguration validates the OpNode configuration
func (r *OpNodeReconciler) validateConfiguration(opNode *optimismv1alpha1.OpNode) error {
	// Check required fields
	if opNode.Spec.OptimismNetworkRef.Name == "" {
		return fmt.Errorf("optimismNetworkRef.name is required")
	}

	// Validate node type
	if opNode.Spec.NodeType == "" {
		return fmt.Errorf("nodeType is required")
	}
	if opNode.Spec.NodeType != "sequencer" && opNode.Spec.NodeType != "replica" {
		return fmt.Errorf("nodeType must be 'sequencer' or 'replica'")
	}

	// Validate sequencer-specific configuration
	if opNode.Spec.NodeType == "sequencer" {
		if opNode.Spec.OpNode.Sequencer == nil || !opNode.Spec.OpNode.Sequencer.Enabled {
			return fmt.Errorf("sequencer configuration is required for sequencer nodes")
		}

		// Sequencers should have discovery disabled for isolation
		if opNode.Spec.OpNode.P2P != nil && opNode.Spec.OpNode.P2P.Discovery != nil && opNode.Spec.OpNode.P2P.Discovery.Enabled {
			return fmt.Errorf("sequencer nodes should have P2P discovery disabled for security")
		}

		// For sequencer nodes, L2RpcUrl should only be set when connecting to external networks
		// If L2RpcUrl is set for a sequencer, it typically means they're part of a larger network
		// and not the primary sequencer, so we allow it but validate it's a valid URL
		if opNode.Spec.L2RpcUrl != "" {
			// Basic URL validation - ensure it starts with http/https
			if len(opNode.Spec.L2RpcUrl) < 7 || 
				(!strings.HasPrefix(opNode.Spec.L2RpcUrl, "http://") && 
				 !strings.HasPrefix(opNode.Spec.L2RpcUrl, "https://")) {
				return fmt.Errorf("L2RpcUrl must be a valid HTTP/HTTPS URL")
			}
		}
	}

	// Validate storage configuration
	if opNode.Spec.OpGeth.Storage != nil {
		if opNode.Spec.OpGeth.Storage.Size.IsZero() {
			return fmt.Errorf("storage size must be specified")
		}
	}

	return nil
}

// fetchOptimismNetwork fetches the referenced OptimismNetwork
func (r *OpNodeReconciler) fetchOptimismNetwork(ctx context.Context, opNode *optimismv1alpha1.OpNode) (*optimismv1alpha1.OptimismNetwork, error) {
	namespace := opNode.Spec.OptimismNetworkRef.Namespace
	if namespace == "" {
		namespace = opNode.Namespace
	}

	var network optimismv1alpha1.OptimismNetwork
	key := types.NamespacedName{
		Name:      opNode.Spec.OptimismNetworkRef.Name,
		Namespace: namespace,
	}

	if err := r.Get(ctx, key, &network); err != nil {
		return nil, err
	}

	return &network, nil
}

// reconcileSecrets manages JWT secrets and P2P keys
func (r *OpNodeReconciler) reconcileSecrets(ctx context.Context, opNode *optimismv1alpha1.OpNode) error {
	// Reconcile JWT secret for Engine API
	if err := r.reconcileJWTSecret(ctx, opNode); err != nil {
		return fmt.Errorf("failed to reconcile JWT secret: %w", err)
	}

	// Reconcile P2P private key if auto-generation is enabled
	if opNode.Spec.OpNode.P2P != nil && opNode.Spec.OpNode.P2P.PrivateKey != nil && opNode.Spec.OpNode.P2P.PrivateKey.Generate {
		if err := r.reconcileP2PSecret(ctx, opNode); err != nil {
			return fmt.Errorf("failed to reconcile P2P secret: %w", err)
		}
	}

	return nil
}

// reconcileJWTSecret creates or updates the JWT secret for Engine API
func (r *OpNodeReconciler) reconcileJWTSecret(ctx context.Context, opNode *optimismv1alpha1.OpNode) error {
	secretName := opNode.Name + "-jwt"

	var secret corev1.Secret
	key := types.NamespacedName{Name: secretName, Namespace: opNode.Namespace}

	if err := r.Get(ctx, key, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new JWT secret
		jwtToken, err := generateJWTToken()
		if err != nil {
			return fmt.Errorf("failed to generate JWT token: %w", err)
		}

		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: opNode.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "opnode",
					"app.kubernetes.io/instance":   opNode.Name,
					"app.kubernetes.io/component":  "jwt-secret",
					"app.kubernetes.io/managed-by": "op-stack-operator",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"jwt": []byte(jwtToken),
			},
		}

		if err := ctrl.SetControllerReference(opNode, &secret, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, &secret)
	}

	return nil
}

// reconcileP2PSecret creates or updates the P2P private key secret
func (r *OpNodeReconciler) reconcileP2PSecret(ctx context.Context, opNode *optimismv1alpha1.OpNode) error {
	secretName := opNode.Name + "-p2p"

	var secret corev1.Secret
	key := types.NamespacedName{Name: secretName, Namespace: opNode.Namespace}

	if err := r.Get(ctx, key, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new P2P private key
		privateKey, err := generateP2PPrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate P2P private key: %w", err)
		}

		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: opNode.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "opnode",
					"app.kubernetes.io/instance":   opNode.Name,
					"app.kubernetes.io/component":  "p2p-secret",
					"app.kubernetes.io/managed-by": "op-stack-operator",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"private-key": []byte(privateKey),
			},
		}

		if err := ctrl.SetControllerReference(opNode, &secret, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, &secret)
	}

	return nil
}

// reconcileStatefulSet manages the StatefulSet for OpNode
func (r *OpNodeReconciler) reconcileStatefulSet(ctx context.Context, opNode *optimismv1alpha1.OpNode, network *optimismv1alpha1.OptimismNetwork) error {
	desiredStatefulSet := resources.CreateOpNodeStatefulSet(opNode, network)

	if err := ctrl.SetControllerReference(opNode, desiredStatefulSet, r.Scheme); err != nil {
		return err
	}

	var currentStatefulSet appsv1.StatefulSet
	key := types.NamespacedName{Name: opNode.Name, Namespace: opNode.Namespace}

	if err := r.Get(ctx, key, &currentStatefulSet); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new StatefulSet
		return r.Create(ctx, desiredStatefulSet)
	}

	// Update existing StatefulSet if needed
	currentStatefulSet.Spec = desiredStatefulSet.Spec
	currentStatefulSet.Labels = desiredStatefulSet.Labels
	currentStatefulSet.Annotations = desiredStatefulSet.Annotations

	return r.Update(ctx, &currentStatefulSet)
}

// reconcileService manages the Service for OpNode
func (r *OpNodeReconciler) reconcileService(ctx context.Context, opNode *optimismv1alpha1.OpNode, network *optimismv1alpha1.OptimismNetwork) error {
	desiredService := resources.CreateOpNodeService(opNode, network)

	if err := ctrl.SetControllerReference(opNode, desiredService, r.Scheme); err != nil {
		return err
	}

	var currentService corev1.Service
	key := types.NamespacedName{Name: opNode.Name, Namespace: opNode.Namespace}

	if err := r.Get(ctx, key, &currentService); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Create new Service
		return r.Create(ctx, desiredService)
	}

	// Update existing Service if needed
	currentService.Spec.Ports = desiredService.Spec.Ports
	currentService.Spec.Type = desiredService.Spec.Type
	currentService.Labels = desiredService.Labels
	currentService.Annotations = desiredService.Annotations

	return r.Update(ctx, &currentService)
}

// updateNodeStatus updates the node operational status
func (r *OpNodeReconciler) updateNodeStatus(_ context.Context, opNode *optimismv1alpha1.OpNode) {
	// For now, we'll set basic status information
	// In a full implementation, this would query the actual node for status

	if opNode.Status.NodeInfo == nil {
		opNode.Status.NodeInfo = &optimismv1alpha1.NodeInfo{}
	}

	// Set basic connectivity status
	opNode.Status.NodeInfo.EngineConnected = true
	opNode.Status.NodeInfo.PeerCount = 0 // Would be queried from actual node

	// Set sync status
	if opNode.Status.NodeInfo.SyncStatus == nil {
		opNode.Status.NodeInfo.SyncStatus = &optimismv1alpha1.SyncStatusInfo{}
	}
	opNode.Status.NodeInfo.SyncStatus.Syncing = false // Would be queried from actual node
}

// handleDeletion handles the deletion of OpNode resources
func (r *OpNodeReconciler) handleDeletion(ctx context.Context, opNode *optimismv1alpha1.OpNode) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Perform cleanup tasks here
	logger.Info("Cleaning up OpNode resources", "name", opNode.Name)

	// Remove finalizer
	controllerutil.RemoveFinalizer(opNode, OpNodeFinalizer)
	if err := r.Update(ctx, opNode); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// generateJWTToken generates a random JWT token for Engine API authentication
func generateJWTToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateP2PPrivateKey generates a P2P private key
func generateP2PPrivateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// updateStatusWithRetry updates the OpNode status with retry logic to handle precondition failures
func (r *OpNodeReconciler) updateStatusWithRetry(ctx context.Context, opNode *optimismv1alpha1.OpNode) error {
	// Import retry here to avoid import at top level
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the resource
		latest := &optimismv1alpha1.OpNode{}
		if err := r.Get(ctx, types.NamespacedName{Name: opNode.Name, Namespace: opNode.Namespace}, latest); err != nil {
			return err
		}

		// Copy individual status fields from opNode to latest to avoid race conditions
		latest.Status.Phase = opNode.Status.Phase
		latest.Status.ObservedGeneration = opNode.Status.ObservedGeneration

		// Deep copy conditions to avoid reference issues
		latest.Status.Conditions = make([]metav1.Condition, len(opNode.Status.Conditions))
		for i, condition := range opNode.Status.Conditions {
			latest.Status.Conditions[i] = metav1.Condition{
				Type:               condition.Type,
				Status:             condition.Status,
				Reason:             condition.Reason,
				Message:            condition.Message,
				LastTransitionTime: condition.LastTransitionTime,
				ObservedGeneration: condition.ObservedGeneration,
			}
		}

		// Deep copy NodeInfo to avoid reference issues
		if opNode.Status.NodeInfo != nil {
			latest.Status.NodeInfo = &optimismv1alpha1.NodeInfo{
				PeerCount:       opNode.Status.NodeInfo.PeerCount,
				EngineConnected: opNode.Status.NodeInfo.EngineConnected,
			}
			if opNode.Status.NodeInfo.SyncStatus != nil {
				latest.Status.NodeInfo.SyncStatus = &optimismv1alpha1.SyncStatusInfo{
					CurrentBlock: opNode.Status.NodeInfo.SyncStatus.CurrentBlock,
					HighestBlock: opNode.Status.NodeInfo.SyncStatus.HighestBlock,
					Syncing:      opNode.Status.NodeInfo.SyncStatus.Syncing,
				}
			}
			if opNode.Status.NodeInfo.ChainHead != nil {
				latest.Status.NodeInfo.ChainHead = &optimismv1alpha1.ChainHeadInfo{
					BlockNumber: opNode.Status.NodeInfo.ChainHead.BlockNumber,
					BlockHash:   opNode.Status.NodeInfo.ChainHead.BlockHash,
					Timestamp:   opNode.Status.NodeInfo.ChainHead.Timestamp,
				}
			}
		}

		return r.Status().Update(ctx, latest)
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&optimismv1alpha1.OpNode{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Named("opnode").
		Complete(r)
}
