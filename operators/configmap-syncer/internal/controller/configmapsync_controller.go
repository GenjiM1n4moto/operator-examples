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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	syncv1 "github.com/rayhe/configmap-syncer/api/v1"
)

// ConfigMapSyncReconciler reconciles a ConfigMapSync object
type ConfigMapSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sync.example.com,resources=configmapsyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sync.example.com,resources=configmapsyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sync.example.com,resources=configmapsyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigMapSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the ConfigMapSync instance
	configMapSync := &syncv1.ConfigMapSync{}
	err := r.Get(ctx, req.NamespacedName, configMapSync)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("ConfigMapSync resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ConfigMapSync")
		return ctrl.Result{}, err
	}

	// Get the source ConfigMap
	sourceConfigMap := &corev1.ConfigMap{}
	sourceKey := types.NamespacedName{
		Name:      configMapSync.Spec.SourceConfigMap.Name,
		Namespace: configMapSync.Spec.SourceConfigMap.Namespace,
	}

	err = r.Get(ctx, sourceKey, sourceConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Source ConfigMap not found", "configmap", sourceKey)
			r.updateStatus(ctx, configMapSync, []string{}, "SourceNotFound", fmt.Sprintf("Source ConfigMap %s not found", sourceKey))
			return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
		}
		return ctrl.Result{}, err
	}

	// Get target namespaces
	targetNamespaces, err := r.getTargetNamespaces(ctx, configMapSync)
	if err != nil {
		log.Error(err, "Failed to get target namespaces")
		return ctrl.Result{}, err
	}

	// Sync ConfigMap to target namespaces
	syncedNamespaces := []string{}
	for _, ns := range targetNamespaces {
		if ns == sourceConfigMap.Namespace {
			continue // Skip source namespace
		}

		err := r.syncConfigMapToNamespace(ctx, sourceConfigMap, ns)
		if err != nil {
			log.Error(err, "Failed to sync ConfigMap to namespace", "namespace", ns)
			continue
		}
		syncedNamespaces = append(syncedNamespaces, ns)
	}

	// Update status
	err = r.updateStatus(ctx, configMapSync, syncedNamespaces, "Synced", fmt.Sprintf("Successfully synced to %d namespaces", len(syncedNamespaces)))
	if err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled ConfigMapSync", "synced_namespaces", len(syncedNamespaces))
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

// getTargetNamespaces returns the list of target namespaces to sync to
func (r *ConfigMapSyncReconciler) getTargetNamespaces(ctx context.Context, configMapSync *syncv1.ConfigMapSync) ([]string, error) {
	// Validate that user specified exactly one way to define target namespaces
	hasTargetNamespaces := len(configMapSync.Spec.TargetNamespaces) > 0
	hasSelector := configMapSync.Spec.Selector != nil

	if !hasTargetNamespaces && !hasSelector {
		return nil, fmt.Errorf("must specify either targetNamespaces or selector")
	}

	if hasTargetNamespaces && hasSelector {
		return nil, fmt.Errorf("cannot specify both targetNamespaces and selector - choose one")
	}

	// Use explicitly specified namespaces
	if hasTargetNamespaces {
		return configMapSync.Spec.TargetNamespaces, nil
	}

	// Use selector to find namespaces
	if hasSelector {
		namespaceList := &corev1.NamespaceList{}
		selector, err := metav1.LabelSelectorAsSelector(configMapSync.Spec.Selector)
		if err != nil {
			return nil, err
		}

		err = r.List(ctx, namespaceList, &client.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return nil, err
		}

		var targetNamespaces []string
		for _, ns := range namespaceList.Items {
			targetNamespaces = append(targetNamespaces, ns.Name)
		}

		if len(targetNamespaces) == 0 {
			return nil, fmt.Errorf("selector matched no namespaces")
		}

		return targetNamespaces, nil
	}

	return nil, fmt.Errorf("unexpected error in target namespace resolution")
}

// syncConfigMapToNamespace syncs the source ConfigMap to the target namespace
func (r *ConfigMapSyncReconciler) syncConfigMapToNamespace(ctx context.Context, sourceConfigMap *corev1.ConfigMap, targetNamespace string) error {
	// Create a copy of the source ConfigMap for the target namespace
	targetConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceConfigMap.Name,
			Namespace: targetNamespace,
			Labels:    sourceConfigMap.Labels,
		},
		Data:       sourceConfigMap.Data,
		BinaryData: sourceConfigMap.BinaryData,
	}

	// Try to get existing ConfigMap
	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: targetConfigMap.Name, Namespace: targetNamespace}, existingConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new ConfigMap
			return r.Create(ctx, targetConfigMap)
		}
		return err
	}

	// Update existing ConfigMap
	existingConfigMap.Data = sourceConfigMap.Data
	existingConfigMap.BinaryData = sourceConfigMap.BinaryData
	existingConfigMap.Labels = sourceConfigMap.Labels

	return r.Update(ctx, existingConfigMap)
}

// updateStatus updates the status of the ConfigMapSync resource
func (r *ConfigMapSyncReconciler) updateStatus(ctx context.Context, configMapSync *syncv1.ConfigMapSync, syncedNamespaces []string, conditionType, message string) error {
	now := metav1.Now()
	configMapSync.Status.SyncedNamespaces = syncedNamespaces
	configMapSync.Status.LastSyncTime = &now

	// Update conditions
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             conditionType,
		Message:            message,
	}

	if conditionType == "SourceNotFound" {
		condition.Status = metav1.ConditionFalse
	}

	// Remove old conditions of the same type and add the new one
	var newConditions []metav1.Condition
	for _, cond := range configMapSync.Status.Conditions {
		if cond.Type != conditionType {
			newConditions = append(newConditions, cond)
		}
	}
	newConditions = append(newConditions, condition)
	configMapSync.Status.Conditions = newConditions

	return r.Status().Update(ctx, configMapSync)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1.ConfigMapSync{}).
		Owns(&corev1.ConfigMap{}).
		Named("configmapsync").
		Complete(r)
}
