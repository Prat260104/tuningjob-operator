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

package main

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
)

func TestSuggest_ValidRequest(t *testing.T) {
	min := "0.001"
	max := "0.1"
	req := SuggestRequest{
		Parameters: []tuningv1alpha1.ParameterSpec{
			{
				Name: "learning_rate",
				Min:  &min,
				Max:  &max,
			},
			{
				Name:   "optimizer",
				Values: []string{"sgd", "adam", "rmsprop"},
			},
		},
		Goal:       "maximize",
		PastTrials: []PastTrial{},
	}

	resp, err := Suggest(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(resp.Parameters))
	}

	if _, ok := resp.Parameters["learning_rate"]; !ok {
		t.Error("missing learning_rate parameter")
	}

	if _, ok := resp.Parameters["optimizer"]; !ok {
		t.Error("missing optimizer parameter")
	}

	lr, err := strconv.ParseFloat(resp.Parameters["learning_rate"], 64)
	if err != nil {
		t.Errorf("learning_rate is not a valid float: %v", err)
	}
	if lr < 0.001 || lr > 0.1 {
		t.Errorf("learning_rate %f out of range [0.001, 0.1]", lr)
	}

	optimizer := resp.Parameters["optimizer"]
	if optimizer != "sgd" && optimizer != "adam" && optimizer != "rmsprop" {
		t.Errorf("optimizer %s not in valid set [sgd, adam, rmsprop]", optimizer)
	}
}

func TestSuggest_InvalidGoal(t *testing.T) {
	req := SuggestRequest{
		Parameters: []tuningv1alpha1.ParameterSpec{},
		Goal:       "invalid",
		PastTrials: []PastTrial{},
	}

	_, err := Suggest(req)
	if err == nil {
		t.Fatal("expected error for invalid goal, got nil")
	}

	if !strings.Contains(err.Error(), "invalid goal") {
		t.Errorf("expected error about invalid goal, got: %v", err)
	}
}

func TestSuggest_InvalidParameter(t *testing.T) {
	req := SuggestRequest{
		Parameters: []tuningv1alpha1.ParameterSpec{
			{
				Name: "invalid",
			},
		},
		Goal:       "maximize",
		PastTrials: []PastTrial{},
	}

	_, err := Suggest(req)
	if err == nil {
		t.Fatal("expected error for invalid parameter, got nil")
	}
}

func TestSuggesterBinary_RealExec(t *testing.T) {
	t.Log("Building suggester binary...")
	buildCmd := exec.Command("go", "build", "-o", "suggester-test", ".")
	buildCmd.Dir = "."
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build suggester binary: %v\n%s", err, output)
	}
	defer exec.Command("rm", "-f", "suggester-test").Run()

	min := "0.001"
	max := "0.1"
	req := SuggestRequest{
		Parameters: []tuningv1alpha1.ParameterSpec{
			{
				Name: "learning_rate",
				Min:  &min,
				Max:  &max,
			},
			{
				Name:   "optimizer",
				Values: []string{"sgd", "adam"},
			},
		},
		Goal: "maximize",
		PastTrials: []PastTrial{
			{
				Parameters: map[string]string{"learning_rate": "0.01", "optimizer": "sgd"},
				Metric:     "0.85",
			},
		},
	}

	input, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	t.Log("Executing suggester binary via exec.Command...")
	cmd := exec.Command("./suggester-test")
	cmd.Stdin = strings.NewReader(string(input))

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("suggester binary failed: %v\nOutput: %s", err, output)
	}

	if cmd.ProcessState.ExitCode() != 0 {
		t.Errorf("expected exit code 0, got %d", cmd.ProcessState.ExitCode())
	}

	var resp SuggestResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("failed to parse suggester output: %v\nOutput: %s", err, output)
	}

	if len(resp.Parameters) != 2 {
		t.Errorf("expected 2 parameters in response, got %d", len(resp.Parameters))
	}

	if _, ok := resp.Parameters["learning_rate"]; !ok {
		t.Error("missing learning_rate in response")
	}

	if _, ok := resp.Parameters["optimizer"]; !ok {
		t.Error("missing optimizer in response")
	}

	t.Logf("Suggester binary executed successfully and returned valid JSON: %s", output)
}
