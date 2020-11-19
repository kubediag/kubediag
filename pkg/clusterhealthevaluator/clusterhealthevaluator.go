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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
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
	// apiserverAccessToken is the kubernetes apiserver access token.
	apiserverAccessToken string
	// clusterHealthEvaluatorEnabled indicates whether clusterHealthEvaluator is enabled.
	clusterHealthEvaluatorEnabled bool
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
	apiserverAccessToken string,
	clusterHealthEvaluatorEnabled bool,
) ClusterHealthEvaluator {
	clusterHealth := new(types.ClusterHealth)

	return &clusterHealthEvaluator{
		Context:                       ctx,
		Logger:                        logger,
		client:                        cli,
		eventRecorder:                 eventRecorder,
		scheme:                        scheme,
		cache:                         cache,
		housekeepingInterval:          housekeepingInterval,
		clusterHealth:                 clusterHealth,
		apiserverAccessToken:          apiserverAccessToken,
		clusterHealthEvaluatorEnabled: clusterHealthEvaluatorEnabled,
	}
}

// Run runs the clusterHealthEvaluator.
func (ce *clusterHealthEvaluator) Run(stopCh <-chan struct{}) {
	if !ce.clusterHealthEvaluatorEnabled {
		return
	}

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

		// List all deployments.
		deployments, err := ce.listDeployments()
		if err != nil {
			ce.Error(err, "failed to list deployments")
			return
		}

		// Evaluate deployment health.
		deploymentHealth, err := ce.evaluateDeploymentHealth(deployments)
		if err != nil {
			ce.Error(err, "failed to evaluate deployment health")
			return
		}

		// Evaluate workload health.
		workloadHealth, err := ce.evaluateWorkloadHealth(podHealth, deploymentHealth)
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

		// Create abnormals on terminating pods.
		err = ce.generateTerminatingPodAbnormal(pods)
		if err != nil {
			ce.Error(err, "failed to create abnormals on terminating pods")
			return
		}
	}, ce.housekeepingInterval, stopCh)
}

// Handler handles http requests for sending prometheus alerts.
func (ce *clusterHealthEvaluator) Handler(w http.ResponseWriter, r *http.Request) {
	if !ce.clusterHealthEvaluatorEnabled {
		http.Error(w, fmt.Sprintf("cluster health evaluator is not enabled"), http.StatusUnprocessableEntity)
		return
	}

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
	podHealth.Score = calculatePodHealthScore(podHealth.Statistics)

	return podHealth, nil
}

// calculatePodHealthScore calculates pod health score according to pod statistics.
func calculatePodHealthScore(statistics types.PodStatistics) int {
	if statistics.Total == 0 {
		return types.MaxHealthScore
	}

	healthy := statistics.Healthy.Ready + statistics.Healthy.Succeeded
	total := statistics.Total

	return types.MaxHealthScore * healthy / total
}

// evaluateDeploymentHealth evaluates deployment health by traversing all deployments and calculates a
// health score according to deployment statistics. It takes a slice containing all deployments in the
// cluster as a parameter.
func (ce *clusterHealthEvaluator) evaluateDeploymentHealth(deployments []appsv1.Deployment) (types.DeploymentHealth, error) {
	var deploymentHealth types.DeploymentHealth

	// Update deployment health statistics.
	for _, deployment := range deployments {
		deploymentHealth.Statistics.Total++

		// Update the deployment health state according to the pod availability.
		if deployment.Status.AvailableReplicas == deployment.Status.Replicas {
			deploymentHealth.Statistics.Healthy++
		} else if float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) < types.OneQuarter {
			deploymentHealth.Statistics.Unhealthy.OneQuarterAvailable++
		} else if float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) >= types.OneQuarter &&
			float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) < types.TwoQuarters {
			deploymentHealth.Statistics.Unhealthy.TwoQuartersAvailable++
		} else if float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) >= types.TwoQuarters &&
			float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) < types.ThreeQuarters {
			deploymentHealth.Statistics.Unhealthy.ThreeQuartersAvailable++
		} else if float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) >= types.ThreeQuarters &&
			float32(deployment.Status.AvailableReplicas)/float32(deployment.Status.Replicas) < types.FourQuarters {
			deploymentHealth.Statistics.Unhealthy.FourQuartersAvailable++
		}
	}

	deploymentHealth.Score = calculateDeploymentHealthScore(deploymentHealth.Statistics)

	return deploymentHealth, nil
}

// calculateDeploymentHealthScore calculates deployment health score according to deployment statistics.
func calculateDeploymentHealthScore(statistics types.DeploymentStatistics) int {
	if statistics.Total == 0 {
		return types.MaxHealthScore
	}

	healthy := statistics.Healthy
	total := statistics.Total
	unhealthyAvailability := int((float32(statistics.Unhealthy.OneQuarterAvailable)*0 +
		float32(statistics.Unhealthy.TwoQuartersAvailable)*types.OneQuarter +
		float32(statistics.Unhealthy.ThreeQuartersAvailable)*types.TwoQuarters +
		float32(statistics.Unhealthy.FourQuartersAvailable)*types.ThreeQuarters))
	unhealthyScore := types.MaxHealthScore * unhealthyAvailability / total
	healthyScore := types.MaxHealthScore * healthy / total

	return healthyScore + unhealthyScore
}

// evaluateWorkloadHealth evaluates workload health according to health of workload resources including
// pod, deployment, statefulset, daemonset and job.
func (ce *clusterHealthEvaluator) evaluateWorkloadHealth(
	podHealth types.PodHealth,
	deploymentHealth types.DeploymentHealth,
) (types.WorkloadHealth, error) {
	var workloadHealth types.WorkloadHealth
	workloadHealth.PodHealth = podHealth
	workloadHealth.DeploymentHealth = deploymentHealth
	workloadHealth.Score = (podHealth.Score + deploymentHealth.Score) / 2

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
	nodeHealth.Score = calculateNodeHealthScore(nodeHealth.Statistics)

	return nodeHealth, nil
}

// calculateNodeHealthScore calculates node health score according to node statistics.
func calculateNodeHealthScore(statistics types.NodeStatistics) int {
	if statistics.Total == 0 {
		return types.MaxHealthScore
	}

	healthy := statistics.Healthy
	total := statistics.Total

	return types.MaxHealthScore * healthy / total
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

// listDeployments lists Deployments from cache.
func (ce *clusterHealthEvaluator) listDeployments() ([]appsv1.Deployment, error) {
	var deploymentList appsv1.DeploymentList
	if err := ce.cache.List(ce, &deploymentList); err != nil {
		return nil, err
	}

	return deploymentList.Items, nil
}

// listNodes lists Nodes from cache.
func (ce *clusterHealthEvaluator) listNodes() ([]corev1.Node, error) {
	var nodeList corev1.NodeList
	if err := ce.cache.List(ce, &nodeList); err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}

// getAbnormal gets an Abnormal from cache.
func (ce *clusterHealthEvaluator) getAbnormal(ctx context.Context, namespace string, name string) (diagnosisv1.Abnormal, error) {
	var abnormal diagnosisv1.Abnormal
	if err := ce.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &abnormal); err != nil {
		return diagnosisv1.Abnormal{}, err
	}

	return abnormal, nil
}

// generateTerminatingPodAbnormal creates abnormals on terminating pods.
func (ce *clusterHealthEvaluator) generateTerminatingPodAbnormal(pods []corev1.Pod) error {
	for _, pod := range pods {
		// Synchronize the pod if the pod is in terminating state and the state of the pod could be obtained.
		if pod.DeletionTimestamp != nil && pod.Status.Phase != corev1.PodUnknown {
			// Retrieve the grace period of pod.
			var gracePeriod time.Duration
			if pod.Spec.TerminationGracePeriodSeconds != nil {
				gracePeriod = time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
			} else {
				gracePeriod = corev1.DefaultTerminationGracePeriodSeconds
			}

			// A terminating pod abnormal is created if the pod is not terminated by deadline.
			deadline := metav1.NewTime(pod.DeletionTimestamp.Add(gracePeriod).Add(util.PodKillGracePeriodSeconds))
			now := metav1.Now()
			if (&deadline).Before(&now) {
				name := fmt.Sprintf("%s.%s.%s", util.TerminatingPodAbnormalNamePrefix, pod.Name, pod.UID)
				namespace := pod.Namespace

				abnormal, err := ce.getAbnormal(ce, namespace, name)
				if err != nil {
					if !apierrors.IsNotFound(err) {
						ce.Error(err, "unable to fetch abnormal", "abnormal", client.ObjectKey{
							Namespace: namespace,
							Name:      name,
						})
						continue
					}

					// Create an abnormal if the abormal on the terminating pod does not exist.
					abnormal = diagnosisv1.Abnormal{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Spec: diagnosisv1.AbnormalSpec{
							Source:   diagnosisv1.CustomSource,
							NodeName: pod.Spec.NodeName,
							AssignedInformationCollectors: []diagnosisv1.NamespacedName{
								{
									Name:      util.DefautlPodCollector,
									Namespace: util.DefautlNamespace,
								},
								{
									Name:      util.DefautlProcessCollector,
									Namespace: util.DefautlNamespace,
								},
							},
							AssignedDiagnosers: []diagnosisv1.NamespacedName{
								{
									Name:      util.DefautlTerminatingPodDiagnoser,
									Namespace: util.DefautlNamespace,
								},
							},
							AssignedRecoverers: []diagnosisv1.NamespacedName{
								{
									Name:      util.DefautlSignalRecoverer,
									Namespace: util.DefautlNamespace,
								},
							},
						},
					}

					if err := ce.client.Create(ce, &abnormal); err != nil {
						ce.Error(err, "unable to create abnormal", "abnormal", client.ObjectKey{
							Namespace: namespace,
							Name:      name,
						})
						continue
					}

					ce.Info("create abnormal successfully", "abnormal", client.ObjectKey{
						Namespace: namespace,
						Name:      name,
					})
				}
			}
		}
	}

	return nil
}
