# Backup Operator 开发指引

本说明文档针对已经使用 Operator SDK 初始化完成的 `backup-operator` 脚手架，帮助你快速熟悉现有结构并开始实现 PVC 备份与恢复逻辑。

## 环境依赖

### 必需组件
- **Go**: 1.21+ ([安装指南](https://golang.org/doc/install))
- **Operator SDK**: v1.33+ ([安装指南](https://sdk.operatorframework.io/docs/installation/))
- **kubectl**: 与集群版本兼容 ([安装指南](https://kubernetes.io/docs/tasks/tools/))
- **kustomize**: v5.0+ (通常随kubectl安装)
- **Kubernetes集群**: v1.24+ (可使用 kind/minikube 测试环境)

### 环境验证
运行以下命令验证环境是否就绪：
```bash
go version                    # 应显示 go1.21 或更高
operator-sdk version          # 应显示 v1.33 或更高
kubectl version --client      # 验证 kubectl 已安装
kubectl cluster-info          # 验证集群连接
```

## 快速开始

### 1. 安装 CRD 到集群
```bash
make install
```

### 2. 本地运行 operator
```bash
make run
```

### 3. 创建测试用的源 PVC（在另一个终端）
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

### 4. 应用样例 BackupPolicy
```bash
kubectl apply -f config/samples/backup_v1alpha1_backuppolicy.yaml
```

### 5. 查看资源状态
```bash
# 查看所有 BackupPolicy
kubectl get backuppolicies -A

# 查看详细信息
kubectl describe backuppolicy -n <namespace> <name>

# 查看关联的备份任务
kubectl get cronjobs -n <namespace>
kubectl get volumesnapshots -A
```

## 项目骨架回顾

脚手架已通过类似如下命令生成：
```bash
operator-sdk init \
  --domain backup.example.com \
  --repo github.com/example/backup-operator \
  --plugins go/v4
```

创建 `BackupPolicy` API：
```bash
operator-sdk create api \
  --group backup \
  --version v1alpha1 \
  --kind BackupPolicy \
  --resource --controller
```

**注意**: 当前项目的实际 module 路径见 `go.mod` 文件。若需修改，需同步更新 `go.mod`、`PROJECT` 文件以及所有 import 语句。

### 目录结构
```
backup-operator/
├── api/v1alpha1/              # CRD 类型定义
│   ├── backuppolicy_types.go  # BackupPolicy API 定义
│   └── groupversion_info.go   # API 版本信息
├── internal/controller/       # 控制器实现
│   ├── backuppolicy_controller.go      # 主 reconcile 逻辑
│   ├── backuppolicy_controller_test.go # 单元测试
│   └── suite_test.go          # 测试套件
├── config/                    # Kubernetes 配置
│   ├── crd/                  # CRD manifests
│   ├── rbac/                 # RBAC 权限配置
│   ├── manager/              # Operator 部署配置
│   ├── samples/              # 示例 CR
│   └── default/              # Kustomize 默认配置
├── test/                      # E2E 测试
│   ├── e2e/                  # E2E 测试代码
│   └── utils/                # 测试工具函数
├── cmd/main.go               # Operator 入口
├── Makefile                  # 构建和部署命令
├── Dockerfile                # 容器镜像构建
└── go.mod                    # Go 模块依赖
```

## 扩展 BackupPolicy API

### 1. 完善类型定义
在 `api/v1alpha1/backuppolicy_types.go` 中完善 `BackupPolicySpec` 与 `BackupPolicyStatus`：

#### Spec 建议字段：
- `targets`：PVC 选择器（支持名称或标签选择器）
- `schedule`：Cron 表达式（如 `"0 2 * * *"`）+ 可选时区
- `retention`：备份保留策略（结构化类型，包含最大备份数、保留天数等）
- `destination`：备份目标配置（S3、NFS等，通过 Secret 引用凭证）
- `restore`：默认恢复策略（可选）

#### Status 建议字段：
- `phase`：当前阶段（如 `Active`、`Error`、`Suspended`）
- `lastBackupTime`：最后一次备份时间
- `nextRunTime`：下次备份预计时间
- `storedBackups`：备份元数据列表（需定义 `StoredBackup` 结构体）
- `conditions`：条件列表（使用标准的 `metav1.Condition`）

#### 需要添加的类型：
```go
// StoredBackup 存储备份的元数据
type StoredBackup struct {
    Name      string       `json:"name"`
    Timestamp *metav1.Time `json:"timestamp"`
    PVCName   string       `json:"pvcName"`
    Size      string       `json:"size,omitempty"`
    Location  string       `json:"location"`
    Status    string       `json:"status"`
}

// RetentionPolicy 定义备份保留策略
type RetentionPolicy struct {
    MaxBackups int `json:"maxBackups,omitempty"`
    MaxDays    int `json:"maxDays,omitempty"`
}
```

### 2. 添加 Kubebuilder 标记
为字段添加验证和打印列标记：
```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
Schedule string `json:"schedule"`

// 在 BackupPolicy 结构体上添加
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Last Backup",type=date,JSONPath=`.status.lastBackupTime`
```

### 3. 更新样例 CR
编辑 `config/samples/backup_v1alpha1_backuppolicy.yaml`，保持与类型定义一致。

### 4. 生成代码与 CRD
每次修改类型定义后执行：
```bash
make generate  # 生成 DeepCopy 方法
make manifests # 生成 CRD YAML
```

## 控制器实现要点

在 `internal/controller/backuppolicy_controller.go` 中实现以下核心逻辑：

### 主要功能
1. **解析 BackupPolicy**
   - 解析 Cron 表达式，计算下次运行时间
   - 根据 `targets` 查找匹配的 PVC

2. **创建/更新备份任务**
   - 为每个目标 PVC 创建或更新 CronJob
   - 配置 CronJob 使用合适的备份工具（如 restic、velero 等）

3. **管理备份生命周期**
   - 监听 Job 完成事件，记录备份结果到 `status.storedBackups`
   - 根据 `retention` 策略清理过期备份
   - 更新 `status` 字段（phase、lastBackupTime、conditions）

4. **错误处理**
   - 设置合适的 `conditions`
   - 实现指数退避重试

### Watch 的资源
在 `SetupWithManager` 中配置 watch：
```go
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&backupv1alpha1.BackupPolicy{}).
        Owns(&batchv1.CronJob{}).        // Watch 创建的 CronJob
        Owns(&batchv1.Job{}).             // Watch 备份 Job
        Watches(                          // Watch VolumeSnapshot（可选）
            &snapshotv1.VolumeSnapshot{},
            handler.EnqueueRequestsFromMapFunc(r.findBackupPolicyForSnapshot),
        ).
        Complete(r)
}
```

### 推荐的代码组织
可按职责拆分辅助包到 `internal/` 或 `pkg/`：

- `internal/backup/`
  - `scheduler.go`：CronJob 管理和调度逻辑
  - `job.go`：备份 Job 模板渲染和执行

- `internal/snapshot/`
  - `snapshot.go`：CSI VolumeSnapshot 创建、查询、删除

- `internal/storage/`
  - `interface.go`：存储后端接口定义
  - `s3.go`：S3 后端实现
  - `nfs.go`：NFS 后端实现

- `internal/retention/`
  - `policy.go`：备份保留策略实现

## 本地调试流程

### 基本调试
```bash
# 1. 安装 CRDs
make install

# 2. 运行控制器（带详细日志）
make run

# 3. 在另一个终端应用样例 CR
kubectl apply -f config/samples/backup_v1alpha1_backuppolicy.yaml

# 4. 查看状态
kubectl get backuppolicies -A
kubectl describe backuppolicy <name> -n <namespace>
```

### 查看关联资源
```bash
# 查看创建的 CronJob
kubectl get cronjobs -n <namespace>
kubectl describe cronjob <cronjob-name> -n <namespace>

# 查看备份 Job
kubectl get jobs -n <namespace>
kubectl logs job/<job-name> -n <namespace>

# 查看 VolumeSnapshot（如果使用）
kubectl get volumesnapshots -A
kubectl describe volumesnapshot <snapshot-name> -n <namespace>

# 查看 Operator 日志（如果部署到集群）
kubectl logs -n backup-operator-system deployment/backup-operator-controller-manager -f
```

### 调试技巧
```bash
# 增加日志级别
make run ARGS="--zap-log-level=debug"

# 查看 CRD 定义
kubectl get crd backuppolicies.backup.backup.example.com -o yaml

# 查看 RBAC 权限
kubectl describe clusterrole backup-operator-manager-role
```

## RBAC 权限配置

在 `config/rbac/role.yaml` 中添加必要的权限：

```yaml
# 需要添加的权限示例
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

修改后执行 `make manifests` 更新生成的 RBAC 清单。

## 开发与测试

### 单元测试
```bash
# 运行所有测试
make test

# 运行特定测试
go test ./internal/controller -v -run TestBackupPolicyReconcile

# 查看测试覆盖率
make test-coverage
```

### E2E 测试
在 `test/e2e/e2e_test.go` 中编写端到端测试：
- 备份成功场景
- 备份失败重试
- 保留策略生效
- 恢复流程验证

```bash
# 运行 E2E 测试
make test-e2e
```

### 测试建议
1. 使用 `envtest` 进行控制器单元测试
2. 模拟各种边界情况（PVC 不存在、权限不足等）
3. 验证 status 更新的正确性
4. 测试并发场景（多个 BackupPolicy 同时运行）

## 构建与部署

### 本地构建镜像
```bash
# 构建镜像
make docker-build IMG=<your-registry>/backup-operator:tag

# 推送镜像
make docker-push IMG=<your-registry>/backup-operator:tag
```

### 部署到集群
```bash
# 部署 operator
make deploy IMG=<your-registry>/backup-operator:tag

# 查看部署状态
kubectl get deployment -n backup-operator-system

# 卸载
make undeploy
```

## 常见问题排查

### CRD 安装失败
```bash
# 检查 CRD 定义是否有效
kubectl apply --dry-run=client -f config/crd/bases/

# 手动安装 CRD
kubectl apply -f config/crd/bases/backup.backup.example.com_backuppolicies.yaml
```

### 控制器无法创建 CronJob
```bash
# 检查 RBAC 权限
kubectl auth can-i create cronjobs --as=system:serviceaccount:backup-operator-system:backup-operator-controller-manager

# 查看 Operator 日志
kubectl logs -n backup-operator-system deployment/backup-operator-controller-manager
```

### Reconcile 循环过快
- 检查 Reconcile 逻辑是否正确返回 `ctrl.Result{}`
- 确保只在必要时才 requeue
- 添加合适的 predicate 过滤不相关的事件

### 备份 Job 失败
```bash
# 查看 Job 日志
kubectl logs job/<backup-job-name> -n <namespace>

# 检查 PVC 是否存在
kubectl get pvc <pvc-name> -n <namespace>

# 检查存储凭证 Secret
kubectl get secret <secret-name> -n <namespace>
```

## 下一步

完成基础框架后，可以逐步实现：
1. ✅ 基本的 BackupPolicy CRD 定义和控制器
2. ⏳ CronJob 调度和备份 Job 创建
3. ⏳ VolumeSnapshot 或其他备份机制集成
4. ⏳ 多种存储后端支持（S3、NFS等）
5. ⏳ 备份保留和清理策略
6. ⏳ 恢复功能实现
7. ⏳ Webhook 验证和默认值设置
8. ⏳ Metrics 和告警

准备就绪后，即可在现有脚手架基础上实现完整的 PVC 备份与恢复能力。
