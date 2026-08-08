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

package katibcompat

import (
	"strings"
	"testing"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
)

func TestToKatibParameterSpec_NumericParam(t *testing.T) {
	min := "0.001"
	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name: "learning_rate",
		Min:  &min,
		Max:  &max,
	}

	katibParam, err := ToKatibParameterSpec(param)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if katibParam.Name != "learning_rate" {
		t.Errorf("expected name=learning_rate, got %s", katibParam.Name)
	}

	if katibParam.ParameterType != ParameterTypeDouble {
		t.Errorf("expected parameterType=double, got %s", katibParam.ParameterType)
	}

	if katibParam.FeasibleSpace.Min == nil {
		t.Fatal("expected FeasibleSpace.Min to be set")
	}
	if *katibParam.FeasibleSpace.Min != "0.001" {
		t.Errorf("expected Min=0.001, got %s", *katibParam.FeasibleSpace.Min)
	}

	if katibParam.FeasibleSpace.Max == nil {
		t.Fatal("expected FeasibleSpace.Max to be set")
	}
	if *katibParam.FeasibleSpace.Max != "0.1" {
		t.Errorf("expected Max=0.1, got %s", *katibParam.FeasibleSpace.Max)
	}

	if len(katibParam.FeasibleSpace.List) != 0 {
		t.Errorf("expected FeasibleSpace.List to be empty for numeric param, got %v", katibParam.FeasibleSpace.List)
	}
}

func TestToKatibParameterSpec_CategoricalParam(t *testing.T) {
	param := tuningv1alpha1.ParameterSpec{
		Name:   "optimizer",
		Values: []string{"sgd", "adam", "rmsprop"},
	}

	katibParam, err := ToKatibParameterSpec(param)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if katibParam.Name != "optimizer" {
		t.Errorf("expected name=optimizer, got %s", katibParam.Name)
	}

	if katibParam.ParameterType != ParameterTypeCategorical {
		t.Errorf("expected parameterType=categorical, got %s", katibParam.ParameterType)
	}

	if len(katibParam.FeasibleSpace.List) != 3 {
		t.Fatalf("expected FeasibleSpace.List length=3, got %d", len(katibParam.FeasibleSpace.List))
	}

	expectedValues := map[string]bool{"sgd": true, "adam": true, "rmsprop": true}
	for _, value := range katibParam.FeasibleSpace.List {
		if !expectedValues[value] {
			t.Errorf("unexpected value in FeasibleSpace.List: %s", value)
		}
	}

	if katibParam.FeasibleSpace.Min != nil {
		t.Errorf("expected FeasibleSpace.Min to be nil for categorical param, got %v", *katibParam.FeasibleSpace.Min)
	}

	if katibParam.FeasibleSpace.Max != nil {
		t.Errorf("expected FeasibleSpace.Max to be nil for categorical param, got %v", *katibParam.FeasibleSpace.Max)
	}
}

func TestToKatibParameterSpec_BothFormsSet(t *testing.T) {
	min := "0.001"
	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name:   "invalid_param",
		Min:    &min,
		Max:    &max,
		Values: []string{"sgd", "adam"},
	}

	_, err := ToKatibParameterSpec(param)
	if err == nil {
		t.Fatal("expected error when both Values and Min/Max are set, got nil")
	}

	expectedErrMsg := "has both Values and Min/Max set"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error containing %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestToKatibParameterSpec_NeitherFormSet(t *testing.T) {
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
	}

	_, err := ToKatibParameterSpec(param)
	if err == nil {
		t.Fatal("expected error when neither Values nor Min/Max are set, got nil")
	}

	expectedErrMsg := "must have either Values or both Min and Max"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error containing %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestToKatibParameterSpec_OnlyMinSet(t *testing.T) {
	min := "0.001"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Min:  &min,
	}

	_, err := ToKatibParameterSpec(param)
	if err == nil {
		t.Fatal("expected error when only Min is set, got nil")
	}

	expectedErrMsg := "must have both Min and Max"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error containing %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestToKatibParameterSpec_OnlyMaxSet(t *testing.T) {
	max := "0.1"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Max:  &max,
	}

	_, err := ToKatibParameterSpec(param)
	if err == nil {
		t.Fatal("expected error when only Max is set, got nil")
	}

	expectedErrMsg := "must have both Min and Max"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error containing %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestToKatibParameterSpecs_MultipleParams(t *testing.T) {
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

	katibParams, err := ToKatibParameterSpecs(params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(katibParams) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(katibParams))
	}

	// Check first param (numeric)
	if katibParams[0].Name != "learning_rate" {
		t.Errorf("expected first param name=learning_rate, got %s", katibParams[0].Name)
	}
	if katibParams[0].ParameterType != ParameterTypeDouble {
		t.Errorf("expected first param type=double, got %s", katibParams[0].ParameterType)
	}
	if katibParams[0].FeasibleSpace.Min == nil || *katibParams[0].FeasibleSpace.Min != "0.001" {
		t.Error("first param FeasibleSpace.Min incorrect")
	}

	// Check second param (categorical)
	if katibParams[1].Name != "optimizer" {
		t.Errorf("expected second param name=optimizer, got %s", katibParams[1].Name)
	}
	if katibParams[1].ParameterType != ParameterTypeCategorical {
		t.Errorf("expected second param type=categorical, got %s", katibParams[1].ParameterType)
	}
	if len(katibParams[1].FeasibleSpace.List) != 2 {
		t.Errorf("expected second param List length=2, got %d", len(katibParams[1].FeasibleSpace.List))
	}
}

func TestToKatibParameterSpecs_InvalidParam(t *testing.T) {
	params := []tuningv1alpha1.ParameterSpec{
		{
			Name:   "valid",
			Values: []string{"a", "b"},
		},
		{
			Name: "invalid",
			// neither Values nor Min/Max set
		},
	}

	_, err := ToKatibParameterSpecs(params)
	if err == nil {
		t.Fatal("expected error when one param is invalid, got nil")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected error to mention invalid param, got: %v", err)
	}
}

func TestToKatibParameterSpecs_EmptySlice(t *testing.T) {
	params := []tuningv1alpha1.ParameterSpec{}

	katibParams, err := ToKatibParameterSpecs(params)
	if err != nil {
		t.Fatalf("expected no error for empty slice, got %v", err)
	}

	if len(katibParams) != 0 {
		t.Errorf("expected empty result, got %d params", len(katibParams))
	}
}

func TestParameterType_Constants(t *testing.T) {
	// Verify constants match Katib's expected values
	if ParameterTypeInt != "int" {
		t.Errorf("ParameterTypeInt should be 'int', got %s", ParameterTypeInt)
	}
	if ParameterTypeDouble != "double" {
		t.Errorf("ParameterTypeDouble should be 'double', got %s", ParameterTypeDouble)
	}
	if ParameterTypeCategorical != "categorical" {
		t.Errorf("ParameterTypeCategorical should be 'categorical', got %s", ParameterTypeCategorical)
	}
	if ParameterTypeDiscrete != "discrete" {
		t.Errorf("ParameterTypeDiscrete should be 'discrete', got %s", ParameterTypeDiscrete)
	}
}

func TestKatibParameterSpec_StructShape(t *testing.T) {
	// Verify the struct can be constructed with all Katib-expected fields
	min := "1.0"
	max := "10.0"
	step := "0.5"

	param := KatibParameterSpec{
		Name:          "test_param",
		ParameterType: ParameterTypeDouble,
		FeasibleSpace: FeasibleSpace{
			Min:  &min,
			Max:  &max,
			Step: &step,
			List: nil,
		},
	}

	if param.Name != "test_param" {
		t.Error("struct field Name not accessible")
	}
	if param.ParameterType != ParameterTypeDouble {
		t.Error("struct field ParameterType not accessible")
	}
	if param.FeasibleSpace.Min == nil || *param.FeasibleSpace.Min != "1.0" {
		t.Error("struct field FeasibleSpace.Min not accessible")
	}
	if param.FeasibleSpace.Step == nil || *param.FeasibleSpace.Step != "0.5" {
		t.Error("struct field FeasibleSpace.Step not accessible")
	}
}
