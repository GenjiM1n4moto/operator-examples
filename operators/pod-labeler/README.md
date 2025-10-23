# Pod-Labeler Operator

## Project Overview

Pod-Labeler is a Kubernetes Operator that automatically adds labels to Pods based on predefined rules. This is an intermediate-level learning project that demonstrates multi-resource watching and Finalizer usage.

## Learning Objectives

- **Multi-resource Watching**: Simultaneously monitor Pod and Namespace resource changes
- **Finalizer Mechanism**: Ensure completeness of resource cleanup
- **Label Management**: Dynamic label assignment and updates
- **Lifecycle Control**: Full lifecycle management of Pod creation, updates, and deletion

## Features

- ✅ Automatically add labels to Pods based on namespace labels
- ✅ Support conditional labels based on Pod attributes
- ✅ Provide label cleanup mechanism
- ✅ Support label templates and variable substitution

## Complexity Level

⭐⭐⭐☆☆ (Intermediate)

## Status

📋 To Be Developed

## operator-sdk Initialization Commands

### 1. Initialize Project
```bash
cd operators/pod-labeler
operator-sdk init --domain example.com --repo github.com/rayhe/operator-example/operators/pod-labeler
```

### 2. Create API and Controller
```bash
operator-sdk create api --group labels --version v1 --kind PodLabeler --resource --controller
```

## TODO After Executing operator-sdk Commands

### 1. Define CRD (Custom Resource Definition)
- [ ] Edit `api/v1/podlabeler_types.go`
- [ ] Define Spec fields:
  - selector: Select which Pods to label
  - labelRules: List of label rules (key, value, valueFrom)
  - conditions: Conditional label rules
- [ ] Define Status fields:
  - labeledPodsCount: Number of labeled Pods
  - lastSyncTime: Last synchronization time
  - conditions: Status conditions
- [ ] Run `make generate` to update generated code
- [ ] Run `make manifests` to generate CRD manifests

### 2. Implement Controller Logic
- [ ] Edit `internal/controller/podlabeler_controller.go`
- [ ] Implement Reconcile loop:
  - Watch PodLabeler CR changes
  - List matching Pods based on selector
  - Parse labelRules and conditions
  - Apply labels to matching Pods
  - Update CR status
- [ ] Add Finalizer handling logic (cleanup labels)
- [ ] Set up Watch to monitor Pod resource changes

### 3. Configure RBAC
- [ ] Check generated RBAC configuration in `config/rbac/role.yaml`
- [ ] Ensure controller has the following permissions:
  - List/Watch/Get Pods
  - Update Pod labels
  - Get/Update PodLabeler CRs and their status

### 4. Write Tests
- [ ] Write unit tests in `internal/controller/podlabeler_controller_test.go`
- [ ] Write e2e tests in `test/e2e/e2e_test.go`
- [ ] Test edge cases (Pod not exists, permission errors, etc.)

### 5. Create Sample Resources
- [ ] Create sample PodLabeler CR in `config/samples/`
- [ ] Create test Pod manifests
- [ ] Refer to YAML examples in the project overview

### 6. Build and Deploy
- [ ] Build image: `make docker-build IMG=<registry>/pod-labeler:tag`
- [ ] Push image: `make docker-push IMG=<registry>/pod-labeler:tag`
- [ ] Install CRD: `make install`
- [ ] Deploy operator: `make deploy IMG=<registry>/pod-labeler:tag`

### 7. Verify and Debug
- [ ] Create test Pods and PodLabeler CR
- [ ] Verify labels are correctly applied
- [ ] Check operator logs
- [ ] Test Finalizer cleanup logic

## Common Development Commands

```bash
# Generate code (after modifying types)
make generate

# Generate manifests (CRDs, RBAC, etc.)
make manifests

# Run tests
make test

# Run locally (without deploying to cluster)
make run

# Install CRD to cluster
make install

# Build Docker image
make docker-build IMG=<your-registry>/pod-labeler:tag

# Deploy to cluster
make deploy IMG=<your-registry>/pod-labeler:tag

# Uninstall CRD
make uninstall

# Undeploy controller
make undeploy
```

## Planned Feature Example

```yaml
apiVersion: labels.example.com/v1
kind: PodLabeler
metadata:
  name: production-pods
  namespace: default
spec:
  selector:
    matchLabels:
      environment: "production"
  labelRules:
    - key: "auto-labeled"
      value: "true"
    - key: "environment"
      valueFrom: "namespace.labels.environment"
    - key: "team"
      valueFrom: "pod.labels.team"
  conditions:
    - field: "pod.spec.nodeName"
      operator: "NotEmpty"
      labels:
        node-scheduled: "true"
```

## Implementation Tips

1. **valueFrom Parsing**: Need to implement logic to extract values from namespace.labels or pod.labels
2. **Condition Evaluation**: Implement field value checking (NotEmpty, Equals, and other operators)
3. **Finalizer**: Clean up added labels when deleting PodLabeler CR
4. **Watch Setup**: Use `Owns()` or `Watches()` to monitor Pod changes

A suitable project for learning intermediate Kubernetes development concepts.
