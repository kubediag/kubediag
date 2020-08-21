# Process Collector

Process Collector 是一个 Kube Diagnoser 内置的信息采集器，用于收集节点上的进程信息。

## 如何部署

运行以下命令注册 Process Collector 信息采集器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/deploy/process_collector.yaml
```

## 请求类型

Process Collector 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/informationcollector/processcollector`。Process Collector 可以对 POST 和 GET 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Process Collector 获取节点进程列表并将进程列表记录到 Abnormal 的 `.status.context.processInformation` 字段。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 当接收到 GET 请求时，Process Collector 获取节点进程列表。处理成功返回节点进程列表，返回码为 `200`；处理失败则返回错误信息，返回码为 `500`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.assignedInformationCollectors` 字段中包含 `process-collector` 来使用 Process Collector 采集信息。一个指定 Process Collector 采集信息的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: process-collector
spec:
  assignedInformationCollectors:
  - name: process-collector
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  skipDiagnosis: true
  skipInformationCollection: false
  skipRecovery: true
  source: Custom
```

Process Collector 成功采集信息后会将节点进程列表记录到 `.status.context.processInformation` 字段：

```yaml
status:
  context:
    processInformation:
    - command:
      - /sbin/init
      - splash
      cpuPercent: 0.5935039694514427
      createTime: "2020-08-20T10:11:23+08:00"
      memoryInfo:
        data: 0
        hwm: 0
        locked: 0
        rss: 10039296
        stack: 0
        swap: 0
        vms: 231333888
      nice: 20
      pid: 1
      ppid: 0
      status: S
      tgid: 1
```
