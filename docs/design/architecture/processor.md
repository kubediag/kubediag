# Processor

Processor 是处理用户诊断请求的实体，通常通过 Operation 资源进行注册。

## 背景和动机

Kube Diagnoser 内置实现了部分收集信息或分析数据的操作。通过在 Kube Diagnoser 的代码中实现诊断操作可以对诊断能力进行扩展，但是这种方式存在许多弊端：

* 诊断操作功能与 Kube Diagnoser 的版本耦合。
* Kube Diagnoser 项目的维护者需要维护所有的诊断操作而不是只维护一个标准的插件接口。
* 诊断操作插件的开发者需要熟悉 Kube Diagnoser 的代码来扩展自己的诊断操作并公开自己插件的源代码。
* 复杂诊断操作插件的引入可能导致 Kube Diagnoser 项目维护成本骤增。

Processor 是 Kube Diagnoser 维护者与诊断操作插件维护者之间的接口，该接口可以帮助开发者将新实现的诊断操作插件注册到 Kube Diagnoser 中。

## 设计细节

Processor 通过 [Operation](./graph-based-pipeline.md#operation) API 进行注册，注册时需要在 Operation 中定义 Processor 的详细信息。

### API 对象

`Processor` API 对象的数据结构如下：

```go
// Processor 描述了如何在 Kube Diagnoser 中注册一个操作处理器。
type Processor struct {
    // ExternalIP 是操作处理器的监听 IP。
    // 如果该字段为空，那么默认为 Kube Diagnoser Agent 的地址。
    ExternalIP *string `json:"externalIP,omitempty"`
    // ExternalPort 是操作处理器的服务端口。
    // 如果该字段为空，那么默认为 Kube Diagnoser Agent 的服务端口。
    ExternalPort *int32 `json:"externalPort,omitempty"`
    // Path 是操作处理器服务的 HTTP 路径。
    Path *string `json:"path,omitempty"`
    // Scheme 是操作处理器服务的协议。
    Scheme *string `json:"scheme,omitempty"`
    // 操作处理器超时的秒数。
    // 默认为 30 秒。最小值为 1。
    TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}
```

### 通过 HTTP 请求让 Processor 执行诊断操作

Kube Diagnoser Agent 中的 Executor 负责向 Processor 发送 HTTP 请求让 Processor 执行诊断操作。HTTP 请求必须满足以下条件：

* 必须是 POST 请求。
* HTTP 请求中一般会包含请求体，请求体必须是 JSON 对象。
* HTTP 请求体中一般包含表示全局信息的键值对：
  * `diagnosis.uid` 记录了 Diagnosis 的 UID。
  * `diagnosis.namespace` 记录了 Diagnosis 的 Namespace。
  * `diagnosis.name` 记录了 Diagnosis 的 Name。
  * `pod.namespace` 记录了该 Processor 需要关注的 Pod 的 Namespace。
  * `pod.name` 记录了该 Processor 需要关注的 Pod 的 Name。
  * `container` 记录了该 Processor 需要关注的容器的 Name。
  * `node` 记录了该 Processor 需要关注的 Node 的 Name。

Processor 的实现必须满足以下条件：

* 能够处理 POST 请求。
* 能够处理 HTTP 请求中包含的请求体，即将 JSON 对象解析为 Map、Dict 等数据结构。
* 如果 HTTP 返回中包含返回体，返回体必须是 JSON 对象，详情可参考 [RFC 4627](https://www.ietf.org/rfc/rfc4627.txt)。

### 举例说明

通过创建下列 Operation 可以在 Kube Diagnoser 中注册向进程发送信号的 Processor：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Operation
metadata:
  name: signal-sender
spec:
  processor:
    path: /processor/signalsender
    scheme: http
    timeoutSeconds: 60
```

该 Operation 注册了一个向进程发送信号的 Processor，其监听地址与 Kube Diagnoser Agent 一致，HTTP 访问路径为 /processor/signalsender，请求超时时间为 60 秒。如果 Processor 是由诊断操作插件的开发者自行维护的，需要在创建时声明监听的地址以及端口号：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Operation
metadata:
  name: custom-operation
spec:
  processor:
    externalIP: 10.0.2.15
    externalPort: 6060
    path: /customoperation
    scheme: http
    timeoutSeconds: 60
```

以向进程 27065 发送信号为例，一次操作执行的流程如下：

1. 创建一个 Diagnosis 对象如: `defalt/kill-process` ，在对象中描述了要诊断的 Pod 和使用的 OperationSet，其中就包括发送信号的 Processor。
1. Kube Diagnoser Agent 中的 Executor 向发送信号的 Processor 执行 HTTP 请求，请求类型为 POST，请求体中包含如下 JSON 对象：

   ```json
   {
       "diagnosis.namespace": "default",
       "diagnosis.name": "kill-process",
       "pod.namespace": "default",
       "pod.name": "some-pod",
       "container": "debian",
       "signal.sender.pid": 27065,
       "signal.sender.signal": 15
   }
   ```

1. Processor 接收到请求后解析请求体中的 JSON 对象，向进程 27065 发送 SIGTERM 信号，如果这个 Processor 需要 Pod 的相关信息，可以通过 `pod.name` 和 `pod.namespace` 字段获取。
1. Processor 发送信号成功则向 Kube Diagnoser Agent 返回 200 状态码。
1. Processor 发送信号失败则向 Kube Diagnoser Agent 返回 500 状态码。
1. Kube Diagnoser Agent 中的 Executor 在请求返回后继续执行诊断逻辑。
