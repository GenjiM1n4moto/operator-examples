# Backup Operator 测试环境摘要

## ✅ 已部署的资源

### 1. MinIO 对象存储
```
Namespace: minio
Service: minio.minio.svc.cluster.local:9000
Status: Running ✅
Buckets: backups, test-bucket
```

### 2. 测试 PVC
```
Name: test-backup-pvc
Namespace: default
Size: 1Gi
Status: Bound ✅
Data: ~10MB 测试数据
```

### 3. BackupPolicy CRD
```
Name: test-minio-backup
Strategy: external (MinIO/S3)
Schedule: */5 * * * * (每 5 分钟)
Target: PVC with label app=demo
Status: Created ✅
```

## 🔑 MinIO 访问信息

```yaml
Endpoint: http://minio.minio.svc.cluster.local:9000
Access Key: minioadmin
Secret Key: minioadmin123
Bucket: backups
```

## 📋 测试步骤

### 步骤 1: 验证所有资源就绪

```bash
# 检查 MinIO
kubectl get pods -n minio
kubectl get svc -n minio

# 检查测试 PVC
kubectl get pvc test-backup-pvc
kubectl get pod test-data-writer

# 检查 BackupPolicy
kubectl get backuppolicies
```

### 步骤 2: 运行 Operator

```bash
cd /home/rayhe/github/operator-example/operators/backup-operator

# 运行 operator（前台）
make run

# 或者在后台运行
make run > /tmp/operator.log 2>&1 &
```

### 步骤 3: 观察 Operator 日志

Operator 应该会：
1. 检测到 BackupPolicy CR
2. 根据 selector 找到 test-backup-pvc
3. 创建备份 Job（因为是首次运行）
4. Job 将 PVC 数据备份到 MinIO

预期日志输出：
```
INFO    Reconciling BackupPolicy    name=test-minio-backup
INFO    Found target PVCs    count=1
INFO    Creating backup Job for external storage
INFO    Backup Job created (simulated)
```

### 步骤 4: 验证备份Job创建

```bash
# 查看创建的 Job
kubectl get jobs -n default

# 查看 Job Pod
kubectl get pods -l backup.backup.example.com/policy=test-minio-backup

# 查看 Job 日志
kubectl logs job/<job-name>
```

### 步骤 5: 验证 MinIO 中的备份

```bash
# 使用 mc 检查备份文件
kubectl run -it --rm minio-check --image=minio/mc --restart=Never -- sh -c "
  mc alias set myminio http://minio.minio.svc.cluster.local:9000 minioadmin minioadmin123
  mc ls myminio/backups/test/
"
```

### 步骤 6: 检查 BackupPolicy Status

```bash
# 查看状态
kubectl describe backuppolicy test-minio-backup

# 查看完整 YAML
kubectl get backuppolicy test-minio-backup -o yaml
```

预期 status 字段：
```yaml
status:
  phase: Active
  lastBackupTime: "2025-10-05T06:15:00Z"
  backupCount: 1
  storedBackups:
  - name: test-minio-backup-test-backup-pvc-20251005-061500
    timestamp: "2025-10-05T06:15:00Z"
    pvcName: test-backup-pvc
    namespace: default
    location: s3://backups/test/...
    status: Completed
    strategy: external
```

## 🧪 测试场景

### 场景 1: 基本备份功能
- ✅ Operator 检测 BackupPolicy
- ✅ 查找匹配的 PVC
- ✅ 创建备份 Job
- ⏳ Job 执行备份到 MinIO
- ⏳ 更新 status.storedBackups

### 场景 2: 调度功能
- ⏳ 等待 5 分钟
- ⏳ Operator 自动触发下一次备份
- ⏳ 验证 nextRunTime 更新

### 场景 3: 保留策略
- ⏳ 创建超过 5 个备份
- ⏳ 验证旧备份被清理
- ⏳ 只保留最近 5 个备份

### 场景 4: 错误处理
- ⏳ 模拟 MinIO 不可用
- ⏳ 验证 Operator 重试
- ⏳ 检查 conditions 中的错误信息

## 📊 当前实现状态

### 已实现 ✅
- CRD 定义和验证
- BackupPolicy Reconciler 框架
- 策略接口 (Strategy pattern)
- PVC 发现逻辑
- S3Backend 接口（模拟）
- MinIO 部署和配置
- 测试资源创建

### 模拟实现 ⚠️
当前以下功能是**模拟**的（打印日志但不实际执行）：
- S3Backend.Upload/Download/Delete
- VolumeSnapshot 创建
- Backup Job 创建（定义了但未实际 create）

### 待实现 ⏳
1. **实际 Job 创建**
   - 调用 `r.client.Create(ctx, job)`
   - 等待 Job 完成
   - 获取备份结果

2. **S3 SDK 集成**
   - 导入 AWS SDK v2
   - 实现真实的 upload/download
   - 支持 MinIO endpoint

3. **Job 状态监听**
   - Watch Job 完成事件
   - 更新 BackupPolicy status
   - 错误处理和重试

4. **Cron 调度**
   - 集成 robfig/cron
   - 解析 schedule 表达式
   - 计算 nextRunTime

## 🔧 开发建议

### 下一步实现优先级

1. **让 Job 真正创建** (最重要)
   ```go
   // In external_strategy.go
   if err := e.client.Create(ctx, job); err != nil {
       return nil, fmt.Errorf("failed to create backup Job: %w", err)
   }
   ```

2. **验证 Job 创建**
   ```bash
   kubectl get jobs
   kubectl describe job <job-name>
   ```

3. **检查 Job 日志**
   ```bash
   kubectl logs job/<job-name>
   ```

4. **调试备份脚本**
   - 确保 restic 命令正确
   - 验证 MinIO 凭证可用
   - 检查网络连通性

## 📝 测试清单

- [ ] MinIO 运行正常
- [ ] 测试 PVC 包含数据
- [ ] BackupPolicy 创建成功
- [ ] Operator 启动并 reconcile
- [ ] 备份 Job 被创建
- [ ] Job Pod 启动
- [ ] 备份数据上传到 MinIO
- [ ] BackupPolicy status 更新
- [ ] 5分钟后自动触发第二次备份
- [ ] 保留策略生效

## 🐛 常见问题

### Operator 无法启动
```bash
# 检查 Go 模块
go mod tidy

# 重新生成代码
make generate
make manifests
```

### Job 创建失败
```bash
# 检查 RBAC 权限
kubectl auth can-i create jobs --as=system:serviceaccount:default:default

# 查看 Operator 日志中的错误
```

### 备份失败
```bash
# 检查 Job Pod 日志
kubectl logs <job-pod-name>

# 检查 Secret
kubectl get secret minio-credentials -o yaml

# 测试 MinIO 连接
kubectl run test --rm -it --image=minio/mc -- \
  mc alias set test http://minio.minio.svc.cluster.local:9000 minioadmin minioadmin123
```

## 🎯 成功标准

测试成功的标志：
1. ✅ Operator 运行无错误
2. ✅ 备份 Job 创建成功
3. ✅ Job 完成 (Status=Completed)
4. ✅ MinIO bucket 中有备份文件
5. ✅ BackupPolicy.status.storedBackups 有记录
6. ✅ 自动调度工作正常

## 🚀 测试命令一览

```bash
# 环境准备
make install
make run

# 资源检查
kubectl get backuppolicies
kubectl get pvc test-backup-pvc
kubectl get pods -n minio
kubectl get jobs

# 日志查看
kubectl logs -f deployment/backup-operator-controller-manager -n backup-operator-system
kubectl logs job/<backup-job-name>

# MinIO 验证
kubectl port-forward -n minio svc/minio-console 9001:9001
# 浏览器: http://localhost:9001

# 清理
kubectl delete backuppolicy test-minio-backup
kubectl delete pvc test-backup-pvc
kubectl delete pod test-data-writer
kubectl delete -f test/minio-deployment.yaml
```
