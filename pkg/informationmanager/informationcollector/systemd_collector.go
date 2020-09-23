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
	"strings"

	"github.com/go-logr/logr"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// systemdCollector manages information of systemd on the node.
type systemdCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
}

// NewSystemdCollector creates a new systemdCollector.
func NewSystemdCollector(
	ctx context.Context,
	logger logr.Logger,
) types.AbnormalProcessor {
	return &systemdCollector{
		Context: ctx,
		Logger:  logger,
	}
}

// Handler handles http requests for systemd information.
func (sc *systemdCollector) Handler(w http.ResponseWriter, r *http.Request) {
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

		// List all systemd unit names in context.
		names, err := util.ListSystemdUnitNamesFromProcessInformationContext(abnormal, sc)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list systemd unit names: %v", err), http.StatusInternalServerError)
			return
		}

		// List properties of specified unit, job, or the manager itself.
		units := make([]types.Unit, 0)
		for _, name := range names {
			properties, err := util.SystemdUnitProperties(name)
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

		// Set systemd property information in status context.
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.SystemdUnitPropertyInformationContextKey, units)
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
		// Systemd unit names are separated via ",", which is "%2C" in url encoding From UTF-8.
		// A query for systemd unit properties of kubelet, docker and manager itself is in the form of:
		//
		// systemdUnitNameInformationContextKey=kubelet%2Cdocker%2C
		values, ok := r.URL.Query()[util.SystemdUnitNameInformationContextKey]
		if !ok || len(values) == 0 {
			http.Error(w, fmt.Sprintf("systemd unit name not specified"), http.StatusBadRequest)
			return
		}
		names := strings.Split(values[0], ",")

		// List properties of specified unit, job, or the manager itself.
		units := make([]types.Unit, 0)
		for _, name := range names {
			properties, err := util.SystemdUnitProperties(name)
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

		data, err := json.Marshal(units)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal units: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
