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
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
)

const (
	ContextKeyMountInfo = "collector.system.mountinfo"

	mountinfoPath = "/proc/1/mountinfo"
)

// mountInfoCollector manages detail of mount of a node
type mountInfoCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// mountInfoCollectorEnabled indicates whether mountInfoCollector is enabled.
	mountInfoCollectorEnabled bool
}

// NewMountInfoCollector creates a new mountInfoCollector.
func NewMountInfoCollector(
	ctx context.Context,
	logger logr.Logger,
	mountInfoCollectorEnabled bool,
) processors.Processor {
	return &mountInfoCollector{
		Context:                   ctx,
		Logger:                    logger,
		mountInfoCollectorEnabled: mountInfoCollectorEnabled,
	}
}

// Handler handles http requests for pod information.
func (mic *mountInfoCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !mic.mountInfoCollectorEnabled {
		http.Error(w, fmt.Sprintf("mountinfocollector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		mountInfoData, err := ioutil.ReadFile(mountinfoPath)
		if err != nil {
			mic.Error(err, "can get mount info", "path", mountinfoPath)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyMountInfo] = string(mountInfoData)
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
