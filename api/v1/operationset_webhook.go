/*
Copyright 2021 The KubeDiag Authors.

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

// operationsetlog is for logging in this package.
var operationsetlog = logf.Log.WithName("operationset-webhook")

// SetupWebhookWithManager setups the OperationSet webhook.
func (r *OperationSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-kubediag-org-v1-operationset,mutating=true,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=operationsets,verbs=create;update,versions=v1,name=moperationset.kb.io

var _ webhook.Defaulter = &OperationSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *OperationSet) Default() {
	operationsetlog.Info("defaulting OperationSet", "operationset", r.Name)

	for i := 0; i < len(r.Spec.AdjacencyList); i++ {
		r.Spec.AdjacencyList[i].ID = i
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-kubediag-org-v1-operationset,mutating=false,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=operationsets,versions=v1,name=voperationset.kb.io

var _ webhook.Validator = &OperationSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *OperationSet) ValidateCreate() error {
	operationsetlog.Info("validating creation of OperationSet", "operationset", r.Name)

	return r.validateOperationSet()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OperationSet) ValidateUpdate(old runtime.Object) error {
	operationsetlog.Info("validating update of OperationSet", "operationset", r.Name)

	return r.validateOperationSet()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OperationSet) ValidateDelete() error {
	operationsetlog.Info("validating deletion of OperationSet", "operationset", r.Name)

	return nil
}

// validateOperationSet validates OperationSet and returns err if any invalidation is found.
func (r *OperationSet) validateOperationSet() error {
	var allErrs field.ErrorList

	if len(r.Spec.AdjacencyList) <= 1 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("adjacencyList"),
			r.Spec.AdjacencyList, "must contain at least two nodes"))
	} else {
		if r.Spec.AdjacencyList[0].Operation != "" || len(r.Spec.AdjacencyList[0].Dependences) != 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("adjacencyList").Index(0),
				r.Spec.AdjacencyList, "must not contains any operation or dependences"))
		}
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.kubediag.org", Kind: "OperationSet"},
		r.Name, allErrs)
}
