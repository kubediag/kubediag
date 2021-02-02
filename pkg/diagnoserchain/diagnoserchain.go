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

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

var (
	diagnoserChainSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_success_count",
			Help: "Counter of successful diagnosis syncs by diagnoser chain",
		},
	)
	diagnoserChainSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_skip_count",
			Help: "Counter of skipped diagnosis syncs by diagnoser chain",
		},
	)
	diagnoserChainSyncFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_fail_count",
			Help: "Counter of failed diagnosis syncs by diagnoser chain",
		},
	)
	diagnoserChainSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_error_count",
			Help: "Counter of erroneous diagnosis syncs by diagnoser chain",
		},
	)
	diagnoserChainCommandExecutorSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_command_executor_success_count",
			Help: "Counter of successful command executor runs by diagnoser chain",
		},
	)
	diagnoserChainCommandExecutorFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_command_executor_fail_count",
			Help: "Counter of failed command executor runs by diagnoser chain",
		},
	)
	diagnoserChainProfilerSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_profiler_success_count",
			Help: "Counter of successful profiler runs by diagnoser chain",
		},
	)
	diagnoserChainProfilerFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_profiler_fail_count",
			Help: "Counter of failed profiler runs by diagnoser chain",
		},
	)
)

// diagnoserChain manages diagnosers in the system.
type diagnoserChain struct {
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
	// transport is the transport for sending http requests to diagnosers.
	transport *http.Transport
	// bindAddress is the address on which to advertise.
	bindAddress string
	// port is the port for the kube diagnoser to serve on.
	port int
	// dataRoot is root directory of persistent kube diagnoser data.
	dataRoot string
	// diagnoserChainCh is a channel for queuing Diagnoses to be processed by diagnoser chain.
	diagnoserChainCh chan diagnosisv1.Diagnosis
}

// NewDiagnoserChain creates a new diagnoserChain.
func NewDiagnoserChain(
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
	diagnoserChainCh chan diagnosisv1.Diagnosis,
) types.DiagnosisManager {
	metrics.Registry.MustRegister(
		diagnoserChainSyncSuccessCount,
		diagnoserChainSyncSkipCount,
		diagnoserChainSyncFailCount,
		diagnoserChainSyncErrorCount,
		diagnoserChainCommandExecutorSuccessCount,
		diagnoserChainCommandExecutorFailCount,
		diagnoserChainProfilerSuccessCount,
		diagnoserChainProfilerFailCount,
	)

	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &diagnoserChain{
		Context:          ctx,
		Logger:           logger,
		client:           cli,
		eventRecorder:    eventRecorder,
		scheme:           scheme,
		cache:            cache,
		nodeName:         nodeName,
		transport:        transport,
		bindAddress:      bindAddress,
		port:             port,
		dataRoot:         dataRoot,
		diagnoserChainCh: diagnoserChainCh,
	}
}

// Run runs the diagnoser chain.
func (dc *diagnoserChain) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !dc.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process diagnoses queuing in diagnoser chain channel.
		case diagnosis := <-dc.diagnoserChainCh:
			err := dc.client.Get(dc, client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			}, &diagnosis)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}

				err := util.QueueDiagnosis(dc, dc.diagnoserChainCh, diagnosis)
				if err != nil {
					dc.Error(err, "failed to send diagnosis to diagnoser chain queue", "diagnosis", client.ObjectKey{
						Name:      diagnosis.Name,
						Namespace: diagnosis.Namespace,
					})
				}
				continue
			}

			// Only process diagnosis in DiagnosisDiagnosing phase.
			if diagnosis.Status.Phase != diagnosisv1.DiagnosisDiagnosing {
				continue
			}

			if util.IsDiagnosisNodeNameMatched(diagnosis, dc.nodeName) {
				diagnosis, err := dc.SyncDiagnosis(diagnosis)
				if err != nil {
					dc.Error(err, "failed to sync Diagnosis", "diagnosis", diagnosis)
				}

				dc.Info("syncing Diagnosis successfully", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			}
		// Stop diagnoser chain on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncDiagnosis syncs diagnoses.
func (dc *diagnoserChain) SyncDiagnosis(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	dc.Info("starting to sync Diagnosis", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosers, err := dc.listDiagnosers()
	if err != nil {
		dc.Error(err, "failed to list Diagnosers")
		dc.addDiagnosisToDiagnoserChainQueue(diagnosis)
		return diagnosis, err
	}

	diagnosis, err = dc.runDiagnosis(diagnosers, diagnosis)
	if err != nil {
		dc.Error(err, "failed to run diagnosis")
		dc.addDiagnosisToDiagnoserChainQueue(diagnosis)
		return diagnosis, err
	}

	// Increment counter of successful diagnosis syncs by diagnoser chain.
	diagnoserChainSyncSuccessCount.Inc()

	return diagnosis, nil
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
	if err := dc.cache.List(dc, &diagnoserList); err != nil {
		return nil, err
	}

	return diagnoserList.Items, nil
}

// runDiagnosis diagnoses an diagnosis with diagnosers.
func (dc *diagnoserChain) runDiagnosis(diagnosers []diagnosisv1.Diagnoser, diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	// Run command executor of Diagnoser type.
	for _, executorSpec := range diagnosis.Spec.CommandExecutors {
		if executorSpec.Type == diagnosisv1.DiagnoserType {
			executorStatus, err := util.RunCommandExecutor(executorSpec, dc)
			if err != nil {
				diagnoserChainCommandExecutorFailCount.Inc()
				dc.Error(err, "failed to run command executor", "command", executorSpec.Command, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			} else {
				diagnoserChainCommandExecutorSuccessCount.Inc()
			}

			diagnosis.Status.CommandExecutors = append(diagnosis.Status.CommandExecutors, executorStatus)
		}
	}

	// Run profiler of Diagnoser type.
	for _, profilerSpec := range diagnosis.Spec.Profilers {
		if profilerSpec.Type == diagnosisv1.DiagnoserType {
			profilerStatus, err := util.RunProfiler(dc, diagnosis.Name, diagnosis.Namespace, dc.bindAddress, dc.dataRoot, profilerSpec, diagnosis.Spec.PodReference, dc.client, dc)
			if err != nil {
				diagnoserChainProfilerFailCount.Inc()
				dc.Error(err, "failed to run profiler", "profiler", profilerSpec, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			} else {
				diagnoserChainProfilerSuccessCount.Inc()
			}

			diagnosis.Status.Profilers = append(diagnosis.Status.Profilers, profilerStatus)
		}
	}

	// Skip diagnosis if AssignedDiagnosers is empty.
	if len(diagnosis.Spec.AssignedDiagnosers) == 0 {
		diagnoserChainSyncSkipCount.Inc()
		dc.Info("skipping diagnosis", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
		dc.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "SkippingDiagnosis", "Skipping diagnosis")

		diagnosis, err := dc.sendDiagnosisToRecovererChain(diagnosis)
		if err != nil {
			return diagnosis, err
		}

		return diagnosis, nil
	}

	for _, diagnoser := range diagnosers {
		// Execute only matched diagnosers.
		matched := false
		for _, assignedDiagnoser := range diagnosis.Spec.AssignedDiagnosers {
			if diagnoser.Name == assignedDiagnoser.Name && diagnoser.Namespace == assignedDiagnoser.Namespace {
				dc.Info("assigned diagnoser matched", "diagnoser", client.ObjectKey{
					Name:      diagnoser.Name,
					Namespace: diagnoser.Namespace,
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

		dc.Info("running diagnosis", "diagnoser", client.ObjectKey{
			Name:      diagnoser.Name,
			Namespace: diagnoser.Namespace,
		}, "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		var host string
		var port int32
		if diagnoser.Spec.ExternalIP != nil {
			host = *diagnoser.Spec.ExternalIP
		} else {
			host = dc.bindAddress
		}
		if diagnoser.Spec.ExternalPort != nil {
			port = *diagnoser.Spec.ExternalPort
		} else {
			port = int32(dc.port)
		}
		path := diagnoser.Spec.Path
		scheme := strings.ToLower(string(diagnoser.Spec.Scheme))
		url := util.FormatURL(scheme, host, strconv.Itoa(int(port)), path)
		timeout := time.Duration(diagnoser.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: dc.transport,
		}

		// Send http request to the diagnosers with payload of diagnosis.
		result, err := util.DoHTTPRequestWithDiagnosis(diagnosis, url, *cli, dc)
		if err != nil {
			dc.Error(err, "failed to do http request to diagnoser", "diagnoser", client.ObjectKey{
				Name:      diagnoser.Name,
				Namespace: diagnoser.Namespace,
			}, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			continue
		}

		// Validate an diagnosis after processed by a diagnoser.
		err = util.ValidateDiagnosisResult(result, diagnosis)
		if err != nil {
			dc.Error(err, "invalid result from diagnoser", "diagnoser", client.ObjectKey{
				Name:      diagnoser.Name,
				Namespace: diagnoser.Namespace,
			}, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			continue
		}

		diagnosis.Status = result.Status
		diagnosis.Status.Diagnoser = &diagnosisv1.NamespacedName{
			Name:      diagnoser.Name,
			Namespace: diagnoser.Namespace,
		}
		diagnosis, err := dc.sendDiagnosisToRecovererChain(diagnosis)
		if err != nil {
			return diagnosis, err
		}

		dc.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "Identified", "Diagnosis identified by %s/%s", diagnoser.Namespace, diagnoser.Name)

		return diagnosis, nil
	}

	diagnosis, err := dc.setDiagnosisFailed(diagnosis)
	if err != nil {
		return diagnosis, err
	}

	dc.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "FailedIdentify", "Unable to identify diagnosis %s(%s)", diagnosis.Name, diagnosis.UID)

	return diagnosis, nil
}

// sendDiagnosisToRecovererChain sends Diagnosis to recoverer chain.
func (dc *diagnoserChain) sendDiagnosisToRecovererChain(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	dc.Info("sending Diagnosis to recoverer chain", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.Phase = diagnosisv1.DiagnosisRecovering
	diagnosis.Status.Identifiable = true
	util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
		Type:   diagnosisv1.DiagnosisIdentified,
		Status: corev1.ConditionTrue,
	})
	if err := dc.client.Status().Update(dc, &diagnosis); err != nil {
		dc.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	return diagnosis, nil
}

// setDiagnosisFailed sets diagnosis phase to Failed.
func (dc *diagnoserChain) setDiagnosisFailed(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	dc.Info("setting Diagnosis phase to failed", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
	diagnosis.Status.Identifiable = false
	util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
		Type:   diagnosisv1.DiagnosisIdentified,
		Status: corev1.ConditionFalse,
	})
	if err := dc.client.Status().Update(dc, &diagnosis); err != nil {
		dc.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	diagnoserChainSyncFailCount.Inc()

	return diagnosis, nil
}

// addDiagnosisToDiagnoserChainQueue adds Diagnosis to the queue processed by diagnoser chain.
func (dc *diagnoserChain) addDiagnosisToDiagnoserChainQueue(diagnosis diagnosisv1.Diagnosis) {
	diagnoserChainSyncErrorCount.Inc()

	err := util.QueueDiagnosis(dc, dc.diagnoserChainCh, diagnosis)
	if err != nil {
		dc.Error(err, "failed to send diagnosis to diagnoser chain queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}

// addDiagnosisToDiagnoserChainQueueWithTimer adds Diagnosis to the queue processed by diagnoser chain with a timer.
func (dc *diagnoserChain) addDiagnosisToDiagnoserChainQueueWithTimer(diagnosis diagnosisv1.Diagnosis) {
	diagnoserChainSyncErrorCount.Inc()

	err := util.QueueDiagnosisWithTimer(dc, 30*time.Second, dc.diagnoserChainCh, diagnosis)
	if err != nil {
		dc.Error(err, "failed to send diagnosis to diagnoser chain queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}
