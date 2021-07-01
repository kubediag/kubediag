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
	"os/exec"

	"github.com/go-logr/logr"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/diagnoser/kubernetes"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	ContextKeySubpathRemountRecoverResult = "recover.kubernetes.subpath_remount.result"
)

// subPathRemountRecover recover the bug of subpath-remount
type subPathRemountRecover struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// subPathRemountEnabled indicates whether subPathRemountDiagnoser and subPathRemountRecover is enabled.
	subPathRemountEnabled bool
}

// NewSubPathRemountRecover creates a new subPathRemountRecover.
func NewSubPathRemountRecover(
	ctx context.Context,
	logger logr.Logger,
	subPathRemountEnabled bool,
) processors.Processor {
	return &subPathRemountRecover{
		Context:               ctx,
		Logger:                logger,
		subPathRemountEnabled: subPathRemountEnabled,
	}
}

// Handler handles http requests for pod information.
func (srr *subPathRemountRecover) Handler(w http.ResponseWriter, r *http.Request) {
	if !srr.subPathRemountEnabled {
		http.Error(w, fmt.Sprintf("subpath remount recover is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			srr.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		target := contexts[kubernetes.ContextKeySubpathRemountOriginalDestinationPath]
		if target == "" {
			srr.Error(err, "extract contexts lack of some value", "key", "diagnosis.kubernetes.bug.subpathremount.firstdestination")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		srr.Info("will umount subpath", "target", target)
		err = unmountInHost(target)
		if err != nil {
			srr.Error(err, "failed to umount", "target", target)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeySubpathRemountRecoverResult] = fmt.Sprintf("Succeesfully umount %s on host", target)
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

// unmountInHost unmounts the target.
func unmountInHost(target string) error {
	command := exec.Command("nsenter", "-t", "1", "--mount", "umount", target)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Unmount failed: %v\nUnmounting arguments: %s\nOutput: %s\n", err, target, string(output))
	}
	return nil
}
