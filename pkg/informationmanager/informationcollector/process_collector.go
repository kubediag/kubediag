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

package informationcollector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	psutil "github.com/shirou/gopsutil/process"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// ProcessCollector manages information of all processes on the node.
type ProcessCollector interface {
	Handler(http.ResponseWriter, *http.Request)
	ListProcesses() ([]Process, error)
}

// processCollectorImpl implements ProcessCollector interface.
type processCollectorImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Log represents the ability to log messages.
	Log logr.Logger
}

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

// NewProcessCollector creates a new ProcessCollector.
func NewProcessCollector(
	ctx context.Context,
	log logr.Logger,
) ProcessCollector {
	return &processCollectorImpl{
		Context: ctx,
		Log:     log,
	}
}

// Handler handles http requests for process information.
func (pc *processCollectorImpl) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var abnormal diagnosisv1.Abnormal
		err = json.Unmarshal(body, &abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body into an abnormal: %v", err), http.StatusNotAcceptable)
			return
		}

		// List all processes on the node.
		processes, err := pc.ListProcesses()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list processes: %v", err), http.StatusInternalServerError)
			return
		}

		// Set process information in status context.
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.ProcessInformationContextKey, processes)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal abnormal: %v", err), http.StatusInternalServerError)
			return
		}

		// Response with error if abnormal data size exceeds max data size.
		if len(data) > util.MaxDataSize {
			http.Error(w, fmt.Sprintf("abnormal data size %d exceeds max data size %d", len(data), util.MaxDataSize), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	case "GET":
		// List all processes on the node.
		processes, err := pc.ListProcesses()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list processes: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(processes)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal processes: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// ListProcesses lists all processes on the node.
func (pc *processCollectorImpl) ListProcesses() ([]Process, error) {
	pc.Log.Info("listing processes")

	procs, err := psutil.Processes()
	if err != nil {
		return nil, err
	}

	processes := make([]Process, 0, len(procs))
	for _, proc := range procs {
		ppid, err := proc.Ppid()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get ppid of %d", proc.Pid))
		}
		tgid, err := proc.Tgid()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get tgid of %d", proc.Pid))
		}
		cmdlineSlice, err := proc.CmdlineSlice()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get command line arguments of %d", proc.Pid))
		}
		status, err := proc.Status()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get status of %d", proc.Pid))
		}
		createTime, err := proc.CreateTime()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get create time of %d", proc.Pid))
		}
		cpuPercent, err := proc.CPUPercent()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get cpu percent of %d", proc.Pid))
		}
		nice, err := proc.Nice()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get nice value of %d", proc.Pid))
		}
		memoryInfo, err := proc.MemoryInfo()
		if err != nil {
			pc.Log.Error(err, fmt.Sprintf("unable to get memory information of %d", proc.Pid))
		}

		process := Process{
			PID:        proc.Pid,
			PPID:       ppid,
			TGID:       tgid,
			Command:    cmdlineSlice,
			Status:     status,
			CreateTime: time.Unix(0, createTime*int64(time.Millisecond)),
			CPUPercent: cpuPercent,
			Nice:       nice,
			MemoryInfo: memoryInfo,
		}
		processes = append(processes, process)
	}

	return processes, nil
}
