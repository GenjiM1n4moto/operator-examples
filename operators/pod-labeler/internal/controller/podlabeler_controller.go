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
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	labelsv1 "github.com/rayhe/operator-example/operators/pod-labeler/api/v1"
)

const (
	podLabelerFinalizer = "labels.example.com/finalizer"
)

// PodLabelerReconciler reconciles a PodLabeler object
type PodLabelerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=labels.example.com,resources=podlabelers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=labels.example.com,resources=podlabelers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=labels.example.com,resources=podlabelers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodLabelerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the PodLabeler instance
	labeler := &labelsv1.PodLabeler{}
	err := r.Get(ctx, req.NamespacedName, labeler)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, could have been deleted after reconcile request
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PodLabeler")
		return ctrl.Result{}, err
	}

	// Handle deletion with finalizer
	if !labeler.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(labeler, podLabelerFinalizer) {
			// Remove labels from pods before deleting the PodLabeler
			if err := r.cleanupLabels(ctx, labeler); err != nil {
				log.Error(err, "Failed to cleanup labels")
				return ctrl.Result{}, err
			}

			// Remove finalizer
			controllerutil.RemoveFinalizer(labeler, podLabelerFinalizer)
			if err := r.Update(ctx, labeler); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(labeler, podLabelerFinalizer) {
		controllerutil.AddFinalizer(labeler, podLabelerFinalizer)
		if err := r.Update(ctx, labeler); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Validate label rules
	if err := r.validateLabelRules(labeler.Spec.LabelRules); err != nil {
		log.Error(err, "Invalid label rules")
		// Update status with error condition
		now := metav1.NewTime(time.Now())
		labeler.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "ValidationFailed",
				Message:            fmt.Sprintf("Label rules validation failed: %v", err),
				LastTransitionTime: now,
			},
		}
		r.Status().Update(ctx, labeler)
		return ctrl.Result{}, err
	}

	// Convert label selector
	selector, err := metav1.LabelSelectorAsSelector(&labeler.Spec.Selector)
	if err != nil {
		log.Error(err, "Failed to convert label selector")
		return ctrl.Result{}, err
	}

	// List matching pods in the same namespace
	podList := &corev1.PodList{}
	err = r.List(ctx, podList, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     labeler.Namespace,
	})
	if err != nil {
		log.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	log.Info("Found matching pods", "count", len(podList.Items))

	// Apply labels to each pod
	labeledCount := 0
	for i := range podList.Items {
		pod := &podList.Items[i]

		// Initialize labels map if nil
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}

		modified := false
		for _, labelRule := range labeler.Spec.LabelRules {
			value := labelRule.Value

			// Handle ValueFrom if specified
			if labelRule.ValueFrom != "" {
				extractedValue, err := r.extractValueFrom(ctx, pod, labelRule.ValueFrom)
				if err != nil {
					log.Error(err, "Failed to extract value from", "valueFrom", labelRule.ValueFrom, "pod", pod.Name)
					continue
				}
				value = extractedValue
			}

			// Only update if value changed
			if pod.Labels[labelRule.Key] != value {
				pod.Labels[labelRule.Key] = value
				modified = true
			}
		}

		// Update pod if modified
		if modified {
			if err := r.Update(ctx, pod); err != nil {
				log.Error(err, "Failed to update pod labels", "pod", pod.Name)
				// Continue to next pod instead of failing completely
				continue
			}
			labeledCount++
			log.Info("Updated pod labels", "pod", pod.Name, "namespace", pod.Namespace)
		}
	}

	// Update status
	now := metav1.NewTime(time.Now())
	labeler.Status.LabeledPodsCount = labeledCount
	labeler.Status.LastSyncTime = &now
	labeler.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "LabelsApplied",
			Message:            fmt.Sprintf("Successfully labeled %d pods", labeledCount),
			LastTransitionTime: now,
		},
	}

	if err := r.Status().Update(ctx, labeler); err != nil {
		log.Error(err, "Failed to update PodLabeler status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation complete", "labeledPods", labeledCount)
	return ctrl.Result{}, nil
}

// cleanupLabels removes labels from pods when PodLabeler is deleted
func (r *PodLabelerReconciler) cleanupLabels(ctx context.Context, labeler *labelsv1.PodLabeler) error {
	log := ctrl.LoggerFrom(ctx)

	selector, err := metav1.LabelSelectorAsSelector(&labeler.Spec.Selector)
	if err != nil {
		return err
	}

	podList := &corev1.PodList{}
	err = r.List(ctx, podList, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     labeler.Namespace,
	})
	if err != nil {
		return err
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		modified := false

		for _, labelRule := range labeler.Spec.LabelRules {
			if _, exists := pod.Labels[labelRule.Key]; exists {
				delete(pod.Labels, labelRule.Key)
				modified = true
			}
		}

		if modified {
			if err := r.Update(ctx, pod); err != nil {
				log.Error(err, "Failed to remove labels from pod", "pod", pod.Name)
				continue
			}
			log.Info("Removed labels from pod", "pod", pod.Name)
		}
	}

	return nil
}

// validateLabelRules validates that each label rule has exactly one of value or valueFrom
func (r *PodLabelerReconciler) validateLabelRules(rules []labelsv1.LabelRule) error {
	for i, rule := range rules {
		// Check if key is specified
		if rule.Key == "" {
			return fmt.Errorf("label rule at index %d: key is required", i)
		}

		// Check mutual exclusivity of value and valueFrom
		hasValue := rule.Value != ""
		hasValueFrom := rule.ValueFrom != ""

		if !hasValue && !hasValueFrom {
			return fmt.Errorf("label rule '%s' at index %d: exactly one of 'value' or 'valueFrom' must be specified, but neither was provided", rule.Key, i)
		}

		if hasValue && hasValueFrom {
			return fmt.Errorf("label rule '%s' at index %d: exactly one of 'value' or 'valueFrom' must be specified, but both were provided (value=%q, valueFrom=%q)", rule.Key, i, rule.Value, rule.ValueFrom)
		}

		// Validate valueFrom format if present
		if hasValueFrom {
			parts := strings.Split(rule.ValueFrom, ".")
			if len(parts) != 3 {
				return fmt.Errorf("label rule '%s' at index %d: invalid valueFrom format %q (expected 'namespace.labels.<key>' or 'pod.labels.<key>')", rule.Key, i, rule.ValueFrom)
			}
			if parts[1] != "labels" {
				return fmt.Errorf("label rule '%s' at index %d: invalid valueFrom field %q (only 'labels' is supported)", rule.Key, i, parts[1])
			}
			if parts[0] != "namespace" && parts[0] != "pod" {
				return fmt.Errorf("label rule '%s' at index %d: invalid valueFrom source %q (only 'namespace' and 'pod' are supported)", rule.Key, i, parts[0])
			}
		}
	}
	return nil
}

// extractValueFrom extracts label value from namespace or pod labels
func (r *PodLabelerReconciler) extractValueFrom(ctx context.Context, pod *corev1.Pod, valueFrom string) (string, error) {
	// Parse valueFrom format: "namespace.labels.key" or "pod.labels.key"
	parts := strings.Split(valueFrom, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid valueFrom format: %s (expected format: 'namespace.labels.key' or 'pod.labels.key')", valueFrom)
	}

	source := parts[0] // "namespace" or "pod"
	field := parts[1]  // "labels"
	key := parts[2]    // label key

	if field != "labels" {
		return "", fmt.Errorf("unsupported field: %s (only 'labels' is supported)", field)
	}

	switch source {
	case "pod":
		if value, exists := pod.Labels[key]; exists {
			return value, nil
		}
		return "", fmt.Errorf("label %s not found in pod labels", key)

	case "namespace":
		namespace := &corev1.Namespace{}
		if err := r.Get(ctx, client.ObjectKey{Name: pod.Namespace}, namespace); err != nil {
			return "", fmt.Errorf("failed to get namespace: %w", err)
		}
		if value, exists := namespace.Labels[key]; exists {
			return value, nil
		}
		return "", fmt.Errorf("label %s not found in namespace labels", key)

	default:
		return "", fmt.Errorf("unsupported source: %s (only 'namespace' and 'pod' are supported)", source)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodLabelerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&labelsv1.PodLabeler{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.findPodLabelersForPod)).
		Named("podlabeler").
		Complete(r)
}

// findPodLabelersForPod finds all PodLabelers that match a given pod
func (r *PodLabelerReconciler) findPodLabelersForPod(ctx context.Context, pod client.Object) []reconcile.Request {
	podLabelerList := &labelsv1.PodLabelerList{}
	err := r.List(ctx, podLabelerList, &client.ListOptions{
		Namespace: pod.GetNamespace(),
	})
	if err != nil {
		return []reconcile.Request{}
	}

	var requests []reconcile.Request
	for _, labeler := range podLabelerList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&labeler.Spec.Selector)
		if err != nil {
			continue
		}

		// Check if the pod matches this PodLabeler's selector
		// Convert pod labels to labels.Set for matching
		if selector.Matches(labels.Set(pod.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&labeler),
			})
		}
	}

	return requests
}
