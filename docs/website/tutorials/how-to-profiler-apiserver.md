# 如何使用 Go Profiler 剖析 APIServer

在本教程中，你将部署一个配置为收集性能剖析文件数据的 Go 应用剖析集群的 APIServer，并将使用浏览器界面查看性能剖析文件数据。

## 准备工作

在教程开始前，您需要确定 kubernetes 集群中已经正确安装 KubeDiag，并且确认有以下资源

* ServiceAccount：[apiserver-profiler-sa](../../config/rbac/apiserver_profiler_viewer_serviceaccount.yaml)
* ClusterRole：[apiserver-profiler-role](../../config/rbac/apiserver_profiler_viewer_clusterrole.yaml)
* ClusterRoleBinding：[apiserver-profiler-rolebinding](../../config/rbac/apiserver_profiler_viewer_binding.yaml)

## 教程目标

阅读本教程后，你将熟悉以下内容：

* 如何创建 Go Profiler 剖析集群的 APIServer 的性能
* 怎样查看性能剖析结果

## 创建 Go Profiler

获取由 `apiserver-profiler-sa` ServiceAccount 生成的 Secret 资源，用以下命令获取 Secret 的名字：

```bash
kubectl get secret -n kubediag | grep apiserver-profiler-sa-token | awk '{print $1}'
```

使用如下示例创建一个 Go Profiler Diagnosis。注意替换其中的 `<secret-name>`、`<node-name>`、`<apiserver-url>` 地址，其中 `<secret-name>` 是由 `apiserver-profiler-sa` ServiceAccount 生成的 Secret 对象的名字，`<node-name>` 是 KubeDiag Agent 所运行的节点名字，`<apiserver-url>` 是当前集群的 APIServer URL。

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: go-profiler
spec:
  source: Custom
  profilers:
  - name: go-profiler
    type: InformationCollector
    go:
      source: <apiserver-url>
      type: Heap
      tls:
        secretReference:
          name: <secret-name>
          namespace: kubediag
    timeoutSeconds: 60
    expirationSeconds: 300
  nodeName: <node-name>
```

下载上面的例子并保存为文件 `apiserver-profiler.yaml`。

使用 `kubectl apply` 来创建这个Go Profiler Diagnosis。

```bash
kubectl apply -f apiserver-profiler.yaml
```

上面这个示例中创建了一个 Type 为 `InformationCollector` 的 Go Profiler Diagnosis，它将被 `<node-name>` 上运行的 KubeDiag Agent 监测到并在诊断器的 `InformationCollector` 阶段进行同步。`tls.secretReference` 中指定的 Secret 将被用于获取 `token` 和 `ca.crt` 进行连接到 `<apiserver-url>` 的认证。Agent 在认证通过后下载 Go Profiler 性能剖析文件于本地保存。这里性能剖析的 Type 是 `Heap`，代表 APIServer 的堆信息将在此刻收集。

查看 Diagnosis 的状态，性能剖析的执行结果被同步到 `.status.profilers` 字段：

```yaml
status:
  profilers:
  - endpoint: 10.0.2.15:45771
    name: go-profiler
    type: InformationCollector
  recoverable: true
  startTime: "2021-02-03T07:23:02Z"
```

## 查看性能剖析结果

在浏览器中打开 `10.0.2.15:45771`，显示 Profiler 界面，即可查看 APIServer 的堆分析结果与火焰图等。
