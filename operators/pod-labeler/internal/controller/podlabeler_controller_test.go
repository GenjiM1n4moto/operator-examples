/*
Copyright 2025.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	labelsv1 "github.com/rayhe/operator-example/operators/pod-labeler/api/v1"
)

var _ = Describe("PodLabeler Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	ctx := context.Background()

	Context("When applying static labels to pods", func() {
		It("should add labels to matching pods", func() {
			labelerName := "test-labeler-static"
			podName := "test-pod-1"
			namespace := "default"

			By("Creating a test pod")
			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testPod)).To(Succeed())

			By("Creating a PodLabeler with static value")
			podLabeler := &labelsv1.PodLabeler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      labelerName,
					Namespace: namespace,
				},
				Spec: labelsv1.PodLabelerSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					LabelRules: []labelsv1.LabelRule{
						{
							Key:   "managed-by",
							Value: "pod-labeler",
						},
						{
							Key:   "environment",
							Value: "testing",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, podLabeler)).To(Succeed())

			By("Reconciling the PodLabeler")
			reconciler := &PodLabelerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      labelerName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying labels were applied to the pod")
			Eventually(func() bool {
				updatedPod := &corev1.Pod{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, updatedPod)
				if err != nil {
					return false
				}
				return updatedPod.Labels["managed-by"] == "pod-labeler" &&
					updatedPod.Labels["environment"] == "testing"
			}, timeout, interval).Should(BeTrue())

			By("Verifying status was updated")
			Eventually(func() bool {
				updatedLabeler := &labelsv1.PodLabeler{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: labelerName, Namespace: namespace}, updatedLabeler)
				if err != nil {
					return false
				}
				return updatedLabeler.Status.LabeledPodsCount == 1 &&
					len(updatedLabeler.Status.Conditions) > 0 &&
					updatedLabeler.Status.Conditions[0].Type == "Ready"
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, podLabeler)).To(Succeed())
			Expect(k8sClient.Delete(ctx, testPod)).To(Succeed())
		})
	})

	Context("When using ValueFrom for dynamic labels", func() {
		It("should extract values from namespace labels", func() {
			labelerName := "test-labeler-valuefrom"
			podName := "test-pod-2"
			namespaceName := "test-namespace"

			By("Creating a namespace with labels")
			testNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						"environment": "production",
						"team":        "platform",
					},
				},
			}
			Expect(k8sClient.Create(ctx, testNamespace)).To(Succeed())

			By("Creating a test pod in the namespace")
			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespaceName,
					Labels: map[string]string{
						"app": "myapp",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testPod)).To(Succeed())

			By("Creating a PodLabeler with valueFrom")
			podLabeler := &labelsv1.PodLabeler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      labelerName,
					Namespace: namespaceName,
				},
				Spec: labelsv1.PodLabelerSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "myapp",
						},
					},
					LabelRules: []labelsv1.LabelRule{
						{
							Key:       "auto-environment",
							ValueFrom: "namespace.labels.environment",
						},
						{
							Key:       "auto-team",
							ValueFrom: "namespace.labels.team",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, podLabeler)).To(Succeed())

			By("Reconciling the PodLabeler")
			reconciler := &PodLabelerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      labelerName,
					Namespace: namespaceName,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying labels were extracted and applied")
			Eventually(func() bool {
				updatedPod := &corev1.Pod{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespaceName}, updatedPod)
				if err != nil {
					return false
				}
				return updatedPod.Labels["auto-environment"] == "production" &&
					updatedPod.Labels["auto-team"] == "platform"
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, podLabeler)).To(Succeed())
			Expect(k8sClient.Delete(ctx, testPod)).To(Succeed())
			Expect(k8sClient.Delete(ctx, testNamespace)).To(Succeed())
		})
	})

	Context("When validating label rules", func() {
		It("should reject rules with both value and valueFrom at CRD level", func() {
			labelerName := "test-labeler-invalid"
			namespace := "default"

			By("Attempting to create a PodLabeler with invalid rules (both value and valueFrom)")
			podLabeler := &labelsv1.PodLabeler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      labelerName,
					Namespace: namespace,
				},
				Spec: labelsv1.PodLabelerSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					LabelRules: []labelsv1.LabelRule{
						{
							Key:       "invalid",
							Value:     "static-value",
							ValueFrom: "namespace.labels.something",
						},
					},
				},
			}

			By("Expecting the API server to reject the invalid CR")
			err := k8sClient.Create(ctx, podLabeler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one of value or valueFrom must be specified"))
		})

		It("should reject rules with neither value nor valueFrom", func() {
			labelerName := "test-labeler-empty"
			namespace := "default"

			By("Attempting to create a PodLabeler with empty rule")
			podLabeler := &labelsv1.PodLabeler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      labelerName,
					Namespace: namespace,
				},
				Spec: labelsv1.PodLabelerSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					LabelRules: []labelsv1.LabelRule{
						{
							Key: "empty-rule",
							// No value or valueFrom
						},
					},
				},
			}

			By("Expecting the API server to reject the invalid CR")
			err := k8sClient.Create(ctx, podLabeler)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one of value or valueFrom must be specified"))
		})
	})

	Context("When PodLabeler is deleted (Finalizer)", func() {
		It("should remove labels from pods", func() {
			labelerName := "test-labeler-finalizer"
			podName := "test-pod-3"
			namespace := "default"

			By("Creating a test pod")
			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testPod)).To(Succeed())

			By("Creating a PodLabeler")
			podLabeler := &labelsv1.PodLabeler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      labelerName,
					Namespace: namespace,
				},
				Spec: labelsv1.PodLabelerSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					LabelRules: []labelsv1.LabelRule{
						{
							Key:   "cleanup-test",
							Value: "should-be-removed",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, podLabeler)).To(Succeed())

			By("Reconciling to apply labels")
			reconciler := &PodLabelerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      labelerName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying label was applied")
			Eventually(func() bool {
				updatedPod := &corev1.Pod{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, updatedPod)
				return err == nil && updatedPod.Labels["cleanup-test"] == "should-be-removed"
			}, timeout, interval).Should(BeTrue())

			By("Deleting the PodLabeler")
			Expect(k8sClient.Delete(ctx, podLabeler)).To(Succeed())

			By("Reconciling to trigger finalizer cleanup")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      labelerName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying label was removed from pod")
			Eventually(func() bool {
				updatedPod := &corev1.Pod{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, updatedPod)
				if err != nil {
					return false
				}
				_, exists := updatedPod.Labels["cleanup-test"]
				return !exists // Label should be removed
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testPod)).To(Succeed())
		})
	})
})
