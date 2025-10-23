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
	"time"

	backupv1alpha1 "github.com/example/backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BackupResult contains the result of a backup operation
type BackupResult struct {
	// Name of the backup (snapshot name, file name, etc.)
	Name string
	// Full location/path of the backup
	Location string
	// Size of the backup in bytes
	SizeBytes int64
	// Human-readable size (e.g., "1.5Gi")
	Size string
	// Backup timestamp
	Timestamp time.Time
	// Additional metadata
	Metadata map[string]string
}

// Strategy defines the interface for different backup strategies
type Strategy interface {
	// Backup performs a backup of the given PVC
	Backup(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) (*BackupResult, error)

	// ListBackups lists all backups for the given PVC
	ListBackups(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) ([]backupv1alpha1.StoredBackup, error)

	// DeleteBackup deletes a specific backup
	DeleteBackup(ctx context.Context, backup *backupv1alpha1.StoredBackup, policy *backupv1alpha1.BackupPolicy) error

	// Cleanup removes old backups according to retention policy
	Cleanup(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) error

	// Restore restores a backup to a PVC (optional, for future implementation)
	Restore(ctx context.Context, backup *backupv1alpha1.StoredBackup, targetPVC *corev1.PersistentVolumeClaim) error
}
