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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	optimismv1alpha1 "github.com/ethereum-optimism/op-stack-operator/api/v1alpha1"
)

// CreateOpNodeService creates a Kubernetes Service for OpNode
func CreateOpNodeService(opNode *optimismv1alpha1.OpNode, network *optimismv1alpha1.OptimismNetwork) *corev1.Service {
	labels := map[string]string{
		"app.kubernetes.io/name":       "opnode",
		"app.kubernetes.io/instance":   opNode.Name,
		"app.kubernetes.io/component":  "consensus-layer",
		"app.kubernetes.io/part-of":    "op-stack",
		"app.kubernetes.io/managed-by": "op-stack-operator",
		"optimism.io/network":          network.Spec.NetworkName,
		"optimism.io/node-type":        opNode.Spec.NodeType,
	}

	// Default service type
	serviceType := corev1.ServiceTypeClusterIP
	if opNode.Spec.Service != nil && opNode.Spec.Service.Type != "" {
		serviceType = opNode.Spec.Service.Type
	}

	// Default annotations
	annotations := make(map[string]string)
	if opNode.Spec.Service != nil && len(opNode.Spec.Service.Annotations) > 0 {
		annotations = opNode.Spec.Service.Annotations
	}

	// Build service ports
	ports := buildServicePorts(opNode)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        opNode.Name,
			Namespace:   opNode.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: labels,
			Ports:    ports,
		},
	}

	return service
}

// buildServicePorts builds the service ports based on OpNode configuration
func buildServicePorts(opNode *optimismv1alpha1.OpNode) []corev1.ServicePort {
	var ports []corev1.ServicePort

	// If custom ports are specified, use them
	if opNode.Spec.Service != nil && len(opNode.Spec.Service.Ports) > 0 {
		for _, portConfig := range opNode.Spec.Service.Ports {
			port := corev1.ServicePort{
				Name:       portConfig.Name,
				Port:       portConfig.Port,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(portConfig.Port),
			}

			if portConfig.TargetPort.IntVal != 0 || portConfig.TargetPort.StrVal != "" {
				port.TargetPort = portConfig.TargetPort
			}

			if portConfig.Protocol != "" {
				port.Protocol = portConfig.Protocol
			}

			ports = append(ports, port)
		}
	} else {
		// Default ports based on configuration
		ports = buildDefaultServicePorts(opNode)
	}

	return ports
}

// buildDefaultServicePorts creates default service ports based on OpNode configuration
func buildDefaultServicePorts(opNode *optimismv1alpha1.OpNode) []corev1.ServicePort {
	var ports []corev1.ServicePort

	// op-geth HTTP RPC port
	if opNode.Spec.OpGeth.Networking != nil &&
		opNode.Spec.OpGeth.Networking.HTTP != nil &&
		opNode.Spec.OpGeth.Networking.HTTP.Enabled {
		port := getDefaultInt32(opNode.Spec.OpGeth.Networking.HTTP.Port, 8545)
		ports = append(ports, corev1.ServicePort{
			Name:       "geth-http",
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	// op-geth WebSocket port
	if opNode.Spec.OpGeth.Networking != nil &&
		opNode.Spec.OpGeth.Networking.WS != nil &&
		opNode.Spec.OpGeth.Networking.WS.Enabled {
		port := getDefaultInt32(opNode.Spec.OpGeth.Networking.WS.Port, 8546)
		ports = append(ports, corev1.ServicePort{
			Name:       "geth-ws",
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	// op-geth P2P port
	if opNode.Spec.OpGeth.Networking != nil && opNode.Spec.OpGeth.Networking.P2P != nil {
		port := getDefaultInt32(opNode.Spec.OpGeth.Networking.P2P.Port, 30303)
		ports = append(ports, corev1.ServicePort{
			Name:       "geth-p2p",
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	// op-node RPC port
	if opNode.Spec.OpNode.RPC != nil && opNode.Spec.OpNode.RPC.Enabled {
		port := getDefaultInt32(opNode.Spec.OpNode.RPC.Port, 9545)
		ports = append(ports, corev1.ServicePort{
			Name:       "node-rpc",
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	// op-node P2P port
	if opNode.Spec.OpNode.P2P != nil && opNode.Spec.OpNode.P2P.Enabled {
		port := getDefaultInt32(opNode.Spec.OpNode.P2P.ListenPort, 9003)
		ports = append(ports, corev1.ServicePort{
			Name:       "node-p2p",
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	// Metrics port (if enabled)
	// We'll assume metrics are enabled for service creation
	ports = append(ports, corev1.ServicePort{
		Name:       "metrics",
		Port:       7300,
		TargetPort: intstr.FromInt32(7300),
		Protocol:   corev1.ProtocolTCP,
	})

	return ports
}
