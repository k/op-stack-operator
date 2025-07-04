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

package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
	"github.com/ethereum-optimism/op-stack-operator/pkg/config"
)

// CreateOpNodeStatefulSet creates a StatefulSet for OpNode (op-geth + op-node)
func CreateOpNodeStatefulSet(
	opNode *optimismv1alpha1.OpNode,
	network *optimismv1alpha1.OptimismNetwork,
) *appsv1.StatefulSet {
	labels := map[string]string{
		"app.kubernetes.io/name":       "opnode",
		"app.kubernetes.io/instance":   opNode.Name,
		"app.kubernetes.io/component":  "consensus-layer",
		"app.kubernetes.io/part-of":    "op-stack",
		"app.kubernetes.io/managed-by": "op-stack-operator",
		"optimism.io/network":          network.Spec.NetworkName,
		"optimism.io/node-type":        opNode.Spec.NodeType,
	}

	// Default storage size if not specified
	storageSize := resource.MustParse("1Ti")
	if opNode.Spec.OpGeth.Storage != nil && !opNode.Spec.OpGeth.Storage.Size.IsZero() {
		storageSize = opNode.Spec.OpGeth.Storage.Size
	}

	// Default storage class
	storageClass := "fast-ssd"
	if opNode.Spec.OpGeth.Storage != nil && opNode.Spec.OpGeth.Storage.StorageClass != "" {
		storageClass = opNode.Spec.OpGeth.Storage.StorageClass
	}

	// Default access mode
	accessMode := corev1.ReadWriteOnce
	if opNode.Spec.OpGeth.Storage != nil && opNode.Spec.OpGeth.Storage.AccessMode != "" {
		accessMode = corev1.PersistentVolumeAccessMode(opNode.Spec.OpGeth.Storage.AccessMode)
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opNode.Name,
			Namespace: opNode.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    int32Ptr(1),
			ServiceName: opNode.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						createOpGethContainer(opNode, network),
						createOpNodeContainer(opNode, network),
					},
					Volumes:         createVolumes(opNode, network),
					SecurityContext: createPodSecurityContext(network),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "geth-data",
						Labels: labels,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
						StorageClassName: &storageClass,
					},
				},
			},
		},
	}

	return statefulSet
}

// createOpGethContainer creates the op-geth container
func createOpGethContainer(
	opNode *optimismv1alpha1.OpNode,
	network *optimismv1alpha1.OptimismNetwork,
) corev1.Container {
	// Default resource requirements for op-geth
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2000m"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("8000m"),
			corev1.ResourceMemory: resource.MustParse("32Gi"),
		},
	}

	// Override with user-specified resources
	if opNode.Spec.Resources != nil && opNode.Spec.Resources.OpGeth != nil {
		resources = *opNode.Spec.Resources.OpGeth
	} else if network.Spec.SharedConfig != nil && network.Spec.SharedConfig.Resources != nil {
		// Fall back to shared config
		if len(network.Spec.SharedConfig.Resources.Requests) > 0 {
			resources.Requests = network.Spec.SharedConfig.Resources.Requests
		}
		if len(network.Spec.SharedConfig.Resources.Limits) > 0 {
			resources.Limits = network.Spec.SharedConfig.Resources.Limits
		}
	}

	// Default data directory
	dataDir := "/data/geth"
	if opNode.Spec.OpGeth.DataDir != "" {
		dataDir = opNode.Spec.OpGeth.DataDir
	}

	// Build command args
	args := []string{
		"--datadir=" + dataDir,
		"--networkid=" + fmt.Sprintf("%d", network.Spec.ChainID),
		"--rollup.sequencerhttp=" + getSequencerEndpoint(opNode, network),
	}

	// Add sync mode
	syncMode := "snap"
	if opNode.Spec.OpGeth.SyncMode != "" {
		syncMode = opNode.Spec.OpGeth.SyncMode
	}
	args = append(args, "--syncmode="+syncMode)

	// Add HTTP RPC configuration
	if opNode.Spec.OpGeth.Networking != nil &&
		opNode.Spec.OpGeth.Networking.HTTP != nil &&
		opNode.Spec.OpGeth.Networking.HTTP.Enabled {
		httpConfig := opNode.Spec.OpGeth.Networking.HTTP
		args = append(args, "--http")
		args = append(args, "--http.addr="+getDefaultString(httpConfig.Host, "0.0.0.0"))
		args = append(args, "--http.port="+fmt.Sprintf("%d", getDefaultInt32(httpConfig.Port, 8545)))
		if len(httpConfig.APIs) > 0 {
			args = append(args, "--http.api="+joinStrings(httpConfig.APIs))
		}
		if httpConfig.CORS != nil && len(httpConfig.CORS.Origins) > 0 {
			args = append(args, "--http.corsdomain="+joinStrings(httpConfig.CORS.Origins))
		}
	}

	// Add WebSocket configuration
	if opNode.Spec.OpGeth.Networking != nil &&
		opNode.Spec.OpGeth.Networking.WS != nil &&
		opNode.Spec.OpGeth.Networking.WS.Enabled {
		wsConfig := opNode.Spec.OpGeth.Networking.WS
		args = append(args, "--ws")
		args = append(args, "--ws.addr="+getDefaultString(wsConfig.Host, "0.0.0.0"))
		args = append(args, "--ws.port="+fmt.Sprintf("%d", getDefaultInt32(wsConfig.Port, 8546)))
		if len(wsConfig.APIs) > 0 {
			args = append(args, "--ws.api="+joinStrings(wsConfig.APIs))
		}
		if len(wsConfig.Origins) > 0 {
			args = append(args, "--ws.origins="+joinStrings(wsConfig.Origins))
		}
	}

	// Add auth RPC configuration
	if opNode.Spec.OpGeth.Networking != nil && opNode.Spec.OpGeth.Networking.AuthRPC != nil {
		authConfig := opNode.Spec.OpGeth.Networking.AuthRPC
		args = append(args, "--authrpc.addr="+getDefaultString(authConfig.Host, "127.0.0.1"))
		args = append(args, "--authrpc.port="+fmt.Sprintf("%d", getDefaultInt32(authConfig.Port, 8551)))
		args = append(args, "--authrpc.jwtsecret=/secrets/jwt/jwt")
	}

	container := corev1.Container{
		Name:            "op-geth",
		Image:           config.DefaultImages.OpGeth,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"geth"},
		Args:            args,
		Resources:       resources,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 8545, Protocol: corev1.ProtocolTCP},
			{Name: "ws", ContainerPort: 8546, Protocol: corev1.ProtocolTCP},
			{Name: "authrpc", ContainerPort: 8551, Protocol: corev1.ProtocolTCP},
			{Name: "p2p", ContainerPort: 30303, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "geth-data", MountPath: dataDir},
			{Name: "jwt-secret", MountPath: "/secrets/jwt", ReadOnly: true},
			{Name: "rollup-config", MountPath: "/config", ReadOnly: true},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/",
					Port: intstr.FromInt(8545),
					HTTPHeaders: []corev1.HTTPHeader{
						{
							Name:  "Content-Type",
							Value: "application/json",
						},
					},
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       30,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/",
					Port: intstr.FromInt(8545),
					HTTPHeaders: []corev1.HTTPHeader{
						{
							Name:  "Content-Type",
							Value: "application/json",
						},
					},
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
	}

	return container
}

// createOpNodeContainer creates the op-node container
func createOpNodeContainer(
	opNode *optimismv1alpha1.OpNode,
	network *optimismv1alpha1.OptimismNetwork,
) corev1.Container {
	// Default resource requirements for op-node
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2000m"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
	}

	// Override with user-specified resources
	if opNode.Spec.Resources != nil && opNode.Spec.Resources.OpNode != nil {
		resources = *opNode.Spec.Resources.OpNode
	}

	// Build command args
	authRPCPort := getAuthRPCPort(opNode)
	args := []string{
		"--l1=" + network.Spec.L1RpcUrl,
		"--l2=http://127.0.0.1:" + fmt.Sprintf("%d", authRPCPort),
		"--l2.jwt-secret=/secrets/jwt/jwt",
		"--rollup.config=/config/rollup.json",
	}

	// Add network name if provided
	if network.Spec.NetworkName != "" {
		args = append(args, "--network="+network.Spec.NetworkName)
	}

	// Add RPC configuration
	if opNode.Spec.OpNode.RPC != nil && opNode.Spec.OpNode.RPC.Enabled {
		rpcConfig := opNode.Spec.OpNode.RPC
		args = append(args, "--rpc.addr="+getDefaultString(rpcConfig.Host, "0.0.0.0"))
		args = append(args, "--rpc.port="+fmt.Sprintf("%d", getDefaultInt32(rpcConfig.Port, 9545)))
		if rpcConfig.EnableAdmin {
			args = append(args, "--rpc.enable-admin")
		}
	}

	// Add P2P configuration
	if opNode.Spec.OpNode.P2P != nil && opNode.Spec.OpNode.P2P.Enabled {
		p2pConfig := opNode.Spec.OpNode.P2P
		args = append(args, "--p2p.listen.tcp="+fmt.Sprintf("%d", getDefaultInt32(p2pConfig.ListenPort, 9003)))

		if p2pConfig.Discovery != nil && !p2pConfig.Discovery.Enabled {
			args = append(args, "--p2p.no-discovery")
		}

		if len(p2pConfig.Static) > 0 {
			for _, peer := range p2pConfig.Static {
				args = append(args, "--p2p.static="+peer)
			}
		}

		// Add P2P private key path if either auto-generated or user-provided
		if p2pConfig.PrivateKey != nil &&
			(p2pConfig.PrivateKey.Generate || p2pConfig.PrivateKey.SecretRef != nil) {
			args = append(args, "--p2p.priv.path=/secrets/p2p/private-key")
		}
	}

	// Add sequencer configuration
	if opNode.Spec.OpNode.Sequencer != nil && opNode.Spec.OpNode.Sequencer.Enabled {
		args = append(args, "--sequencer.enabled")
		if opNode.Spec.OpNode.Sequencer.BlockTime != "" {
			args = append(args, "--sequencer.l1-confs=4")
		}
	}

	// Add logging configuration
	if network.Spec.SharedConfig != nil && network.Spec.SharedConfig.Logging != nil {
		logging := network.Spec.SharedConfig.Logging
		if logging.Level != "" {
			args = append(args, "--log.level="+logging.Level)
		}
		if logging.Format != "" {
			args = append(args, "--log.format="+logging.Format)
		}
	}

	// Add metrics configuration
	if network.Spec.SharedConfig != nil &&
		network.Spec.SharedConfig.Metrics != nil &&
		network.Spec.SharedConfig.Metrics.Enabled {
		metrics := network.Spec.SharedConfig.Metrics
		args = append(args, "--metrics.enabled")
		args = append(args, "--metrics.addr=0.0.0.0")
		args = append(args, "--metrics.port="+fmt.Sprintf("%d", getDefaultInt32(metrics.Port, 7300)))
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "jwt-secret", MountPath: "/secrets/jwt", ReadOnly: true},
		{Name: "rollup-config", MountPath: "/config", ReadOnly: true},
	}

	// Add P2P key mount if either auto-generated or user-provided
	if opNode.Spec.OpNode.P2P != nil &&
		opNode.Spec.OpNode.P2P.PrivateKey != nil &&
		(opNode.Spec.OpNode.P2P.PrivateKey.Generate || opNode.Spec.OpNode.P2P.PrivateKey.SecretRef != nil) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "p2p-key", MountPath: "/secrets/p2p", ReadOnly: true,
		})
	}

	container := corev1.Container{
		Name:            "op-node",
		Image:           config.DefaultImages.OpNode,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"op-node"},
		Args:            args,
		Resources:       resources,
		Ports: []corev1.ContainerPort{
			{Name: "rpc", ContainerPort: 9545, Protocol: corev1.ProtocolTCP},
			{Name: "p2p", ContainerPort: 9003, Protocol: corev1.ProtocolTCP},
			{Name: "metrics", ContainerPort: 7300, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: volumeMounts,
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(9545),
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       30,
			FailureThreshold:    5,
			TimeoutSeconds:      10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(9545),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			FailureThreshold:    3,
			TimeoutSeconds:      5,
		},
	}

	return container
}

// createVolumes creates the volumes for the pod
func createVolumes(opNode *optimismv1alpha1.OpNode, network *optimismv1alpha1.OptimismNetwork) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "jwt-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: opNode.Name + "-jwt",
				},
			},
		},
		{
			Name: "rollup-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: network.Name + "-rollup-config",
					},
				},
			},
		},
	}

	// Add P2P key volume if either auto-generated or user-provided
	if opNode.Spec.OpNode.P2P != nil &&
		opNode.Spec.OpNode.P2P.PrivateKey != nil &&
		(opNode.Spec.OpNode.P2P.PrivateKey.Generate || opNode.Spec.OpNode.P2P.PrivateKey.SecretRef != nil) {

		// Determine the secret name based on how the key is managed
		var secretName string
		if opNode.Spec.OpNode.P2P.PrivateKey.Generate {
			// Use the auto-generated secret name pattern
			secretName = opNode.Name + "-p2p"
		} else if opNode.Spec.OpNode.P2P.PrivateKey.SecretRef != nil {
			// Use the user-provided secret name
			secretName = opNode.Spec.OpNode.P2P.PrivateKey.SecretRef.Name
		}

		volumes = append(volumes, corev1.Volume{
			Name: "p2p-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
	}

	return volumes
}

// createPodSecurityContext creates the pod security context
func createPodSecurityContext(network *optimismv1alpha1.OptimismNetwork) *corev1.PodSecurityContext {
	securityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		RunAsUser:    int64Ptr(1000),
		FSGroup:      int64Ptr(1000),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	// Override with network-specific security settings
	if network.Spec.SharedConfig != nil && network.Spec.SharedConfig.Security != nil {
		security := network.Spec.SharedConfig.Security
		if security.RunAsNonRoot != nil {
			securityContext.RunAsNonRoot = security.RunAsNonRoot
		}
		if security.RunAsUser != nil {
			securityContext.RunAsUser = security.RunAsUser
		}
		if security.FSGroup != nil {
			securityContext.FSGroup = security.FSGroup
		}
		if security.SeccompProfile != nil {
			securityContext.SeccompProfile = security.SeccompProfile
		}
	}

	return securityContext
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

func getDefaultString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func getDefaultInt32(value, defaultValue int32) int32 {
	if value == 0 {
		return defaultValue
	}
	return value
}

func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += "," + strs[i]
	}
	return result
}

// getSequencerEndpoint returns the configured sequencer endpoint for op-geth
func getSequencerEndpoint(opNode *optimismv1alpha1.OpNode, network *optimismv1alpha1.OptimismNetwork) string {
	// If this node is a sequencer, point to itself (localhost)
	if opNode.Spec.OpNode.Sequencer != nil && opNode.Spec.OpNode.Sequencer.Enabled {
		// Use localhost since op-geth and op-node run in the same pod
		// Get the configured HTTP port for this sequencer
		port := getOpGethHTTPPort(opNode)
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	// For replica nodes, check if L2RpcUrl is provided (external sequencer)
	if opNode.Spec.L2RpcUrl != "" {
		return opNode.Spec.L2RpcUrl
	}

	// For replica nodes, use their sequencer reference (internal sequencer)
	if opNode.Spec.SequencerRef != nil {
		// Use the explicit sequencer reference from the OpNode
		sequencerServiceName := opNode.Spec.SequencerRef.Name

		// For cross-namespace references, include the namespace
		if opNode.Spec.SequencerRef.Namespace != "" &&
			opNode.Spec.SequencerRef.Namespace != opNode.Namespace {
			sequencerServiceName = fmt.Sprintf("%s.%s.svc.cluster.local",
				opNode.Spec.SequencerRef.Name,
				opNode.Spec.SequencerRef.Namespace)
		}

		// Use default port since we can't access the sequencer's configuration
		// In a future enhancement, we could look up the sequencer OpNode to get its configured port
		port := int32(8545) // Default HTTP port

		return fmt.Sprintf("http://%s:%d", sequencerServiceName, port)
	}

	// Fallback: use the naming convention for backward compatibility
	// This maintains compatibility but should be considered deprecated
	sequencerServiceName := fmt.Sprintf("%s-sequencer", network.Name)
	port := int32(8545) // Default port assumption

	return fmt.Sprintf("http://%s:%d", sequencerServiceName, port)
}

// getOpGethHTTPPort returns the configured HTTP port for op-geth
func getOpGethHTTPPort(opNode *optimismv1alpha1.OpNode) int32 {
	if opNode.Spec.OpGeth.Networking != nil &&
		opNode.Spec.OpGeth.Networking.HTTP != nil {
		return getDefaultInt32(opNode.Spec.OpGeth.Networking.HTTP.Port, 8545)
	}
	return 8545
}

// getAuthRPCPort returns the configured AuthRPC port for op-geth
func getAuthRPCPort(opNode *optimismv1alpha1.OpNode) int32 {
	if opNode.Spec.OpGeth.Networking != nil && opNode.Spec.OpGeth.Networking.AuthRPC != nil {
		return getDefaultInt32(opNode.Spec.OpGeth.Networking.AuthRPC.Port, 8551)
	}
	return 8551
}
