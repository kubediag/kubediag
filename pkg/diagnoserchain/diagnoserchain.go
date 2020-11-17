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
			Help: "Counter of successful abnormal syncs by diagnoser chain",
		},
	)
	diagnoserChainSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_skip_count",
			Help: "Counter of skipped abnormal syncs by diagnoser chain",
		},
	)
	diagnoserChainSyncFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_fail_count",
			Help: "Counter of failed abnormal syncs by diagnoser chain",
		},
	)
	diagnoserChainSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnoser_chain_sync_error_count",
			Help: "Counter of erroneous abnormal syncs by diagnoser chain",
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
	// diagnoserChainCh is a channel for queuing Abnormals to be processed by diagnoser chain.
	diagnoserChainCh chan diagnosisv1.Abnormal
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
	diagnoserChainCh chan diagnosisv1.Abnormal,
) types.AbnormalManager {
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
		// Process abnormals queuing in diagnoser chain channel.
		case abnormal := <-dc.diagnoserChainCh:
			err := dc.client.Get(dc, client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			}, &abnormal)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}

				err := util.QueueAbnormal(dc, dc.diagnoserChainCh, abnormal)
				if err != nil {
					dc.Error(err, "failed to send abnormal to diagnoser chain queue", "abnormal", client.ObjectKey{
						Name:      abnormal.Name,
						Namespace: abnormal.Namespace,
					})
				}
				continue
			}

			// Only process abnormal in AbnormalDiagnosing phase.
			if abnormal.Status.Phase != diagnosisv1.AbnormalDiagnosing {
				continue
			}

			if util.IsAbnormalNodeNameMatched(abnormal, dc.nodeName) {
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

	// Increment counter of successful abnormal syncs by diagnoser chain.
	diagnoserChainSyncSuccessCount.Inc()

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
	if err := dc.cache.List(dc, &diagnoserList); err != nil {
		return nil, err
	}

	return diagnoserList.Items, nil
}

// runDiagnosis diagnoses an abnormal with diagnosers.
func (dc *diagnoserChain) runDiagnosis(diagnosers []diagnosisv1.Diagnoser, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Run command executor of Diagnoser type.
	for _, executor := range abnormal.Spec.CommandExecutors {
		if executor.Type == diagnosisv1.DiagnoserType {
			executor, err := util.RunCommandExecutor(executor, dc)
			if err != nil {
				diagnoserChainCommandExecutorFailCount.Inc()
				dc.Error(err, "failed to run command executor", "command", executor.Command, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				executor.Error = err.Error()
			}

			diagnoserChainCommandExecutorSuccessCount.Inc()
			abnormal.Status.CommandExecutors = append(abnormal.Status.CommandExecutors, executor)
		}
	}

	// Run profiler of Diagnoser type.
	for _, profiler := range abnormal.Spec.Profilers {
		if profiler.Type == diagnosisv1.DiagnoserType {
			profiler, err := util.RunProfiler(dc, abnormal.Name, abnormal.Namespace, profiler, dc.client, dc)
			if err != nil {
				diagnoserChainProfilerFailCount.Inc()
				dc.Error(err, "failed to run profiler", "profiler", profiler, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				profiler.Error = err.Error()
			}

			diagnoserChainProfilerFailCount.Inc()
			abnormal.Status.Profilers = append(abnormal.Status.Profilers, profiler)
		}
	}

	// Skip diagnosis if AssignedDiagnosers is empty.
	if len(abnormal.Spec.AssignedDiagnosers) == 0 {
		diagnoserChainSyncSkipCount.Inc()
		dc.Info("skipping diagnosis", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
		dc.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "SkippingDiagnosis", "Skipping diagnosis")

		abnormal, err := dc.sendAbnormalToRecovererChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	for _, diagnoser := range diagnosers {
		// Execute only matched diagnosers.
		matched := false
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
		url := util.FormatURL(scheme, host, strconv.Itoa(int(port)), path)
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

		dc.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "Identified", "Abnormal identified by %s/%s", diagnoser.Namespace, diagnoser.Name)

		return abnormal, nil
	}

	abnormal, err := dc.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	dc.eventRecorder.Eventf(&abnormal, corev1.EventTypeWarning, "FailedIdentify", "Unable to identify abnormal %s(%s)", abnormal.Name, abnormal.UID)

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
	if err := dc.client.Status().Update(dc, &abnormal); err != nil {
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
	if err := dc.client.Status().Update(dc, &abnormal); err != nil {
		dc.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	diagnoserChainSyncFailCount.Inc()

	return abnormal, nil
}

// addAbnormalToDiagnoserChainQueue adds Abnormal to the queue processed by diagnoser chain.
func (dc *diagnoserChain) addAbnormalToDiagnoserChainQueue(abnormal diagnosisv1.Abnormal) {
	diagnoserChainSyncErrorCount.Inc()

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
	diagnoserChainSyncErrorCount.Inc()

	err := util.QueueAbnormalWithTimer(dc, 30*time.Second, dc.diagnoserChainCh, abnormal)
	if err != nil {
		dc.Error(err, "failed to send abnormal to diagnoser chain queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
