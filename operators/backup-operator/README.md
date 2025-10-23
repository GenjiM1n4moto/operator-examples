# Backup Operator Development Guide

This guide is designed for the `backup-operator` scaffold that has already been initialized using Operator SDK. It helps you quickly understand the existing structure and start implementing PVC backup and restore logic.

## Environment Dependencies

### Required Components
- **Go**: 1.21+ ([Installation Guide](https://golang.org/doc/install))
- **Operator SDK**: v1.33+ ([Installation Guide](https://sdk.operatorframework.io/docs/installation/))
- **kubectl**: Compatible with cluster version ([Installation Guide](https://kubernetes.io/docs/tasks/tools/))
- **kustomize**: v5.0+ (usually installed with kubectl)
- **Kubernetes Cluster**: v1.24+ (can use kind/minikube for testing)

### Environment Verification
Run the following commands to verify your environment is ready:
```bash
go version                    # Should show go1.21 or higher
operator-sdk version          # Should show v1.33 or higher
kubectl version --client      # Verify kubectl is installed
kubectl cluster-info          # Verify cluster connection
```

## Quick Start

### 1. Install CRDs to the cluster
```bash
make install
```

### 2. Run operator locally
```bash
make run
```

### 3. Create a test source PVC (in another terminal)
```bash
kubectl create namespace backup-test
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
  namespace: backup-test
  labels:
    app: demo
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
EOF
```

### 4. Apply sample BackupPolicy
```bash
kubectl apply -f config/samples/backup_v1alpha1_backuppolicy.yaml
```

### 5. Check resource status
```bash
# List all BackupPolicies
kubectl get backuppolicies -A

# View detailed information
kubectl describe backuppolicy -n <namespace> <name>

# View associated backup jobs
kubectl get cronjobs -n <namespace>
kubectl get volumesnapshots -A
```

## Project Scaffold Overview

The scaffold was generated using commands similar to:
```bash
operator-sdk init \
  --domain backup.example.com \
  --repo github.com/example/backup-operator \
  --plugins go/v4
```

Creating the `BackupPolicy` API:
```bash
operator-sdk create api \
  --group backup \
  --version v1alpha1 \
  --kind BackupPolicy \
  --resource --controller
```

**Note**: The actual module path of the current project is in the `go.mod` file. If you need to modify it, you must synchronize updates to `go.mod`, `PROJECT` file, and all import statements.

### Directory Structure
```
backup-operator/
├── api/v1alpha1/              # CRD type definitions
│   ├── backuppolicy_types.go  # BackupPolicy API definition
│   └── groupversion_info.go   # API version info
├── internal/controller/       # Controller implementation
│   ├── backuppolicy_controller.go      # Main reconcile logic
│   ├── backuppolicy_controller_test.go # Unit tests
│   └── suite_test.go          # Test suite
├── config/                    # Kubernetes configuration
│   ├── crd/                  # CRD manifests
│   ├── rbac/                 # RBAC permissions
│   ├── manager/              # Operator deployment config
│   ├── samples/              # Sample CRs
│   └── default/              # Kustomize default config
├── test/                      # E2E tests
│   ├── e2e/                  # E2E test code
│   └── utils/                # Test utility functions
├── cmd/main.go               # Operator entry point
├── Makefile                  # Build and deployment commands
├── Dockerfile                # Container image build
└── go.mod                    # Go module dependencies
```

## Extending BackupPolicy API

### 1. Complete Type Definitions
Complete the `BackupPolicySpec` and `BackupPolicyStatus` in `api/v1alpha1/backuppolicy_types.go`:

#### Recommended Spec fields:
- `targets`: PVC selector (supports name or label selector)
- `schedule`: Cron expression (e.g., `"0 2 * * *"`) + optional timezone
- `retention`: Backup retention policy (structured type including max backups, retention days, etc.)
- `destination`: Backup destination configuration (S3, NFS, etc., with credentials via Secret reference)
- `restore`: Default restore strategy (optional)

#### Recommended Status fields:
- `phase`: Current phase (e.g., `Active`, `Error`, `Suspended`)
- `lastBackupTime`: Last backup time
- `nextRunTime`: Next scheduled backup time
- `storedBackups`: Backup metadata list (needs `StoredBackup` struct definition)
- `conditions`: Condition list (using standard `metav1.Condition`)

#### Types to add:
```go
// StoredBackup stores backup metadata
type StoredBackup struct {
    Name      string       `json:"name"`
    Timestamp *metav1.Time `json:"timestamp"`
    PVCName   string       `json:"pvcName"`
    Size      string       `json:"size,omitempty"`
    Location  string       `json:"location"`
    Status    string       `json:"status"`
}

// RetentionPolicy defines backup retention policy
type RetentionPolicy struct {
    MaxBackups int `json:"maxBackups,omitempty"`
    MaxDays    int `json:"maxDays,omitempty"`
}
```

### 2. Add Kubebuilder Markers
Add validation and printer column markers to fields:
```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
Schedule string `json:"schedule"`

// Add on BackupPolicy struct
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Last Backup",type=date,JSONPath=`.status.lastBackupTime`
```

### 3. Update Sample CR
Edit `config/samples/backup_v1alpha1_backuppolicy.yaml` to match type definitions.

### 4. Generate Code and CRDs
Execute after each type definition modification:
```bash
make generate  # Generate DeepCopy methods
make manifests # Generate CRD YAML
```

## Controller Implementation Key Points

Implement the following core logic in `internal/controller/backuppolicy_controller.go`:

### Main Features
1. **Parse BackupPolicy**
   - Parse Cron expressions, calculate next run time
   - Find matching PVCs based on `targets`

2. **Create/Update Backup Jobs**
   - Create or update CronJob for each target PVC
   - Configure CronJob to use appropriate backup tools (e.g., restic, velero)

3. **Manage Backup Lifecycle**
   - Listen for Job completion events, record backup results in `status.storedBackups`
   - Clean up expired backups according to `retention` policy
   - Update `status` fields (phase, lastBackupTime, conditions)

4. **Error Handling**
   - Set appropriate `conditions`
   - Implement exponential backoff retry

### Resources to Watch
Configure watches in `SetupWithManager`:
```go
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&backupv1alpha1.BackupPolicy{}).
        Owns(&batchv1.CronJob{}).        // Watch created CronJobs
        Owns(&batchv1.Job{}).             // Watch backup Jobs
        Watches(                          // Watch VolumeSnapshot (optional)
            &snapshotv1.VolumeSnapshot{},
            handler.EnqueueRequestsFromMapFunc(r.findBackupPolicyForSnapshot),
        ).
        Complete(r)
}
```

### Recommended Code Organization
Split helper packages by responsibility into `internal/` or `pkg/`:

- `internal/backup/`
  - `scheduler.go`: CronJob management and scheduling logic
  - `job.go`: Backup Job template rendering and execution

- `internal/snapshot/`
  - `snapshot.go`: CSI VolumeSnapshot creation, query, deletion

- `internal/storage/`
  - `interface.go`: Storage backend interface definition
  - `s3.go`: S3 backend implementation
  - `nfs.go`: NFS backend implementation

- `internal/retention/`
  - `policy.go`: Backup retention policy implementation

## Local Debugging Workflow

### Basic Debugging
```bash
# 1. Install CRDs
make install

# 2. Run controller (with verbose logging)
make run

# 3. Apply sample CR in another terminal
kubectl apply -f config/samples/backup_v1alpha1_backuppolicy.yaml

# 4. Check status
kubectl get backuppolicies -A
kubectl describe backuppolicy <name> -n <namespace>
```

### View Related Resources
```bash
# View created CronJobs
kubectl get cronjobs -n <namespace>
kubectl describe cronjob <cronjob-name> -n <namespace>

# View backup Jobs
kubectl get jobs -n <namespace>
kubectl logs job/<job-name> -n <namespace>

# View VolumeSnapshots (if used)
kubectl get volumesnapshots -A
kubectl describe volumesnapshot <snapshot-name> -n <namespace>

# View Operator logs (if deployed to cluster)
kubectl logs -n backup-operator-system deployment/backup-operator-controller-manager -f
```

### Debugging Tips
```bash
# Increase log level
make run ARGS="--zap-log-level=debug"

# View CRD definition
kubectl get crd backuppolicies.backup.backup.example.com -o yaml

# View RBAC permissions
kubectl describe clusterrole backup-operator-manager-role
```

## RBAC Permission Configuration

Add necessary permissions in `config/rbac/role.yaml`:

```yaml
# Example permissions to add
rules:
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]

- apiGroups: ["batch"]
  resources: ["cronjobs", "jobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

- apiGroups: ["snapshot.storage.k8s.io"]
  resources: ["volumesnapshots"]
  verbs: ["get", "list", "watch", "create", "delete"]

- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
```

Execute `make manifests` after modifications to update generated RBAC manifests.

## Development and Testing

### Unit Tests
```bash
# Run all tests
make test

# Run specific test
go test ./internal/controller -v -run TestBackupPolicyReconcile

# View test coverage
make test-coverage
```

### E2E Tests
Write end-to-end tests in `test/e2e/e2e_test.go`:
- Successful backup scenarios
- Backup failure and retry
- Retention policy effectiveness
- Restore process verification

```bash
# Run E2E tests
make test-e2e
```

### Testing Recommendations
1. Use `envtest` for controller unit testing
2. Mock various edge cases (PVC not found, insufficient permissions, etc.)
3. Verify correctness of status updates
4. Test concurrent scenarios (multiple BackupPolicies running simultaneously)

## Build and Deployment

### Build Image Locally
```bash
# Build image
make docker-build IMG=<your-registry>/backup-operator:tag

# Push image
make docker-push IMG=<your-registry>/backup-operator:tag
```

### Deploy to Cluster
```bash
# Deploy operator
make deploy IMG=<your-registry>/backup-operator:tag

# Check deployment status
kubectl get deployment -n backup-operator-system

# Uninstall
make undeploy
```

## Troubleshooting

### CRD Installation Failed
```bash
# Check if CRD definition is valid
kubectl apply --dry-run=client -f config/crd/bases/

# Manually install CRD
kubectl apply -f config/crd/bases/backup.backup.example.com_backuppolicies.yaml
```

### Controller Cannot Create CronJob
```bash
# Check RBAC permissions
kubectl auth can-i create cronjobs --as=system:serviceaccount:backup-operator-system:backup-operator-controller-manager

# View Operator logs
kubectl logs -n backup-operator-system deployment/backup-operator-controller-manager
```

### Reconcile Loop Too Fast
- Check if Reconcile logic correctly returns `ctrl.Result{}`
- Ensure requeue only when necessary
- Add appropriate predicates to filter irrelevant events

### Backup Job Failed
```bash
# View Job logs
kubectl logs job/<backup-job-name> -n <namespace>

# Check if PVC exists
kubectl get pvc <pvc-name> -n <namespace>

# Check storage credentials Secret
kubectl get secret <secret-name> -n <namespace>
```

## Next Steps

After completing the basic framework, you can gradually implement:
1. ✅ Basic BackupPolicy CRD definition and controller
2. ⏳ CronJob scheduling and backup Job creation
3. ⏳ VolumeSnapshot or other backup mechanism integration
4. ⏳ Multiple storage backend support (S3, NFS, etc.)
5. ⏳ Backup retention and cleanup policies
6. ⏳ Restore functionality implementation
7. ⏳ Webhook validation and default value settings
8. ⏳ Metrics and alerting

Once ready, you can implement complete PVC backup and restore capabilities on the existing scaffold.
