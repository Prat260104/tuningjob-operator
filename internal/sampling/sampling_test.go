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

package sampling

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
)

func TestSampler_SampleParameter_Numeric(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name: "learning_rate",
		Min:  &min,
		Max:  &max,
	}

	value, err := sampler.SampleParameter(param)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("expected numeric value, got non-parseable: %s", value)
	}

	minVal, _ := strconv.ParseFloat(min, 64)
	maxVal, _ := strconv.ParseFloat(max, 64)

	if floatVal < minVal || floatVal > maxVal {
		t.Errorf("sampled value %f out of range [%f, %f]", floatVal, minVal, maxVal)
	}
}

func TestSampler_SampleParameter_Categorical(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	param := tuningv1alpha1.ParameterSpec{
		Name:   "optimizer",
		Values: []string{"sgd", "adam", "rmsprop"},
	}

	value, err := sampler.SampleParameter(param)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	validValues := map[string]bool{"sgd": true, "adam": true, "rmsprop": true}
	if !validValues[value] {
		t.Errorf("sampled value %s not in valid set %v", value, param.Values)
	}
}

func TestSampler_SampleParameter_BothFormsSet(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name:   "invalid_param",
		Min:    &min,
		Max:    &max,
		Values: []string{"sgd", "adam"},
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error when both Values and Min/Max are set, got nil")
	}

	if !strings.Contains(err.Error(), "has both Values and Min/Max set") {
		t.Errorf("expected error about both forms set, got: %v", err)
	}
}

func TestSampler_SampleParameter_NeitherFormSet(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error when neither Values nor Min/Max are set, got nil")
	}

	if !strings.Contains(err.Error(), "must have either Values or both Min and Max") {
		t.Errorf("expected error about missing forms, got: %v", err)
	}
}

func TestSampler_SampleParameter_OnlyMinSet(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Min:  &min,
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error when only Min is set, got nil")
	}

	if !strings.Contains(err.Error(), "must have either Values or both Min and Max") {
		t.Errorf("expected error about incomplete range, got: %v", err)
	}
}

func TestSampler_SampleParameter_OnlyMaxSet(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Max:  &max,
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error when only Max is set, got nil")
	}

	if !strings.Contains(err.Error(), "must have either Values or both Min and Max") {
		t.Errorf("expected error about incomplete range, got: %v", err)
	}
}

func TestSampler_SampleParameter_InvalidMinValue(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	min := "not-a-number"
	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Min:  &min,
		Max:  &max,
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error for invalid min value, got nil")
	}

	if !strings.Contains(err.Error(), "invalid min value") {
		t.Errorf("expected error about invalid min, got: %v", err)
	}
}

func TestSampler_SampleParameter_InvalidMaxValue(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	max := "not-a-number"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Min:  &min,
		Max:  &max,
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error for invalid max value, got nil")
	}

	if !strings.Contains(err.Error(), "invalid max value") {
		t.Errorf("expected error about invalid max, got: %v", err)
	}
}

func TestSampler_SampleParameter_Deterministic(t *testing.T) {
	seed := int64(12345)

	sampler1 := NewSampler(rand.New(rand.NewSource(seed)))
	sampler2 := NewSampler(rand.New(rand.NewSource(seed)))

	min := "0.0"
	max := "1.0"
	param := tuningv1alpha1.ParameterSpec{
		Name: "test_param",
		Min:  &min,
		Max:  &max,
	}

	value1, _ := sampler1.SampleParameter(param)
	value2, _ := sampler2.SampleParameter(param)

	if value1 != value2 {
		t.Errorf("expected deterministic sampling with same seed, got %s and %s", value1, value2)
	}
}

func TestSampler_SampleParameter_NilSampler(t *testing.T) {
	sampler := NewSampler(nil)

	min := "0.0"
	max := "1.0"
	param := tuningv1alpha1.ParameterSpec{
		Name: "test_param",
		Min:  &min,
		Max:  &max,
	}

	// Should use default rand source without panicking
	_, err := sampler.SampleParameter(param)
	if err != nil {
		t.Fatalf("expected no error with nil rand, got %v", err)
	}
}

func TestSampler_SampleParameters_Multiple(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	max := "0.1"
	params := []tuningv1alpha1.ParameterSpec{
		{
			Name: "learning_rate",
			Min:  &min,
			Max:  &max,
		},
		{
			Name:   "optimizer",
			Values: []string{"sgd", "adam"},
		},
	}

	assignments, err := sampler.SampleParameters(params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	if _, ok := assignments["learning_rate"]; !ok {
		t.Error("missing learning_rate in assignments")
	}

	if _, ok := assignments["optimizer"]; !ok {
		t.Error("missing optimizer in assignments")
	}

	// Validate learning_rate is numeric
	lr := assignments["learning_rate"]
	lrVal, err := strconv.ParseFloat(lr, 64)
	if err != nil {
		t.Errorf("learning_rate is not a valid float: %v", err)
	}
	if lrVal < 0.001 || lrVal > 0.1 {
		t.Errorf("learning_rate %f out of range [0.001, 0.1]", lrVal)
	}

	// Validate optimizer is categorical
	opt := assignments["optimizer"]
	if opt != "sgd" && opt != "adam" {
		t.Errorf("optimizer %s not in valid set [sgd, adam]", opt)
	}
}

func TestSampler_SampleParameters_ErrorPropagation(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	params := []tuningv1alpha1.ParameterSpec{
		{
			Name:   "valid_param",
			Values: []string{"a", "b"},
		},
		{
			Name: "invalid_param",
			// neither Values nor Min/Max
		},
	}

	_, err := sampler.SampleParameters(params)
	if err == nil {
		t.Fatal("expected error when one param is invalid, got nil")
	}

	if !strings.Contains(err.Error(), "invalid_param") {
		t.Errorf("expected error to mention invalid_param, got: %v", err)
	}
}

func TestSampler_SampleParameters_EmptySlice(t *testing.T) {
	sampler := NewSampler(rand.New(rand.NewSource(42)))

	assignments, err := sampler.SampleParameters([]tuningv1alpha1.ParameterSpec{})
	if err != nil {
		t.Fatalf("expected no error for empty params, got %v", err)
	}

	if len(assignments) != 0 {
		t.Errorf("expected empty assignments, got %d", len(assignments))
	}
}

func TestNewSampler_NilRand(t *testing.T) {
	sampler := NewSampler(nil)
	if sampler == nil {
		t.Fatal("NewSampler should not return nil")
	}
	if sampler.Rand != nil {
		t.Error("expected Rand field to be nil when passed nil")
	}
}

func TestNewSampler_WithRand(t *testing.T) {
	r := rand.New(rand.NewSource(123))
	sampler := NewSampler(r)

	if sampler == nil {
		t.Fatal("NewSampler should not return nil")
	}
	if sampler.Rand != r {
		t.Error("expected Rand field to match provided rand source")
	}
}
