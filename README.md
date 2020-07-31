# Kube Diagnoser

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Kube Diagnoser 是什么

Kube Diagnoser 是一个用于 [Kubernetes](https://kubernetes.io) 集群故障发现、诊断以及恢复的框架，集群的每个节点上运行着 Kube Diagnoser 的 Agent 组件来处理故障事件。Abnormal 自定义资源是对故障事件的抽象。通过创建 Abnormal 自定义资源，用户可以启动对已知的故障事件自动化诊断恢复的流水线。Kube Diagnoser 维护了故障诊断过程中的状态机，用户通过查看 Status 字段可以获取诊断结果。一次成功的故障诊断通常由以下几个组件完成，每个组件分别对应了故障诊断过程中的状态：

* 故障事件源：产生故障事件，故障通常由 Event、日志或者用户自定义逻辑生成。
* 信息采集器：采集诊断需要的信息，也可以用于监控功能增强（如应用程序性能剖析）。
* 故障分析器：对故障进行分析并标记是否被成功识别。
* 故障恢复器：对被成功识别的故障进行恢复。

## 部署 Kube Diagnoser

Kube Diagnoser 在集群中以 [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) 的方式部署，Kube Diagnoser 要求 Kubernetes 集群版本不低于 `1.15`。

如果安装了 [`kustomize`](https://github.com/kubernetes-sigs/kustomize) 工具，运行以下命令进行部署：

```bash
make deploy
```

使用 `kubectl` 命令行工具进行部署：

```bash
kubectl create namespace kube-diagnoser
kubectl apply -f config/rbac
kubectl apply -f config/kube-diagnoser
```

查看是否所有运行 Kube Diagnoser 的 Pod 处于 Running 状态：

```bash
kubectl get -n kube-diagnoser pod -o wide
```

通过访问节点的 10357 端口可以获取 Prometheus 监控数据：

```bash
curl 0.0.0.0:10357/metrics
```

## 通过 Abnormal 触发故障诊断流程

Abnormal 自定义资源是对故障事件的抽象。通过创建 Abnormal 可以启动对已知的故障事件自动化诊断恢复的流水线，一个查看某个节点上磁盘占用量过高 Pod 列表的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: pod-disk-usage
  namespace: default
spec:
  assignedDiagnosers:
  - name: pod-disk-usage-diagnoser
    namespace: kube-diagnoser
  assignedInformationCollectors:
  - name: pod-disk-usage-information-collector
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  skipDiagnosis: false
  skipInformationCollection: false
  skipRecovery: true
  source: Custom
status:
  conditions:
  - lastTransitionTime: "2020-07-30T06:51:27Z"
    status: "True"
    type: InformationCollected
  - lastTransitionTime: "2020-07-30T06:51:29Z"
    status: "True"
    type: Identified
  - lastTransitionTime: "2020-07-30T06:51:29Z"
    status: "True"
    type: Recovered
  diagnoser:
    name: kube-diagnoser
    namespace: pod-disk-usage-diagnoser
  identifiable: true
  phase: Succeeded
  recoverable: true
  recoverer:
    name: ""
    namespace: ""
  context:
  - name: cicd-jenkins-master-67f68bcd5c-wdlc4
    namespace: cicd
    usage: 21G
  - name: ingress-nginx-pw7wk
    namespace: kube-system
    usage: 12G
  - name: kibana-fcc7fc4cc-jbh27
    namespace: elasticsearch
    usage: 8G
  startTime: "2020-07-30T06:51:27Z"
```

该故障定义了一次对节点 `10.177.16.22` 的磁盘使用量诊断，整个处理流程如下：

* Kube Diagnoser Agent 向信息采集器 `pod-disk-usage-information-collector` 发送请求以获取 Pod 磁盘使用量详情。
* Kube Diagnoser Agent 向故障诊断器 `pod-disk-usage-diagnoser` 发送请求以获取 Pod 磁盘使用量诊断结果。故障诊断成功后 `.status.identifiable` 字段被设置为 `true`。
* 由于 `.spec.skipRecovery` 字段被设置为 `true`，自动恢复流程被跳过。
* 诊断成功结束后 `.status.phase` 字段被设置为 `Succeeded` 并且 `.status.context` 字段记录了磁盘使用量较多的 Pod 列表。

详细信息参考 [Abnormal API 设计](./docs/architecture/abnormal.md)。

## 路线图

Kube Diagnoser 的后续工作包括：

* 与 [Kuberhealthy](https://github.com/Comcast/kuberhealthy) 的集成。
* 支持更丰富的故障事件源，如 Elasticsearch、Prometheus 报警等。
* 支持 Golang、Java、Python 等语言的性能剖析。
* 易于集成的客户端开发库。
