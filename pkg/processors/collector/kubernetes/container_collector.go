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
	ContextKeyContainerList = "collector.kubernetes.container.list"
)

// containerCollector manages information of all containers on the node.
type containerCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client is the API client that performs all operations against a docker server.
	client *client.Client
	// containerCollectorEnabled indicates whether containerCollector is enabled.
	containerCollectorEnabled bool
}

// NewContainerCollector creates a new containerCollector.
func NewContainerCollector(
	ctx context.Context,
	logger logr.Logger,
	dockerEndpoint string,
	containerCollectorEnabled bool,
) (processors.Processor, error) {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}

	return &containerCollector{
		Context:                   ctx,
		Logger:                    logger,
		client:                    cli,
		containerCollectorEnabled: containerCollectorEnabled,
	}, nil
}

// Handler handles http requests for container information.
func (cc *containerCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !cc.containerCollectorEnabled {
		http.Error(w, fmt.Sprintf("container collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		// List all containers on the node.
		containers, err := cc.listContainers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list containers: %v", err), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(containers)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal containers: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyContainerList] = string(raw)
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

// listContainers lists all containers on the node.
func (cc *containerCollector) listContainers() ([]dockertypes.Container, error) {
	cc.Info("listing containers")

	cc.client.NegotiateAPIVersion(cc)
	containers, err := cc.client.ContainerList(cc, dockertypes.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	return containers, nil
}
