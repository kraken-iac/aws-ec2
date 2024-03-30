// +kubebuilder:object:generate=true

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

package option

// TODO: Move this package into a shared types repo

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/Jeffail/gabs/v2"
	krakenv1alpha1 "github.com/kraken-iac/kraken/api/v1alpha1"
)

type ValueFromConfigMap struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func (vfcm ValueFromConfigMap) ToConfigMapDependency() krakenv1alpha1.ConfigMapDependency {
	return krakenv1alpha1.ConfigMapDependency{
		Name: vfcm.Name,
		Key:  vfcm.Key,
	}
}

type ValueFromSecret struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func (vfs ValueFromSecret) ToSecretDependency() {
	panic("Not implemented")
}

type ValueFromKrakenResource struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func (vfkr ValueFromKrakenResource) ToKrakenResourceDependency(kind reflect.Kind) krakenv1alpha1.KrakenResourceDependency {
	return krakenv1alpha1.KrakenResourceDependency{
		Kind:        vfkr.Kind,
		Name:        vfkr.Name,
		Path:        vfkr.Path,
		ReflectKind: kind,
	}
}

type ValueFrom struct {
	ConfigMap      *ValueFromConfigMap      `json:"configMap,omitempty"`
	Secret         *ValueFromSecret         `json:"secret,omitempty"`
	KrakenResource *ValueFromKrakenResource `json:"krakenResource,omitempty"`
}

func (vf ValueFrom) AddToDependencyRequestSpec(dr *krakenv1alpha1.DependencyRequestSpec, kind reflect.Kind) {
	if vf.KrakenResource != nil {
		dr.KrakenResourceDependencies = append(dr.KrakenResourceDependencies, vf.KrakenResource.ToKrakenResourceDependency(kind))
	}
	if vf.ConfigMap != nil {
		dr.ConfigMapDependencies = append(dr.ConfigMapDependencies, vf.ConfigMap.ToConfigMapDependency())
	}
	if vf.Secret != nil {
		panic("Unimplemented")
	}
}

type String struct {
	Value     *string    `json:"value,omitempty"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

func (s String) ToApplicableValue(dv krakenv1alpha1.DependentValues) (*string, error) {
	if s.Value != nil {
		return s.Value, nil
	}
	if s.ValueFrom == nil {
		return nil, nil
	}
	if s.ValueFrom.ConfigMap != nil {
		return getValueFromConfigMap(s.ValueFrom.ConfigMap, dv.FromConfigMaps)
	}
	if s.ValueFrom.KrakenResource != nil {
		return getValueFromKrakenResource[string](s.ValueFrom.KrakenResource, dv.FromKrakenResources)
	}
	return nil, errors.New("ValueFrom object is not nil but does not contain any non-nil pointer references")
}

type Int struct {
	Value     *int       `json:"value,omitempty"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

func (i Int) ToApplicableValue(dv krakenv1alpha1.DependentValues) (*int, error) {
	if i.Value != nil {
		return i.Value, nil
	}
	if i.ValueFrom == nil {
		return nil, nil
	}
	if i.ValueFrom.ConfigMap != nil {
		valString, err := getValueFromConfigMap(i.ValueFrom.ConfigMap, dv.FromConfigMaps)
		if err != nil {
			return nil, err
		}
		val, err := strconv.Atoi(*valString)
		if err != nil {
			return nil, err
		}
		return &val, nil
	}
	if i.ValueFrom.KrakenResource != nil {
		// Unmarshalled JSON numbers are of type float64
		valFloat, err := getValueFromKrakenResource[float64](i.ValueFrom.KrakenResource, dv.FromKrakenResources)
		if err != nil {
			return nil, err
		}
		val := int(*valFloat)
		return &val, nil
	}
	return nil, errors.New("ValueFrom object is not nil but does not contain any non-nil pointer references")
}

func getValueFromConfigMap(cmRef *ValueFromConfigMap, cmVals krakenv1alpha1.DependentValuesFromConfigMaps) (*string, error) {
	cm, exists := cmVals[cmRef.Name]
	if !exists {
		return nil, fmt.Errorf("ConfigMap \"%s\" does not exist in DependentValues", cmRef.Name)
	}
	val, exists := cm[cmRef.Key]
	if !exists {
		return nil, fmt.Errorf("key \"%s\" does not exist in DependentValues ConfigMap \"%s\"", cmRef.Key, cmRef.Name)
	}
	return &val, nil
}

func getValueFromKrakenResource[T any](
	krRef *ValueFromKrakenResource,
	krVals krakenv1alpha1.DependentValuesFromKrakenResources,
) (*T, error) {
	kind, exists := krVals[krRef.Kind]
	if !exists {
		return nil, fmt.Errorf("no entry for kind \"%s\" in DependentValues", krRef.Kind)
	}
	resource, exists := kind[krRef.Name]
	if !exists {
		return nil, fmt.Errorf("no entry for resource \"%s\" in DependentValues", krRef.Name)
	}
	jsonVal, exists := resource[krRef.Path]
	if !exists {
		return nil, fmt.Errorf("no entry for path \"%s\" in DependentValues", krRef.Path)
	}

	jsonContainer, err := gabs.ParseJSON(jsonVal.Raw)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %s", err)
	}
	data := jsonContainer.Data()

	var val T
	expectedType := reflect.TypeOf(val).Kind()
	actualType := reflect.TypeOf(data).Kind()
	if actualType != expectedType {
		return nil, fmt.Errorf(
			"provided value \"%s\" is of type \"%s\"; expected type \"%s\"",
			data,
			actualType,
			expectedType,
		)
	}

	val = data.(T)
	return &val, nil
}
