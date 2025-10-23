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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Target struct {
	// If PVCName is set, PVCLabelSelector should not be set
	PVCName          string               `json:"pvcName,omitempty"`
	PVCLabelSelector metav1.LabelSelector `json:"pvcLabelSelector,omitempty"`
	// If PVCName is not set, PVCLabelSelector must be set
	Namespace string `json:"namespace,omitempty"`
}

type Retention struct {
	MaxBackups int    `json:"maxBackups,omitempty"`
	MaxAge     string `json:"maxAge,omitempty"`
}

type Destination struct {
	// Backup destination type: s3, nfs, gcs, azure
	// +kubebuilder:validation:Enum=s3;nfs;gcs;azure
	Type string `json:"type,omitempty"`

	// Destination URL or endpoint
	// Examples:
	//   S3: s3://bucket-name/prefix
	//   NFS: nfs://server-address/export/path
	//   GCS: gs://bucket-name/prefix
	URL string `json:"url,omitempty"`

	// Custom endpoint for S3-compatible storage (e.g., MinIO)
	// Examples:
	//   MinIO: http://minio.minio.svc.cluster.local:9000
	//   Ceph: http://ceph-rgw.ceph.svc:8080
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Secret name containing credentials for accessing the destination
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`

	// Storage class for S3-compatible backends (STANDARD, GLACIER, DEEP_ARCHIVE)
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

type Restore struct {
	Namespace string               `json:"namespace,omitempty"`
	Selector  metav1.LabelSelector `json:"selector,omitempty"`
}

type StoredBackup struct {
	// Backup name/identifier
	Name string `json:"name"`

	// When this backup was created
	Timestamp *metav1.Time `json:"timestamp"`

	// Source PVC name
	PVCName string `json:"pvcName"`

	// Source PVC namespace
	Namespace string `json:"namespace"`

	// Backup size (human-readable, e.g., "1.5Gi")
	Size string `json:"size,omitempty"`

	// Full location/path of the backup
	// Examples:
	//   Snapshot: default/pvc-snapshot-xyz
	//   S3: s3://bucket/backups/mysql-20250103-020000.tar.gz
	Location string `json:"location"`

	// Backup status: Completed, Failed, InProgress
	Status string `json:"status"`

	// Backup strategy used: snapshot, external
	Strategy string `json:"strategy,omitempty"`
}

// BackupPolicySpec defines the desired state of BackupPolicy.
type BackupPolicySpec struct {
	// Label selector for PVCs to backup
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// Namespaces to search for PVCs (empty means all namespaces if RBAC permits)
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Cron schedule for backups (e.g., "0 2 * * *" for daily at 2 AM)
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// Backup strategy: "snapshot" (VolumeSnapshot) or "external" (S3/NFS)
	// snapshot: Fast, local, short-term (default)
	// external: Slower, remote, long-term
	// +kubebuilder:validation:Enum=snapshot;external
	// +kubebuilder:default=snapshot
	// +optional
	Strategy string `json:"strategy,omitempty"`

	// Retention policy for backup cleanup
	Retention Retention `json:"retention,omitempty"`

	// Destination for external backups (required when strategy=external)
	// +optional
	Destination Destination `json:"destination,omitempty"`

	// Restore configuration (optional, for future restore operations)
	// +optional
	Restore Restore `json:"restore,omitempty"`
}

// BackupPolicyStatus defines the observed state of BackupPolicy.
type BackupPolicyStatus struct {
	// Current phase: Active, Error, Suspended
	Phase string `json:"phase,omitempty"`

	// Timestamp of the last successful backup
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// Calculated next run time based on schedule
	NextRunTime *metav1.Time `json:"nextRunTime,omitempty"`

	// Total number of backups currently stored
	BackupCount int `json:"backupCount,omitempty"`

	// List of stored backups with metadata
	StoredBackups []StoredBackup `json:"storedBackups,omitempty"`

	// Standard condition types for status reporting
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Last Backup",type=date,JSONPath=`.status.lastBackupTime`
// +kubebuilder:printcolumn:name="Backups",type=integer,JSONPath=`.status.backupCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// BackupPolicy is the Schema for the backuppolicies API.
type BackupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupPolicySpec   `json:"spec,omitempty"`
	Status BackupPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupPolicyList contains a list of BackupPolicy.
type BackupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []BackupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicy{}, &BackupPolicyList{})
}
