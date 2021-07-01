# KubeDiag

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## KubeDiag 是什么

KubeDiag 是一个用于 [Kubernetes](https://kubernetes.io) 集群故障发现、诊断以及恢复的框架，集群的每个节点上运行着 KubeDiag 的 Agent 组件来处理故障事件。Abnormal 自定义资源是对故障事件的抽象。通过创建 Abnormal 自定义资源，用户可以启动对已知的故障事件自动化诊断恢复的流水线。KubeDiag 维护了故障诊断过程中的状态机，用户通过查看 Status 字段可以获取诊断结果。

## 部署 KubeDiag

KubeDiag 包括 Master 和 Agent 组件，Master 在集群中以 [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) 的方式部署，Agent 在集群中以 [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) 的方式部署。KubeDiag 要求 Kubernetes 集群版本不低于 `1.16`。

KubeDiag Master 建议使用 [Cert Manager](https://github.com/jetstack/cert-manager) 管理 Webhook Server 的证书。如果集群中未部署 Cert Manager 可参考[官方文档](https://cert-manager.io/docs/installation/kubernetes/)进行安装，运行以下命令进行快速安装：

```bash
# Kubernetes 1.16+
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.2/cert-manager.yaml
```

使用 `kubectl` 命令行工具进行部署：

```bash
kubectl create namespace kubediag
kubectl apply -f config/deploy
```

查看是否所有运行 KubeDiag 的 Pod 处于 Running 状态：

```bash
kubectl get -n kubediag pod -o wide
```

在开发环境进行调试时，如果安装了 [`kustomize`](https://github.com/kubernetes-sigs/kustomize) 工具，运行以下命令可以将当前分支上的代码打包成镜像并进行部署：

```bash
make docker-build
make deploy
```

## 兼容性

下列是当前维护的 KubeDiag 版本以及其确认支持的 Kubernetes 版本，不在确认支持列表中的 Kubernetes 版本也可能正常工作：

| KubeDiag | Kubernetes 1.16 | Kubernetes 1.17 |
|-|-|-|
| [`release-0.1`](https://github.com/kubediag/kubediag/tree/release-0.1) | Yes | Yes |

## 贡献代码

我们欢迎任何形式的帮助，包括但不限定于完善文档、提出问题、修复 Bug 和增加特性。详情可参考[文档](./CONTRIBUTING.md)。
