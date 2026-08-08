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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
	"github.com/Prat260104/tuningjob-operator/internal/sampling"
)

func TestConstructJobFromTemplate_WithSuggesterFunc(t *testing.T) {
	fakeSuggester := func(ctx context.Context, input SuggesterInput) (SuggesterOutput, error) {
		if input.Goal != "maximize" && input.Goal != "minimize" {
			return SuggesterOutput{}, fmt.Errorf("invalid goal: %s", input.Goal)
		}

		return SuggesterOutput{
			Parameters: map[string]string{
				"learning_rate": "0.025",
				"optimizer":     "adam",
			},
		}, nil
	}

	reconciler := &TuningJobReconciler{
		Sampler:       sampling.NewSampler(rand.New(rand.NewSource(42))),
		Scheme:        setupScheme(),
		SuggesterFunc: fakeSuggester,
	}

	min := "0.001"
	max := "0.1"
	tuningJob := &tuningv1alpha1.TuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tuning",
			Namespace: "default",
		},
		Spec: tuningv1alpha1.TuningJobSpec{
			MaxTrials: 5,
			Goal:      "maximize",
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
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "training",
									Image: "training:latest",
								},
							},
						},
					},
				},
			},
		},
	}

	existingJobs := &batchv1.JobList{}

	job, err := reconciler.constructJobFromTemplate(context.Background(), tuningJob, existingJobs, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parametersJSON := job.Annotations[ParameterAnnotationKey]
	if parametersJSON == "" {
		t.Fatal("expected parameters annotation, got empty string")
	}

	var assignments map[string]string
	if err := json.Unmarshal([]byte(parametersJSON), &assignments); err != nil {
		t.Fatalf("failed to unmarshal parameters: %v", err)
	}

	if assignments["learning_rate"] != "0.025" {
		t.Errorf("expected learning_rate=0.025 from fake suggester, got %s", assignments["learning_rate"])
	}

	if assignments["optimizer"] != "adam" {
		t.Errorf("expected optimizer=adam from fake suggester, got %s", assignments["optimizer"])
	}

	var envVars []corev1.EnvVar
	for _, container := range job.Spec.Template.Spec.Containers {
		envVars = append(envVars, container.Env...)
	}

	foundLR := false
	foundOpt := false
	for _, env := range envVars {
		if env.Name == "TUNING_PARAM_learning_rate" && env.Value == "0.025" {
			foundLR = true
		}
		if env.Name == "TUNING_PARAM_optimizer" && env.Value == "adam" {
			foundOpt = true
		}
	}

	if !foundLR {
		t.Error("expected TUNING_PARAM_learning_rate=0.025 in container env")
	}
	if !foundOpt {
		t.Error("expected TUNING_PARAM_optimizer=adam in container env")
	}
}

func TestConstructJobFromTemplate_SuggesterError(t *testing.T) {
	failingSuggester := func(ctx context.Context, input SuggesterInput) (SuggesterOutput, error) {
		return SuggesterOutput{}, fmt.Errorf("suggester unavailable")
	}

	reconciler := &TuningJobReconciler{
		Sampler:       sampling.NewSampler(rand.New(rand.NewSource(42))),
		Scheme:        setupScheme(),
		SuggesterFunc: failingSuggester,
	}

	min := "0.001"
	max := "0.1"
	tuningJob := &tuningv1alpha1.TuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tuning",
			Namespace: "default",
		},
		Spec: tuningv1alpha1.TuningJobSpec{
			MaxTrials: 5,
			Goal:      "maximize",
			Parameters: []tuningv1alpha1.ParameterSpec{
				{
					Name: "learning_rate",
					Min:  &min,
					Max:  &max,
				},
			},
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "training",
									Image: "training:latest",
								},
							},
						},
					},
				},
			},
		},
	}

	existingJobs := &batchv1.JobList{}

	_, err := reconciler.constructJobFromTemplate(context.Background(), tuningJob, existingJobs, 0)
	if err == nil {
		t.Fatal("expected error when suggester fails, got nil")
	}

	if !strings.Contains(err.Error(), "suggester failed") {
		t.Errorf("expected error about suggester failure, got: %v", err)
	}
}

func TestConstructJobFromTemplate_SuggesterReceivesPastTrials(t *testing.T) {
	var receivedInput SuggesterInput

	capturingSuggester := func(ctx context.Context, input SuggesterInput) (SuggesterOutput, error) {
		receivedInput = input
		return SuggesterOutput{
			Parameters: map[string]string{
				"learning_rate": "0.03",
			},
		}, nil
	}

	reconciler := &TuningJobReconciler{
		Sampler:       sampling.NewSampler(rand.New(rand.NewSource(42))),
		Scheme:        setupScheme(),
		SuggesterFunc: capturingSuggester,
		Recorder:      record.NewFakeRecorder(10),
	}

	min := "0.001"
	max := "0.1"
	tuningJob := &tuningv1alpha1.TuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tuning",
			Namespace: "default",
		},
		Spec: tuningv1alpha1.TuningJobSpec{
			MaxTrials: 5,
			Goal:      "maximize",
			Parameters: []tuningv1alpha1.ParameterSpec{
				{
					Name: "learning_rate",
					Min:  &min,
					Max:  &max,
				},
			},
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "training",
									Image: "training:latest",
								},
							},
						},
					},
				},
			},
		},
	}

	existingJobs := &batchv1.JobList{}

	_, err := reconciler.constructJobFromTemplate(context.Background(), tuningJob, existingJobs, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedInput.Goal != "maximize" {
		t.Errorf("expected goal=maximize, got %s", receivedInput.Goal)
	}

	if len(receivedInput.Parameters) != 1 {
		t.Errorf("expected 1 parameter spec, got %d", len(receivedInput.Parameters))
	}
}

func TestCollectPastTrials_EmptyWithNoCompletedJobs(t *testing.T) {
	reconciler := &TuningJobReconciler{
		Sampler:  sampling.NewSampler(rand.New(rand.NewSource(42))),
		Scheme:   setupScheme(),
		Recorder: record.NewFakeRecorder(10),
	}

	tuningJob := &tuningv1alpha1.TuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tuning",
			Namespace: "default",
		},
		Spec: tuningv1alpha1.TuningJobSpec{
			MaxTrials: 5,
			Goal:      "maximize",
		},
	}

	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tuning-trial-0",
				Namespace: "default",
				Labels: map[string]string{
					"tuning.dev/trial-index": "0",
				},
			},
			Status: batchv1.JobStatus{
				Active: 1,
			},
		},
	}

	pastTrials := reconciler.collectPastTrials(context.Background(), tuningJob, jobs)

	if len(pastTrials) != 0 {
		t.Errorf("expected 0 past trials for incomplete jobs, got %d", len(pastTrials))
	}
}

func TestProductionSuggester_ValidResponse(t *testing.T) {
	t.Log("Building suggester binary for integration test...")
	suggesterPath := "../../bin/suggester-test"

	min := "0.001"
	max := "0.1"
	input := SuggesterInput{
		Parameters: []tuningv1alpha1.ParameterSpec{
			{
				Name: "learning_rate",
				Min:  &min,
				Max:  &max,
			},
		},
		Goal:       "maximize",
		PastTrials: []PastTrial{},
	}

	suggesterFunc := NewProductionSuggester(suggesterPath)

	output, err := suggesterFunc(context.Background(), input)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "no such file") {
			t.Skip("Suggester binary not found at ../../bin/suggester-test - run 'go build -o bin/suggester cmd/suggester/*.go' first")
		}
		t.Fatalf("expected no error from production suggester, got %v", err)
	}

	if len(output.Parameters) != 1 {
		t.Errorf("expected 1 parameter in output, got %d", len(output.Parameters))
	}

	lr, ok := output.Parameters["learning_rate"]
	if !ok {
		t.Fatal("expected learning_rate in output parameters")
	}

	lrVal, err := strconv.ParseFloat(lr, 64)
	if err != nil {
		t.Errorf("learning_rate is not a valid float: %v", err)
	}

	if lrVal < 0.001 || lrVal > 0.1 {
		t.Errorf("learning_rate %f out of expected range [0.001, 0.1]", lrVal)
	}

	t.Logf("Production suggester successfully returned: %v", output.Parameters)
}
