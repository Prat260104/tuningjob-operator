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
	"testing"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
)

func TestIsBetter_Maximize(t *testing.T) {
	tests := []struct {
		name         string
		newValue     float64
		currentValue float64
		expected     bool
	}{
		{"higher is better", 0.95, 0.90, true},
		{"lower is worse", 0.85, 0.90, false},
		{"equal is not better", 0.90, 0.90, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBetter(tt.newValue, tt.currentValue, "maximize")
			if result != tt.expected {
				t.Errorf("isBetter(%f, %f, maximize) = %v, want %v", tt.newValue, tt.currentValue, result, tt.expected)
			}
		})
	}
}

func TestIsBetter_Minimize(t *testing.T) {
	tests := []struct {
		name         string
		newValue     float64
		currentValue float64
		expected     bool
	}{
		{"lower is better", 0.05, 0.10, true},
		{"higher is worse", 0.15, 0.10, false},
		{"equal is not better", 0.10, 0.10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBetter(tt.newValue, tt.currentValue, "minimize")
			if result != tt.expected {
				t.Errorf("isBetter(%f, %f, minimize) = %v, want %v", tt.newValue, tt.currentValue, result, tt.expected)
			}
		})
	}
}

func TestShouldUpdateBestTrial_NoBestYet(t *testing.T) {
	tuningJob := &tuningv1alpha1.TuningJob{
		Spec: tuningv1alpha1.TuningJobSpec{
			Goal: "maximize",
		},
		Status: tuningv1alpha1.TuningJobStatus{
			BestTrial: nil,
		},
	}

	newTrial := &tuningv1alpha1.TrialResult{
		TrialName:   "test-trial-0",
		MetricValue: "0.85",
	}

	result := shouldUpdateBestTrial(tuningJob, newTrial, 0.85)
	if !result {
		t.Error("should update when no best trial exists")
	}
}

func TestShouldUpdateBestTrial_Maximize(t *testing.T) {
	tuningJob := &tuningv1alpha1.TuningJob{
		Spec: tuningv1alpha1.TuningJobSpec{
			Goal: "maximize",
		},
		Status: tuningv1alpha1.TuningJobStatus{
			BestTrial: &tuningv1alpha1.TrialResult{
				TrialName:   "test-trial-0",
				MetricValue: "0.85",
			},
		},
	}

	tests := []struct {
		name         string
		newMetric    float64
		metricStr    string
		shouldUpdate bool
	}{
		{"better metric", 0.90, "0.90", true},
		{"worse metric", 0.80, "0.80", false},
		{"equal metric", 0.85, "0.85", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTrial := &tuningv1alpha1.TrialResult{
				TrialName:   "test-trial-new",
				MetricValue: tt.metricStr,
			}

			result := shouldUpdateBestTrial(tuningJob, newTrial, tt.newMetric)
			if result != tt.shouldUpdate {
				t.Errorf("shouldUpdateBestTrial with metric %f = %v, want %v", tt.newMetric, result, tt.shouldUpdate)
			}
		})
	}
}

func TestShouldUpdateBestTrial_Minimize(t *testing.T) {
	tuningJob := &tuningv1alpha1.TuningJob{
		Spec: tuningv1alpha1.TuningJobSpec{
			Goal: "minimize",
		},
		Status: tuningv1alpha1.TuningJobStatus{
			BestTrial: &tuningv1alpha1.TrialResult{
				TrialName:   "test-trial-0",
				MetricValue: "0.15",
			},
		},
	}

	tests := []struct {
		name         string
		newMetric    float64
		metricStr    string
		shouldUpdate bool
	}{
		{"better metric (lower)", 0.10, "0.10", true},
		{"worse metric (higher)", 0.20, "0.20", false},
		{"equal metric", 0.15, "0.15", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTrial := &tuningv1alpha1.TrialResult{
				TrialName:   "test-trial-new",
				MetricValue: tt.metricStr,
			}

			result := shouldUpdateBestTrial(tuningJob, newTrial, tt.newMetric)
			if result != tt.shouldUpdate {
				t.Errorf("shouldUpdateBestTrial with metric %f = %v, want %v", tt.newMetric, result, tt.shouldUpdate)
			}
		})
	}
}

func TestShouldUpdateBestTrial_InvalidCurrentMetric(t *testing.T) {
	tuningJob := &tuningv1alpha1.TuningJob{
		Spec: tuningv1alpha1.TuningJobSpec{
			Goal: "maximize",
		},
		Status: tuningv1alpha1.TuningJobStatus{
			BestTrial: &tuningv1alpha1.TrialResult{
				TrialName:   "test-trial-0",
				MetricValue: "invalid",
			},
		},
	}

	newTrial := &tuningv1alpha1.TrialResult{
		TrialName:   "test-trial-new",
		MetricValue: "0.85",
	}

	result := shouldUpdateBestTrial(tuningJob, newTrial, 0.85)
	if !result {
		t.Error("should update when current metric is invalid")
	}
}
