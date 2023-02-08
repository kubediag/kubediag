/*
Copyright 2022 The KubeDiag Authors.

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
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/util"
)

// TaskReconciler reconciles a Task object.
type TaskReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	nodeName   string
	executorCh chan diagnosisv1.Task
}

// NewTaskReconciler creates a new TaskReconciler.
func NewTaskReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	nodeName string,
	executorCh chan diagnosisv1.Task,
) *TaskReconciler {
	return &TaskReconciler{
		Client:     cli,
		Log:        log,
		Scheme:     scheme,
		nodeName:   nodeName,
		executorCh: executorCh,
	}
}

//+kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=tasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=tasks/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Task object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *TaskReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("task", req.NamespacedName)

	log.Info("reconciling Task")

	var task diagnosisv1.Task
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		log.Error(err, "unable to fetch Task")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The master will process a task which is not found or completed, or has not been accept yet, while the agent will process
	// a task in Pending and Running phases.
	if !util.IsTaskNodeNameMatched(task, r.nodeName) {
		return ctrl.Result{}, nil
	}

	switch task.Status.Phase {
	case diagnosisv1.TaskPending:
		log.Info("task accepted by kubediag agent", "task", client.ObjectKey{
			Name:      task.Name,
			Namespace: task.Namespace,
		})

		task.Status.Phase = diagnosisv1.TaskRunning
		util.UpdateTaskCondition(&task.Status, &diagnosisv1.TaskCondition{
			Type:    diagnosisv1.TaskAccepted,
			Status:  corev1.ConditionTrue,
			Reason:  "TaskAccepted",
			Message: fmt.Sprintf("Task is accepted by agent on node %s", task.Spec.NodeName),
		})
		if err := r.Status().Update(ctx, &task); err != nil {
			log.Error(err, "unable to update Task")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	case diagnosisv1.TaskRunning:
		err := util.QueueTask(ctx, r.executorCh, task)
		if err != nil {
			log.Error(err, "failed to send diagnosis to executor queue")
		}
		diagnosisAgentQueuedCount.Inc()
	case diagnosisv1.TaskSucceeded:
		diagnosisName := strings.Split(req.Name, ".")[0]
		var diagnosis diagnosisv1.Diagnosis
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace,
			Name:      diagnosisName,
		}, &diagnosis); err != nil {
			log.Error(err, "unable to fetch Diagnosis")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		if util.Contains(diagnosis.Status.Checkpoint.SynchronizedTasks, req.Name) {
			return ctrl.Result{}, nil
		}

		diagnosis.Status.Checkpoint.Active -= 1
		diagnosis.Status.Checkpoint.Succeeded += 1
		diagnosis.Status.Checkpoint.SynchronizedTasks = append(diagnosis.Status.Checkpoint.SynchronizedTasks, req.Name)

		if diagnosis.Status.Context == nil {
			diagnosis.Status.Context = new(diagnosisv1.DiagnosisContext)
			diagnosis.Status.Context.Operations = make(map[string]diagnosisv1.OperationContext, 0)
		}
		operationKey := strconv.Itoa(diagnosis.Status.Checkpoint.PathIndex) + "." + strconv.Itoa(diagnosis.Status.Checkpoint.NodeIndex) + "." + task.Spec.Operation
		operationValue, ok := diagnosis.Status.Context.Operations[operationKey]
		if ok {
			operationValue[task.Name] = task.Status.Results
		} else {
			operationValue = make(map[string]diagnosisv1.TaskContext, 0)
			operationValue[task.Name] = task.Status.Results
		}
		diagnosis.Status.Context.Operations[operationKey] = operationValue

		if err := r.Status().Update(ctx, &diagnosis); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
		}
		return ctrl.Result{}, nil
	case diagnosisv1.TaskFailed:
		diagnosisName := strings.Split(req.Name, ".")[0]
		var diagnosis diagnosisv1.Diagnosis
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace,
			Name:      diagnosisName,
		}, &diagnosis); err != nil {
			log.Error(err, "unable to fetch Diagnosis")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		if util.Contains(diagnosis.Status.Checkpoint.SynchronizedTasks, req.Name) {
			return ctrl.Result{}, nil
		}

		diagnosis.Status.Checkpoint.Active -= 1
		diagnosis.Status.Checkpoint.Failed += 1
		diagnosis.Status.Checkpoint.SynchronizedTasks = append(diagnosis.Status.Checkpoint.SynchronizedTasks, req.Name)

		if err := r.Status().Update(ctx, &diagnosis); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update Diagnosis: %s", err)
		}
		return ctrl.Result{}, nil
	case diagnosisv1.TaskUnknown:
		log.Info("ignoring Task in phase Unknown")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Task{}).
		Complete(r)
}
