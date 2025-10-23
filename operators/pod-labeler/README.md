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

## 计划功能

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

适合学习 Kubernetes 中级开发概念的项目。