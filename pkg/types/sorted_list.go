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

package types

import (
	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

// SortedDiagnosisListByStartTime contains sorted diagnoses by StartTime in ascending order.
// It satisfies sort.Interface by implemeting the following methods:
//
// Len() int
// Less(i, j int) bool
// Swap(i, j int)
type SortedDiagnosisListByStartTime []diagnosisv1.Diagnosis

// Len is the number of elements in SortedDiagnosisListByStartTime.
func (l SortedDiagnosisListByStartTime) Len() int {
	return len(l)
}

// Less reports whether the element with index i should sort before the element with index j.
func (l SortedDiagnosisListByStartTime) Less(i, j int) bool {
	if i > len(l) || j > len(l) {
		return false
	}

	return l[i].Status.StartTime.Before(&l[j].Status.StartTime)
}

// Swap swaps the elements with indexes i and j.
func (l SortedDiagnosisListByStartTime) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// SortedTaskListByStartTime contains sorted tasks by StartTime in ascending order.
// It satisfies sort.Interface by implemeting the following methods:
//
// Len() int
// Less(i, j int) bool
// Swap(i, j int)
type SortedTaskListByStartTime []diagnosisv1.Task

// Len is the number of elements in SortedDiagnosisListByStartTime.
func (l SortedTaskListByStartTime) Len() int {
	return len(l)
}

// Less reports whether the element with index i should sort before the element with index j.
func (l SortedTaskListByStartTime) Less(i, j int) bool {
	if i > len(l) || j > len(l) {
		return false
	}

	return l[i].Status.StartTime.Before(&l[j].Status.StartTime)
}

// Swap swaps the elements with indexes i and j.
func (l SortedTaskListByStartTime) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
