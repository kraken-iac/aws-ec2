/*
Copyright 2024.

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

package v1alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var ec2instancelog = logf.Log.WithName("ec2instance-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *EC2Instance) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-aws-kraken-iac-eoinfennessy-com-v1alpha1-ec2instance,mutating=false,failurePolicy=fail,sideEffects=None,groups=aws.kraken-iac.eoinfennessy.com,resources=ec2instances,verbs=create;update,versions=v1alpha1,name=vec2instance.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &EC2Instance{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *EC2Instance) ValidateCreate() (admission.Warnings, error) {
	ec2instancelog.Info("validate create", "name", r.Name)
	return nil, r.validateSpec()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *EC2Instance) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	ec2instancelog.Info("validate update", "name", r.Name)
	return nil, r.validateSpec()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *EC2Instance) ValidateDelete() (admission.Warnings, error) {
	ec2instancelog.Info("validate delete", "name", r.Name)
	return nil, nil
}

func (r *EC2Instance) validateSpec() error {
	var errs field.ErrorList

	optionErrs := r.validateOptionFields()
	errs = append(errs, optionErrs...)

	if len(errs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "aws.kraken-iac.eoinfennessy.com", Kind: "EC2Instance"},
		r.Name,
		errs,
	)
}

func (r *EC2Instance) validateOptionFields() field.ErrorList {
	var errs field.ErrorList
	if err := r.Spec.ImageID.Validate(); err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("imageID"), r.Spec.ImageID, err.Error()))
	}
	if err := r.Spec.InstanceType.Validate(); err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("instanceType"), r.Spec.InstanceType, err.Error()))
	}
	if err := r.Spec.MaxCount.Validate(); err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("maxCount"), r.Spec.MaxCount, err.Error()))
	}
	if err := r.Spec.MinCount.Validate(); err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("minCount"), r.Spec.MinCount, err.Error()))
	}
	return errs
}
