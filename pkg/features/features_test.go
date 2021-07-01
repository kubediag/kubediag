/*
Copyright 2020 The KubeDiag Authors.

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

package features

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/component-base/featuregate"
)

func TestEnabled(t *testing.T) {
	featureGate := NewFeatureGate()

	tests := []struct {
		key      featuregate.Feature
		expected bool
		desc     string
	}{
		{
			key:      Alertmanager,
			expected: true,
			desc:     "feature enabled",
		},
		{
			key:      Eventer,
			expected: false,
			desc:     "feature not enabled",
		},
		{
			key:      featuregate.Feature("TestInvalid"),
			expected: false,
			desc:     "feature not found",
		},
	}

	for _, test := range tests {
		enabled := featureGate.Enabled(test.key)
		assert.Equal(t, test.expected, enabled, test.desc)
	}
}

func TestKnownFeatures(t *testing.T) {
	featureGate := NewFeatureGate()
	knownFeatures := strings.Join(featureGate.KnownFeatures(), " ")

	tests := []struct {
		feature  featuregate.Feature
		expected bool
		desc     string
	}{
		{
			feature:  Alertmanager,
			expected: true,
			desc:     "feature known",
		},
		{
			feature:  featuregate.Feature("TestInvalid"),
			expected: false,
			desc:     "feature not known",
		},
	}

	for _, test := range tests {
		if test.expected {
			assert.Contains(t, knownFeatures, test.feature)
		} else {
			assert.NotContains(t, knownFeatures, test.feature)
		}
	}
}

func TestSetFromMap(t *testing.T) {
	type expectedStruct struct {
		enabled bool
		err     error
	}

	tests := []struct {
		featureMap map[string]bool
		feature    featuregate.Feature
		expected   expectedStruct
		desc       string
	}{
		{
			featureMap: map[string]bool{"Alertmanager": false},
			feature:    Alertmanager,
			expected: expectedStruct{
				enabled: false,
				err:     nil,
			},
			desc: "feature setted",
		},
		{
			featureMap: map[string]bool{"TestInvalid": true},
			feature:    featuregate.Feature("TestInvalid"),
			expected: expectedStruct{
				enabled: false,
				err:     fmt.Errorf("unrecognized feature gate: TestInvalid"),
			},
			desc: "feature not known",
		},
	}

	for _, test := range tests {
		featureGate := NewFeatureGate()
		err := featureGate.SetFromMap(test.featureMap)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
		enabled := featureGate.Enabled(test.feature)
		assert.Equal(t, test.expected.enabled, enabled, test.desc)
	}
}
