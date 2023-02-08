/*
Copyright 2020 The KubeDiag Authors.

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

package controllers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/util"
)

// Kubediag master metrics
var (
	diagnosisMasterSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_master_skip_count",
			Help: "Counter of diagnosis sync skip by kubediag master",
		},
	)
	diagnosisTotalCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_total_count",
			Help: "Counter of total diagnosis",
		},
	)
	diagnosisTotalSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_total_success_count",
			Help: "Counter of total success diagnosis",
		},
	)
	diagnosisTotalFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_total_fail_count",
			Help: "Counter of total fail diagnosis",
		},
	)
	diagnosisInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "diagnosis_info",
			Help: "Information about diagnosis",
		},
		[]string{"name", "operationset", "phase"},
	)
)

// Kubediag agent metrics
var (
	diagnosisAgentSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_agent_skip_count",
			Help: "Counter of diagnosis sync skip by agent",
		},
	)
	diagnosisAgentQueuedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_agent_queued_count",
			Help: "Counter of diagnosis sync queued by agent",
		},
	)
)

// DiagnosisReconciler reconciles a Diagnosis object.
type DiagnosisReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	eventRecorder record.EventRecorder

	nodeName   string
	executorCh chan diagnosisv1.Diagnosis
}

// NewDiagnosisReconciler creates a new DiagnosisReconciler.
func NewDiagnosisReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	eventRecorder record.EventRecorder,
	nodeName string,
	executorCh chan diagnosisv1.Diagnosis,
) *DiagnosisReconciler {
	metrics.Registry.MustRegister(
		diagnosisMasterSkipCount,
		diagnosisTotalCount,
		diagnosisTotalSuccessCount,
		diagnosisTotalFailCount,
		diagnosisInfo,
		diagnosisAgentSkipCount,
		diagnosisAgentQueuedCount,
	)

	return &DiagnosisReconciler{
		Client:        cli,
		Log:           log,
		Scheme:        scheme,
		eventRecorder: eventRecorder,
		nodeName:      nodeName,
		executorCh:    executorCh,
	}
}

// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=diagnoses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=diagnoses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=commonevents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=commonevents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile synchronizes a Diagnosis object according to the phase.
func (r *DiagnosisReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("diagnosis", req.NamespacedName)

	log.Info("reconciling Diagnosis")

	// Classify and calculate diagnosis according to the phase.
	r.collectDiagnosisMetricsWithPhase(ctx, log)

	var diagnosis diagnosisv1.Diagnosis
	if err := r.Get(ctx, req.NamespacedName, &diagnosis); err != nil {
		log.Error(err, "unable to fetch Diagnosis")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The master will process a diagnosis which is not found or completed, or has not been accept yet, while the agent will process
	// a diagnosis in Pending and Running phases.
	switch diagnosis.Status.Phase {
	case "":
		log.Info("diagnosis accepted by kubediag master", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		if diagnosis.Spec.TargetSelector == nil {
			log.Error(fmt.Errorf("target selector is empty"), "ignoring invalid Diagnosis")

			diagnosisMasterSkipCount.Inc()
			diagnosisTotalCount.Inc()

			diagnosis.Status.StartTime = metav1.Now()
			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				log.Error(err, "target selector not found")
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			return ctrl.Result{}, nil
		}

		diagnosis.Status.StartTime = metav1.Now()
		diagnosis.Status.Phase = diagnosisv1.DiagnosisPending
		if err := r.Status().Update(ctx, &diagnosis); err != nil {
			log.Error(err, "unable to update Diagnosis")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	case diagnosisv1.DiagnosisPending:
		// Set node names from node selector, pod selector and pod names.
		nodeNames := make([]string, 0)
		if diagnosis.Spec.TargetSelector.NodeSelector != nil {
			labelSelector, err := metav1.LabelSelectorAsSelector(diagnosis.Spec.TargetSelector.NodeSelector)
			if err != nil {
				log.Error(err, "unable to get node label selector")
				return ctrl.Result{}, err
			}

			var nodeList corev1.NodeList
			if err := r.List(ctx, &nodeList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
				log.Error(err, "unable to list Nodes")
				return ctrl.Result{}, err
			}

			for _, node := range nodeList.Items {
				nodeNames = append(nodeNames, node.Name)
			}
		} else if len(diagnosis.Spec.TargetSelector.NodeNames) != 0 {
			nodeNames = append(nodeNames, diagnosis.Spec.TargetSelector.NodeNames...)
		} else if diagnosis.Spec.TargetSelector.PodSelector != nil {
			labelSelector, err := metav1.LabelSelectorAsSelector(diagnosis.Spec.TargetSelector.NodeSelector)
			if err != nil {
				log.Error(err, "unable to get pod label selector")
				return ctrl.Result{}, err
			}

			var podList corev1.PodList
			if err := r.List(ctx, &podList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
				log.Error(err, "unable to list Pods")
				return ctrl.Result{}, err
			}

			for _, pod := range podList.Items {
				if pod.Spec.NodeName != "" {
					nodeNames = append(nodeNames, pod.Spec.NodeName)
				}
			}
		} else if len(diagnosis.Spec.TargetSelector.PodReferences) != 0 {
			for _, podReference := range diagnosis.Spec.TargetSelector.PodReferences {
				var pod corev1.Pod
				if err := r.Get(ctx, client.ObjectKey{
					Name:      podReference.Name,
					Namespace: podReference.Namespace,
				}, &pod); err != nil {
					log.Error(err, "unable to fetch Pod")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
				if pod.Spec.NodeName != "" {
					nodeNames = append(nodeNames, pod.Spec.NodeName)
				}
			}
		}
		// Deduplicate node names.
		nodeNames = util.RemoveDuplicateStrings(nodeNames)

		log.Info("diagnosis accepted by kubediag master", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		diagnosis.Status.Phase = diagnosisv1.DiagnosisRunning
		diagnosis.Status.NodeNames = nodeNames
		if diagnosis.Spec.Parameters != nil {
			diagnosis.Status.Context.Parameters = diagnosis.Spec.Parameters
		}
		if err := r.Status().Update(ctx, &diagnosis); err != nil {
			log.Error(err, "unable to update Diagnosis")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		diagnosisTotalCount.Inc()
	case diagnosisv1.DiagnosisRunning:
		log.Info("starting to run Diagnosis", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})

		// Fetch operationSet according to diagnosis.
		var operationset diagnosisv1.OperationSet
		err := r.Get(ctx, client.ObjectKey{
			Name: diagnosis.Spec.OperationSet,
		}, &operationset)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("operation set is not found", "operationset", diagnosis.Spec.OperationSet, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})

				r.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation set is not found", diagnosis.Namespace, diagnosis.Name)
				diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
				util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
					Type:    diagnosisv1.OperationSetNotFound,
					Status:  corev1.ConditionTrue,
					Reason:  "OperationSetNotFound",
					Message: fmt.Sprintf("OperationSet %s is not found", diagnosis.Spec.OperationSet),
				})
				if err := r.Status().Update(ctx, &diagnosis); err != nil {
					return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
				}
				diagnosisTotalFailCount.Inc()
				return ctrl.Result{}, nil
			}

			return ctrl.Result{}, err
		}

		// Validate the operation set is ready.
		if !operationset.Status.Ready {
			log.Info("the graph has not been updated according to the latest specification")

			r.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation set is not ready", diagnosis.Namespace, diagnosis.Name)
			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
			util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.OperationSetNotReady,
				Status:  corev1.ConditionTrue,
				Reason:  "OperationSetNotReady",
				Message: fmt.Sprintf("OperationSet %s is not ready because the graph has not been updated according to the latest specification", operationset.Name),
			})
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			diagnosisTotalFailCount.Inc()
			return ctrl.Result{}, nil
		}

		// Update hash value calculated from adjacency list of operation set.
		diagnosisLabels := diagnosis.GetLabels()
		if diagnosisLabels == nil {
			diagnosisLabels = make(map[string]string)
		}
		diagnosisAdjacencyListHash, ok := diagnosisLabels[util.OperationSetUniqueLabelKey]
		if !ok {
			diagnosisLabels[util.OperationSetUniqueLabelKey] = util.ComputeHash(operationset.Spec.AdjacencyList)
			diagnosis.SetLabels(diagnosisLabels)
			if err := r.Update(ctx, &diagnosis); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
			}

			log.Info("hash value of adjacency list calculated")
			return ctrl.Result{}, nil
		}

		// Validate the graph defined by operation set is not changed.
		operationSetLabels := operationset.GetLabels()
		if operationSetLabels == nil {
			operationSetLabels = make(map[string]string)
		}
		operationSetAdjacencyListHash := operationSetLabels[util.OperationSetUniqueLabelKey]
		if operationSetAdjacencyListHash != diagnosisAdjacencyListHash {
			log.Info("hash value caculated from adjacency list has been changed", "new", operationSetAdjacencyListHash, "old", diagnosisAdjacencyListHash)

			r.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation set has been changed during execution", diagnosis.Namespace, diagnosis.Name)
			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
			util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.OperationSetChanged,
				Status:  corev1.ConditionTrue,
				Reason:  "OperationSetChanged",
				Message: fmt.Sprintf("OperationSet %s specification has been changed during diagnosis execution", operationset.Name),
			})
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			diagnosisTotalFailCount.Inc()
			return ctrl.Result{}, nil
		}

		// Set initial checkpoint before operation execution.
		if diagnosis.Status.Checkpoint == nil {
			diagnosis.Status.Checkpoint = &diagnosisv1.Checkpoint{
				PathIndex:         0,
				NodeIndex:         0,
				Desired:           0,
				Active:            0,
				Succeeded:         0,
				Failed:            0,
				SynchronizedTasks: []string{},
			}
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			diagnosisTotalFailCount.Inc()
			return ctrl.Result{}, nil
		}

		// Retrieve operation node information.
		checkpoint := diagnosis.Status.Checkpoint
		paths := operationset.Status.Paths
		if checkpoint.PathIndex >= len(paths) {
			return ctrl.Result{}, fmt.Errorf("invalid path index %d of length %d", checkpoint.PathIndex, len(paths))
		}
		path := paths[checkpoint.PathIndex]
		if checkpoint.NodeIndex >= len(path) {
			return ctrl.Result{}, fmt.Errorf("invalid node index %d of length %d", checkpoint.NodeIndex, len(path))
		}
		node := path[checkpoint.NodeIndex]

		// Set desired number of tasks.
		desired := diagnosis.Status.Checkpoint.Desired
		active := diagnosis.Status.Checkpoint.Active
		succeeded := diagnosis.Status.Checkpoint.Succeeded
		failed := diagnosis.Status.Checkpoint.Failed
		if diagnosis.Status.Checkpoint.Desired == 0 {
			diagnosis.Status.Checkpoint.Desired = len(diagnosis.Status.NodeNames)
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			return ctrl.Result{}, nil
		}

		// Create tasks for current checkpoint.
		if active+succeeded+failed != desired {
			for _, nodeName := range diagnosis.Status.NodeNames {
				log.Info("creating task", "task", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				}, "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				}, "operationset", operationset.Name, "node", node, "path", path)

				owner := []metav1.OwnerReference{
					{
						APIVersion: diagnosis.APIVersion,
						Kind:       diagnosis.Kind,
						Name:       diagnosis.Name,
						UID:        diagnosis.UID,
					},
				}

				task := diagnosisv1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:            diagnosis.Name + "." + string(diagnosis.UID)[0:8] + "." + nodeName + "." + strconv.Itoa(diagnosis.Status.Checkpoint.PathIndex) + "." + strconv.Itoa(diagnosis.Status.Checkpoint.NodeIndex) + "." + node.Operation,
						Namespace:       diagnosis.Namespace,
						OwnerReferences: owner,
					},
					Spec: diagnosisv1.TaskSpec{
						Operation: node.Operation,
						NodeName:  nodeName,
					},
				}

				taskLabels := make(map[string]string)
				taskLabels["diagnosis-namespace"] = diagnosis.Namespace
				taskLabels["diagnosis-name"] = diagnosis.Name
				task.SetLabels(taskLabels)

				if err := r.Create(ctx, &task); err != nil {
					if apierrors.IsAlreadyExists(err) {
						if task.Status.Phase == "" {
							task.Status.StartTime = metav1.Now()
							task.Status.Phase = diagnosisv1.TaskPending
							if err := r.Status().Update(ctx, &task); err != nil {
								log.Error(err, "1 unable to update Task")
								return ctrl.Result{}, client.IgnoreNotFound(err)
							}
						}
						continue
					} else {
						log.Error(err, "unable to create Task")
						return ctrl.Result{}, err
					}
				}
				task.Status.StartTime = metav1.Now()
				task.Status.Phase = diagnosisv1.TaskPending
				if err := r.Status().Update(ctx, &task); err != nil {
					log.Error(err, "2 unable to update Task")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
				active += 1
			}

			diagnosis.Status.Checkpoint.Active = active
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
			}

			return ctrl.Result{}, nil
		} else if succeeded+failed == desired && succeeded != 0 {
			// Set current path as succeeded path if current operation is succeeded.
			if diagnosis.Status.SucceededPath == nil {
				diagnosis.Status.SucceededPath = make(diagnosisv1.Path, 0, len(path))
			}
			diagnosis.Status.SucceededPath = append(diagnosis.Status.SucceededPath, node)

			// Set phase to succeeded if current path has been finished and all operations are succeeded.
			if checkpoint.NodeIndex == len(path)-1 {
				log.Info("running diagnosis successfully", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
				r.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "DiagnosisSucceeded", "Running %s/%s diagnosis successfully", diagnosis.Namespace, diagnosis.Name)

				util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
					Type:    diagnosisv1.DiagnosisComplete,
					Status:  corev1.ConditionTrue,
					Reason:  "DiagnosisComplete",
					Message: fmt.Sprintf("Diagnosis is completed"),
				})
				diagnosis.Status.Phase = diagnosisv1.DiagnosisSucceeded
				if err := r.Status().Update(ctx, &diagnosis); err != nil {
					return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
				}
				return ctrl.Result{}, nil
			}

			// Increment node index if path has remaining operations to executed.
			checkpoint.NodeIndex++
			checkpoint.Active = 0
			checkpoint.Desired = 0
			checkpoint.Succeeded = 0
			checkpoint.Failed = 0
			checkpoint.SynchronizedTasks = []string{}
		} else if failed == desired {
			log.Info("failed to execute operation", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			}, "operationset", operationset.Name, "node", node, "path", path)
			r.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "OperationFailed", "Failed to execute operation %s", node.Operation)

			// Set current path as failed path and clear succeeded path if current operation is failed.
			if diagnosis.Status.FailedPaths == nil {
				diagnosis.Status.FailedPaths = make([]diagnosisv1.Path, 0, len(paths))
			}
			diagnosis.Status.FailedPaths = append(diagnosis.Status.FailedPaths, path)
			diagnosis.Status.SucceededPath = nil

			// Set phase to failed if all paths are failed.
			if checkpoint.PathIndex == len(paths)-1 {
				log.Info("failed to run diagnosis", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
				r.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s", diagnosis.Namespace, diagnosis.Name)
				diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
				if err := r.Status().Update(ctx, &diagnosis); err != nil {
					return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
				}
				return ctrl.Result{}, nil
			}

			// Increment path index if paths has remaining paths to executed.
			checkpoint.PathIndex++
			checkpoint.NodeIndex = 0
			checkpoint.Active = 0
			checkpoint.Desired = 0
			checkpoint.Succeeded = 0
			checkpoint.Failed = 0
			checkpoint.SynchronizedTasks = []string{}
		}

		if err := r.Status().Update(ctx, &diagnosis); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
		}

		return ctrl.Result{}, nil
	case diagnosisv1.DiagnosisFailed:
		diagnosisTotalFailCount.Inc()
	case diagnosisv1.DiagnosisSucceeded:
		diagnosisTotalSuccessCount.Inc()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager setups DiagnosisReconciler with the provided manager.
func (r *DiagnosisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Diagnosis{}).
		Complete(r)
}

func (r *DiagnosisReconciler) collectDiagnosisMetricsWithPhase(ctx context.Context, log logr.Logger) {
	var diagnosisList diagnosisv1.DiagnosisList
	if err := r.List(ctx, &diagnosisList); err != nil {
		log.Error(err, "error in collect diagnosis metrics")
		return
	}

	diagnosisInfo.Reset()
	for _, diag := range diagnosisList.Items {
		diagnosisInfo.WithLabelValues(diag.Name, diag.Spec.OperationSet, string(diag.Status.Phase)).Set(1)
	}
}
