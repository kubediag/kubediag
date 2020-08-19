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
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// processCollector manages information of all processes on the node.
type processCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
}

// NewProcessCollector creates a new ProcessCollector.
func NewProcessCollector(
	ctx context.Context,
	logger logr.Logger,
) types.AbnormalProcessor {
	return &processCollector{
		Context: ctx,
		Logger:  logger,
	}
}

// Handler handles http requests for process information.
func (pc *processCollector) Handler(w http.ResponseWriter, r *http.Request) {
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
		processes, err := pc.listProcesses()
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
		processes, err := pc.listProcesses()
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
		}
		tgid, err := proc.Tgid()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get tgid of %d", proc.Pid))
		}
		cmdlineSlice, err := proc.CmdlineSlice()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get command line arguments of %d", proc.Pid))
		}
		status, err := proc.Status()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get status of %d", proc.Pid))
		}
		createTime, err := proc.CreateTime()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get create time of %d", proc.Pid))
		}
		cpuPercent, err := proc.CPUPercent()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get cpu percent of %d", proc.Pid))
		}
		nice, err := proc.Nice()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get nice value of %d", proc.Pid))
		}
		memoryInfo, err := proc.MemoryInfo()
		if err != nil {
			pc.Error(err, fmt.Sprintf("unable to get memory information of %d", proc.Pid))
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
