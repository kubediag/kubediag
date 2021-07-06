# KubeDiag

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## KubeDiag 是什么

KubeDiag 为 Kubernetes 集群中的诊断运维管理提供了一套统一的编排框架。用户通过 Kubernetes 自定义资源可以定义运维操作、如何执行复杂的诊断运维流水线、如何通过报警自动触发诊断运维流水线。该系统通过下列自定义资源为用户提供了运维操作的自动化管理能力：

* Operation 用于定义故障运维和集群检查等操作。
* OperationSet 用于定义诊断运维流水线。
* Trigger 支持用户通过 Prometheus、Kafka 等系统自动触发诊断运维流水线。
* Diagnosis 中记录了一次诊断运维流水线的结果和状况。

## 先决条件

用于安装 KubeDiag 的集群版本需要满足以下条件：

* [Kubernetes](https://github.com/kubernetes/kubernetes) 1.16+

如果您使用 Helm 来进行安装，那么 Helm 的版本需要满足下列条件：

* [Helm](https://github.com/helm/helm) 3.0+

## 安装 KubeDiag

KubeDiag 包括 Master 和 Agent 组件，Master 在集群中以 [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) 的方式安装，Agent 在集群中以 [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) 的方式安装。

KubeDiag Master 建议使用 [Cert Manager](https://github.com/jetstack/cert-manager) 管理 Webhook Server 的证书。如果集群中未安装 Cert Manager 可参考[官方文档](https://cert-manager.io/docs/installation/kubernetes/)进行安装，运行以下命令进行快速安装：

```bash
# Kubernetes 1.16+
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.2/cert-manager.yaml
```

使用 `kubectl` 命令行工具进行安装：

```bash
kubectl create namespace kubediag
kubectl apply -f config/deploy
```

查看是否所有运行 KubeDiag 的 Pod 处于 Running 状态：

```bash
kubectl get -n kubediag pod -o wide
```

您也可以使用 Helm 进行安装，执行下列命令安装 KubeDiag：

```bash
helm repo add kubediag https://kubediag.github.io/kubediag-helm
helm repo update
helm install kubediag/kubediag-helm --create-namespace --generate-name --namespace kubediag
```

在开发环境进行调试时，如果安装了 [`kustomize`](https://github.com/kubernetes-sigs/kustomize) 工具，运行以下命令可以将当前分支上的代码打包成镜像并进行安装：

```bash
make docker-build
make deploy
```

## 兼容性

下列是当前维护的 KubeDiag 版本以及其确认支持的 Kubernetes 版本，不在确认支持列表中的 Kubernetes 版本也可能正常工作：

| KubeDiag | Kubernetes 1.16 | Kubernetes 1.17 | Kubernetes 1.18 | Kubernetes 1.19 |
|-|-|-|-|-|
| [`release-0.1`](https://github.com/kubediag/kubediag/tree/release-0.1) | Yes | Yes | | |
| [`release-0.2`](https://github.com/kubediag/kubediag/tree/release-0.2) | Yes | Yes | Yes | Yes |

## 贡献代码

我们欢迎任何形式的帮助，包括但不限定于完善文档、提出问题、修复 Bug 和增加特性。详情可参考[文档](./CONTRIBUTING.md)。

## 行为准则

您在参与本项目的过程中须遵守 [CNCF 行为准则](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)。
