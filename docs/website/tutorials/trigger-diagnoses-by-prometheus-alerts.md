# 通过 Prometheus 报警触发诊断

本文介绍了如何通过 Prometheus 报警创建 Diagnosis 来触发诊断。

## 开始之前

在教程开始前，您需要确定 Kubernetes 集群中已经正确安装 Kube Diagnoser。

## 将 Kube Diagnoser Master 注册到 Prometheus 的 Alertmanager 列表

通过配置 Prometheus 可以将 Kube Diagnoser Master 注册到 Prometheus 的 Alertmanager 列表。Kube Diagnoser Master 在接收到报警后会匹配报警消息和 Trigger 中定义的模板，如果匹配成功则根据报警消息中的数据创建 Diagnosis 对象并触发诊断工作流。Prometheus 配置项详情可参考[官方文档](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)。

[Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) 为用户提供了在 Kubernetes 集群中对 Prometheus 以及其相关监控组件的管理能力。通过配置 [Prometheus](https://github.com/prometheus-operator/prometheus-operator/blob/master/Documentation/api.md#prometheus) 自定义资源中的 [`.spec.alerting`](https://github.com/prometheus-operator/prometheus-operator/blob/master/Documentation/api.md#alertingspec) 字段可以将 Kube Diagnoser Master 注册到 Prometheus 的 Alertmanager 列表，部分配置如下所示：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
# ......
spec:
  # ......
  alerting:
    alertmanagers:
    # ......
    - name: kube-diagnoser-master
      namespace: kube-diagnoser
      port: http
```

如果您没有使用 Prometheus Operator 来管理 Prometheus，还可以通过直接修改配置文件的方式注册 Kube Diagnoser Master，部分配置如下所示：

```yaml
alerting:
  # ......
  alertmanagers:
  # ......
  - kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
        - kube-diagnoser
    scheme: http
    path_prefix: /
    timeout: 10s
    api_version: v1
    relabel_configs:
    - source_labels: [__meta_kubernetes_service_name]
      separator: ;
      regex: kube-diagnoser-master
      replacement: $1
      action: keep
    - source_labels: [__meta_kubernetes_endpoint_port_name]
      separator: ;
      regex: http
      replacement: $1
      action: keep
```

## 为 Prometheus 报警项创建 Trigger 对象

您可以通过 Trigger 对象来定义 Kube Diagnoser 根据接收到的报警信息自动创建 Diagnosis 的方式。下列 Trigger 在 NodeClockNotSynchronising 报警触发时创建 Diagnosis 来触发诊断流程：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Trigger
metadata:
  name: node-clock-not-synchronising
spec:
  operationSet: node-clock-debugger
  sourceTemplate:
    prometheusAlertTemplate:
      regexp:
        alertName: NodeClockNotSynchronising
      nodeNameReferenceLabel: instance
      podNameReferenceLabel: pod
      podNamespaceReferenceLabel: namespace
      containerReferenceLabel: container
      parameterInjectionLabels:
      - endpoint
      - service
      - severity
```

NodeClockNotSynchronising 报警消息包含下列标签：

```
alertname="NodeClockNotSynchronising"
container="kube-rbac-proxy"
endpoint="https"
instance="my-node"
job="node-exporter"
namespace="monitoring"
pod="node-exporter-2vqfp"
service="node-exporter"
severity="warning"
```

Kube Diagnoser 接收到 NodeClockNotSynchronising 报警消息时会创建下列 Diagnosis：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Diagnosis
metadata:
  labels:
    adjacency-list-hash: 57db4479b7
  name: prometheus-alert.nodeclocknotsynchronising.94df165
  namespace: kube-diagnoser
spec:
  nodeName: my-node
  operationSet: node-clock-debugger
  parameters:
    endpoint: https
    service: node-exporter
    severity: warning
  podReference:
    container: kube-rbac-proxy
    name: node-exporter-2vqfp
    namespace: monitoring
```

在该示例中，Trigger 与创建出的 Diagnosis 中各字段对应关系如下：

* Trigger 的 `.spec.sourceTemplate.prometheusAlertTemplate.regexp.alertName` 用于匹配 Prometheus 报警名，该字段是一个 [RE2](https://github.com/google/re2/wiki/Syntax) 正则表达式，如果匹配成功则基于该报警消息创建 Diagnosis。
* Trigger 的 `.spec.operationSet` 与 Diagnosis 的 `.spec.operationSet` 相同。
* Trigger 的 `.spec.sourceTemplate.prometheusAlertTemplate.nodeNameReferenceLabel` 为 `instance`，而 NodeClockNotSynchronising 报警消息中包含 `instance="my-node"` 标签，该标签的值 `my-node` 与 Diagnosis 的 `.spec.nodeName` 一致。
* Trigger 的 `.spec.sourceTemplate.prometheusAlertTemplate.podNameReferenceLabel` 为 `pod`，而 NodeClockNotSynchronising 报警消息中包含 `pod="node-exporter-2vqfp"` 标签，该标签的值 `node-exporter-2vqfp` 与 Diagnosis 的 `.spec.podReference.name` 一致。
* Trigger 的 `.spec.sourceTemplate.prometheusAlertTemplate.podNamespaceReferenceLabel` 为 `namespace`，而 NodeClockNotSynchronising 报警消息中包含 `namespace="monitoring"` 标签，该标签的值 `monitoring` 与 Diagnosis 的 `.spec.podReference.namespace` 一致。
* Trigger 的 `.spec.sourceTemplate.prometheusAlertTemplate.containerReferenceLabel` 为 `container`，而 NodeClockNotSynchronising 报警消息中包含 `pod="node-exporter-2vqfp"` 标签，该标签的值 `node-exporter-2vqfp` 与 Diagnosis 的 `.spec.podReference.container` 一致。
* Trigger 的 `.spec.sourceTemplate.prometheusAlertTemplate.parameterInjectionLabels` 是一个列表，该列表包含了需要注入 Diagnosis 中报警标签的键并且与 Diagnosis 的 `.spec.parameters` 一致。
