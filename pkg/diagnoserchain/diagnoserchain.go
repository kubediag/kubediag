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

package diagnoserchain

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

// DiagnoserChain manages diagnoser in the system.
type DiagnoserChain interface {
	Run(<-chan struct{})
	ListDiagnosers() ([]diagnosisv1.Diagnoser, error)
	SyncAbnormal(diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
	Handler(http.ResponseWriter, *http.Request)
}

// diagnoserChainImpl implements DiagnoserChain interface.
type diagnoserChainImpl struct {
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
	// Channel for queuing Abnormals to be processed by diagnoser chain.
	diagnoserChainCh chan diagnosisv1.Abnormal
	// Channel for queuing Abnormals to be processed by recoverer chain.
	recovererChainCh chan diagnosisv1.Abnormal
}

// NewDiagnoserChain creates a new DiagnoserChain.
func NewDiagnoserChain(
	ctx context.Context,
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	diagnoserChainCh chan diagnosisv1.Abnormal,
	recovererChainCh chan diagnosisv1.Abnormal,
) DiagnoserChain {
	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &diagnoserChainImpl{
		Context:          ctx,
		Client:           cli,
		Log:              log,
		Scheme:           scheme,
		Cache:            cache,
		NodeName:         nodeName,
		transport:        transport,
		diagnoserChainCh: diagnoserChainCh,
		recovererChainCh: recovererChainCh,
	}
}

// Run runs the diagnoser chain.
func (dc *diagnoserChainImpl) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !dc.Cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in diagnoser chain channel.
		case abnormal := <-dc.diagnoserChainCh:
			if util.IsAbnormalNodeNameMatched(abnormal, dc.NodeName) {
				abnormal, err := dc.SyncAbnormal(abnormal)
				if err != nil {
					dc.Log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				dc.Log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}
		// Stop diagnoser chain on stop signal.
		case <-stopCh:
			return
		}
	}
}

// ListDiagnosers lists Diagnosers from cache.
func (dc *diagnoserChainImpl) ListDiagnosers() ([]diagnosisv1.Diagnoser, error) {
	dc.Log.Info("listing Diagnosers")

	var diagnoserList diagnosisv1.DiagnoserList
	if err := dc.Cache.List(dc.Context, &diagnoserList); err != nil {
		return nil, err
	}

	return diagnoserList.Items, nil
}

// SyncAbnormal syncs abnormals.
func (dc *diagnoserChainImpl) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	dc.Log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	diagnosers, err := dc.ListDiagnosers()
	if err != nil {
		dc.Log.Error(err, "failed to list Diagnosers")
		dc.addAbnormalToDiagnoserChainQueue(abnormal)
		return abnormal, err
	}

	abnormal, err = dc.runDiagnosis(diagnosers, abnormal)
	if err != nil {
		dc.Log.Error(err, "failed to run diagnosis")
		dc.addAbnormalToDiagnoserChainQueue(abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// Handler handles http requests and response with diagnosers.
func (dc *diagnoserChainImpl) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		diagnosers, err := dc.ListDiagnosers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list diagnosers: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(diagnosers)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal diagnosers: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// runDiagnosis diagnoses an abnormal with diagnosers.
func (dc *diagnoserChainImpl) runDiagnosis(diagnosers []diagnosisv1.Diagnoser, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Skip diagnosis if SkipDiagnosis is true.
	if abnormal.Spec.SkipDiagnosis {
		dc.Log.Info("skipping diagnosis", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
		abnormal, err := dc.sendAbnormalToRecovererChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	for _, diagnoser := range diagnosers {
		// Execute only matched diagnosers if AssignedDiagnosers is not empty.
		matched := false
		if len(abnormal.Spec.AssignedDiagnosers) == 0 {
			matched = true
		} else {
			for _, assignedDiagnoser := range abnormal.Spec.AssignedDiagnosers {
				if diagnoser.Name == assignedDiagnoser.Name && diagnoser.Namespace == assignedDiagnoser.Namespace {
					dc.Log.Info("assigned diagnoser matched", "diagnoser", client.ObjectKey{
						Name:      diagnoser.Name,
						Namespace: diagnoser.Namespace,
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

		dc.Log.Info("running diagnosis", "diagnoser", client.ObjectKey{
			Name:      diagnoser.Name,
			Namespace: diagnoser.Namespace,
		}, "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		scheme := strings.ToLower(string(diagnoser.Spec.Scheme))
		host := diagnoser.Spec.IP
		port := diagnoser.Spec.Port
		path := diagnoser.Spec.Path
		url := util.FormatURL(scheme, host, port, path)
		timeout := time.Duration(diagnoser.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: dc.transport,
		}

		// Send http request to the diagnosers with payload of abnormal.
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, dc.Log)
		if err != nil {
			dc.Log.Error(err, "failed to do http request to diagnoser", "diagnoser", client.ObjectKey{
				Name:      diagnoser.Name,
				Namespace: diagnoser.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		// Validate an abnormal after processed by a diagnoser.
		err = util.ValidateAbnormalResult(result, abnormal)
		if err != nil {
			dc.Log.Error(err, "invalid result from diagnoser", "diagnoser", client.ObjectKey{
				Name:      diagnoser.Name,
				Namespace: diagnoser.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		abnormal.Status = result.Status
		abnormal.Status.Diagnoser = &diagnosisv1.NamespacedName{
			Name:      diagnoser.Name,
			Namespace: diagnoser.Namespace,
		}
		abnormal, err := dc.sendAbnormalToRecovererChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	abnormal, err := dc.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	return abnormal, nil
}

// sendAbnormalToRecovererChain sends Abnormal to recoverer chain.
func (dc *diagnoserChainImpl) sendAbnormalToRecovererChain(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	dc.Log.Info("sending Abnormal to recoverer chain", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalRecovering
	abnormal.Status.Identifiable = true
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalIdentified,
		Status: corev1.ConditionTrue,
	})
	if err := dc.Client.Status().Update(dc.Context, &abnormal); err != nil {
		dc.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (dc *diagnoserChainImpl) setAbnormalFailed(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	dc.Log.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	abnormal.Status.Identifiable = false
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalIdentified,
		Status: corev1.ConditionFalse,
	})
	if err := dc.Client.Status().Update(dc.Context, &abnormal); err != nil {
		dc.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToDiagnoserChainQueue adds Abnormal to the queue processed by diagnoser chain.
func (dc *diagnoserChainImpl) addAbnormalToDiagnoserChainQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(dc.Context, dc.diagnoserChainCh, abnormal)
	if err != nil {
		dc.Log.Error(err, "failed to send abnormal to diagnoser chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToDiagnoserChainQueueWithTimer adds Abnormal to the queue processed by diagnoser chain with a timer.
func (dc *diagnoserChainImpl) addAbnormalToDiagnoserChainQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(dc.Context, 30*time.Second, dc.diagnoserChainCh, abnormal)
	if err != nil {
		dc.Log.Error(err, "failed to send abnormal to diagnoser chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
