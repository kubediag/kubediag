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

// +kubebuilder:webhook:path=/mutate-diagnosis-kubediag-org-v1-diagnosis,mutating=true,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=diagnoses,verbs=create;update,versions=v1,name=mdiagnosis.kb.io

var _ webhook.Defaulter = &Diagnosis{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Diagnosis) Default() {
	diagnosislog.Info("defaulting Diagnosis", "diagnosis", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-kubediag-org-v1-diagnosis,mutating=false,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=diagnoses,versions=v1,name=vdiagnosis.kb.io

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

	if r.Spec.OperationSet == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("operationSet"),
			r.Spec.OperationSet, "must not be empty"))
	}
	if r.Spec.NodeName == "" && r.Spec.PodReference == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("nodeName"),
			r.Spec.NodeName, "must not be empty if podReference is empty"))
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.kubediag.org", Kind: "Diagnosis"},
		r.Name, allErrs)
}
