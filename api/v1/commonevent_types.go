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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommonEventSpec defines the desired state of CommonEvent.
type CommonEventSpec struct {
	Summary       string            `json:"summary"`
	Source        string            `json:"source"`
	Severity      string            `json:"severity"`
	Timestamp     string            `json:"timestamp,omitempty"`
	Class         string            `json:"class,omitempty"`
	Component     string            `json:"component,omitempty"`
	Group         string            `json:"group,omitempty"`
	CustomDetails map[string]string `json:"customDetails,omitempty"`
}

// CommonEventStatus defines the observed state of CommonEvent.
type CommonEventStatus struct {
	Count          int          `json:"count,omitempty"`
	Resolved       bool         `json:"resolved,omitempty"`
	Diagnosed      bool         `json:"diagnosed,omitempty"`
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CommonEvent is the Schema for the commonevents API.
type CommonEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommonEventSpec   `json:"spec,omitempty"`
	Status CommonEventStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CommonEventList contains a list of CommonEvent.
type CommonEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CommonEvent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CommonEvent{}, &CommonEventList{})
}
