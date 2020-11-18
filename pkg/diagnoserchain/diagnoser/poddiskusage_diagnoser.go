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

package diagnoser

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/go-logr/logr"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// podDiskUsageDiagnoser manages diagnosis that finding disk usage of pods.
type podDiskUsageDiagnoser struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// podDiskUsageDiagnoserEnabled indicates whether podDiskUsageDiagnoser is enabled.
	podDiskUsageDiagnoserEnabled bool
}

// NewPodDiskUsageDiagnoser creates a new podDiskUsageDiagnoser.
func NewPodDiskUsageDiagnoser(
	ctx context.Context,
	logger logr.Logger,
	podDiskUsageDiagnoserEnabled bool,
) types.AbnormalProcessor {
	return &podDiskUsageDiagnoser{
		Context:                      ctx,
		Logger:                       logger,
		podDiskUsageDiagnoserEnabled: podDiskUsageDiagnoserEnabled,
	}
}

// Handler handles http requests for diagnosing pod disk usage.
func (pd *podDiskUsageDiagnoser) Handler(w http.ResponseWriter, r *http.Request) {
	if !pd.podDiskUsageDiagnoserEnabled {
		http.Error(w, fmt.Sprintf("pod disk usage diagnoser is not enabled"), http.StatusUnprocessableEntity)
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

		var abnormal diagnosisv1.Abnormal
		err = json.Unmarshal(body, &abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body into an abnormal: %v", err), http.StatusNotAcceptable)
			return
		}

		// List all pods on the node.
		pods, err := util.ListPodsFromPodInformationContext(abnormal, pd)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}

		// List 10 pods with the most disk usage in descending order.
		sorted := make(types.PodDiskUsageList, 0, len(pods))
		for _, pod := range pods {
			// Get pod data path with kubelet pod directory and uid.
			path := filepath.Join(util.KubeletPodDirectory, string(pod.UID))
			diskUsage, err := util.DiskUsage(path)
			if err != nil {
				pd.Error(err, "failed to get pod disk usage")
			}
			podDiskUsage := types.PodDiskUsage{
				ObjectMeta: pod.ObjectMeta,
				DiskUsage:  diskUsage,
				Path:       path,
			}
			sorted = append(sorted, podDiskUsage)
		}
		sort.Sort(sort.Reverse(sorted))
		if len(sorted) > 10 {
			sorted = sorted[:10]
		}

		// Remove pod information in status context.
		abnormal, removed, err := util.RemoveAbnormalStatusContext(abnormal, util.PodInformationContextKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to remove context field: %v", err), http.StatusInternalServerError)
			return
		}
		if !removed {
			http.Error(w, fmt.Sprintf("failed to remove context field: %v", err), http.StatusInternalServerError)
			return
		}

		// Set pod disk usage diagnosis result in status context.
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.PodDiskUsageDiagnosisContextKey, sorted)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal abnormal: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
