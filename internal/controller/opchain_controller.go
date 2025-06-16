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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rollupv1alpha1 "github.com/oplabs/opstack-operator/api/v1alpha1"
	"github.com/oplabs/opstack-operator/pkg/manifests"
	"github.com/oplabs/opstack-operator/pkg/opdeployer"
)

const (
	// FinalizerName is the finalizer used for OPChain resources
	FinalizerName = "rollup.oplabs.io/opchain"

	// ConditionL1ContractsDeployed indicates L1 contracts are deployed
	ConditionL1ContractsDeployed = "L1ContractsDeployed"

	// ConditionL2NodesReady indicates L2 nodes are ready
	ConditionL2NodesReady = "L2NodesReady"

	// DefaultOpDeployerPath is the default path to the op-deployer binary
	DefaultOpDeployerPath = "/usr/local/bin/op-deployer"
)

// OPChainReconciler reconciles a OPChain object
type OPChainReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OpDeployerPath string
	ManifestGen    *manifests.Generator
}

// +kubebuilder:rbac:groups=rollup.oplabs.io,resources=opchains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rollup.oplabs.io,resources=opchains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rollup.oplabs.io,resources=opchains/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OPChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch OPChain resource
	opchain := &rollupv1alpha1.OPChain{}
	if err := r.Get(ctx, req.NamespacedName, opchain); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("OPChain resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get OPChain resource")
		return ctrl.Result{}, err
	}

	// 2. Handle deletion
	if !opchain.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, opchain)
	}

	// 3. Ensure finalizer
	if !controllerutil.ContainsFinalizer(opchain, FinalizerName) {
		controllerutil.AddFinalizer(opchain, FinalizerName)
		return ctrl.Result{}, r.Update(ctx, opchain)
	}

	// 4. Initialize status if needed
	if opchain.Status.Phase == "" {
		opchain.Status.Phase = rollupv1alpha1.OPChainPhasePending
		opchain.Status.ObservedGeneration = opchain.Generation
		return r.updateStatus(ctx, opchain)
	}

	// 5. Deploy L1 contracts if needed
	if !r.isL1Deployed(opchain) {
		return r.deployL1Contracts(ctx, opchain)
	}

	// 6. Deploy L2 components if contracts are deployed
	if !r.areL2ComponentsReady(opchain) {
		return r.deployL2Components(ctx, opchain)
	}

	// 7. Update status and schedule next reconcile
	return r.updateStatus(ctx, opchain)
}

// handleDeletion handles cleanup when OPChain is being deleted
func (r *OPChainReconciler) handleDeletion(ctx context.Context, opchain *rollupv1alpha1.OPChain) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// TODO: Implement cleanup logic
	// - Delete generated Kubernetes resources
	// - Clean up op-deployer work directory
	// - Remove any external dependencies

	logger.Info("Cleaning up OPChain resources", "name", opchain.Name)

	controllerutil.RemoveFinalizer(opchain, FinalizerName)
	return ctrl.Result{}, r.Update(ctx, opchain)
}

// isL1Deployed checks if L1 contracts are deployed
func (r *OPChainReconciler) isL1Deployed(opchain *rollupv1alpha1.OPChain) bool {
	for _, condition := range opchain.Status.Conditions {
		if condition.Type == ConditionL1ContractsDeployed && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// areL2ComponentsReady checks if L2 components are ready
func (r *OPChainReconciler) areL2ComponentsReady(opchain *rollupv1alpha1.OPChain) bool {
	for _, condition := range opchain.Status.Conditions {
		if condition.Type == ConditionL2NodesReady && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// deployL1Contracts deploys L1 contracts using op-deployer
func (r *OPChainReconciler) deployL1Contracts(ctx context.Context, opchain *rollupv1alpha1.OPChain) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deploying L1 contracts", "name", opchain.Name)

	// Update status to deploying
	opchain.Status.Phase = rollupv1alpha1.OPChainPhaseDeploying
	if _, err := r.updateStatus(ctx, opchain); err != nil {
		return ctrl.Result{}, err
	}

	// Get deployer private key from secret
	deployerKey, err := r.getSecretValue(ctx, opchain.Namespace, opchain.Spec.L1.DeployerPrivateKeySecret, "key")
	if err != nil {
		r.setCondition(opchain, ConditionL1ContractsDeployed, metav1.ConditionFalse, "SecretNotFound", err.Error())
		opchain.Status.Phase = rollupv1alpha1.OPChainPhaseError
		_, _ = r.updateStatus(ctx, opchain)
		return ctrl.Result{RequeueAfter: time.Minute * 5}, err
	}

	// Create op-deployer client
	workDir := fmt.Sprintf("/tmp/opchain-%s-%s", opchain.Namespace, opchain.Name)
	opDeployerClient := opdeployer.NewClient(r.OpDeployerPath, workDir)

	// Deploy L1 contracts
	deployConfig := &opdeployer.IntentConfig{
		L1ChainID:          opchain.Spec.L1.ChainID,
		L2ChainID:          opchain.Spec.ChainID,
		L1RpcUrl:           opchain.Spec.L1.RpcUrl,
		DeployerPrivateKey: deployerKey,
	}

	result, err := opDeployerClient.DeployL1Contracts(ctx, deployConfig)
	if err != nil {
		logger.Error(err, "Failed to deploy L1 contracts")
		r.setCondition(opchain, ConditionL1ContractsDeployed, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		opchain.Status.Phase = rollupv1alpha1.OPChainPhaseError
		_, _ = r.updateStatus(ctx, opchain)
		return ctrl.Result{RequeueAfter: time.Minute * 5}, err
	}

	// Store contract addresses and config
	opchain.Status.ContractAddresses = result.ContractAddresses
	r.setCondition(opchain, ConditionL1ContractsDeployed, metav1.ConditionTrue, "DeploymentSuccessful", "L1 contracts deployed successfully")

	// Create config secrets for L2 components
	if err := r.createConfigSecrets(ctx, opchain, result); err != nil {
		logger.Error(err, "Failed to create config secrets")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	logger.Info("L1 contracts deployed successfully", "name", opchain.Name)
	return r.updateStatus(ctx, opchain)
}

// deployL2Components deploys L2 components using manifest generation
func (r *OPChainReconciler) deployL2Components(ctx context.Context, opchain *rollupv1alpha1.OPChain) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deploying L2 components", "name", opchain.Name)

	// Get configuration from secrets
	genesis, err := r.getSecretValue(ctx, opchain.Namespace, fmt.Sprintf("%s-genesis", opchain.Name), "genesis.json")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	rollupConfig, err := r.getSecretValue(ctx, opchain.Namespace, fmt.Sprintf("%s-rollup", opchain.Name), "rollup.json")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	jwtSecret, err := r.getSecretValue(ctx, opchain.Namespace, fmt.Sprintf("%s-jwt", opchain.Name), "jwt")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Generate manifests
	config := &manifests.Config{
		Genesis:           genesis,
		RollupConfig:      rollupConfig,
		JWTSecret:         jwtSecret,
		ContractAddresses: opchain.Status.ContractAddresses,
	}

	manifests, err := r.ManifestGen.GenerateManifests(opchain, config)
	if err != nil {
		logger.Error(err, "Failed to generate manifests")
		r.setCondition(opchain, ConditionL2NodesReady, metav1.ConditionFalse, "ManifestGenerationFailed", err.Error())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Apply manifests
	for _, manifest := range manifests {
		if err := ctrl.SetControllerReference(opchain, manifest, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference")
			continue
		}

		if err := r.Create(ctx, manifest); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				logger.Error(err, "Failed to create manifest", "kind", manifest.GetObjectKind())
				continue
			}
			// Update existing resource
			if err := r.Update(ctx, manifest); err != nil {
				logger.Error(err, "Failed to update manifest", "kind", manifest.GetObjectKind())
			}
		}
	}

	// Update status
	r.setCondition(opchain, ConditionL2NodesReady, metav1.ConditionTrue, "ComponentsDeployed", "L2 components deployed successfully")
	opchain.Status.Phase = rollupv1alpha1.OPChainPhaseRunning

	logger.Info("L2 components deployed successfully", "name", opchain.Name)
	return r.updateStatus(ctx, opchain)
}

// createConfigSecrets creates secrets containing genesis, rollup config, and JWT
func (r *OPChainReconciler) createConfigSecrets(ctx context.Context, opchain *rollupv1alpha1.OPChain, result *opdeployer.DeploymentResult) error {
	// Create genesis secret
	genesisSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-genesis", opchain.Name),
			Namespace: opchain.Namespace,
		},
		Data: map[string][]byte{
			"genesis.json": []byte(result.Genesis),
		},
	}
	if err := ctrl.SetControllerReference(opchain, genesisSecret, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, genesisSecret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// Create rollup config secret
	rollupSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rollup", opchain.Name),
			Namespace: opchain.Namespace,
		},
		Data: map[string][]byte{
			"rollup.json": []byte(result.RollupConfig),
		},
	}
	if err := ctrl.SetControllerReference(opchain, rollupSecret, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, rollupSecret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// Create JWT secret
	jwtToken, err := generateJWTSecret()
	if err != nil {
		return err
	}

	jwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-jwt", opchain.Name),
			Namespace: opchain.Namespace,
		},
		Data: map[string][]byte{
			"jwt": []byte(jwtToken),
		},
	}
	if err := ctrl.SetControllerReference(opchain, jwtSecret, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, jwtSecret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// getSecretValue retrieves a value from a Kubernetes secret
func (r *OPChainReconciler) getSecretValue(ctx context.Context, namespace, secretName, key string) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	value, exists := secret.Data[key]
	if !exists {
		return "", fmt.Errorf("key %s not found in secret %s", key, secretName)
	}

	return string(value), nil
}

// setCondition sets a condition on the OPChain status
func (r *OPChainReconciler) setCondition(opchain *rollupv1alpha1.OPChain, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	for i, existingCondition := range opchain.Status.Conditions {
		if existingCondition.Type == conditionType {
			opchain.Status.Conditions[i] = condition
			return
		}
	}
	opchain.Status.Conditions = append(opchain.Status.Conditions, condition)
}

// updateStatus updates the OPChain status
func (r *OPChainReconciler) updateStatus(ctx context.Context, opchain *rollupv1alpha1.OPChain) (ctrl.Result, error) {
	opchain.Status.ObservedGeneration = opchain.Generation
	if err := r.Status().Update(ctx, opchain); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// generateJWTSecret generates a random JWT secret
func generateJWTSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OPChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize manifest generator if not provided
	if r.ManifestGen == nil {
		gen, err := manifests.NewGenerator()
		if err != nil {
			return fmt.Errorf("failed to create manifest generator: %w", err)
		}
		r.ManifestGen = gen
	}

	// Set default op-deployer path if not provided
	if r.OpDeployerPath == "" {
		r.OpDeployerPath = DefaultOpDeployerPath
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rollupv1alpha1.OPChain{}).
		Owns(&corev1.Secret{}).
		Named("opchain").
		Complete(r)
}
