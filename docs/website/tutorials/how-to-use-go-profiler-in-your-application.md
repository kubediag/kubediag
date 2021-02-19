# 剖析 Go 应用的性能

本教程介绍如何修改 Go 应用以捕获性能剖析数据，并查看剖析数据。

## 教程目标

阅读本教程后，你将熟悉以下内容：

* 如何创建 Go Profiler 剖析你的应用的性能
* 怎样查看性能剖析结果

## 创建 Go Profiler

假如你已经有一个 HTTP Server 的 Go 应用，你可以在代码中加入如下引用。

```go
import _ "net/http/pprof"
```

如果你想使用一个全新的 Go 应用，也可以复制如下代码生成一个：

```go
package main

import (
    "net/http"
    _ "net/http/pprof"
)

func main() {
    http.ListenAndServe(":9090", nil)
}
```

运行这个应用，然后使用如下示例创建一个 Diagnosis 进行性能剖析。注意创建前修改你的 `<source-ip>` 与 `<node-name>`，其中 `<source-ip>` 是你的 Go 应用访问 IP，`<node-name>` 是运行了 Kube-diagnoser Agent的节点 Name。

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Diagnosis
metadata:
  name: go-profiler
spec:
  source: Custom
  profilers:
  - name: go-profiler
    type: InformationCollector
    go:
      source: <source-ip>:9090
      type: Heap
    timeoutSeconds: 60
    expirationSeconds: 300
  nodeName: <node-name>
```

查看 Diagnosis 的状态，性能剖析的执行结果被同步到 `.status.profilers` 字段：

```yaml
status:
  profilers:
  - endpoint: 10.0.2.15:45778
    name: go-profiler
    type: InformationCollector
  recoverable: true
  startTime: "2021-02-05T12:23:02Z"
```

## 查看性能剖析结果

在浏览器中打开 `10.0.2.15:45778`，显示 Profiler 界面，即可查看此应用的堆分析结果与火焰图等。
