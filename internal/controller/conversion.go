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

package controller

import (
	"fmt"

	"github.com/kraken-iac/aws-ec2-instance/api/v1alpha1"
	krakenv1alpha1 "github.com/kraken-iac/kraken/api/core/v1alpha1"
)

type ec2InstanceApplicableValues struct {
	imageID      string
	instanceType string
	maxCount     int
	minCount     int
}

func toApplicableValues(
	ec2Spec v1alpha1.EC2InstanceSpec,
	depValues krakenv1alpha1.DependentValues,
) (*ec2InstanceApplicableValues, error) {
	av := ec2InstanceApplicableValues{}

	if imageID, err := ec2Spec.ImageID.ToApplicableValue(depValues); err != nil {
		return nil, err
	} else if imageID == nil {
		return nil, fmt.Errorf("no applicable value provided for ImageID")
	} else {
		av.imageID = *imageID
	}

	if instanceType, err := ec2Spec.InstanceType.ToApplicableValue(depValues); err != nil {
		return nil, err
	} else if instanceType == nil {
		return nil, fmt.Errorf("no applicable value provided for InstanceType")
	} else {
		av.instanceType = *instanceType
	}

	if maxCount, err := ec2Spec.MaxCount.ToApplicableValue(depValues); err != nil {
		return nil, err
	} else if maxCount == nil {
		return nil, fmt.Errorf("no applicable value provided for MaxCount")
	} else {
		av.maxCount = *maxCount
	}

	if minCount, err := ec2Spec.MinCount.ToApplicableValue(depValues); err != nil {
		return nil, err
	} else if minCount == nil {
		return nil, fmt.Errorf("no applicable value provided for MinCount")
	} else {
		av.minCount = *minCount
	}

	return &av, nil
}
