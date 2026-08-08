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

package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ParameterSpec defines a hyperparameter and its search space.
// Either Values (categorical) or both Min and Max (numeric range) must be set, but not both forms.
type ParameterSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +optional
	Min *string `json:"min,omitempty"`

	// +optional
	Max *string `json:"max,omitempty"`

	// +optional
	Values []string `json:"values,omitempty"`
}

// TuningJobSpec defines the desired state of TuningJob
type TuningJobSpec struct {
	// +kubebuilder:validation:Required
	JobTemplate batchv1.JobTemplateSpec `json:"jobTemplate"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	MaxTrials int32 `json:"maxTrials"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxFailedTrials *int32 `json:"maxFailedTrials,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	Parallelism *int32 `json:"parallelism,omitempty"`

	// +optional
	Parameters []ParameterSpec `json:"parameters,omitempty"`

	// +kubebuilder:validation:Enum=maximize;minimize
	// +kubebuilder:validation:Required
	Goal string `json:"goal"`
}

type TrialResult struct {
	// +optional
	TrialName string `json:"trialName,omitempty"`

	// +optional
	MetricValue string `json:"metricValue,omitempty"`

	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// TuningJobStatus defines the observed state of TuningJob.
type TuningJobStatus struct {
	// +optional
	Active []string `json:"active,omitempty"`

	// +optional
	Succeeded []string `json:"succeeded,omitempty"`

	// +optional
	Failed []string `json:"failed,omitempty"`

	// +optional
	TrialsLaunched int32 `json:"trialsLaunched,omitempty"`

	// +optional
	TrialsFailed int32 `json:"trialsFailed,omitempty"`

	// +optional
	Phase string `json:"phase,omitempty"`

	// +optional
	BestTrial *TrialResult `json:"bestTrial,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TuningJob is the Schema for the tuningjobs API
type TuningJob struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of TuningJob
	// +required
	Spec TuningJobSpec `json:"spec"`

	// status defines the observed state of TuningJob
	// +optional
	Status TuningJobStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TuningJobList contains a list of TuningJob
type TuningJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []TuningJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TuningJob{}, &TuningJobList{})
		return nil
	})
}
