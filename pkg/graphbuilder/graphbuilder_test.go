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

package graphbuilder

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

func TestSearchDiagnosisPaths(t *testing.T) {
	type expectedStruct struct {
		paths [][]int64
		err   error
	}

	tests := []struct {
		adjacencyList []diagnosisv1.Node
		expected      expectedStruct
		desc          string
	}{
		{
			adjacencyList: []diagnosisv1.Node{},
			expected: expectedStruct{
				paths: nil,
				err:   fmt.Errorf("start node not found in graph"),
			},
			desc: "start node not found",
		},
		{
			adjacencyList: []diagnosisv1.Node{
				{
					ID: 0,
				},
			},
			expected: expectedStruct{
				paths: [][]int64{},
				err:   nil,
			},
			desc: "one node graph",
		},
		{
			adjacencyList: []diagnosisv1.Node{
				{
					ID: 0,
					To: []int{1, 2, 3},
				},
				{
					ID: 1,
					To: []int{4, 5},
				},
				{
					ID: 2,
					To: []int{5},
				},
				{
					ID: 3,
					To: []int{6},
				},
				{
					ID: 4,
					To: []int{7, 8},
				},
				{
					ID: 5,
					To: []int{3, 6, 7},
				},
				{
					ID: 6,
					To: []int{8},
				},
				{
					ID: 7,
				},
				{
					ID: 8,
				},
			},
			expected: expectedStruct{
				paths: [][]int64{
					[]int64{0, 1, 4, 7},
					[]int64{0, 1, 5, 7},
					[]int64{0, 2, 5, 7},
					[]int64{0, 1, 4, 8},
					[]int64{0, 3, 6, 8},
					[]int64{0, 1, 5, 3, 6, 8},
					[]int64{0, 2, 5, 3, 6, 8},
					[]int64{0, 1, 5, 6, 8},
					[]int64{0, 2, 5, 6, 8},
				},
				err: nil,
			},
			desc: "paths found",
		},
	}

	for _, test := range tests {
		graph, err := newGraphFromAdjacencyList(test.adjacencyList)
		if err != nil {
			t.Errorf("invalid adjacency list: %s", err)
		}

		paths, err := searchDiagnosisPaths(graph, len(test.adjacencyList))
		assert.Equal(t, len(test.expected.paths), len(paths), test.desc)
		for i := 0; i < len(test.expected.paths); i++ {
			found := false
			for j := 0; j < len(paths); j++ {
				if assert.ObjectsAreEqual(test.expected.paths[i], paths[j]) {
					found = true
				}
			}
			if !found {
				assert.Fail(t, "expected %s not found in path", test.desc)
			}
		}
		if test.expected.err == nil {
			assert.NoError(t, err, test.desc)
		} else {
			assert.EqualError(t, err, test.expected.err.Error(), test.desc)
		}
	}
}
