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
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	// defaultExecRoot is the default root path to store docker goroutine dump logs.
	defaultExecRoot = "/var/run/docker"
	// stacksLogNamePrefix is the prefix for dockerd to generate goroutine stack logs.
	stacksLogNamePrefix = "goroutine-stacks"
	// stacksLogSubPath is the subpath for kubediag to store dockerd goroutine stack logs.
	stacksLogSubPath = "dockerd-goroutine"

	ContextKeyDockerdGoRoutineCollector = "collector.runtime.dockerd.goroutine"
)

// dockerdGoroutineCollector retrives dockerd goroutine on the node.
type dockerdGoroutineCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// dataRoot is root directory of persistent kubediag data.
	dataRoot string
	// dockerdGoroutineCollectorEnabled indicates whether dockerdGoroutineCollector is enabled.
	dockerdGoroutineCollectorEnabled bool
}

// NewDockerdGoroutineCollector creates a new dockerdGoroutineCollector.
func NewDockerdGoroutineCollector(
	ctx context.Context,
	logger logr.Logger,
	dataRoot string,
	dockerdGoroutineCollectorEnabled bool,
) processors.Processor {
	return &dockerdGoroutineCollector{
		Context:                          ctx,
		Logger:                           logger,
		dataRoot:                         dataRoot,
		dockerdGoroutineCollectorEnabled: dockerdGoroutineCollectorEnabled,
	}
}

// Handler handles http requests for collecting dockerd goroutine.
func (dc *dockerdGoroutineCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !dc.dockerdGoroutineCollectorEnabled {
		http.Error(w, fmt.Sprintf("dockerd goroutine collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		// Get pid of docker daemon.
		pids, err := util.GetProgramPID("dockerd")
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to find pid of dockerd: %v", err), http.StatusInternalServerError)
			return
		}

		if len(pids) != 1 {
			http.Error(w, fmt.Sprintf("more than 1 dockerd pid found"), http.StatusInternalServerError)
			return
		}

		// A full stack trace of docker daemon can be logged by sending a SIGUSR1 signal to the daemon.
		// https://docs.docker.com/config/daemon/#force-a-stack-trace-to-be-logged
		dc.Info("sending SIGUSR1 to dockerd", "pid", pids[0])
		err = syscall.Kill(pids[0], syscall.SIGUSR1)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to send SIGUSR1 to dockerd: %v", err), http.StatusInternalServerError)
			return
		}

		// Wait 5 seconds for docker daemon to generate stack log.
		<-time.After(5 * time.Second)

		files, err := ioutil.ReadDir(defaultExecRoot)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to get files under %s: %v", defaultExecRoot, err), http.StatusInternalServerError)
			return
		}

		// Sort all files by modification time and move the latest goroutine log to kubediag data path.
		var stacksLogPath string
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime().After(files[j].ModTime())
		})
		for _, file := range files {
			if strings.HasPrefix(file.Name(), stacksLogNamePrefix) {
				// Create directory to store dockerd goroutines if the directory is not exist.
				stacksLogDir := filepath.Join(dc.dataRoot, stacksLogSubPath)
				if _, err := os.Stat(stacksLogDir); os.IsNotExist(err) {
					err := os.MkdirAll(stacksLogDir, os.ModePerm)
					if err != nil {
						http.Error(w, fmt.Sprintf("unable to create directory %s to store dockerd goroutines: %v", stacksLogDir, err), http.StatusInternalServerError)
						return
					}
				}

				// Move dockerd stack log to kubediag data path.
				stacksLogPath = filepath.Join(stacksLogDir, file.Name())
				_, err := util.BlockingRunCommandWithTimeout([]string{"mv", filepath.Join(defaultExecRoot, file.Name()), stacksLogPath}, 30)
				if err != nil {
					http.Error(w, fmt.Sprintf("unable to move dockerd stack log to %s: %v", stacksLogPath, err), http.StatusInternalServerError)
					return
				}
				break
			}
		}

		result := make(map[string]string)
		result[ContextKeyDockerdGoRoutineCollector] = stacksLogPath
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
