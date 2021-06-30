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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubediag/kubediag/pkg/util"
)

// EventReconciler reconciles an Event object.
type EventReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	eventChainCh chan corev1.Event
}

// NewEventReconciler creates a new EventReconciler.
func NewEventReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	eventChainCh chan corev1.Event,
) *EventReconciler {
	return &EventReconciler{
		Client:       cli,
		Log:          log,
		Scheme:       scheme,
		eventChainCh: eventChainCh,
	}
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get;update;patch

// Reconcile synchronizes an Event object.
func (r *EventReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("event", req.NamespacedName)

	var event corev1.Event
	if err := r.Get(ctx, req.NamespacedName, &event); err != nil {
		log.Error(err, "unable to fetch Event")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err := util.QueueEvent(ctx, r.eventChainCh, event)
	if err != nil {
		log.Error(err, "failed to send event to event queue")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager setups EventReconciler with the provided manager.
func (r *EventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		Complete(r)
}
