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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/config"
	"github.com/ethereum-optimism/op-stack-operator/pkg/utils"
)

const (
	OpBatcherFinalizer = "opbatcher.optimism.io/finalizer"

	// Phase constants
	OpNodePhaseRunning = "Running"
)

// OpBatcherReconciler reconciles a OpBatcher object
type OpBatcherReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opbatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opbatchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opbatchers/finalizers,verbs=update
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=optimismnetworks,verbs=get;list;watch
// +kubebuilder:rbac:groups=optimism.optimism.io,resources=opnodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpBatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the OpBatcher instance
	var opBatcher optimismv1alpha1.OpBatcher
	if err := r.Get(ctx, req.NamespacedName, &opBatcher); err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get OpBatcher")
		return ctrl.Result{}, err
	}

	// Handle deletion with finalizers
	if opBatcher.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &opBatcher)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&opBatcher, OpBatcherFinalizer) {
		controllerutil.AddFinalizer(&opBatcher, OpBatcherFinalizer)
		if err := r.Update(ctx, &opBatcher); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate configuration
	if err := r.validateConfiguration(&opBatcher); err != nil {
		log.Error(err, "Configuration validation failed")
		utils.SetCondition(&opBatcher.Status.Conditions, "ConfigurationValid", metav1.ConditionFalse, "ValidationFailed", err.Error())
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhaseError
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	// Fetch referenced OptimismNetwork
	network, err := r.fetchOptimismNetwork(ctx, &opBatcher)
	if err != nil {
		log.Error(err, "Failed to fetch OptimismNetwork")
		utils.SetCondition(&opBatcher.Status.Conditions, "NetworkReady", metav1.ConditionFalse, "NetworkNotFound", err.Error())
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhaseError
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	// Check if network is ready
	if network.Status.Phase != optimismv1alpha1.OptimismNetworkPhaseReady {
		log.Info("OptimismNetwork is not ready yet", "network", network.Name, "phase", network.Status.Phase)
		utils.SetCondition(&opBatcher.Status.Conditions, "NetworkReady", metav1.ConditionFalse, "NetworkNotReady", "OptimismNetwork is not ready")
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhasePending
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Fetch referenced OpNode sequencer
	sequencer, err := r.fetchSequencer(ctx, &opBatcher)
	if err != nil {
		log.Error(err, "Failed to fetch sequencer OpNode")
		utils.SetCondition(&opBatcher.Status.Conditions, "SequencerReady", metav1.ConditionFalse, "SequencerNotFound", err.Error())
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhaseError
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	// Check if sequencer is ready
	if sequencer.Status.Phase != optimismv1alpha1.OpNodePhaseRunning {
		log.Info("Sequencer OpNode is not ready yet", "sequencer", sequencer.Name, "phase", sequencer.Status.Phase)
		utils.SetCondition(&opBatcher.Status.Conditions, "SequencerReady", metav1.ConditionFalse, "SequencerNotReady", "Sequencer OpNode is not ready")
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhasePending
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Validate private key secret exists
	if err := r.validatePrivateKeySecret(ctx, &opBatcher); err != nil {
		log.Error(err, "Private key secret validation failed")
		utils.SetCondition(&opBatcher.Status.Conditions, optimismv1alpha1.OpBatcherConditionPrivateKeyLoaded, metav1.ConditionFalse, "SecretNotFound", err.Error())
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhaseError
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	// Reconcile Deployment
	if err := r.reconcileDeployment(ctx, &opBatcher, network, sequencer); err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		utils.SetCondition(&opBatcher.Status.Conditions, "DeploymentReady", metav1.ConditionFalse, "DeploymentFailed", err.Error())
		opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhaseError
		r.updateStatusWithRetry(ctx, &opBatcher)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, &opBatcher); err != nil {
		log.Error(err, "Failed to reconcile Service")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Update status to Running
	utils.SetCondition(&opBatcher.Status.Conditions, "ConfigurationValid", metav1.ConditionTrue, "ValidationPassed", "Configuration is valid")
	utils.SetCondition(&opBatcher.Status.Conditions, "NetworkReady", metav1.ConditionTrue, "NetworkReady", "OptimismNetwork is ready")
	utils.SetCondition(&opBatcher.Status.Conditions, "SequencerReady", metav1.ConditionTrue, "SequencerReady", "Sequencer OpNode is ready")
	utils.SetCondition(&opBatcher.Status.Conditions, optimismv1alpha1.OpBatcherConditionPrivateKeyLoaded, metav1.ConditionTrue, "SecretFound", "Private key loaded from secret")
	utils.SetCondition(&opBatcher.Status.Conditions, optimismv1alpha1.OpBatcherConditionL1Connected, metav1.ConditionTrue, "ConnectionEstablished", "Connected to L1 RPC endpoint")
	utils.SetCondition(&opBatcher.Status.Conditions, optimismv1alpha1.OpBatcherConditionL2Connected, metav1.ConditionTrue, "SequencerReachable", "Connected to L2 sequencer")
	utils.SetCondition(&opBatcher.Status.Conditions, "DeploymentReady", metav1.ConditionTrue, "DeploymentRunning", "Deployment is running")

	opBatcher.Status.Phase = optimismv1alpha1.OpBatcherPhaseRunning
	opBatcher.Status.ObservedGeneration = opBatcher.Generation

	// Initialize batcher info if nil
	if opBatcher.Status.BatcherInfo == nil {
		opBatcher.Status.BatcherInfo = &optimismv1alpha1.OpBatcherInfo{}
	}

	r.updateStatusWithRetry(ctx, &opBatcher)

	log.Info("OpBatcher reconciliation completed successfully")
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

func (r *OpBatcherReconciler) handleDeletion(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(opBatcher, OpBatcherFinalizer) {
		// Perform cleanup tasks here if needed
		log.Info("Performing cleanup for OpBatcher", "name", opBatcher.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(opBatcher, OpBatcherFinalizer)
		if err := r.Update(ctx, opBatcher); err != nil {
			log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpBatcherReconciler) validateConfiguration(opBatcher *optimismv1alpha1.OpBatcher) error {
	if opBatcher.Spec.OptimismNetworkRef.Name == "" {
		return fmt.Errorf("optimismNetworkRef.name is required")
	}

	if opBatcher.Spec.SequencerRef.Name == "" {
		return fmt.Errorf("sequencerRef.name is required")
	}

	if opBatcher.Spec.PrivateKey.SecretRef.Name == "" {
		return fmt.Errorf("privateKey.secretRef.name is required")
	}

	if opBatcher.Spec.PrivateKey.SecretRef.Key == "" {
		return fmt.Errorf("privateKey.secretRef.key is required")
	}

	return nil
}

func (r *OpBatcherReconciler) fetchOptimismNetwork(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher) (*optimismv1alpha1.OptimismNetwork, error) {
	networkNamespace := opBatcher.Spec.OptimismNetworkRef.Namespace
	if networkNamespace == "" {
		networkNamespace = opBatcher.Namespace
	}

	var network optimismv1alpha1.OptimismNetwork
	networkKey := types.NamespacedName{
		Name:      opBatcher.Spec.OptimismNetworkRef.Name,
		Namespace: networkNamespace,
	}

	if err := r.Get(ctx, networkKey, &network); err != nil {
		return nil, fmt.Errorf("failed to get OptimismNetwork %s: %w", networkKey, err)
	}

	return &network, nil
}

func (r *OpBatcherReconciler) fetchSequencer(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher) (*optimismv1alpha1.OpNode, error) {
	sequencerNamespace := opBatcher.Spec.SequencerRef.Namespace
	if sequencerNamespace == "" {
		sequencerNamespace = opBatcher.Namespace
	}

	var sequencer optimismv1alpha1.OpNode
	sequencerKey := types.NamespacedName{
		Name:      opBatcher.Spec.SequencerRef.Name,
		Namespace: sequencerNamespace,
	}

	if err := r.Get(ctx, sequencerKey, &sequencer); err != nil {
		return nil, fmt.Errorf("failed to get sequencer OpNode %s: %w", sequencerKey, err)
	}

	// Validate that the referenced OpNode is actually a sequencer
	if sequencer.Spec.NodeType != optimismv1alpha1.OpNodeTypeSequencer {
		return nil, fmt.Errorf("referenced OpNode %s is not a sequencer (nodeType: %s)", sequencerKey, sequencer.Spec.NodeType)
	}

	return &sequencer, nil
}

func (r *OpBatcherReconciler) validatePrivateKeySecret(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher) error {
	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Name:      opBatcher.Spec.PrivateKey.SecretRef.Name,
		Namespace: opBatcher.Namespace,
	}

	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return fmt.Errorf("failed to get private key secret %s: %w", secretKey, err)
	}

	if _, exists := secret.Data[opBatcher.Spec.PrivateKey.SecretRef.Key]; !exists {
		return fmt.Errorf("private key not found in secret %s at key %s", secretKey, opBatcher.Spec.PrivateKey.SecretRef.Key)
	}

	return nil
}

func (r *OpBatcherReconciler) reconcileDeployment(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher, network *optimismv1alpha1.OptimismNetwork, sequencer *optimismv1alpha1.OpNode) error {
	deployment := r.createDeployment(opBatcher, network, sequencer)

	// Set owner reference
	if err := ctrl.SetControllerReference(opBatcher, deployment, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Create or update deployment
	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Update existing deployment
	existing.Spec = deployment.Spec
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}

func (r *OpBatcherReconciler) createDeployment(opBatcher *optimismv1alpha1.OpBatcher, network *optimismv1alpha1.OptimismNetwork, sequencer *optimismv1alpha1.OpNode) *appsv1.Deployment {
	labels := map[string]string{
		"app":       "op-batcher",
		"instance":  opBatcher.Name,
		"component": "batcher",
	}

	// Build container arguments
	args := r.buildContainerArgs(opBatcher, network, sequencer)

	// Default resource requirements
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    intstr.Parse("100m").IntVal,
			corev1.ResourceMemory: intstr.Parse("256Mi").IntVal,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    intstr.Parse("1000m").IntVal,
			corev1.ResourceMemory: intstr.Parse("2Gi").IntVal,
		},
	}

	if opBatcher.Spec.Resources != nil {
		resources = *opBatcher.Spec.Resources
	}

	// Environment variables
	env := []corev1.EnvVar{
		{
			Name: "OP_BATCHER_PRIVATE_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &opBatcher.Spec.PrivateKey.SecretRef,
			},
		},
	}

	// Container ports
	ports := []corev1.ContainerPort{}
	if opBatcher.Spec.RPC != nil && (opBatcher.Spec.RPC.Enabled == nil || *opBatcher.Spec.RPC.Enabled) {
		rpcPort := int32(8548)
		if opBatcher.Spec.RPC.Port != nil {
			rpcPort = *opBatcher.Spec.RPC.Port
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          "rpc",
			ContainerPort: rpcPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	if opBatcher.Spec.Metrics != nil && (opBatcher.Spec.Metrics.Enabled == nil || *opBatcher.Spec.Metrics.Enabled) {
		metricsPort := int32(7300)
		if opBatcher.Spec.Metrics.Port != nil {
			metricsPort = *opBatcher.Spec.Metrics.Port
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          "metrics",
			ContainerPort: metricsPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opBatcher.Name,
			Namespace: opBatcher.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{1000}[0],
						FSGroup:      &[]int64{1000}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "op-batcher",
							Image:           config.DefaultImages.OpBatcher,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            args,
							Env:             env,
							Ports:           ports,
							Resources:       resources,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &[]bool{false}[0],
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								ReadOnlyRootFilesystem: &[]bool{true}[0],
								RunAsNonRoot:           &[]bool{true}[0],
								RunAsUser:              &[]int64{1000}[0],
							},
						},
					},
				},
			},
		},
	}
}

func (r *OpBatcherReconciler) buildContainerArgs(opBatcher *optimismv1alpha1.OpBatcher, network *optimismv1alpha1.OptimismNetwork, sequencer *optimismv1alpha1.OpNode) []string {
	args := []string{"op-batcher"}

	// L1 RPC endpoint
	args = append(args, "--l1-eth-rpc", network.Spec.L1RpcUrl)

	// L2 RPC endpoint - construct from sequencer service
	l2RpcUrl := fmt.Sprintf("http://%s:8545", sequencer.Name)
	args = append(args, "--rollup-rpc", l2RpcUrl)

	// Batching configuration
	if opBatcher.Spec.Batching != nil {
		if opBatcher.Spec.Batching.MaxChannelDuration != nil {
			args = append(args, "--max-channel-duration", opBatcher.Spec.Batching.MaxChannelDuration.Duration.String())
		}
		if opBatcher.Spec.Batching.SubSafetyMargin != nil {
			args = append(args, "--sub-safety-margin", fmt.Sprintf("%d", *opBatcher.Spec.Batching.SubSafetyMargin))
		}
		if opBatcher.Spec.Batching.TargetL1TxSize != nil {
			args = append(args, "--target-l1-tx-size-bytes", fmt.Sprintf("%d", *opBatcher.Spec.Batching.TargetL1TxSize))
		}
		if opBatcher.Spec.Batching.TargetNumFrames != nil {
			args = append(args, "--target-num-frames", fmt.Sprintf("%d", *opBatcher.Spec.Batching.TargetNumFrames))
		}
		if opBatcher.Spec.Batching.ApproxComprRatio != nil {
			args = append(args, "--approx-compr-ratio", *opBatcher.Spec.Batching.ApproxComprRatio)
		}
	}

	// Data availability configuration
	if opBatcher.Spec.DataAvailability != nil {
		if opBatcher.Spec.DataAvailability.Type != "" {
			if opBatcher.Spec.DataAvailability.Type == "blobs" {
				args = append(args, "--data-availability-type", "blobs")
				if opBatcher.Spec.DataAvailability.MaxBlobsPerTx != nil {
					args = append(args, "--max-blobs-per-tx", fmt.Sprintf("%d", *opBatcher.Spec.DataAvailability.MaxBlobsPerTx))
				}
			} else {
				args = append(args, "--data-availability-type", "calldata")
			}
		}
	}

	// Throttling configuration
	if opBatcher.Spec.Throttling != nil {
		if opBatcher.Spec.Throttling.MaxPendingTx != nil {
			args = append(args, "--max-pending-tx", fmt.Sprintf("%d", *opBatcher.Spec.Throttling.MaxPendingTx))
		}
	}

	// L1 transaction configuration
	if opBatcher.Spec.L1Transaction != nil {
		if opBatcher.Spec.L1Transaction.FeeLimitMultiplier != nil {
			args = append(args, "--fee-limit-multiplier", fmt.Sprintf("%d", *opBatcher.Spec.L1Transaction.FeeLimitMultiplier))
		}
		if opBatcher.Spec.L1Transaction.ResubmissionTimeout != nil {
			args = append(args, "--resubmission-timeout", opBatcher.Spec.L1Transaction.ResubmissionTimeout.Duration.String())
		}
		if opBatcher.Spec.L1Transaction.NumConfirmations != nil {
			args = append(args, "--num-confirmations", fmt.Sprintf("%d", *opBatcher.Spec.L1Transaction.NumConfirmations))
		}
	}

	// RPC configuration
	if opBatcher.Spec.RPC != nil && (opBatcher.Spec.RPC.Enabled == nil || *opBatcher.Spec.RPC.Enabled) {
		rpcHost := "127.0.0.1"
		if opBatcher.Spec.RPC.Host != "" {
			rpcHost = opBatcher.Spec.RPC.Host
		}
		rpcPort := int32(8548)
		if opBatcher.Spec.RPC.Port != nil {
			rpcPort = *opBatcher.Spec.RPC.Port
		}
		args = append(args, "--rpc.addr", rpcHost)
		args = append(args, "--rpc.port", fmt.Sprintf("%d", rpcPort))

		if opBatcher.Spec.RPC.EnableAdmin != nil && *opBatcher.Spec.RPC.EnableAdmin {
			args = append(args, "--rpc.enable-admin")
		}
	}

	// Metrics configuration
	if opBatcher.Spec.Metrics != nil && (opBatcher.Spec.Metrics.Enabled == nil || *opBatcher.Spec.Metrics.Enabled) {
		metricsHost := "0.0.0.0"
		if opBatcher.Spec.Metrics.Host != "" {
			metricsHost = opBatcher.Spec.Metrics.Host
		}
		metricsPort := int32(7300)
		if opBatcher.Spec.Metrics.Port != nil {
			metricsPort = *opBatcher.Spec.Metrics.Port
		}
		args = append(args, "--metrics.addr", metricsHost)
		args = append(args, "--metrics.port", fmt.Sprintf("%d", metricsPort))
		args = append(args, "--metrics.enabled")
	}

	return args
}

func (r *OpBatcherReconciler) reconcileService(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher) error {
	service := r.createService(opBatcher)

	// Set owner reference
	if err := ctrl.SetControllerReference(opBatcher, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Create or update service
	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, service); err != nil {
				return fmt.Errorf("failed to create service: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get service: %w", err)
	}

	// Update existing service (preserve ClusterIP)
	existing.Spec.Ports = service.Spec.Ports
	existing.Spec.Selector = service.Spec.Selector
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	return nil
}

func (r *OpBatcherReconciler) createService(opBatcher *optimismv1alpha1.OpBatcher) *corev1.Service {
	labels := map[string]string{
		"app":       "op-batcher",
		"instance":  opBatcher.Name,
		"component": "batcher",
	}

	ports := []corev1.ServicePort{}

	// RPC port
	if opBatcher.Spec.RPC != nil && (opBatcher.Spec.RPC.Enabled == nil || *opBatcher.Spec.RPC.Enabled) {
		rpcPort := int32(8548)
		if opBatcher.Spec.RPC.Port != nil {
			rpcPort = *opBatcher.Spec.RPC.Port
		}
		ports = append(ports, corev1.ServicePort{
			Name:       "rpc",
			Port:       rpcPort,
			TargetPort: intstr.FromInt32(rpcPort),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	// Metrics port
	if opBatcher.Spec.Metrics != nil && (opBatcher.Spec.Metrics.Enabled == nil || *opBatcher.Spec.Metrics.Enabled) {
		metricsPort := int32(7300)
		if opBatcher.Spec.Metrics.Port != nil {
			metricsPort = *opBatcher.Spec.Metrics.Port
		}
		ports = append(ports, corev1.ServicePort{
			Name:       "metrics",
			Port:       metricsPort,
			TargetPort: intstr.FromInt32(metricsPort),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opBatcher.Name,
			Namespace: opBatcher.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports:    ports,
		},
	}
}

func (r *OpBatcherReconciler) updateStatusWithRetry(ctx context.Context, opBatcher *optimismv1alpha1.OpBatcher) error {
	return utils.RetryStatusUpdate(ctx, r.Client, opBatcher, func() error {
		return r.Status().Update(ctx, opBatcher)
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpBatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&optimismv1alpha1.OpBatcher{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("opbatcher").
		Complete(r)
}
