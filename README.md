# Kubernetes Operator Examples

A collection of production-ready Kubernetes operators demonstrating progressive complexity in operator development patterns, from basic resource synchronization to advanced backup automation with scheduling and multiple strategies.

## Project Overview

This repository contains Kubernetes operators built using the Operator SDK and Kubebuilder framework, demonstrating progressive complexity:

1. **ConfigMap-Syncer** - Synchronizes ConfigMaps across namespaces
2. **Pod-Labeler** - Dynamically labels pods with finalizer-based cleanup
3. **Backup-Operator** - Automated PVC backup with scheduling and retention policies
4. **Database-Operator** (Planned) - Multi-database cluster management (PostgreSQL first, with plans for MySQL, Redis, etc.)

The first three operators are fully functional and showcase different aspects of Kubernetes operator development, including custom resource definitions (CRDs), reconciliation loops, status management, RBAC configuration, and comprehensive testing.

## Operators

### 1. ConfigMap-Syncer (Beginner Level)

**Purpose**: Synchronizes ConfigMap resources from a source namespace to multiple target namespaces, enabling centralized configuration management.

**Key Features**:
- Explicit namespace list or label selector-based targeting
- Periodic synchronization to detect configuration drift
- Status tracking with conditions
- Preservation of both string and binary data

**Use Cases**:
- Centralized configuration management
- Multi-tenant environments with shared configurations
- Configuration propagation across environments

**Documentation**: [operators/configmap-syncer/](operators/configmap-syncer/)

### 2. Pod-Labeler (Intermediate Level)

**Purpose**: Automatically applies labels to pods based on configurable rules, supporting both static values and dynamic extraction from namespace/pod labels.

**Key Features**:
- Static label values
- Dynamic value extraction from namespace or pod labels
- Finalizer pattern for automatic cleanup on deletion
- CEL (Common Expression Language) validation at CRD level
- Multi-resource watching (reacts to pod and namespace changes)

**Use Cases**:
- Automatic environment labeling based on namespace
- Cost allocation tagging
- Service mesh metadata injection
- Compliance and governance labeling

**Documentation**: [operators/pod-labeler/](operators/pod-labeler/)

### 3. Backup-Operator (Advanced Level)

**Purpose**: Provides automated PVC backup capabilities with cron-based scheduling, retention policies, and multiple backup strategies.

**Key Features**:
- **Two Backup Strategies**:
  - **VolumeSnapshot**: Fast, local snapshots using CSI VolumeSnapshot
  - **External Storage**: Long-term archival to S3-compatible storage via Restic
- Cron-based scheduling with conflict prevention
- Retention policies (max backups count and age-based)
- Job lifecycle management with automatic cleanup
- Status history tracking (bounded to 200 entries)
- Cross-namespace backup support with credential management
- S3-compatible storage support (AWS S3, MinIO, Ceph)

**Use Cases**:
- Automated database backup
- Disaster recovery planning
- Compliance and data retention
- Dev/staging environment snapshots

**Documentation**: [operators/backup-operator/](operators/backup-operator/)

## Technology Stack

- **Language**: Go 1.21+
- **Framework**: Operator SDK / Kubebuilder
- **Testing**: Ginkgo/Gomega with envtest
- **Validation**: CEL (Common Expression Language)
- **Dependencies**:
  - `sigs.k8s.io/controller-runtime` - Controller framework
  - `k8s.io/api` and `k8s.io/apimachinery` - Kubernetes types
  - `github.com/robfig/cron/v3` - Cron parsing (backup-operator)
  - `github.com/aws/aws-sdk-go-v2` - AWS S3 SDK (backup-operator)

## Getting Started

### Prerequisites

- Go 1.21 or later
- Kubernetes cluster (1.24+)
- kubectl configured
- Operator SDK or Kubebuilder (for development)

### Installation

Each operator can be installed independently:

```bash
# Navigate to the operator directory
cd operators/<operator-name>

# Install CRDs to the cluster
make install

# Run the operator locally (for development)
make run

# Or deploy to the cluster
make docker-build IMG=<your-registry>/operator:tag
make docker-push IMG=<your-registry>/operator:tag
make deploy IMG=<your-registry>/operator:tag
```

### Quick Start Examples

#### ConfigMap-Syncer Example

```yaml
apiVersion: sync.example.com/v1
kind: ConfigMapSync
metadata:
  name: config-sync-example
  namespace: default
spec:
  sourceConfigMap:
    name: my-config
    namespace: default
  targetNamespaces:
    - dev
    - staging
    - prod
```

#### Pod-Labeler Example

```yaml
apiVersion: labels.example.com/v1
kind: PodLabeler
metadata:
  name: env-labeler
  namespace: production
spec:
  selector:
    matchLabels:
      app: myapp
  labelRules:
    - key: environment
      value: production
    - key: region
      valueFrom: namespace.labels.region
```

#### Backup-Operator Example

```yaml
apiVersion: backup.backup.example.com/v1alpha1
kind: BackupPolicy
metadata:
  name: daily-db-backup
  namespace: databases
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  strategy: external
  selector:
    matchLabels:
      backup: enabled
  namespaces:
    - databases
  retention:
    maxBackups: 7
    maxAge: 720h
  destination:
    url: s3://my-bucket/backups
    credentialsSecret: s3-credentials
```

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/GenjiM1n4moto/operator-examples.git
cd operator-examples

# Navigate to specific operator
cd operators/<operator-name>

# Generate code and manifests
make generate
make manifests

# Run tests
make test

# Build binary
make build
```

### Testing

Each operator includes comprehensive testing:

```bash
# Run unit tests with envtest
make test

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Architecture and Design Patterns

### Common Patterns Across All Operators

1. **Reconciliation Loop**: Standard Kubernetes controller pattern
2. **Status Subresource**: Separate spec (desired state) from status (observed state)
3. **Conditions Pattern**: Standard Kubernetes conditions for operational visibility
4. **RBAC**: Least-privilege principle with auto-generated permissions
5. **Label Selectors**: Dynamic resource discovery
6. **Error Handling**: Graceful degradation with partial success patterns

### Advanced Patterns (Backup-Operator)

- **Strategy Pattern**: Pluggable backup implementations
- **Job Management**: Create, monitor, and cleanup Kubernetes Jobs
- **Cron Scheduling**: Time-based automation with conflict prevention
- **Backend Abstraction**: Interface-based storage for portability
- **Status History**: Bounded backup metadata tracking
- **Cross-Namespace Operations**: Credential propagation for multi-namespace backups

## Roadmap

### Upcoming Operators

- **Database-Operator** (Planned): An advanced-level operator for managing database clusters across multiple database systems. The operator will support various databases with a unified API, starting with PostgreSQL as the first implementation:

  **Planned Database Support**:
  - PostgreSQL (First Implementation)
  - MySQL/MariaDB (Future)
  - Redis (Future)
  - MongoDB (Future)

  **Core Features**:
  - Automated database cluster deployment and lifecycle management
  - Primary-replica replication configuration
  - Automatic failover for high availability
  - Data backup and restore capabilities
  - Monitoring and alerting integration
  - Rolling upgrade support with zero downtime
  - Complex state machine for cluster lifecycle management
  - Data persistence with safety and consistency guarantees

  The initial PostgreSQL implementation will serve as a reference architecture for adding support for other database systems.

### Planned Improvements

- Enhanced E2E testing infrastructure
- Helm charts for easier deployment
- Operator Lifecycle Manager (OLM) integration
- Prometheus metrics and Grafana dashboards
- Webhooks for admission control and conversion

## Contributing

Contributions are welcome! Please feel free to submit issues, fork the repository, and create pull requests for any improvements.

### Development Workflow

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the Apache License 2.0 - see individual operator directories for details.

## Acknowledgments

- Built with [Operator SDK](https://sdk.operatorframework.io/)
- Inspired by Kubernetes community best practices
- Testing framework provided by [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/)

## Contact

For questions, issues, or suggestions, please open an issue on GitHub.

## Resources

- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Operator SDK Documentation](https://sdk.operatorframework.io/docs/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime)
