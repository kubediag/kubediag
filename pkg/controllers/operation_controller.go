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
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
)

var (
	operationInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "operation_info",
			Help: "Information about Operation",
		},
		[]string{"name"},
	)
)

// OperationReconciler reconciles a Operation object.
type OperationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func NewOperationReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
) *OperationReconciler {
	metrics.Registry.MustRegister(
		operationInfo,
	)
	return &OperationReconciler{
		Client: cli,
		Log:    log,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=Operations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=Operations/status,verbs=get;update;patch

func (r *OperationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("operation", req.NamespacedName)
	r.collectOperationMetrics(ctx, log)

	return ctrl.Result{}, nil
}

func (r *OperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Operation{}).
		Complete(r)
}

func (r *OperationReconciler) collectOperationMetrics(ctx context.Context, log logr.Logger) {
	var operationList diagnosisv1.OperationList
	err := r.Client.List(ctx, &operationList)
	if err != nil {
		log.Error(err, "Error in collect Operation metrics")
		return
	}

	operationInfo.Reset()
	for _, op := range operationList.Items {
		operationInfo.WithLabelValues(op.Name).Set(1)
	}
	log.Info("Collected operation metrics.")

}
