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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/discovery"
)

var _ = Describe("OptimismNetwork Controller", func() {
	Context("When reconciling an OptimismNetwork", func() {
		const (
			OptimismNetworkName      = "test-network"
			OptimismNetworkNamespace = "default"
			timeout                  = time.Second * 10
			duration                 = time.Second * 10
			interval                 = time.Millisecond * 250
			// Real Alchemy Sepolia URL for testing
			testL1RpcUrl = "https://eth-sepolia.g.alchemy.com/v2/zeFYT4eQdrTCht4MM6BhQFqWzZ81QO8O"
		)

		ctx := context.Background()

		typeNamespace := types.NamespacedName{
			Name:      OptimismNetworkName,
			Namespace: OptimismNetworkNamespace,
		}

		BeforeEach(func() {
			// Clean up any existing resources
			network := &optimismv1alpha1.OptimismNetwork{}
			if err := k8sClient.Get(ctx, typeNamespace, network); err == nil {
				// Remove finalizers to allow immediate deletion
				network.Finalizers = []string{}
				Expect(k8sClient.Update(ctx, network)).To(Succeed())

				// Delete the resource
				Expect(k8sClient.Delete(ctx, network)).To(Succeed())

				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespace, network)
					return err != nil
				}, time.Second*5, time.Millisecond*100).Should(BeTrue())
			}
		})

		AfterEach(func() {
			// Clean up resources
			network := &optimismv1alpha1.OptimismNetwork{}
			if err := k8sClient.Get(ctx, typeNamespace, network); err == nil {
				// Remove finalizers to allow immediate deletion
				network.Finalizers = []string{}
				Expect(k8sClient.Update(ctx, network)).To(Succeed())

				// Delete the resource
				Expect(k8sClient.Delete(ctx, network)).To(Succeed())

				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespace, network)
					return err != nil
				}, time.Second*5, time.Millisecond*100).Should(BeTrue())
			}

			// Also clean up any ConfigMaps that might have been created
			configMaps := []string{
				OptimismNetworkName + "-rollup-config",
				OptimismNetworkName + "-genesis",
				"test-rollup-config", // From ConfigMap reference test
			}
			for _, cmName := range configMaps {
				cm := &corev1.ConfigMap{}
				cmKey := types.NamespacedName{Name: cmName, Namespace: OptimismNetworkNamespace}
				if err := k8sClient.Get(ctx, cmKey, cm); err == nil {
					Expect(k8sClient.Delete(ctx, cm)).To(Succeed())
				}
			}
		})

		It("Should create an OptimismNetwork with valid configuration", func() {
			By("Creating a new OptimismNetwork")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName:  "test-sepolia",
					ChainID:      11155420,
					L1ChainID:    11155111,
					L1RpcUrl:     testL1RpcUrl,
					L1BeaconUrl:  "https://sepolia-beacon.example.com",
					L1RpcTimeout: 10 * time.Second,
					RollupConfig: &optimismv1alpha1.ConfigSource{
						AutoDiscover: true,
					},
					L2Genesis: &optimismv1alpha1.ConfigSource{
						AutoDiscover: true,
					},
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
						CacheTimeout:    24 * time.Hour,
					},
					SharedConfig: &optimismv1alpha1.SharedConfig{
						Logging: &optimismv1alpha1.LoggingConfig{
							Level:  "info",
							Format: "logfmt",
							Color:  false,
						},
						Metrics: &optimismv1alpha1.MetricsConfig{
							Enabled: true,
							Port:    7300,
							Path:    "/metrics",
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
						Security: &optimismv1alpha1.SecurityConfig{
							RunAsNonRoot: boolPtr(true),
							RunAsUser:    int64Ptr(1000),
							FSGroup:      int64Ptr(1000),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking the OptimismNetwork was created successfully")
			Expect(createdNetwork.Spec.NetworkName).Should(Equal("test-sepolia"))
			Expect(createdNetwork.Spec.ChainID).Should(Equal(int64(11155420)))
			Expect(createdNetwork.Spec.L1ChainID).Should(Equal(int64(11155111)))
			Expect(createdNetwork.Spec.L1RpcUrl).Should(Equal(testL1RpcUrl))
		})

		It("Should validate required fields", func() {
			By("Creating an OptimismNetwork with missing chainID")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					// ChainID is missing (0)
					L1ChainID: 11155111,
					L1RpcUrl:  testL1RpcUrl,
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				return createdNetwork.Status.Phase == "Error"
			}, timeout, interval).Should(BeTrue())

			By("Checking the error condition is set")
			Expect(createdNetwork.Status.Conditions).Should(HaveLen(1))
			Expect(createdNetwork.Status.Conditions[0].Type).Should(Equal("ConfigurationValid"))
			Expect(createdNetwork.Status.Conditions[0].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdNetwork.Status.Conditions[0].Reason).Should(Equal("InvalidConfiguration"))
		})

		It("Should validate chain ID relationship", func() {
			By("Creating an OptimismNetwork with same L1 and L2 chain IDs")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   1,
					L1ChainID: 1, // Same as chainID - should be invalid
					L1RpcUrl:  testL1RpcUrl,
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				return createdNetwork.Status.Phase == "Error"
			}, timeout, interval).Should(BeTrue())

			By("Checking the validation error is set")
			Expect(createdNetwork.Status.Conditions).Should(HaveLen(1))
			Expect(createdNetwork.Status.Conditions[0].Type).Should(Equal("ConfigurationValid"))
			Expect(createdNetwork.Status.Conditions[0].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdNetwork.Status.Conditions[0].Message).Should(ContainSubstring("chainID cannot be the same as l1ChainID"))
		})

		It("Should validate ConfigSource configuration", func() {
			By("Creating an OptimismNetwork with multiple ConfigSource options")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   11155420,
					L1ChainID: 11155111,
					L1RpcUrl:  testL1RpcUrl,
					RollupConfig: &optimismv1alpha1.ConfigSource{
						Inline:       `{"genesis": {}}`,
						AutoDiscover: true, // Both inline and autoDiscover - should be invalid
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				return createdNetwork.Status.Phase == "Error"
			}, timeout, interval).Should(BeTrue())

			By("Checking the validation error is set")
			Expect(createdNetwork.Status.Conditions).Should(HaveLen(1))
			Expect(createdNetwork.Status.Conditions[0].Type).Should(Equal("ConfigurationValid"))
			Expect(createdNetwork.Status.Conditions[0].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdNetwork.Status.Conditions[0].Message).Should(ContainSubstring("only one of inline, configMapRef, or autoDiscover can be specified"))
		})

		It("Should discover contract addresses for well-known networks", func() {
			By("Creating an OptimismNetwork for op-sepolia")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "op-sepolia",
					ChainID:     11155420,
					L1ChainID:   11155111,
					L1RpcUrl:    testL1RpcUrl, // Now using real Alchemy URL
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
						CacheTimeout:    24 * time.Hour,
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() error {
				return k8sClient.Get(ctx, networkLookupKey, createdNetwork)
			}, timeout, interval).Should(Succeed())

			By("Checking that contract discovery completed")
			// With real Alchemy URL, contract discovery should work for well-known networks
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				// Check if contracts discovered condition exists
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "ContractsDiscovered" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Should create ConfigMaps when autoDiscover is enabled", func() {
			By("Creating an OptimismNetwork with autoDiscover enabled")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "test-network",
					ChainID:     11155420, // OP Sepolia chain ID
					L1ChainID:   11155111, // Sepolia chain ID (matches Alchemy URL)
					L1RpcUrl:    testL1RpcUrl,
					RollupConfig: &optimismv1alpha1.ConfigSource{
						AutoDiscover: true,
					},
					L2Genesis: &optimismv1alpha1.ConfigSource{
						AutoDiscover: true,
					},
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
						CacheTimeout:    24 * time.Hour,
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			// Check that ConfigMaps are created
			By("Checking rollup config ConfigMap is created")
			rollupConfigMap := &corev1.ConfigMap{}
			rollupConfigMapKey := types.NamespacedName{
				Name:      OptimismNetworkName + "-rollup-config",
				Namespace: OptimismNetworkNamespace,
			}

			Eventually(func() error {
				return k8sClient.Get(ctx, rollupConfigMapKey, rollupConfigMap)
			}, timeout, interval).Should(Succeed())

			Expect(rollupConfigMap.Data).Should(HaveKey("rollup.json"))
			Expect(rollupConfigMap.Data["rollup.json"]).Should(ContainSubstring(`"l2_chain_id": 11155420`))

			By("Checking genesis ConfigMap is created")
			genesisConfigMap := &corev1.ConfigMap{}
			genesisConfigMapKey := types.NamespacedName{
				Name:      OptimismNetworkName + "-genesis",
				Namespace: OptimismNetworkNamespace,
			}

			Eventually(func() error {
				return k8sClient.Get(ctx, genesisConfigMapKey, genesisConfigMap)
			}, timeout, interval).Should(Succeed())

			Expect(genesisConfigMap.Data).Should(HaveKey("genesis.json"))
			Expect(genesisConfigMap.Data["genesis.json"]).Should(ContainSubstring(`"chainId": 11155420`))
		})

		It("Should handle ConfigMap reference validation", func() {
			By("Creating a ConfigMap first")
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollup-config",
					Namespace: OptimismNetworkNamespace,
				},
				Data: map[string]string{
					"rollup.json": `{"test": "config"}`,
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).Should(Succeed())

			By("Creating an OptimismNetwork that references the ConfigMap")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   11155420,
					L1ChainID: 11155111,
					L1RpcUrl:  testL1RpcUrl,
					RollupConfig: &optimismv1alpha1.ConfigSource{
						ConfigMapRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-rollup-config",
							},
							Key: "rollup.json",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				// Should pass configuration validation since ConfigMap exists
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "ConfigurationValid" && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Should set proper status conditions and phases", func() {
			By("Creating an OptimismNetwork")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName: "op-sepolia",
					ChainID:     11155420,
					L1ChainID:   11155111,
					L1RpcUrl:    testL1RpcUrl,
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() int {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return 0
				}
				return len(createdNetwork.Status.Conditions)
			}, timeout, interval).Should(BeNumerically(">", 0))

			By("Checking that status conditions are set")
			// Should have at least ConfigurationValid condition
			configCondition := false
			for _, condition := range createdNetwork.Status.Conditions {
				if condition.Type == "ConfigurationValid" {
					configCondition = true
					Expect(condition.Status).Should(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).Should(Equal("ValidConfiguration"))
					break
				}
			}
			Expect(configCondition).Should(BeTrue())

			By("Checking observed generation is set")
			Expect(createdNetwork.Status.ObservedGeneration).Should(Equal(createdNetwork.Generation))
		})

		It("Should handle finalizer and deletion properly", func() {
			By("Creating an OptimismNetwork")
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OptimismNetworkNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					ChainID:   11155420,
					L1ChainID: 11155111,
					L1RpcUrl:  testL1RpcUrl,
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				// Check that finalizer is added
				for _, finalizer := range createdNetwork.Finalizers {
					if finalizer == OptimismNetworkFinalizer {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Deleting the OptimismNetwork")
			Expect(k8sClient.Delete(ctx, createdNetwork)).Should(Succeed())

			By("Checking the OptimismNetwork is eventually deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Unit tests for controller methods", func() {
		var reconciler *OptimismNetworkReconciler

		BeforeEach(func() {
			reconciler = &OptimismNetworkReconciler{
				Client:           k8sClient,
				Scheme:           k8sClient.Scheme(),
				DiscoveryService: discovery.NewContractDiscoveryService(24 * time.Hour),
			}
		})

		Describe("validateConfiguration", func() {
			It("Should accept valid configuration", func() {
				network := &optimismv1alpha1.OptimismNetwork{
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						ChainID:   10,
						L1ChainID: 1,
						L1RpcUrl:  "https://mainnet.infura.io/v3/test",
					},
				}

				err := reconciler.validateConfiguration(network)
				Expect(err).Should(BeNil())
			})

			It("Should reject missing required fields", func() {
				network := &optimismv1alpha1.OptimismNetwork{
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						// Missing ChainID
						L1ChainID: 1,
						L1RpcUrl:  "https://mainnet.infura.io/v3/test",
					},
				}

				err := reconciler.validateConfiguration(network)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("chainID is required"))
			})

			It("Should reject same L1 and L2 chain IDs", func() {
				network := &optimismv1alpha1.OptimismNetwork{
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						ChainID:   1,
						L1ChainID: 1,
						L1RpcUrl:  "https://mainnet.infura.io/v3/test",
					},
				}

				err := reconciler.validateConfiguration(network)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("chainID cannot be the same as l1ChainID"))
			})

			It("Should validate ConfigSource options", func() {
				network := &optimismv1alpha1.OptimismNetwork{
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						ChainID:   10,
						L1ChainID: 1,
						L1RpcUrl:  "https://mainnet.infura.io/v3/test",
						RollupConfig: &optimismv1alpha1.ConfigSource{
							Inline:       "test",
							AutoDiscover: true, // Both set - should be invalid
						},
					},
				}

				err := reconciler.validateConfiguration(network)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("only one of inline, configMapRef, or autoDiscover can be specified"))
			})
		})

		Describe("generateRollupConfig", func() {
			It("Should generate valid rollup config JSON", func() {
				network := &optimismv1alpha1.OptimismNetwork{
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						ChainID:   10,
						L1ChainID: 1,
					},
				}

				addresses := &optimismv1alpha1.NetworkContractAddresses{
					OptimismPortalAddr: "0x1234567890123456789012345678901234567890",
					SystemConfigAddr:   "0x0987654321098765432109876543210987654321",
				}

				config := reconciler.generateRollupConfig(network, addresses)
				Expect(config).Should(ContainSubstring(`"l1_chain_id": 1`))
				Expect(config).Should(ContainSubstring(`"l2_chain_id": 10`))
				Expect(config).Should(ContainSubstring("0x1234567890123456789012345678901234567890"))
				Expect(config).Should(ContainSubstring("0x0987654321098765432109876543210987654321"))
			})
		})

		Describe("generateGenesisConfig", func() {
			It("Should generate valid genesis config JSON", func() {
				network := &optimismv1alpha1.OptimismNetwork{
					Spec: optimismv1alpha1.OptimismNetworkSpec{
						ChainID: 10,
					},
				}

				addresses := &optimismv1alpha1.NetworkContractAddresses{}

				config := reconciler.generateGenesisConfig(network, addresses)
				Expect(config).Should(ContainSubstring(`"chainId": 10`))
				Expect(config).Should(ContainSubstring(`"optimism"`))
				Expect(config).Should(ContainSubstring(`"alloc"`))
			})
		})
	})
})

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
