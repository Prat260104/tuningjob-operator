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

// Package controller implements the TuningJob Kubernetes controller.
// It reconciles TuningJob resources by creating trial Jobs with injected hyperparameters,
// tracking trial results via ConfigMaps, and identifying the best-performing configuration.
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
	"github.com/Prat260104/tuningjob-operator/internal/sampling"
)

type TuningJobReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	Sampler       *sampling.Sampler
	SuggesterFunc func(ctx context.Context, input SuggesterInput) (SuggesterOutput, error)
}

type SuggesterInput struct {
	Parameters []tuningv1alpha1.ParameterSpec
	Goal       string
	PastTrials []PastTrial
}

type PastTrial struct {
	Parameters map[string]string
	Metric     string
}

type SuggesterOutput struct {
	Parameters map[string]string
}

type ParameterAssignment = sampling.Assignment

const (
	ParameterAnnotationKey = "tuning.dev/parameters"
	ParameterEnvPrefix     = "TUNING_PARAM_"
	ResultConfigMapKey     = "metric"
)

// +kubebuilder:rbac:groups=tuning.tuning.dev,resources=tuningjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tuning.tuning.dev,resources=tuningjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tuning.tuning.dev,resources=tuningjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

func (r *TuningJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var tuningJob tuningv1alpha1.TuningJob
	if err := r.Get(ctx, req.NamespacedName, &tuningJob); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get TuningJob")
		return ctrl.Result{}, err
	}

	defer func() {
		if err := r.Status().Update(ctx, &tuningJob); err != nil {
			log.Error(err, "Failed to update TuningJob status")
		}
	}()

	if tuningJob.Spec.Goal != "maximize" && tuningJob.Spec.Goal != "minimize" {
		log.Error(nil, "Invalid goal", "goal", tuningJob.Spec.Goal)
		return ctrl.Result{}, fmt.Errorf("goal must be either 'maximize' or 'minimize', got %s", tuningJob.Spec.Goal)
	}

	var childJobs batchv1.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "Failed to list child Jobs")
		return ctrl.Result{}, err
	}

	activeJobs := []string{}
	succeededJobs := []string{}
	failedJobs := []string{}

	for _, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "":
			activeJobs = append(activeJobs, job.Name)
		case batchv1.JobComplete:
			succeededJobs = append(succeededJobs, job.Name)
			if err := r.processCompletedTrial(ctx, &tuningJob, &job); err != nil {
				log.Error(err, "Failed to process completed trial", "job", job.Name)
			}
		case batchv1.JobFailed:
			failedJobs = append(failedJobs, job.Name)
		}
	}

	tuningJob.Status.Active = activeJobs
	tuningJob.Status.Succeeded = succeededJobs
	tuningJob.Status.Failed = failedJobs
	tuningJob.Status.TrialsLaunched = int32(len(childJobs.Items))
	tuningJob.Status.TrialsFailed = int32(len(failedJobs))

	maxFailures := int32(0)
	if tuningJob.Spec.MaxFailedTrials != nil {
		maxFailures = *tuningJob.Spec.MaxFailedTrials
	}

	if tuningJob.Status.TrialsFailed >= maxFailures && maxFailures > 0 {
		tuningJob.Status.Phase = "Failed"
		r.Recorder.Event(&tuningJob, corev1.EventTypeWarning, "MaxFailuresReached", "Maximum failed trials reached")
		return ctrl.Result{}, nil
	}

	if tuningJob.Status.TrialsLaunched >= tuningJob.Spec.MaxTrials {
		tuningJob.Status.Phase = "Succeeded"
		return ctrl.Result{}, nil
	}

	parallelism := int32(1)
	if tuningJob.Spec.Parallelism != nil {
		parallelism = *tuningJob.Spec.Parallelism
	}

	if int32(len(activeJobs)) >= parallelism {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	trialIndex := tuningJob.Status.TrialsLaunched
	job, err := r.constructJobFromTemplate(ctx, &tuningJob, &childJobs, trialIndex)
	if err != nil {
		log.Error(err, "Failed to construct Job from template")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "Failed to create Job", "job", job.Name)
		return ctrl.Result{}, err
	}

	log.Info("Created Job", "job", job.Name, "trial", trialIndex)
	r.Recorder.Eventf(&tuningJob, corev1.EventTypeNormal, "JobCreated", "Created trial Job %s", job.Name)

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *TuningJobReconciler) constructJobFromTemplate(ctx context.Context, tuningJob *tuningv1alpha1.TuningJob, childJobs *batchv1.JobList, trialIndex int32) (*batchv1.Job, error) {
	name := fmt.Sprintf("%s-trial-%d", tuningJob.Name, trialIndex)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   tuningJob.Namespace,
		},
		Spec: *tuningJob.Spec.JobTemplate.Spec.DeepCopy(),
	}

	for k, v := range tuningJob.Spec.JobTemplate.Annotations {
		job.Annotations[k] = v
	}
	for k, v := range tuningJob.Spec.JobTemplate.Labels {
		job.Labels[k] = v
	}
	job.Labels["tuning.dev/tuning-job"] = tuningJob.Name
	job.Labels["tuning.dev/trial-index"] = fmt.Sprintf("%d", trialIndex)

	if len(tuningJob.Spec.Parameters) > 0 {
		var assignments sampling.Assignment
		var err error
		log := log.FromContext(ctx)

		if r.SuggesterFunc != nil {
			pastTrials, err := r.collectPastTrials(ctx, tuningJob, childJobs.Items)
			if err != nil {
				return nil, fmt.Errorf("failed to collect past trials: %w", err)
			}

			input := SuggesterInput{
				Parameters: tuningJob.Spec.Parameters,
				Goal:       tuningJob.Spec.Goal,
				PastTrials: pastTrials,
			}

			output, err := r.SuggesterFunc(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("suggester failed: %w", err)
			}
			assignments = output.Parameters
		} else {
			log.Info("SuggesterFunc not configured, falling back to random sampling", "tuningjob", tuningJob.Name)
			assignments, err = r.Sampler.SampleParameters(tuningJob.Spec.Parameters)
			if err != nil {
				return nil, fmt.Errorf("failed to sample parameters: %w", err)
			}
		}

		parametersJSON, err := json.Marshal(assignments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal parameters: %w", err)
		}
		job.Annotations[ParameterAnnotationKey] = string(parametersJSON)

		if err := injectParametersIntoJob(job, assignments); err != nil {
			return nil, fmt.Errorf("failed to inject parameters: %w", err)
		}
	}

	if err := ctrl.SetControllerReference(tuningJob, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func injectParametersIntoJob(job *batchv1.Job, assignments ParameterAssignment) error {
	if len(job.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("job template must have at least one container")
	}

	for i := range job.Spec.Template.Spec.Containers {
		container := &job.Spec.Template.Spec.Containers[i]
		for paramName, paramValue := range assignments {
			envVarName := ParameterEnvPrefix + paramName
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  envVarName,
				Value: paramValue,
			})
		}
	}

	return nil
}

func isJobFinished(job *batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}

// Note: re-reads and re-compares already-processed trials on every reconcile; harmless due to strict > / < comparison
// in isBetter, but not optimized to skip already-recorded results.
func (r *TuningJobReconciler) processCompletedTrial(ctx context.Context, tuningJob *tuningv1alpha1.TuningJob, job *batchv1.Job) error {
	log := log.FromContext(ctx)

	trialIndex := job.Labels["tuning.dev/trial-index"]
	configMapName := fmt.Sprintf("%s-trial-%s-result", tuningJob.Name, trialIndex)

	var resultCM corev1.ConfigMap
	cmKey := client.ObjectKey{
		Namespace: tuningJob.Namespace,
		Name:      configMapName,
	}

	if err := r.Get(ctx, cmKey, &resultCM); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Result ConfigMap not found for trial", "job", job.Name, "configmap", configMapName)
			return nil
		}
		return fmt.Errorf("failed to get result ConfigMap: %w", err)
	}

	metricStr, ok := resultCM.Data[ResultConfigMapKey]
	if !ok {
		log.Info("Result ConfigMap missing metric key", "job", job.Name, "configmap", configMapName)
		return nil
	}

	metricValue, err := strconv.ParseFloat(metricStr, 64)
	if err != nil {
		log.Error(err, "Failed to parse metric value", "job", job.Name, "metric", metricStr)
		return nil
	}

	parametersJSON := job.Annotations[ParameterAnnotationKey]
	var parameters map[string]string
	if parametersJSON != "" {
		if err := json.Unmarshal([]byte(parametersJSON), &parameters); err != nil {
			log.Error(err, "Failed to unmarshal parameters from job annotation", "job", job.Name)
			return nil
		}
	}

	newTrial := &tuningv1alpha1.TrialResult{
		TrialName:   job.Name,
		MetricValue: metricStr,
		Parameters:  parameters,
	}

	if shouldUpdateBestTrial(tuningJob, newTrial, metricValue) {
		tuningJob.Status.BestTrial = newTrial
		log.Info("Updated best trial", "job", job.Name, "metric", metricStr)
	}

	return nil
}

func (r *TuningJobReconciler) collectPastTrials(ctx context.Context, tuningJob *tuningv1alpha1.TuningJob, jobs []batchv1.Job) ([]PastTrial, error) {
	var pastTrials []PastTrial

	for _, job := range jobs {
		isFinished, finishedType := isJobFinished(&job)
		if !isFinished || finishedType != batchv1.JobComplete {
			continue
		}

		trialIndex := job.Labels["tuning.dev/trial-index"]
		configMapName := fmt.Sprintf("%s-trial-%s-result", tuningJob.Name, trialIndex)

		var resultCM corev1.ConfigMap
		cmKey := client.ObjectKey{
			Namespace: tuningJob.Namespace,
			Name:      configMapName,
		}

		if err := r.Get(ctx, cmKey, &resultCM); err != nil {
			continue
		}

		metricStr, ok := resultCM.Data[ResultConfigMapKey]
		if !ok {
			continue
		}

		parametersJSON := job.Annotations[ParameterAnnotationKey]
		var parameters map[string]string
		if parametersJSON != "" {
			if err := json.Unmarshal([]byte(parametersJSON), &parameters); err != nil {
				continue
			}
		}

		pastTrials = append(pastTrials, PastTrial{
			Parameters: parameters,
			Metric:     metricStr,
		})
	}

	return pastTrials, nil
}

func shouldUpdateBestTrial(tuningJob *tuningv1alpha1.TuningJob, newTrial *tuningv1alpha1.TrialResult, newMetric float64) bool {
	if tuningJob.Status.BestTrial == nil {
		return true
	}

	currentMetricStr := tuningJob.Status.BestTrial.MetricValue
	currentMetric, err := strconv.ParseFloat(currentMetricStr, 64)
	if err != nil {
		return true
	}

	return isBetter(newMetric, currentMetric, tuningJob.Spec.Goal)
}

func isBetter(newValue, currentValue float64, goal string) bool {
	if goal == "maximize" {
		return newValue > currentValue
	}
	return newValue < currentValue
}

const jobOwnerKey = ".metadata.controller"

func (r *TuningJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &batchv1.Job{}, jobOwnerKey, func(rawObj client.Object) []string {
		job := rawObj.(*batchv1.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != tuningv1alpha1.GroupVersion.String() || owner.Kind != "TuningJob" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&tuningv1alpha1.TuningJob{}).
		Owns(&batchv1.Job{}).
		Named("tuningjob").
		Complete(r)
}
