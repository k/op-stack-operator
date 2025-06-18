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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpBatcherSpec defines the desired state of OpBatcher.
type OpBatcherSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of OpBatcher. Edit opbatcher_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// OpBatcherStatus defines the observed state of OpBatcher.
type OpBatcherStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpBatcher is the Schema for the opbatchers API.
type OpBatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpBatcherSpec   `json:"spec,omitempty"`
	Status OpBatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpBatcherList contains a list of OpBatcher.
type OpBatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpBatcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpBatcher{}, &OpBatcherList{})
}
