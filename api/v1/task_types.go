/*
Copyright 2022 The KubeDiag Authors.

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
	// TaskPending means that the task has been accepted by the system, but the operation has not been started.
	TaskPending TaskPhase = "Pending"
	// TaskRunning means the task has been bound to a node and the operations has been started.
	// At least one operation is still running.
	TaskRunning TaskPhase = "Running"
	// TaskSucceeded means that the operationhas voluntarily terminated with success.
	TaskSucceeded TaskPhase = "Succeeded"
	// TaskFailed means that the operation has terminated in a failure.
	TaskFailed TaskPhase = "Failed"
	// TaskUnknown means that for some reason the state of the task could not be obtained, typically due
	// to an error in communicating with the host of the task.
	TaskUnknown TaskPhase = "Unknown"

	// TaskAccepted means that the task has been accepted by kubediag agent.
	TaskAccepted TaskConditionType = "Accepted"
	// TaskComplete means the task has completed its execution.
	TaskComplete TaskConditionType = "Complete"
	// TaskIncomplete means that the operation fails to complete with success.
	TaskIncomplete TaskConditionType = "Failed"
	// OperationNotFound means the operation is not found when running Task.
	OperationNotFound TaskConditionType = "OperationNotFound"
)

// TaskPhase is a label for the condition of a task at the current time.
type TaskPhase string

// TaskConditionType is a valid value for TaskCondition.Type.
type TaskConditionType string

// TaskSpec defines the desired state of Task.
type TaskSpec struct {
	// Operation is the name of operation which represents task to be executed.
	Operation string `json:"operation"`
	// One of NodeName and PodReference must be specified.
	// NodeName is a specific node which the task is on.
	// +optional
	NodeName string `json:"nodeName,omitempty"`
	// PodReference contains details of the target pod.
	// +optional
	PodReference *PodReference `json:"podReference,omitempty"`
	// Parameters is a set of the parameters to be passed to opreations.
	// Parameters and Results are encoded into a json object and sent to operation processor when
	// running a task.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// PodReference contains details of the target pod.
type PodReference struct {
	NamespacedName `json:",inline"`
	// Container specifies name of the target container.
	// +optional
	Container string `json:"container,omitempty"`
}

// NamespacedName represents a kubernetes api resource.
type NamespacedName struct {
	// Namespace specifies the namespace of a kubernetes api resource.
	Namespace string `json:"namespace"`
	// Name specifies the name of a kubernetes api resource.
	Name string `json:"name"`
}

// TaskStatus defines the observed state of Task
type TaskStatus struct {
	// Phase is a simple, high-level summary of where the task is in its lifecycle.
	// The conditions array, the reason and message fields contain more detail about the
	// pod's status.
	// There are five possible phase values:
	//
	// TaskPending: The task has been accepted by the system, but no operation has been started.
	// TaskRunning: The task has been bound to a node and the operation has been started.
	// TaskSucceeded: The task has voluntarily terminated a response code of 200.
	// TaskFailed: The task has terminated in a failure.
	// TaskUnknown: For some reason the state of the task could not be obtained, typically due to an error
	// in communicating with the host of the task.
	// +optional
	Phase TaskPhase `json:"phase,omitempty"`
	// Conditions contains current service state of task.
	// +optional
	Conditions []TaskCondition `json:"conditions,omitempty"`
	// StartTime is RFC 3339 date and time at which the object was acknowledged by the system.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`
	// Results contains results of a task.
	// Parameters and Results are encoded into a json object and sent to operation processor when running task.
	// +optional
	Results map[string]string `json:"results,omitempty"`
}

// TaskCondition contains details for the current condition of this diagnosis.
type TaskCondition struct {
	// Type is the type of the condition.
	Type TaskConditionType `json:"type"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".spec.operation",name=Operation,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.nodeName",name=NodeName,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.podReference.namespace",name=PodNamespace,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.podReference.name",name=PodName,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.podReference.container",name=PodContainer,type=string
// +kubebuilder:printcolumn:JSONPath=".status.phase",name=Phase,type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Task is the Schema for the tasks API.
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec,omitempty"`
	Status TaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TaskList contains a list of Task.
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Task{}, &TaskList{})
}
