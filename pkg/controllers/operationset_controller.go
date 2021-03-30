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
	log := r.Log.WithValues("operationset", req.NamespacedName)

	log.Info("reconciling OperationSet")

	var operationset diagnosisv1.OperationSet
	if err := r.Get(ctx, req.NamespacedName, &operationset); err != nil {
		log.Error(err, "unable to fetch OperationSet")

		// TODO: Remove finalizers of referenced Operations.

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Remove finalizers of deleted diagnoses.
	finalizers := operationset.GetFinalizers()
	if operationset.DeletionTimestamp != nil && len(finalizers) != 0 {
		for _, finalizer := range finalizers {
			namespacedName, err := util.StringToNamespacedName(finalizer)
			if err != nil {
				finalizers := util.RemoveFinalizer(operationset.GetFinalizers(), finalizer)
				operationset.SetFinalizers(finalizers)
				continue
			}

			var diagnosis diagnosisv1.Diagnosis
			if err := r.Get(ctx, namespacedName, &diagnosis); apierrors.IsNotFound(err) {
				finalizers := util.RemoveFinalizer(operationset.GetFinalizers(), finalizer)
				operationset.SetFinalizers(finalizers)
			}
		}

		if err := r.Update(ctx, &operationset); err != nil {
			log.Error(err, "unable to update OperationSet")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	// TODO: Update OperationSet on specification change.
	if !operationset.Status.Ready {
		err := util.QueueOperationSet(ctx, r.graphBuilderCh, operationset)
		if err != nil {
			log.Error(err, "failed to send operationset to graph builder queue")
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager setups OperationSetReconciler with the provided manager.
func (r *OperationSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.OperationSet{}).
		Complete(r)
}
