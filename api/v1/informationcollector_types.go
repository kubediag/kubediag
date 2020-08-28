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

// InformationCollectorSpec defines the desired state of InformationCollector.
type InformationCollectorSpec struct {
	// IP is the serving ip of the information collector.
	IP string `json:"ip"`
	// Port is the serving port of the information collector.
	Port string `json:"port"`
	// Path is the serving http path of information collector.
	// +optional
	Path string `json:"path,omitempty"`
	// MetricPath is the prometheus metric path of information collector.
	// +optional
	MetricPath string `json:"metricPath,omitempty"`
	// Scheme is the serving scheme of information collector.
	// +optional
	Scheme string `json:"scheme,omitempty"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// InformationCollectorStatus defines the observed state of InformationCollector.
type InformationCollectorStatus struct {
	// Ready specifies whether the information collector has passed its readiness probe.
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// InformationCollector is the Schema for the informationcollectors API.
type InformationCollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InformationCollectorSpec   `json:"spec,omitempty"`
	Status InformationCollectorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InformationCollectorList contains a list of InformationCollector.
type InformationCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InformationCollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InformationCollector{}, &InformationCollectorList{})
}
