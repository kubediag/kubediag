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
	informationManagerSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_success_count",
			Help: "Counter of successful abnormal syncs by information manager",
		},
	)
	informationManagerSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_skip_count",
			Help: "Counter of skipped abnormal syncs by information manager",
		},
	)
	informationManagerSyncFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_fail_count",
			Help: "Counter of failed abnormal syncs by information manager",
		},
	)
	informationManagerSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "information_manager_sync_error_count",
			Help: "Counter of erroneous abnormal syncs by information manager",
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
	// informationManagerCh is a channel for queuing Abnormals to be processed by information manager.
	informationManagerCh chan diagnosisv1.Abnormal
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
	informationManagerCh chan diagnosisv1.Abnormal,
) types.AbnormalManager {
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
		// Process abnormals queuing in information manager channel.
		case abnormal := <-im.informationManagerCh:
			if util.IsAbnormalNodeNameMatched(abnormal, im.nodeName) {
				abnormal, err := im.SyncAbnormal(abnormal)
				if err != nil {
					im.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				im.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}
		// Stop information manager on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncAbnormal syncs abnormals.
func (im *informationManager) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	im.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	_, condition := util.GetAbnormalCondition(&abnormal.Status, diagnosisv1.InformationCollected)
	if condition != nil {
		im.Info("ignoring Abnormal in phase InformationCollecting with condition InformationCollected", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	} else {
		informationCollectors, err := im.listInformationCollectors()
		if err != nil {
			im.Error(err, "failed to list InformationCollectors")
			im.addAbnormalToInformationManagerQueue(abnormal)
			return abnormal, err
		}

		abnormal, err := im.runInformationCollection(informationCollectors, abnormal)
		if err != nil {
			im.Error(err, "failed to run collection")
			im.addAbnormalToInformationManagerQueue(abnormal)
			return abnormal, err
		}
	}

	// Increment counter of successful abnormal syncs by information manager.
	informationManagerSyncSuccessCount.Inc()

	return abnormal, nil
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
func (im *informationManager) runInformationCollection(informationCollectors []diagnosisv1.InformationCollector, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Run command executor of InformationCollector type.
	for _, executor := range abnormal.Spec.CommandExecutors {
		if executor.Type == diagnosisv1.InformationCollectorType {
			executor, err := util.RunCommandExecutor(executor, im)
			if err != nil {
				informationManagerCommandExecutorFailCount.Inc()
				im.Error(err, "failed to run command executor", "command", executor.Command, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				executor.Error = err.Error()
			}

			informationManagerCommandExecutorSuccessCount.Inc()
			abnormal.Status.CommandExecutors = append(abnormal.Status.CommandExecutors, executor)
		}
	}

	// Run profiler of InformationCollector type.
	for _, profiler := range abnormal.Spec.Profilers {
		if profiler.Type == diagnosisv1.InformationCollectorType {
			profiler, err := util.RunProfiler(im, abnormal.Name, abnormal.Namespace, profiler, im.client, im)
			if err != nil {
				informationManagerProfilerFailCount.Inc()
				im.Error(err, "failed to run profiler", "profiler", profiler, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
				profiler.Error = err.Error()
			}

			informationManagerProfilerSuccessCount.Inc()
			abnormal.Status.Profilers = append(abnormal.Status.Profilers, profiler)
		}
	}

	// Skip collection if AssignedInformationCollectors is empty.
	if len(abnormal.Spec.AssignedInformationCollectors) == 0 {
		informationManagerSyncSkipCount.Inc()
		im.Info("skipping collection", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
		im.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "SkippingCollection", "Skipping collection")

		abnormal, err := im.sendAbnormalToDiagnoserChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	informationCollected := false
	for _, collector := range informationCollectors {
		// Execute only matched information collectors.
		matched := false
		for _, assignedCollector := range abnormal.Spec.AssignedInformationCollectors {
			if collector.Name == assignedCollector.Name && collector.Namespace == assignedCollector.Namespace {
				im.Info("assigned collector matched", "collector", client.ObjectKey{
					Name:      collector.Name,
					Namespace: collector.Namespace,
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

		im.Info("running collection", "collector", client.ObjectKey{
			Name:      collector.Name,
			Namespace: collector.Namespace,
		}, "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		scheme := strings.ToLower(string(collector.Spec.Scheme))
		host := collector.Spec.IP
		port := collector.Spec.Port
		path := collector.Spec.Path
		url := util.FormatURL(scheme, host, strconv.Itoa(int(port)), path)
		timeout := time.Duration(collector.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: im.transport,
		}

		// Send http request to the information collector with payload of abnormal.
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, im)
		if err != nil {
			im.Error(err, "failed to do http request to collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		// Validate an abnormal after processed by an information collector.
		err = util.ValidateAbnormalResult(result, abnormal)
		if err != nil {
			im.Error(err, "invalid result from collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		informationCollected = true
		abnormal.Status = result.Status

		im.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "InformationCollected", "Information collected by %s/%s", collector.Namespace, collector.Name)
	}

	// All assigned information collectors will be executed. The Abnormal will be sent to diagnoser chain
	// if any information is collected successfully.
	if informationCollected {
		abnormal, err := im.sendAbnormalToDiagnoserChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	abnormal, err := im.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	im.eventRecorder.Eventf(&abnormal, corev1.EventTypeWarning, "FailedCollect", "Unable to collect information for abnormal %s(%s)", abnormal.Name, abnormal.UID)

	return abnormal, nil
}

// sendAbnormalToDiagnoserChain sends Abnormal to diagnoser chain.
func (im *informationManager) sendAbnormalToDiagnoserChain(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	im.Info("sending Abnormal to diagnoser chain", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalDiagnosing
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionTrue,
	})
	if err := im.client.Status().Update(im, &abnormal); err != nil {
		im.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (im *informationManager) setAbnormalFailed(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	im.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionFalse,
	})
	if err := im.client.Status().Update(im, &abnormal); err != nil {
		im.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	informationManagerSyncFailCount.Inc()

	return abnormal, nil
}

// addAbnormalToInformationManagerQueue adds Abnormal to the queue processed by information manager.
func (im *informationManager) addAbnormalToInformationManagerQueue(abnormal diagnosisv1.Abnormal) {
	informationManagerSyncErrorCount.Inc()

	err := util.QueueAbnormal(im, im.informationManagerCh, abnormal)
	if err != nil {
		im.Error(err, "failed to send abnormal to information manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToInformationManagerQueueWithTimer adds Abnormal to the queue processed by information manager with a timer.
func (im *informationManager) addAbnormalToInformationManagerQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	informationManagerSyncErrorCount.Inc()

	err := util.QueueAbnormalWithTimer(im, 30*time.Second, im.informationManagerCh, abnormal)
	if err != nil {
		im.Error(err, "failed to send abnormal to information manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
