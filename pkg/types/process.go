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

	psutil "github.com/shirou/gopsutil/process"
)

// Process contains information of a process.
type Process struct {
	// PID is process ID of the process.
	PID int32 `json:"pid"`
	// PPID is parent process ID of the process.
	PPID int32 `json:"ppid"`
	// TGID is thread Group ID of the process.
	TGID int32 `json:"tgid"`
	// Command contains a slice of the process command line arguments.
	Command []string `json:"command"`
	// Status is the process status.
	Status string `json:"status"`
	// CreateTime is created time of the process.
	CreateTime time.Time `json:"createTime"`
	// CPUPercent is percent of the CPU time this process uses.
	CPUPercent float64 `json:"cpuPercent"`
	// Nice is nice value of the process.
	Nice int32 `json:"nice"`
	// MemoryInfo contains memory information.
	MemoryInfo *psutil.MemoryInfoStat `json:"memoryInfo"`
}
