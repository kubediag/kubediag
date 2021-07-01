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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubediag/kubediag/pkg/executor"
	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	ContextKeyPodDetail = "collector.kubernetes.pod.detail"
)

// podDetailCollector manages detail of a pod.
type podDetailCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// podCollectorEnabled indicates whether podListCollector and podDetailCollector is enabled.
	podCollectorEnabled bool
}

// NewPodDetailCollector creates a new podDetailCollector.
func NewPodDetailCollector(
	ctx context.Context,
	logger logr.Logger,
	cache cache.Cache,
	nodeName string,
	podCollectorEnabled bool,
) processors.Processor {
	return &podDetailCollector{
		Context:             ctx,
		Logger:              logger,
		cache:               cache,
		podCollectorEnabled: podCollectorEnabled,
	}
}

// Handler handles http requests for pod information.
func (pc *podDetailCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !pc.podCollectorEnabled {
		http.Error(w, fmt.Sprintf("pod collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			pc.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if contexts[executor.PodNamespaceTelemetryKey] == "" ||
			contexts[executor.PodNameTelemetryKey] == "" {
			pc.Error(err, "extract contexts lack of pod namespace and name")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pod := corev1.Pod{}
		err = pc.cache.Get(pc.Context,
			client.ObjectKey{
				Namespace: contexts[executor.PodNamespaceTelemetryKey],
				Name:      contexts[executor.PodNameTelemetryKey],
			}, &pod)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get pod: %v", err), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(pod)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal pod: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyPodDetail] = string(raw)
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
