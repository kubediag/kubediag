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

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	psutil "github.com/shirou/gopsutil/process"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/types"
)

const (
	ContextKeyProcessList = "collector.system.process.list"
)

// processCollector manages information of all processes on the node.
type processCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// processCollectorEnabled indicates whether processCollector is enabled.
	processCollectorEnabled bool
}

// NewProcessCollector creates a new processCollector.
func NewProcessCollector(
	ctx context.Context,
	logger logr.Logger,
	processCollectorEnabled bool,
) processors.Processor {
	return &processCollector{
		Context:                 ctx,
		Logger:                  logger,
		processCollectorEnabled: processCollectorEnabled,
	}
}

// Handler handles http requests for process information.
func (pc *processCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !pc.processCollectorEnabled {
		http.Error(w, fmt.Sprintf("process collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		// List all processes on the node.
		processes, err := pc.listProcesses()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list processes: %v", err), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(processes)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal processes: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyProcessList] = string(raw)
		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal result: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// listProcesses lists all processes on the node.
func (pc *processCollector) listProcesses() ([]types.Process, error) {
	pc.Info("listing processes")

	procs, err := psutil.Processes()
	if err != nil {
		return nil, err
	}

	processes := make([]types.Process, 0, len(procs))
	for _, proc := range procs {
		ppid, err := proc.Ppid()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get ppid of %d", proc.Pid))
			continue
		}
		tgid, err := proc.Tgid()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get tgid of %d", proc.Pid))
			continue
		}
		cmdlineSlice, err := proc.CmdlineSlice()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get command line arguments of %d", proc.Pid))
			continue
		}
		status, err := proc.Status()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get status of %d", proc.Pid))
			continue
		}
		createTime, err := proc.CreateTime()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get create time of %d", proc.Pid))
			continue
		}
		cpuPercent, err := proc.CPUPercent()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get cpu percent of %d", proc.Pid))
			continue
		}
		nice, err := proc.Nice()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get nice value of %d", proc.Pid))
			continue
		}
		memoryInfo, err := proc.MemoryInfo()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get memory information of %d", proc.Pid))
			continue
		}

		process := types.Process{
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
