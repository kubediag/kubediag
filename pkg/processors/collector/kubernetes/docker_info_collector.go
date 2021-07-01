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

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
)

const (
	ContextKeyDockerInfo = "collector.kubernetes.docker.info"
)

// dockerInfoCollector fetches system-wide information on docker.
type dockerInfoCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client is the API client that performs all operations against a docker server.
	client *client.Client
	// dockerInfoCollectorEnabled indicates whether dockerInfoCollector is enabled.
	dockerInfoCollectorEnabled bool
}

// NewDockerInfoCollector creates a new dockerInfoCollector.
func NewDockerInfoCollector(
	ctx context.Context,
	logger logr.Logger,
	dockerEndpoint string,
	dockerInfoCollectorEnabled bool,
) (processors.Processor, error) {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}

	return &dockerInfoCollector{
		Context:                    ctx,
		Logger:                     logger,
		client:                     cli,
		dockerInfoCollectorEnabled: dockerInfoCollectorEnabled,
	}, nil
}

// Handler handles http requests for docker information.
func (dc *dockerInfoCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !dc.dockerInfoCollectorEnabled {
		http.Error(w, fmt.Sprintf("docker info collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		// Fetch system-wide information on docker.
		info, err := dc.getDockerInfo()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get information about the docker server.: %v", err), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(info)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal docker info: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyDockerInfo] = string(raw)
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

// getDockerInfo gets information about the docker server.
func (dc *dockerInfoCollector) getDockerInfo() (dockertypes.Info, error) {
	dc.Info("getting information about the docker server")

	dc.client.NegotiateAPIVersion(dc)
	info, err := dc.client.Info(dc)
	if err != nil {
		return dockertypes.Info{}, err
	}

	return info, nil
}
