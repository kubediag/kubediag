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

package recovererchain

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// recovererChain manages recoverers in the system.
type recovererChain struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// eventRecorder knows how to record events on behalf of an EventSource.
	eventRecorder record.EventRecorder
	// scheme defines methods for serializing and deserializing API objects.
	scheme *runtime.Scheme
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// nodeName specifies the node name.
	nodeName string
	// transport is the transport for sending http requests to recoverers.
	transport *http.Transport
	// recovererChainCh is a channel for queuing Abnormals to be processed by recoverer chain.
	recovererChainCh chan diagnosisv1.Abnormal
}

// NewRecovererChain creates a new recovererChain.
func NewRecovererChain(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	recovererChainCh chan diagnosisv1.Abnormal,
) types.AbnormalManager {
	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &recovererChain{
		Context:          ctx,
		Logger:           logger,
		client:           cli,
		eventRecorder:    eventRecorder,
		scheme:           scheme,
		cache:            cache,
		nodeName:         nodeName,
		transport:        transport,
		recovererChainCh: recovererChainCh,
	}
}

// Run runs the recoverer chain.
func (rc *recovererChain) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !rc.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in recoverer chain channel.
		case abnormal := <-rc.recovererChainCh:
			if util.IsAbnormalNodeNameMatched(abnormal, rc.nodeName) {
				abnormal, err := rc.SyncAbnormal(abnormal)
				if err != nil {
					rc.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				rc.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}
		// Stop recoverer chain on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncAbnormal syncs abnormals.
func (rc *recovererChain) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	rc.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	recoverers, err := rc.listRecoverers()
	if err != nil {
		rc.Error(err, "failed to list Recoverers")
		rc.addAbnormalToRecovererChainQueue(abnormal)
		return abnormal, err
	}

	abnormal, err = rc.runRecovery(recoverers, abnormal)
	if err != nil {
		rc.Error(err, "failed to run recovery")
		rc.addAbnormalToRecovererChainQueue(abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// Handler handles http requests and response with recoverers.
func (rc *recovererChain) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		recoverers, err := rc.listRecoverers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list recoverers: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(recoverers)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal recoverers: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// listRecoverers lists Recoverers from cache.
func (rc *recovererChain) listRecoverers() ([]diagnosisv1.Recoverer, error) {
	rc.Info("listing Recoverers")

	var recovererList diagnosisv1.RecovererList
	if err := rc.cache.List(rc, &recovererList); err != nil {
		return nil, err
	}

	return recovererList.Items, nil
}

// runRecovery recovers an abnormal with recoverers.
func (rc *recovererChain) runRecovery(recoverers []diagnosisv1.Recoverer, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Run command executor of Recoverer type.
	for _, executor := range abnormal.Spec.CommandExecutors {
		if executor.Type == diagnosisv1.RecovererType {
			executor, err := util.RunCommandExecutor(executor, rc)
			if err != nil {
				rc.Error(err, "failed to run command executor", "command", executor.Command, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				executor.Error = err.Error()
			}

			abnormal.Status.CommandExecutors = append(abnormal.Status.CommandExecutors, executor)
		}
	}

	// Run profiler of Recoverer type.
	for _, profiler := range abnormal.Spec.Profilers {
		if profiler.Type == diagnosisv1.RecovererType {
			profiler, err := util.RunProfiler(rc, abnormal.Name, abnormal.Namespace, profiler, rc.client, rc)
			if err != nil {
				rc.Error(err, "failed to run profiler", "profiler", profiler, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				profiler.Error = err.Error()
			}

			abnormal.Status.Profilers = append(abnormal.Status.Profilers, profiler)
		}
	}

	// Skip recovery if AssignedRecoverers is empty.
	if len(abnormal.Spec.AssignedRecoverers) == 0 {
		rc.Info("skipping recovery", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		rc.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "SkippingRecovery", "Skipping recovery")

		abnormal, err := rc.setAbnormalSucceeded(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	for _, recoverer := range recoverers {
		// Execute only matched recoverers.
		matched := false
		for _, assignedRecoverer := range abnormal.Spec.AssignedRecoverers {
			if recoverer.Name == assignedRecoverer.Name && recoverer.Namespace == assignedRecoverer.Namespace {
				rc.Info("assigned recoverer matched", "recoverer", client.ObjectKey{
					Name:      recoverer.Name,
					Namespace: recoverer.Namespace,
				}, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		rc.Info("running recovery", "recoverer", client.ObjectKey{
			Name:      recoverer.Name,
			Namespace: recoverer.Namespace,
		}, "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		scheme := strings.ToLower(string(recoverer.Spec.Scheme))
		host := recoverer.Spec.IP
		port := recoverer.Spec.Port
		path := recoverer.Spec.Path
		url := util.FormatURL(scheme, host, port, path)
		timeout := time.Duration(recoverer.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: rc.transport,
		}

		// Send http request to the recoverers with payload of abnormal.
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, rc)
		if err != nil {
			rc.Error(err, "failed to do http request to recoverer", "recoverer", client.ObjectKey{
				Name:      recoverer.Name,
				Namespace: recoverer.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		// Validate an abnormal after processed by a recoverer.
		err = util.ValidateAbnormalResult(result, abnormal)
		if err != nil {
			rc.Error(err, "invalid result from recoverer", "recoverer", client.ObjectKey{
				Name:      recoverer.Name,
				Namespace: recoverer.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		abnormal.Status = result.Status
		abnormal.Status.Recoverer = &diagnosisv1.NamespacedName{
			Name:      recoverer.Name,
			Namespace: recoverer.Namespace,
		}
		abnormal, err := rc.setAbnormalSucceeded(abnormal)
		if err != nil {
			return abnormal, err
		}

		rc.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "Recovered", "Abnormal recovered by %s/%s", recoverer.Namespace, recoverer.Name)

		return abnormal, nil
	}

	abnormal, err := rc.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	rc.eventRecorder.Eventf(&abnormal, corev1.EventTypeWarning, "FailedRecover", "Unable to recover abnormal %s(%s)", abnormal.Name, abnormal.UID)

	return abnormal, nil
}

// setAbnormalSucceeded sets abnormal phase to Succeeded.
func (rc *recovererChain) setAbnormalSucceeded(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	rc.Info("setting Abnormal phase to succeeded", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalSucceeded
	abnormal.Status.Recoverable = true
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalRecovered,
		Status: corev1.ConditionTrue,
	})
	if err := rc.client.Status().Update(rc, &abnormal); err != nil {
		rc.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (rc *recovererChain) setAbnormalFailed(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	rc.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	abnormal.Status.Recoverable = false
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalRecovered,
		Status: corev1.ConditionFalse,
	})
	if err := rc.client.Status().Update(rc, &abnormal); err != nil {
		rc.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToRecovererChainQueue adds Abnormal to the queue processed by recoverer chain.
func (rc *recovererChain) addAbnormalToRecovererChainQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(rc, rc.recovererChainCh, abnormal)
	if err != nil {
		rc.Error(err, "failed to send abnormal to recoverer chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToRecovererChainQueueWithTimer adds Abnormal to the queue processed by recoverer chain with a timer.
func (rc *recovererChain) addAbnormalToRecovererChainQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(rc, 30*time.Second, rc.recovererChainCh, abnormal)
	if err != nil {
		rc.Error(err, "failed to send abnormal to recoverer chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
