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
	// ExternalIP is the external serving ip of the recoverer.
	// +optional
	ExternalIP *string `json:"externalIP,omitempty"`
	// ExternalPort is the external serving port of the recoverer.
	// +optional
	ExternalPort *int32 `json:"externalPort,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Recoverer is the Schema for the recoverers API.
type Recoverer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RecovererSpec `json:"spec,omitempty"`
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
