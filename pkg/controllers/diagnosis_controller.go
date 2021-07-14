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

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	diagnosisMasterAssignNodeCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_master_assign_node_count",
			Help: "Counter of diagnosis sync by kubediag master",
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
	Log    logr.Logger
	Scheme *runtime.Scheme

	mode       string
	nodeName   string
	executorCh chan diagnosisv1.Diagnosis
}

// NewDiagnosisReconciler creates a new DiagnosisReconciler.
func NewDiagnosisReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	mode string,
	nodeName string,
	executorCh chan diagnosisv1.Diagnosis,
) *DiagnosisReconciler {
	if mode == "master" {
		metrics.Registry.MustRegister(
			diagnosisMasterSkipCount,
			diagnosisMasterAssignNodeCount,
			diagnosisTotalCount,
			diagnosisTotalSuccessCount,
			diagnosisTotalFailCount,
			diagnosisInfo,
		)
	} else if mode == "agent" {
		metrics.Registry.MustRegister(
			diagnosisAgentSkipCount,
			diagnosisAgentQueuedCount,
		)
	}

	return &DiagnosisReconciler{
		Client:     cli,
		Log:        log,
		Scheme:     scheme,
		mode:       mode,
		nodeName:   nodeName,
		executorCh: executorCh,
	}
}

// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=diagnoses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=diagnoses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=operations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
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

	// The master will process an diagnosis which is not found or completed, or has not been accept yet, while the agent will process
	// an diagnosis in Pending and Running phases.
	if r.mode == "master" {
		switch diagnosis.Status.Phase {
		case "":
			if diagnosis.Spec.NodeName == "" && diagnosis.Spec.PodReference == nil {
				// Ignore diagnosis if nodeName and podReference are both empty.
				log.Error(fmt.Errorf("nodeName and podReference are both empty"), "ignoring invalid Diagnosis")

				diagnosisMasterSkipCount.Inc()
				diagnosisTotalCount.Inc()

				return ctrl.Result{}, nil
			} else if diagnosis.Spec.NodeName == "" {
				// Set diagnosis NodeName if NodeName is empty and PodReference is not nil.
				var pod corev1.Pod
				if err := r.Get(ctx, client.ObjectKey{
					Name:      diagnosis.Spec.PodReference.Name,
					Namespace: diagnosis.Spec.PodReference.Namespace,
				}, &pod); err != nil {
					log.Error(err, "unable to fetch Pod")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				diagnosis.Spec.NodeName = pod.Spec.NodeName
				if err := r.Update(ctx, &diagnosis); err != nil {
					log.Error(err, "unable to update Diagnosis")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				diagnosisMasterAssignNodeCount.Inc()
			} else {
				log.Info("diagnosis accepted by kubediag master", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})

				diagnosis.Status.StartTime = metav1.Now()
				diagnosis.Status.Phase = diagnosisv1.DiagnosisPending
				if err := r.Status().Update(ctx, &diagnosis); err != nil {
					log.Error(err, "unable to update Diagnosis")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
				diagnosisTotalCount.Inc()
			}
		case diagnosisv1.DiagnosisFailed:
			diagnosisTotalFailCount.Inc()
		case diagnosisv1.DiagnosisSucceeded:
			diagnosisTotalSuccessCount.Inc()
		}
	} else if r.mode == "agent" {
		if !util.IsDiagnosisNodeNameMatched(diagnosis, r.nodeName) {
			return ctrl.Result{}, nil
		}

		switch diagnosis.Status.Phase {
		case diagnosisv1.DiagnosisPending:
			log.Info("diagnosis accepted by kubediag agent", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})

			diagnosis.Status.Phase = diagnosisv1.DiagnosisRunning
			util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.DiagnosisAccepted,
				Status:  corev1.ConditionTrue,
				Reason:  "DiagnosisAccepted",
				Message: fmt.Sprintf("Diagnosis is accepted by agent on node %s", diagnosis.Spec.NodeName),
			})
			if err := r.Status().Update(ctx, &diagnosis); err != nil {
				log.Error(err, "unable to update Diagnosis")
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			err := util.QueueDiagnosis(ctx, r.executorCh, diagnosis)
			if err != nil {
				log.Error(err, "failed to send diagnosis to executor queue")
			}
			diagnosisAgentQueuedCount.Inc()
		case diagnosisv1.DiagnosisRunning:
			err := util.QueueDiagnosis(ctx, r.executorCh, diagnosis)
			if err != nil {
				log.Error(err, "failed to send diagnosis to executor queue")
			}
			diagnosisAgentQueuedCount.Inc()
		case diagnosisv1.DiagnosisSucceeded:
			log.Info("ignoring Diagnosis in phase Succeeded")
		case diagnosisv1.DiagnosisFailed:
			log.Info("ignoring Diagnosis in phase Failed")
		case diagnosisv1.DiagnosisUnknown:
			log.Info("ignoring Diagnosis in phase Unknown")
		}
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
	if r.mode == "agent" {
		return
	}

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
