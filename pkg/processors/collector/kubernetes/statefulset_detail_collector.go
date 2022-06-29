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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	ContextKeyStatefuSetNamespaceKey = "collector.kubernetes.statefulset.namespace"
	ContextKeyStatefuSetNameKey      = "collector.kubernetes.statefulset.name"
	ContextKeyStatefuSetDetailResult = "collector.kubernetes.statefulset.detail.result"
)

// statefulsetDetailCollector manages information of all containers on the node.
type statefulsetDetailCollector struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// statefulsetDetailCollectorEnabled indicates whether statefulsetDetailCollector is enabled.
	statefulsetDetailCollectorEnabled bool
}

// NewStatefuSetDetailCollector creates a new statefulsetDetailCollector.
func NewStatefuSetDetailCollector(
	ctx context.Context,
	logger logr.Logger,
	cache cache.Cache,
	statefulsetDetailCollectorEnabled bool,
) processors.Processor {
	return &statefulsetDetailCollector{
		Context:                           ctx,
		Logger:                            logger,
		cache:                             cache,
		statefulsetDetailCollectorEnabled: statefulsetDetailCollectorEnabled,
	}
}

// extractStatefuSetParameters get name and namespace from context or pod details.
func extractStatefuSetParameters(contexts map[string]string) (name, namespace string, err error) {
	if contexts[ContextKeyStatefuSetNameKey] != "" && contexts[ContextKeyStatefuSetNamespaceKey] != "" {
		return contexts[ContextKeyStatefuSetNameKey], contexts[ContextKeyStatefuSetNamespaceKey], nil
	}

	podDetails, ok := contexts[ContextKeyPodDetail]
	if !ok {
		return "", "", fmt.Errorf("failed to get statefulset name and namespace.")
	}

	pod := corev1.Pod{}
	err = json.Unmarshal([]byte(podDetails), &pod)
	if err != nil {
		return "", "", err
	}

	if pod.OwnerReferences[0].Kind != "StatefulSet" {
		return "", "", fmt.Errorf("failed to get statefulset info from pod detail.")
	}

	return pod.OwnerReferences[0].Name, pod.Namespace, nil
}

// Handler handles http requests for container information.
func (sc *statefulsetDetailCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !sc.statefulsetDetailCollectorEnabled {
		http.Error(w, fmt.Sprintf("StatefuSet Detail Collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			sc.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Get name and namespace from context or pod details.
		name, namespace, err := extractStatefuSetParameters(contexts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Get StatefuSet details.
		var ss appsv1.StatefulSet
		err = sc.cache.Get(sc.Context, client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, &ss)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get statefulset: %v", err), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(ss)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal statefulset: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyStatefuSetDetailResult] = string(raw)
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
