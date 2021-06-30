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

package alertmanager

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

func TestMatchPrometheusAlert(t *testing.T) {
	type expectedStruct struct {
		matched bool
		err     error
	}

	time := time.Now()
	tests := []struct {
		prometheusAlertTemplate diagnosisv1.PrometheusAlertTemplate
		alert                   *types.Alert
		expected                expectedStruct
		desc                    string
	}{
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					AlertName: "alert1",
					Labels: model.LabelSet{
						"alertname": "alert1",
						"node":      "node1",
					},
					Annotations: model.LabelSet{
						"message":   "message1",
						"namespace": "namespace1",
					},
					StartsAt:     regexp.QuoteMeta(time.String()),
					EndsAt:       regexp.QuoteMeta(time.String()),
					GeneratorURL: "url1",
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					Labels: model.LabelSet{
						"alertname": "alert1",
						"node":      "node1",
					},
					Annotations: model.LabelSet{
						"message":   "message1",
						"namespace": "namespace1",
					},
					StartsAt:     time,
					EndsAt:       time,
					GeneratorURL: "url1",
				},
			},
			expected: expectedStruct{
				matched: true,
				err:     nil,
			},
			desc: "exact match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{},
			alert: &types.Alert{
				Alert: model.Alert{
					Labels: model.LabelSet{
						"alertname": "alert1",
						"node":      "node1",
					},
					Annotations: model.LabelSet{
						"message":   "message1",
						"namespace": "namespace1",
					},
					StartsAt:     time,
					EndsAt:       time,
					GeneratorURL: "url1",
				},
			},
			expected: expectedStruct{
				matched: true,
				err:     nil,
			},
			desc: "empty prometheus alert template",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					AlertName: "alert1",
					Labels: model.LabelSet{
						"alertname": "alert1",
						"node":      "node1",
					},
					Annotations: model.LabelSet{
						"message":   "message1",
						"namespace": "namespace1",
					},
					StartsAt:     regexp.QuoteMeta(time.String()),
					EndsAt:       regexp.QuoteMeta(time.String()),
					GeneratorURL: "url1",
				},
			},
			alert: &types.Alert{},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "empty diagnosis prometheus alert",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					AlertName: "alert1",
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					Labels: model.LabelSet{
						"alertname": "alert2",
					},
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "alert name not match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					AlertName: "alert1",
					Labels: model.LabelSet{
						"alertname": "alert1",
						"node":      "node1",
					},
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					Labels: model.LabelSet{
						"alertname": "alert1",
						"node":      "node2",
					},
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "labels not match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					Annotations: model.LabelSet{
						"message":   "message1",
						"namespace": "namespace1",
					},
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					Annotations: model.LabelSet{
						"message":   "message1",
						"namespace": "namespace2",
					},
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "annotations not match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					StartsAt: "invalid time",
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					StartsAt: time,
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "start time not match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					EndsAt: "invalid time",
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					EndsAt: time,
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "end time not match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					GeneratorURL: "url1",
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					GeneratorURL: "url2",
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "generator url not match",
		},
		{
			prometheusAlertTemplate: diagnosisv1.PrometheusAlertTemplate{
				Regexp: diagnosisv1.PrometheusAlertTemplateRegexp{
					AlertName: "(alert1",
				},
			},
			alert: &types.Alert{
				Alert: model.Alert{
					Labels: model.LabelSet{
						"alertname": "alert1",
					},
				},
			},
			expected: expectedStruct{
				matched: false,
				err:     fmt.Errorf("error parsing regexp: missing closing ): `(alert1`"),
			},
			desc: "invalid regular expression pattern",
		},
	}

	for _, test := range tests {
		matched, err := matchPrometheusAlert(test.prometheusAlertTemplate, test.alert)
		assert.Equal(t, test.expected.matched, matched, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}
