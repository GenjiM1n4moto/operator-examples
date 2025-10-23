# Pod-Labeler Operator

## 项目简介

Pod-Labeler 是一个 Kubernetes Operator，用于根据预定义规则自动为 Pod 添加标签。这是一个中级难度的学习项目，展示了多资源监听和 Finalizer 的使用。

## 学习目标

- **多资源监听**: 同时监听 Pod 和 Namespace 资源变化
- **Finalizer 机制**: 确保资源清理的完整性
- **标签管理**: 动态标签分配和更新
- **生命周期控制**: Pod 创建、更新、删除的全生命周期管理

## 功能特性

- ✅ 根据命名空间标签自动为 Pod 添加标签
- ✅ 支持基于 Pod 属性的条件标签
- ✅ 提供标签清理机制
- ✅ 支持标签模板和变量替换

## 复杂度等级

⭐⭐⭐☆☆ (中级)

## 状态

📋 待开发

## operator-sdk 初始化命令

### 1. 初始化项目
```bash
cd operators/pod-labeler
operator-sdk init --domain example.com --repo github.com/rayhe/operator-example/operators/pod-labeler
```

### 2. 创建 API 和 Controller
```bash
operator-sdk create api --group labels --version v1 --kind PodLabeler --resource --controller
```

## 执行 operator-sdk 命令后的 TODO

### 1. 定义 CRD（Custom Resource Definition）
- [ ] 编辑 `api/v1/podlabeler_types.go`
- [ ] 定义 Spec 字段：
  - selector: 选择要标记的 Pod
  - labelRules: 标签规则列表（key、value、valueFrom）
  - conditions: 条件标签规则
- [ ] 定义 Status 字段：
  - labeledPodsCount: 已标记的 Pod 数量
  - lastSyncTime: 最后同步时间
  - conditions: 状态条件
- [ ] 运行 `make generate` 更新生成的代码
- [ ] 运行 `make manifests` 生成 CRD manifests

### 2. 实现 Controller 逻辑
- [ ] 编辑 `internal/controller/podlabeler_controller.go`
- [ ] 实现 Reconcile 循环：
  - 监听 PodLabeler CR 变化
  - 根据 selector 列出匹配的 Pod
  - 解析 labelRules 和 conditions
  - 应用标签到匹配的 Pod
  - 更新 CR status
- [ ] 添加 Finalizer 处理逻辑（清理标签）
- [ ] 设置 Watch 监听 Pod 资源变化

### 3. 配置 RBAC
- [ ] 检查生成的 RBAC 配置 `config/rbac/role.yaml`
- [ ] 确保 controller 有以下权限：
  - List/Watch/Get Pods
  - Update Pod labels
  - Get/Update PodLabeler CRs 及其 status

### 4. 编写测试
- [ ] 在 `internal/controller/podlabeler_controller_test.go` 编写单元测试
- [ ] 在 `test/e2e/e2e_test.go` 编写 e2e 测试
- [ ] 测试边界情况（Pod 不存在、权限错误等）

### 5. 创建示例资源
- [ ] 在 `config/samples/` 创建示例 PodLabeler CR
- [ ] 创建测试用的 Pod manifests
- [ ] 参考项目简介中的 YAML 示例

### 6. 构建和部署
- [ ] 构建镜像: `make docker-build IMG=<registry>/pod-labeler:tag`
- [ ] 推送镜像: `make docker-push IMG=<registry>/pod-labeler:tag`
- [ ] 安装 CRD: `make install`
- [ ] 部署 operator: `make deploy IMG=<registry>/pod-labeler:tag`

### 7. 验证和调试
- [ ] 创建测试 Pod 和 PodLabeler CR
- [ ] 验证标签是否正确应用
- [ ] 检查 operator 日志
- [ ] 测试 Finalizer 清理逻辑

## 常用开发命令

```bash
# 生成代码（修改 types 后）
make generate

# 生成 manifests（CRDs、RBAC 等）
make manifests

# 运行测试
make test

# 本地运行（不部署到集群）
make run

# 安装 CRD 到集群
make install

# 构建 Docker 镜像
make docker-build IMG=<your-registry>/pod-labeler:tag

# 部署到集群
make deploy IMG=<your-registry>/pod-labeler:tag

# 卸载 CRD
make uninstall

# 取消部署 controller
make undeploy
```

## 计划功能示例

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

## 实现提示

1. **valueFrom 解析**: 需要实现从 namespace.labels 或 pod.labels 提取值的逻辑
2. **Condition 评估**: 实现字段值检查（NotEmpty、Equals 等操作符）
3. **Finalizer**: 在删除 PodLabeler CR 时清理已添加的标签
4. **Watch 设置**: 使用 `Owns()` 或 `Watches()` 监听 Pod 变化

适合学习 Kubernetes 中级开发概念的项目。