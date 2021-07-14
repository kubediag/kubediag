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

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

var (
	triggerInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trigger_info",
			Help: "Information about trigger",
		},
		[]string{"name"},
	)
)

// TriggerReconciler reconciles a Trigger object.
type TriggerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewTriggerReconciler creates a new TriggerReconciler.
func NewTriggerReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
) *TriggerReconciler {
	metrics.Registry.MustRegister(triggerInfo)
	return &TriggerReconciler{
		Client: cli,
		Log:    log,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=triggers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=triggers/status,verbs=get;update;patch

// Reconcile synchronizes a Trigger object.
func (r *TriggerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("trigger", req.NamespacedName)
	r.collectTriggerMetrics(ctx, log)

	return ctrl.Result{}, nil
}

// SetupWithManager setups TriggerReconciler with the provided manager.
func (r *TriggerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Trigger{}).
		Complete(r)
}

func (r *TriggerReconciler) collectTriggerMetrics(ctx context.Context, log logr.Logger) {
	var triggerList diagnosisv1.TriggerList
	err := r.Client.List(ctx, &triggerList)
	if err != nil {
		log.Error(err, "error in collect trigger metrics")
		return
	}

	triggerInfo.Reset()
	for _, tg := range triggerList.Items {
		triggerInfo.WithLabelValues(tg.Name).Set(1)
	}
	log.Info("collected trigger metrics.")
}
