# Pod Disk Usage Diagnoser

Pod Disk Usage Diagnoser 是一个 Kube Diagnoser 内置的故障分析器，用于分析节点上的 Pod 磁盘使用量。

## 如何部署

运行以下命令注册 Pod Disk Usage Diagnoser 故障分析器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/deploy/pod_collector.yaml
```

## 请求类型

Pod Disk Usage Diagnoser 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/informationcollector/podcollector`。Pod Disk Usage Diagnoser 可以对 POST 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Pod Disk Usage Diagnoser 从 Abnormal 的 `.status.context.podInformation` 字段获取节点 Pod 列表并将节点上磁盘使用量过高 Pod 列表记录到 Abnormal 的 `.status.context.podDiskUsageDiagnosis` 字段。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.assignedInformationCollectors` 字段中包含 `pod-collector` 来使用 Pod Disk Usage Diagnoser 采集信息。Pod Disk Usage Diagnoser 会从 Abnormal 的 `.status.context.podInformation` 字段获取节点 Pod 列表，因此 Pod Disk Usage Diagnoser 通常与 Pod Collector 配合使用。一个指定 Pod Disk Usage Diagnoser 采集信息的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: pod-disk-usage-diagnosis
  namespace: default
spec:
  assignedDiagnosers:
  - name: pod-disk-usage-diagnoser
    namespace: kube-diagnoser
  assignedInformationCollectors:
  - name: pod-collector
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  skipDiagnosis: false
  skipInformationCollection: false
  skipRecovery: true
  source: Custom
```

Pod Disk Usage Diagnoser 成功分析故障后会将节点上磁盘使用量过高 Pod 列表记录到 `.status.context.podDiskUsageDiagnosis` 字段：

```yaml
status:
  context:
    podDiskUsageDiagnosis:
    - diskUsage: 77824
      metadata:
        creationTimestamp: "2020-07-15T03:35:24Z"
        generateName: kube-flannel-ds-amd64-
        labels:
          app: flannel
          controller-revision-hash: 7f489b5c67
          pod-template-generation: "1"
          tier: node
        name: kube-flannel-ds-amd64-qdjtb
        namespace: kube-system
        ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: DaemonSet
          name: kube-flannel-ds-amd64
          uid: 94094c6d-4779-4172-9d39-dce884184fef
        resourceVersion: "1762390"
        selfLink: /api/v1/namespaces/kube-system/pods/kube-flannel-ds-amd64-qdjtb
        uid: 2bfed5a3-67fd-4721-99ae-6d584150c891
      path: /var/lib/kubelet/pods/2bfed5a3-67fd-4721-99ae-6d584150c891
```
