/*
Copyright 2022 The KubeDiag Authors.

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
	"strings"

	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
	"github.com/kubediag/kubediag/pkg/types"
)

const (
	SystemdUnitListParameter = "collector.systemd_unit_list.parameter"
	SystemdUnitListResult    = "collector.systemd_unit_list.result"
)

// systemdCollector manages information of all processes on the node.
type systemdCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// systemdCollectorEnabled indicates whether systemdCollector is enabled.
	systemdCollectorEnabled bool
}

// NewSystemdCollector creates a new systemdCollector.
func NewSystemdCollector(
	ctx context.Context,
	logger logr.Logger,
	systemdCollectorEnabled bool,
) processors.Processor {
	return &systemdCollector{
		Context:                 ctx,
		Logger:                  logger,
		systemdCollectorEnabled: systemdCollectorEnabled,
	}
}

// Handler handles http requests for systemd information.
func (sc *systemdCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !sc.systemdCollectorEnabled {
		http.Error(w, fmt.Sprintf("systemd collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			sc.Error(err, "failed to extract contexts")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Retrieve the list of systemd unit names.
		var names []string
		if value, ok := contexts[SystemdUnitListParameter]; !ok {
			http.Error(w, fmt.Sprintf("must specify systemd unit list"), http.StatusBadRequest)
			return
		} else {
			names = strings.Split(value, ",")
		}

		// List properties of specified unit, job, or the manager itself.
		units := make([]types.Unit, 0)
		for _, name := range names {
			properties, err := types.SystemdUnitProperties(name)
			if err != nil {
				sc.Error(err, "failed to get properties of systemd unit", "unit", name)
				continue
			}
			unit := types.Unit{
				Name:       name,
				Properties: properties,
			}
			units = append(units, unit)
		}

		// Encode the systemd unit properties into JSON string.
		unitsString, err := json.Marshal(units)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal systemd units: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[SystemdUnitListResult] = string(unitsString)

		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal systemd collector result: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
