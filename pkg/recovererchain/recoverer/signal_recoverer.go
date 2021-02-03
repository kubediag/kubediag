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

package recoverer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"syscall"

	"github.com/go-logr/logr"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/types"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

// signalRecoverer manages recovery that sending signal to processes.
type signalRecoverer struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// signalRecovererEnabled indicates whether signalRecoverer is enabled.
	signalRecovererEnabled bool
}

// NewSignalRecoverer creates a new signalRecoverer.
func NewSignalRecoverer(
	ctx context.Context,
	logger logr.Logger,
	signalRecovererEnabled bool,
) types.DiagnosisProcessor {
	return &signalRecoverer{
		Context:                ctx,
		Logger:                 logger,
		signalRecovererEnabled: signalRecovererEnabled,
	}
}

// Handler handles http requests for sending signal to processes.
func (sr *signalRecoverer) Handler(w http.ResponseWriter, r *http.Request) {
	if !sr.signalRecovererEnabled {
		http.Error(w, fmt.Sprintf("signal recoverer is not enabled"), http.StatusUnprocessableEntity)
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

		var diagnosis diagnosisv1.Diagnosis
		err = json.Unmarshal(body, &diagnosis)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body into an diagnosis: %v", err), http.StatusNotAcceptable)
			return
		}

		// Get process signal details.
		signals, err := util.ListSignalsFromSignalRecoveryContext(diagnosis, sr)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get process signal details: %v", err), http.StatusInternalServerError)
			return
		}

		// Send signals to processes.
		for _, signal := range signals {
			sr.Info("sending signal to process", "process", signal.PID, "signal", signal.Signal)
			err := syscall.Kill(signal.PID, signal.Signal)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to send signal %d to process %d: %v", signal.Signal, signal.PID, err), http.StatusInternalServerError)
				return
			}
		}

		data, err := json.Marshal(diagnosis)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal diagnosis: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
