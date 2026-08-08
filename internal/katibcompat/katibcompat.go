/*
Copyright 2026.

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

// Package katibcompat provides translation functions to prove TuningJob's parameter
// representation is compatible with Katib's v1beta1 Experiment API parameter spec.
//
// Scope: This is a standalone translation layer demonstrating lossless bidirectional
// mapping between parameter representations. It does NOT import the real kubeflow/katib
// Go module or integrate with Katib's suggestion service at runtime — that would be a
// separate integration project requiring Katib deployment and gRPC client setup.
//
// Purpose: Proves that TuningJob parameters can be translated to/from Katib's format
// without data loss, enabling future integration or migration paths without forcing
// users to adopt Katib's API surface in their trial Jobs.
package katibcompat

import (
	"fmt"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
)

// ParameterType represents Katib's parameter type enum.
// Based on Katib v1beta1 API: https://github.com/kubeflow/katib
// Valid values: "int", "double", "categorical", "discrete"
type ParameterType string

const (
	ParameterTypeInt         ParameterType = "int"
	ParameterTypeDouble      ParameterType = "double"
	ParameterTypeCategorical ParameterType = "categorical"
	ParameterTypeDiscrete    ParameterType = "discrete"
)

// FeasibleSpace defines the search space for a parameter.
// Mirrors Katib v1beta1 FeasibleSpace from api.proto.
// Reference: github.com/kubeflow/katib/pkg/apis/manager/v1beta1/gen-doc/api.md
type FeasibleSpace struct {
	// Min is the minimum value for numeric parameters (int/double).
	// +optional
	Min *string `json:"min,omitempty"`

	// Max is the maximum value for numeric parameters (int/double).
	// +optional
	Max *string `json:"max,omitempty"`

	// List is the set of valid values for categorical/discrete parameters.
	// +optional
	List []string `json:"list,omitempty"`

	// Step is the step size for int/double parameters.
	// +optional
	Step *string `json:"step,omitempty"`
}

// KatibParameterSpec represents a single parameter in Katib v1beta1 Experiment API.
// Based on: github.com/kubeflow/katib/pkg/apis/controller/experiments/v1beta1.ParameterSpec
type KatibParameterSpec struct {
	// Name is the parameter name.
	Name string `json:"name"`

	// ParameterType is the type of parameter: "int", "double", "categorical", or "discrete".
	ParameterType ParameterType `json:"parameterType"`

	// FeasibleSpace defines the valid range or set of values for this parameter.
	FeasibleSpace FeasibleSpace `json:"feasibleSpace"`
}

// ToKatibParameterSpec translates a TuningJob ParameterSpec into Katib's parameter format.
//
// Translation rules:
// - Numeric parameters (Min/Max set) → ParameterType "double" with FeasibleSpace.Min/Max
// - Categorical parameters (Values set) → ParameterType "categorical" with FeasibleSpace.List
//
// Returns error if:
// - Both Values and Min/Max are set (ambiguous)
// - Neither Values nor Min/Max are set (incomplete)
// - Only one of Min/Max is set (incomplete range)
func ToKatibParameterSpec(param tuningv1alpha1.ParameterSpec) (KatibParameterSpec, error) {
	hasCategorical := len(param.Values) > 0
	hasNumeric := param.Min != nil || param.Max != nil

	if hasCategorical && hasNumeric {
		return KatibParameterSpec{}, fmt.Errorf(
			"parameter %s has both Values and Min/Max set; must specify only one form",
			param.Name,
		)
	}

	if !hasCategorical && !hasNumeric {
		return KatibParameterSpec{}, fmt.Errorf(
			"parameter %s must have either Values or both Min and Max",
			param.Name,
		)
	}

	if hasCategorical {
		return KatibParameterSpec{
			Name:          param.Name,
			ParameterType: ParameterTypeCategorical,
			FeasibleSpace: FeasibleSpace{
				List: param.Values,
			},
		}, nil
	}

	// Numeric parameter
	if param.Min == nil || param.Max == nil {
		return KatibParameterSpec{}, fmt.Errorf(
			"parameter %s must have both Min and Max for numeric range",
			param.Name,
		)
	}

	return KatibParameterSpec{
		Name:          param.Name,
		ParameterType: ParameterTypeDouble,
		FeasibleSpace: FeasibleSpace{
			Min: param.Min,
			Max: param.Max,
		},
	}, nil
}

// ToKatibParameterSpecs translates a slice of TuningJob parameters to Katib format.
func ToKatibParameterSpecs(params []tuningv1alpha1.ParameterSpec) ([]KatibParameterSpec, error) {
	katibParams := make([]KatibParameterSpec, 0, len(params))

	for _, param := range params {
		katibParam, err := ToKatibParameterSpec(param)
		if err != nil {
			return nil, err
		}
		katibParams = append(katibParams, katibParam)
	}

	return katibParams, nil
}
