# Processor

Processor 是处理用户诊断请求的实体，通常通过 Operation 资源进行注册。

## 背景和动机

KubeDiag 内置实现了部分收集信息或分析数据的操作。通过在 KubeDiag 的代码中实现诊断操作可以对诊断能力进行扩展，但是这种方式存在许多弊端：

* 诊断操作功能与 KubeDiag 的版本耦合。
* KubeDiag 项目的维护者需要维护所有的诊断操作而不是只维护一个标准的插件接口。
* 诊断操作插件的开发者需要熟悉 KubeDiag 的代码来扩展自己的诊断操作并公开自己插件的源代码。
* 复杂诊断操作插件的引入可能导致 KubeDiag 项目维护成本骤增。

Processor 是 KubeDiag 维护者与诊断操作插件维护者之间的接口，该接口可以帮助开发者将新实现的诊断操作插件注册到 KubeDiag 中。

## 设计细节

Processor 通过 [Operation](./graph-based-pipeline.md#operation) API 进行注册，注册时需要在 Operation 中定义 Processor 的详细信息。

### API 对象

`Processor` API 对象的数据结构如下：

```go
// Processor 描述了如何在 KubeDiag 中注册一个操作处理器。
type Processor struct {
    // ExternalAddress 是操作处理器的监听地址。
    // 如果该字段为空，那么默认为 KubeDiag Agent 的地址。
    ExternalAddress *string `json:"externalAddress,omitempty"`
    // ExternalPort 是操作处理器的服务端口。
    // 如果该字段为空，那么默认为 KubeDiag Agent 的服务端口。
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

KubeDiag Agent 中的 Executor 负责向 Processor 发送 HTTP 请求让 Processor 执行诊断操作。HTTP 请求必须满足以下条件：

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

通过创建下列 Operation 可以在 KubeDiag 中注册向进程发送信号的 Processor：

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: signal-sender
spec:
  processor:
    path: /processor/signalsender
    scheme: http
    timeoutSeconds: 60
```

该 Operation 注册了一个向进程发送信号的 Processor，其监听地址与 KubeDiag Agent 一致，HTTP 访问路径为 /processor/signalsender，请求超时时间为 60 秒。如果 Processor 是由诊断操作插件的开发者自行维护的，需要在创建时声明监听的地址以及端口号：

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: custom-operation
spec:
  processor:
    externalAddress: 10.0.2.15
    externalPort: 6060
    path: /customoperation
    scheme: http
    timeoutSeconds: 60
```

以向进程 27065 发送信号为例，一次操作执行的流程如下：

1. 创建一个 Diagnosis 对象如: `defalt/kill-process` ，在对象中描述了要诊断的 Pod 和使用的 OperationSet，其中就包括发送信号的 Processor。
1. KubeDiag Agent 中的 Executor 向发送信号的 Processor 执行 HTTP 请求，请求类型为 POST，请求体中包含如下 JSON 对象：

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
1. Processor 发送信号成功则向 KubeDiag Agent 返回 200 状态码。
1. Processor 发送信号失败则向 KubeDiag Agent 返回 500 状态码。
1. KubeDiag Agent 中的 Executor 在请求返回后继续执行诊断逻辑。


## 命名与规范

当我们设计实现一个 Processor 时，需要考虑这个 Processor 的 IO ，即如何访问、如何响应。另外还有一些需要命名的地方。在此我们制定一个规范，以便于更直观、更统一地管理和使用 Processor。

所有需要命名的地方，可以分为两类：

1. 以 `key-value` 格式构建的 Parameters 或 Contexts ，他们分别存在于该 Processor 的调用参数和返回结果中。这部分我们要求使用 `.` 连接成员，使用下划线 `_` 连接词。
1. 除了 1 以外的场景，我们规定使用驼峰形命名。

下面是设计 Processor 过程中需要命名的地方以及命名范例。

### URL 和 Parameter

一个 Processor 的访问 URL ，要求满足 `/processor/nameOfProcessor` 的格式。 例如：

* /processor/podListCollector
* /processor/subpathRemountDiagnoser
* /processor/nodeCordon
* /processor/containerdGoroutineCollector

访问 Processor 时，使用 `map[string]string` 结构传参， map 中要求使用 `param` 前缀，使用 `.` 连接成员，使用 `_` 连接词。例如：

* `param.diagnoser.runtime.core_file_profiler.expiration_seconds`
* `param.diagnoser.runtime.go_profiler.tls.secret_reference.namespace`

key 中成员的设计可以参考[成员设计](#成员设计)

### Response 中的 Context

Processor 处理请求后会将结果以 `map[string]string` 的格式返回给 KubeDiag Agent 。 我们称这个 map 为 Context 。因为其中的内容会作为后续 Processor 的 Parameters 。

上文提到 Parameter 的格式， 显然，此处 Context 的 key 的命名规范也是一样的。不过为了与 Parameter 区分，所以不需要 `param` 前缀。例如：

* `diagnoser.kubernetes.subpath_remount.original_source_path`
* `collector.kubernetes.pod.detail`
* `collector.system.mountinfo`
* `diagnoser.runtime.core_file_profiler.result.endpoint`

key 中成员的设计可以参考[成员设计](#成员设计)

### Processor 中的日志标题

Processor 的日志标题命名与它的 URL 命名保持一致。也即：

* /processor/podListCollector
* /processor/subpathRemountDiagnoser
* /processor/nodeCordon
* /processor/containerdGoroutineCollector

### 成员设计

key 中成员的设计没有太多的条件约束，只有一条强制要求： Parameter 中的 key 要以 `param` 为前缀。

但是为了方便阅读，我们还是建议保持一定的规则：

1. 第一成员为 Processor 的分类，如：
    * collector  即完成信息采集工作的 Processor
    * diagnoser 即完成信息分析，故障确认工作的 Processor
    * recover 即完成故障恢复或处理工作的 Processor
1. 第二成员为工作域的描述， 如：
    * kubernetes  表示该 Processor 做的工作与 kubernetes / docker 有关
    * runtime 表示该 Processor 做的工作与进程运行时有关
    * system  表示该 Processor 做的工作与宿主机系统有关
1. 第三成员及之后的成员， 表示更细粒度的责任域划分。

这里提供的规则仅是建议。我们不强求设计 Parameter / Context 字段时严格执行这种思路。例如 Processor 可能完成了 kubernetes 信息采集和节点上 system 层面的诊断， 此时我们确实无法将该 Processor 进行合适的分类。将相关的 key 使用 `kubernetes.playbook_20210311.execution_command` 来描述也是可以正常工作的。 但请确保你的其他 Processor  、 或其他人的 Processor  知晓这个 Processor 的 key 的命名和内容结构 (schema)。
