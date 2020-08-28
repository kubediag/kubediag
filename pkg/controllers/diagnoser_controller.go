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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
)

// DiagnoserReconciler reconciles a Diagnoser object
type DiagnoserReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	diagnoserChainCh chan diagnosisv1.Abnormal
}

func NewDiagnoserReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	diagnoserChainCh chan diagnosisv1.Abnormal,
) *DiagnoserReconciler {
	return &DiagnoserReconciler{
		Client:           cli,
		Log:              log,
		Scheme:           scheme,
		diagnoserChainCh: diagnoserChainCh,
	}
}

// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=diagnosers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=diagnosers/status,verbs=get;update;patch

func (r *DiagnoserReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *DiagnoserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Diagnoser{}).
		Complete(r)
}
