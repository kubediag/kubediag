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
	"time"
)

// FileStatusList contains information of itself and all files it contains if it is a directory.
// The caller of sort.Interface must ensure FileStatuses is not nil. It satisfies sort.Interface by
// implemeting the following methods and comparing disk usage of files:
//
// Len() int
// Less(i, j int) bool
// Swap(i, j int)
type FileStatusList struct {
	// FileStatus contains information of a specified file or directory.
	FileStatus `json:",inline"`
	// FileStatuses contains information of files under the specified directory.
	FileStatuses []FileStatus `json:"fileStatuses,omitempty"`
}

// FileStatus contains information of a file.
// See stat(2) linux manual page for more details:
//
// https://man7.org/linux/man-pages/man2/stat.2.html
type FileStatus struct {
	// UID is the uid of a file.
	UID uint32 `json:"uid"`
	// GID is the gid of a file.
	GID uint32 `json:"gid"`
	// Inode represents inode number of a file.
	Inode uint64 `json:"inode"`
	// Links represents number of hard links of a file.
	Links uint64 `json:"links"`
	// Mode represents a file's mode and permission.
	Mode string `json:"mode"`
	// ModTime is the modification time of a file.
	ModTime time.Time `json:"modTime"`
	// DiskUsage is the disk usage of a file in bytes.
	DiskUsage int `json:"diskUsage"`
	// Path is the absolute path of a file.
	Path string `json:"path"`
}

// Len is the number of elements in FileStatusList.
func (fl FileStatusList) Len() int {
	return len(fl.FileStatuses)
}

// Less reports whether the element with index i should sort before the element with index j.
func (fl FileStatusList) Less(i, j int) bool {
	if i > len(fl.FileStatuses) || j > len(fl.FileStatuses) {
		return false
	}

	return fl.FileStatuses[i].DiskUsage < fl.FileStatuses[j].DiskUsage
}

// Swap swaps the elements with indexes i and j.
func (fl FileStatusList) Swap(i, j int) {
	fl.FileStatuses[i], fl.FileStatuses[j] = fl.FileStatuses[j], fl.FileStatuses[i]
}
