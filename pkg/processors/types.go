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
	"context"
	"net/http"

	"github.com/go-logr/logr"
)

const (
	// DefaultTimeoutSeconds is the default number of time out seconds.
	DefaultTimeoutSeconds = 30
)

// Processor manages http requests for processing diagnoses.
type Processor interface {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// Handler handles http requests.
	Handler(http.ResponseWriter, *http.Request)
}

// CommandExecutorRequest is the request body data struct of command executor.
type CommandExecutorRequest struct {
	// Parameter is the parameter for executing a command.
	Parameter string `json:"parameter"`
}

// CommandExecutorRequestParameter is the parameter for executing a command.
type CommandExecutorRequestParameter struct {
	// Command represents a command being prepared and run.
	Command string `json:"command"`
	// Args is arguments to the command.
	Args []string `json:"args,omitempty"`
	// Number of seconds after which the command times out.
	// Defaults to 30 seconds. Minimum value is 1.
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// CommandExecutorResponse is the response body data struct of command executor.
type CommandExecutorResponse struct {
	// Stdout is standard output of the command.
	Stdout string `json:"stdout,omitempty"`
	// Stderr is standard error of the command.
	Stderr string `json:"stderr,omitempty"`
}
