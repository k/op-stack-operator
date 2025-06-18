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

package integration

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
)

var _ = Describe("OptimismNetwork Integration", func() {
	Context("When reconciling an OptimismNetwork", func() {
		const (
			OptimismNetworkNamespace = "default"
			timeout                  = time.Second * 10
			duration                 = time.Second * 10
			interval                 = time.Millisecond * 250
		)

		var OptimismNetworkName string

		var testL1RpcUrl string

		var (
			ctx           = context.Background()
			typeNamespace = types.NamespacedName{
				Name:      OptimismNetworkName,
				Namespace: OptimismNetworkNamespace,
			}
		)

		BeforeEach(func() {
			// Generate unique name for each test to avoid conflicts
			OptimismNetworkName = fmt.Sprintf("test-network-%d", time.Now().UnixNano())

			// Get L1 RPC URL from environment variable, fallback to localhost for CI
			testL1RpcUrl = os.Getenv("TEST_L1_RPC_URL")
			if testL1RpcUrl == "" {
				// Fallback for CI/testing without real RPC
				testL1RpcUrl = "http://localhost:8545"
				Skip("Skipping integration tests - no TEST_L1_RPC_URL environment variable set")
			}

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
			// Clean up resources with retry logic to handle resource version conflicts
			network := &optimismv1alpha1.OptimismNetwork{}
			if err := k8sClient.Get(ctx, typeNamespace, network); err == nil {
				// Remove finalizers to allow immediate deletion with retry
				Eventually(func() bool {
					// Get fresh copy to avoid version conflicts
					if err := k8sClient.Get(ctx, typeNamespace, network); err != nil {
						return true // Already deleted
					}
					network.Finalizers = []string{}
					return k8sClient.Update(ctx, network) == nil
				}, time.Second*3, time.Millisecond*100).Should(BeTrue())

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
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking validation error condition is set")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "ConfigurationValid" && condition.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Should validate L1ChainID relationship", func() {
			By("Creating an OptimismNetwork with invalid L1ChainID")
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
					NetworkName: "test-sepolia",
					ChainID:     11155420,
					L1ChainID:   1, // Should be 11155111 for Sepolia testnet
					L1RpcUrl:    testL1RpcUrl,
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking L1 connection error condition is set")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "L1Connected" && condition.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Should test L1 connectivity", func() {
			By("Creating an OptimismNetwork with valid L1 RPC")
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
					NetworkName: "test-sepolia",
					ChainID:     11155420,
					L1ChainID:   11155111,
					L1RpcUrl:    testL1RpcUrl,
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "well-known",
						CacheTimeout:    24 * time.Hour,
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

			By("Checking L1 connection is established")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "L1Connected" && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, time.Second*30, interval).Should(BeTrue()) // Longer timeout for network calls
		})

		It("Should discover contract addresses for well-known networks", func() {
			By("Creating an OptimismNetwork for OP Sepolia")
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
						CacheTimeout:    24 * time.Hour,
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OptimismNetworkNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}

			Eventually(func() bool {
				return k8sClient.Get(ctx, networkLookupKey, createdNetwork) == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking contract addresses are discovered")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "ContractsDiscovered" && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// Verify specific addresses are populated
			Eventually(func() string {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return ""
				}
				if createdNetwork.Status.NetworkInfo != nil && createdNetwork.Status.NetworkInfo.DiscoveredContracts != nil {
					return createdNetwork.Status.NetworkInfo.DiscoveredContracts.L2OutputOracleAddr
				}
				return ""
			}, timeout, interval).Should(Equal("0x90E9c4f8a994a250F6aEfd61CAFb4F2e895D458F"))
		})

		It("Should generate ConfigMaps for rollup config and genesis", func() {
			By("Creating an OptimismNetwork with auto-discover configs")
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

			By("Checking rollup config ConfigMap is created")
			rollupConfigMapKey := types.NamespacedName{
				Name:      OptimismNetworkName + "-rollup-config",
				Namespace: OptimismNetworkNamespace,
			}
			rollupConfigMap := &corev1.ConfigMap{}

			Eventually(func() error {
				return k8sClient.Get(ctx, rollupConfigMapKey, rollupConfigMap)
			}, time.Second*30, interval).Should(Succeed()) // Longer timeout for config generation

			By("Checking genesis ConfigMap is created")
			genesisConfigMapKey := types.NamespacedName{
				Name:      OptimismNetworkName + "-genesis",
				Namespace: OptimismNetworkNamespace,
			}
			genesisConfigMap := &corev1.ConfigMap{}

			Eventually(func() error {
				return k8sClient.Get(ctx, genesisConfigMapKey, genesisConfigMap)
			}, time.Second*30, interval).Should(Succeed()) // Longer timeout for config generation

			By("Verifying ConfigMap contents")
			Expect(rollupConfigMap.Data).Should(HaveKey("rollup.json"))
			Expect(genesisConfigMap.Data).Should(HaveKey("genesis.json"))
		})

		It("Should handle ConfigMap references", func() {
			By("Creating a ConfigMap for rollup config")
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollup-config",
					Namespace: OptimismNetworkNamespace,
				},
				Data: map[string]string{
					"rollup.json": `{"genesis": {"l1": {"hash": "0x123", "number": 123}}, "block_time": 2}`,
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
					NetworkName: "test-network",
					ChainID:     12345,
					L1ChainID:   11155111,
					L1RpcUrl:    testL1RpcUrl,
					RollupConfig: &optimismv1alpha1.ConfigSource{
						ConfigMapRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-rollup-config",
							},
							Key: "rollup.json",
						},
					},
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "manual",
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

			By("Checking configuration is valid")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				for _, condition := range createdNetwork.Status.Conditions {
					if condition.Type == "ConfigurationValid" && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		// Note: Contract discovery service testing is handled through integration with the controller

		It("Should handle finalizer properly", func() {
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
					NetworkName: "test-network",
					ChainID:     12345,
					L1ChainID:   11155111,
					L1RpcUrl:    testL1RpcUrl,
					ContractAddresses: &optimismv1alpha1.ContractAddressConfig{
						DiscoveryMethod: "manual",
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

			By("Checking finalizer is added")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				if err != nil {
					return false
				}
				for _, finalizer := range createdNetwork.Finalizers {
					if finalizer == "optimismnetwork.optimism.io/finalizer" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
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
