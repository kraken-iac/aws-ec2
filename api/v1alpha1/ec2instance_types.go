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
	"reflect"

	"github.com/kraken-iac/common/types/option"
	"github.com/kraken-iac/kraken/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EC2InstanceSpec defines the desired state of EC2Instance
type EC2InstanceSpec struct {
	ImageID      option.String `json:"imageID"`
	InstanceType option.String `json:"instanceType"`
	MaxCount     option.Int    `json:"maxCount"`
	MinCount     option.Int    `json:"minCount"`

	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

func (s EC2InstanceSpec) GenerateDependencyRequestSpec() v1alpha1.DependencyRequestSpec {
	dr := v1alpha1.DependencyRequestSpec{}
	if s.ImageID.ValueFrom != nil {
		s.ImageID.ValueFrom.AddToDependencyRequestSpec(&dr, reflect.String)
	}
	if s.InstanceType.ValueFrom != nil {
		s.InstanceType.ValueFrom.AddToDependencyRequestSpec(&dr, reflect.String)
	}
	if s.MaxCount.ValueFrom != nil {
		s.MaxCount.ValueFrom.AddToDependencyRequestSpec(&dr, reflect.Int)
	}
	if s.MinCount.ValueFrom != nil {
		s.MinCount.ValueFrom.AddToDependencyRequestSpec(&dr, reflect.Int)
	}
	return dr
}

// EC2InstanceStatus defines the observed state of EC2Instance
type EC2InstanceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EC2Instance is the Schema for the ec2instances API
type EC2Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EC2InstanceSpec   `json:"spec,omitempty"`
	Status EC2InstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EC2InstanceList contains a list of EC2Instance
type EC2InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EC2Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EC2Instance{}, &EC2InstanceList{})
}
