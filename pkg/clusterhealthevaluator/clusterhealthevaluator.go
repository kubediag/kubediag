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

package clusterhealthevaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// ClusterHealthEvaluator evaluates the health status of kubernetes cluster.
type ClusterHealthEvaluator interface {
	// Run runs the ClusterHealthEvaluator.
	Run(<-chan struct{})
	// Handler handles http requests to query health status of kubernetes cluster.
	Handler(http.ResponseWriter, *http.Request)
}

// clusterHealthEvaluator manages cluster health evaluations.
type clusterHealthEvaluator struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// RWMutex protects clusterHealth.
	sync.RWMutex

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// eventRecorder knows how to record events on behalf of an EventSource.
	eventRecorder record.EventRecorder
	// scheme defines methods for serializing and deserializing API objects.
	scheme *runtime.Scheme
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// housekeepingInterval is the housekeeping interval of cluster health evaluations.
	housekeepingInterval time.Duration
	// clusterHealth represents the health of kubernetes cluster.
	clusterHealth *types.ClusterHealth
}

// NewClusterHealthEvaluator creates a new ClusterHealthEvaluator.
func NewClusterHealthEvaluator(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	housekeepingInterval time.Duration,
) ClusterHealthEvaluator {
	clusterHealth := new(types.ClusterHealth)

	return &clusterHealthEvaluator{
		Context:              ctx,
		Logger:               logger,
		client:               cli,
		eventRecorder:        eventRecorder,
		scheme:               scheme,
		cache:                cache,
		housekeepingInterval: housekeepingInterval,
		clusterHealth:        clusterHealth,
	}
}

// Run runs the clusterHealthEvaluator.
func (ce *clusterHealthEvaluator) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !ce.cache.WaitForCacheSync(stopCh) {
		return
	}

	go wait.Until(func() {
		ce.Info("starting to evaluate cluster health")

		// List all pods.
		pods, err := ce.listPods()
		if err != nil {
			ce.Error(err, "failed to list pods")
			return
		}

		// Evaluate pod health.
		podHealth, err := ce.evaluatePodHealth(pods)
		if err != nil {
			ce.Error(err, "failed to evaluate pod health")
			return
		}

		// Evaluate workload health.
		workloadHealth, err := ce.evaluateWorkloadHealth(podHealth)
		if err != nil {
			ce.Error(err, "failed to evaluate workload health")
			return
		}

		// List all nodes.
		nodes, err := ce.listNodes()
		if err != nil {
			ce.Error(err, "failed to list nodes")
			return
		}

		// Evaluate node health.
		nodeHealth, err := ce.evaluateNodeHealth(nodes)
		if err != nil {
			ce.Error(err, "failed to evaluate node health")
			return
		}

		// Update kubernetes cluster health.
		ce.setClusterHealth(workloadHealth, nodeHealth)
		ce.Info("evaluating cluster health successfully")
	}, ce.housekeepingInterval, stopCh)
}

// Handler handles http requests for sending prometheus alerts.
func (ce *clusterHealthEvaluator) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		clusterHealth := ce.getClusterHealth()
		data, err := json.Marshal(clusterHealth)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal cluster health: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// evaluatePodHealth evaluates pod health by traversing all pods and calculates a health score according to
// pod statistics. It takes a slice containing all pods in the cluster as a parameter.
func (ce *clusterHealthEvaluator) evaluatePodHealth(pods []corev1.Pod) (types.PodHealth, error) {
	var podHealth types.PodHealth
	containerStateReasons := make(map[string]int)

	// Update pod health statistics.
	for _, pod := range pods {
		podHealth.Statistics.Total++

		// Update the pod health state according to the phase.
		switch pod.Status.Phase {
		case corev1.PodRunning:
			ready := true
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					ready = false
					break
				}
			}

			// A pod in Running phase could be in any of the following states:
			//
			// Ready: All containers in the pod is in ready state.
			// Unready: One of the containers is not ready.
			// Terminating: Pod DeletionTimestamp is not nil.
			if pod.DeletionTimestamp != nil {
				podHealth.Statistics.Unhealthy.Terminating++
			} else if ready {
				podHealth.Statistics.Healthy.Ready++
			} else {
				podHealth.Statistics.Unhealthy.Unready++
				reason := util.GetPodUnhealthyReason(pod)
				if !util.UpdatePodUnhealthyReasonStatistics(containerStateReasons, reason) {
					ce.Info("container unhealthy reason not found", "pod", client.ObjectKey{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					}, "phase", corev1.PodRunning)
				}
			}
		case corev1.PodSucceeded:
			podHealth.Statistics.Healthy.Succeeded++
		case corev1.PodPending:
			podHealth.Statistics.Unhealthy.Pending++
			reason := util.GetPodUnhealthyReason(pod)
			if !util.UpdatePodUnhealthyReasonStatistics(containerStateReasons, reason) {
				ce.Info("container unhealthy reason not found", "pod", client.ObjectKey{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				}, "phase", corev1.PodPending)
			}
		case corev1.PodFailed:
			podHealth.Statistics.Unhealthy.Failed++
			reason := util.GetPodUnhealthyReason(pod)
			if !util.UpdatePodUnhealthyReasonStatistics(containerStateReasons, reason) {
				ce.Info("container unhealthy reason not found", "pod", client.ObjectKey{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				}, "phase", corev1.PodFailed)
			}
		case corev1.PodUnknown:
			podHealth.Statistics.Unhealthy.Unknown++
			reason := util.GetPodUnhealthyReason(pod)
			if !util.UpdatePodUnhealthyReasonStatistics(containerStateReasons, reason) {
				ce.Info("container unhealthy reason not found", "pod", client.ObjectKey{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				}, "phase", corev1.PodUnknown)
			}
		}
	}

	podHealth.Statistics.Unhealthy.ContainerStateReasons = containerStateReasons
	podHealth.Score = util.CalculatePodHealthScore(podHealth.Statistics)

	return podHealth, nil
}

// evaluateWorkloadHealth evaluates workload health according to health of workload resources including
// pod, deployment, statefulset, daemonset and job.
func (ce *clusterHealthEvaluator) evaluateWorkloadHealth(podHealth types.PodHealth) (types.WorkloadHealth, error) {
	var workloadHealth types.WorkloadHealth
	workloadHealth.PodHealth = podHealth
	workloadHealth.Score = podHealth.Score

	return workloadHealth, nil
}

// evaluateNodeHealth evaluates node health by traversing all nodes and calculates a health score according to
// node statistics. It takes a slice containing all nodes in the cluster as a parameter.
func (ce *clusterHealthEvaluator) evaluateNodeHealth(nodes []corev1.Node) (types.NodeHealth, error) {
	var nodeHealth types.NodeHealth
	unhealthy := make(map[corev1.NodeConditionType]int)

	// Update node health statistics.
	for _, node := range nodes {
		nodeHealth.Statistics.Total++

		// Update the node health state according to the condition.
		if util.IsNodeReady(node) {
			nodeHealth.Statistics.Healthy++
		} else {
			conditionType := util.GetNodeUnhealthyConditionType(node)
			unhealthy[conditionType]++
		}
	}

	nodeHealth.Statistics.Unhealthy = unhealthy
	nodeHealth.Score = util.CalculateNodeHealthScore(nodeHealth.Statistics)

	return nodeHealth, nil
}

// setClusterHealth updates kubernetes cluster health.
func (ce *clusterHealthEvaluator) setClusterHealth(
	workloadHealth types.WorkloadHealth,
	nodeHealth types.NodeHealth,
) {
	ce.Lock()
	defer ce.Unlock()

	ce.clusterHealth.WorkloadHealth = workloadHealth
	ce.clusterHealth.NodeHealth = nodeHealth
	ce.clusterHealth.Score = (workloadHealth.Score + nodeHealth.Score) / 2
}

// getClusterHealth retrieves kubernetes cluster health.
func (ce *clusterHealthEvaluator) getClusterHealth() types.ClusterHealth {
	ce.RLock()
	defer ce.RUnlock()

	return *ce.clusterHealth
}

// listPods lists Pods from cache.
func (ce *clusterHealthEvaluator) listPods() ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := ce.cache.List(ce, &podList); err != nil {
		return nil, err
	}

	return podList.Items, nil
}

// listNodes lists Nodes from cache.
func (ce *clusterHealthEvaluator) listNodes() ([]corev1.Node, error) {
	var nodeList corev1.NodeList
	if err := ce.cache.List(ce, &nodeList); err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}
