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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

// OperationSetReconciler reconciles a OperationSet object.
type OperationSetReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	graphBuilderCh chan diagnosisv1.OperationSet
}

// NewOperationSetReconciler creates a new OperationSetReconciler.
func NewOperationSetReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	graphBuilderCh chan diagnosisv1.OperationSet,
) *OperationSetReconciler {
	return &OperationSetReconciler{
		Client:         cli,
		Log:            log,
		Scheme:         scheme,
		graphBuilderCh: graphBuilderCh,
	}
}

// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=operationsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=operationsets/status,verbs=get;update;patch

// Reconcile synchronizes a OperationSet object.
func (r *OperationSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("operationSet", req.NamespacedName)

	log.Info("reconciling OperationSet")

	var err error
	var operationSet diagnosisv1.OperationSet
	if err = r.Get(ctx, req.NamespacedName, &operationSet); err != nil {
		log.Error(err, "unable to fetch OperationSet")
		// Handle of case 'NotFound':
		if apierrors.IsNotFound(err) {
			// 1. Clean finalizers in Operation with this OperationSet.
			log.Info("clean finalizers in Operation with this OperationSet", "OperationSet", req.Name)
			if err = r.cleanFinalizerWithOperationSet(ctx, req.Name); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle of case 'Terminating':
	// Remove finalizers of deleted diagnoses if operationSet in terminating
	if !operationSet.DeletionTimestamp.IsZero() && len(operationSet.GetFinalizers()) > 0 {
		err = r.cleanExpiredFinalizers(ctx, operationSet, log)
		return ctrl.Result{}, err
	}

	// Handle of other cases:
	// reconcile related Operations' finalizers, if abnormal, this Diagnosis can not work.
	abnormal, err := r.reconcileOperation(ctx, operationSet, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	if abnormal {
		log.Info("OperationSet use an inactive or no-exist Operation", "OperationSet", req.Name)
		operationSet.Status.Ready = false
		return ctrl.Result{}, r.Status().Update(ctx, &operationSet, &client.UpdateOptions{})
	}

	// fixme: Whether OperationSet's spec changed or not, we will always build graph.
	err = util.QueueOperationSet(ctx, r.graphBuilderCh, operationSet)
	if err != nil {
		log.Error(err, "failed to send operationSet to graph builder queue")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager setups OperationSetReconciler with the provided manager.
func (r *OperationSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.OperationSet{}).
		Complete(r)
}

// cleanExpiredFinalizers delete the element in the finalizers of this OperationSet that points to a deleted Diagnosis object.
func (r *OperationSetReconciler) cleanExpiredFinalizers(ctx context.Context, operationSet diagnosisv1.OperationSet, log logr.Logger) error {
	finalizers := operationSet.GetFinalizers()
	remainingFinalizers := []string{}
	var diagnosis diagnosisv1.Diagnosis
	for _, finalizer := range finalizers {
		// finalizer with a non-namespaced-name will not be saved
		namespacedName, err := util.StringToNamespacedName(finalizer)
		if err != nil {
			continue
		}
		// finalizer with a non-exist-diagnosis will not be saved
		if err := r.Get(ctx, namespacedName, &diagnosis); apierrors.IsNotFound(err) {
			continue
		}
		remainingFinalizers = append(remainingFinalizers, finalizer)
	}

	if len(remainingFinalizers) < len(finalizers) {
		operationSet.SetFinalizers(remainingFinalizers)
		if err := r.Update(ctx, &operationSet); err != nil {
			log.Error(err, "unable to update OperationSet")
			return client.IgnoreNotFound(err)
		}
	}
	return nil

}

// cleanFinalizerWithOperationSet clean finalizers point to this OperationSet object in all Operations.
func (r *OperationSetReconciler) cleanFinalizerWithOperationSet(ctx context.Context, operationSetName string) error {
	operationList := diagnosisv1.OperationList{}
	err := r.List(ctx, &operationList, &client.ListOptions{})
	if err != nil {
		return err
	}
	for _, operation := range operationList.Items {
		if util.HasFinalizer(operation.GetFinalizers(), operationSetName) {
			cleanedUpFinalizers := util.RemoveFinalizer(operation.GetFinalizers(), operationSetName)
			operation.SetFinalizers(cleanedUpFinalizers)
			err = r.Update(ctx, &operation, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// reconcileOperation check and update related Operations' finalizer.
// If any related Operation object is terminating or not found, return a 'true' means abnormal.
func (r *OperationSetReconciler) reconcileOperation(ctx context.Context, operationSet diagnosisv1.OperationSet, log logr.Logger) (bool, error) {
	operationList := diagnosisv1.OperationList{}
	err := r.List(ctx, &operationList, &client.ListOptions{})
	if err != nil {
		return false, err
	}
	appendList, removeList, err := checkOperationAndMarkFinalizer(operationSet, operationList)
	if err != nil {
		// there must be at least one operation
		log.Error(err, "some dependent Operations are terminating or not found")
		return true, nil
	}
	for _, appendOperation := range appendList {
		_, newFinalizers := util.AppendFinalizerIfNotExist(appendOperation.Finalizers, operationSet.Name)
		appendOperation.SetFinalizers(newFinalizers)
		err = r.Update(ctx, &appendOperation, &client.UpdateOptions{})
		if err != nil {
			log.Error(err, "failed to update operation to append finalizers", "Operation", appendOperation.Name, "OperationSet", operationSet.Name)
			return false, err
		}
	}
	for _, removeOperation := range removeList {
		newFinalizers := util.RemoveFinalizer(removeOperation.Finalizers, operationSet.Name)
		removeOperation.SetFinalizers(newFinalizers)
		err = r.Update(ctx, &removeOperation, &client.UpdateOptions{})
		if err != nil {
			log.Error(err, "failed to update operation to remove finalizers", "Operation", removeOperation.Name, "OperationSet", operationSet.Name)
			return false, err
		}
	}
	return false, nil
}

// checkOperationAndMarkFinalizer will check each Operation in cluster to confirm:
// 1. All related Operations are exist, otherwise return error;
// 2. All related active Operations without finalizer mention to this OperationSet will add to appendList;
// 3. All no-related Operations with finalizer mention to this OperationSet will add to removeList;
func checkOperationAndMarkFinalizer(operationSet diagnosisv1.OperationSet, operationList diagnosisv1.OperationList) ([]diagnosisv1.Operation, []diagnosisv1.Operation, error) {
	nodesHealthMap := make(map[string]bool)
	for _, node := range operationSet.Spec.AdjacencyList {
		if node.Operation == "" {
			continue
		}
		nodesHealthMap[node.Operation] = false
	}
	var err error
	var appendList, removeList []diagnosisv1.Operation
	for _, operation := range operationList.Items {
		if _, exist := nodesHealthMap[operation.Name]; exist {
			if operation.DeletionTimestamp.IsZero() {
				nodesHealthMap[operation.Name] = true
				if !util.HasFinalizer(operation.Finalizers, operationSet.Name) {
					appendList = append(appendList, operation)
				}
			}
		} else {
			if util.HasFinalizer(operation.Finalizers, operationSet.Name) {
				removeList = append(removeList, operation)
			}
		}
	}

	for node, ok := range nodesHealthMap {
		if !ok {
			err = fmt.Errorf("operation: %s is inactive or dispeared in cluster", node)
			break
		}
	}
	return appendList, removeList, err
}
