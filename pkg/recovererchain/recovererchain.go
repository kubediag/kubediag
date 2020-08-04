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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// RecovererChain manages recoverer in the system.
type RecovererChain interface {
	Run(<-chan struct{})
	ListRecoverers() ([]diagnosisv1.Recoverer, error)
	SyncAbnormal(diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
	Handler(http.ResponseWriter, *http.Request)
}

// recovererChainImpl implements RecovererChain interface.
type recovererChainImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Client knows how to perform CRUD operations on Kubernetes objects.
	Client client.Client
	// Log represents the ability to log messages.
	Log logr.Logger
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme *runtime.Scheme
	// Cache knows how to load Kubernetes objects.
	Cache cache.Cache
	// NodeName specifies the node name.
	NodeName string

	// Transport for sending http requests to information collectors.
	transport *http.Transport
	// Channel for queuing Abnormals to be processed by recoverer chain.
	recovererChainCh chan diagnosisv1.Abnormal
}

// NewRecovererChain creates a new RecovererChain.
func NewRecovererChain(
	ctx context.Context,
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	recovererChainCh chan diagnosisv1.Abnormal,
) RecovererChain {
	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &recovererChainImpl{
		Context:          ctx,
		Client:           cli,
		Log:              log,
		Scheme:           scheme,
		Cache:            cache,
		NodeName:         nodeName,
		transport:        transport,
		recovererChainCh: recovererChainCh,
	}
}

// Run runs the recoverer chain.
func (rc *recovererChainImpl) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !rc.Cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in recoverer chain channel.
		case abnormal := <-rc.recovererChainCh:
			if util.IsAbnormalNodeNameMatched(abnormal, rc.NodeName) {
				abnormal, err := rc.SyncAbnormal(abnormal)
				if err != nil {
					rc.Log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				rc.Log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
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

// ListRecoverers lists Recoverers from cache.
func (rc *recovererChainImpl) ListRecoverers() ([]diagnosisv1.Recoverer, error) {
	rc.Log.Info("listing Recoverers")

	var recovererList diagnosisv1.RecovererList
	if err := rc.Cache.List(rc.Context, &recovererList); err != nil {
		return nil, err
	}

	return recovererList.Items, nil
}

// SyncAbnormal syncs abnormals.
func (rc *recovererChainImpl) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	rc.Log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	recoverers, err := rc.ListRecoverers()
	if err != nil {
		rc.Log.Error(err, "failed to list Recoverers")
		rc.addAbnormalToRecovererChainQueue(abnormal)
		return abnormal, err
	}

	abnormal, err = rc.runRecovery(recoverers, abnormal)
	if err != nil {
		rc.Log.Error(err, "failed to run recovery")
		rc.addAbnormalToRecovererChainQueue(abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// Handler handles http requests and response with recoverers.
func (rc *recovererChainImpl) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		recoverers, err := rc.ListRecoverers()
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

// runRecovery recovers an abnormal with recoverers.
func (rc *recovererChainImpl) runRecovery(recoverers []diagnosisv1.Recoverer, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Skip recovery if SkipRecovery is true.
	if abnormal.Spec.SkipRecovery {
		rc.Log.Info("skipping recovery", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
		abnormal, err := rc.setAbnormalSucceeded(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	for _, recoverer := range recoverers {
		// Execute only matched recoverers if AssignedRecoverers is not empty.
		matched := false
		if len(abnormal.Spec.AssignedRecoverers) == 0 {
			matched = true
		} else {
			for _, assignedRecoverer := range abnormal.Spec.AssignedRecoverers {
				if recoverer.Name == assignedRecoverer.Name && recoverer.Namespace == assignedRecoverer.Namespace {
					rc.Log.Info("assigned recoverer matched", "recoverer", client.ObjectKey{
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
		}

		if !matched {
			continue
		}

		rc.Log.Info("running recovery", "recoverer", client.ObjectKey{
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
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, rc.Log)
		if err != nil {
			rc.Log.Error(err, "failed to do http request to recoverer", "recoverer", client.ObjectKey{
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
			rc.Log.Error(err, "invalid result from recoverer", "recoverer", client.ObjectKey{
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

		return abnormal, nil
	}

	abnormal, err := rc.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalSucceeded sets abnormal phase to Succeeded.
func (rc *recovererChainImpl) setAbnormalSucceeded(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	rc.Log.Info("setting Abnormal phase to succeeded", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalSucceeded
	abnormal.Status.Recoverable = true
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalRecovered,
		Status: corev1.ConditionTrue,
	})
	if err := rc.Client.Status().Update(rc.Context, &abnormal); err != nil {
		rc.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (rc *recovererChainImpl) setAbnormalFailed(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	rc.Log.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	abnormal.Status.Recoverable = false
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalRecovered,
		Status: corev1.ConditionFalse,
	})
	if err := rc.Client.Status().Update(rc.Context, &abnormal); err != nil {
		rc.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToRecovererChainQueue adds Abnormal to the queue processed by recoverer chain.
func (rc *recovererChainImpl) addAbnormalToRecovererChainQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(rc.Context, rc.recovererChainCh, abnormal)
	if err != nil {
		rc.Log.Error(err, "failed to send abnormal to recoverer chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToRecovererChainQueueWithTimer adds Abnormal to the queue processed by recoverer chain with a timer.
func (rc *recovererChainImpl) addAbnormalToRecovererChainQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(rc.Context, 30*time.Second, rc.recovererChainCh, abnormal)
	if err != nil {
		rc.Log.Error(err, "failed to send abnormal to recoverer chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
