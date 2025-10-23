# MinIO 测试环境

本目录包含用于测试 backup-operator 的 MinIO 部署配置。

## 已部署的 MinIO 实例

### 📊 部署信息

```
Namespace: minio
Service: minio.minio.svc.cluster.local
API Port: 9000 (S3 API)
Console Port: 9001 (Web UI)
```

### 🔑 访问凭证

```
Access Key: minioadmin
Secret Key: minioadmin123
```

### 🪣 已创建的 Buckets

- `backups` - 用于备份存储
- `test-bucket` - 用于测试

## 快速使用

### 1. 查看 MinIO 状态

```bash
# 查看 Pod
kubectl get pods -n minio

# 查看 Service
kubectl get svc -n minio

# 查看 MinIO 日志
kubectl logs -n minio deployment/minio
```

### 2. 访问 MinIO Console（Web UI）

```bash
# 通过 port-forward 访问
kubectl port-forward -n minio svc/minio-console 9001:9001

# 然后在浏览器打开: http://localhost:9001
# 登录: minioadmin / minioadmin123
```

或者通过 NodePort（已配置）：

```bash
# 获取 Minikube IP
minikube ip

# 访问: http://<minikube-ip>:30001
```

### 3. 使用 MinIO Client (mc)

```bash
# 进入 MinIO Pod
kubectl exec -it -n minio deployment/minio -- sh

# 配置 alias
mc alias set local http://localhost:9000 minioadmin minioadmin123

# 列出 buckets
mc ls local/

# 创建新 bucket
mc mb local/new-bucket

# 上传文件
mc cp /etc/hostname local/backups/test.txt

# 列出文件
mc ls local/backups/
```

### 4. 在 backup-operator 中使用

在 BackupPolicy 中引用 MinIO：

```yaml
apiVersion: backup.backup.example.com/v1alpha1
kind: BackupPolicy
metadata:
  name: test-minio-backup
  namespace: default
spec:
  strategy: external
  schedule: "*/5 * * * *"  # 每 5 分钟

  selector:
    matchLabels:
      app: demo

  namespaces:
    - default

  destination:
    type: s3
    url: s3://backups/test
    credentialsSecret: minio-credentials

  retention:
    maxBackups: 5
    maxAge: "1h"
```

## 文件说明

### minio-deployment.yaml
MinIO 的完整部署配置：
- Namespace
- PersistentVolumeClaim (10Gi)
- Deployment (1 replica)
- Service (ClusterIP)
- Console Service (NodePort 30001)

### minio-credentials.yaml
MinIO 访问凭证 Secret，包含：
- `access-key`: minioadmin
- `secret-key`: minioadmin123
- `endpoint`: http://minio.minio.svc.cluster.local:9000
- `region`: us-east-1
- `restic-password`: backup-password-123

### create-buckets.yaml
创建测试 buckets 的 Job：
- backups
- test-bucket

### test-minio-connection.yaml
测试 MinIO S3 API 的 Job，验证：
- 连接性
- 文件上传
- 文件下载
- Bucket 列表

## 测试连接

```bash
# 运行连接测试
kubectl apply -f test/test-minio-connection.yaml

# 查看测试结果
kubectl logs job/test-minio-connection

# 清理测试 Job
kubectl delete job test-minio-connection
```

## 故障排查

### MinIO Pod 无法启动

```bash
# 查看 Pod 事件
kubectl describe pod -n minio -l app=minio

# 查看日志
kubectl logs -n minio -l app=minio

# 检查 PVC
kubectl get pvc -n minio
```

### 无法创建 Bucket

```bash
# 检查 MinIO 是否就绪
kubectl get pods -n minio

# 测试 API 连接
kubectl run -n minio test --rm -it --restart=Never --image=minio/mc -- \
  mc alias set test http://minio.minio.svc.cluster.local:9000 minioadmin minioadmin123
```

### 访问被拒绝

确认凭证正确：
```bash
kubectl get secret minio-credentials -o yaml
```

## 清理

```bash
# 删除所有 MinIO 资源
kubectl delete -f test/minio-deployment.yaml

# 删除 Secret
kubectl delete -f test/minio-credentials.yaml

# 删除测试 Jobs
kubectl delete job -n minio create-minio-buckets
kubectl delete job test-minio-connection
```

## 性能配置

当前配置为开发/测试环境，资源限制：
- Memory: 256Mi - 512Mi
- CPU: 100m - 500m
- Storage: 10Gi

生产环境建议调整：
```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "4Gi"
    cpu: "2000m"
```

## 安全建议

⚠️ **当前配置仅用于开发/测试！**

生产环境需要：
1. 修改默认密码
2. 启用 TLS/SSL
3. 配置 RBAC
4. 使用 Secret 管理凭证（不要硬编码）
5. 启用审计日志
6. 配置网络策略

## 扩展阅读

- [MinIO Documentation](https://min.io/docs/)
- [MinIO S3 API Compatibility](https://min.io/docs/minio/linux/developers/s3-api-compatibility.html)
- [MinIO Client Guide](https://min.io/docs/minio/linux/reference/minio-mc.html)
