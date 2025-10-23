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
	"path"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/example/backup-operator/api/v1alpha1"
	"github.com/example/backup-operator/internal/storage"
)

const managedSecretNamespaceLabel = "backup.backup.example.com/namespace"

// ExternalStrategy implements backup using external storage (S3, NFS, etc.)
type ExternalStrategy struct {
	client  client.Client
	backend storage.Backend
}

// NewExternalStrategy creates a new external storage backup strategy
func NewExternalStrategy(c client.Client, backend storage.Backend) Strategy {
	return &ExternalStrategy{client: c, backend: backend}
}

// Backup creates a backup Job that uploads PVC data to external storage
func (e *ExternalStrategy) Backup(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) (*BackupResult, error) {
	logger := log.FromContext(ctx)

	if err := e.ensureCredentialsSecret(ctx, pvc.Namespace, policy); err != nil {
		return nil, err
	}

	repoURL, err := e.repositoryURL(policy, pvc)
	if err != nil {
		return nil, err
	}

	backupName := fmt.Sprintf("%s-%s-%s", policy.Name, pvc.Name, time.Now().Format("20060102-150405"))
	logger.Info("Creating backup Job for external storage", "job", backupName, "pvc", pvc.Name, "namespace", pvc.Namespace, "repo", repoURL)

	job := e.buildBackupJob(backupName, pvc, policy, repoURL)
	if err := e.client.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create backup Job %s/%s: %w", pvc.Namespace, backupName, err)
	}

	sizeQty := pvc.Status.Capacity[corev1.ResourceStorage]
	result := &BackupResult{
		Name:      backupName,
		Location:  repoURL,
		Timestamp: time.Now(),
		SizeBytes: sizeQty.Value(),
		Size:      humanReadableQuantity(sizeQty),
		Metadata: map[string]string{
			"pvc":         pvc.Name,
			"namespace":   pvc.Namespace,
			"strategy":    "external",
			"repository":  repoURL,
			"destination": policy.Spec.Destination.Type,
		},
	}

	return result, nil
}

// buildBackupJob creates a Kubernetes Job for backing up PVC to external storage
func (e *ExternalStrategy) buildBackupJob(backupName string, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy, repoURL string) *batchv1.Job {
	backoffLimit := int32(3)
	// 完成后60秒自动清理（从10分钟改为1分钟）
	ttlSecondsAfterFinished := int32(60)
	// Job最多运行30分钟，避免频繁重试导致额外负载
	activeDeadlineSeconds := int64(1800)

	labels := map[string]string{
		LabelPolicy:          policy.Name,
		LabelPVC:             pvc.Name,
		LabelStrategy:        "external",
		LabelPolicyNamespace: policy.Namespace,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: pvc.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"backup.backup.example.com/job": backupName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "backup",
							Image:   "restic/restic:latest",
							Command: []string{"/bin/sh", "-c", e.buildBackupCommand(backupName, pvc, policy, repoURL)},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
									ReadOnly:  true,
								},
							},
							Env: e.buildBackupEnv(policy, repoURL),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	if policy.Namespace == pvc.Namespace {
		job.OwnerReferences = []metav1.OwnerReference{*ownerReferenceFor(policy, pvc.Namespace)}
	}

	return job
}

// buildBackupCommand generates the backup command executed inside the Job pod
func (e *ExternalStrategy) buildBackupCommand(backupName string, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy, repoURL string) string {
	return fmt.Sprintf(`set -euo pipefail
export RESTIC_REPOSITORY="%s"
export RESTIC_TAG_POLICY="policy:%s"
export RESTIC_TAG_PVC="pvc:%s"
export RESTIC_TAG_NAMESPACE="namespace:%s"

echo "Starting backup %s" >&2
restic -r "$RESTIC_REPOSITORY" init >/dev/null 2>&1 || true
restic -r "$RESTIC_REPOSITORY" backup /data --tag "$RESTIC_TAG_POLICY" --tag "$RESTIC_TAG_PVC" --tag "$RESTIC_TAG_NAMESPACE" --hostname "%s"

if [ -n "${RETENTION_MAX_BACKUPS:-}" ] || [ -n "${RETENTION_MAX_AGE:-}" ]; then
  ARGS=""
  if [ -n "${RETENTION_MAX_BACKUPS:-}" ]; then
    ARGS="$ARGS --keep-last ${RETENTION_MAX_BACKUPS}"
  fi
  if [ -n "${RETENTION_MAX_AGE:-}" ]; then
    ARGS="$ARGS --keep-within ${RETENTION_MAX_AGE}"
  fi
  restic -r "$RESTIC_REPOSITORY" forget $ARGS --prune
fi
`, repoURL, policy.Name, pvc.Name, pvc.Namespace, backupName, pvc.Namespace)
}

// buildBackupEnv creates environment variables for the backup Job
func (e *ExternalStrategy) buildBackupEnv(policy *backupv1alpha1.BackupPolicy, repoURL string) []corev1.EnvVar {
	dest := policy.Spec.Destination
	env := []corev1.EnvVar{
		{Name: "RESTIC_REPOSITORY", Value: repoURL},
		{Name: "AWS_S3_FORCE_PATH_STYLE", Value: "true"},
	}

	if policy.Spec.Retention.MaxBackups > 0 {
		env = append(env, corev1.EnvVar{Name: "RETENTION_MAX_BACKUPS", Value: strconv.Itoa(policy.Spec.Retention.MaxBackups)})
	}
	if policy.Spec.Retention.MaxAge != "" {
		env = append(env, corev1.EnvVar{Name: "RETENTION_MAX_AGE", Value: policy.Spec.Retention.MaxAge})
	}

	if dest.CredentialsSecret != "" {
		optional := true
		env = append(env,
			corev1.EnvVar{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: dest.CredentialsSecret},
						Key:                  "access-key",
					},
				},
			},
			corev1.EnvVar{
				Name: "AWS_SECRET_ACCESS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: dest.CredentialsSecret},
						Key:                  "secret-key",
					},
				},
			},
			corev1.EnvVar{
				Name: "RESTIC_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: dest.CredentialsSecret},
						Key:                  "restic-password",
					},
				},
			},
			corev1.EnvVar{
				Name: "AWS_DEFAULT_REGION",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: dest.CredentialsSecret},
						Key:                  "region",
						Optional:             &optional,
					},
				},
			},
		)
	}

	if dest.Endpoint != "" {
		endpoint := strings.TrimSuffix(dest.Endpoint, "/")
		env = append(env,
			corev1.EnvVar{Name: "AWS_ENDPOINT_URL", Value: endpoint},
			corev1.EnvVar{Name: "AWS_S3_ENDPOINT", Value: endpoint},
		)
	}

	return env
}

// ListBackups is currently a no-op because retention is handled by restic inside the Job
func (e *ExternalStrategy) ListBackups(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) ([]backupv1alpha1.StoredBackup, error) {
	return []backupv1alpha1.StoredBackup{}, nil
}

// DeleteBackup deletes metadata or artifacts recorded for a backup
func (e *ExternalStrategy) DeleteBackup(ctx context.Context, backup *backupv1alpha1.StoredBackup, policy *backupv1alpha1.BackupPolicy) error {
	if e.backend == nil || backup.Location == "" {
		return nil
	}
	if err := e.backend.Delete(ctx, backup.Location); err != nil {
		return fmt.Errorf("failed to delete backup object %s: %w", backup.Location, err)
	}
	return nil
}

// Cleanup relies on the Job's restic forget/prune logic
func (e *ExternalStrategy) Cleanup(ctx context.Context, pvc *corev1.PersistentVolumeClaim, policy *backupv1alpha1.BackupPolicy) error {
	log.FromContext(ctx).Info("Cleanup for external strategy is handled inside backup Jobs", "pvc", pvc.Name)
	return nil
}

// Restore restores a backup from external storage (not yet implemented)
func (e *ExternalStrategy) Restore(ctx context.Context, backup *backupv1alpha1.StoredBackup, targetPVC *corev1.PersistentVolumeClaim) error {
	return fmt.Errorf("external restore not yet implemented")
}

func (e *ExternalStrategy) repositoryURL(policy *backupv1alpha1.BackupPolicy, pvc *corev1.PersistentVolumeClaim) (string, error) {
	dest := policy.Spec.Destination
	switch dest.Type {
	case "s3":
		bucket, prefix := splitS3URL(dest.URL)
		repoPath := path.Join(prefix, policy.Name, pvc.Namespace, pvc.Name)
		endpoint := strings.TrimSuffix(dest.Endpoint, "/")
		if endpoint != "" {
			return fmt.Sprintf("s3:%s/%s/%s", endpoint, bucket, repoPath), nil
		}
		return fmt.Sprintf("s3:%s/%s", bucket, repoPath), nil
	default:
		return "", fmt.Errorf("unsupported external destination type: %s", dest.Type)
	}
}

func splitS3URL(raw string) (bucket string, prefix string) {
	clean := strings.TrimPrefix(raw, "s3://")
	parts := strings.SplitN(clean, "/", 2)
	bucket = parts[0]
	if len(parts) > 1 {
		prefix = parts[1]
	}
	return bucket, prefix
}

func (e *ExternalStrategy) ensureCredentialsSecret(ctx context.Context, targetNamespace string, policy *backupv1alpha1.BackupPolicy) error {
	dest := policy.Spec.Destination
	if dest.CredentialsSecret == "" {
		return nil
	}
	if targetNamespace == policy.Namespace {
		return nil
	}

	namespacedName := types.NamespacedName{Name: dest.CredentialsSecret, Namespace: targetNamespace}
	existing := &corev1.Secret{}
	if err := e.client.Get(ctx, namespacedName, existing); err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret %s/%s: %w", targetNamespace, dest.CredentialsSecret, err)
	}

	source := &corev1.Secret{}
	if err := e.client.Get(ctx, types.NamespacedName{Name: dest.CredentialsSecret, Namespace: policy.Namespace}, source); err != nil {
		return fmt.Errorf("failed to read credentials secret %s/%s: %w", policy.Namespace, dest.CredentialsSecret, err)
	}

	copy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dest.CredentialsSecret,
			Namespace: targetNamespace,
			Labels: map[string]string{
				LabelManaged:                "true",
				LabelPolicy:                 policy.Name,
				managedSecretNamespaceLabel: policy.Namespace,
			},
		},
		Data: source.Data,
		Type: source.Type,
	}

	if err := e.client.Create(ctx, copy); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to copy credentials secret to namespace %s: %w", targetNamespace, err)
	}
	return nil
}
