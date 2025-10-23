# MinIO 快速参考

## 🎯 部署摘要

✅ **MinIO 已成功部署到本地集群！**

```
Status:     Running
Namespace:  minio
Endpoint:   http://minio.minio.svc.cluster.local:9000
Console:    http://localhost:9001 (通过 port-forward)
Storage:    10Gi PVC
```

## 🔑 访问信息

```bash
Access Key:  minioadmin
Secret Key:  minioadmin123
```

## 🪣 可用 Buckets

- `backups` - 用于 backup-operator
- `test-bucket` - 测试用

## 📝 常用命令

### 查看状态
```bash
kubectl get pods -n minio
kubectl get svc -n minio
```

### 访问 Web Console
```bash
kubectl port-forward -n minio svc/minio-console 9001:9001
# 浏览器打开: http://localhost:9001
```

### 使用 mc 命令行
```bash
kubectl exec -it -n minio deployment/minio -- sh
mc alias set local http://localhost:9000 minioadmin minioadmin123
mc ls local/
```

### 测试连接
```bash
kubectl apply -f test/test-minio-connection.yaml
kubectl logs job/test-minio-connection
```

## 🚀 使用示例

### 创建 BackupPolicy

```yaml
apiVersion: backup.backup.example.com/v1alpha1
kind: BackupPolicy
metadata:
  name: minio-backup
spec:
  strategy: external
  schedule: "*/10 * * * *"
  selector:
    matchLabels:
      app: demo
  destination:
    type: s3
    url: s3://backups/my-app
    credentialsSecret: minio-credentials
  retention:
    maxBackups: 10
    maxAge: "24h"
```

### 应用配置
```bash
kubectl apply -f your-backuppolicy.yaml
kubectl get backuppolicies
kubectl describe backuppolicy minio-backup
```

## 🔧 故障排查

### Pod 无法启动
```bash
kubectl describe pod -n minio -l app=minio
kubectl logs -n minio deployment/minio
```

### 连接超时
```bash
# 检查 Service
kubectl get svc -n minio

# 测试内部连接
kubectl run test --rm -it --image=busybox -- \
  wget -O- http://minio.minio.svc.cluster.local:9000/minio/health/live
```

### Secret 问题
```bash
kubectl get secret minio-credentials -o yaml
kubectl describe secret minio-credentials
```

## 🧹 清理

```bash
# 删除 MinIO
kubectl delete -f test/minio-deployment.yaml

# 删除凭证
kubectl delete secret minio-credentials

# 删除测试资源
kubectl delete job -n minio --all
```

## 📚 下一步

1. 部署 backup-operator CRD
   ```bash
   cd operators/backup-operator
   make install
   ```

2. 运行 operator
   ```bash
   make run
   ```

3. 创建测试 PVC
   ```bash
   kubectl apply -f examples/test-pvc.yaml
   ```

4. 创建 BackupPolicy
   ```bash
   kubectl apply -f config/samples/backup_v1alpha1_backuppolicy.yaml
   ```

5. 验证备份
   ```bash
   kubectl get backuppolicies
   kubectl describe backuppolicy <name>
   ```

## 🌐 访问 MinIO Console

### 方法 1: Port Forward
```bash
kubectl port-forward -n minio svc/minio-console 9001:9001
```
访问: http://localhost:9001

### 方法 2: NodePort (如果使用 Minikube)
```bash
minikube service minio-console -n minio --url
```

登录凭证:
- Username: `minioadmin`
- Password: `minioadmin123`
