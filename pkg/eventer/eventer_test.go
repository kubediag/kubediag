/*
Copyright 2021 The KubeDiag Authors.

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

package eventer

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestMatchKubernetesEvent(t *testing.T) {
	type expectedStruct struct {
		matched bool
		err     error
	}

	tests := []struct {
		kubernetesEventTemplate diagnosisv1.KubernetesEventTemplate
		event                   corev1.Event
		expected                expectedStruct
		desc                    string
	}{
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Name:      "event1",
					Namespace: "namespace1",
					Reason:    "reason1",
					Message:   "message1",
					Source: corev1.EventSource{
						Component: "component1",
						Host:      "host1",
					},
				},
			},
			event: corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event1",
					Namespace: "namespace1",
				},
				Reason:  "reason1",
				Message: "message1",
				Source: corev1.EventSource{
					Component: "component1",
					Host:      "host1",
				},
			},
			expected: expectedStruct{
				matched: true,
				err:     nil,
			},
			desc: "exact match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{},
			event: corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event1",
					Namespace: "namespace1",
				},
			},
			expected: expectedStruct{
				matched: true,
				err:     nil,
			},
			desc: "empty kubernetes event template",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Name:      "event1",
					Namespace: "namespace1",
				},
			},
			event: corev1.Event{},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "empty diagnosis kubernetes event",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Name: "event1",
				},
			},
			event: corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name: "event2",
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "event name not match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Namespace: "namespace1",
				},
			},
			event: corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace2",
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "event namespace not match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Reason: "reason1",
				},
			},
			event: corev1.Event{
				Reason: "reason2",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "event reason not match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Message: "message1",
				},
			},
			event: corev1.Event{
				Message: "message2",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "event message not match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Source: corev1.EventSource{
						Component: "component1",
					},
				},
			},
			event: corev1.Event{
				Source: corev1.EventSource{
					Component: "component2",
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "event source component not match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Source: corev1.EventSource{
						Host: "host1",
					},
				},
			},
			event: corev1.Event{
				Source: corev1.EventSource{
					Host: "host2",
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "event source host not match",
		},
		{
			kubernetesEventTemplate: diagnosisv1.KubernetesEventTemplate{
				Regexp: diagnosisv1.KubernetesEventTemplateRegexp{
					Name: "(event1",
				},
			},
			event: corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name: "event1",
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     fmt.Errorf("error parsing regexp: missing closing ): `(event1`"),
			},
			desc: "invalid regular expression pattern",
		},
	}

	for _, test := range tests {
		matched, err := matchKubernetesEvent(test.kubernetesEventTemplate, test.event)
		assert.Equal(t, test.expected.matched, matched, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}
