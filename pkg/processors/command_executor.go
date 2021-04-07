/*
Copyright 2021 The Kube Diagnoser Authors.

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

package processors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"syscall"
	"time"

	"github.com/go-logr/logr"
)

// commandExecutor handles request for running specified command and respond with command result.
type commandExecutor struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// commandExecutorEnabled indicates whether commandExecutor is enabled.
	commandExecutorEnabled bool
}

// NewCommandExecutor creates a new commandExecutor.
func NewCommandExecutor(
	ctx context.Context,
	logger logr.Logger,
	commandExecutorEnabled bool,
) Processor {
	return &commandExecutor{
		Context:                ctx,
		Logger:                 logger,
		commandExecutorEnabled: commandExecutorEnabled,
	}
}

// Handler handles http requests for executing a command.
func (ce *commandExecutor) Handler(w http.ResponseWriter, r *http.Request) {
	if !ce.commandExecutorEnabled {
		http.Error(w, fmt.Sprintf("command executor is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var commandExecutorRequest CommandExecutorRequest
		err = json.Unmarshal(body, &commandExecutorRequest)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body: %v", err), http.StatusNotAcceptable)
			return
		}

		var parameter CommandExecutorRequestParameter
		err = json.Unmarshal([]byte(commandExecutorRequest.Parameter), &parameter)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal command executor request parameter: %v", err), http.StatusNotAcceptable)
			return
		}

		var commandExecutorResponse CommandExecutorResponse
		stdout, stderr, err := ce.executeCommand(parameter.Command, parameter.Args, parameter.TimeoutSeconds)
		if stdout != "" {
			commandExecutorResponse.Stdout = stdout
		}
		if stderr != "" {
			commandExecutorResponse.Stderr = stderr
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(commandExecutorResponse)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal command executor response: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// RunCommandExecutor runs the command with timeout.
// It returns stdout, stderr and an error.
func (ce *commandExecutor) executeCommand(name string, args []string, timeoutSeconds *int32) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("invalid command name")
	}

	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	// Setting a new process group id to avoid suicide.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Start()
	if err != nil {
		return "", "", err
	}

	// Wait and signal completion of command.
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	var timeout <-chan time.Time
	if timeoutSeconds != nil {
		timeout = time.After(time.Duration(*timeoutSeconds) * time.Second)
	} else {
		timeout = time.After(time.Duration(DefaultTimeoutSeconds) * time.Second)
	}

	select {
	// Kill the process if timeout happened.
	case <-timeout:
		// Kill the process and all of its children with its process group id.
		// TODO: Kill timed out process on failure.
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			ce.Error(err, "failed to get process group id on command timed out", "command", name)
		} else {
			err = syscall.Kill(-pgid, syscall.SIGKILL)
			if err != nil {
				ce.Error(err, "failed to kill process on command timed out", "command", name)
			}
		}

		return "", "", fmt.Errorf("command %v timed out", name)
	// Set output and error if command completed before timeout.
	case err := <-done:
		if err != nil {
			if cmd.Stderr != nil {
				return "", "", fmt.Errorf(buf.String())
			}
			return "", "", err
		}

		return buf.String(), "", nil
	}
}
