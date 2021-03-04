/*
Copyright 2020 The Kube Diagnoser Authors.

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
	// Edges contains all edges in the directed acyclic graph which represents operation paths on running a diagnosis.
	Edges []Edge `json:"edges"`
}

// Edge is an edge in the directed acyclic graph.
// It represents an edge of the first operation in one operation path if from is nil.
type Edge struct {
	// From is the from node of the edge.
	// It represents the start of running a diagnosis if nil.
	// +optional
	From *Node `json:"from,omitempty"`
	// To is the to node of the edge.
	To Node `json:"to"`
}

// Node is a node in the directed acyclic graph. It contains id and operation name.
type Node struct {
	// ID is the unique identifier of the node.
	ID int `json:"id"`
	// Operation is the name of operation running on the node.
	Operation string `json:"operation"`
}

// OperationSetStatus defines the observed state of OperationSet.
type OperationSetStatus struct {
	// TopologicalSorts is the collection of all topological sorts of the directed acyclic graph.
	TopologicalSorts []TopologicalSort `json:"topologicalSorts"`
	// Specifies whether a valid directed acyclic graph can be generated via provided edges.
	Ready bool `json:"ready"`
}

// TopologicalSort represents a linear ordering of nodes along the direction of every directed edge.
type TopologicalSort []Node

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

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
