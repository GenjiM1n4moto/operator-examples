/*
Copyright 2025 hepj1999@gmail.com.

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

package backup

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/example/backup-operator/api/v1alpha1"
)

var (
	volumeSnapshotGVK = schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshot",
	}
	volumeSnapshotListGVK = schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshotList",
	}
)

// SnapshotStrategy implements backup using Kubernetes VolumeSnapshots
type SnapshotStrategy struct {
	client client.Client
}

// NewSnapshotStrategy creates a new snapshot-based backup strategy
func NewSnapshotStrategy(c client.Client) Strategy {
	return &SnapshotStrategy{client: c}
}

// Backup creates a VolumeSnapshot for the given PVC
func (s *SnapshotStrategy) Backup(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) (*BackupResult, error) {
	logger := log.FromContext(ctx)

	snapshotName := fmt.Sprintf("%s-%s-%s", policy.Name, pvc.Name, time.Now().Format("20060102-150405"))
	logger.Info("Creating VolumeSnapshot", "snapshot", snapshotName, "pvc", pvc.Name, "namespace", pvc.Namespace)

	ownerRef := ownerReferenceFor(policy, pvc.Namespace)
	snapshot := newVolumeSnapshot(snapshotName, pvc, policy, ownerRef)

	if err := s.client.Create(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("failed to create VolumeSnapshot %s/%s: %w", pvc.Namespace, snapshotName, err)
	}

	sizeQty := pvc.Status.Capacity[corev1.ResourceStorage]
	result := &BackupResult{
		Name:      snapshotName,
		Location:  fmt.Sprintf("%s/%s", pvc.Namespace, snapshotName),
		Timestamp: time.Now(),
		SizeBytes: sizeQty.Value(),
		Size:      humanReadableQuantity(sizeQty),
		Metadata: map[string]string{
			"namespace": pvc.Namespace,
			"pvc":       pvc.Name,
			"strategy":  "snapshot",
		},
	}

	return result, nil
}

// ListBackups lists all VolumeSnapshots for the given PVC
func (s *SnapshotStrategy) ListBackups(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) ([]backupv1alpha1.StoredBackup, error) {
	snapshotList := &unstructured.UnstructuredList{}
	snapshotList.SetGroupVersionKind(volumeSnapshotListGVK)

	selector := client.MatchingLabels{
		LabelPolicy:   policy.Name,
		LabelPVC:      pvc.Name,
		LabelStrategy: "snapshot",
	}

	if err := s.client.List(ctx, snapshotList, client.InNamespace(pvc.Namespace), selector); err != nil {
		return nil, fmt.Errorf("failed to list VolumeSnapshots for pvc %s/%s: %w", pvc.Namespace, pvc.Name, err)
	}

	var backups []backupv1alpha1.StoredBackup
	for _, item := range snapshotList.Items {
		backups = append(backups, snapshotFromUnstructured(item, pvc))
	}

	sort.Slice(backups, func(i, j int) bool {
		ti := backupTimeOrZero(backups[i].Timestamp)
		tj := backupTimeOrZero(backups[j].Timestamp)
		return ti.After(tj)
	})

	return backups, nil
}

// DeleteBackup deletes a specific VolumeSnapshot
func (s *SnapshotStrategy) DeleteBackup(ctx context.Context, backup *backupv1alpha1.StoredBackup, policy *backupv1alpha1.BackupPolicy) error {
	snapshot := &unstructured.Unstructured{}
	snapshot.SetGroupVersionKind(volumeSnapshotGVK)
	snapshot.SetNamespace(backup.Namespace)
	snapshot.SetName(backup.Name)

	if err := s.client.Delete(ctx, snapshot); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete VolumeSnapshot %s/%s: %w", backup.Namespace, backup.Name, err)
	}
	return nil
}

// Cleanup removes old snapshots according to retention policy
func (s *SnapshotStrategy) Cleanup(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) error {
	logger := log.FromContext(ctx)

	backups, err := s.ListBackups(ctx, pvc, policy)
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		return nil
	}

	retention := policy.Spec.Retention
	now := time.Now()
	var toDelete []backupv1alpha1.StoredBackup

	for i, backupInfo := range backups {
		exceedMaxBackups := retention.MaxBackups > 0 && i >= retention.MaxBackups
		exceedMaxAge := false

		if retention.MaxAge != "" && backupInfo.Timestamp != nil {
			if maxAge, parseErr := time.ParseDuration(retention.MaxAge); parseErr == nil {
				if now.Sub(backupInfo.Timestamp.Time) > maxAge {
					exceedMaxAge = true
				}
			} else {
				logger.Error(parseErr, "Failed to parse retention maxAge", "value", retention.MaxAge)
			}
		}

		if exceedMaxBackups || exceedMaxAge {
			toDelete = append(toDelete, backupInfo)
		}
	}

	for _, backupInfo := range toDelete {
		if err := s.DeleteBackup(ctx, &backupInfo, policy); err != nil {
			logger.Error(err, "Failed to delete expired snapshot", "snapshot", backupInfo.Name, "namespace", backupInfo.Namespace)
		}
	}

	if len(toDelete) > 0 {
		logger.Info("Snapshot cleanup completed", "pvc", pvc.Name, "deleted", len(toDelete))
	}
	return nil
}

// Restore restores a VolumeSnapshot to a new PVC (not yet implemented)
func (s *SnapshotStrategy) Restore(ctx context.Context, backup *backupv1alpha1.StoredBackup, targetPVC *corev1.PersistentVolumeClaim) error {
	return fmt.Errorf("snapshot restore not yet implemented")
}

func newVolumeSnapshot(name string, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy, owner *metav1.OwnerReference) *unstructured.Unstructured {
	snapshot := &unstructured.Unstructured{}
	snapshot.SetGroupVersionKind(volumeSnapshotGVK)
	snapshot.SetNamespace(pvc.Namespace)
	snapshot.SetName(name)

	labels := map[string]string{
		LabelPolicy:   policy.Name,
		LabelPVC:      pvc.Name,
		LabelStrategy: "snapshot",
	}
	snapshot.SetLabels(labels)

	if owner != nil {
		snapshot.SetOwnerReferences([]metav1.OwnerReference{*owner})
	}

	spec := map[string]any{
		"source": map[string]any{
			"persistentVolumeClaimName": pvc.Name,
		},
	}
	_ = unstructured.SetNestedField(snapshot.Object, spec, "spec")

	return snapshot
}

func ownerReferenceFor(policy *backupv1alpha1.BackupPolicy, dependentNamespace string) *metav1.OwnerReference {
	if policy.Namespace != dependentNamespace {
		return nil
	}
	controller := true
	blockDelete := true
	return &metav1.OwnerReference{
		APIVersion:         policy.APIVersion,
		Kind:               policy.Kind,
		Name:               policy.Name,
		UID:                policy.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockDelete,
	}
}

func snapshotFromUnstructured(obj unstructured.Unstructured, pvc *corev1.PersistentVolumeClaim) backupv1alpha1.StoredBackup {
	stored := backupv1alpha1.StoredBackup{
		Name:      obj.GetName(),
		PVCName:   pvc.Name,
		Namespace: obj.GetNamespace(),
		Strategy:  "snapshot",
		Status:    "Pending",
		Location:  fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()),
	}

	if ts := obj.GetCreationTimestamp(); !ts.IsZero() {
		stored.Timestamp = &metav1.Time{Time: ts.Time}
	}

	if ready, found, _ := unstructured.NestedBool(obj.Object, "status", "readyToUse"); found {
		if ready {
			stored.Status = "Completed"
		} else {
			stored.Status = "InProgress"
		}
	}

	if quantityStr, found, _ := unstructured.NestedString(obj.Object, "status", "restoreSize"); found {
		if qty, err := resource.ParseQuantity(quantityStr); err == nil {
			stored.Size = qty.String()
		}
	}

	if creationTime, found, _ := unstructured.NestedMap(obj.Object, "status", "creationTime"); found {
		var seconds int64
		var nanos int64

		if value, ok := creationTime["seconds"]; ok {
			switch typed := value.(type) {
			case int64:
				seconds = typed
			case int32:
				seconds = int64(typed)
			case float64:
				seconds = int64(typed)
			case string:
				if parsed, err := time.Parse(time.RFC3339, typed); err == nil {
					stored.Timestamp = &metav1.Time{Time: parsed}
				}
			}
		}

		if value, ok := creationTime["nanos"]; ok {
			switch typed := value.(type) {
			case int64:
				nanos = typed
			case int32:
				nanos = int64(typed)
			case float64:
				nanos = int64(typed)
			}
		}

		if seconds != 0 || nanos != 0 {
			stored.Timestamp = &metav1.Time{Time: time.Unix(seconds, nanos)}
		}
	}

	return stored
}

func backupTimeOrZero(ts *metav1.Time) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.Time
}

func humanReadableQuantity(qty resource.Quantity) string {
	if qty.Sign() <= 0 {
		return "0"
	}
	return qty.String()
}
