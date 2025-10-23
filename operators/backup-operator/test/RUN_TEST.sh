#!/bin/bash
# Backup Operator 测试脚本

set -e

echo "=================================================="
echo "Backup Operator 测试环境验证"
echo "=================================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查函数
check_resource() {
    local resource=$1
    local name=$2
    local namespace=$3

    if [ -n "$namespace" ]; then
        if kubectl get $resource $name -n $namespace &> /dev/null; then
            echo -e "${GREEN}✅${NC} $resource/$name (namespace: $namespace)"
            return 0
        else
            echo -e "${RED}❌${NC} $resource/$name (namespace: $namespace)"
            return 1
        fi
    else
        if kubectl get $resource $name &> /dev/null; then
            echo -e "${GREEN}✅${NC} $resource/$name"
            return 0
        else
            echo -e "${RED}❌${NC} $resource/$name"
            return 1
        fi
    fi
}

echo "📋 检查必需资源..."
echo ""

# 检查 MinIO
echo "1. MinIO 对象存储:"
check_resource pod minio-df4b9756b-6wdpd minio || true
check_resource svc minio minio || true
check_resource secret minio-credentials default || true

# 检查测试 PVC
echo ""
echo "2. 测试 PVC:"
check_resource pvc test-backup-pvc default || true
check_resource pod test-data-writer default || true

# 检查 CRD
echo ""
echo "3. BackupPolicy CRD:"
check_resource crd backuppolicies.backup.backup.example.com "" || true

# 检查 BackupPolicy
echo ""
echo "4. BackupPolicy 实例:"
check_resource backuppolicy test-minio-backup default || true

echo ""
echo "=================================================="
echo "MinIO 连接测试"
echo "=================================================="
echo ""

kubectl run minio-test --rm -i --restart=Never --image=minio/mc -- sh -c "
    mc alias set test http://minio.minio.svc.cluster.local:9000 minioadmin minioadmin123 && \
    mc ls test/backups/ && \
    echo '✅ MinIO 连接正常，backups bucket 可访问'
" 2>/dev/null || echo -e "${YELLOW}⚠️${NC}  MinIO 连接测试失败（可能是首次运行，bucket 为空）"

echo ""
echo "=================================================="
echo "BackupPolicy 状态"
echo "=================================================="
echo ""

kubectl get backuppolicies test-minio-backup -o wide 2>/dev/null || echo -e "${RED}❌${NC} BackupPolicy 未创建"

echo ""
echo "=================================================="
echo "测试 PVC 数据"
echo "=================================================="
echo ""

echo "PVC 中的文件:"
kubectl exec test-data-writer -- ls -lh /data/ 2>/dev/null || echo -e "${RED}❌${NC} 无法访问 test-data-writer Pod"

echo ""
echo "=================================================="
echo "下一步操作"
echo "=================================================="
echo ""

echo "运行 operator:"
echo -e "${YELLOW}  cd /home/rayhe/github/operator-example/operators/backup-operator${NC}"
echo -e "${YELLOW}  make run${NC}"
echo ""

echo "在另一个终端观察变化:"
echo -e "${YELLOW}  watch kubectl get backuppolicies,jobs,pods${NC}"
echo ""

echo "查看 operator 日志（如果已运行）:"
echo -e "${YELLOW}  # operator 控制台输出${NC}"
echo ""

echo "查看 BackupPolicy 详情:"
echo -e "${YELLOW}  kubectl describe backuppolicy test-minio-backup${NC}"
echo ""

echo "查看备份 Job（如果已创建）:"
echo -e "${YELLOW}  kubectl get jobs -l backup.backup.example.com/policy=test-minio-backup${NC}"
echo -e "${YELLOW}  kubectl logs job/<job-name>${NC}"
echo ""

echo "访问 MinIO Console:"
echo -e "${YELLOW}  kubectl port-forward -n minio svc/minio-console 9001:9001${NC}"
echo -e "${YELLOW}  # 浏览器打开: http://localhost:9001${NC}"
echo -e "${YELLOW}  # 登录: minioadmin / minioadmin123${NC}"
echo ""

echo "=================================================="
echo "测试准备完成！"
echo "=================================================="
