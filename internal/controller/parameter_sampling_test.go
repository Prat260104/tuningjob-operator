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
	"math/rand"
	"strconv"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
	"github.com/Prat260104/tuningjob-operator/internal/sampling"
)

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = tuningv1alpha1.AddToScheme(s)
	return s
}

func TestSampleParameter_Categorical(t *testing.T) {
	sampler := sampling.NewSampler(rand.New(rand.NewSource(42)))

	param := tuningv1alpha1.ParameterSpec{
		Name:   "optimizer",
		Values: []string{"sgd", "adam", "rmsprop"},
	}

	value, err := sampler.SampleParameter(param)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	found := false
	for _, v := range param.Values {
		if v == value {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sampled value %s not in allowed values %v", value, param.Values)
	}

	if value != "sgd" && value != "adam" && value != "rmsprop" {
		t.Errorf("sampled value %s is not one of the expected categorical values", value)
	}
}

func TestSampleParameter_Numeric(t *testing.T) {
	sampler := sampling.NewSampler(rand.New(rand.NewSource(42)))

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
		t.Fatalf("expected numeric value, got %s", value)
	}

	minVal, _ := strconv.ParseFloat(min, 64)
	maxVal, _ := strconv.ParseFloat(max, 64)

	if floatVal < minVal || floatVal > maxVal {
		t.Errorf("sampled value %f not in range [%f, %f]", floatVal, minVal, maxVal)
	}

	if floatVal < 0.001 || floatVal > 0.1 {
		t.Errorf("sampled learning_rate %f not within expected range [0.001, 0.1]", floatVal)
	}
}

func TestSampleParameter_Deterministic(t *testing.T) {
	seed := int64(12345)

	sampler1 := sampling.NewSampler(rand.New(rand.NewSource(seed)))
	sampler2 := sampling.NewSampler(rand.New(rand.NewSource(seed)))

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

func TestSampleParameter_BothFormsSet(t *testing.T) {
	sampler := sampling.NewSampler(rand.New(rand.NewSource(42)))

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

	expectedErrMsg := "has both Values and Min/Max set"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error containing %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestSampleParameter_NeitherFormSet(t *testing.T) {
	sampler := sampling.NewSampler(rand.New(rand.NewSource(42)))

	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error when neither Values nor Min/Max are set, got nil")
	}

	expectedErrMsg := "must have either Values or both Min and Max"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error containing %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestSampleParameter_OnlyMinSet(t *testing.T) {
	sampler := sampling.NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	param := tuningv1alpha1.ParameterSpec{
		Name: "invalid_param",
		Min:  &min,
	}

	_, err := sampler.SampleParameter(param)
	if err == nil {
		t.Fatal("expected error when only Min is set, got nil")
	}
}

func TestSampleParameters_Multiple(t *testing.T) {
	sampler := sampling.NewSampler(rand.New(rand.NewSource(42)))

	min := "0.001"
	max := "0.1"
	params := []tuningv1alpha1.ParameterSpec{
		{
			Name:   "optimizer",
			Values: []string{"sgd", "adam"},
		},
		{
			Name: "learning_rate",
			Min:  &min,
			Max:  &max,
		},
	}

	assignments, err := sampler.SampleParameters(params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(assignments) != 2 {
		t.Errorf("expected 2 assignments, got %d", len(assignments))
	}

	optimizerValue, ok := assignments["optimizer"]
	if !ok {
		t.Error("missing optimizer assignment")
	} else {
		if optimizerValue != "sgd" && optimizerValue != "adam" {
			t.Errorf("optimizer value %s not in expected values [sgd, adam]", optimizerValue)
		}
	}

	learningRateValue, ok := assignments["learning_rate"]
	if !ok {
		t.Error("missing learning_rate assignment")
	} else {
		lr, err := strconv.ParseFloat(learningRateValue, 64)
		if err != nil {
			t.Errorf("learning_rate value %s is not a valid float", learningRateValue)
		} else if lr < 0.001 || lr > 0.1 {
			t.Errorf("learning_rate value %f not in range [0.001, 0.1]", lr)
		}
	}
}

func TestInjectParametersIntoJob(t *testing.T) {
	job := &batchv1.Job{
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "trial",
							Image: "test:latest",
						},
					},
				},
			},
		},
	}

	assignments := ParameterAssignment{
		"learning_rate": "0.01",
		"batch_size":    "32",
		"optimizer":     "adam",
	}

	err := injectParametersIntoJob(job, assignments)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	envVars := job.Spec.Template.Spec.Containers[0].Env
	if len(envVars) != 3 {
		t.Errorf("expected 3 env vars, got %d", len(envVars))
	}

	envMap := make(map[string]string)
	for _, env := range envVars {
		envMap[env.Name] = env.Value
	}

	expectedEnvs := map[string]string{
		"TUNING_PARAM_learning_rate": "0.01",
		"TUNING_PARAM_batch_size":    "32",
		"TUNING_PARAM_optimizer":     "adam",
	}

	for expectedName, expectedValue := range expectedEnvs {
		if actualValue, ok := envMap[expectedName]; !ok {
			t.Errorf("missing env var %s", expectedName)
		} else if actualValue != expectedValue {
			t.Errorf("expected %s=%s, got %s=%s", expectedName, expectedValue, expectedName, actualValue)
		}
	}
}

func TestInjectParametersIntoJob_MultipleContainers(t *testing.T) {
	job := &batchv1.Job{
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "test:latest"},
						{Name: "sidecar", Image: "sidecar:latest"},
					},
				},
			},
		},
	}

	assignments := ParameterAssignment{
		"param1": "value1",
	}

	err := injectParametersIntoJob(job, assignments)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for i, container := range job.Spec.Template.Spec.Containers {
		found := false
		for _, env := range container.Env {
			if env.Name == "TUNING_PARAM_param1" && env.Value == "value1" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("container %d (%s) missing expected env var", i, container.Name)
		}
	}
}

func TestConstructJobFromTemplate_WithParameters(t *testing.T) {
	reconciler := &TuningJobReconciler{
		Sampler: sampling.NewSampler(rand.New(rand.NewSource(42))),
		Scheme:  setupScheme(),
	}

	min := "0.001"
	max := "0.1"
	tuningJob := &tuningv1alpha1.TuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: tuningv1alpha1.TuningJobSpec{
			MaxTrials: 3,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "trial",
									Image: "test:latest",
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
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
		},
	}

	job, err := reconciler.constructJobFromTemplate(context.Background(), tuningJob, &batchv1.JobList{}, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if job.Annotations[ParameterAnnotationKey] == "" {
		t.Error("expected parameter annotation to be set")
	}

	var assignments ParameterAssignment
	err = json.Unmarshal([]byte(job.Annotations[ParameterAnnotationKey]), &assignments)
	if err != nil {
		t.Fatalf("failed to unmarshal parameter annotation: %v", err)
	}

	if len(assignments) != 2 {
		t.Errorf("expected 2 parameter assignments, got %d", len(assignments))
	}

	if _, ok := assignments["learning_rate"]; !ok {
		t.Error("missing learning_rate in assignments")
	}

	if _, ok := assignments["optimizer"]; !ok {
		t.Error("missing optimizer in assignments")
	}

	envVars := job.Spec.Template.Spec.Containers[0].Env
	if len(envVars) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(envVars))
	}
}

func TestConstructJobFromTemplate_NoParameters(t *testing.T) {
	reconciler := &TuningJobReconciler{
		Scheme: setupScheme(),
	}

	tuningJob := &tuningv1alpha1.TuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: tuningv1alpha1.TuningJobSpec{
			MaxTrials: 3,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "trial",
									Image: "test:latest",
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}

	job, err := reconciler.constructJobFromTemplate(context.Background(), tuningJob, &batchv1.JobList{}, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if job.Annotations[ParameterAnnotationKey] != "" {
		t.Error("expected no parameter annotation when no parameters defined")
	}

	envVars := job.Spec.Template.Spec.Containers[0].Env
	if len(envVars) != 0 {
		t.Errorf("expected 0 env vars, got %d", len(envVars))
	}
}
