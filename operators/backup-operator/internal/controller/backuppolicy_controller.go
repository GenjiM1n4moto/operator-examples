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

package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/example/backup-operator/api/v1alpha1"
	"github.com/example/backup-operator/internal/backup"
	"github.com/example/backup-operator/internal/storage"
)

const (
	// Default strategy if not specified
	defaultStrategy = "snapshot"

	// Requeue intervals
	requeueAfterError     = 1 * time.Minute
	requeueAfterSuccess   = 5 * time.Minute
	requeueWhileJobActive = 1 * time.Minute
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.backup.example.com,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.backup.example.com,resources=backuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.backup.example.com,resources=backuppolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *BackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the BackupPolicy
	policy := &backupv1alpha1.BackupPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("BackupPolicy not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get BackupPolicy")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling BackupPolicy",
		"name", policy.Name,
		"namespace", policy.Namespace,
		"strategy", policy.Spec.Strategy)

	// Check and update status of running backup Jobs
	if err := r.handleJobCompletion(ctx, policy); err != nil {
		logger.Error(err, "Failed to handle Job completion")
		// Continue with reconciliation even if this fails
	}

	// Clean up old completed/failed Jobs to reduce resource usage
	if err := r.cleanupOldJobs(ctx, policy); err != nil {
		logger.Error(err, "Failed to cleanup old Jobs")
		// Continue with reconciliation even if this fails
	}

	// Initialize strategy if not set
	strategy := policy.Spec.Strategy
	if strategy == "" {
		strategy = defaultStrategy
	}

	// Get backup strategy implementation
	backupStrategy, err := r.getBackupStrategy(ctx, strategy, policy)
	if err != nil {
		logger.Error(err, "Failed to get backup strategy", "strategy", strategy)
		r.updateStatus(ctx, policy, "Error", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	// Find target PVCs
	pvcs, err := r.findTargetPVCs(ctx, policy)
	if err != nil {
		logger.Error(err, "Failed to find target PVCs")
		r.updateStatus(ctx, policy, "Error", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	if len(pvcs) == 0 {
		logger.Info("No PVCs found matching selector")
		r.updateStatus(ctx, policy, "Active", "No PVCs found")
		return ctrl.Result{RequeueAfter: requeueAfterSuccess}, nil
	}

	logger.Info("Found target PVCs", "count", len(pvcs))

	// Check if it's time to backup (based on schedule)
	shouldBackup, nextRun := r.shouldBackupNow(policy)
	if !shouldBackup {
		logger.Info("Not time to backup yet", "nextRun", nextRun)
		r.updateStatusWithNextRun(ctx, policy, "Active", nextRun)
		return ctrl.Result{RequeueAfter: time.Until(nextRun)}, nil
	}

	activeJobs, err := r.hasActiveBackupJobs(ctx, policy, pvcs)
	if err != nil {
		logger.Error(err, "Failed to check for active backup Jobs")
		r.updateStatus(ctx, policy, "Error", "Failed to check running Jobs")
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}
	if activeJobs {
		logger.Info("Previous backup Jobs are still running, skipping new run")
		r.updateStatusWithNextRun(ctx, policy, "Active", nextRun)
		return ctrl.Result{RequeueAfter: requeueWhileJobActive}, nil
	}

	// Track previous backups so we can merge with the new run
	existingBackups := make(map[string]backupv1alpha1.StoredBackup)
	for _, stored := range policy.Status.StoredBackups {
		existingBackups[backupKey(stored.Namespace, stored.Name)] = stored
	}

	// Perform backup for each PVC
	backupErrors := 0

	for _, pvc := range pvcs {
		logger.Info("Backing up PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)

		result, err := backupStrategy.Backup(ctx, &pvc, policy)
		if err != nil {
			logger.Error(err, "Failed to backup PVC", "pvc", pvc.Name)
			backupErrors++
			continue
		}

		// Add to stored backups with Running status
		// Status will be updated when Job completes via Job watch event
		storedBackup := backupv1alpha1.StoredBackup{
			Name:      result.Name,
			Timestamp: &metav1.Time{Time: result.Timestamp},
			PVCName:   pvc.Name,
			Namespace: pvc.Namespace,
			Size:      result.Size,
			Location:  result.Location,
			Status:    "Running",
			Strategy:  strategy,
		}

		existingBackups[backupKey(storedBackup.Namespace, storedBackup.Name)] = storedBackup

		// Run cleanup to remove old backups
		if err := backupStrategy.Cleanup(ctx, &pvc, policy); err != nil {
			logger.Error(err, "Failed to cleanup old backups", "pvc", pvc.Name)
		}
	}

	// Update status with the combined view of old + new backups
	policy.Status.StoredBackups = normalizeStoredBackups(existingBackups)
	policy.Status.BackupCount = len(policy.Status.StoredBackups)

	nextRun, nextRunErr := r.nextRun(policy, time.Now())
	if nextRunErr != nil {
		logger.Error(nextRunErr, "Failed to parse cron schedule, using default 1 hour interval", "schedule", policy.Spec.Schedule)
	}
	policy.Status.NextRunTime = &metav1.Time{Time: nextRun}

	if backupErrors > 0 {
		msg := fmt.Sprintf("Backup completed with %d errors", backupErrors)
		r.updateStatus(ctx, policy, "Error", msg)
		return ctrl.Result{RequeueAfter: requeueAfterError}, fmt.Errorf("backup completed with %d errors", backupErrors)
	}

	r.updateStatus(ctx, policy, "Active", "Backup completed successfully")

	return ctrl.Result{RequeueAfter: time.Until(nextRun)}, nil
}

// getBackupStrategy returns the appropriate backup strategy based on the policy
func (r *BackupPolicyReconciler) getBackupStrategy(ctx context.Context, strategy string, policy *backupv1alpha1.BackupPolicy) (backup.Strategy, error) {
	switch strategy {
	case "snapshot":
		return backup.NewSnapshotStrategy(r.Client), nil

	case "external":
		// Get storage backend configuration
		backend, err := r.getStorageBackend(ctx, policy)
		if err != nil {
			return nil, fmt.Errorf("failed to get storage backend: %w", err)
		}
		return backup.NewExternalStrategy(r.Client, backend), nil

	default:
		return nil, fmt.Errorf("unknown backup strategy: %s", strategy)
	}
}

// getStorageBackend creates a storage backend based on the destination configuration
func (r *BackupPolicyReconciler) getStorageBackend(ctx context.Context, policy *backupv1alpha1.BackupPolicy) (storage.Backend, error) {
	dest := policy.Spec.Destination

	if dest.Type == "" {
		return nil, fmt.Errorf("destination type is required for external strategy")
	}

	config := &storage.Config{
		Type:         dest.Type,
		StorageClass: dest.StorageClass,
	}

	switch dest.Type {
	case "s3":
		bucket, prefix := parseS3URL(dest.URL)
		if bucket == "" {
			return nil, fmt.Errorf("destination URL must include bucket name for S3 backend")
		}
		config.Bucket = bucket
		config.Prefix = prefix
		config.Endpoint = dest.Endpoint

	case "nfs":
		config.Prefix = dest.URL
		config.Endpoint = dest.Endpoint

	default:
		config.Endpoint = dest.Endpoint
	}

	// Load credentials from Secret if specified
	if dest.CredentialsSecret != "" {
		secret := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      dest.CredentialsSecret,
			Namespace: policy.Namespace,
		}, secret)

		if err != nil {
			return nil, fmt.Errorf("failed to get credentials secret: %w", err)
		}

		// Parse credentials based on storage type
		if dest.Type == "s3" {
			config.AccessKey = string(secret.Data["access-key"])
			config.SecretKey = string(secret.Data["secret-key"])
			config.Region = string(secret.Data["region"])

			// Endpoint can also be in Secret (for backward compatibility)
			if config.Endpoint == "" && len(secret.Data["endpoint"]) > 0 {
				config.Endpoint = string(secret.Data["endpoint"])
			}
		}
	}

	return storage.NewBackend(config)
}

// parseS3URL parses s3://bucket/prefix format
func parseS3URL(url string) (bucket, prefix string) {
	// Remove s3:// prefix if present
	url = strings.TrimPrefix(url, "s3://")

	parts := strings.SplitN(url, "/", 2)
	bucket = parts[0]
	if len(parts) > 1 {
		prefix = parts[1]
	}
	return bucket, prefix
}

const maxStatusHistory = 200

func backupKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func normalizeStoredBackups(backups map[string]backupv1alpha1.StoredBackup) []backupv1alpha1.StoredBackup {
	list := make([]backupv1alpha1.StoredBackup, 0, len(backups))
	for _, b := range backups {
		list = append(list, b)
	}

	sort.SliceStable(list, func(i, j int) bool {
		ti := storedBackupTime(list[i])
		tj := storedBackupTime(list[j])
		if ti.Equal(tj) {
			// Fall back to name to keep deterministic order
			if list[i].Namespace == list[j].Namespace {
				return list[i].Name > list[j].Name
			}
			return list[i].Namespace > list[j].Namespace
		}
		return ti.After(tj)
	})

	if len(list) > maxStatusHistory {
		list = list[:maxStatusHistory]
	}

	return list
}

func storedBackupTime(b backupv1alpha1.StoredBackup) time.Time {
	if b.Timestamp == nil {
		return time.Time{}
	}
	return b.Timestamp.Time
}

// findTargetPVCs finds all PVCs matching the policy selector
func (r *BackupPolicyReconciler) findTargetPVCs(ctx context.Context, policy *backupv1alpha1.BackupPolicy) ([]corev1.PersistentVolumeClaim, error) {
	logger := log.FromContext(ctx)

	var allPVCs []corev1.PersistentVolumeClaim

	// Determine which namespaces to search
	namespaces := policy.Spec.Namespaces
	if len(namespaces) == 0 {
		// If no namespaces specified, search in policy's namespace
		namespaces = []string{policy.Namespace}
	}

	// Search in each namespace
	for _, ns := range namespaces {
		pvcList := &corev1.PersistentVolumeClaimList{}

		// List PVCs with label selector
		listOpts := []client.ListOption{
			client.InNamespace(ns),
		}

		if len(policy.Spec.Selector.MatchLabels) > 0 {
			listOpts = append(listOpts, client.MatchingLabels(policy.Spec.Selector.MatchLabels))
		}

		if err := r.List(ctx, pvcList, listOpts...); err != nil {
			logger.Error(err, "Failed to list PVCs", "namespace", ns)
			continue
		}

		allPVCs = append(allPVCs, pvcList.Items...)
	}

	return allPVCs, nil
}

func (r *BackupPolicyReconciler) hasActiveBackupJobs(ctx context.Context, policy *backupv1alpha1.BackupPolicy, pvcs []corev1.PersistentVolumeClaim) (bool, error) {
	namespaces := map[string]struct{}{
		policy.Namespace: {},
	}
	for _, ns := range policy.Spec.Namespaces {
		namespaces[ns] = struct{}{}
	}
	for _, pvc := range pvcs {
		namespaces[pvc.Namespace] = struct{}{}
	}

	for ns := range namespaces {
		jobList := &batchv1.JobList{}
		if err := r.List(ctx, jobList,
			client.InNamespace(ns),
			client.MatchingLabels{
				backup.LabelPolicy:          policy.Name,
				backup.LabelPolicyNamespace: policy.Namespace,
			}); err != nil {
			return false, err
		}

		for _, job := range jobList.Items {
			if job.Status.Active > 0 {
				return true, nil
			}
			if job.Status.Succeeded == 0 && job.Status.Failed == 0 && job.Status.CompletionTime == nil {
				return true, nil
			}
		}
	}

	return false, nil
}

// shouldBackupNow determines if a backup should be performed now based on the schedule
func (r *BackupPolicyReconciler) shouldBackupNow(policy *backupv1alpha1.BackupPolicy) (bool, time.Time) {
	logger := log.Log.WithName("shouldBackupNow")

	if policy.Status.NextRunTime != nil {
		next := policy.Status.NextRunTime.Time
		if time.Now().Before(next) {
			return false, next
		}
	}

	lastBackup := policy.Status.LastBackupTime
	if lastBackup == nil {
		// Never backed up, do it now
		return true, time.Now()
	}

	nextRun, err := r.nextRun(policy, lastBackup.Time)
	if err != nil {
		logger.Error(err, "Failed to parse cron schedule, using default 1 hour interval",
			"schedule", policy.Spec.Schedule)
	}

	if !time.Now().Before(nextRun) {
		return true, nextRun
	}

	return false, nextRun
}

func (r *BackupPolicyReconciler) nextRun(policy *backupv1alpha1.BackupPolicy, from time.Time) (time.Time, error) {
	if from.IsZero() {
		from = time.Now()
	}

	if policy.Spec.Schedule == "" {
		return from.Add(1 * time.Hour), nil
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(policy.Spec.Schedule)
	if err != nil {
		return from.Add(1 * time.Hour), err
	}

	return schedule.Next(from), nil
}

// updateStatus updates the BackupPolicy status
func (r *BackupPolicyReconciler) updateStatus(ctx context.Context, policy *backupv1alpha1.BackupPolicy, phase string, message string) {
	logger := log.FromContext(ctx)

	policy.Status.Phase = phase

	// Update conditions
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             phase,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if phase == "Error" {
		condition.Status = metav1.ConditionFalse
	}

	// Find and update or append condition
	found := false
	for i, c := range policy.Status.Conditions {
		if c.Type == "Ready" {
			policy.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		policy.Status.Conditions = append(policy.Status.Conditions, condition)
	}

	if err := r.Status().Update(ctx, policy); err != nil {
		logger.Error(err, "Failed to update BackupPolicy status")
	}
}

// updateStatusWithNextRun updates status with next run time
func (r *BackupPolicyReconciler) updateStatusWithNextRun(ctx context.Context, policy *backupv1alpha1.BackupPolicy, phase string, nextRun time.Time) {
	policy.Status.NextRunTime = &metav1.Time{Time: nextRun}
	r.updateStatus(ctx, policy, phase, fmt.Sprintf("Next backup at %s", nextRun.Format(time.RFC3339)))
}

// handleJobCompletion checks if a backup Job has completed and updates BackupPolicy status
func (r *BackupPolicyReconciler) handleJobCompletion(ctx context.Context, policy *backupv1alpha1.BackupPolicy) error {
	logger := log.FromContext(ctx)

	namespaces := map[string]struct{}{
		policy.Namespace: {},
	}
	for _, ns := range policy.Spec.Namespaces {
		namespaces[ns] = struct{}{}
	}
	for _, stored := range policy.Status.StoredBackups {
		if stored.Namespace != "" {
			namespaces[stored.Namespace] = struct{}{}
		}
	}

	jobsByKey := make(map[string]batchv1.Job)
	for ns := range namespaces {
		jobList := &batchv1.JobList{}
		if err := r.List(ctx, jobList,
			client.InNamespace(ns),
			client.MatchingLabels{
				backup.LabelPolicy:          policy.Name,
				backup.LabelPolicyNamespace: policy.Namespace,
			}); err != nil {
			logger.Error(err, "Failed to list backup Jobs", "namespace", ns)
			continue
		}

		for _, job := range jobList.Items {
			jobsByKey[backupKey(ns, job.Name)] = job
		}
	}

	changed := false
	var latestCompletion time.Time
	for i := range policy.Status.StoredBackups {
		stored := &policy.Status.StoredBackups[i]
		job, found := jobsByKey[backupKey(stored.Namespace, stored.Name)]
		if !found {
			continue
		}

		if job.Status.Succeeded > 0 && stored.Status != "Completed" {
			stored.Status = "Completed"
			if job.Status.CompletionTime != nil {
				stored.Timestamp = &metav1.Time{Time: job.Status.CompletionTime.Time}
				if job.Status.CompletionTime.After(latestCompletion) {
					latestCompletion = job.Status.CompletionTime.Time
				}
			}
			logger.Info("Backup Job completed successfully", "job", job.Name, "backup", stored.Name)
			changed = true
			continue
		}

		if job.Status.Failed > 0 && stored.Status != "Failed" {
			stored.Status = "Failed"
			logger.Error(nil, "Backup Job failed", "job", job.Name, "backup", stored.Name, "failed", job.Status.Failed)
			changed = true
		}
	}

	if !latestCompletion.IsZero() {
		if policy.Status.LastBackupTime == nil || latestCompletion.After(policy.Status.LastBackupTime.Time) {
			policy.Status.LastBackupTime = &metav1.Time{Time: latestCompletion}
		}
		nextRun, err := r.nextRun(policy, latestCompletion)
		if err != nil {
			logger.Error(err, "Failed to parse cron schedule, using default 1 hour interval", "schedule", policy.Spec.Schedule)
			nextRun = latestCompletion.Add(1 * time.Hour)
		}
		policy.Status.NextRunTime = &metav1.Time{Time: nextRun}
	}

	if !changed {
		return nil
	}

	if err := r.Status().Update(ctx, policy); err != nil {
		return fmt.Errorf("failed to update BackupPolicy status: %w", err)
	}

	return nil
}

// cleanupOldJobs removes old completed/failed Jobs to reduce API server load
// Keeps only the 3 most recent completed Jobs and immediately deletes failed Jobs
func (r *BackupPolicyReconciler) cleanupOldJobs(ctx context.Context, policy *backupv1alpha1.BackupPolicy) error {
	logger := log.FromContext(ctx)

	// Limit for successful Jobs to keep (like CronJob's successfulJobsHistoryLimit)
	successfulJobsHistoryLimit := 3
	// Limit for failed Jobs (keep 1 for debugging)
	failedJobsHistoryLimit := 1

	namespaces := map[string]struct{}{
		policy.Namespace: {},
	}
	for _, ns := range policy.Spec.Namespaces {
		namespaces[ns] = struct{}{}
	}

	for ns := range namespaces {
		jobList := &batchv1.JobList{}
		if err := r.List(ctx, jobList,
			client.InNamespace(ns),
			client.MatchingLabels{
				backup.LabelPolicy:          policy.Name,
				backup.LabelPolicyNamespace: policy.Namespace,
			}); err != nil {
			logger.Error(err, "Failed to list Jobs for cleanup", "namespace", ns)
			continue
		}

		// Separate Jobs by status
		var completedJobs, failedJobs, runningJobs []batchv1.Job
		for _, job := range jobList.Items {
			if job.Status.Succeeded > 0 {
				completedJobs = append(completedJobs, job)
			} else if job.Status.Failed > 0 {
				failedJobs = append(failedJobs, job)
			} else {
				runningJobs = append(runningJobs, job)
			}
		}

		// Sort completed Jobs by completion time (most recent first)
		sort.Slice(completedJobs, func(i, j int) bool {
			if completedJobs[i].Status.CompletionTime == nil {
				return false
			}
			if completedJobs[j].Status.CompletionTime == nil {
				return true
			}
			return completedJobs[i].Status.CompletionTime.After(completedJobs[j].Status.CompletionTime.Time)
		})

		// Delete old completed Jobs (keep only the most recent N)
		if len(completedJobs) > successfulJobsHistoryLimit {
			for i := successfulJobsHistoryLimit; i < len(completedJobs); i++ {
				job := &completedJobs[i]
				logger.Info("Deleting old completed Job", "job", job.Name, "namespace", job.Namespace)
				if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
					logger.Error(err, "Failed to delete old Job", "job", job.Name)
				}
			}
		}

		// Sort failed Jobs by start time (most recent first)
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return false
			}
			if failedJobs[j].Status.StartTime == nil {
				return true
			}
			return failedJobs[i].Status.StartTime.After(failedJobs[j].Status.StartTime.Time)
		})

		// Delete old failed Jobs (keep only 1 for debugging)
		if len(failedJobs) > failedJobsHistoryLimit {
			for i := failedJobsHistoryLimit; i < len(failedJobs); i++ {
				job := &failedJobs[i]
				logger.Info("Deleting old failed Job", "job", job.Name, "namespace", job.Namespace)
				if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
					logger.Error(err, "Failed to delete failed Job", "job", job.Name)
				}
			}
		}

		// Check for stuck running Jobs (running for more than 10 minutes)
		for _, job := range runningJobs {
			if job.Status.StartTime != nil {
				runningDuration := time.Since(job.Status.StartTime.Time)
				if runningDuration > 10*time.Minute {
					logger.Info("Deleting stuck running Job", "job", job.Name, "namespace", job.Namespace, "duration", runningDuration)
					if err := r.Delete(ctx, &job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
						logger.Error(err, "Failed to delete stuck Job", "job", job.Name)
					}
				}
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.BackupPolicy{}).
		Owns(&batchv1.Job{}). // Watch Jobs created by this controller
		Named("backuppolicy").
		Complete(r)
}
