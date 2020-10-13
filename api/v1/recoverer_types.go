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

// RecovererSpec defines the desired state of Recoverer.
type RecovererSpec struct {
	// IP is the serving ip of the recoverer.
	IP string `json:"ip"`
	// Port is the serving port of the recoverer.
	Port int32 `json:"port"`
	// Path is the serving http path of recoverer.
	// +optional
	Path string `json:"path,omitempty"`
	// Scheme is the serving scheme of recoverer.
	// +optional
	Scheme string `json:"scheme,omitempty"`
	// Number of seconds after which the recoverer times out.
	// Defaults to 30 seconds. Minimum value is 1.
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// RecovererStatus defines the observed state of Recoverer.
type RecovererStatus struct {
	// Ready specifies whether the recoverer has passed its readiness probe.
	Ready bool `json:"ready"`
	// LastRecovery contains details about last recovery executed by this recoverer.
	// +optional
	LastRecovery *Recovery `json:"lastRecovery,omitempty"`
}

// Recovery contains details about a recovery.
type Recovery struct {
	// StartTime specifies the known start time for this recovery.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`
	// EndTime specifies the known end time for this recovery.
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
	// Abnormal specifies details about last abnormal which has been successfully recovered.
	// +optional
	Abnormal metav1.ObjectMeta `json:"abnormal,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Recoverer is the Schema for the recoverers API.
type Recoverer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RecovererSpec   `json:"spec,omitempty"`
	Status RecovererStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RecovererList contains a list of Recoverer.
type RecovererList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Recoverer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Recoverer{}, &RecovererList{})
}
