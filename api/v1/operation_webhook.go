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
	"net"

	"github.com/asaskevich/govalidator"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// operationlog is for logging of operation webhook.
var operationlog = logf.Log.WithName("operation-webhook")

// SetupWebhookWithManager setups the Operation webhook.
func (r *Operation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-kubediag-org-v1-operation,mutating=true,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=operations,verbs=create;update,versions=v1,name=moperation.kb.io

var _ webhook.Defaulter = &Operation{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Operation) Default() {
	operationlog.Info("defaulting Operation", "operation", r.Name)

	if r.Spec.Processor.HTTPServer != nil {
		if r.Spec.Processor.HTTPServer.Scheme == nil {
			var scheme string = "http"
			r.Spec.Processor.HTTPServer.Scheme = &scheme
		}
		if r.Spec.Processor.HTTPServer.Path == nil {
			var path string = "/"
			r.Spec.Processor.HTTPServer.Path = &path
		}
	}
	if r.Spec.Processor.TimeoutSeconds == nil {
		var timeoutSeconds int32 = 30
		r.Spec.Processor.TimeoutSeconds = &timeoutSeconds
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-kubediag-org-v1-operation,mutating=false,failurePolicy=fail,groups=diagnosis.kubediag.org,resources=operations,versions=v1,name=voperation.kb.io

var _ webhook.Validator = &Operation{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Operation) ValidateCreate() error {
	operationlog.Info("validating creation of Operation", "operation", r.Name)

	return r.validateOperation()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Operation) ValidateUpdate(old runtime.Object) error {
	operationlog.Info("validating update of Operation", "operation", r.Name)

	return r.validateOperation()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Operation) ValidateDelete() error {
	operationlog.Info("validating deletion of Operation", "operation", r.Name)

	return nil
}

// validateOperation validates Operation and returns err if any invalidation is found.
func (r *Operation) validateOperation() error {
	var allErrs field.ErrorList

	if r.Spec.Processor.HTTPServer == nil && r.Spec.Processor.ScriptRunner == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("processor"),
			r.Spec.Processor, "must specify either http server or script runner"))
	} else if r.Spec.Processor.HTTPServer != nil && r.Spec.Processor.ScriptRunner != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("processor"),
			r.Spec.Processor, "one and only one processor should be specified."))
	} else if r.Spec.Processor.HTTPServer != nil {
		if r.Spec.Processor.HTTPServer.Address != nil {
			if net.ParseIP(*r.Spec.Processor.HTTPServer.Address) == nil && !govalidator.IsDNSName(*r.Spec.Processor.HTTPServer.Address) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("processor").Child("httpServer").Child("address"),
					r.Spec.Processor.HTTPServer.Address, "must be a valid ip or dns address"))
			}
		}
		if r.Spec.Processor.HTTPServer.Port != nil {
			if *r.Spec.Processor.HTTPServer.Port <= 0 || *r.Spec.Processor.HTTPServer.Port > 65535 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("processor").Child("httpServer").Child("port"),
					r.Spec.Processor.HTTPServer.Port, "must be greater than 0 and less equal to 65535"))
			}
		}
		if r.Spec.Processor.HTTPServer.Scheme != nil {
			if *r.Spec.Processor.HTTPServer.Scheme != "http" && *r.Spec.Processor.HTTPServer.Scheme != "https" {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("processor").Child("httpServer").Child("scheme"),
					r.Spec.Processor.HTTPServer.Scheme, "must be either http or https"))
			}
		}
	}
	if r.Spec.Processor.TimeoutSeconds != nil {
		if *r.Spec.Processor.TimeoutSeconds <= 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("processor").Child("timeoutSeconds"),
				r.Spec.Processor.TimeoutSeconds, "must be greater than 0"))
		}
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.kubediag.org", Kind: "Operation"},
		r.Name, allErrs)
}
