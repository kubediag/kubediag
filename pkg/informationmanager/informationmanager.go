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

package informationmanager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/types"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

var (
	informationManagerSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_success_count",
			Help: "Counter of successful diagnosis syncs by information manager",
		},
	)
	informationManagerSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_skip_count",
			Help: "Counter of skipped diagnosis syncs by information manager",
		},
	)
	informationManagerSyncFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_fail_count",
			Help: "Counter of failed diagnosis syncs by information manager",
		},
	)
	informationManagerSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_error_count",
			Help: "Counter of erroneous diagnosis syncs by information manager",
		},
	)
	informationManagerCommandExecutorSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_command_executor_success_count",
			Help: "Counter of successful command executor runs by information manager",
		},
	)
	informationManagerCommandExecutorFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_command_executor_fail_count",
			Help: "Counter of failed command executor runs by information manager",
		},
	)
	informationManagerProfilerSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_profiler_success_count",
			Help: "Counter of successful profiler runs by information manager",
		},
	)
	informationManagerProfilerFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_profiler_fail_count",
			Help: "Counter of failed profiler runs by information manager",
		},
	)
)

// informationManager manages information collectors in the system.
type informationManager struct {
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
	// transport is the transport for sending http requests to information collectors.
	transport *http.Transport
	// bindAddress is the address on which to advertise.
	bindAddress string
	// port is the port for the kube diagnoser to serve on.
	port int
	// dataRoot is root directory of persistent kube diagnoser data.
	dataRoot string
	// informationManagerCh is a channel for queuing Diagnoses to be processed by information manager.
	informationManagerCh chan diagnosisv1.Diagnosis
}

// NewInformationManager creates a new informationManager.
func NewInformationManager(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	bindAddress string,
	port int,
	dataRoot string,
	informationManagerCh chan diagnosisv1.Diagnosis,
) types.DiagnosisManager {
	metrics.Registry.MustRegister(
		informationManagerSyncSuccessCount,
		informationManagerSyncSkipCount,
		informationManagerSyncFailCount,
		informationManagerSyncErrorCount,
		informationManagerCommandExecutorSuccessCount,
		informationManagerCommandExecutorFailCount,
		informationManagerProfilerSuccessCount,
		informationManagerProfilerFailCount,
	)

	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &informationManager{
		Context:              ctx,
		Logger:               logger,
		client:               cli,
		eventRecorder:        eventRecorder,
		scheme:               scheme,
		cache:                cache,
		nodeName:             nodeName,
		transport:            transport,
		bindAddress:          bindAddress,
		port:                 port,
		dataRoot:             dataRoot,
		informationManagerCh: informationManagerCh,
	}
}

// Run runs the information manager.
func (im *informationManager) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !im.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process diagnoses queuing in information manager channel.
		case diagnosis := <-im.informationManagerCh:
			err := im.client.Get(im, client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			}, &diagnosis)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}

				err := util.QueueDiagnosis(im, im.informationManagerCh, diagnosis)
				if err != nil {
					im.Error(err, "failed to send diagnosis to information manager queue", "diagnosis", client.ObjectKey{
						Name:      diagnosis.Name,
						Namespace: diagnosis.Namespace,
					})
				}
				continue
			}

			// Only process diagnosis in InformationCollecting phase.
			if diagnosis.Status.Phase != diagnosisv1.InformationCollecting {
				continue
			}

			if util.IsDiagnosisNodeNameMatched(diagnosis, im.nodeName) {
				diagnosis, err := im.SyncDiagnosis(diagnosis)
				if err != nil {
					im.Error(err, "failed to sync Diagnosis", "diagnosis", diagnosis)
				}

				im.Info("syncing Diagnosis successfully", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			}
		// Stop information manager on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncDiagnosis syncs diagnoses.
func (im *informationManager) SyncDiagnosis(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	im.Info("starting to sync Diagnosis", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	_, condition := util.GetDiagnosisCondition(&diagnosis.Status, diagnosisv1.InformationCollected)
	if condition != nil {
		im.Info("ignoring Diagnosis in phase InformationCollecting with condition InformationCollected", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	} else {
		informationCollectors, err := im.listInformationCollectors()
		if err != nil {
			im.Error(err, "failed to list InformationCollectors")
			im.addDiagnosisToInformationManagerQueue(diagnosis)
			return diagnosis, err
		}

		diagnosis, err := im.runInformationCollection(informationCollectors, diagnosis)
		if err != nil {
			im.Error(err, "failed to run collection")
			im.addDiagnosisToInformationManagerQueue(diagnosis)
			return diagnosis, err
		}
	}

	// Increment counter of successful diagnosis syncs by information manager.
	informationManagerSyncSuccessCount.Inc()

	return diagnosis, nil
}

// Handler handles http requests and response with information collectors.
func (im *informationManager) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		informationCollectors, err := im.listInformationCollectors()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list information collectors: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(informationCollectors)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal information collectors: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// listInformationCollectors lists InformationCollectors from cache.
func (im *informationManager) listInformationCollectors() ([]diagnosisv1.InformationCollector, error) {
	im.Info("listing InformationCollectors")

	var informationCollectorList diagnosisv1.InformationCollectorList
	if err := im.cache.List(im, &informationCollectorList); err != nil {
		return nil, err
	}

	return informationCollectorList.Items, nil
}

// runInformationCollection collects information from information collectors.
func (im *informationManager) runInformationCollection(informationCollectors []diagnosisv1.InformationCollector, diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	// Run command executor of InformationCollector type.
	for _, executorSpec := range diagnosis.Spec.CommandExecutors {
		if executorSpec.Type == diagnosisv1.InformationCollectorType {
			executorStatus, err := util.RunCommandExecutor(executorSpec, im)
			if err != nil {
				informationManagerCommandExecutorFailCount.Inc()
				im.Error(err, "failed to run command executor", "command", executorSpec.Command, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			} else {
				informationManagerCommandExecutorSuccessCount.Inc()
			}

			diagnosis.Status.CommandExecutors = append(diagnosis.Status.CommandExecutors, executorStatus)
		}
	}

	// Run profiler of InformationCollector type.
	for _, profilerSpec := range diagnosis.Spec.Profilers {
		if profilerSpec.Type == diagnosisv1.InformationCollectorType {
			profilerStatus, err := util.RunProfiler(im, diagnosis.Name, diagnosis.Namespace, im.bindAddress, im.dataRoot, profilerSpec, diagnosis.Spec.PodReference, im.client, im)
			if err != nil {
				informationManagerProfilerFailCount.Inc()
				im.Error(err, "failed to run profiler", "profiler", profilerSpec, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			} else {
				informationManagerProfilerSuccessCount.Inc()
			}

			diagnosis.Status.Profilers = append(diagnosis.Status.Profilers, profilerStatus)
		}
	}

	// Skip collection if AssignedInformationCollectors is empty.
	if len(diagnosis.Spec.AssignedInformationCollectors) == 0 {
		informationManagerSyncSkipCount.Inc()
		im.Info("skipping collection", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
		im.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "SkippingCollection", "Skipping collection")

		diagnosis, err := im.sendDiagnosisToDiagnoserChain(diagnosis)
		if err != nil {
			return diagnosis, err
		}

		return diagnosis, nil
	}

	informationCollected := false
	for _, collector := range informationCollectors {
		// Execute only matched information collectors.
		matched := false
		for _, assignedCollector := range diagnosis.Spec.AssignedInformationCollectors {
			if collector.Name == assignedCollector.Name && collector.Namespace == assignedCollector.Namespace {
				im.Info("assigned collector matched", "collector", client.ObjectKey{
					Name:      collector.Name,
					Namespace: collector.Namespace,
				}, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		im.Info("running collection", "collector", client.ObjectKey{
			Name:      collector.Name,
			Namespace: collector.Namespace,
		}, "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		var host string
		var port int32
		if collector.Spec.ExternalIP != nil {
			host = *collector.Spec.ExternalIP
		} else {
			host = im.bindAddress
		}
		if collector.Spec.ExternalPort != nil {
			port = *collector.Spec.ExternalPort
		} else {
			port = int32(im.port)
		}
		path := collector.Spec.Path
		scheme := strings.ToLower(string(collector.Spec.Scheme))
		url := util.FormatURL(scheme, host, strconv.Itoa(int(port)), path)
		timeout := time.Duration(collector.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: im.transport,
		}

		// Send http request to the information collector with payload of diagnosis.
		result, err := util.DoHTTPRequestWithDiagnosis(diagnosis, url, *cli, im)
		if err != nil {
			im.Error(err, "failed to do http request to collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			continue
		}

		// Validate an diagnosis after processed by an information collector.
		err = util.ValidateDiagnosisResult(result, diagnosis)
		if err != nil {
			im.Error(err, "invalid result from collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			continue
		}

		informationCollected = true
		diagnosis.Status = result.Status

		im.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "InformationCollected", "Information collected by %s/%s", collector.Namespace, collector.Name)
	}

	// All assigned information collectors will be executed. The Diagnosis will be sent to diagnoser chain
	// if any information is collected successfully.
	if informationCollected {
		diagnosis, err := im.sendDiagnosisToDiagnoserChain(diagnosis)
		if err != nil {
			return diagnosis, err
		}

		return diagnosis, nil
	}

	diagnosis, err := im.setDiagnosisFailed(diagnosis)
	if err != nil {
		return diagnosis, err
	}

	im.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "FailedCollect", "Unable to collect information for diagnosis %s(%s)", diagnosis.Name, diagnosis.UID)

	return diagnosis, nil
}

// sendDiagnosisToDiagnoserChain sends Diagnosis to diagnoser chain.
func (im *informationManager) sendDiagnosisToDiagnoserChain(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	im.Info("sending Diagnosis to diagnoser chain", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.Phase = diagnosisv1.DiagnosisDiagnosing
	util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionTrue,
	})
	if err := im.client.Status().Update(im, &diagnosis); err != nil {
		im.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	return diagnosis, nil
}

// setDiagnosisFailed sets diagnosis phase to Failed.
func (im *informationManager) setDiagnosisFailed(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	im.Info("setting Diagnosis phase to failed", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
	util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionFalse,
	})
	if err := im.client.Status().Update(im, &diagnosis); err != nil {
		im.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	informationManagerSyncFailCount.Inc()

	return diagnosis, nil
}

// addDiagnosisToInformationManagerQueue adds Diagnosis to the queue processed by information manager.
func (im *informationManager) addDiagnosisToInformationManagerQueue(diagnosis diagnosisv1.Diagnosis) {
	informationManagerSyncErrorCount.Inc()

	err := util.QueueDiagnosis(im, im.informationManagerCh, diagnosis)
	if err != nil {
		im.Error(err, "failed to send diagnosis to information manager queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}

// addDiagnosisToInformationManagerQueueWithTimer adds Diagnosis to the queue processed by information manager with a timer.
func (im *informationManager) addDiagnosisToInformationManagerQueueWithTimer(diagnosis diagnosisv1.Diagnosis) {
	informationManagerSyncErrorCount.Inc()

	err := util.QueueDiagnosisWithTimer(im, 30*time.Second, im.informationManagerCh, diagnosis)
	if err != nil {
		im.Error(err, "failed to send diagnosis to information manager queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}
