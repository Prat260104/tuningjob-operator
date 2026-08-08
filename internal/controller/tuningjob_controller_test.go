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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	tuningv1alpha1 "github.com/Prat260104/tuningjob-operator/api/v1alpha1"
	"github.com/Prat260104/tuningjob-operator/internal/sampling"
)

var _ = Describe("TuningJob Controller", func() {
	Context("When reconciling a TuningJob", func() {
		const (
			resourceName      = "test-tuningjob"
			resourceNamespace = "default"
			maxTrials         = int32(3)
		)

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			By("Creating a TuningJob with maxTrials=3")
			parallelism := int32(3)
			tuningJob := &tuningv1alpha1.TuningJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: tuningv1alpha1.TuningJobSpec{
					MaxTrials:   maxTrials,
					Parallelism: &parallelism,
					Goal:        "maximize",
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "trial",
											Image: "busybox:latest",
											Command: []string{
												"sh", "-c", "echo trial complete",
											},
										},
									},
									RestartPolicy: corev1.RestartPolicyNever,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, tuningJob)).To(Succeed())
		})

		AfterEach(func() {
			tuningJob := &tuningv1alpha1.TuningJob{}
			err := k8sClient.Get(ctx, typeNamespacedName, tuningJob)
			if err == nil {
				By("Cleaning up the TuningJob")
				Expect(k8sClient.Delete(ctx, tuningJob)).To(Succeed())
			}
		})

		It("should create child Jobs up to maxTrials", func() {
			reconciler := &TuningJobReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
				Sampler:  sampling.NewSampler(rand.New(rand.NewSource(42))),
			}

			By("Reconciling until all trials are created")
			Eventually(func() int32 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				tuningJob := &tuningv1alpha1.TuningJob{}
				err = k8sClient.Get(ctx, typeNamespacedName, tuningJob)
				if err != nil {
					return 0
				}
				return tuningJob.Status.TrialsLaunched
			}, time.Second*30, time.Millisecond*200).Should(Equal(maxTrials))

			By("Verifying the TuningJob status")
			tuningJob := &tuningv1alpha1.TuningJob{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, tuningJob)).To(Succeed())
			Expect(tuningJob.Status.TrialsLaunched).To(Equal(maxTrials))

			By("Verifying child Jobs were created")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList)).To(Succeed())

			jobsForTuningJob := 0
			for _, job := range jobList.Items {
				if job.Labels["tuning.dev/tuning-job"] == resourceName {
					jobsForTuningJob++
				}
			}
			Expect(jobsForTuningJob).To(Equal(int(maxTrials)))
		})
	})

	Context("When reconciling a TuningJob with parameters", func() {
		const (
			resourceName      = "test-tuningjob-params"
			resourceNamespace = "default"
			maxTrials         = int32(2)
		)

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			By("Creating a TuningJob with parameters")
			parallelism := int32(2)
			min := "0.001"
			max := "0.1"
			tuningJob := &tuningv1alpha1.TuningJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: tuningv1alpha1.TuningJobSpec{
					MaxTrials:   maxTrials,
					Parallelism: &parallelism,
					Goal:        "maximize",
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "trial",
											Image: "busybox:latest",
											Command: []string{
												"sh", "-c", "echo trial complete",
											},
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
							Values: []string{"sgd", "adam", "rmsprop"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, tuningJob)).To(Succeed())
		})

		AfterEach(func() {
			tuningJob := &tuningv1alpha1.TuningJob{}
			err := k8sClient.Get(ctx, typeNamespacedName, tuningJob)
			if err == nil {
				By("Cleaning up the TuningJob")
				Expect(k8sClient.Delete(ctx, tuningJob)).To(Succeed())
			}
		})

		It("should inject parameters as env vars and store in annotations", func() {
			reconciler := &TuningJobReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
				Sampler:  sampling.NewSampler(rand.New(rand.NewSource(12345))),
			}

			By("Reconciling until all trials are created")
			Eventually(func() int32 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				tuningJob := &tuningv1alpha1.TuningJob{}
				err = k8sClient.Get(ctx, typeNamespacedName, tuningJob)
				if err != nil {
					return 0
				}
				return tuningJob.Status.TrialsLaunched
			}, time.Second*30, time.Millisecond*200).Should(Equal(maxTrials))

			By("Verifying parameters are injected in Jobs")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList)).To(Succeed())

			jobsWithParams := 0
			for _, job := range jobList.Items {
				if job.Labels["tuning.dev/tuning-job"] != resourceName {
					continue
				}

				jobsWithParams++

				By("Checking parameter annotation exists")
				Expect(job.Annotations).To(HaveKey(ParameterAnnotationKey))

				var assignments ParameterAssignment
				err := json.Unmarshal([]byte(job.Annotations[ParameterAnnotationKey]), &assignments)
				Expect(err).NotTo(HaveOccurred())
				Expect(assignments).To(HaveKey("learning_rate"))
				Expect(assignments).To(HaveKey("optimizer"))

				By("Checking env vars are injected")
				Expect(job.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				envVars := job.Spec.Template.Spec.Containers[0].Env

				envMap := make(map[string]string)
				for _, env := range envVars {
					envMap[env.Name] = env.Value
				}

				Expect(envMap).To(HaveKey("TUNING_PARAM_learning_rate"))
				Expect(envMap).To(HaveKey("TUNING_PARAM_optimizer"))
				Expect(envMap["TUNING_PARAM_learning_rate"]).To(Equal(assignments["learning_rate"]))
				Expect(envMap["TUNING_PARAM_optimizer"]).To(Equal(assignments["optimizer"]))
			}

			Expect(jobsWithParams).To(Equal(int(maxTrials)))
		})
	})

	Context("When tracking trial results", func() {
		const (
			resourceName      = "test-tuningjob-results"
			resourceNamespace = "default"
		)

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			By("Creating a TuningJob")
			tuningJob := &tuningv1alpha1.TuningJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: tuningv1alpha1.TuningJobSpec{
					MaxTrials: 5,
					Goal:      "maximize",
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "trial",
											Image: "busybox:latest",
										},
									},
									RestartPolicy: corev1.RestartPolicyNever,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, tuningJob)).To(Succeed())
		})

		AfterEach(func() {
			tuningJob := &tuningv1alpha1.TuningJob{}
			err := k8sClient.Get(ctx, typeNamespacedName, tuningJob)
			if err == nil {
				By("Cleaning up the TuningJob")
				Expect(k8sClient.Delete(ctx, tuningJob)).To(Succeed())
			}

			cmList := &corev1.ConfigMapList{}
			_ = k8sClient.List(ctx, cmList)
			for _, cm := range cmList.Items {
				if strings.Contains(cm.Name, resourceName) {
					_ = k8sClient.Delete(ctx, &cm)
				}
			}
		})

		It("should track best trial based on results", func() {
			reconciler := &TuningJobReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
				Sampler:  sampling.NewSampler(rand.New(rand.NewSource(42))),
			}

			By("Creating a completed Job with parameters")
			jobName := resourceName + "-trial-0"
			params := map[string]string{
				"learning_rate": "0.01",
				"optimizer":     "adam",
			}
			paramsJSON, _ := json.Marshal(params)

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: resourceNamespace,
					Labels: map[string]string{
						"tuning.dev/tuning-job":  resourceName,
						"tuning.dev/trial-index": "0",
					},
					Annotations: map[string]string{
						ParameterAnnotationKey: string(paramsJSON),
					},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "trial",
									Image: "busybox:latest",
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}

			tuningJob := &tuningv1alpha1.TuningJob{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, tuningJob)).To(Succeed())
			Expect(ctrl.SetControllerReference(tuningJob, job, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Create(ctx, job)).To(Succeed())

			By("Updating job status to completed")
			now := metav1.Now()
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: resourceNamespace}, job); err != nil {
					return err
				}
				job.Status.StartTime = &now
				job.Status.CompletionTime = &now
				job.Status.Conditions = []batchv1.JobCondition{
					{
						Type:   batchv1.JobSuccessCriteriaMet,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				}
				return k8sClient.Status().Update(ctx, job)
			}, time.Second*5).Should(Succeed())

			By("Creating result ConfigMap")
			resultCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName + "-trial-0-result",
					Namespace: resourceNamespace,
				},
				Data: map[string]string{
					"metric": "0.87",
				},
			}
			Expect(k8sClient.Create(ctx, resultCM)).To(Succeed())

			By("Reconciling to process the result")
			Eventually(func() bool {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				tuningJob := &tuningv1alpha1.TuningJob{}
				err = k8sClient.Get(ctx, typeNamespacedName, tuningJob)
				if err != nil {
					return false
				}
				return tuningJob.Status.BestTrial != nil
			}, time.Second*10, time.Millisecond*200).Should(BeTrue())

			By("Verifying best trial is recorded")
			Expect(k8sClient.Get(ctx, typeNamespacedName, tuningJob)).To(Succeed())
			Expect(tuningJob.Status.BestTrial).NotTo(BeNil())
			Expect(tuningJob.Status.BestTrial.TrialName).To(Equal(jobName))
			Expect(tuningJob.Status.BestTrial.MetricValue).To(Equal("0.87"))
			Expect(tuningJob.Status.BestTrial.Parameters).To(Equal(params))
		})
	})
})
