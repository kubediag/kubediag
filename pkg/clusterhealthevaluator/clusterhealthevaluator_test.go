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
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-diagnoser/kube-diagnoser/pkg/types"
)

func TestEvaluatePodHealth(t *testing.T) {
	type expectedStruct struct {
		podHealth types.PodHealth
		err       error
	}

	time := time.Now()
	logger := log.NullLogger{}

	tests := []struct {
		pods     []corev1.Pod
		expected expectedStruct
		desc     string
	}{
		{
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 100,
					Statistics: types.PodStatistics{
						Total: 0,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               0,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{},
						},
					},
				},
				err: nil,
			},
			desc: "empty pods",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Ready: true,
							},
						},
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 100,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     1,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               0,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{},
						},
					},
				},
				err: nil,
			},
			desc: "ready pod",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Ready: false,
							},
						},
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               1,
							Terminating:           0,
							Pending:               0,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{"Unknown": 1},
						},
					},
				},
				err: nil,
			},
			desc: "unready pod",
		},
		{
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &metav1.Time{Time: time},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           1,
							Pending:               0,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{},
						},
					},
				},
				err: nil,
			},
			desc: "terminating pod",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 100,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 1,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               0,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{},
						},
					},
				},
				err: nil,
			},
			desc: "succeeded pod",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               1,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{"Unknown": 1},
						},
					},
				},
				err: nil,
			},
			desc: "pending pod",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               0,
							Failed:                1,
							Unknown:               0,
							ContainerStateReasons: map[string]int{"Unknown": 1},
						},
					},
				},
				err: nil,
			},
			desc: "failed pod",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodUnknown,
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               0,
							Failed:                0,
							Unknown:               1,
							ContainerStateReasons: map[string]int{"Unknown": 1},
						},
					},
				},
				err: nil,
			},
			desc: "unknown pod",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Ready: false,
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{
										Reason: "CrashLoopBackOff",
									},
								},
							},
						},
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               1,
							Terminating:           0,
							Pending:               0,
							Failed:                0,
							Unknown:               0,
							ContainerStateReasons: map[string]int{"CrashLoopBackOff": 1},
						},
					},
				},
				err: nil,
			},
			desc: "unready pod with waiting state",
		},
		{
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								LastTerminationState: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										Reason: "Error",
									},
								},
							},
						},
					},
				},
			},
			expected: expectedStruct{
				podHealth: types.PodHealth{
					Score: 0,
					Statistics: types.PodStatistics{
						Total: 1,
						Healthy: types.HealthyPodStatistics{
							Ready:     0,
							Succeeded: 0,
						},
						Unhealthy: types.UnhealthyPodStatistics{
							Unready:               0,
							Terminating:           0,
							Pending:               0,
							Failed:                1,
							Unknown:               0,
							ContainerStateReasons: map[string]int{"Error": 1},
						},
					},
				},
				err: nil,
			},
			desc: "failed pod with last terminated state",
		},
	}

	for _, test := range tests {
		podHealth, err := evaluatePodHealth(test.pods, logger)
		assert.Equal(t, test.expected.podHealth, podHealth, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

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

func TestEvaluateDeploymentHealth(t *testing.T) {
	type expectedStruct struct {
		deploymentHealth types.DeploymentHealth
		err              error
	}

	logger := log.NullLogger{}

	tests := []struct {
		deployments []appsv1.Deployment
		expected    expectedStruct
		desc        string
	}{
		{
			deployments: []appsv1.Deployment{
				{
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 100,
					},
				},
			},
			expected: expectedStruct{
				deploymentHealth: types.DeploymentHealth{
					Score: 100,
					Statistics: types.DeploymentStatistics{
						Total:   1,
						Healthy: 1,
						Unhealthy: types.UnhealthyDeploymentStatistics{
							OneQuarterAvailable:    0,
							TwoQuartersAvailable:   0,
							ThreeQuartersAvailable: 0,
							FourQuartersAvailable:  0,
						},
					},
				},
				err: nil,
			},
			desc: "healthy deployment",
		},
		{
			deployments: []appsv1.Deployment{
				{
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 0,
					},
				},
			},
			expected: expectedStruct{
				deploymentHealth: types.DeploymentHealth{
					Score: 0,
					Statistics: types.DeploymentStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDeploymentStatistics{
							OneQuarterAvailable:    1,
							TwoQuartersAvailable:   0,
							ThreeQuartersAvailable: 0,
							FourQuartersAvailable:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy deployment with one quarter available",
		},
		{
			deployments: []appsv1.Deployment{
				{
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 25,
					},
				},
			},
			expected: expectedStruct{
				deploymentHealth: types.DeploymentHealth{
					Score: 0,
					Statistics: types.DeploymentStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDeploymentStatistics{
							OneQuarterAvailable:    0,
							TwoQuartersAvailable:   1,
							ThreeQuartersAvailable: 0,
							FourQuartersAvailable:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy deployment with two quarters available",
		},
		{
			deployments: []appsv1.Deployment{
				{
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 50,
					},
				},
			},
			expected: expectedStruct{
				deploymentHealth: types.DeploymentHealth{
					Score: 0,
					Statistics: types.DeploymentStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDeploymentStatistics{
							OneQuarterAvailable:    0,
							TwoQuartersAvailable:   0,
							ThreeQuartersAvailable: 1,
							FourQuartersAvailable:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy deployment with three quarters available",
		},
		{
			deployments: []appsv1.Deployment{
				{
					Spec: appsv1.DeploymentSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 75,
					},
				},
			},
			expected: expectedStruct{
				deploymentHealth: types.DeploymentHealth{
					Score: 0,
					Statistics: types.DeploymentStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDeploymentStatistics{
							OneQuarterAvailable:    0,
							TwoQuartersAvailable:   0,
							ThreeQuartersAvailable: 0,
							FourQuartersAvailable:  1,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy deployment with four quarters available",
		},
	}

	for _, test := range tests {
		deploymentHealth, err := evaluateDeploymentHealth(test.deployments, logger)
		assert.Equal(t, test.expected.deploymentHealth, deploymentHealth, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
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

func TestEvaluateStatefulSetHealth(t *testing.T) {
	type expectedStruct struct {
		statefulsetHealth types.StatefulSetHealth
		err               error
	}

	logger := log.NullLogger{}

	tests := []struct {
		statefulsets []appsv1.StatefulSet
		expected     expectedStruct
		desc         string
	}{
		{
			statefulsets: []appsv1.StatefulSet{
				{
					Spec: appsv1.StatefulSetSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 100,
					},
				},
			},
			expected: expectedStruct{
				statefulsetHealth: types.StatefulSetHealth{
					Score: 100,
					Statistics: types.StatefulSetStatistics{
						Total:   1,
						Healthy: 1,
						Unhealthy: types.UnhealthyStatefulSetStatistics{
							OneQuarterReady:    0,
							TwoQuartersReady:   0,
							ThreeQuartersReady: 0,
							FourQuartersReady:  0,
						},
					},
				},
				err: nil,
			},
			desc: "healthy statefulset",
		},
		{
			statefulsets: []appsv1.StatefulSet{
				{
					Spec: appsv1.StatefulSetSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 0,
					},
				},
			},
			expected: expectedStruct{
				statefulsetHealth: types.StatefulSetHealth{
					Score: 0,
					Statistics: types.StatefulSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyStatefulSetStatistics{
							OneQuarterReady:    1,
							TwoQuartersReady:   0,
							ThreeQuartersReady: 0,
							FourQuartersReady:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy statefulset with one quarter ready",
		},
		{
			statefulsets: []appsv1.StatefulSet{
				{
					Spec: appsv1.StatefulSetSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 25,
					},
				},
			},
			expected: expectedStruct{
				statefulsetHealth: types.StatefulSetHealth{
					Score: 0,
					Statistics: types.StatefulSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyStatefulSetStatistics{
							OneQuarterReady:    0,
							TwoQuartersReady:   1,
							ThreeQuartersReady: 0,
							FourQuartersReady:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy statefulset with two quarters ready",
		},
		{
			statefulsets: []appsv1.StatefulSet{
				{
					Spec: appsv1.StatefulSetSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 50,
					},
				},
			},
			expected: expectedStruct{
				statefulsetHealth: types.StatefulSetHealth{
					Score: 0,
					Statistics: types.StatefulSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyStatefulSetStatistics{
							OneQuarterReady:    0,
							TwoQuartersReady:   0,
							ThreeQuartersReady: 1,
							FourQuartersReady:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy statefulset with three quarters ready",
		},
		{
			statefulsets: []appsv1.StatefulSet{
				{
					Spec: appsv1.StatefulSetSpec{
						Replicas: func() *int32 { var replicas int32 = 100; return &replicas }(),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 75,
					},
				},
			},
			expected: expectedStruct{
				statefulsetHealth: types.StatefulSetHealth{
					Score: 0,
					Statistics: types.StatefulSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyStatefulSetStatistics{
							OneQuarterReady:    0,
							TwoQuartersReady:   0,
							ThreeQuartersReady: 0,
							FourQuartersReady:  1,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy statefulset with four quarters ready",
		},
	}

	for _, test := range tests {
		statefulsetHealth, err := evaluateStatefulSetHealth(test.statefulsets, logger)
		assert.Equal(t, test.expected.statefulsetHealth, statefulsetHealth, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
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

func TestEvaluateDaemonSetHealth(t *testing.T) {
	type expectedStruct struct {
		daemonsetHealth types.DaemonSetHealth
		err             error
	}

	logger := log.NullLogger{}

	tests := []struct {
		daemonsets []appsv1.DaemonSet
		expected   expectedStruct
		desc       string
	}{
		{
			daemonsets: []appsv1.DaemonSet{
				{
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 100,
						NumberAvailable:        100,
					},
				},
			},
			expected: expectedStruct{
				daemonsetHealth: types.DaemonSetHealth{
					Score: 100,
					Statistics: types.DaemonSetStatistics{
						Total:   1,
						Healthy: 1,
						Unhealthy: types.UnhealthyDaemonSetStatistics{
							OneQuarterAvailableAndScheduled:    0,
							TwoQuartersAvailableAndScheduled:   0,
							ThreeQuartersAvailableAndScheduled: 0,
							FourQuartersAvailableAndScheduled:  0,
						},
					},
				},
				err: nil,
			},
			desc: "healthy daemonset",
		},
		{
			daemonsets: []appsv1.DaemonSet{
				{
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 100,
						NumberAvailable:        0,
					},
				},
			},
			expected: expectedStruct{
				daemonsetHealth: types.DaemonSetHealth{
					Score: 0,
					Statistics: types.DaemonSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDaemonSetStatistics{
							OneQuarterAvailableAndScheduled:    1,
							TwoQuartersAvailableAndScheduled:   0,
							ThreeQuartersAvailableAndScheduled: 0,
							FourQuartersAvailableAndScheduled:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy daemonset with one quarter available",
		},
		{
			daemonsets: []appsv1.DaemonSet{
				{
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 100,
						NumberAvailable:        25,
					},
				},
			},
			expected: expectedStruct{
				daemonsetHealth: types.DaemonSetHealth{
					Score: 0,
					Statistics: types.DaemonSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDaemonSetStatistics{
							OneQuarterAvailableAndScheduled:    0,
							TwoQuartersAvailableAndScheduled:   1,
							ThreeQuartersAvailableAndScheduled: 0,
							FourQuartersAvailableAndScheduled:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy daemonset with two quarters available",
		},
		{
			daemonsets: []appsv1.DaemonSet{
				{
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 100,
						NumberAvailable:        50,
					},
				},
			},
			expected: expectedStruct{
				daemonsetHealth: types.DaemonSetHealth{
					Score: 0,
					Statistics: types.DaemonSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDaemonSetStatistics{
							OneQuarterAvailableAndScheduled:    0,
							TwoQuartersAvailableAndScheduled:   0,
							ThreeQuartersAvailableAndScheduled: 1,
							FourQuartersAvailableAndScheduled:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy daemonset with three quarters available",
		},
		{
			daemonsets: []appsv1.DaemonSet{
				{
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 100,
						NumberAvailable:        75,
					},
				},
			},
			expected: expectedStruct{
				daemonsetHealth: types.DaemonSetHealth{
					Score: 0,
					Statistics: types.DaemonSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDaemonSetStatistics{
							OneQuarterAvailableAndScheduled:    0,
							TwoQuartersAvailableAndScheduled:   0,
							ThreeQuartersAvailableAndScheduled: 0,
							FourQuartersAvailableAndScheduled:  1,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy daemonset with four quarters available",
		},
		{
			daemonsets: []appsv1.DaemonSet{
				{
					Status: appsv1.DaemonSetStatus{
						DesiredNumberScheduled: 100,
						NumberAvailable:        75,
						NumberMisscheduled:     25,
					},
				},
			},
			expected: expectedStruct{
				daemonsetHealth: types.DaemonSetHealth{
					Score: 0,
					Statistics: types.DaemonSetStatistics{
						Total:   1,
						Healthy: 0,
						Unhealthy: types.UnhealthyDaemonSetStatistics{
							OneQuarterAvailableAndScheduled:    0,
							TwoQuartersAvailableAndScheduled:   0,
							ThreeQuartersAvailableAndScheduled: 1,
							FourQuartersAvailableAndScheduled:  0,
						},
					},
				},
				err: nil,
			},
			desc: "unhealthy daemonset with four quarters available and one quarter misscheduled",
		},
	}

	for _, test := range tests {
		daemonsetHealth, err := evaluateDaemonSetHealth(test.daemonsets, logger)
		assert.Equal(t, test.expected.daemonsetHealth, daemonsetHealth, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
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

func TestEvaluateWorkloadHealth(t *testing.T) {
	type expectedStruct struct {
		workloadHealth types.WorkloadHealth
		err            error
	}

	logger := log.NullLogger{}

	tests := []struct {
		podHealth         types.PodHealth
		deploymentHealth  types.DeploymentHealth
		statefulsetHealth types.StatefulSetHealth
		daemonsetHealth   types.DaemonSetHealth
		expected          expectedStruct
		desc              string
	}{
		{
			podHealth: types.PodHealth{
				Score: 80,
			},
			deploymentHealth: types.DeploymentHealth{
				Score: 60,
			},
			statefulsetHealth: types.StatefulSetHealth{
				Score: 40,
			},
			daemonsetHealth: types.DaemonSetHealth{
				Score: 20,
			},
			expected: expectedStruct{
				workloadHealth: types.WorkloadHealth{
					Score: 50,
					PodHealth: types.PodHealth{
						Score: 80,
					},
					DeploymentHealth: types.DeploymentHealth{
						Score: 60,
					},
					StatefulSetHealth: types.StatefulSetHealth{
						Score: 40,
					},
					DaemonSetHealth: types.DaemonSetHealth{
						Score: 20,
					},
				},
				err: nil,
			},
			desc: "score calculated",
		},
	}

	for _, test := range tests {
		workloadHealth, err := evaluateWorkloadHealth(test.podHealth, test.deploymentHealth, test.statefulsetHealth, test.daemonsetHealth, logger)
		assert.Equal(t, test.expected.workloadHealth, workloadHealth, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestEvaluateNodeHealth(t *testing.T) {
	type expectedStruct struct {
		nodeHealth types.NodeHealth
		err        error
	}

	logger := log.NullLogger{}

	tests := []struct {
		nodes    []corev1.Node
		expected expectedStruct
		desc     string
	}{
		{
			nodes: []corev1.Node{
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			expected: expectedStruct{
				nodeHealth: types.NodeHealth{
					Score: 100,
					Statistics: types.NodeStatistics{
						Total:     1,
						Healthy:   1,
						Unhealthy: map[corev1.NodeConditionType]int{},
					},
				},
				err: nil,
			},
			desc: "healthy node",
		},
		{
			nodes: []corev1.Node{
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{},
					},
				},
			},
			expected: expectedStruct{
				nodeHealth: types.NodeHealth{
					Score: 0,
					Statistics: types.NodeStatistics{
						Total:     1,
						Healthy:   0,
						Unhealthy: map[corev1.NodeConditionType]int{"Unknown": 1},
					},
				},
				err: nil,
			},
			desc: "unknown node",
		},
		{
			nodes: []corev1.Node{
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeMemoryPressure,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			expected: expectedStruct{
				nodeHealth: types.NodeHealth{
					Score: 0,
					Statistics: types.NodeStatistics{
						Total:     1,
						Healthy:   0,
						Unhealthy: map[corev1.NodeConditionType]int{corev1.NodeMemoryPressure: 1},
					},
				},
				err: nil,
			},
			desc: "unhealthy node with memory pressure",
		},
	}

	for _, test := range tests {
		nodeHealth, err := evaluateNodeHealth(test.nodes, logger)
		assert.Equal(t, test.expected.nodeHealth, nodeHealth, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
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
