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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodDiskUsageList contains disk usage information of pods.
// It satisfies sort.Interface by implemeting the following methods:
//
// Len() int
// Less(i, j int) bool
// Swap(i, j int)
type PodDiskUsageList []PodDiskUsage

// PodDiskUsage contains disk usage information of a pod.
type PodDiskUsage struct {
	// ObjectMeta is metadata of the pod.
	metav1.ObjectMeta `json:"metadata"`
	// DiskUsage is the disk usage of the pod in bytes.
	DiskUsage int `json:"diskUsage"`
	// Path is the pod data path.
	Path string `json:"path"`
}

// Len is the number of elements in PodDiskUsageList.
func (pl PodDiskUsageList) Len() int {
	return len(pl)
}

// Less reports whether the element with index i should sort before the element with index j.
func (pl PodDiskUsageList) Less(i, j int) bool {
	if i > len(pl) || j > len(pl) {
		return false
	}

	return pl[i].DiskUsage < pl[j].DiskUsage
}

// Swap swaps the elements with indexes i and j.
func (pl PodDiskUsageList) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}
