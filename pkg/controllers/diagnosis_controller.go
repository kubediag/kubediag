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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
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
	return &DiagnosisReconciler{
		Client:     cli,
		Log:        log,
		Scheme:     scheme,
		mode:       mode,
		nodeName:   nodeName,
		executorCh: executorCh,
	}
}

// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=diagnoses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=diagnoses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=operations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch

// Reconcile synchronizes a Diagnosis object according to the phase.
func (r *DiagnosisReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("diagnosis", req.NamespacedName)

	log.Info("reconciling Diagnosis")

	var diagnosis diagnosisv1.Diagnosis
	var err error
	if err = r.Get(ctx, req.NamespacedName, &diagnosis); err != nil {
		log.Error(err, "unable to fetch Diagnosis")
		// Handle of case 'NotFound': clean finalizers in OperationSet with this Diagnosis.
		if errors.IsNotFound(err) {
			log.Info("clean finalizers in OperationSet with this Diagnosis", "Diagnosis", req.NamespacedName)
			err = r.cleanFinalizerWithDiagnosis(ctx, req.NamespacedName.String())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The master will process an diagnosis which has not been accept yet, while the agent will process
	// an diagnosis in Pending and Running phases.
	if r.mode == "master" {
		switch diagnosis.Status.Phase {
		case "":
			// Handle of other cases:
			// reconcile related OperationSet's finalizer, if abnormal, this Diagnosis can not work.
			dependentOperationSetAbnormal, err := r.reconcileOperationSet(ctx, req.NamespacedName.String(), diagnosis.Spec.OperationSet, log)
			if err != nil {
				return ctrl.Result{}, err
			}
			if dependentOperationSetAbnormal {
				log.Info("Diagnosis use an inactive or no-exist OperationSet", "Diagnosis", req.NamespacedName, "OperationSet", diagnosis.Spec.OperationSet)
				diagnosis.Status.StartTime = metav1.Now()
				diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
				return ctrl.Result{}, r.Status().Update(ctx, &diagnosis, &client.UpdateOptions{})
			}

			if diagnosis.Spec.NodeName == "" && diagnosis.Spec.PodReference == nil {
				// Ignore diagnosis if nodeName and podReference are both empty.
				log.Error(fmt.Errorf("nodeName and podReference are both empty"), "ignoring invalid Diagnosis")
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
			} else {
				log.Info("diagnosis accepted by kube diagnoser master", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})

				diagnosis.Status.StartTime = metav1.Now()
				diagnosis.Status.Phase = diagnosisv1.DiagnosisPending
				if err := r.Status().Update(ctx, &diagnosis); err != nil {
					log.Error(err, "unable to update Diagnosis")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
			}
		}
	} else if r.mode == "agent" {
		if !util.IsDiagnosisNodeNameMatched(diagnosis, r.nodeName) {
			return ctrl.Result{}, nil
		}

		switch diagnosis.Status.Phase {
		case diagnosisv1.DiagnosisPending:
			log.Info("diagnosis accepted by kube diagnoser agent", "diagnosis", client.ObjectKey{
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
		case diagnosisv1.DiagnosisRunning:
			err := util.QueueDiagnosis(ctx, r.executorCh, diagnosis)
			if err != nil {
				log.Error(err, "failed to send diagnosis to executor queue")
			}
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

// reconcileOperationSet check and update related OperationSet's finalizer.
// If OperationSet object is terminating or not found, return a 'true' means abnormal.
func (r *DiagnosisReconciler) reconcileOperationSet(ctx context.Context, diagnosisName, operationSetName string, log logr.Logger) (bool, error) {
	var operationSet diagnosisv1.OperationSet
	if err := r.Get(ctx, client.ObjectKey{Name: operationSetName}, &operationSet); err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}
		return true, nil
	}
	if !operationSet.DeletionTimestamp.IsZero() || !operationSet.Status.Ready {
		return true, nil
	}
	append, completedFinalizers := util.AppendFinalizerIfNotExist(operationSet.GetFinalizers(), diagnosisName)
	if append {
		operationSet.SetFinalizers(completedFinalizers)
		if err := r.Update(ctx, &operationSet); err != nil {
			log.Error(err, "unable to update OperationSet", "OperationSet", operationSetName)
			return false, err
		}
	}
	return false, nil
}

// cleanFinalizerWithDiagnosis clean finalizers point to this Diagnosis object in all OperationSets.
func (r *DiagnosisReconciler) cleanFinalizerWithDiagnosis(ctx context.Context, diagnosisName string) error {
	operationSetList := diagnosisv1.OperationSetList{}
	err := r.List(ctx, &operationSetList, &client.ListOptions{})
	if err != nil {
		return err
	}
	for index := range operationSetList.Items {
		os := operationSetList.Items[index]
		if util.HasFinalizer(os.GetFinalizers(), diagnosisName) {
			cleanedUpFinalizers := util.RemoveFinalizer(os.GetFinalizers(), diagnosisName)
			os.SetFinalizers(cleanedUpFinalizers)
			err = r.Update(ctx, &os, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
