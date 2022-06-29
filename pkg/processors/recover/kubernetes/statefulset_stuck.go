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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/collector/kubernetes"
	"github.com/kubediag/kubediag/pkg/processors/collector/system"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	ContextKeyStatefuSetStuckResultName = "recover.kubernetes.statefulset_stuck.result"
	ContextKeyStatefulSetStuckBugLink   = "recover.kubernetes.statefulset_stuck.bug_link"

	statefulSetStuckBugLink = "https://github.com/kubernetes/kubernetes/issues/67250"
)

// StatefuSetStuck recover a bug of statefulset.
type StatefuSetStuck struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// statefulsetStuckEnabled indicates whether StatefuSetStuck is enabled.
	statefulsetStuckEnabled bool
}

// NewStatefuSetStuck creates a new StatefuSetStuck.
func NewStatefuSetStuck(
	ctx context.Context,
	logger logr.Logger,
	client client.Client,
	statefulsetStuckEnabled bool,
) processors.Processor {
	return &StatefuSetStuck{
		Context:                 ctx,
		Logger:                  logger,
		client:                  client,
		statefulsetStuckEnabled: statefulsetStuckEnabled,
	}
}

// Handler handles http requests for marking node as unschedulable.
func (ss *StatefuSetStuck) Handler(w http.ResponseWriter, r *http.Request) {
	if !ss.statefulsetStuckEnabled {
		http.Error(w, fmt.Sprintf("statefulset stuck is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			ss.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Get StatefuSet info
		if contexts[kubernetes.ContextKeyStatefuSetDetailResult] == "" {
			ss.Error(err, fmt.Sprintf("need %s and %s in extract contexts", kubernetes.ContextKeyPodDetail, system.ContextKeyMountInfo))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		statefulset := appsv1.StatefulSet{}
		err = json.Unmarshal([]byte(contexts[kubernetes.ContextKeyStatefuSetDetailResult]), &statefulset)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to unmarshal statefulset: %v", err), http.StatusInternalServerError)
			return
		}

		// Get Pod Info
		if contexts[kubernetes.ContextKeyPodDetail] == "" {
			ss.Error(err, fmt.Sprintf("need %s in extract contexts", kubernetes.ContextKeyPodDetail))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pod := corev1.Pod{}
		err = json.Unmarshal([]byte(contexts[kubernetes.ContextKeyPodDetail]), &pod)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to unmarshal pod: %v", err), http.StatusInternalServerError)
			return
		}

		statefulsetImages := make(map[string]bool, 0)
		for _, c := range statefulset.Spec.Template.Spec.Containers {
			statefulsetImages[c.Image] = true
		}

		for _, c := range pod.Status.ContainerStatuses {
			ss.Info("confirm images", "container_state", c.State, "statefulset_images", statefulsetImages)
			_, ok := statefulsetImages[c.Image]
			if !ok && c.State.Waiting != nil && (c.State.Waiting.Reason == "ImagePullBackOff" || c.State.Waiting.Reason == "ErrImagePull") {
				ss.Info("start to delete pod", "namespace", pod.Namespace, "name", pod.Name,
					"pod_image", c.Image)
				err = ss.client.Delete(ss.Context, &pod, &client.DeleteOptions{})
				if err != nil {
					ss.Error(err, "failed to delete pod", "namespace", pod.Namespace, "name", pod.Name)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				result := make(map[string]string)
				result[ContextKeyStatefuSetStuckResultName] = fmt.Sprintf("Succeesfully delete pod %s/%s on host", pod.Namespace, pod.Name)
				result[ContextKeyStatefulSetStuckBugLink] = statefulSetStuckBugLink
				data, err := json.Marshal(result)
				if err != nil {
					http.Error(w, fmt.Sprintf("failed to marshal result: %v", err), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
			}
		}

	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
