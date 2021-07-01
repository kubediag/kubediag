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
	"strconv"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/util"
)

var (
	operationsetInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "operationset_info",
			Help: "Information about operationset",
		},
		[]string{"name", "ready"},
	)
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
	metrics.Registry.MustRegister(
		operationsetInfo,
	)
	return &OperationSetReconciler{
		Client:         cli,
		Log:            log,
		Scheme:         scheme,
		graphBuilderCh: graphBuilderCh,
	}
}

// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=operationsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=operationsets/status,verbs=get;update;patch

// Reconcile synchronizes a OperationSet object.
func (r *OperationSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("operationSet", req.NamespacedName)

	log.Info("reconciling OperationSet")
	r.collectOperationsetMetrics(ctx, log)

	var operationSet diagnosisv1.OperationSet
	if err := r.Get(ctx, req.NamespacedName, &operationSet); err != nil {
		log.Error(err, "unable to fetch OperationSet")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Update ready status and hash value calculated from adjacency list on specification change.
	labels := operationSet.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	oldAdjacencyListHash := labels[util.OperationSetUniqueLabelKey]
	newAdjacencyListHash := util.ComputeHash(operationSet.Spec.AdjacencyList)
	if oldAdjacencyListHash != newAdjacencyListHash {
		log.Info("hash value caculated from adjacency list has been changed", "new", newAdjacencyListHash, "old", oldAdjacencyListHash)

		// Set ready status to false if hash value is changed.
		if operationSet.Status.Ready {
			operationSet.Status.Ready = false
			if err := r.Status().Update(ctx, &operationSet); err != nil {
				log.Error(err, "unable to update OperationSet")
				return ctrl.Result{}, err
			}
		}

		labels[util.OperationSetUniqueLabelKey] = newAdjacencyListHash
		operationSet.SetLabels(labels)
		if err := r.Update(ctx, &operationSet); err != nil {
			log.Error(err, "unable to update OperationSet")
			return ctrl.Result{}, err
		}
	}

	if !operationSet.Status.Ready {
		err := util.QueueOperationSet(ctx, r.graphBuilderCh, operationSet)
		if err != nil {
			log.Error(err, "failed to send operationSet to graph builder queue")
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

func (r *OperationSetReconciler) collectOperationsetMetrics(ctx context.Context, log logr.Logger) {
	var operationsetList diagnosisv1.OperationSetList
	err := r.Client.List(ctx, &operationsetList)
	if err != nil {
		log.Error(err, "Error in collect Operationset metrics")
		return
	}

	operationsetInfo.Reset()
	for _, ops := range operationsetList.Items {
		operationsetInfo.WithLabelValues(ops.Name, strconv.FormatBool(ops.Status.Ready)).Set(1)
	}
	log.Info("Collected operationset metrics.")
}
