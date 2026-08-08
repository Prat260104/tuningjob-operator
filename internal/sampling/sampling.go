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

// Package sampling provides hyperparameter sampling functionality shared between
// the controller and suggester binary. It handles random sampling from numeric ranges
// and categorical value sets with deterministic seed support for testing.
package sampling

import (
	"fmt"
	"math/rand"
	"strconv"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
)

type Assignment map[string]string

type Sampler struct {
	Rand *rand.Rand
}

func NewSampler(r *rand.Rand) *Sampler {
	return &Sampler{Rand: r}
}

func (s *Sampler) SampleParameters(params []tuningv1alpha1.ParameterSpec) (Assignment, error) {
	assignments := make(Assignment)

	for _, param := range params {
		value, err := s.SampleParameter(param)
		if err != nil {
			return nil, fmt.Errorf("failed to sample parameter %s: %w", param.Name, err)
		}
		assignments[param.Name] = value
	}

	return assignments, nil
}

func (s *Sampler) SampleParameter(param tuningv1alpha1.ParameterSpec) (string, error) {
	hasCategorical := len(param.Values) > 0
	hasNumeric := param.Min != nil && param.Max != nil

	if hasCategorical && hasNumeric {
		return "", fmt.Errorf("parameter %s has both Values and Min/Max set; must specify only one form", param.Name)
	}

	if !hasCategorical && !hasNumeric {
		return "", fmt.Errorf("parameter %s must have either Values or both Min and Max", param.Name)
	}

	if hasCategorical {
		idx := s.randInt(len(param.Values))
		return param.Values[idx], nil
	}

	minVal, err := strconv.ParseFloat(*param.Min, 64)
	if err != nil {
		return "", fmt.Errorf("invalid min value: %w", err)
	}
	maxVal, err := strconv.ParseFloat(*param.Max, 64)
	if err != nil {
		return "", fmt.Errorf("invalid max value: %w", err)
	}

	value := minVal + s.randFloat64()*(maxVal-minVal)
	return strconv.FormatFloat(value, 'f', -1, 64), nil
}

func (s *Sampler) randInt(n int) int {
	if s.Rand != nil {
		return s.Rand.Intn(n)
	}
	return rand.Intn(n)
}

func (s *Sampler) randFloat64() float64 {
	if s.Rand != nil {
		return s.Rand.Float64()
	}
	return rand.Float64()
}
