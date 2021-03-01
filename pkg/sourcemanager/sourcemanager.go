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
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/types"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

var (
	sourceManagerSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "source_manager_sync_success_count",
			Help: "Counter of successful diagnosis syncs by source manager",
		},
	)
	sourceManagerSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "source_manager_sync_error_count",
			Help: "Counter of erroneous diagnosis syncs by source manager",
		},
	)
	prometheusAlertGeneratedDiagnosisCreationCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prometheus_alert_generated_diagnosis_creation_count",
			Help: "Counter of prometheus alert generated diagnosis creations by source manager",
		},
	)
	eventGeneratedDiagnosisCreationCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "event_generated_diagnosis_creation_count",
			Help: "Counter of event generated diagnosis creations by source manager",
		},
	)
)

// sourceManager manages diagnosis sources in the system.
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
	// sourceManagerCh is a channel for queuing Diagnoses to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Diagnosis
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
	sourceManagerCh chan diagnosisv1.Diagnosis,
) types.DiagnosisManager {
	metrics.Registry.MustRegister(
		sourceManagerSyncSuccessCount,
		sourceManagerSyncErrorCount,
		prometheusAlertGeneratedDiagnosisCreationCount,
		eventGeneratedDiagnosisCreationCount,
	)

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
		// Process diagnoses queuing in source manager channel.
		case diagnosis := <-sm.sourceManagerCh:
			if diagnosis.Generation != 0 {
				err := sm.client.Get(sm, client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				}, &diagnosis)
				if err != nil {
					if apierrors.IsNotFound(err) {
						continue
					}

					err := util.QueueDiagnosis(sm, sm.sourceManagerCh, diagnosis)
					if err != nil {
						sm.Error(err, "failed to send diagnosis to source manager queue", "diagnosis", client.ObjectKey{
							Name:      diagnosis.Name,
							Namespace: diagnosis.Namespace,
						})
					}
					continue
				}

				// Only process diagnosis which has not been accept yet.
				if diagnosis.Status.Phase != "" {
					continue
				}
			}

			diagnosis, err := sm.SyncDiagnosis(diagnosis)
			if err != nil {
				sm.Error(err, "failed to sync Diagnosis", "diagnosis", diagnosis)
			}

			sm.Info("syncing Diagnosis successfully", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
		// Stop source manager on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncDiagnosis syncs diagnoses.
func (sm *sourceManager) SyncDiagnosis(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	// Create an diagnosis from specified source if it is nonexistent.
	if diagnosis.Generation == 0 {
		diagnosisSources, err := sm.listDiagnosisSources()
		if err != nil {
			sm.Error(err, "failed to list DiagnosisSources")
			sm.addDiagnosisToSourceManagerQueue(diagnosis)
			return diagnosis, err
		}

		if diagnosis.Spec.Source == diagnosisv1.PrometheusAlertSource && diagnosis.Spec.PrometheusAlert != nil {
			diagnosis, err := sm.createDiagnosisFromPrometheusAlert(diagnosisSources, diagnosis)
			if err != nil {
				sm.addDiagnosisToSourceManagerQueue(diagnosis)
				return diagnosis, err
			}
		} else if diagnosis.Spec.Source == diagnosisv1.KubernetesEventSource && diagnosis.Spec.KubernetesEvent != nil {
			diagnosis, err := sm.createDiagnosisFromKubernetesEvent(diagnosisSources, diagnosis)
			if err != nil {
				sm.addDiagnosisToSourceManagerQueue(diagnosis)
				return diagnosis, err
			}
		}
	} else {
		sm.Info("starting to sync Diagnosis", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		sm.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "Accepted", "Accepted diagnosis")

		diagnosis, err := sm.sendDiagnosisToInformationManager(diagnosis)
		if err != nil {
			sm.addDiagnosisToSourceManagerQueue(diagnosis)
			return diagnosis, err
		}
	}

	// Increment counter of successful diagnosis syncs by source manager.
	sourceManagerSyncSuccessCount.Inc()

	return diagnosis, nil
}

// Handler handles http requests.
func (sm *sourceManager) Handler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
}

// listDiagnosisSources lists DiagnosisSources from cache.
func (sm *sourceManager) listDiagnosisSources() ([]diagnosisv1.DiagnosisSource, error) {
	var diagnosisSourcesList diagnosisv1.DiagnosisSourceList
	if err := sm.cache.List(sm, &diagnosisSourcesList); err != nil {
		return nil, err
	}

	return diagnosisSourcesList.Items, nil
}

// createDiagnosisFromPrometheusAlert creates an Diagnosis from prometheus alert and diagnosis sources.
func (sm *sourceManager) createDiagnosisFromPrometheusAlert(diagnosisSources []diagnosisv1.DiagnosisSource, diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	for _, diagnosisSource := range diagnosisSources {
		sourceTemplate := diagnosisSource.Spec.SourceTemplate
		if sourceTemplate.PrometheusAlertTemplate != nil {
			// Set all fields of the diagnosis according to diagnosis source if the prometheus alert contains
			// all match of the regular expression pattern defined in prometheus alert template.
			matched, err := util.MatchPrometheusAlert(*sourceTemplate.PrometheusAlertTemplate, diagnosis)
			if err != nil {
				sm.Error(err, "failed to compare diagnosis source template and prometheus alert")
				continue
			}

			if matched {
				sm.Info("creating Diagnosis from prometheus alert", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})

				diagnosis.Spec.NodeName = string(diagnosis.Spec.PrometheusAlert.Labels[sourceTemplate.PrometheusAlertTemplate.NodeNameReferenceLabel])
				diagnosis.Spec.AssignedInformationCollectors = diagnosisSource.Spec.AssignedInformationCollectors
				diagnosis.Spec.AssignedDiagnosers = diagnosisSource.Spec.AssignedDiagnosers
				diagnosis.Spec.AssignedRecoverers = diagnosisSource.Spec.AssignedRecoverers
				diagnosis.Spec.CommandExecutors = diagnosisSource.Spec.CommandExecutors
				diagnosis.Spec.Profilers = diagnosisSource.Spec.Profilers
				diagnosis.Spec.Context = diagnosisSource.Spec.Context

				if err := sm.client.Create(sm, &diagnosis); err != nil {
					if !apierrors.IsAlreadyExists(err) {
						sm.Error(err, "unable to create Diagnosis")
						return diagnosis, err
					}
				} else {
					// Increment counter of prometheus alert generated diagnosis creations by source manager.
					prometheusAlertGeneratedDiagnosisCreationCount.Inc()
				}

				return diagnosis, nil
			}
		}
	}

	return diagnosis, nil
}

// createDiagnosisFromKubernetesEvent creates an Diagnosis from kubernetes event and diagnosis sources.
func (sm *sourceManager) createDiagnosisFromKubernetesEvent(diagnosisSources []diagnosisv1.DiagnosisSource, diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	for _, diagnosisSource := range diagnosisSources {
		sourceTemplate := diagnosisSource.Spec.SourceTemplate
		if sourceTemplate.KubernetesEventTemplate != nil {
			// Set all fields of the diagnosis according to diagnosis source if the kubernetes event contains
			// all match of the regular expression pattern defined in kubernetes event template.
			matched, err := util.MatchKubernetesEvent(*sourceTemplate.KubernetesEventTemplate, diagnosis)
			if err != nil {
				sm.Error(err, "failed to compare diagnosis source template and kubernetes event")
				continue
			}

			if matched {
				sm.Info("creating Diagnosis from kubernetes event", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})

				// Set EventTime of the event for compatibility since validation failure will be encountered
				// if EventTime is nil.
				if diagnosis.Spec.KubernetesEvent.EventTime.IsZero() {
					diagnosis.Spec.KubernetesEvent.EventTime = metav1.NewMicroTime(time.Unix(0, 0))
				}

				diagnosis.Spec.NodeName = diagnosis.Spec.KubernetesEvent.Source.Host
				diagnosis.Spec.AssignedInformationCollectors = diagnosisSource.Spec.AssignedInformationCollectors
				diagnosis.Spec.AssignedDiagnosers = diagnosisSource.Spec.AssignedDiagnosers
				diagnosis.Spec.AssignedRecoverers = diagnosisSource.Spec.AssignedRecoverers
				diagnosis.Spec.CommandExecutors = diagnosisSource.Spec.CommandExecutors
				diagnosis.Spec.Profilers = diagnosisSource.Spec.Profilers
				diagnosis.Spec.Context = diagnosisSource.Spec.Context

				if err := sm.client.Create(sm, &diagnosis); err != nil {
					if !apierrors.IsAlreadyExists(err) {
						sm.Error(err, "unable to create Diagnosis")
						return diagnosis, err
					}
				} else {
					// Increment counter of event generated diagnosis creations by source manager.
					eventGeneratedDiagnosisCreationCount.Inc()
				}

				return diagnosis, nil
			}
		}
	}

	return diagnosis, nil
}

// sendDiagnosisToInformationManager sends Diagnosis to information manager.
func (sm *sourceManager) sendDiagnosisToInformationManager(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	sm.Info("sending Diagnosis to information manager", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.StartTime = metav1.Now()
	diagnosis.Status.Phase = diagnosisv1.InformationCollecting
	if err := sm.client.Status().Update(sm, &diagnosis); err != nil {
		sm.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	return diagnosis, nil
}

// addDiagnosisToSourceManagerQueue adds Diagnosis to the queue processed by source manager.
func (sm *sourceManager) addDiagnosisToSourceManagerQueue(diagnosis diagnosisv1.Diagnosis) {
	sourceManagerSyncErrorCount.Inc()

	err := util.QueueDiagnosis(sm, sm.sourceManagerCh, diagnosis)
	if err != nil {
		sm.Error(err, "failed to send diagnosis to source manager queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}

// addDiagnosisToSourceManagerQueueWithTimer adds Diagnosis to the queue processed by source manager with a timer.
func (sm *sourceManager) addDiagnosisToSourceManagerQueueWithTimer(diagnosis diagnosisv1.Diagnosis) {
	sourceManagerSyncErrorCount.Inc()

	err := util.QueueDiagnosisWithTimer(sm, 30*time.Second, sm.sourceManagerCh, diagnosis)
	if err != nil {
		sm.Error(err, "failed to send diagnosis to source manager queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}
