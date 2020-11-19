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

package clusterhealthevaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"netease.com/k8s/kube-diagnoser/pkg/types"
)

func TestCalculatePodHealthScore(t *testing.T) {
	tests := []struct {
		statistics types.PodStatistics
		expected   int
		desc       string
	}{
		{
			statistics: types.PodStatistics{},
			expected:   100,
			desc:       "empty pod statistics",
		},
		{
			statistics: types.PodStatistics{
				Total: 50,
			},
			expected: 0,
			desc:     "empty healthy pod statistics",
		},
		{
			statistics: types.PodStatistics{
				Total: 50,
				Healthy: types.HealthyPodStatistics{
					Ready:     10,
					Succeeded: 10,
				},
			},
			expected: 40,
			desc:     "score calculated",
		},
	}

	for _, test := range tests {
		score := calculatePodHealthScore(test.statistics)
		assert.Equal(t, test.expected, score, test.desc)
	}
}

func TestCalculateDeploymentHealthScore(t *testing.T) {
	tests := []struct {
		statistics types.DeploymentStatistics
		expected   int
		desc       string
	}{
		{
			statistics: types.DeploymentStatistics{},
			expected:   100,
			desc:       "empty deployment statistics",
		},
		{
			statistics: types.DeploymentStatistics{
				Total: 50,
			},
			expected: 0,
			desc:     "empty healthy deployment statistics",
		},
		{
			statistics: types.DeploymentStatistics{
				Total:   100,
				Healthy: 50,
				Unhealthy: types.UnhealthyDeploymentStatistics{
					OneQuarterAvailable:    5,
					TwoQuartersAvailable:   10,
					ThreeQuartersAvailable: 15,
					FourQuartersAvailable:  20,
				},
			},
			expected: 50*1 + int(5.0*0+10.0*0.25+15.0*0.5+20.0*0.75),
			desc:     "score calculated",
		},
	}

	for _, test := range tests {
		score := calculateDeploymentHealthScore(test.statistics)
		assert.Equal(t, test.expected, score, test.desc)
	}
}

func TestCalculateStatefulSetHealthScore(t *testing.T) {
	tests := []struct {
		statistics types.StatefulSetStatistics
		expected   int
		desc       string
	}{
		{
			statistics: types.StatefulSetStatistics{},
			expected:   100,
			desc:       "empty statefulset statistics",
		},
		{
			statistics: types.StatefulSetStatistics{
				Total: 50,
			},
			expected: 0,
			desc:     "empty healthy statefulset statistics",
		},
		{
			statistics: types.StatefulSetStatistics{
				Total:   100,
				Healthy: 50,
				Unhealthy: types.UnhealthyStatefulSetStatistics{
					OneQuarterReady:    5,
					TwoQuartersReady:   10,
					ThreeQuartersReady: 15,
					FourQuartersReady:  20,
				},
			},
			expected: 50*1 + int(5.0*0+10.0*0.25+15.0*0.5+20.0*0.75),
			desc:     "score calculated",
		},
	}

	for _, test := range tests {
		score := calculateStatefulSetHealthScore(test.statistics)
		assert.Equal(t, test.expected, score, test.desc)
	}
}

func TestCalculateDaemonSetHealthScore(t *testing.T) {
	tests := []struct {
		statistics types.DaemonSetStatistics
		expected   int
		desc       string
	}{
		{
			statistics: types.DaemonSetStatistics{},
			expected:   100,
			desc:       "empty daemonset statistics",
		},
		{
			statistics: types.DaemonSetStatistics{
				Total: 50,
			},
			expected: 0,
			desc:     "empty healthy daemonset statistics",
		},
		{
			statistics: types.DaemonSetStatistics{
				Total:   100,
				Healthy: 50,
				Unhealthy: types.UnhealthyDaemonSetStatistics{
					OneQuarterAvailableAndScheduled:    5,
					TwoQuartersAvailableAndScheduled:   10,
					ThreeQuartersAvailableAndScheduled: 15,
					FourQuartersAvailableAndScheduled:  20,
				},
			},
			expected: 50*1 + int(5.0*0+10.0*0.25+15.0*0.5+20.0*0.75),
			desc:     "score calculated",
		},
	}

	for _, test := range tests {
		score := calculateDaemonSetHealthScore(test.statistics)
		assert.Equal(t, test.expected, score, test.desc)
	}
}

func TestCalculateNodeHealthScore(t *testing.T) {
	tests := []struct {
		statistics types.NodeStatistics
		expected   int
		desc       string
	}{
		{
			statistics: types.NodeStatistics{},
			expected:   100,
			desc:       "empty node statistics",
		},
		{
			statistics: types.NodeStatistics{
				Total: 50,
			},
			expected: 0,
			desc:     "empty healthy node statistics",
		},
		{
			statistics: types.NodeStatistics{
				Total:   50,
				Healthy: 20,
			},
			expected: 40,
			desc:     "score calculated",
		},
	}

	for _, test := range tests {
		score := calculateNodeHealthScore(test.statistics)
		assert.Equal(t, test.expected, score, test.desc)
	}
}