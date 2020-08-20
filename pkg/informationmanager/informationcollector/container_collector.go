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

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-logr/logr"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// containerCollector manages information of all containers on the node.
type containerCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client is the API client that performs all operations against a docker server.
	client *client.Client
}

// NewContainerCollector creates a new ContainerCollector.
func NewContainerCollector(
	ctx context.Context,
	logger logr.Logger,
	dockerEndpoint string,
) (types.AbnormalProcessor, error) {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}

	return &containerCollector{
		Context: ctx,
		Logger:  logger,
		client:  cli,
	}, nil
}

// Handler handles http requests for container information.
func (cc *containerCollector) Handler(w http.ResponseWriter, r *http.Request) {
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

		// List all containers on the node.
		containers, err := cc.listContainers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list containers: %v", err), http.StatusInternalServerError)
			return
		}

		// Set container information in status context.
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.ContainerInformationContextKey, containers)
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
		// List all containers on the node.
		containers, err := cc.listContainers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list containers: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(containers)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal containers: %v", err), http.StatusInternalServerError)
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

	cc.client.NegotiateAPIVersion(cc.Context)
	containers, err := cc.client.ContainerList(cc.Context, dockertypes.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	return containers, nil
}
