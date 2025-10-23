# ConfigMap-Syncer Operator

## Project Overview

ConfigMap-Syncer is a Kubernetes Operator that synchronizes ConfigMap resources across multiple namespaces. This is an educational project designed to help learn Kubernetes Controller and Operator development.

## Features

- **ConfigMap Synchronization**: Sync source ConfigMaps to specified target namespaces
- **Selector Support**: Automatically discover target namespaces using label selectors
- **Status Tracking**: Provide detailed synchronization status and condition information
- **High Availability**: Support Leader Election for running multiple replicas

## Quick Start

### Development Mode

```bash
# Install CRD
make install

# Run Controller (local development)
make run
```

### Production Deployment

```bash
# Build image
make docker-build IMG=configmap-syncer:v1.0.0

# Deploy to cluster
make deploy IMG=configmap-syncer:v1.0.0
```

## Usage Examples

### Basic Usage

```yaml
apiVersion: sync.example.com/v1
kind: ConfigMapSync
metadata:
  name: basic-sync
  namespace: source-ns
spec:
  sourceConfigMap:
    name: app-config
    namespace: source-ns
  targetNamespaces:
    - target-ns1
    - target-ns2
```

### Using Selectors

```yaml
apiVersion: sync.example.com/v1
kind: ConfigMapSync
metadata:
  name: selector-sync
  namespace: source-ns
spec:
  sourceConfigMap:
    name: shared-config
    namespace: source-ns
  selector:
    matchLabels:
      sync: "enabled"
```

## Project Structure

```
operators/configmap-syncer/
├── api/v1/                 # CRD definitions
├── internal/controller/    # Controller logic
├── config/                 # Kubernetes configurations
├── cmd/                    # Program entry point
├── hack/                   # Build scripts
├── test/                   # Test files
├── Makefile               # Build commands
├── Dockerfile             # Container build
└── README.md              # Project documentation
```

## Learning Points

This project covers the following Kubernetes development concepts:

1. **CRD Design**: Custom Resource Definition
2. **Controller Pattern**: Controller pattern and Reconcile loop
3. **Watch Mechanism**: Resource change monitoring
4. **RBAC**: Permission management
5. **Status Management**: Status and condition updates
6. **Error Handling**: Error handling and retry mechanisms

## Build and Test

```bash
# Install dependencies
go mod tidy

# Run tests
make test

# Generate code
make generate

# Update CRDs
make manifests

# Build binary
make build
```

## Cleanup

```bash
# Stop development mode
# Press Ctrl+C to stop make run

# Clean up production deployment
make undeploy

# Delete CRD
make uninstall
```

## Complexity Level

⭐⭐☆☆☆ (Beginner)

Suitable as an introductory project for Kubernetes Operator development.
