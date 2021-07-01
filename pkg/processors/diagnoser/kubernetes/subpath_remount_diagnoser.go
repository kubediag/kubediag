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
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/collector/kubernetes"
	"github.com/kubediag/kubediag/pkg/processors/collector/system"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	ContextKeySubpathRemountDiagnosisResult         = "diagnoser.kubernetes.subpath_remount.result"
	ContextKeySubpathRemountBugLink                 = "diagnoser.kubernetes.subpath_remount.bug_link"
	ContextKeySubpathRemountOriginalSourcePath      = "diagnoser.kubernetes.subpath_remount.original_source_path"
	ContextKeySubpathRemountOriginalDestinationPath = "diagnoser.kubernetes.subpath_remount.original_destination_path"

	bugLink    = "https://github.com/kubernetes/kubernetes/issues/68211"
	matchRegex = `^.*mounting.*volume-subpaths.*to rootfs.*at.*no such file or directory.*`
)

// subPathRemountDiagnoser will diagnosis a bug that happens when remount subpath
type subPathRemountDiagnoser struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// subPathRemountEnabled indicates whether subPathRemountDiagnoser and subPathRemountRecover is enabled.
	subPathRemountEnabled bool
}

// NewSubPathRemountDiagnoser creates a new subPathRemountDiagnoser.
func NewSubPathRemountDiagnoser(
	ctx context.Context,
	logger logr.Logger,
	cache cache.Cache,
	subPathRemountEnabled bool,
) processors.Processor {
	return &subPathRemountDiagnoser{
		Context:               ctx,
		Logger:                logger,
		cache:                 cache,
		subPathRemountEnabled: subPathRemountEnabled,
	}
}

// Handler handles http requests for pod information.
func (srd *subPathRemountDiagnoser) Handler(w http.ResponseWriter, r *http.Request) {
	if !srd.subPathRemountEnabled {
		http.Error(w, fmt.Sprintf("subpath remount diagnosis is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			srd.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if contexts[kubernetes.ContextKeyPodDetail] == "" || contexts[system.ContextKeyMountInfo] == "" {
			srd.Error(err, fmt.Sprintf("need %s and %s in extract contexts", kubernetes.ContextKeyPodDetail, system.ContextKeyMountInfo))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pod := corev1.Pod{}
		err = json.Unmarshal([]byte(contexts[kubernetes.ContextKeyPodDetail]), &pod)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to unmarshal pod: %v", err), http.StatusInternalServerError)
			return
		}

		// condition 1: pod mount as subPath from a configmap or a secret
		subPathMount := false
		for _, c := range pod.Spec.Containers {
			for _, vm := range c.VolumeMounts {
				if vm.SubPath != "" || vm.SubPathExpr != "" {
					if mountFromConfigmapOrSecret(pod.Spec.Volumes, vm.Name) {
						subPathMount = true
						break
					}
				}
			}
			if subPathMount {
				break
			}
		}

		if !subPathMount {
			srd.Error(err, "not a subpath remount pod")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// condition 2: pod is stuck in creating and always failed with specific message
		message := ""
		for _, containerStat := range pod.Status.ContainerStatuses {
			if containerStat.Ready {
				continue
			}
			if containerStat.LastTerminationState.Terminated != nil {
				message = containerStat.LastTerminationState.Terminated.Message
				break
			}
			if containerStat.State.Waiting != nil {
				message = containerStat.State.Waiting.Message
				break
			}
			if containerStat.State.Terminated != nil {
				message = containerStat.State.Terminated.Message
				break
			}
		}

		reg, err := regexp.Compile(matchRegex)
		if err != nil {
			srd.Error(err, "regex: %s is invalid", "regex", matchRegex)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !reg.MatchString(message) {
			srd.Info("container start-message can not match subpath-bug-regex", "message", message)
			http.Error(w, fmt.Sprintf("container start-message can not match subpath-bug-regex"), http.StatusInternalServerError)
			return
		}
		sourcePath := getMountSourcePath(message)
		if sourcePath == "" {
			srd.Info("can not find source path from message", "message", message)
			http.Error(w, fmt.Sprintf("container start-message does not contain mount source"), http.StatusInternalServerError)
			return
		}
		srd.Info("get mount source path", "path", sourcePath)

		mountInfos := strings.Split(contexts[system.ContextKeyMountInfo], "\n")
		var mountLine string
		for _, line := range mountInfos {
			if strings.Contains(line, sourcePath) {
				mountLine = line
				break
			}
		}
		src, dst, deleted := isSourceDeleted(mountLine)
		if !deleted {
			srd.Info("can not get a deleted mount source", "mountLine", mountLine)
			http.Error(w, fmt.Sprintf("can not find a deleted mount source"), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeySubpathRemountDiagnosisResult] = "A kubernetes bug #68211 has been encountered on pod subpath remounting."
		result[ContextKeySubpathRemountBugLink] = bugLink
		result[ContextKeySubpathRemountOriginalSourcePath] = src
		result[ContextKeySubpathRemountOriginalDestinationPath] = dst
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

func mountFromConfigmapOrSecret(volumes []corev1.Volume, name string) bool {
	for _, v := range volumes {
		if v.Name != name {
			continue
		}
		if v.ConfigMap != nil || v.Secret != nil {
			return true
		}
	}
	return false
}

func getMountSourcePath(path string) string {
	subStrs1 := strings.SplitN(path, "mounting", 2)
	if len(subStrs1) < 2 {
		return ""
	}
	subStrs2 := strings.SplitN(subStrs1[1], "to rootfs", 2)
	if len(subStrs2) < 2 {
		return ""
	}
	subStrs3 := strings.SplitN(subStrs2[0], `"`, 3)
	if len(subStrs3) < 3 {
		return ""
	}
	src := strings.Trim(subStrs3[1], `\`)
	return src
}

func isSourceDeleted(mount string) (string, string, bool) {
	subStrs := strings.Split(mount, " ")
	return subStrs[3], subStrs[4], strings.HasSuffix(subStrs[3], `/deleted`)
}
