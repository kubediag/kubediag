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

package alertmanager

import (
	"github.com/prometheus/alertmanager/types"
)

const (
	// SummaryLabel is the name of the label containing the an alert's summary.
	SummaryLabel = "summary"
	// DescriptionLabel is the name of the label containing the an alert's description.
	DescriptionLabel = "description"
	// SourceLabel is the name of the label containing the an alert's source.
	SourceLabel = "source"
	// SeverityLabel is the name of the label containing the an alert's severity.
	SeverityLabel = "severity"
	// ComponentLabel is the name of the label containing the an alert's component.
	ComponentLabel = "component"
	// GroupLabel is the name of the label containing the an alert's group.
	GroupLabel = "group"
)

type Alert types.Alert

// Summary returns the summary of the alert. It is equivalent to the "summary" label.
func (a *Alert) Summary() string {
	return string(a.Annotations[SummaryLabel])
}

// Description returns the description of the alert. It is equivalent to the "description" label.
func (a *Alert) Description() string {
	return string(a.Annotations[DescriptionLabel])
}

// Source returns the source of the alert. It is equivalent to the "source" label.
func (a *Alert) Source() string {
	return string(a.Labels[SourceLabel])
}

// Severity returns the severity of the alert. It is equivalent to the "severity" label.
func (a *Alert) Severity() string {
	return string(a.Labels[SeverityLabel])
}

// Component returns the component of the alert. It is equivalent to the "component" label.
func (a *Alert) Component() string {
	return string(a.Labels[ComponentLabel])
}

// Group returns the group of the alert. It is equivalent to the "group" label.
func (a *Alert) Group() string {
	return string(a.Labels[GroupLabel])
}
