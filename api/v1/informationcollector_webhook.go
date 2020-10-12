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
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// informationcollectorlog is for logging of informationcollector webhook.
var informationcollectorlog = logf.Log.WithName("informationcollector-webhook")

// SetupWebhookWithManager setups the InformationCollector webhook.
func (r *InformationCollector) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-diagnosis-netease-com-v1-informationcollector,mutating=true,failurePolicy=fail,groups=diagnosis.netease.com,resources=informationcollectors,verbs=create;update,versions=v1,name=minformationcollector.kb.io

var _ webhook.Defaulter = &InformationCollector{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *InformationCollector) Default() {
	informationcollectorlog.Info("defaulting InformationCollector", "informationcollector", client.ObjectKey{
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

// +kubebuilder:webhook:verbs=create;update,path=/validate-diagnosis-netease-com-v1-informationcollector,mutating=false,failurePolicy=fail,groups=diagnosis.netease.com,resources=informationcollectors,versions=v1,name=vinformationcollector.kb.io

var _ webhook.Validator = &InformationCollector{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *InformationCollector) ValidateCreate() error {
	informationcollectorlog.Info("validating creation of InformationCollector", "informationcollector", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateInformationCollector()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *InformationCollector) ValidateUpdate(old runtime.Object) error {
	informationcollectorlog.Info("validating update of InformationCollector", "informationcollector", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return r.validateInformationCollector()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *InformationCollector) ValidateDelete() error {
	informationcollectorlog.Info("validating deletion of InformationCollector", "informationcollector", client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namespace,
	})

	return nil
}

// validateInformationCollector validates InformationCollector and returns err if any invalidation is found.
func (r *InformationCollector) validateInformationCollector() error {
	var allErrs field.ErrorList

	if net.ParseIP(r.Spec.IP) == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("ip"),
			r.Spec.IP, "must be valid ip"))
	}
	if _, err := strconv.Atoi(r.Spec.Port); err == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("ip"),
			r.Spec.Port, "must be valid port"))
	}
	if r.Spec.Scheme != "http" && r.Spec.Scheme != "https" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("scheme"),
			r.Spec.Scheme, "must be either http or https"))
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "diagnosis.netease.com", Kind: "InformationCollector"},
		r.Name, allErrs)
}
