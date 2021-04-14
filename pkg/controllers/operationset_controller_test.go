package controllers

import (
	"fmt"
	"reflect"
	"testing"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testCase struct {
	desc         string
	operationSet diagnosisv1.OperationSet
	operations   diagnosisv1.OperationList
	expectAppend []diagnosisv1.Operation
	expectRemove []diagnosisv1.Operation
	expectHealth error
}

var deletionTime = metav1.Now()

var allCases = []testCase{
	{
		desc:         "case-1",
		operationSet: buildSimpleOS("test-os-healthy", "o1", "o2"),
		operations: diagnosisv1.OperationList{
			Items: []diagnosisv1.Operation{
				buildO("o1", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o4", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
				buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o6", nil, "test-os-healthy3", "test-os-healthy2"),
			},
		},
		expectAppend: []diagnosisv1.Operation{
			buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
		},
		expectRemove: []diagnosisv1.Operation{
			buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
			buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
		},
		expectHealth: nil,
	},
	{
		desc:         "case-2",
		operationSet: buildSimpleOS("test-os-healthy", "o1", "o2", "o3"),
		operations: diagnosisv1.OperationList{
			Items: []diagnosisv1.Operation{
				buildO("o1", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o4", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
				buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o6", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o7", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o8", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
			},
		},
		expectAppend: []diagnosisv1.Operation{
			buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
		},
		expectRemove: []diagnosisv1.Operation{
			buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
			buildO("o7", &deletionTime, "test-os-healthy", "test-os-healthy2"),
		},
		expectHealth: fmt.Errorf("operation: %s is inactive or dispeared in cluster", "o3"),
	},
	{
		desc:         "case-3",
		operationSet: buildSimpleOS("test-os-healthy", "o1", "o2", "o4"),
		operations: diagnosisv1.OperationList{
			Items: []diagnosisv1.Operation{
				buildO("o1", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o4", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
				buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o6", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o7", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o8", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
			},
		},
		expectAppend: []diagnosisv1.Operation{
			buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
		},
		expectRemove: []diagnosisv1.Operation{
			buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
			buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
			buildO("o7", &deletionTime, "test-os-healthy", "test-os-healthy2"),
		},
		expectHealth: fmt.Errorf("operation: %s is inactive or dispeared in cluster", "o4"),
	},
	{
		desc:         "case-4",
		operationSet: buildSimpleOS("test-os-healthy", "o1", "o2", "o5", "o9"),
		operations: diagnosisv1.OperationList{
			Items: []diagnosisv1.Operation{
				buildO("o1", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o4", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
				buildO("o5", nil, "test-os-healthy", "test-os-healthy2"),
				buildO("o6", nil, "test-os-healthy3", "test-os-healthy2"),
				buildO("o7", &deletionTime, "test-os-healthy", "test-os-healthy2"),
				buildO("o8", &deletionTime, "test-os-healthy3", "test-os-healthy2"),
			},
		},
		expectAppend: []diagnosisv1.Operation{
			buildO("o2", nil, "test-os-healthy3", "test-os-healthy2"),
		},
		expectRemove: []diagnosisv1.Operation{
			buildO("o3", &deletionTime, "test-os-healthy", "test-os-healthy2"),
			buildO("o7", &deletionTime, "test-os-healthy", "test-os-healthy2"),
		},
		expectHealth: fmt.Errorf("operation: %s is inactive or dispeared in cluster", "o9"),
	},
}

func buildO(name string, deletion *metav1.Time, finalizers ...string) diagnosisv1.Operation {
	result := diagnosisv1.Operation{}
	result.Name = name
	result.Finalizers = finalizers
	result.SetDeletionTimestamp(deletion)
	return result
}

func buildSimpleOS(name string, op ...string) diagnosisv1.OperationSet {
	operationSet := diagnosisv1.OperationSet{}
	operationSet.Name = name
	operationSet.Spec.AdjacencyList = make([]diagnosisv1.Node, len(op)+1)
	operationSet.Spec.AdjacencyList[0] = diagnosisv1.Node{
		ID: 0,
		To: diagnosisv1.NodeSet{
			1, 2, 3, 4,
		},
	}
	for i, o := range op {
		operationSet.Spec.AdjacencyList[i+1] = diagnosisv1.Node{
			ID:        i + 1,
			Operation: o,
		}
	}
	return operationSet
}

func TestCheckOperationAndMarkFinalizer(t *testing.T) {
	for _, c := range allCases {
		a, r, h := checkOperationAndMarkFinalizer(c.operationSet, c.operations)
		if !reflect.DeepEqual(a, c.expectAppend) {
			t.Errorf("%s : expect append result: %+v, actual append result: %+v", c.desc, c.expectAppend, a)
		}
		if !reflect.DeepEqual(r, c.expectRemove) {
			t.Errorf("%s : expect remove result: %+v, actual remove result: %+v", c.desc, c.expectRemove, r)
		}
		if !reflect.DeepEqual(h, c.expectHealth) {
			t.Errorf("%s : expect health result: %+v, actual health result: %+v", c.desc, c.expectHealth, h)
		}
	}
}
