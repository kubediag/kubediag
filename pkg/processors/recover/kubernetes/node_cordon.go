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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubediag/kubediag/pkg/processors"
)

const (
	ContextKeyNodeCordonResultName = "recover.kubernetes.node_cordon.result.name"
)

// nodeCordon marks node as unschedulable.
type nodeCordon struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// nodeName specifies the node name.
	nodeName string
	// nodeCordonEnabled indicates whether nodeCordon is enabled.
	nodeCordonEnabled bool
}

// NewNodeCordon creates a new nodeCordon.
func NewNodeCordon(
	ctx context.Context,
	logger logr.Logger,
	client client.Client,
	nodeName string,
	nodeCordonEnabled bool,
) processors.Processor {
	return &nodeCordon{
		Context:           ctx,
		Logger:            logger,
		client:            client,
		nodeName:          nodeName,
		nodeCordonEnabled: nodeCordonEnabled,
	}
}

// Handler handles http requests for marking node as unschedulable.
func (nc *nodeCordon) Handler(w http.ResponseWriter, r *http.Request) {
	if !nc.nodeCordonEnabled {
		http.Error(w, fmt.Sprintf("node cordon is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		var node corev1.Node
		if err := nc.client.Get(nc, client.ObjectKey{Name: nc.nodeName}, &node); err != nil {
			http.Error(w, fmt.Sprintf("unable to fetch Node"), http.StatusUnprocessableEntity)
			return
		}

		result := make(map[string]string)
		result[ContextKeyNodeCordonResultName] = nc.nodeName
		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal result: %v", err), http.StatusInternalServerError)
			return
		}

		if node.Spec.Unschedulable {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}

		node.Spec.Unschedulable = true
		if err := nc.client.Update(nc, &node); err != nil {
			http.Error(w, fmt.Sprintf("unable to update Node"), http.StatusUnprocessableEntity)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
