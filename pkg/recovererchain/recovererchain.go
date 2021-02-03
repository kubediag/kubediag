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
	recovererChainSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_sync_success_count",
			Help: "Counter of successful diagnosis syncs by recoverer chain",
		},
	)
	recovererChainSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_sync_skip_count",
			Help: "Counter of skipped diagnosis syncs by recoverer chain",
		},
	)
	recovererChainSyncFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_sync_fail_count",
			Help: "Counter of failed diagnosis syncs by recoverer chain",
		},
	)
	recovererChainSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_sync_error_count",
			Help: "Counter of erroneous diagnosis syncs by recoverer chain",
		},
	)
	recovererChainCommandExecutorSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_command_executor_success_count",
			Help: "Counter of successful command executor runs by recoverer chain",
		},
	)
	recovererChainCommandExecutorFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_command_executor_fail_count",
			Help: "Counter of failed command executor runs by recoverer chain",
		},
	)
	recovererChainProfilerSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_profiler_success_count",
			Help: "Counter of successful profiler runs by recoverer chain",
		},
	)
	recovererChainProfilerFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "recoverer_chain_profiler_fail_count",
			Help: "Counter of failed profiler runs by recoverer chain",
		},
	)
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
	// bindAddress is the address on which to advertise.
	bindAddress string
	// port is the port for the kube diagnoser to serve on.
	port int
	// dataRoot is root directory of persistent kube diagnoser data.
	dataRoot string
	// recovererChainCh is a channel for queuing Diagnoses to be processed by recoverer chain.
	recovererChainCh chan diagnosisv1.Diagnosis
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
	bindAddress string,
	port int,
	dataRoot string,
	recovererChainCh chan diagnosisv1.Diagnosis,
) types.DiagnosisManager {
	metrics.Registry.MustRegister(
		recovererChainSyncSuccessCount,
		recovererChainSyncSkipCount,
		recovererChainSyncFailCount,
		recovererChainSyncErrorCount,
		recovererChainCommandExecutorSuccessCount,
		recovererChainCommandExecutorFailCount,
		recovererChainProfilerSuccessCount,
		recovererChainProfilerFailCount,
	)

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
		bindAddress:      bindAddress,
		port:             port,
		dataRoot:         dataRoot,
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
		// Process diagnoses queuing in recoverer chain channel.
		case diagnosis := <-rc.recovererChainCh:
			err := rc.client.Get(rc, client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			}, &diagnosis)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}

				err := util.QueueDiagnosis(rc, rc.recovererChainCh, diagnosis)
				if err != nil {
					rc.Error(err, "failed to send diagnosis to recoverer chain queue", "diagnosis", client.ObjectKey{
						Name:      diagnosis.Name,
						Namespace: diagnosis.Namespace,
					})
				}
				continue
			}

			// Only process diagnosis in DiagnosisRecovering phase.
			if diagnosis.Status.Phase != diagnosisv1.DiagnosisRecovering {
				continue
			}

			if util.IsDiagnosisNodeNameMatched(diagnosis, rc.nodeName) {
				diagnosis, err := rc.SyncDiagnosis(diagnosis)
				if err != nil {
					rc.Error(err, "failed to sync Diagnosis", "diagnosis", diagnosis)
				}

				rc.Info("syncing Diagnosis successfully", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			}
		// Stop recoverer chain on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncDiagnosis syncs diagnoses.
func (rc *recovererChain) SyncDiagnosis(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	rc.Info("starting to sync Diagnosis", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	recoverers, err := rc.listRecoverers()
	if err != nil {
		rc.Error(err, "failed to list Recoverers")
		rc.addDiagnosisToRecovererChainQueue(diagnosis)
		return diagnosis, err
	}

	diagnosis, err = rc.runRecovery(recoverers, diagnosis)
	if err != nil {
		rc.Error(err, "failed to run recovery")
		rc.addDiagnosisToRecovererChainQueue(diagnosis)
		return diagnosis, err
	}

	// Increment counter of successful diagnosis syncs by recoverer chain.
	recovererChainSyncSuccessCount.Inc()

	return diagnosis, nil
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

// runRecovery recovers an diagnosis with recoverers.
func (rc *recovererChain) runRecovery(recoverers []diagnosisv1.Recoverer, diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	// Run command executor of Recoverer type.
	for _, executorSpec := range diagnosis.Spec.CommandExecutors {
		if executorSpec.Type == diagnosisv1.RecovererType {
			executorStatus, err := util.RunCommandExecutor(executorSpec, rc)
			if err != nil {
				recovererChainCommandExecutorFailCount.Inc()
				rc.Error(err, "failed to run command executor", "command", executorSpec.Command, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			} else {
				recovererChainCommandExecutorSuccessCount.Inc()
			}

			diagnosis.Status.CommandExecutors = append(diagnosis.Status.CommandExecutors, executorStatus)
		}
	}

	// Run profiler of Recoverer type.
	for _, profilerSpec := range diagnosis.Spec.Profilers {
		if profilerSpec.Type == diagnosisv1.RecovererType {
			profilerStatus, err := util.RunProfiler(rc, diagnosis.Name, diagnosis.Namespace, rc.bindAddress, rc.dataRoot, profilerSpec, diagnosis.Spec.PodReference, rc.client, rc)
			if err != nil {
				recovererChainProfilerFailCount.Inc()
				rc.Error(err, "failed to run profiler", "profiler", profilerSpec, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			} else {
				recovererChainProfilerSuccessCount.Inc()
			}

			diagnosis.Status.Profilers = append(diagnosis.Status.Profilers, profilerStatus)
		}
	}

	// Skip recovery if AssignedRecoverers is empty.
	if len(diagnosis.Spec.AssignedRecoverers) == 0 {
		recovererChainSyncSkipCount.Inc()
		rc.Info("skipping recovery", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
		rc.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "SkippingRecovery", "Skipping recovery")

		diagnosis, err := rc.setDiagnosisSucceeded(diagnosis)
		if err != nil {
			return diagnosis, err
		}

		return diagnosis, nil
	}

	for _, recoverer := range recoverers {
		// Execute only matched recoverers.
		matched := false
		for _, assignedRecoverer := range diagnosis.Spec.AssignedRecoverers {
			if recoverer.Name == assignedRecoverer.Name && recoverer.Namespace == assignedRecoverer.Namespace {
				rc.Info("assigned recoverer matched", "recoverer", client.ObjectKey{
					Name:      recoverer.Name,
					Namespace: recoverer.Namespace,
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

		rc.Info("running recovery", "recoverer", client.ObjectKey{
			Name:      recoverer.Name,
			Namespace: recoverer.Namespace,
		}, "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		var host string
		var port int32
		if recoverer.Spec.ExternalIP != nil {
			host = *recoverer.Spec.ExternalIP
		} else {
			host = rc.bindAddress
		}
		if recoverer.Spec.ExternalPort != nil {
			port = *recoverer.Spec.ExternalPort
		} else {
			port = int32(rc.port)
		}
		path := recoverer.Spec.Path
		scheme := strings.ToLower(string(recoverer.Spec.Scheme))
		url := util.FormatURL(scheme, host, strconv.Itoa(int(port)), path)
		timeout := time.Duration(recoverer.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: rc.transport,
		}

		// Send http request to the recoverers with payload of diagnosis.
		result, err := util.DoHTTPRequestWithDiagnosis(diagnosis, url, *cli, rc)
		if err != nil {
			rc.Error(err, "failed to do http request to recoverer", "recoverer", client.ObjectKey{
				Name:      recoverer.Name,
				Namespace: recoverer.Namespace,
			}, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			continue
		}

		// Validate an diagnosis after processed by a recoverer.
		err = util.ValidateDiagnosisResult(result, diagnosis)
		if err != nil {
			rc.Error(err, "invalid result from recoverer", "recoverer", client.ObjectKey{
				Name:      recoverer.Name,
				Namespace: recoverer.Namespace,
			}, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			continue
		}

		diagnosis.Status = result.Status
		diagnosis.Status.Recoverer = &diagnosisv1.NamespacedName{
			Name:      recoverer.Name,
			Namespace: recoverer.Namespace,
		}
		diagnosis, err := rc.setDiagnosisSucceeded(diagnosis)
		if err != nil {
			return diagnosis, err
		}

		rc.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "Recovered", "Diagnosis recovered by %s/%s", recoverer.Namespace, recoverer.Name)

		return diagnosis, nil
	}

	diagnosis, err := rc.setDiagnosisFailed(diagnosis)
	if err != nil {
		return diagnosis, err
	}

	rc.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "FailedRecover", "Unable to recover diagnosis %s(%s)", diagnosis.Name, diagnosis.UID)

	return diagnosis, nil
}

// setDiagnosisSucceeded sets diagnosis phase to Succeeded.
func (rc *recovererChain) setDiagnosisSucceeded(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	rc.Info("setting Diagnosis phase to succeeded", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.Phase = diagnosisv1.DiagnosisSucceeded
	diagnosis.Status.Recoverable = true
	util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
		Type:   diagnosisv1.DiagnosisRecovered,
		Status: corev1.ConditionTrue,
	})
	if err := rc.client.Status().Update(rc, &diagnosis); err != nil {
		rc.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	return diagnosis, nil
}

// setDiagnosisFailed sets diagnosis phase to Failed.
func (rc *recovererChain) setDiagnosisFailed(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	rc.Info("setting Diagnosis phase to failed", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
	diagnosis.Status.Recoverable = false
	util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
		Type:   diagnosisv1.DiagnosisRecovered,
		Status: corev1.ConditionFalse,
	})
	if err := rc.client.Status().Update(rc, &diagnosis); err != nil {
		rc.Error(err, "unable to update Diagnosis")
		return diagnosis, err
	}

	recovererChainSyncFailCount.Inc()

	return diagnosis, nil
}

// addDiagnosisToRecovererChainQueue adds Diagnosis to the queue processed by recoverer chain.
func (rc *recovererChain) addDiagnosisToRecovererChainQueue(diagnosis diagnosisv1.Diagnosis) {
	recovererChainSyncErrorCount.Inc()

	err := util.QueueDiagnosis(rc, rc.recovererChainCh, diagnosis)
	if err != nil {
		rc.Error(err, "failed to send diagnosis to recoverer chain queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}

// addDiagnosisToRecovererChainQueueWithTimer adds Diagnosis to the queue processed by recoverer chain with a timer.
func (rc *recovererChain) addDiagnosisToRecovererChainQueueWithTimer(diagnosis diagnosisv1.Diagnosis) {
	recovererChainSyncErrorCount.Inc()

	err := util.QueueDiagnosisWithTimer(rc, 30*time.Second, rc.recovererChainCh, diagnosis)
	if err != nil {
		rc.Error(err, "failed to send diagnosis to recoverer chain queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}
