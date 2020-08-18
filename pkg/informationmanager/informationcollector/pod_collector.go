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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// PodCollector manages information of all pods on the node.
type PodCollector interface {
	Handler(http.ResponseWriter, *http.Request)
	ListPods() ([]corev1.Pod, error)
}

// podCollectorImpl implements PodCollector interface.
type podCollectorImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Log represents the ability to log messages.
	Log logr.Logger
	// Cache knows how to load Kubernetes objects.
	Cache cache.Cache
	// NodeName specifies the node name.
	NodeName string
}

// NewPodCollector creates a new PodCollector.
func NewPodCollector(
	ctx context.Context,
	log logr.Logger,
	cache cache.Cache,
	nodeName string,
) PodCollector {
	return &podCollectorImpl{
		Context:  ctx,
		Log:      log,
		Cache:    cache,
		NodeName: nodeName,
	}
}

// Handler handles http requests for pod information.
func (pc *podCollectorImpl) Handler(w http.ResponseWriter, r *http.Request) {
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

		// List all pods on the node.
		pods, err := pc.ListPods()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}

		// Set pod information in status context.
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.PodInformationContextKey, pods)
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
		// List all pods on the node.
		pods, err := pc.ListPods()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(pods)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal pods: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// ListPods lists Pods from cache.
func (pc *podCollectorImpl) ListPods() ([]corev1.Pod, error) {
	pc.Log.Info("listing Pods on node")

	var podList corev1.PodList
	if err := pc.Cache.List(pc.Context, &podList); err != nil {
		return nil, err
	}

	podsOnNode := util.RetrievePodsOnNode(podList.Items, pc.NodeName)

	return podsOnNode, nil
}
