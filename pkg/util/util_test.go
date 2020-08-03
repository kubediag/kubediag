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
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
)

func TestUpdateAbnormalCondition(t *testing.T) {
	abnormalStatus := diagnosisv1.AbnormalStatus{
		Conditions: []diagnosisv1.AbnormalCondition{
			{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync abnormal successfully",
			},
		},
	}

	tests := []struct {
		status    *diagnosisv1.AbnormalStatus
		condition diagnosisv1.AbnormalCondition
		expected  bool
		desc      string
	}{
		{
			status: &abnormalStatus,
			condition: diagnosisv1.AbnormalCondition{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync abnormal successfully",
			},
			expected: false,
			desc:     "all equal, no update",
		},
		{
			status: &abnormalStatus,
			condition: diagnosisv1.AbnormalCondition{
				Type:    diagnosisv1.AbnormalIdentified,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync abnormal successfully",
			},
			expected: true,
			desc:     "not equal Type, should get updated",
		},
		{
			status: &abnormalStatus,
			condition: diagnosisv1.AbnormalCondition{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionFalse,
				Reason:  "successfully",
				Message: "sync abnormal successfully",
			},
			expected: true,
			desc:     "not equal Status, should get updated",
		},
	}

	for _, test := range tests {
		resultStatus := UpdateAbnormalCondition(test.status, &test.condition)
		assert.Equal(t, test.expected, resultStatus, test.desc)
	}
}

func TestGetAbnormalCondition(t *testing.T) {
	type expectedStruct struct {
		index     int
		condition *diagnosisv1.AbnormalCondition
	}

	tests := []struct {
		status   *diagnosisv1.AbnormalStatus
		condType diagnosisv1.AbnormalConditionType
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
			status: &diagnosisv1.AbnormalStatus{
				Conditions: nil,
			},
			condType: diagnosisv1.InformationCollected,
			expected: expectedStruct{-1, nil},
			desc:     "conditions nil, not found",
		},
		{
			status: &diagnosisv1.AbnormalStatus{
				Conditions: []diagnosisv1.AbnormalCondition{
					{
						Type:    diagnosisv1.InformationCollected,
						Status:  corev1.ConditionTrue,
						Reason:  "successfully",
						Message: "sync abnormal successfully",
					},
				},
			},
			condType: diagnosisv1.InformationCollected,
			expected: expectedStruct{0, &diagnosisv1.AbnormalCondition{
				Type:    diagnosisv1.InformationCollected,
				Status:  corev1.ConditionTrue,
				Reason:  "successfully",
				Message: "sync abnormal successfully"},
			},
			desc: "condition found",
		},
	}

	for _, test := range tests {
		resultIndex, resultCond := GetAbnormalCondition(test.status, test.condType)
		assert.Equal(t, test.expected.index, resultIndex, test.desc)
		assert.Equal(t, test.expected.condition, resultCond, test.desc)
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

func TestValidateAbnormalResult(t *testing.T) {
	time := time.Now()
	abnormal := diagnosisv1.Abnormal{
		Spec: diagnosisv1.AbnormalSpec{
			NodeName: "node1",
		},
		Status: diagnosisv1.AbnormalStatus{
			Identifiable: false,
			Recoverable:  false,
			Phase:        diagnosisv1.AbnormalDiagnosing,
			Conditions: []diagnosisv1.AbnormalCondition{
				{
					Type:    diagnosisv1.InformationCollected,
					Status:  corev1.ConditionTrue,
					Reason:  "successfully",
					Message: "sync abnormal successfully",
				},
			},
			StartTime: metav1.NewTime(time),
			Diagnoser: &diagnosisv1.NamespacedName{
				Namespace: "default",
				Name:      "diagnoser1",
			},
		},
	}

	invalidSpec := abnormal
	invalidSpec.Spec.NodeName = "node2"

	invalidIdentifiable := abnormal
	invalidIdentifiable.Status.Identifiable = true

	invalidRecoverable := abnormal
	invalidRecoverable.Status.Recoverable = true

	invalidPhase := abnormal
	invalidPhase.Status.Phase = diagnosisv1.AbnormalFailed

	invalidConditions := abnormal
	invalidConditions.Status.Conditions = []diagnosisv1.AbnormalCondition{}

	invalidMessage := abnormal
	invalidMessage.Status.Message = "message"

	invalidReason := abnormal
	invalidReason.Status.Reason = "reason"

	invalidStartTime := abnormal
	invalidStartTime.Status.StartTime = metav1.NewTime(time.Add(1000))

	invalidDiagnoser := abnormal
	invalidDiagnoser.Status.Diagnoser = &diagnosisv1.NamespacedName{
		Namespace: "default",
		Name:      "diagnoser2",
	}

	invalidRecoverer := abnormal
	invalidRecoverer.Status.Recoverer = &diagnosisv1.NamespacedName{
		Namespace: "default",
		Name:      "recoverer1",
	}

	valid := abnormal
	valid.Status.Context = &runtime.RawExtension{
		Raw: []byte("test"),
	}

	tests := []struct {
		result   diagnosisv1.Abnormal
		current  diagnosisv1.Abnormal
		expected error
		desc     string
	}{
		{
			current:  diagnosisv1.Abnormal{},
			result:   diagnosisv1.Abnormal{},
			expected: nil,
			desc:     "empty abnormal",
		},
		{
			current:  abnormal,
			result:   abnormal,
			expected: nil,
			desc:     "no change",
		},
		{
			current:  abnormal,
			result:   valid,
			expected: nil,
			desc:     "valid abnormal",
		},
		{
			current:  abnormal,
			result:   invalidSpec,
			expected: fmt.Errorf("spec field of Abnormal must not be modified"),
			desc:     "invalid spec field",
		},
		{
			current:  abnormal,
			result:   invalidIdentifiable,
			expected: fmt.Errorf("identifiable filed of Abnormal must not be modified"),
			desc:     "invalid identifiable field",
		},
		{
			current:  abnormal,
			result:   invalidRecoverable,
			expected: fmt.Errorf("recoverable filed of Abnormal must not be modified"),
			desc:     "invalid recoverable field",
		},
		{
			current:  abnormal,
			result:   invalidPhase,
			expected: fmt.Errorf("phase filed of Abnormal must not be modified"),
			desc:     "invalid phase field",
		},
		{
			current:  abnormal,
			result:   invalidConditions,
			expected: fmt.Errorf("conditions filed of Abnormal must not be modified"),
			desc:     "invalid conditions field",
		},
		{
			current:  abnormal,
			result:   invalidMessage,
			expected: fmt.Errorf("message filed of Abnormal must not be modified"),
			desc:     "invalid message field",
		},
		{
			current:  abnormal,
			result:   invalidReason,
			expected: fmt.Errorf("reason filed of Abnormal must not be modified"),
			desc:     "invalid reason field",
		},
		{
			current:  abnormal,
			result:   invalidStartTime,
			expected: fmt.Errorf("startTime filed of Abnormal must not be modified"),
			desc:     "invalid startTime field",
		},
		{
			current:  abnormal,
			result:   invalidDiagnoser,
			expected: fmt.Errorf("diagnoser filed of Abnormal must not be modified"),
			desc:     "invalid diagnoser field",
		},
		{
			current:  abnormal,
			result:   invalidRecoverer,
			expected: fmt.Errorf("recoverer filed of Abnormal must not be modified"),
			desc:     "invalid recoverer field",
		},
	}

	for _, test := range tests {
		err := ValidateAbnormalResult(test.result, test.current)
		assert.Equal(t, test.expected, err, test.desc)
	}
}

func TestIsAbnormalNodeNameMatched(t *testing.T) {
	tests := []struct {
		abnormal diagnosisv1.Abnormal
		node     string
		expected bool
		desc     string
	}{
		{
			abnormal: diagnosisv1.Abnormal{
				Spec: diagnosisv1.AbnormalSpec{
					NodeName: "",
				},
			},
			node:     "node1",
			expected: true,
			desc:     "empty node name",
		},
		{
			abnormal: diagnosisv1.Abnormal{
				Spec: diagnosisv1.AbnormalSpec{
					NodeName: "node1",
				},
			},
			node:     "node1",
			expected: true,
			desc:     "node name matched",
		},
	}

	for _, test := range tests {
		matched := IsAbnormalNodeNameMatched(test.abnormal, test.node)
		assert.Equal(t, test.expected, matched, test.desc)
	}
}
