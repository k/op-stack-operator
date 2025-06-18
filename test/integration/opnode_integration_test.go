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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
)

var _ = Describe("OpNode Integration", func() {
	Context("When reconciling an OpNode", func() {
		const (
			OpNodeNamespace = "default"
			timeout         = time.Second * 30
			duration        = time.Second * 10
			interval        = time.Millisecond * 250
		)

		var OpNodeName string
		var OptimismNetworkName string

		var (
			ctx = context.Background()
		)

		BeforeEach(func() {
			// Generate unique names for each test to avoid conflicts
			OpNodeName = fmt.Sprintf("test-opnode-%d", time.Now().UnixNano())
			OptimismNetworkName = fmt.Sprintf("test-network-%d", time.Now().UnixNano())

			// Add small delay to avoid resource conflicts
			time.Sleep(100 * time.Millisecond)

			// Create prerequisite OptimismNetwork first
			network := &optimismv1alpha1.OptimismNetwork{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OptimismNetwork",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OptimismNetworkName,
					Namespace: OpNodeNamespace,
				},
				Spec: optimismv1alpha1.OptimismNetworkSpec{
					NetworkName:  "test-sepolia",
					ChainID:      11155420,
					L1ChainID:    11155111,
					L1RpcUrl:     "http://localhost:8545", // Mock for testing
					L1BeaconUrl:  "http://localhost:5052",
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
						},
						Metrics: &optimismv1alpha1.MetricsConfig{
							Enabled: true,
							Port:    7300,
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, network)).Should(Succeed())

			// Wait for OptimismNetwork to be created and then manually set it to Ready phase for testing
			networkLookupKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OpNodeNamespace}
			createdNetwork := &optimismv1alpha1.OptimismNetwork{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, networkLookupKey, createdNetwork)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Manually set the OptimismNetwork to Ready phase for testing (since L1 RPC is mock)
			// Use Eventually with retry to handle race conditions with the controller
			Eventually(func() bool {
				// Get the latest version to avoid conflicts
				if err := k8sClient.Get(ctx, networkLookupKey, createdNetwork); err != nil {
					return false
				}
				createdNetwork.Status.Phase = "Ready"
				err := k8sClient.Status().Update(ctx, createdNetwork)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Clean up any existing OpNode resources
			opNode := &optimismv1alpha1.OpNode{}
			opNodeKey := types.NamespacedName{Name: OpNodeName, Namespace: OpNodeNamespace}
			if err := k8sClient.Get(ctx, opNodeKey, opNode); err == nil {
				// Remove finalizers to allow immediate deletion
				opNode.Finalizers = []string{}
				Expect(k8sClient.Update(ctx, opNode)).To(Succeed())

				// Delete the resource
				Expect(k8sClient.Delete(ctx, opNode)).To(Succeed())

				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, opNodeKey, opNode)
					return err != nil
				}, time.Second*5, time.Millisecond*100).Should(BeTrue())
			}
		})

		AfterEach(func() {
			// Clean up OpNode resources
			opNode := &optimismv1alpha1.OpNode{}
			opNodeKey := types.NamespacedName{Name: OpNodeName, Namespace: OpNodeNamespace}
			if err := k8sClient.Get(ctx, opNodeKey, opNode); err == nil {
				// Remove finalizers to allow immediate deletion with retry
				Eventually(func() bool {
					// Get fresh copy to avoid version conflicts
					if err := k8sClient.Get(ctx, opNodeKey, opNode); err != nil {
						return true // Already deleted
					}
					opNode.Finalizers = []string{}
					return k8sClient.Update(ctx, opNode) == nil
				}, time.Second*3, time.Millisecond*100).Should(BeTrue())

				// Delete the resource
				Expect(k8sClient.Delete(ctx, opNode)).To(Succeed())

				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, opNodeKey, opNode)
					return err != nil
				}, time.Second*5, time.Millisecond*100).Should(BeTrue())
			}

			// Clean up OptimismNetwork
			network := &optimismv1alpha1.OptimismNetwork{}
			networkKey := types.NamespacedName{Name: OptimismNetworkName, Namespace: OpNodeNamespace}
			if err := k8sClient.Get(ctx, networkKey, network); err == nil {
				// Remove finalizers to allow immediate deletion with retry
				Eventually(func() bool {
					// Get fresh copy to avoid version conflicts
					if err := k8sClient.Get(ctx, networkKey, network); err != nil {
						return true // Already deleted
					}
					network.Finalizers = []string{}
					return k8sClient.Update(ctx, network) == nil
				}, time.Second*3, time.Millisecond*100).Should(BeTrue())

				// Delete the resource
				Expect(k8sClient.Delete(ctx, network)).To(Succeed())

				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, networkKey, network)
					return err != nil
				}, time.Second*5, time.Millisecond*100).Should(BeTrue())
			}

			// Clean up any generated secrets
			secrets := []string{
				OpNodeName + "-jwt",
				OpNodeName + "-p2p",
			}
			for _, secretName := range secrets {
				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: secretName, Namespace: OpNodeNamespace}
				if err := k8sClient.Get(ctx, secretKey, secret); err == nil {
					Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
				}
			}
		})

		It("Should create an OpNode replica with valid configuration", func() {
			By("Creating a new OpNode replica")
			opNode := &optimismv1alpha1.OpNode{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OpNode",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OpNodeName,
					Namespace: OpNodeNamespace,
				},
				Spec: optimismv1alpha1.OpNodeSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name:      OptimismNetworkName,
						Namespace: OpNodeNamespace,
					},
					NodeType: "replica",
					OpNode: optimismv1alpha1.OpNodeConfig{
						SyncMode: "execution-layer",
						P2P: &optimismv1alpha1.P2PConfig{
							Enabled:    true,
							ListenPort: 9003,
							Discovery: &optimismv1alpha1.P2PDiscoveryConfig{
								Enabled: true,
							},
							PrivateKey: &optimismv1alpha1.SecretKeyRef{
								Generate: true,
							},
						},
						RPC: &optimismv1alpha1.RPCConfig{
							Enabled:     true,
							Host:        "0.0.0.0",
							Port:        9545,
							EnableAdmin: false,
						},
						Sequencer: &optimismv1alpha1.SequencerConfig{
							Enabled: false,
						},
					},
					OpGeth: optimismv1alpha1.OpGethConfig{
						DataDir:  "/data/geth",
						SyncMode: "snap",
						Storage: &optimismv1alpha1.StorageConfig{
							Size:         resource.MustParse("100Gi"),
							StorageClass: "standard",
							AccessMode:   "ReadWriteOnce",
						},
						Networking: &optimismv1alpha1.GethNetworkingConfig{
							HTTP: &optimismv1alpha1.HTTPConfig{
								Enabled: true,
								Host:    "0.0.0.0",
								Port:    8545,
								APIs:    []string{"web3", "eth", "net"},
							},
							WS: &optimismv1alpha1.WSConfig{
								Enabled: true,
								Host:    "0.0.0.0",
								Port:    8546,
								APIs:    []string{"web3", "eth"},
							},
							AuthRPC: &optimismv1alpha1.AuthRPCConfig{
								Host: "127.0.0.1",
								Port: 8551,
								APIs: []string{"engine", "eth"},
							},
						},
					},
					Resources: &optimismv1alpha1.OpNodeResources{
						OpNode: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2000m"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
						},
						OpGeth: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
							},
						},
					},
					Service: &optimismv1alpha1.ServiceConfig{
						Type: corev1.ServiceTypeClusterIP,
						Annotations: map[string]string{
							"test.annotation": "test-value",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, opNode)).Should(Succeed())

			opNodeLookupKey := types.NamespacedName{Name: OpNodeName, Namespace: OpNodeNamespace}
			createdOpNode := &optimismv1alpha1.OpNode{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, opNodeLookupKey, createdOpNode)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking the OpNode was created successfully")
			Expect(createdOpNode.Spec.NodeType).Should(Equal("replica"))
			Expect(createdOpNode.Spec.OptimismNetworkRef.Name).Should(Equal(OptimismNetworkName))
			Expect(createdOpNode.Spec.OpNode.P2P.Discovery.Enabled).Should(BeTrue())
			Expect(createdOpNode.Spec.OpNode.Sequencer.Enabled).Should(BeFalse())

			By("Checking that the controller adds finalizers")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, opNodeLookupKey, createdOpNode)
				if err != nil {
					return false
				}
				return len(createdOpNode.Finalizers) > 0
			}, timeout, interval).Should(BeTrue())

			By("Checking that JWT secret is created")
			jwtSecretKey := types.NamespacedName{Name: OpNodeName + "-jwt", Namespace: OpNodeNamespace}
			jwtSecret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jwtSecretKey, jwtSecret)
				if err != nil {
					// Log the error for debugging but don't fail immediately
					fmt.Printf("JWT secret not found yet: %v\n", err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(jwtSecret.Data).Should(HaveKey("jwt"))
			Expect(len(jwtSecret.Data["jwt"])).Should(BeNumerically(">", 0))

			By("Checking that P2P secret is created")
			p2pSecretKey := types.NamespacedName{Name: OpNodeName + "-p2p", Namespace: OpNodeNamespace}
			p2pSecret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, p2pSecretKey, p2pSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(p2pSecret.Data).Should(HaveKey("private-key"))
			Expect(len(p2pSecret.Data["private-key"])).Should(BeNumerically(">", 0))

			By("Checking that StatefulSet is created")
			statefulSetKey := types.NamespacedName{Name: OpNodeName, Namespace: OpNodeNamespace}
			statefulSet := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, statefulSetKey, statefulSet)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(statefulSet.Spec.Replicas).Should(Equal(int32Ptr(1)))
			Expect(statefulSet.Spec.Template.Spec.Containers).Should(HaveLen(2)) // op-geth + op-node
			Expect(statefulSet.Spec.VolumeClaimTemplates).Should(HaveLen(1))     // geth-data

			By("Checking that Service is created")
			serviceKey := types.NamespacedName{Name: OpNodeName, Namespace: OpNodeNamespace}
			service := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, serviceKey, service)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))
			Expect(service.Annotations).Should(HaveKeyWithValue("test.annotation", "test-value"))
			Expect(len(service.Spec.Ports)).Should(BeNumerically(">", 0))

			By("Checking OpNode status conditions")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, opNodeLookupKey, createdOpNode)
				if err != nil {
					return false
				}
				return len(createdOpNode.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Check for expected conditions
			expectedConditions := []string{
				"ConfigurationValid",
				"NetworkReference",
				"SecretsReady",
			}

			for _, conditionType := range expectedConditions {
				Eventually(func() bool {
					err := k8sClient.Get(ctx, opNodeLookupKey, createdOpNode)
					if err != nil {
						return false
					}
					for _, condition := range createdOpNode.Status.Conditions {
						if condition.Type == conditionType {
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue(), fmt.Sprintf("Condition %s should be present", conditionType))
			}
		})

		It("Should create an OpNode sequencer with proper security configuration", func() {
			By("Creating a new OpNode sequencer")
			opNode := &optimismv1alpha1.OpNode{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OpNode",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OpNodeName + "-sequencer",
					Namespace: OpNodeNamespace,
				},
				Spec: optimismv1alpha1.OpNodeSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name:      OptimismNetworkName,
						Namespace: OpNodeNamespace,
					},
					NodeType: "sequencer",
					OpNode: optimismv1alpha1.OpNodeConfig{
						SyncMode: "execution-layer",
						P2P: &optimismv1alpha1.P2PConfig{
							Enabled:    true,
							ListenPort: 9003,
							Discovery: &optimismv1alpha1.P2PDiscoveryConfig{
								Enabled: false, // Disabled for sequencer security
							},
							Static: []string{
								"16Uiu2HAm...", // Static peers for sequencer
							},
							PrivateKey: &optimismv1alpha1.SecretKeyRef{
								Generate: true,
							},
						},
						RPC: &optimismv1alpha1.RPCConfig{
							Enabled:     true,
							Host:        "0.0.0.0",
							Port:        9545,
							EnableAdmin: true, // Admin enabled for sequencer
						},
						Sequencer: &optimismv1alpha1.SequencerConfig{
							Enabled:       true,
							BlockTime:     "2s",
							MaxTxPerBlock: 1000,
						},
					},
					OpGeth: optimismv1alpha1.OpGethConfig{
						DataDir:  "/data/geth",
						SyncMode: "snap",
						Storage: &optimismv1alpha1.StorageConfig{
							Size:         resource.MustParse("500Gi"),
							StorageClass: "fast-ssd",
							AccessMode:   "ReadWriteOnce",
						},
						Networking: &optimismv1alpha1.GethNetworkingConfig{
							HTTP: &optimismv1alpha1.HTTPConfig{
								Enabled: true,
								Host:    "0.0.0.0",
								Port:    8545,
								APIs:    []string{"web3", "eth", "net", "debug"},
							},
							AuthRPC: &optimismv1alpha1.AuthRPCConfig{
								Host: "127.0.0.1",
								Port: 8551,
								APIs: []string{"engine", "eth"},
							},
							P2P: &optimismv1alpha1.GethP2PConfig{
								Port:        30303,
								MaxPeers:    25,
								NoDiscovery: true, // No discovery for sequencer security
								NetRestrict: "10.0.0.0/8",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, opNode)).Should(Succeed())

			sequencerLookupKey := types.NamespacedName{Name: OpNodeName + "-sequencer", Namespace: OpNodeNamespace}
			createdSequencer := &optimismv1alpha1.OpNode{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, sequencerLookupKey, createdSequencer)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking the sequencer configuration")
			Expect(createdSequencer.Spec.NodeType).Should(Equal("sequencer"))
			Expect(createdSequencer.Spec.OpNode.Sequencer.Enabled).Should(BeTrue())
			Expect(createdSequencer.Spec.OpNode.P2P.Discovery.Enabled).Should(BeFalse())     // Security for sequencers
			Expect(createdSequencer.Spec.OpNode.RPC.EnableAdmin).Should(BeTrue())            // Admin for sequencers
			Expect(createdSequencer.Spec.OpGeth.Networking.P2P.NoDiscovery).Should(BeTrue()) // P2P security
		})

		It("Should handle configuration validation errors", func() {
			By("Creating an OpNode with invalid configuration")
			invalidOpNode := &optimismv1alpha1.OpNode{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OpNode",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OpNodeName + "-invalid",
					Namespace: OpNodeNamespace,
				},
				Spec: optimismv1alpha1.OpNodeSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "", // Invalid empty name
					},
					NodeType: "invalid-type", // Invalid node type
				},
			}

			By("Expecting creation to fail due to CRD validation")
			err := k8sClient.Create(ctx, invalidOpNode)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("Unsupported value: \"invalid-type\""))
			Expect(err.Error()).Should(ContainSubstring("supported values: \"sequencer\", \"replica\""))

			By("Creating an OpNode with missing network reference")
			invalidOpNode2 := &optimismv1alpha1.OpNode{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "optimism.optimism.io/v1alpha1",
					Kind:       "OpNode",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      OpNodeName + "-invalid2",
					Namespace: OpNodeNamespace,
				},
				Spec: optimismv1alpha1.OpNodeSpec{
					OptimismNetworkRef: optimismv1alpha1.OptimismNetworkRef{
						Name: "non-existent-network",
					},
					NodeType: "replica", // Valid node type
				},
			}

			// This should succeed creation but fail validation in controller
			Expect(k8sClient.Create(ctx, invalidOpNode2)).Should(Succeed())

			invalidLookupKey := types.NamespacedName{Name: OpNodeName + "-invalid2", Namespace: OpNodeNamespace}
			createdInvalid := &optimismv1alpha1.OpNode{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, invalidLookupKey, createdInvalid)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking that validation errors are reported in status")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, invalidLookupKey, createdInvalid)
				if err != nil {
					return false
				}
				return createdInvalid.Status.Phase == "Error"
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, invalidLookupKey, createdInvalid)
				if err != nil {
					return false
				}
				for _, condition := range createdInvalid.Status.Conditions {
					if condition.Type == "NetworkReference" && condition.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})
})

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}
