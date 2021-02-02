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
	"path/filepath"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// diagnosislog is for logging of diagnosis webhook.
var diagnosislog = logf.Log.WithName("diagnosis-webhook")

// SetupWebhookWithManager setups the Diagnosis webhook.
func (r *Diagnosis) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-netease-com-v1-diagnosis,mutating=true,failurePolicy=fail,groups=diagnosis.netease.com,resources=diagnoses,verbs=create;update,versions=v1,name=mdiagnosis.kb.io

var _ webhook.Defaulter = &Diagnosis{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Diagnosis) Default() {
	diagnosislog.Info("defaulting Diagnosis", "diagnosis", client.ObjectKey{
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
	if r.Spec.Profilers != nil {
		for i, profiler := range r.Spec.Profilers {
			if profiler.TimeoutSeconds == 0 {
				r.Spec.Profilers[i].TimeoutSeconds = 30
			}
			if profiler.ExpirationSeconds == 0 {
				r.Spec.Profilers[i].ExpirationSeconds = 7200
			}
		}
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-netease-com-v1-diagnosis,mutating=false,failurePolicy=fail,groups=diagnosis.netease.com,resources=diagnoses,versions=v1,name=vdiagnosis.kb.io

var _ webhook.Validator = &Diagnosis{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Diagnosis) ValidateCreate() error {
	diagnosislog.Info("validating creation of Diagnosis", "diagnosis", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateDiagnosis()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Diagnosis) ValidateUpdate(old runtime.Object) error {
	diagnosislog.Info("validating update of Diagnosis", "diagnosis", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateDiagnosis()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Diagnosis) ValidateDelete() error {
	diagnosislog.Info("validating deletion of Diagnosis", "diagnosis", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return nil
}

// validateDiagnosis validates Diagnosis and returns err if any invalidation is found.
func (r *Diagnosis) validateDiagnosis() error {
	var allErrs field.ErrorList

	if r.Spec.NodeName == "" && r.Spec.PodReference == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("nodeName"),
			r.Spec.NodeName, "must not be empty if podReference is empty"))
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
			if profilerSpec.Go == nil && profilerSpec.Java == nil ||
				profilerSpec.Go != nil && profilerSpec.Java != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profilers").Index(i),
					r.Spec.Profilers[i], "must specify one and only one of the programming languages"))
			}
			if profilerSpec.Type != InformationCollectorType &&
				profilerSpec.Type != DiagnoserType &&
				profilerSpec.Type != RecovererType {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profilers").Index(i).Child("type"),
					r.Spec.Profilers[i].Type, "must be InformationCollector, Diagnoser or Recoverer"))
			}
			if profilerSpec.Java != nil {
				if profilerSpec.Java.Type != ArthasJavaProfilerType &&
					profilerSpec.Java.Type != MemoryAnalyzerJavaProfilerType {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profilers").Index(i).Child("java").Child("type"),
						r.Spec.Profilers[i].Java.Type, "must be Arthas or MemoryAnalyzer"))
				}
				if profilerSpec.Java.Type == MemoryAnalyzerJavaProfilerType {
					if !filepath.IsAbs(profilerSpec.Java.HPROFFilePath) {
						allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profilers").Index(i).Child("java").Child("hprofFilePath"),
							r.Spec.Profilers[i].Java.HPROFFilePath, "must be an absolute path"))
					}
				}
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
		schema.GroupKind{Group: "diagnosis.netease.com", Kind: "Diagnosis"},
		r.Name, allErrs)
}
