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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DiagnosisPending means that the diagnosis has been accepted by the system, but no operation has been started.
	DiagnosisPending DiagnosisPhase = "Pending"
	// DiagnosisRunning means the diagnosis has been bound to a node and one of the operations have been started.
	// At least one operation is still running.
	DiagnosisRunning DiagnosisPhase = "Running"
	// DiagnosisSucceeded means that all operations in some path have voluntarily terminated with a response code
	// of 200, and the system is not going to execute rest operations.
	DiagnosisSucceeded DiagnosisPhase = "Succeeded"
	// DiagnosisFailed means that all paths in the graph have terminated, and at least one operation in each path
	// terminated in a failure.
	DiagnosisFailed DiagnosisPhase = "Failed"
	// DiagnosisUnknown means that for some reason the state of the diagnosis could not be obtained, typically due
	// to an error in communicating with the host of the diagnosis.
	DiagnosisUnknown DiagnosisPhase = "Unknown"

	// DiagnosisComplete means the diagnosis has completed its execution.
	DiagnosisComplete DiagnosisConditionType = "Complete"
	// OperationSetChanged means the operation set specification has been changed during diagnosis execution.
	OperationSetChanged DiagnosisConditionType = "OperationSetChanged"
	// OperationSetNotReady means the graph has not been updated according to the latest specification.
	OperationSetNotReady DiagnosisConditionType = "OperationSetNotReady"
	// OperationSetNotFound means the operation set is not found when running Diagnosis.
	OperationSetNotFound DiagnosisConditionType = "OperationSetNotFound"
)

// DiagnosisSpec defines the desired state of Diagnosis.
type DiagnosisSpec struct {
	// OperationSet is the name of operation set which represents diagnosis pipeline to be executed.
	OperationSet string `json:"operationSet"`
	// Parameters is a set of the parameters to be passed to operations.
	// Parameters and OperationResults are encoded into a json object and sent to operation processor when
	// running diagnosis.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
	// TargetSelector contains information to calculate target node to schedule tasks on.
	TargetSelector *TargetSelector `json:"targetSelector,omitempty"`
}

// TargetSelector contains information to calculate target node to schedule tasks on.
type TargetSelector struct {
	// NodeSelector queries over a set of nodes. Tasks will be scheduled on the result nodes of matched nodes.
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
	// NodeNames specifies nodes which tasks should be scheduled on.
	NodeNames []string `json:"nodeNames,omitempty"`
	// PodSelector queries over a set of pods. A tasks will be scheduled on the node which any matched pod is on.
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`
	// PodReferences specifies pods which tasks should be scheduled on the same node.
	PodReferences []PodReference `json:"podReferences,omitempty"`
}

// DiagnosisStatus defines the observed state of Diagnosis.
type DiagnosisStatus struct {
	// Phase is a simple, high-level summary of where the diagnosis is in its lifecycle.
	// The conditions array, the reason and message fields contain more detail about the
	// pod's status.
	// There are five possible phase values:
	//
	// DiagnosisPending: The diagnosis has been accepted by the system, but no operation has been started.
	// DiagnosisRunning: The diagnosis has been bound to a node and one of the operations have been started.
	// At least one operation is still running.
	// DiagnosisSucceeded: All operations in some path have voluntarily terminated with a response code
	// of 200, and the system is not going to execute rest operations.
	// DiagnosisFailed: All paths in the graph have terminated, and at least one operation in each path
	// terminated in a failure.
	// DiagnosisUnknown: For some reason the state of the diagnosis could not be obtained, typically due
	// to an error in communicating with the host of the diagnosis.
	// +optional
	Phase DiagnosisPhase `json:"phase,omitempty"`
	// Conditions contains current service state of diagnosis.
	// +optional
	Conditions []DiagnosisCondition `json:"conditions,omitempty"`
	// StartTime is RFC 3339 date and time at which the object was acknowledged by the system.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`
	// NodeNames contains all nodes which tasks should be scheduled on.
	NodeNames []string `json:"nodeNames,omitempty"`
	// FailedPaths contains all failed paths in diagnosis pipeline.
	// The last node in the path is the one which fails to execute operation.
	// +optional
	FailedPaths []Path `json:"failedPaths,omitempty"`
	// SucceededPath is the succeeded paths in diagnosis pipeline.
	// +optional
	SucceededPath Path `json:"succeededPath,omitempty"`
	// Checkpoint is the checkpoint for resuming unfinished diagnosis.
	// +optional
	Checkpoint *Checkpoint `json:"checkpoint,omitempty"`
	// Context stores all information generated during the diagnosis execution.
	// +optional
	Context *DiagnosisContext `json:"context,omitempty"`
}

// DiagnosisCondition contains details for the current condition of this diagnosis.
type DiagnosisCondition struct {
	// Type is the type of the condition.
	Type DiagnosisConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime specifies last time the condition transitioned from one status
	// to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Message is a human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// Checkpoint is the checkpoint for resuming unfinished diagnosis.
type Checkpoint struct {
	// PathIndex is the index of current path in operation set status.
	PathIndex int `json:"pathIndex"`
	// NodeIndex is the index of current node in path.
	NodeIndex int `json:"nodeIndex"`
	// The number of desired tasks.
	Desired int `json:"desired"`
	// The number of pending and running tasks.
	Active int `json:"active"`
	// The number of tasks which reached phase Failed.
	Succeeded int `json:"succeeded"`
	// The number of tasks which reached phase Succeeded.
	Failed int `json:"failed"`
	// SynchronizedTasks contains a list of synchronized task of current checkpoint.
	SynchronizedTasks []string `json:"synchronizedTasks,omitempty"`
}

// DiagnosisContext stores all information generated during the diagnosis execution.
type DiagnosisContext struct {
	// Parameters is a set of the parameters defined in the spec.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
	// Operations contains the results of a diagnosis.
	// +optional
	Operations map[string]OperationContext `json:"operations,omitempty"`
}

// OperationContext contains the results of an operation.
type OperationContext map[string]TaskContext

// TaskContext contains the results of a task.
type TaskContext map[string]string

// DiagnosisPhase is a label for the condition of a diagnosis at the current time.
type DiagnosisPhase string

// DiagnosisConditionType is a valid value for DiagnosisCondition.Type.
type DiagnosisConditionType string

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".spec.operationSet",name=OperationSet,type=string
// +kubebuilder:printcolumn:JSONPath=".status.phase",name=Phase,type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Diagnosis is the Schema for the diagnoses API.
type Diagnosis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiagnosisSpec   `json:"spec,omitempty"`
	Status DiagnosisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DiagnosisList contains a list of Diagnosis.
type DiagnosisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Diagnosis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Diagnosis{}, &DiagnosisList{})
}
