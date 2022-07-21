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

package pagerdutyeventer

// PagerDutyMessage contains a message with the payload in PD-CEF.
// See more details at https://support.pagerduty.com/docs/pd-cef.
// This code block refers to https://github.com/prometheus/alertmanager/blob/v0.24.0/notify/pagerduty/pagerduty.go#L75.
type PagerDutyMessage struct {
	RoutingKey  string            `json:"routing_key,omitempty"`
	ServiceKey  string            `json:"service_key,omitempty"`
	DedupKey    string            `json:"dedup_key,omitempty"`
	IncidentKey string            `json:"incident_key,omitempty"`
	EventType   string            `json:"event_type,omitempty"`
	Description string            `json:"description,omitempty"`
	EventAction string            `json:"event_action"`
	Payload     *PagerDutyPayload `json:"payload"`
	Client      string            `json:"client,omitempty"`
	ClientURL   string            `json:"client_url,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Images      []PagerDutyImage  `json:"images,omitempty"`
	Links       []PagerDutyLink   `json:"links,omitempty"`
}

// PagerDutyPayload contains the details of each PD-CEF field.
// See more details at https://support.pagerduty.com/docs/pd-cef.
type PagerDutyPayload struct {
	Summary       string            `json:"summary"`
	Source        string            `json:"source"`
	Severity      string            `json:"severity"`
	Timestamp     string            `json:"timestamp,omitempty"`
	Class         string            `json:"class,omitempty"`
	Component     string            `json:"component,omitempty"`
	Group         string            `json:"group,omitempty"`
	CustomDetails map[string]string `json:"custom_details,omitempty"`
}

type PagerDutyLink struct {
	HRef string `json:"href"`
	Text string `json:"text"`
}

type PagerDutyImage struct {
	Src  string `json:"src"`
	Alt  string `json:"alt"`
	Href string `json:"href"`
}
