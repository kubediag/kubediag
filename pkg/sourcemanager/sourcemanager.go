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

package sourcemanager

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// sourceManager manages abnormal sources in the system.
type sourceManager struct {
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
	// sourceManagerCh is a channel for queuing Abnormals to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Abnormal
}

// NewSourceManager creates a new sourceManager.
func NewSourceManager(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	sourceManagerCh chan diagnosisv1.Abnormal,
) types.AbnormalManager {
	return &sourceManager{
		Context:         ctx,
		Logger:          logger,
		client:          cli,
		eventRecorder:   eventRecorder,
		scheme:          scheme,
		cache:           cache,
		nodeName:        nodeName,
		sourceManagerCh: sourceManagerCh,
	}
}

// Run runs the source manager.
func (sm *sourceManager) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !sm.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in source manager channel.
		case abnormal := <-sm.sourceManagerCh:
			abnormal, err := sm.SyncAbnormal(abnormal)
			if err != nil {
				sm.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
			}

			if abnormal.Generation != 0 {
				sm.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}
		// Stop source manager on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncAbnormal syncs abnormals.
func (sm *sourceManager) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Create an abnormal from specified source if it is nonexistent.
	if abnormal.Generation == 0 {
		abnormalSources, err := sm.listAbnormalSources()
		if err != nil {
			sm.Error(err, "failed to list AbnormalSources")
			sm.addAbnormalToSourceManagerQueue(abnormal)
			return abnormal, err
		}

		if abnormal.Spec.Source == diagnosisv1.PrometheusAlertSource && abnormal.Spec.PrometheusAlert != nil {
			abnormal, err := sm.createAbnormalFromPrometheusAlert(abnormalSources, abnormal)
			if err != nil {
				sm.addAbnormalToSourceManagerQueue(abnormal)
				return abnormal, err
			}
		} else if abnormal.Spec.Source == diagnosisv1.KubernetesEventSource && abnormal.Spec.KubernetesEvent != nil {
			abnormal, err := sm.createAbnormalFromKubernetesEvent(abnormalSources, abnormal)
			if err != nil {
				sm.addAbnormalToSourceManagerQueue(abnormal)
				return abnormal, err
			}
		}
	} else {
		sm.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		sm.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "Accepted", "Accepted abnormal")

		abnormal, err := sm.sendAbnormalToInformationManager(abnormal)
		if err != nil {
			sm.addAbnormalToSourceManagerQueue(abnormal)
			return abnormal, err
		}
	}

	return abnormal, nil
}

// Handler handles http requests.
func (sm *sourceManager) Handler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
}

// listAbnormalSources lists AbnormalSources from cache.
func (sm *sourceManager) listAbnormalSources() ([]diagnosisv1.AbnormalSource, error) {
	var abnormalSourcesList diagnosisv1.AbnormalSourceList
	if err := sm.cache.List(sm, &abnormalSourcesList); err != nil {
		return nil, err
	}

	return abnormalSourcesList.Items, nil
}

// createAbnormalFromPrometheusAlert creates an Abnormal from prometheus alert and abnormal sources.
func (sm *sourceManager) createAbnormalFromPrometheusAlert(abnormalSources []diagnosisv1.AbnormalSource, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	for _, abnormalSource := range abnormalSources {
		sourceTemplate := abnormalSource.Spec.SourceTemplate
		if sourceTemplate.Type == diagnosisv1.PrometheusAlertSource && sourceTemplate.PrometheusAlertTemplate != nil {
			// Set all fields of the abnormal according to abnormal source if the prometheus alert contains
			// all match of the regular expression pattern defined in prometheus alert template.
			matched, err := util.MatchPrometheusAlert(*sourceTemplate.PrometheusAlertTemplate, abnormal)
			if err != nil {
				sm.Error(err, "failed to compare abnormal source template and prometheus alert")
				continue
			}

			if matched {
				sm.Info("creating Abnormal from prometheus alert", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})

				abnormal.Spec.NodeName = string(abnormal.Spec.PrometheusAlert.Labels[sourceTemplate.PrometheusAlertTemplate.NodeNameReferenceLabel])
				abnormal.Spec.AssignedInformationCollectors = abnormalSource.Spec.AssignedInformationCollectors
				abnormal.Spec.AssignedDiagnosers = abnormalSource.Spec.AssignedDiagnosers
				abnormal.Spec.AssignedRecoverers = abnormalSource.Spec.AssignedRecoverers
				abnormal.Spec.CommandExecutors = abnormalSource.Spec.CommandExecutors
				abnormal.Spec.Profilers = abnormalSource.Spec.Profilers
				abnormal.Spec.Context = abnormalSource.Spec.Context

				if err := sm.client.Create(sm, &abnormal); err != nil {
					if !apierrors.IsAlreadyExists(err) {
						sm.Error(err, "unable to create Abnormal")
						return abnormal, err
					}
				}

				return abnormal, nil
			}
		}
	}

	return abnormal, nil
}

// createAbnormalFromKubernetesEvent creates an Abnormal from kubernetes event and abnormal sources.
func (sm *sourceManager) createAbnormalFromKubernetesEvent(abnormalSources []diagnosisv1.AbnormalSource, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	for _, abnormalSource := range abnormalSources {
		sourceTemplate := abnormalSource.Spec.SourceTemplate
		if sourceTemplate.Type == diagnosisv1.KubernetesEventSource && sourceTemplate.KubernetesEventTemplate != nil {
			// Set all fields of the abnormal according to abnormal source if the kubernetes event contains
			// all match of the regular expression pattern defined in kubernetes event template.
			matched, err := util.MatchKubernetesEvent(*sourceTemplate.KubernetesEventTemplate, abnormal)
			if err != nil {
				sm.Error(err, "failed to compare abnormal source template and kubernetes event")
				continue
			}

			if matched {
				sm.Info("creating Abnormal from kubernetes event", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})

				// Set EventTime of the event for compatibility since validation failure will be encountered
				// if EventTime is nil.
				if abnormal.Spec.KubernetesEvent.EventTime.IsZero() {
					abnormal.Spec.KubernetesEvent.EventTime = metav1.NewMicroTime(time.Unix(0, 0))
				}

				abnormal.Spec.NodeName = abnormal.Spec.KubernetesEvent.Source.Host
				abnormal.Spec.AssignedInformationCollectors = abnormalSource.Spec.AssignedInformationCollectors
				abnormal.Spec.AssignedDiagnosers = abnormalSource.Spec.AssignedDiagnosers
				abnormal.Spec.AssignedRecoverers = abnormalSource.Spec.AssignedRecoverers
				abnormal.Spec.CommandExecutors = abnormalSource.Spec.CommandExecutors
				abnormal.Spec.Profilers = abnormalSource.Spec.Profilers
				abnormal.Spec.Context = abnormalSource.Spec.Context

				if err := sm.client.Create(sm, &abnormal); err != nil {
					if !apierrors.IsAlreadyExists(err) {
						sm.Error(err, "unable to create Abnormal")
						return abnormal, err
					}
				}

				return abnormal, nil
			}
		}
	}

	return abnormal, nil
}

// sendAbnormalToInformationManager sends Abnormal to information manager.
func (sm *sourceManager) sendAbnormalToInformationManager(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	sm.Info("sending Abnormal to information manager", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.StartTime = metav1.Now()
	abnormal.Status.Phase = diagnosisv1.InformationCollecting
	if err := sm.client.Status().Update(sm, &abnormal); err != nil {
		sm.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToSourceManagerQueue adds Abnormal to the queue processed by source manager.
func (sm *sourceManager) addAbnormalToSourceManagerQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(sm, sm.sourceManagerCh, abnormal)
	if err != nil {
		sm.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToSourceManagerQueueWithTimer adds Abnormal to the queue processed by source manager with a timer.
func (sm *sourceManager) addAbnormalToSourceManagerQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(sm, 30*time.Second, sm.sourceManagerCh, abnormal)
	if err != nil {
		sm.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
