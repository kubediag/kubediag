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

// DiagnoserSpec defines the desired state of Diagnoser.
type DiagnoserSpec struct {
	// IP is the serving ip of the diagnoser.
	IP string `json:"ip"`
	// Port is the serving port of the diagnoser.
	Port string `json:"port"`
	// Path is the serving http path of diagnoser.
	// +optional
	Path string `json:"path,omitempty"`
	// Scheme is the serving scheme of diagnoser.
	// +optional
	Scheme string `json:"scheme,omitempty"`
	// Number of seconds after which the diagnoser times out.
	// Defaults to 1 second. Minimum value is 1.
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// DiagnoserStatus defines the observed state of Diagnoser.
type DiagnoserStatus struct {
	// Ready specifies whether the diagnoser has passed its readiness probe.
	Ready bool `json:"ready"`
	// LastDiagnosis contains details about last diagnosis executed by this diagnoser.
	// +optional
	LastDiagnosis *Diagnosis `json:"lastDiagnosis,omitempty"`
}

// Diagnosis contains details about a diagnosis.
type Diagnosis struct {
	// StartTime specifies the known start time for this diagnosis.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`
	// EndTime specifies the known end time for this diagnosis.
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
	// Abnormal specifies details about last abnormal which has been successfully diagnosed.
	// +optional
	Abnormal metav1.ObjectMeta `json:"abnormal,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Diagnoser is the Schema for the diagnosers API.
type Diagnoser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiagnoserSpec   `json:"spec,omitempty"`
	Status DiagnoserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DiagnoserList contains a list of Diagnoser.
type DiagnoserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Diagnoser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Diagnoser{}, &DiagnoserList{})
}
