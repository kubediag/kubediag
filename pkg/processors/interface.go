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

package processors

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
)

const (
	// DefaultTimeoutSeconds is the default number of time out seconds.
	DefaultTimeoutSeconds = 30

	// DefaultExpirationSeconds is the default seconds of profiler endpoint expires.
	DefaultExpirationSeconds = 7200
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
