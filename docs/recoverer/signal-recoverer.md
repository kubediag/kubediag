# Signal Recoverer

Signal Recoverer 是一个 Kube Diagnoser 内置的故障恢复器，用于向节点上的进程发送信号。

## 如何部署

运行以下命令注册 Signal Recoverer 故障恢复器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/deploy/signal_recoverer.yaml
```

## 请求类型

Signal Recoverer 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/recoverer/signalrecoverer`。Signal Recoverer 可以对 POST 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Signal Recoverer 从 Abnormal 的 `.spec.context.signalRecovery` 字段获取需要发送信号的进程列表并发送指定的信号。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.assignedRecoverers` 字段中包含 `signal-recoverer` 来使用 Signal Recoverer 恢复故障。一个指定 Signal Recoverer 恢复故障的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: signal-recovery
  namespace: default
spec:
  assignedRecoverers:
  - name: signal-recoverer
    namespace: kube-diagnoser
  context:
    signalRecovery:
    - pid: 10842
      signal: 9
  nodeName: 10.177.16.22
  skipDiagnosis: true
  skipInformationCollection: true
  skipRecovery: false
  source: Custom
```
