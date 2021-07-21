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

package v1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var triggerlog = logf.Log.WithName("trigger-webhook")

// SetupWebhookWithManager setups the Trigger webhook.
func (r *Trigger) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-kubediag-org-v1-trigger,mutating=true,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=triggers,verbs=create;update,versions=v1,name=mtrigger.kb.io

var _ webhook.Defaulter = &Trigger{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Trigger) Default() {
	triggerlog.Info("defaulting Trigger", "trigger", r.Name)
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-kubediag-org-v1-trigger,mutating=false,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=triggers,versions=v1,name=vtrigger.kb.io

var _ webhook.Validator = &Trigger{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Trigger) ValidateCreate() error {
	triggerlog.Info("validating creation of Trigger", "trigger", r.Name)

	return r.validateTrigger()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Trigger) ValidateUpdate(old runtime.Object) error {
	triggerlog.Info("validating update of Trigger", "trigger", r.Name)

	return r.validateTrigger()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Trigger) ValidateDelete() error {
	triggerlog.Info("validating deletion of Trigger", "trigger", r.Name)

	return nil
}

// validateTrigger validates Trigger and returns err if any invalidation is found.
func (r *Trigger) validateTrigger() error {
	var allErrs field.ErrorList

	if r.Spec.OperationSet == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("operationSet"),
			r.Spec.OperationSet, "must not be empty"))
	}
	if r.Spec.SourceTemplate.PrometheusAlertTemplate == nil && r.Spec.SourceTemplate.KubernetesEventTemplate == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("sourceTemplate"),
			r.Spec.SourceTemplate, "must specify either prometheus alert template or kubernetes event template"))
	} else if r.Spec.SourceTemplate.PrometheusAlertTemplate != nil && r.Spec.SourceTemplate.KubernetesEventTemplate != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("sourceTemplate"),
			r.Spec.SourceTemplate, "one and only one template should be specified."))
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.kubediag.org", Kind: "Trigger"},
		r.Name, allErrs)
}
