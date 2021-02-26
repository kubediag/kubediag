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

package util

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/types"
)

func TestUpdateDiagnosisCondition(t *testing.T) {
	diagnosisStatus := diagnosisv1.DiagnosisStatus{
		Conditions: []diagnosisv1.DiagnosisCondition{
			{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync diagnosis successfully",
			},
		},
	}

	tests := []struct {
		status    *diagnosisv1.DiagnosisStatus
		condition diagnosisv1.DiagnosisCondition
		expected  bool
		desc      string
	}{
		{
			status: &diagnosisStatus,
			condition: diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync diagnosis successfully",
			},
			expected: false,
			desc:     "all equal, no update",
		},
		{
			status: &diagnosisStatus,
			condition: diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.DiagnosisIdentified,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync diagnosis successfully",
			},
			expected: true,
			desc:     "not equal Type, should get updated",
		},
		{
			status: &diagnosisStatus,
			condition: diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionFalse,
				Reason:  "successfully",
				Message: "sync diagnosis successfully",
			},
			expected: true,
			desc:     "not equal Status, should get updated",
		},
	}

	for _, test := range tests {
		resultStatus := UpdateDiagnosisCondition(test.status, &test.condition)
		assert.Equal(t, test.expected, resultStatus, test.desc)
	}
}

func TestGetDiagnosisCondition(t *testing.T) {
	type expectedStruct struct {
		index     int
		condition *diagnosisv1.DiagnosisCondition
	}

	tests := []struct {
		status   *diagnosisv1.DiagnosisStatus
		condType diagnosisv1.DiagnosisConditionType
		expected expectedStruct
		desc     string
	}{
		{
			status:   nil,
			condType: diagnosisv1.InformationCollected,
			expected: expectedStruct{-1, nil},
			desc:     "status nil, not found",
		},
		{
			status: &diagnosisv1.DiagnosisStatus{
				Conditions: nil,
			},
			condType: diagnosisv1.InformationCollected,
			expected: expectedStruct{-1, nil},
			desc:     "conditions nil, not found",
		},
		{
			status: &diagnosisv1.DiagnosisStatus{
				Conditions: []diagnosisv1.DiagnosisCondition{
					{
						Type:    diagnosisv1.InformationCollected,
						Status:  corev1.ConditionTrue,
						Reason:  "successfully",
						Message: "sync diagnosis successfully",
					},
				},
			},
			condType: diagnosisv1.InformationCollected,
			expected: expectedStruct{0, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync diagnosis successfully"},
			},
			desc: "condition found",
		},
	}

	for _, test := range tests {
		resultIndex, resultCond := GetDiagnosisCondition(test.status, test.condType)
		assert.Equal(t, test.expected.index, resultIndex, test.desc)
		assert.Equal(t, test.expected.condition, resultCond, test.desc)
	}
}

func TestGetPodUnhealthyReason(t *testing.T) {
	tests := []struct {
		pod      corev1.Pod
		expected string
		desc     string
	}{
		{
			pod:      corev1.Pod{},
			expected: "Unknown",
			desc:     "empty pod",
		},
		{
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Ready: true,
						},
					},
				},
			},
			expected: "Unknown",
			desc:     "ready pod",
		},
		{
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Ready: false,
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "reason1",
								},
							},
						},
					},
				},
			},
			expected: "reason1",
			desc:     "terminated pod",
		},
		{
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Ready: false,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "reason2",
								},
							},
						},
					},
				},
			},
			expected: "reason2",
			desc:     "waiting pod",
		},
		{
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Ready: false,
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "reason3",
								},
							},
						},
					},
				},
			},
			expected: "reason3",
			desc:     "pod with last termination",
		},
	}

	for _, test := range tests {
		reason := GetPodUnhealthyReason(test.pod)
		assert.Equal(t, test.expected, reason, test.desc)
	}
}

func TestUpdatePodUnhealthyReasonStatistics(t *testing.T) {
	type expectedStruct struct {
		updated               bool
		containerStateReasons map[string]int
	}

	tests := []struct {
		containerStateReasons map[string]int
		reason                string
		expected              expectedStruct
		desc                  string
	}{
		{
			containerStateReasons: map[string]int{},
			reason:                "",
			expected: expectedStruct{
				updated:               false,
				containerStateReasons: map[string]int{},
			},
			desc: "empty reason",
		},
		{
			containerStateReasons: map[string]int{},
			reason:                "reason1",
			expected: expectedStruct{
				updated:               true,
				containerStateReasons: map[string]int{"reason1": 1},
			},
			desc: "new reason added",
		},
		{
			containerStateReasons: map[string]int{"reason1": 1, "reason2": 1},
			reason:                "reason1",
			expected: expectedStruct{
				updated:               true,
				containerStateReasons: map[string]int{"reason1": 2, "reason2": 1},
			},
			desc: "reason updated",
		},
	}

	for _, test := range tests {
		updated := UpdatePodUnhealthyReasonStatistics(test.containerStateReasons, test.reason)
		assert.Equal(t, test.expected.updated, updated, test.desc)
		assert.Equal(t, test.expected.containerStateReasons, test.containerStateReasons, test.desc)
	}
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		node     corev1.Node
		expected bool
		desc     string
	}{
		{
			node:     corev1.Node{},
			expected: false,
			desc:     "node status is empty",
		},
		{
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
			desc:     "node is ready",
		},
		{
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
			desc:     "node is not ready",
		},
		{
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeNetworkUnavailable,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
			desc:     "node is network unavailable",
		},
	}

	for _, test := range tests {
		ready := IsNodeReady(test.node)
		assert.Equal(t, test.expected, ready, test.desc)
	}
}

func TestGetNodeUnhealthyConditionType(t *testing.T) {
	tests := []struct {
		node     corev1.Node
		expected corev1.NodeConditionType
		desc     string
	}{
		{
			node:     corev1.Node{},
			expected: "Unknown",
			desc:     "node status is empty",
		},
		{
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   "type1",
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: "type1",
			desc:     "unhealthy node",
		},
		{
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: "Unknown",
			desc:     "healthy node",
		},
	}

	for _, test := range tests {
		conditionType := GetNodeUnhealthyConditionType(test.node)
		assert.Equal(t, test.expected, conditionType, test.desc)
	}
}

func TestFormatURL(t *testing.T) {
	tests := []struct {
		scheme   string
		host     string
		port     string
		path     string
		expected *url.URL
		desc     string
	}{
		{
			scheme: "http",
			host:   "127.0.0.1",
			port:   "8080",
			path:   "/test",
			expected: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:8080",
				Path:   "/test",
			},
			desc: "regular url",
		},
	}

	for _, test := range tests {
		resultURL := FormatURL(test.scheme, test.host, test.port, test.path)
		assert.Equal(t, test.expected, resultURL, test.desc)
	}
}

func TestListPodsFromPodInformationContext(t *testing.T) {
	type expectedStruct struct {
		pods []corev1.Pod
		err  error
	}

	logger := log.NullLogger{}
	specRaw, err := json.Marshal(map[string]interface{}{
		"podInformation": []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod2",
				},
			},
		},
	})
	if err != nil {
		t.Errorf("unable to marshal pods: %v", err)
	}
	statusRaw, err := json.Marshal(map[string]interface{}{
		"podInformation": []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod3",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod4",
				},
			},
		},
	})
	if err != nil {
		t.Errorf("unable to marshal pods: %v", err)
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			expected: expectedStruct{
				pods: nil,
				err:  fmt.Errorf("diagnosis status context nil"),
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			expected: expectedStruct{
				pods: nil,
				err:  fmt.Errorf("diagnosis status context empty"),
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: specRaw,
					},
				},
			},
			expected: expectedStruct{
				pods: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod2",
						},
					},
				},
				err: nil,
			},
			desc: "pods found in spec context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: statusRaw,
					},
				},
			},
			expected: expectedStruct{
				pods: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod3",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod4",
						},
					},
				},
				err: nil,
			},
			desc: "pods found in status context",
		},
	}

	for _, test := range tests {
		pods, err := ListPodsFromPodInformationContext(test.diagnosis, logger)
		assert.Equal(t, test.expected.pods, pods, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestListFilePathsFromFilePathInformationContext(t *testing.T) {
	type expectedStruct struct {
		paths []string
		err   error
	}

	logger := log.NullLogger{}
	specRaw, err := json.Marshal(map[string]interface{}{
		"filePathInformation": []string{"/bin/", "/etc/"},
	})
	if err != nil {
		t.Errorf("unable to marshal file paths: %v", err)
	}
	statusRaw, err := json.Marshal(map[string]interface{}{
		"filePathInformation": []string{"/sys/", "/var/"},
	})
	if err != nil {
		t.Errorf("unable to marshal file paths: %v", err)
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			expected: expectedStruct{
				paths: nil,
				err:   fmt.Errorf("diagnosis status context nil"),
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			expected: expectedStruct{
				paths: nil,
				err:   fmt.Errorf("diagnosis status context empty"),
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: specRaw,
					},
				},
			},
			expected: expectedStruct{
				paths: []string{"/bin/", "/etc/"},
				err:   nil,
			},
			desc: "file paths found in spec context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: statusRaw,
					},
				},
			},
			expected: expectedStruct{
				paths: []string{"/sys/", "/var/"},
				err:   nil,
			},
			desc: "file paths found in status context",
		},
	}

	for _, test := range tests {
		paths, err := ListFilePathsFromFilePathInformationContext(test.diagnosis, logger)
		assert.Equal(t, test.expected.paths, paths, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestListSignalsFromSignalRecoveryContext(t *testing.T) {
	type expectedStruct struct {
		signals types.SignalList
		err     error
	}

	logger := log.NullLogger{}
	specRaw, err := json.Marshal(map[string]interface{}{
		"signalRecovery": types.SignalList{
			{
				PID:    1,
				Signal: 1,
			},
			{
				PID:    2,
				Signal: 2,
			},
		},
	})
	if err != nil {
		t.Errorf("unable to marshal signals: %v", err)
	}
	statusRaw, err := json.Marshal(map[string]interface{}{
		"signalRecovery": types.SignalList{
			{
				PID:    3,
				Signal: 3,
			},
			{
				PID:    4,
				Signal: 4,
			},
		},
	})
	if err != nil {
		t.Errorf("unable to marshal signals: %v", err)
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			expected: expectedStruct{
				signals: nil,
				err:     fmt.Errorf("diagnosis status context nil"),
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			expected: expectedStruct{
				signals: nil,
				err:     fmt.Errorf("diagnosis status context empty"),
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: specRaw,
					},
				},
			},
			expected: expectedStruct{
				signals: types.SignalList{
					{
						PID:    1,
						Signal: 1,
					},
					{
						PID:    2,
						Signal: 2,
					},
				},
				err: nil,
			},
			desc: "signals found in spec context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: statusRaw,
					},
				},
			},
			expected: expectedStruct{
				signals: types.SignalList{
					{
						PID:    3,
						Signal: 3,
					},
					{
						PID:    4,
						Signal: 4,
					},
				},
				err: nil,
			},
			desc: "signals found in status context",
		},
	}

	for _, test := range tests {
		signals, err := ListSignalsFromSignalRecoveryContext(test.diagnosis, logger)
		assert.Equal(t, test.expected.signals, signals, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestListProcessesFromProcessInformationContext(t *testing.T) {
	type expectedStruct struct {
		processes []types.Process
		err       error
	}

	logger := log.NullLogger{}
	specRaw, err := json.Marshal(map[string]interface{}{
		"processInformation": []types.Process{
			{
				PID: 1,
			},
			{
				PID: 2,
			},
		},
	})
	if err != nil {
		t.Errorf("unable to marshal signals: %v", err)
	}
	statusRaw, err := json.Marshal(map[string]interface{}{
		"processInformation": []types.Process{
			{
				PID: 3,
			},
			{
				PID: 4,
			},
		},
	})
	if err != nil {
		t.Errorf("unable to marshal signals: %v", err)
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			expected: expectedStruct{
				processes: nil,
				err:       fmt.Errorf("diagnosis status context nil"),
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			expected: expectedStruct{
				processes: nil,
				err:       fmt.Errorf("diagnosis status context empty"),
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: specRaw,
					},
				},
			},
			expected: expectedStruct{
				processes: []types.Process{
					{
						PID: 1,
					},
					{
						PID: 2,
					},
				},
				err: nil,
			},
			desc: "processes found in spec context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: statusRaw,
					},
				},
			},
			expected: expectedStruct{
				processes: []types.Process{
					{
						PID: 3,
					},
					{
						PID: 4,
					},
				},
				err: nil,
			},
			desc: "processes found in status context",
		},
	}

	for _, test := range tests {
		processes, err := ListProcessesFromProcessInformationContext(test.diagnosis, logger)
		assert.Equal(t, test.expected.processes, processes, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestValidateDiagnosisResult(t *testing.T) {
	time := time.Now()
	diagnosis := diagnosisv1.Diagnosis{
		Spec: diagnosisv1.DiagnosisSpec{
			Source: "Custom",
			KubernetesEvent: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name: "event1",
				},
			},
			NodeName: "node1",
			AssignedInformationCollectors: []diagnosisv1.NamespacedName{
				{
					Name: "collector1",
				},
			},
			AssignedDiagnosers: []diagnosisv1.NamespacedName{
				{
					Name: "diagnoser1",
				},
			},
			AssignedRecoverers: []diagnosisv1.NamespacedName{
				{
					Name: "recoverer1",
				},
			},
		},
		Status: diagnosisv1.DiagnosisStatus{
			Phase: diagnosisv1.DiagnosisDiagnosing,
			Conditions: []diagnosisv1.DiagnosisCondition{
				{
					Type:    diagnosisv1.InformationCollected,
					Status:  corev1.ConditionTrue,
					Reason:  "successfully",
					Message: "sync diagnosis successfully",
				},
			},
			StartTime: metav1.NewTime(time),
		},
	}

	invalidSpec := diagnosis
	invalidSpec.Spec.Source = "KubernetesEvent"

	invalidPhase := diagnosis
	invalidPhase.Status.Phase = diagnosisv1.DiagnosisFailed

	invalidConditions := diagnosis
	invalidConditions.Status.Conditions = []diagnosisv1.DiagnosisCondition{}

	invalidStartTime := diagnosis
	invalidStartTime.Status.StartTime = metav1.NewTime(time.Add(1000))

	valid := diagnosis
	valid.Status.Context = &runtime.RawExtension{
		Raw: []byte("test"),
	}

	tests := []struct {
		result   diagnosisv1.Diagnosis
		current  diagnosisv1.Diagnosis
		expected error
		desc     string
	}{
		{
			current:  diagnosisv1.Diagnosis{},
			result:   diagnosisv1.Diagnosis{},
			expected: nil,
			desc:     "empty diagnosis",
		},
		{
			current:  diagnosis,
			result:   diagnosis,
			expected: nil,
			desc:     "no change",
		},
		{
			current:  diagnosis,
			result:   valid,
			expected: nil,
			desc:     "valid diagnosis",
		},
		{
			current:  diagnosis,
			result:   invalidSpec,
			expected: fmt.Errorf("spec field of Diagnosis must not be modified"),
			desc:     "invalid spec field",
		},
		{
			current:  diagnosis,
			result:   invalidPhase,
			expected: fmt.Errorf("phase field of Diagnosis must not be modified"),
			desc:     "invalid phase field",
		},
		{
			current:  diagnosis,
			result:   invalidConditions,
			expected: fmt.Errorf("conditions field of Diagnosis must not be modified"),
			desc:     "invalid conditions field",
		},
		{
			current:  diagnosis,
			result:   invalidStartTime,
			expected: fmt.Errorf("startTime field of Diagnosis must not be modified"),
			desc:     "invalid startTime field",
		},
	}

	for _, test := range tests {
		err := ValidateDiagnosisResult(test.result, test.current)
		if test.expected == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.Error(), test.desc)
		}
	}
}

func TestIsDiagnosisNodeNameMatched(t *testing.T) {
	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		node      string
		expected  bool
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					NodeName: "",
				},
			},
			node:     "node1",
			expected: true,
			desc:     "empty node name",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					NodeName: "node1",
				},
			},
			node:     "node1",
			expected: true,
			desc:     "node name matched",
		},
	}

	for _, test := range tests {
		matched := IsDiagnosisNodeNameMatched(test.diagnosis, test.node)
		assert.Equal(t, test.expected, matched, test.desc)
	}
}

func TestSetDiagnosisSpecContext(t *testing.T) {
	type expectedStruct struct {
		diagnosis diagnosisv1.Diagnosis
		err       error
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		key       string
		value     interface{}
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: nil,
				},
			},
			key:   "key1",
			value: "value1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1"),
						},
					},
				},
				err: nil,
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{},
				},
			},
			key:   "key1",
			value: "value1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1"),
						},
					},
				},
				err: nil,
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key:   "key3",
			value: "value3",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1", "key2", "value2", "key3", "value3"),
						},
					},
				},
				err: nil,
			},
			desc: "context updated",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key:   "key2",
			value: "value3",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1", "key2", "value3"),
						},
					},
				},
				err: nil,
			},
			desc: "context overwritten",
		},
	}

	for _, test := range tests {
		diagnosis, err := SetDiagnosisSpecContext(test.diagnosis, test.key, test.value)
		assert.Equal(t, test.expected.diagnosis, diagnosis, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestSetDiagnosisStatusContext(t *testing.T) {
	type expectedStruct struct {
		diagnosis diagnosisv1.Diagnosis
		err       error
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		key       string
		value     interface{}
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			key:   "key1",
			value: "value1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1"),
						},
					},
				},
				err: nil,
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			key:   "key1",
			value: "value1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1"),
						},
					},
				},
				err: nil,
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key:   "key3",
			value: "value3",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1", "key2", "value2", "key3", "value3"),
						},
					},
				},
				err: nil,
			},
			desc: "context updated",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key:   "key2",
			value: "value3",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1", "key2", "value3"),
						},
					},
				},
				err: nil,
			},
			desc: "context overwritten",
		},
	}

	for _, test := range tests {
		diagnosis, err := SetDiagnosisStatusContext(test.diagnosis, test.key, test.value)
		assert.Equal(t, test.expected.diagnosis, diagnosis, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestGetDiagnosisSpecContext(t *testing.T) {
	type expectedStruct struct {
		value []byte
		err   error
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		key       string
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: nil,
				},
			},
			key: "key1",
			expected: expectedStruct{
				value: nil,
				err:   fmt.Errorf("diagnosis spec context nil"),
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{},
				},
			},
			key: "key1",
			expected: expectedStruct{
				value: nil,
				err:   fmt.Errorf("diagnosis spec context empty"),
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key: "key2",
			expected: expectedStruct{
				value: func(str string) []byte {
					result, err := json.Marshal(str)
					if err != nil {
						t.Errorf("%v", err)
					}
					return result
				}("value2"),
				err: nil,
			},
			desc: "context found",
		},
	}

	for _, test := range tests {
		diagnosis, err := GetDiagnosisSpecContext(test.diagnosis, test.key)
		assert.Equal(t, test.expected.value, diagnosis, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestGetDiagnosisStatusContext(t *testing.T) {
	type expectedStruct struct {
		value []byte
		err   error
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		key       string
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			key: "key1",
			expected: expectedStruct{
				value: nil,
				err:   fmt.Errorf("diagnosis status context nil"),
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			key: "key1",
			expected: expectedStruct{
				value: nil,
				err:   fmt.Errorf("diagnosis status context empty"),
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key: "key2",
			expected: expectedStruct{
				value: func(str string) []byte {
					result, err := json.Marshal(str)
					if err != nil {
						t.Errorf("%v", err)
					}
					return result
				}("value2"),
				err: nil,
			},
			desc: "context found",
		},
	}

	for _, test := range tests {
		diagnosis, err := GetDiagnosisStatusContext(test.diagnosis, test.key)
		assert.Equal(t, test.expected.value, diagnosis, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestRemoveDiagnosisSpecContext(t *testing.T) {
	type expectedStruct struct {
		diagnosis diagnosisv1.Diagnosis
		removed   bool
		err       error
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		key       string
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: nil,
				},
			},
			key: "key1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: nil,
					},
				},
				removed: true,
				err:     nil,
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{},
				},
			},
			key: "key1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{},
					},
				},
				removed: true,
				err:     nil,
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key: "key2",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1"),
						},
					},
				},
				removed: true,
				err:     nil,
			},
			desc: "context removed",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					Context: &runtime.RawExtension{
						Raw: []byte{0, 1, 2},
					},
				},
			},
			key: "key1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Spec: diagnosisv1.DiagnosisSpec{
						Context: &runtime.RawExtension{
							Raw: []byte{0, 1, 2},
						},
					},
				},
				removed: false,
				err:     fmt.Errorf("invalid character '\\x00' looking for beginning of value"),
			},
			desc: "invalid context",
		},
	}

	for _, test := range tests {
		diagnosis, removed, err := RemoveDiagnosisSpecContext(test.diagnosis, test.key)
		assert.Equal(t, test.expected.diagnosis, diagnosis, test.desc)
		assert.Equal(t, test.expected.removed, removed, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestRemoveDiagnosisStatusContext(t *testing.T) {
	type expectedStruct struct {
		diagnosis diagnosisv1.Diagnosis
		removed   bool
		err       error
	}

	tests := []struct {
		diagnosis diagnosisv1.Diagnosis
		key       string
		expected  expectedStruct
		desc      string
	}{
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: nil,
				},
			},
			key: "key1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: nil,
					},
				},
				removed: true,
				err:     nil,
			},
			desc: "nil context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{},
				},
			},
			key: "key1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{},
					},
				},
				removed: true,
				err:     nil,
			},
			desc: "empty context",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: func(keysAndValues ...string) []byte {
							testingMap, err := newTestingMap(keysAndValues...)
							if err != nil {
								t.Errorf("%v", err)
							}
							return testingMap
						}("key1", "value1", "key2", "value2"),
					},
				},
			},
			key: "key2",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{
							Raw: func(keysAndValues ...string) []byte {
								testingMap, err := newTestingMap(keysAndValues...)
								if err != nil {
									t.Errorf("%v", err)
								}
								return testingMap
							}("key1", "value1"),
						},
					},
				},
				removed: true,
				err:     nil,
			},
			desc: "context removed",
		},
		{
			diagnosis: diagnosisv1.Diagnosis{
				Status: diagnosisv1.DiagnosisStatus{
					Context: &runtime.RawExtension{
						Raw: []byte{0, 1, 2},
					},
				},
			},
			key: "key1",
			expected: expectedStruct{
				diagnosis: diagnosisv1.Diagnosis{
					Status: diagnosisv1.DiagnosisStatus{
						Context: &runtime.RawExtension{
							Raw: []byte{0, 1, 2},
						},
					},
				},
				removed: false,
				err:     fmt.Errorf("invalid character '\\x00' looking for beginning of value"),
			},
			desc: "invalid context",
		},
	}

	for _, test := range tests {
		diagnosis, removed, err := RemoveDiagnosisStatusContext(test.diagnosis, test.key)
		assert.Equal(t, test.expected.diagnosis, diagnosis, test.desc)
		assert.Equal(t, test.expected.removed, removed, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestRetrievePodsOnNode(t *testing.T) {
	tests := []struct {
		pods     []corev1.Pod
		nodeName string
		expected []corev1.Pod
		desc     string
	}{
		{
			pods:     []corev1.Pod{},
			nodeName: "node1",
			expected: []corev1.Pod{},
			desc:     "empty slice",
		},
		{
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod2",
					},
					Spec: corev1.PodSpec{
						NodeName: "node2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod3",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				},
			},
			nodeName: "node1",
			expected: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod3",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				},
			},
			desc: "pods not on provided node removed",
		},
	}

	for _, test := range tests {
		resultPods := RetrievePodsOnNode(test.pods, test.nodeName)
		assert.Equal(t, test.expected, resultPods, test.desc)
	}
}

func TestRetrieveDiagnosesOnNode(t *testing.T) {
	tests := []struct {
		diagnoses []diagnosisv1.Diagnosis
		nodeName  string
		expected  []diagnosisv1.Diagnosis
		desc      string
	}{
		{
			diagnoses: []diagnosisv1.Diagnosis{},
			nodeName:  "node1",
			expected:  []diagnosisv1.Diagnosis{},
			desc:      "empty slice",
		},
		{
			diagnoses: []diagnosisv1.Diagnosis{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "diagnosis1",
					},
					Spec: diagnosisv1.DiagnosisSpec{
						NodeName: "node1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "diagnosis2",
					},
					Spec: diagnosisv1.DiagnosisSpec{
						NodeName: "node2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "diagnosis3",
					},
					Spec: diagnosisv1.DiagnosisSpec{
						NodeName: "node1",
					},
				},
			},
			nodeName: "node1",
			expected: []diagnosisv1.Diagnosis{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "diagnosis1",
					},
					Spec: diagnosisv1.DiagnosisSpec{
						NodeName: "node1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "diagnosis3",
					},
					Spec: diagnosisv1.DiagnosisSpec{
						NodeName: "node1",
					},
				},
			},
			desc: "diagnoses not on provided node removed",
		},
	}

	for _, test := range tests {
		resultDiagnoses := RetrieveDiagnosesOnNode(test.diagnoses, test.nodeName)
		assert.Equal(t, test.expected, resultDiagnoses, test.desc)
	}
}

func TestMatchPrometheusAlert(t *testing.T) {
	type expectedStruct struct {
		matched bool
		err     error
	}

	time := time.Now()
	tests := []struct {
		prometheusAlertTemplate diagnosisv1.PrometheusAlertTemplate
		diagnosis               diagnosisv1.Diagnosis
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						Labels: model.LabelSet{
							"alertname": "alert1",
							"node":      "node1",
						},
						Annotations: model.LabelSet{
							"message":   "message1",
							"namespace": "namespace1",
						},
						StartsAt:     metav1.NewTime(time),
						EndsAt:       metav1.NewTime(time),
						GeneratorURL: "url1",
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						Labels: model.LabelSet{
							"alertname": "alert1",
							"node":      "node1",
						},
						Annotations: model.LabelSet{
							"message":   "message1",
							"namespace": "namespace1",
						},
						StartsAt:     metav1.NewTime(time),
						EndsAt:       metav1.NewTime(time),
						GeneratorURL: "url1",
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{},
				},
			},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						Labels: model.LabelSet{
							"alertname": "alert2",
						},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						Labels: model.LabelSet{
							"alertname": "alert1",
							"node":      "node2",
						},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						Annotations: model.LabelSet{
							"message":   "message1",
							"namespace": "namespace2",
						},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						StartsAt: metav1.NewTime(time),
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						EndsAt: metav1.NewTime(time),
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						GeneratorURL: "url2",
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					PrometheusAlert: &diagnosisv1.PrometheusAlert{
						Labels: model.LabelSet{
							"alertname": "alert1",
						},
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
		matched, err := MatchPrometheusAlert(test.prometheusAlertTemplate, test.diagnosis)
		assert.Equal(t, test.expected.matched, matched, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func TestMatchKubernetesEvent(t *testing.T) {
	type expectedStruct struct {
		matched bool
		err     error
	}

	time := time.Now()
	tests := []struct {
		kubernetesEventTemplate diagnosisv1.KubernetesEventTemplate
		diagnosis               diagnosisv1.Diagnosis
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event1",
							Namespace: "namespace1",
						},
						InvolvedObject: corev1.ObjectReference{
							Kind:            "kind1",
							Namespace:       "namespace1",
							Name:            "object1",
							UID:             "uid1",
							APIVersion:      "v1",
							ResourceVersion: "1",
							FieldPath:       "path1",
						},
						Reason:  "reason1",
						Message: "message1",
						Source: corev1.EventSource{
							Component: "component1",
							Host:      "host1",
						},
						FirstTimestamp:      metav1.NewTime(time),
						LastTimestamp:       metav1.NewTime(time),
						Count:               1,
						Type:                "type1",
						Action:              "action1",
						ReportingController: "controller1",
						ReportingInstance:   "instance1",
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "event1",
							Namespace: "namespace1",
						},
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{},
				},
			},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name: "event2",
						},
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "namespace2",
						},
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						Reason: "reason2",
					},
				},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						Message: "message2",
					},
				},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						Source: corev1.EventSource{
							Component: "component2",
						},
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						Source: corev1.EventSource{
							Host: "host2",
						},
					},
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
			diagnosis: diagnosisv1.Diagnosis{
				Spec: diagnosisv1.DiagnosisSpec{
					KubernetesEvent: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name: "event1",
						},
					},
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
		matched, err := MatchKubernetesEvent(test.kubernetesEventTemplate, test.diagnosis)
		assert.Equal(t, test.expected.matched, matched, test.desc)
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}

func newTestingMap(keysAndValues ...string) ([]byte, error) {
	if len(keysAndValues) < 2 || len(keysAndValues)%2 == 1 {
		return nil, fmt.Errorf("invalid input for keys and values: %v", keysAndValues)
	}

	testingMap := make(map[string]interface{})
	for i := 0; i < len(keysAndValues)-1; i = i + 2 {
		testingMap[keysAndValues[i]] = keysAndValues[i+1]
	}

	raw, err := json.Marshal(testingMap)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal testing raw data: %v", err)
	}

	return raw, nil
}
