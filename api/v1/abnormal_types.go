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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// KubernetesEventSource means that the abnormal is detected via kubernetes event.
	KubernetesEventSource AbnormalSource = "KubernetesEvent"
	// CustomSource means that the abnormal is a customized abnormal created by user.
	CustomSource AbnormalSource = "Custom"

	// InformationCollecting means that the information manager is sending abnormal to assigned
	// information collectors.
	InformationCollecting AbnormalPhase = "InformationCollecting"
	// AbnormalDiagnosing means that the abnormal has been passed to diagnoser chain and some of
	// the diagnosers have been started. At least one diagnoser is still running.
	AbnormalDiagnosing AbnormalPhase = "Diagnosing"
	// AbnormalRecovering means that the abnormal has been passed to recoverer chain and some of
	// the recoverers have been started. At least one recoverer is still running.
	AbnormalRecovering AbnormalPhase = "Recovering"
	// AbnormalSucceeded means that the abnormal has been successfully recovered by some of
	// the recoverers.
	AbnormalSucceeded AbnormalPhase = "Succeeded"
	// AbnormalFailed means that all diagnosers and recoverers have been executed, and none of
	// diagnosers and recoverers is able to diagnose and recover the abnormal.
	AbnormalFailed AbnormalPhase = "Failed"
	// AbnormalUnknown means that for some reason the state of the abnormal could not be obtained.
	AbnormalUnknown AbnormalPhase = "Unknown"

	// InformationCollectorType means that the command executor is an information collector.
	InformationCollectorType AbnormalProcessorType = "InformationCollector"
	// DiagnoserType means that the command executor is an diagnoser.
	DiagnoserType AbnormalProcessorType = "Diagnoser"
	// RecovererType means that the command executor is an recoverer.
	RecovererType AbnormalProcessorType = "Recoverer"

	// InformationCollected means that the abnormal has been passed to information manager.
	InformationCollected AbnormalConditionType = "InformationCollected"
	// AbnormalIdentified means that the abnormal has been identified by the diagnoser chain.
	AbnormalIdentified AbnormalConditionType = "Identified"
	// AbnormalRecovered means that the abnormal has been recovered by the recoverer chain.
	AbnormalRecovered AbnormalConditionType = "Recovered"
)

// AbnormalSpec defines the desired state of Abnormal.
type AbnormalSpec struct {
	// Source is the abnormal source. Valid sources are KubernetesEvent and Custom.
	Source AbnormalSource `json:"source"`
	// KubernetesEvent contains the kubernetes event about the abnormal from kubernetes
	// event source. This must be specified if abnormal source is KubernetesEvent.
	// +optional
	KubernetesEvent *corev1.Event `json:"kubernetesEvent,omitempty"`
	// SkipInformationCollection indicates whether the information collection should be skipped.
	// +optional
	SkipInformationCollection bool `json:"skipInformationCollection,omitempty"`
	// SkipDiagnosis indicates whether the diagnosis should be skipped.
	// +optional
	SkipDiagnosis bool `json:"skipDiagnosis,omitempty"`
	// SkipRecovery indicates whether the recovery should be skipped.
	// +optional
	SkipRecovery bool `json:"skipRecovery,omitempty"`
	// NodeName is a specific node which the abnormal is on.
	NodeName string `json:"nodeName"`
	// AssignedInformationCollectors is the list of information collectors to execute
	// information collecting logics. Information collectors would be executed in the
	// specified sequence. No extra information collectors will be executed if the list
	// is empty.
	// +optional
	AssignedInformationCollectors []NamespacedName `json:"assignedInformationCollectors,omitempty"`
	// AssignedDiagnosers is the list of diagnosers to execute diagnosing logics.
	// Diagnosers would be executed in the specified sequence. All diagnosers will
	// be executed until the abnormal is diagnosed if the list is empty.
	// +optional
	AssignedDiagnosers []NamespacedName `json:"assignedDiagnosers,omitempty"`
	// AssignedRecoverers is the list of recoverers to execute recovering logics.
	// Recoverers would be executed in the specified sequence. All recoverers will
	// be executed until the abnormal is recovered if the list is empty.
	// +optional
	AssignedRecoverers []NamespacedName `json:"assignedRecoverers,omitempty"`
	// CommandExecutors is the list of commands to execute during information collecting, diagnosing
	// and recovering.
	// +optional
	CommandExecutors []CommandExecutor `json:"commandExecutors,omitempty"`
	// Context is a blob of information about the abnormal, meant to be user-facing
	// content and display instructions. This field may contain customized values for
	// custom source.
	// +optional
	Context *runtime.RawExtension `json:"context,omitempty"`
}

// AbnormalSource is the source of abnormals.
type AbnormalSource string

// NamespacedName represents a kubernetes api resource.
type NamespacedName struct {
	// Namespace specifies the namespace of a kubernetes api resource.
	Namespace string `json:"namespace"`
	// Name specifies the name of a kubernetes api resource.
	Name string `json:"name"`
}

// CommandExecutor executes a command with the given arguments. A CommandExecutor could be an
// information collector, a diagnoser or a recoverer.
type CommandExecutor struct {
	// Command represents a command being prepared and run.
	Command []string `json:"command"`
	// Type is the type of the command executor. There are three possible type values:
	//
	// InformationCollector: The command executor will be run by information manager.
	// Diagnoser: The command executor will be run by diagnoser chain.
	// Recoverer: The command executor will be run by recoverer chain.
	Type AbnormalProcessorType `json:"type"`
	// Stdout is standard output of the command.
	// +optional
	Stdout string `json:"stdout,omitempty"`
	// Stderr is standard error of the command.
	// +optional
	Stderr string `json:"stderr,omitempty"`
	// Error is the command execution error.
	// +optional
	Error string `json:"error,omitempty"`
	// Number of seconds after which the command times out.
	// Defaults to 1 second. Minimum value is 1.
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// AbnormalStatus defines the observed state of Abnormal.
type AbnormalStatus struct {
	// Identifiable indicates if the abnormal could be identified by the diagnoser chain.
	Identifiable bool `json:"identifiable"`
	// Recoverable indicates if the abnormal could be recovered by the recoverer chain.
	Recoverable bool `json:"recoverable"`
	// Phase is a simple, high-level summary of where the abnormal is in its lifecycle.
	// The conditions array, the reason and message fields contain more detail about the
	// pod's status.
	// There are six possible phase values:
	//
	// Pending: The abnormal has been accepted by the system, but diagnosis and recovery have
	// not been started.
	// Diagnosing: The abnormal has been passed to diagnoser chain and some of the diagnosers
	// have been started. At least one diagnoser is still running.
	// Recovering: The abnormal has been passed to recoverer chain and some of the recoverers
	// have been started. At least one recoverer is still running.
	// Succeeded: The abnormal has been successfully recovered by some of the recoverers.
	// Failed: All diagnosers and recoverers have been executed, and none of diagnosers and
	// recoverers is able to diagnose and recover the abnormal.
	// Unknown: For some reason the state of the abnormal could not be obtained.
	// +optional
	Phase AbnormalPhase `json:"phase,omitempty"`
	// Conditions contains current service state of abnormal.
	// +optional
	Conditions []AbnormalCondition `json:"conditions,omitempty"`
	// Message is a human readable message indicating details about why the abnormal is in
	// this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// Reason is a brief CamelCase message indicating details about why the abnormal is in
	// this state.
	// +optional
	Reason string `json:"reason,omitempty"`
	// StartTime is RFC 3339 date and time at which the object was acknowledged by the system.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`
	// Diagnoser indicates the diagnoser which has identified the abnormal successfully.
	// +optional
	Diagnoser *NamespacedName `json:"diagnoser,omitempty"`
	// Recoverer indicates the recoverer which has recovered the abnormal successfully.
	// +optional
	Recoverer *NamespacedName `json:"recoverer,omitempty"`
	// CommandExecutors is the list of commands to execute during information collecting, diagnosing
	// and recovering.
	// +optional
	CommandExecutors []CommandExecutor `json:"commandExecutors,omitempty"`
	// Context is a blob of information about the abnormal, meant to be user-facing
	// content and display instructions. This field may contain customized values for
	// custom source.
	// +optional
	Context *runtime.RawExtension `json:"context,omitempty"`
}

// AbnormalCondition contains details for the current condition of this abnormal.
type AbnormalCondition struct {
	// Type is the type of the condition.
	Type AbnormalConditionType `json:"type"`
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

// AbnormalPhase is a label for the condition of a abnormal at the current time.
type AbnormalPhase string

// AbnormalProcessorType is a valid for CommandExecutor.Type.
type AbnormalProcessorType string

// AbnormalConditionType is a valid value for AbnormalCondition.Type.
type AbnormalConditionType string

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Abnormal is the Schema for the abnormals API.
type Abnormal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AbnormalSpec   `json:"spec,omitempty"`
	Status AbnormalStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AbnormalList contains a list of Abnormal.
type AbnormalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Abnormal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Abnormal{}, &AbnormalList{})
}
