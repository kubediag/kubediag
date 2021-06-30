# Containerd Goroutine Collector

Containerd Goroutine Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Containerd Goroutine Collector 获取节点上 Containerd 栈信息。

## 背景

在对 Containerd 进行分析的过程中，用户可能需要获取节点上 Containerd 栈信息。通过引入 Containerd Goroutine Collector 可以满足该需求。

## 实现

Containerd Goroutine Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Containerd Goroutine Collector，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Containerd Goroutine Collector：

```bash
$ kubectl get operation containerd-goroutine-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T06:33:56Z"
  generation: 1
  name: containerd-goroutine-collector
  resourceVersion: "35510"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/containerd-goroutine-collector
  uid: bbba5c0d-2bb1-49e2-925a-b6f2643a79fb
spec:
  processor:
    path: /processor/containerdGoroutineGollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Containerd Goroutine Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/containerdGoroutineCollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有 Containerd 栈信息生成时间戳的 String 键值对。键为 `collector.runtime.containerd.goroutine`，值可以被解析为下列数据结构：

| Scheme | Description |
|-|-|
| time.Time | Containerd 栈信息生成时间戳。 |

### 举例说明

一次节点上 Containerd 栈信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Containerd Goroutine Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Containerd Goroutine Collector 接收到请求后在节点上向 Containerd 进程发送 SIGUSR1 信号以生成栈信息，栈信息存储在 Containerd 的日志中。
1. 如果 Containerd Goroutine Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "collector.runtime.containerd.goroutine": "2021-05-17 09:23:46.857587182 +0000 UTC m=+212.139352209"
}
```

1. 如果 Containerd Goroutine Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。
