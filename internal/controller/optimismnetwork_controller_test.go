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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/discovery"
	"github.com/ethereum-optimism/op-stack-operator/pkg/utils"
)

var _ = Describe("OptimismNetwork Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			OptimismNetworkName      = "test-network"
			OptimismNetworkNamespace = "default"

			timeout  = time.Second * 10
			duration = time.Second * 10
			interval = time.Millisecond * 250
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      OptimismNetworkName,
			Namespace: OptimismNetworkNamespace,
		}

		optimismnetwork := &optimismv1alpha1.OptimismNetwork{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind OptimismNetwork")
			err := k8sClient.Get(ctx, typeNamespacedName, optimismnetwork)
			if err != nil && !errors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			if errors.IsNotFound(err) {
				resource := &optimismv1alpha1.OptimismNetwork{
					ObjectMeta: metav1.ObjectMeta{
						Name:      OptimismNetworkName,
						Namespace: OptimismNetworkNamespace,
					},
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						NetworkName: "op-sepolia",
						ChainID:     11155420,
						L1ChainID:   11155111,
						L1RpcUrl:    "https://sepolia.infura.io/v3/test-key",
						L1BeaconUrl: "https://sepolia-beacon.example.com",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Clean up the resource after each test
			resource := &optimismv1alpha1.OptimismNetwork{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &OptimismNetworkReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				DiscoveryService: discovery.NewContractDiscoveryService(24 * time.Hour),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the resource status is set correctly")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, optimismnetwork)
				if err != nil {
					return false
				}
				// Check if status has been initialized
				return optimismnetwork.Status.Phase != ""
			}, timeout, interval).Should(BeTrue())

			By("Checking if finalizer was added")
			Expect(optimismnetwork.Finalizers).To(ContainElement(OptimismNetworkFinalizer))

			By("Checking if status conditions are set")
			Expect(optimismnetwork.Status.Conditions).NotTo(BeEmpty())
		})
	})

	Context("When validating configuration", func() {
		var reconciler *OptimismNetworkReconciler

		BeforeEach(func() {
			reconciler = &OptimismNetworkReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				DiscoveryService: discovery.NewContractDiscoveryService(24 * time.Hour),
			}
		})

		It("should reject configuration with missing chainID", func() {
			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					L1ChainID: 1,
					L1RpcUrl:  "https://mainnet.infura.io/v3/test",
				},
			}

			err := reconciler.validateConfiguration(context.Background(), network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chainID is required"))
		})

		It("should reject configuration with missing L1ChainID", func() {
			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:  10,
					L1RpcUrl: "https://mainnet.infura.io/v3/test",
				},
			}

			err := reconciler.validateConfiguration(context.Background(), network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("l1ChainID is required"))
		})

		It("should reject configuration with missing L1RpcUrl", func() {
			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   10,
					L1ChainID: 1,
				},
			}

			err := reconciler.validateConfiguration(context.Background(), network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("l1RpcUrl is required"))
		})

		It("should reject configuration with same chainID and L1ChainID", func() {
			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   1,
					L1ChainID: 1,
					L1RpcUrl:  "https://mainnet.infura.io/v3/test",
				},
			}

			err := reconciler.validateConfiguration(context.Background(), network)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chainID and l1ChainID cannot be the same"))
		})

		It("should accept valid configuration", func() {
			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   10,
					L1ChainID: 1,
					L1RpcUrl:  "https://mainnet.infura.io/v3/test",
				},
			}

			err := reconciler.validateConfiguration(context.Background(), network)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When handling ConfigMap references", func() {
		var reconciler *OptimismNetworkReconciler
		const testNamespace = "test-configmap"

		BeforeEach(func() {
			reconciler = &OptimismNetworkReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				DiscoveryService: discovery.NewContractDiscoveryService(24 * time.Hour),
			}

			// Create test namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			}
			_ = k8sClient.Create(context.Background(), ns)

			// Create test ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: testNamespace,
				},
				Data: map[string]string{
					"rollup.json": `{"test": "config"}`,
				},
			}
			Expect(k8sClient.Create(context.Background(), configMap)).To(Succeed())
		})

		AfterEach(func() {
			// Clean up test namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			}
			_ = k8sClient.Delete(context.Background(), ns)
		})

		It("should validate existing ConfigMap reference", func() {
			source := &optimismv1alpha1.ConfigSource{
				ConfigMapRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-config",
					},
					Key: "rollup.json",
				},
			}

			err := reconciler.validateConfigSource(context.Background(), source, testNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject non-existent ConfigMap reference", func() {
			source := &optimismv1alpha1.ConfigSource{
				ConfigMapRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "non-existent-config",
					},
				},
			}

			err := reconciler.validateConfigSource(context.Background(), source, testNamespace)
			Expect(err).To(HaveOccurred())
		})

		It("should reject non-existent key in ConfigMap", func() {
			source := &optimismv1alpha1.ConfigSource{
				ConfigMapRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-config",
					},
					Key: "non-existent-key",
				},
			}

			err := reconciler.validateConfigSource(context.Background(), source, testNamespace)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When managing status conditions", func() {
		It("should set condition correctly", func() {
			network := &optimismv1alpha1.OptimismNetwork{}

			// Set a condition
			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionConfigurationValid, utils.ReasonValidConfiguration, "Test message")

			// Verify condition was set
			Expect(network.Status.Conditions).To(HaveLen(1))
			condition := utils.GetCondition(network.Status.Conditions, utils.ConditionConfigurationValid)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal(utils.ReasonValidConfiguration))
			Expect(condition.Message).To(Equal("Test message"))
		})

		It("should update existing condition", func() {
			network := &optimismv1alpha1.OptimismNetwork{}

			// Set initial condition
			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionConfigurationValid, utils.ReasonValidConfiguration, "Initial message")

			// Update condition
			utils.SetConditionFalse(&network.Status.Conditions,
				utils.ConditionConfigurationValid, utils.ReasonInvalidConfiguration, "Updated message")

			// Verify condition was updated, not duplicated
			Expect(network.Status.Conditions).To(HaveLen(1))
			condition := utils.GetCondition(network.Status.Conditions, utils.ConditionConfigurationValid)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal(utils.ReasonInvalidConfiguration))
			Expect(condition.Message).To(Equal("Updated message"))
		})
	})

	Context("When testing contract discovery", func() {
		It("should discover well-known addresses for op-mainnet", func() {
			discoveryService := discovery.NewContractDiscoveryService(24 * time.Hour)

			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "op-mainnet",
					ChainID:     10,
					L1ChainID:   1,
					L1RpcUrl:    "https://mainnet.infura.io/v3/test",
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
					},
				},
			}

			addresses, err := discoveryService.DiscoverContracts(context.Background(), network)
			Expect(err).NotTo(HaveOccurred())
			Expect(addresses).NotTo(BeNil())
			Expect(addresses.L2OutputOracleAddr).NotTo(BeEmpty())
			Expect(addresses.SystemConfigAddr).NotTo(BeEmpty())
			Expect(addresses.DiscoveryMethod).To(Equal("well-known"))
		})

		It("should discover well-known addresses for op-sepolia", func() {
			discoveryService := discovery.NewContractDiscoveryService(24 * time.Hour)

			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "op-sepolia",
					ChainID:     11155420,
					L1ChainID:   11155111,
					L1RpcUrl:    "https://sepolia.infura.io/v3/test",
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
					},
				},
			}

			addresses, err := discoveryService.DiscoverContracts(context.Background(), network)
			Expect(err).NotTo(HaveOccurred())
			Expect(addresses).NotTo(BeNil())
			Expect(addresses.L2OutputOracleAddr).NotTo(BeEmpty())
			Expect(addresses.DiscoveryMethod).To(Equal("well-known"))
		})

		It("should use manual addresses when provided", func() {
			discoveryService := discovery.NewContractDiscoveryService(24 * time.Hour)

			network := &optimismv1alpha1.OptimismNetwork{
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "custom-network",
					ChainID:     999,
					L1ChainID:   1,
					L1RpcUrl:    "https://mainnet.infura.io/v3/test",
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod:    "manual",
						SystemConfigAddr:   "0x1234567890123456789012345678901234567890",
						L2OutputOracleAddr: "0x0987654321098765432109876543210987654321",
					},
				},
			}

			addresses, err := discoveryService.DiscoverContracts(context.Background(), network)
			Expect(err).NotTo(HaveOccurred())
			Expect(addresses).NotTo(BeNil())
			Expect(addresses.SystemConfigAddr).To(Equal("0x1234567890123456789012345678901234567890"))
			Expect(addresses.L2OutputOracleAddr).To(Equal("0x0987654321098765432109876543210987654321"))
			Expect(addresses.DiscoveryMethod).To(Equal("manual"))
		})
	})

	Context("When updating phase", func() {
		var reconciler *OptimismNetworkReconciler

		BeforeEach(func() {
			reconciler = &OptimismNetworkReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				DiscoveryService: discovery.NewContractDiscoveryService(24 * time.Hour),
			}
		})

		It("should set phase to Ready when all conditions are True", func() {
			network := &optimismv1alpha1.OptimismNetwork{}

			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionConfigurationValid, utils.ReasonValidConfiguration, "Valid")
			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionL1Connected, utils.ReasonRPCEndpointReachable, "Connected")
			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionContractsDiscovered, utils.ReasonAddressesResolved, "Discovered")

			reconciler.updatePhase(network)
			Expect(network.Status.Phase).To(Equal("Ready"))
		})

		It("should set phase to Error when any condition is False", func() {
			network := &optimismv1alpha1.OptimismNetwork{}

			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionConfigurationValid, utils.ReasonValidConfiguration, "Valid")
			utils.SetConditionFalse(&network.Status.Conditions,
				utils.ConditionL1Connected, utils.ReasonRPCEndpointUnreachable, "Failed")

			reconciler.updatePhase(network)
			Expect(network.Status.Phase).To(Equal("Error"))
		})

		It("should set phase to Pending when conditions are not all True", func() {
			network := &optimismv1alpha1.OptimismNetwork{}

			utils.SetConditionTrue(&network.Status.Conditions,
				utils.ConditionConfigurationValid, utils.ReasonValidConfiguration, "Valid")
			utils.SetConditionUnknown(&network.Status.Conditions,
				utils.ConditionL1Connected, "Testing", "Testing connection")

			reconciler.updatePhase(network)
			Expect(network.Status.Phase).To(Equal("Pending"))
		})
	})
})
