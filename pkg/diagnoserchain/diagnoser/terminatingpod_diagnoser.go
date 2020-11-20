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
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// terminatingPodDiagnoser manages diagnosis on terminating pods.
type terminatingPodDiagnoser struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// terminatingPodDiagnoserEnabled indicates whether terminatingPodDiagnoser is enabled.
	terminatingPodDiagnoserEnabled bool
}

// NewTerminatingPodDiagnoser creates a new terminatingPodDiagnoser.
func NewTerminatingPodDiagnoser(
	ctx context.Context,
	logger logr.Logger,
	terminatingPodDiagnoserEnabled bool,
) types.AbnormalProcessor {
	return &terminatingPodDiagnoser{
		Context:                        ctx,
		Logger:                         logger,
		terminatingPodDiagnoserEnabled: terminatingPodDiagnoserEnabled,
	}
}

// Handler handles http requests for diagnosing terminating pods.
func (td *terminatingPodDiagnoser) Handler(w http.ResponseWriter, r *http.Request) {
	if !td.terminatingPodDiagnoserEnabled {
		http.Error(w, fmt.Sprintf("terminating pod diagnoser is not enabled"), http.StatusUnprocessableEntity)
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
		pods, err := util.ListPodsFromPodInformationContext(abnormal, td)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}

		// List all processes on the node.
		processes, err := util.ListProcessesFromProcessInformationContext(abnormal, td)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list processes: %v", err), http.StatusInternalServerError)
			return
		}

		// Get all containerd shim processes from pods which are not terminated by deadline.
		abnormalPods := td.getAbnormalPods(pods)
		abnormalShimProcesses, found := td.getContainerdShimProcessesFromPods(abnormalPods, processes)
		if !found {
			http.Error(w, fmt.Sprintf("containerd shim processes of abnormal pods not found"), http.StatusInternalServerError)
			return
		}

		// Set kill signals in status context.
		signals := make(types.SignalList, 0, len(abnormalShimProcesses))
		for _, process := range abnormalShimProcesses {
			signals = append(signals, types.Signal{
				PID:    int(process.PID),
				Signal: syscall.SIGKILL,
			})
		}
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.SignalRecoveryContextKey, signals)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
		}

		// Set terminating pod diagnosis result in status context.
		abnormal, err = util.SetAbnormalStatusContext(abnormal, util.TerminatingPodDiagnosisContextKey, abnormalPods)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
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

		// Remove process information in status context.
		abnormal, removed, err = util.RemoveAbnormalStatusContext(abnormal, util.ProcessInformationContextKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to remove context field: %v", err), http.StatusInternalServerError)
			return
		}
		if !removed {
			http.Error(w, fmt.Sprintf("failed to remove context field: %v", err), http.StatusInternalServerError)
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

// getAbnormalPods gets abnormal pods which are not terminated by deadline.
// The deadline is calculated by the following way:
//
// Deadline = DeletionTimestamp + DeletionGracePeriodSeconds + PodKillGracePeriodSeconds
func (td *terminatingPodDiagnoser) getAbnormalPods(pods []corev1.Pod) []corev1.Pod {
	result := make([]corev1.Pod, 0)
	for _, pod := range pods {
		// A pod is in terminating state if its DeletionTimestamp is set and the state of the pod could be obtained.
		if pod.DeletionTimestamp != nil && pod.Status.Phase != corev1.PodUnknown {
			var gracePeriod time.Duration
			if pod.Spec.TerminationGracePeriodSeconds != nil {
				gracePeriod = time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
			} else {
				gracePeriod = corev1.DefaultTerminationGracePeriodSeconds
			}

			// A pod is taken as an abormal pod if the pod is not terminated by deadline.
			deadline := metav1.NewTime(pod.DeletionTimestamp.Add(gracePeriod).Add(util.PodKillGracePeriodSeconds))
			now := metav1.Now()
			if (&deadline).Before(&now) {
				result = append(result, pod)
			}
		}
	}

	return result
}

// getContainerdShimProcessesFromPods gets all containerd shim processes from pod details.
// Returns containerd shim process slice and true if found, otherwise returns nil and false.
func (td *terminatingPodDiagnoser) getContainerdShimProcessesFromPods(pods []corev1.Pod, processes []types.Process) ([]types.Process, bool) {
	result := make([]types.Process, 0)
	for _, pod := range pods {
		td.Info("getting containerd shim processes from pod", "pod", client.ObjectKey{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		})
		for _, process := range processes {
			// Skip if the process is not a valid containerd shim.
			workdir, found := td.getContainerdShimWorkdir(process.Command)
			if !found {
				continue
			}

			// Retrieve container id from containerd shim working directory.
			containerID, err := td.getContainerIDFromContainerdShimWorkdir(workdir)
			if err != nil {
				td.Error(err, "unable to get container id from working directory", "pod", client.ObjectKey{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				}, "process", process.PID, "command", process.Command)
				continue
			}

			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.ContainerID == "" {
					td.Info("terminating pod has no running container", "pod", client.ObjectKey{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					})
					continue
				}

				id, err := td.parseContainerID(containerStatus.ContainerID)
				if err != nil {
					td.Error(err, "failed to parse container id from containerID string", "pod", client.ObjectKey{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					}, "container", containerStatus.ContainerID)
					continue
				}

				if id == containerID {
					result = append(result, process)
				}
			}
		}
	}

	if len(result) == 0 {
		return nil, false
	}

	return result, true
}

// getContainerdShimWorkdir retrives working directory of the containerd shim. A command contains substring
// "containerd-shim" and "-workdir" is a valid containerd shim process.
// Returns working directory and true if the command is a valid containerd shim process, otherwise returns
// empty string and false.
func (td *terminatingPodDiagnoser) getContainerdShimWorkdir(command []string) (string, bool) {
	cmdString := strings.Join(command, " ")
	if strings.Contains(cmdString, "containerd-shim") && strings.Contains(cmdString, "-workdir") {
		for index, arg := range command {
			if arg == "-workdir" && len(command)-2 > index {
				return command[index+1], true
			}
		}
	}

	return "", false
}

// getContainerIDFromContainerdShimWorkdir retrieves container id from containerd shim working directory.
func (td *terminatingPodDiagnoser) getContainerIDFromContainerdShimWorkdir(workdir string) (string, error) {
	workdirSlice := strings.Split(workdir, "/")
	if workdirSlice == nil || len(workdirSlice) < 1 {
		return "", fmt.Errorf("invalid workdir: %s", workdir)
	}

	return workdirSlice[len(workdirSlice)-1], nil
}

// parseContainerID is a convenience method for retrieving container id from an id string in pod containerStatus.
func (td *terminatingPodDiagnoser) parseContainerID(data string) (string, error) {
	// Trim the quotes and split the type and ID.
	parts := strings.Split(strings.Trim(data, "\""), "://")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid container id: %s", data)
	}

	return parts[1], nil
}
