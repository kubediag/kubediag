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

// OperationSetSpec defines the desired state of OperationSet.
type OperationSetSpec struct {
	// AdjacencyList contains all nodes in the directed acyclic graph. The first node in the list represents the
	// start of a diagnosis.
	AdjacencyList []Node `json:"adjacencyList"`
}

// Node is a node in the directed acyclic graph. It contains details of the operation.
type Node struct {
	// ID is the unique identifier of the node.
	// It is identical to node index in adjacency list and set by admission webhook server.
	// +optional
	ID int `json:"id,omitempty"`
	// To is the list of node ids this node links to.
	// +optional
	To NodeSet `json:"to,omitempty"`
	// Operation is the name of operation running on the node.
	// It is empty if the node is the first in the list.
	// +optional
	Operation string `json:"operation,omitempty"`
	// Dependences is the list of depended node ids.
	// +optional
	Dependences NodeSet `json:"dependences,omitempty"`
}

// NodeSet is the set of node ids.
type NodeSet []int

// OperationSetStatus defines the observed state of OperationSet.
type OperationSetStatus struct {
	// Paths is the collection of all directed paths of the directed acyclic graph.
	// +optional
	Paths []Path `json:"paths,omitempty"`
	// Specifies whether a valid directed acyclic graph can be generated via provided nodes.
	Ready bool `json:"ready"`
}

// Path represents a linear ordering of nodes along the direction of every directed edge.
type Path []Node

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:JSONPath=".status.ready",name=Ready,type=boolean
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OperationSet is the Schema for the operationsets API.
type OperationSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperationSetSpec   `json:"spec,omitempty"`
	Status OperationSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OperationSetList contains a list of OperationSet.
type OperationSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperationSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperationSet{}, &OperationSetList{})
}
