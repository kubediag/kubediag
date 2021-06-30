# Go Profiler

Go Profiler 是一个 [Processor](../design/processor.md)，用户可以通过 Go Profiler 采集节点上的服务器性能信息。

## 背景

在诊断过程中，用户可能需要采集服务器的性能信息。通过引入 Go Profiler 可以满足该需求。

## 实现

Go Profiler 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Go Profiler，该 Operation 在 KubeDiag Agent 部署时已默认注册，执行下列命令可以查看已注册的 Go Profiler：

```bash
$ kubectl get operation go-profiler -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-04-14T08:28:12Z"
  generation: 1
  name: go-profiler
  resourceVersion: "3613010"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/go-profiler
  uid: 933bb82d-4b54-49fa-8035-7dafd2b2ffe5
spec:
  processor:
    path: /processor/goProfiler
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Go Profiler 处理的请求必须为 POST 类型，处理的 HTTP 请求中必须包含特定 JSON 格式请求体。

#### HTTP 请求

POST /processor/goProfiler

#### 请求体参数

```json
{
  "param.diagnoser.runtime.go_profiler.source": "https://10.0.2.15:6443",                // 指定要剖析的地址
  "param.diagnoser.runtime.go_profiler.type": "Heap",                                    // 指定要剖析的类型
  "param.diagnoser.runtime.go_profiler.tls.secret_reference.name": "apiserver-profiler-sa-token-gj9x8",
  "param.diagnoser.runtime.go_profiler.tls.secret_reference.namespace": "kubediag",
  "param.diagnoser.runtime.go_profiler.expiration_seconds": 300                           // 过期时间
}
```

请求体中的参数(为方便阅读此处省略了较长的前缀)：

- `source` 是 Go 语言性能剖析器源。通常是一个 HTTP 访问路径。
- `type` 表示 Go 语言性能剖析器的类型。支持 Profile、Heap、Goroutine 类型。
  - Profile：CPU 分析，按照一定的频率采集所监听的应用程序的 CPU 使用情况，可确定应用程序在主动消耗 CPU 周期时花费时间的位置。
  - Heap：内存分析，在应用程序堆栈分配时记录跟踪，用于监视当前和历史内存使用情况，检查内存泄漏情况。
  - Goroutine：Goroutine 分析，对所有当前 Goroutine 的堆栈跟踪。
- `tls` 表示连接到远程 HTTPS 服务器时要用到的 TLS 配置。对于非 HTTPS 类型的 `source` 不必填写此参数。
  - `secretReference` 是包含 Token 内容的 Secret 引用，用于连接远程 HTTPS 服务器。其下包含了 `namespace`  和 `name` 用于描述指定的 secret 对象
- `expirationSeconds` 是 Go Profiler 提供服务的有效时间，超时后 `OperationResults` 中的 Server 将不可访问。

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 400 | Bad Request |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体

HTTP 请求返回体格式为 map[string]string ，结果中包含 Go Profiler 提供的 HTTP 服务地址与此服务的过期时间：

```
Visit http://10.0.2.15:35869, this server will expire in 300 seconds.
```

这部分信息将会记录在 Diagnosis 对象的 status 中， 如下：

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: go-profiler
  namespace: default
spec:
    ...
status:
  phase: Succeeded
  ...
  operationResults:
    diagnoser.runtime.go_profiler.result.endpoint: '"Visit http://10.0.2.15:35869, this server will expire in 300 seconds."'
  ...
```

### 举例说明

1. 以访问 KubeDiag 自身的8090端口为例，创建 OperationSet 和 Diagnosis:

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: go-profiler
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: go-profiler
---
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: go-profiler
spec:
  parameters:
    1: |
      {
        "source": "http://127.0.0.1:8090",
        "type": "Heap",
        "expirationSeconds": 300
      }
  operationSet: go-profiler
  nodeName: shujiang-virtualbox
```

该 Diagnosis 定义了一个需要执行的 Go 语言内存性能剖析。Go 语言程序的性能剖析数据访问地址为 `http://127.0.0.1:8090`，性能剖析提供服务的有效时间为 300 秒。性能剖析的执行结果会被同步到 `operationResults` 中：

```yaml
status:
  operationResults:
    diagnoser.runtime.go_profiler.result.endpoint: '"Visit http://10.0.2.15:42359, this server will expire in 300 seconds."'
  phase: Succeeded
  startTime: "2021-04-16T06:12:53Z"
  succeededPath:
  - id: 1
    operation: go-profiler
```

在浏览器中打开 `http://10.0.2.15:42359`，显示 Profiler 界面，即可查看此应用的堆分析结果与火焰图等。
