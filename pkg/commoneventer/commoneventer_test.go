package commoneventer

import (
	"testing"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/pagerdutyeventer"
	"github.com/stretchr/testify/assert"
)

func TestMatchCommonEventer(t *testing.T) {
	type expectedStruct struct {
		matched bool
		err     error
	}
	tests := []struct {
		commonEventTemplate diagnosisv1.CommonEventTemplate
		commonEventFormat   pagerdutyeventer.PagerDutyPayload
		expected            expectedStruct
		desc                string
	}{
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Source:    "10.10.101.101",
					Severity:  "Warning",
					Class:     "highCPU",
					Group:     "load",
					Component: "webPing",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Summary:   "Host '10.10.101.101' high CPU load ",
				Source:    "10.10.101.101",
				Severity:  "Warning",
				Class:     "highCPU",
				Group:     "load",
				Component: "webPing",
				Timestamp: "2015-07-17T08:42:58.315+0000",
				CustomDetails: map[string]string{
					"ping time": "2000ms",
					"load avg":  "0.75",
				},
			},
			expected: expectedStruct{
				matched: true,
				err:     nil,
			},
			desc: "exact match",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Summary:   "Host '10.10.101.101' high CPU load ",
				Source:    "10.10.101.101",
				Severity:  "Warning",
				Class:     "highCPU",
				Group:     "load",
				Component: "webPing",
				Timestamp: "2015-07-17T08:42:58.315+0000",
				CustomDetails: map[string]string{
					"ping time": "2000ms",
					"load avg":  "0.75",
				},
			},
			expected: expectedStruct{
				matched: true,
				err:     nil,
			},
			desc: "empty commonEventTemplate",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Source:    "10.10.101.101",
					Severity:  "Warning",
					Class:     "highCPU",
					Group:     "load",
					Component: "webPing",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "empty common event",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Source: "10.10.101.101",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Source: "10.10.102.102",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "source not match",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Severity: "Warning",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Severity: "Error",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "severity not match",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Class: "load",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Class: "web",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "class not match",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Component: "highLoad",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Component: "webPing",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "component not match",
		},
		{
			commonEventTemplate: diagnosisv1.CommonEventTemplate{
				Regexp: diagnosisv1.CommonEventTemplateRegexp{
					Group: "load",
				},
			},
			commonEventFormat: pagerdutyeventer.PagerDutyPayload{
				Group: "www",
			},
			expected: expectedStruct{
				matched: false,
				err:     nil,
			},
			desc: "group not match",
		},
	}
	for _, test := range tests {
		matched, err := matchCommonEvent(test.commonEventTemplate, test.commonEventFormat)
		assert.Equal(t, test.expected.matched, matched, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}
