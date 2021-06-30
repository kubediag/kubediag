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

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"syscall"
	"time"

	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	ContextKeyContainerdGoRoutineCollector = "collector.runtime.containerd.goroutine"
)

// containerdGoroutineCollector retrives containerd goroutine on the node.
type containerdGoroutineCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// containerdGoroutineCollectorEnabled indicates whether containerdGoroutineCollector is enabled.
	containerdGoroutineCollectorEnabled bool
}

// NewContainerdGoroutineCollector creates a new containerdGoroutineCollector.
func NewContainerdGoroutineCollector(
	ctx context.Context,
	logger logr.Logger,
	containerdGoroutineCollectorEnabled bool,
) processors.Processor {
	return &containerdGoroutineCollector{
		Context:                             ctx,
		Logger:                              logger,
		containerdGoroutineCollectorEnabled: containerdGoroutineCollectorEnabled,
	}
}

// Handler handles http requests for collecting containerd goroutine.
func (dc *containerdGoroutineCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !dc.containerdGoroutineCollectorEnabled {
		http.Error(w, fmt.Sprintf("containerd goroutine collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		// Get pid of containerd.
		pids, err := util.GetProgramPID("containerd")
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to find pid of containerd: %v", err), http.StatusInternalServerError)
			return
		}

		if len(pids) != 1 {
			http.Error(w, fmt.Sprintf("more than 1 containerd pid found"), http.StatusInternalServerError)
			return
		}

		dc.Info("sending SIGUSR1 to containerd", "pid", pids[0])
		dumpTime := time.Now()
		err = syscall.Kill(pids[0], syscall.SIGUSR1)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to send SIGUSR1 to containerd: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyContainerdGoRoutineCollector] = dumpTime.String()
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
