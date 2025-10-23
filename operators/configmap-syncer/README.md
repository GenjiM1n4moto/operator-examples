# ConfigMap-Syncer Operator

## 项目简介

ConfigMap-Syncer 是一个 Kubernetes Operator，用于在多个命名空间之间同步 ConfigMap 资源。这是一个教学项目，旨在帮助学习 Kubernetes Controller 和 Operator 的开发。

## 功能特性

- **ConfigMap 同步**: 将源 ConfigMap 同步到指定的目标命名空间
- **选择器支持**: 支持通过标签选择器自动发现目标命名空间
- **状态追踪**: 提供详细的同步状态和条件信息
- **高可用**: 支持 Leader Election，可运行多副本

## 快速开始

### 开发模式

```bash
# 安装 CRD
make install

# 运行 Controller（本地开发）
make run
```

### 生产部署

```bash
# 构建镜像
make docker-build IMG=configmap-syncer:v1.0.0

# 部署到集群
make deploy IMG=configmap-syncer:v1.0.0
```

## 使用示例

### 基本用法

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

### 使用选择器

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

## 项目结构

```
operators/configmap-syncer/
├── api/v1/                 # CRD 定义
├── internal/controller/    # Controller 逻辑
├── config/                 # Kubernetes 配置
├── cmd/                    # 程序入口
├── hack/                   # 构建脚本
├── test/                   # 测试文件
├── Makefile               # 构建命令
├── Dockerfile             # 容器构建
└── README.md              # 项目文档
```

## 学习要点

这个项目涵盖了以下 Kubernetes 开发概念：

1. **CRD 设计**: 自定义资源定义
2. **Controller Pattern**: 控制器模式和 Reconcile 循环
3. **Watch 机制**: 资源变化监听
4. **RBAC**: 权限管理
5. **Status 管理**: 状态和条件更新
6. **Error Handling**: 错误处理和重试机制

## 构建和测试

```bash
# 安装依赖
go mod tidy

# 运行测试
make test

# 生成代码
make generate

# 更新 CRD
make manifests

# 构建二进制
make build
```

## 清理

```bash
# 停止开发模式
# Ctrl+C 停止 make run

# 清理生产部署
make undeploy

# 删除 CRD
make uninstall
```

## 复杂度等级

⭐⭐☆☆☆ (初级)

适合作为 Kubernetes Operator 开发的入门项目。