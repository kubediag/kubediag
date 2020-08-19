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
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// diagnoserChain manages diagnosers in the system.
type diagnoserChain struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// Client knows how to perform CRUD operations on Kubernetes objects.
	Client client.Client
	// EventRecorder knows how to record events on behalf of an EventSource.
	EventRecorder record.EventRecorder
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
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	diagnoserChainCh chan diagnosisv1.Abnormal,
	recovererChainCh chan diagnosisv1.Abnormal,
) types.AbnormalManager {
	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &diagnoserChain{
		Context:          ctx,
		Logger:           logger,
		Client:           cli,
		EventRecorder:    eventRecorder,
		Scheme:           scheme,
		Cache:            cache,
		NodeName:         nodeName,
		transport:        transport,
		diagnoserChainCh: diagnoserChainCh,
		recovererChainCh: recovererChainCh,
	}
}

// Run runs the diagnoser chain.
func (dc *diagnoserChain) Run(stopCh <-chan struct{}) {
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
					dc.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				dc.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
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

// SyncAbnormal syncs abnormals.
func (dc *diagnoserChain) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	dc.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	diagnosers, err := dc.listDiagnosers()
	if err != nil {
		dc.Error(err, "failed to list Diagnosers")
		dc.addAbnormalToDiagnoserChainQueue(abnormal)
		return abnormal, err
	}

	abnormal, err = dc.runDiagnosis(diagnosers, abnormal)
	if err != nil {
		dc.Error(err, "failed to run diagnosis")
		dc.addAbnormalToDiagnoserChainQueue(abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// Handler handles http requests and response with diagnosers.
func (dc *diagnoserChain) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		diagnosers, err := dc.listDiagnosers()
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

// listDiagnosers lists Diagnosers from cache.
func (dc *diagnoserChain) listDiagnosers() ([]diagnosisv1.Diagnoser, error) {
	dc.Info("listing Diagnosers")

	var diagnoserList diagnosisv1.DiagnoserList
	if err := dc.Cache.List(dc, &diagnoserList); err != nil {
		return nil, err
	}

	return diagnoserList.Items, nil
}

// runDiagnosis diagnoses an abnormal with diagnosers.
func (dc *diagnoserChain) runDiagnosis(diagnosers []diagnosisv1.Diagnoser, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Skip diagnosis if SkipDiagnosis is true.
	if abnormal.Spec.SkipDiagnosis {
		dc.Info("skipping diagnosis", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		dc.EventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "SkippingDiagnosis", "Skipping diagnosis")

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
					dc.Info("assigned diagnoser matched", "diagnoser", client.ObjectKey{
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

		dc.Info("running diagnosis", "diagnoser", client.ObjectKey{
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
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, dc)
		if err != nil {
			dc.Error(err, "failed to do http request to diagnoser", "diagnoser", client.ObjectKey{
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
			dc.Error(err, "invalid result from diagnoser", "diagnoser", client.ObjectKey{
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

		dc.EventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "Identified", "Abnormal identified by %s/%s", diagnoser.Namespace, diagnoser.Name)

		return abnormal, nil
	}

	abnormal, err := dc.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	dc.EventRecorder.Eventf(&abnormal, corev1.EventTypeWarning, "FailedIdentify", "Unable to identify abnormal %s(%s)", abnormal.Name, abnormal.UID)

	return abnormal, nil
}

// sendAbnormalToRecovererChain sends Abnormal to recoverer chain.
func (dc *diagnoserChain) sendAbnormalToRecovererChain(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	dc.Info("sending Abnormal to recoverer chain", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalRecovering
	abnormal.Status.Identifiable = true
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalIdentified,
		Status: corev1.ConditionTrue,
	})
	if err := dc.Client.Status().Update(dc, &abnormal); err != nil {
		dc.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (dc *diagnoserChain) setAbnormalFailed(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	dc.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	abnormal.Status.Identifiable = false
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalIdentified,
		Status: corev1.ConditionFalse,
	})
	if err := dc.Client.Status().Update(dc, &abnormal); err != nil {
		dc.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToDiagnoserChainQueue adds Abnormal to the queue processed by diagnoser chain.
func (dc *diagnoserChain) addAbnormalToDiagnoserChainQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(dc, dc.diagnoserChainCh, abnormal)
	if err != nil {
		dc.Error(err, "failed to send abnormal to diagnoser chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToDiagnoserChainQueueWithTimer adds Abnormal to the queue processed by diagnoser chain with a timer.
func (dc *diagnoserChain) addAbnormalToDiagnoserChainQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(dc, 30*time.Second, dc.diagnoserChainCh, abnormal)
	if err != nil {
		dc.Error(err, "failed to send abnormal to diagnoser chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
