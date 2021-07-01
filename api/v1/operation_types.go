/*
Copyright 2020 The KubeDiag Authors.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperationSpec defines the desired state of Operation.
type OperationSpec struct {
	// Processor describes how to register a operation processor into kubediag.
	Processor Processor `json:"processor"`
	// Dependences is the list of all depended operations required to be precedently executed.
	// +optional
	Dependences []string `json:"dependences,omitempty"`
	// Storage represents the type of storage for operation results.
	// Operation results will not be stored if nil.
	// +optional
	Storage *Storage `json:"storage,omitempty"`
}

// Processor describes how to register a operation processor into kubediag.
type Processor struct {
	// ExternalIP is the external serving ip of the processor.
	// Defaults to kubediag agent advertised address if not specified.
	// +optional
	ExternalIP *string `json:"externalIP,omitempty"`
	// ExternalPort is the external serving port of the processor.
	// Defaults to kubediag agent serving port if not specified.
	// +optional
	ExternalPort *int32 `json:"externalPort,omitempty"`
	// Path is the serving http path of processor.
	// +optional
	Path *string `json:"path,omitempty"`
	// Scheme is the serving scheme of processor.
	// +optional
	Scheme *string `json:"scheme,omitempty"`
	// Number of seconds after which the processor times out.
	// Defaults to 30 seconds. Minimum value is 1.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// Storage represents the type of storage for operation results.
type Storage struct {
	// HostPath represents a directory on the host.
	// +optional
	HostPath *HostPath `json:"hostPath,omitempty"`
}

// HostPath represents a directory on the host.
type HostPath struct {
	// Path of the directory on the host.
	// Defaults to kubediag agent data root if not specified.
	Path string `json:"path"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// Operation is the Schema for the operations API.
type Operation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OperationSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// OperationList contains a list of Operation.
type OperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Operation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Operation{}, &OperationList{})
}
