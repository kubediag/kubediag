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

package v1

import (
	"net"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// diagnoserlog is for logging of diagnoser webhook.
var diagnoserlog = logf.Log.WithName("diagnoser-webhook")

// SetupWebhookWithManager setups the Diagnoser webhook.
func (r *Diagnoser) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-netease-com-v1-diagnoser,mutating=true,failurePolicy=fail,groups=diagnosis.netease.com,resources=diagnosers,verbs=create;update,versions=v1,name=mdiagnoser.kb.io

var _ webhook.Defaulter = &Diagnoser{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Diagnoser) Default() {
	abnormallog.Info("defaulting Diagnoser", "diagnoser", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	if r.Spec.Scheme == "" {
		r.Spec.Scheme = "http"
	}
	if r.Spec.TimeoutSeconds == 0 {
		r.Spec.TimeoutSeconds = 30
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-netease-com-v1-diagnoser,mutating=false,failurePolicy=fail,groups=diagnosis.netease.com,resources=diagnosers,versions=v1,name=vdiagnoser.kb.io

var _ webhook.Validator = &Diagnoser{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Diagnoser) ValidateCreate() error {
	diagnoserlog.Info("validating creation of Diagnoser", "diagnoser", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateDiagnoser()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Diagnoser) ValidateUpdate(old runtime.Object) error {
	diagnoserlog.Info("validating update of Diagnoser", "diagnoser", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateDiagnoser()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Diagnoser) ValidateDelete() error {
	diagnoserlog.Info("validating deletion of Diagnoser", "diagnoser", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return nil
}

// validateDiagnoser validates Diagnoser and returns err if any invalidation is found.
func (r *Diagnoser) validateDiagnoser() error {
	var allErrs field.ErrorList

	if net.ParseIP(r.Spec.IP) == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("ip"),
			r.Spec.IP, "must be valid ip"))
	}
	if r.Spec.Port < 1 || 65535 < r.Spec.Port {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("port"),
			r.Spec.Port, "must be valid port"))
	}
	if r.Spec.Scheme != "http" && r.Spec.Scheme != "https" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("scheme"),
			r.Spec.Scheme, "must be either http or https"))
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.netease.com", Kind: "Diagnoser"},
		r.Name, allErrs)
}
