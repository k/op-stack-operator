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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("OpBatcher Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-opbatcher"
		const networkName = "test-network"
		const sequencerName = "test-sequencer"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		networkNamespacedName := types.NamespacedName{
			Name:      networkName,
			Namespace: "default",
		}

		sequencerNamespacedName := types.NamespacedName{
			Name:      sequencerName,
			Namespace: "default",
		}

		opbatcher := &optimismv1alpha1.OpBatcher{}
		network := &optimismv1alpha1.OptimismNetwork{}
		sequencer := &optimismv1alpha1.OpNode{}

		BeforeEach(func() {
			By("Creating the OptimismNetwork")
			network = &optimismv1alpha1.OptimismNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      networkName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "test-network",
					ChainID:     10,
					L1ChainID:   1,
					L1RpcUrl:    "https://ethereum-sepolia-rpc.publicnode.com",
					SharedConfig: &optimismv1alpha1.SharedConfig{
						Logging: &optimismv1alpha1.LoggingConfig{
							Level:  "info",
							Format: "logfmt",
							Color:  false,
						},
						Metrics: &optimismv1alpha1.MetricsConfig{
							Enabled: true,
							Port:    7300,
						},
						Resources: &optimismv1alpha1.ResourceConfig{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				},
				Status: optimismv1alpha1.OptimismNetworkStatus{
					Phase: "Ready",
					Conditions: []metav1.Condition{
						{
							Type:   "ConfigurationValid",
							Status: metav1.ConditionTrue,
							Reason: "ValidConfiguration",
						},
						{
							Type:   "L1Connected",
							Status: metav1.ConditionTrue,
							Reason: "RPCEndpointReachable",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, network)).To(Succeed())

			By("Creating the sequencer OpNode")
			sequencer = &optimismv1alpha1.OpNode{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sequencerName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpNodeSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: networkName,
					},
					NodeType: "sequencer",
					OpNode: optimismv1alpha1.OpNodeConfig{
						Sequencer: &optimismv1alpha1.SequencerConfig{
							Enabled: true,
						},
					},
				},
				Status: optimismv1alpha1.OpNodeStatus{
					Phase: OpNodePhaseRunning,
					Conditions: []metav1.Condition{
						{
							Type:   "ConfigurationValid",
							Status: metav1.ConditionTrue,
							Reason: "ValidConfiguration",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sequencer)).To(Succeed())

			By("Creating the private key secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "batcher-private-key",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"private-key": []byte("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		})

		AfterEach(func() {
			// Clean up resources
			resource := &optimismv1alpha1.OpBatcher{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, sequencerNamespacedName, sequencer)
			if err == nil {
				Expect(k8sClient.Delete(ctx, sequencer)).To(Succeed())
			}

			err = k8sClient.Get(ctx, networkNamespacedName, network)
			if err == nil {
				Expect(k8sClient.Delete(ctx, network)).To(Succeed())
			}

			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "batcher-private-key", Namespace: "default"}, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Creating a new OpBatcher")
			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: networkName,
					},
					SequencerRef: &optimismv1alpha1.SequencerReference{
						Name: sequencerName,
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "batcher-private-key",
							},
							Key: "private-key",
						},
					},
					Batching: &optimismv1alpha1.BatchingConfig{
						MaxChannelDuration: "10m",
						SubSafetyMargin:    10,
						TargetL1TxSize:     120000,
						TargetNumFrames:    1,
						ApproxComprRatio:   "0.4",
					},
					DataAvailability: &optimismv1alpha1.DataAvailabilityConfig{
						Type:          "blobs",
						MaxBlobsPerTx: 6,
					},
					RPC: &optimismv1alpha1.RPCConfig{
						Enabled: true,
						Host:    "127.0.0.1",
						Port:    8548,
					},
					Metrics: &optimismv1alpha1.MetricsConfig{
						Enabled: true,
						Host:    "0.0.0.0",
						Port:    7300,
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that OpBatcher was updated with proper conditions")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, opbatcher)
			}, timeout, interval).Should(Succeed())

			// Should have configuration valid condition
			Expect(opbatcher.Status.Conditions).To(ContainElement(HaveField("Type", "ConfigurationValid")))
			Expect(opbatcher.Status.Conditions).To(ContainElement(HaveField("Type", "NetworkReference")))
		})

		It("should create a Deployment", func() {
			By("Creating a new OpBatcher")
			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: networkName,
					},
					SequencerRef: &optimismv1alpha1.SequencerReference{
						Name: sequencerName,
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "batcher-private-key",
							},
							Key: "private-key",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that a Deployment was created")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, deployment)
			}, timeout, interval).Should(Succeed())

			Expect(deployment.Name).To(Equal(resourceName))
			Expect(deployment.Namespace).To(Equal("default"))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("op-batcher"))
		})

		It("should create a Service", func() {
			By("Creating a new OpBatcher")
			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: networkName,
					},
					SequencerRef: &optimismv1alpha1.SequencerReference{
						Name: sequencerName,
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "batcher-private-key",
							},
							Key: "private-key",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that a Service was created")
			service := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, service)
			}, timeout, interval).Should(Succeed())

			Expect(service.Name).To(Equal(resourceName))
			Expect(service.Namespace).To(Equal("default"))
			Expect(service.Spec.Ports).NotTo(BeEmpty())
		})

		It("should handle validation errors gracefully", func() {
			By("Creating an OpBatcher with invalid configuration")
			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "", // Invalid: empty network name
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "nonexistent-secret",
							},
							Key: "private-key",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred()) // Should not error, but should set error conditions

			By("Checking that OpBatcher has error condition")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, opbatcher)
			}, timeout, interval).Should(Succeed())

			Expect(opbatcher.Status.Phase).To(Equal(OpBatcherPhaseError))
			configValid := false
			for _, condition := range opbatcher.Status.Conditions {
				if condition.Type == "ConfigurationValid" && condition.Status == metav1.ConditionFalse {
					configValid = true
					break
				}
			}
			Expect(configValid).To(BeTrue())
		})

		It("should handle missing OptimismNetwork gracefully", func() {
			By("Creating an OpBatcher with nonexistent network reference")
			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "nonexistent-network",
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "batcher-private-key",
							},
							Key: "private-key",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that OpBatcher has network error condition")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, opbatcher)
			}, timeout, interval).Should(Succeed())

			Expect(opbatcher.Status.Phase).To(Equal(OpBatcherPhaseError))
			networkRef := false
			for _, condition := range opbatcher.Status.Conditions {
				if condition.Type == "NetworkReference" && condition.Status == metav1.ConditionFalse {
					networkRef = true
					break
				}
			}
			Expect(networkRef).To(BeTrue())
		})

		It("should wait for OptimismNetwork to be ready", func() {
			By("Creating an OpBatcher with network that's not ready")
			// Update network to not be ready
			Expect(k8sClient.Get(ctx, networkNamespacedName, network)).To(Succeed())
			network.Status.Phase = "Pending"
			Expect(k8sClient.Status().Update(ctx, network)).To(Succeed())

			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: networkName,
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "batcher-private-key",
							},
							Key: "private-key",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that OpBatcher is pending")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, opbatcher)
			}, timeout, interval).Should(Succeed())

			Expect(opbatcher.Status.Phase).To(Equal(OpBatcherPhasePending))
			networkReady := false
			for _, condition := range opbatcher.Status.Conditions {
				if condition.Type == "NetworkReady" && condition.Status == metav1.ConditionFalse {
					networkReady = true
					break
				}
			}
			Expect(networkReady).To(BeTrue())
		})

		It("should handle finalizer correctly", func() {
			By("Creating a new OpBatcher")
			opbatcher = &optimismv1alpha1.OpBatcher{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: networkName,
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "batcher-private-key",
							},
							Key: "private-key",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, opbatcher)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &OpBatcherReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that finalizer was added")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, opbatcher)
			}, timeout, interval).Should(Succeed())

			found := false
			for _, finalizer := range opbatcher.Finalizers {
				if finalizer == OpBatcherFinalizer {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())

			By("Deleting the OpBatcher")
			Expect(k8sClient.Delete(ctx, opbatcher)).To(Succeed())

			By("Reconciling deletion")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that resource was deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, opbatcher)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Configuration validation", func() {
		var reconciler *OpBatcherReconciler

		BeforeEach(func() {
			reconciler = &OpBatcherReconciler{}
		})

		It("should validate required fields", func() {
			opbatcher := &optimismv1alpha1.OpBatcher{
				Spec: optimismv1alpha1.OpBatcherSpec{
					// Missing OptimismNetworkRef
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-secret",
							},
							Key: "private-key",
						},
					},
				},
			}

			err := reconciler.validateConfiguration(opbatcher)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("optimismNetworkRef.name is required"))
		})

		It("should validate private key configuration", func() {
			opbatcher := &optimismv1alpha1.OpBatcher{
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "test-network",
					},
					// Missing PrivateKey
				},
			}

			err := reconciler.validateConfiguration(opbatcher)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("privateKey.secretRef is required"))
		})

		It("should validate batching configuration", func() {
			opbatcher := &optimismv1alpha1.OpBatcher{
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "test-network",
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-secret",
							},
							Key: "private-key",
						},
					},
					Batching: &optimismv1alpha1.BatchingConfig{
						TargetL1TxSize: 500, // Too small
					},
				},
			}

			err := reconciler.validateConfiguration(opbatcher)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("batching.targetL1TxSize must be at least 1000 bytes"))
		})

		It("should validate data availability configuration", func() {
			opbatcher := &optimismv1alpha1.OpBatcher{
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "test-network",
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-secret",
							},
							Key: "private-key",
						},
					},
					DataAvailability: &optimismv1alpha1.DataAvailabilityConfig{
						Type: "invalid-type",
					},
				},
			}

			err := reconciler.validateConfiguration(opbatcher)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dataAvailability.type must be 'blobs' or 'calldata'"))
		})

		It("should accept valid configuration", func() {
			opbatcher := &optimismv1alpha1.OpBatcher{
				Spec: optimismv1alpha1.OpBatcherSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "test-network",
					},
					PrivateKey: optimismv1alpha1.SecretKeyRef{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-secret",
							},
							Key: "private-key",
						},
					},
					Batching: &optimismv1alpha1.BatchingConfig{
						MaxChannelDuration: "10m",
						SubSafetyMargin:    10,
						TargetL1TxSize:     120000,
						TargetNumFrames:    1,
						ApproxComprRatio:   "0.4",
					},
					DataAvailability: &optimismv1alpha1.DataAvailabilityConfig{
						Type:          "blobs",
						MaxBlobsPerTx: 6,
					},
				},
			}

			err := reconciler.validateConfiguration(opbatcher)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
