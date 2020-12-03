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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// abnormallog is for logging of abnormal webhook.
var abnormallog = logf.Log.WithName("abnormal-webhook")

// SetupWebhookWithManager setups the Abnormal webhook.
func (r *Abnormal) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-netease-com-v1-abnormal,mutating=true,failurePolicy=fail,groups=diagnosis.netease.com,resources=abnormals,verbs=create;update,versions=v1,name=mabnormal.kb.io

var _ webhook.Defaulter = &Abnormal{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Abnormal) Default() {
	abnormallog.Info("defaulting Abnormal", "abnormal", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	if r.Spec.Source == "" {
		r.Spec.Source = CustomSource
	}
	if r.Spec.CommandExecutors != nil {
		for i, commandExecutor := range r.Spec.CommandExecutors {
			if commandExecutor.TimeoutSeconds == 0 {
				r.Spec.CommandExecutors[i].TimeoutSeconds = 30
			}
		}
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-netease-com-v1-abnormal,mutating=false,failurePolicy=fail,groups=diagnosis.netease.com,resources=abnormals,versions=v1,name=vabnormal.kb.io

var _ webhook.Validator = &Abnormal{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Abnormal) ValidateCreate() error {
	abnormallog.Info("validating creation of Abnormal", "abnormal", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateAbnormal()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Abnormal) ValidateUpdate(old runtime.Object) error {
	abnormallog.Info("validating update of Abnormal", "abnormal", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateAbnormal()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Abnormal) ValidateDelete() error {
	abnormallog.Info("validating deletion of Abnormal", "abnormal", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return nil
}

// validateAbnormal validates Abnormal and returns err if any invalidation is found.
func (r *Abnormal) validateAbnormal() error {
	var allErrs field.ErrorList

	if r.Spec.NodeName == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("nodeName"),
			r.Spec.NodeName, "must not be empty"))
	}
	if r.Spec.CommandExecutors != nil {
		for i, commandExecutor := range r.Spec.CommandExecutors {
			if commandExecutor.Type != InformationCollectorType &&
				commandExecutor.Type != DiagnoserType &&
				commandExecutor.Type != RecovererType {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("commandExecutors").Index(i).Child("type"),
					r.Spec.CommandExecutors[i].Type, "must be InformationCollector, Diagnoser or Recoverer"))
			}
			if commandExecutor.TimeoutSeconds <= 0 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("commandExecutors").Index(i).Child("timeoutSeconds"),
					r.Spec.CommandExecutors[i].TimeoutSeconds, "must be more than 0"))
			}
		}
	}
	if r.Spec.Profilers != nil {
		for i, profilerSpec := range r.Spec.Profilers {
			if profilerSpec.Type != InformationCollectorType &&
				profilerSpec.Type != DiagnoserType &&
				profilerSpec.Type != RecovererType {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profilers").Index(i).Child("type"),
					r.Spec.Profilers[i].Type, "must be InformationCollector, Diagnoser or Recoverer"))
			}
			if profilerSpec.TimeoutSeconds <= 0 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profilers").Index(i).Child("timeoutSeconds"),
					r.Spec.Profilers[i].TimeoutSeconds, "must be more than 0"))
			}
		}
	}
	if r.Status.CommandExecutors != nil {
		for i, commandExecutor := range r.Status.CommandExecutors {
			if commandExecutor.Type != InformationCollectorType &&
				commandExecutor.Type != DiagnoserType &&
				commandExecutor.Type != RecovererType {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("commandExecutors").Index(i).Child("type"),
					r.Status.CommandExecutors[i].Type, "must be InformationCollector, Diagnoser or Recoverer"))
			}
			if commandExecutor.TimeoutSeconds <= 0 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("commandExecutors").Index(i).Child("timeoutSeconds"),
					r.Status.CommandExecutors[i].TimeoutSeconds, "must be more than 0"))
			}
		}
	}
	if r.Status.Profilers != nil {
		for i, profilerStatus := range r.Status.Profilers {
			if profilerStatus.Type != InformationCollectorType &&
				profilerStatus.Type != DiagnoserType &&
				profilerStatus.Type != RecovererType {
				allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("profilers").Index(i).Child("type"),
					r.Status.Profilers[i].Type, "must be InformationCollector, Diagnoser or Recoverer"))
			}
		}
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.netease.com", Kind: "Abnormal"},
		r.Name, allErrs)
}
