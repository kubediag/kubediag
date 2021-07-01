# Dockerd Goroutine Collector

Dockerd Goroutine Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Dockerd Goroutine Collector 获取节点上 Dockerd 栈信息。

## 背景

在对 Dockerd 进行分析的过程中，用户可能需要获取节点上 Dockerd 栈信息。通过引入 Dockerd Goroutine Collector 可以满足该需求。

## 实现

Dockerd Goroutine Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Dockerd Goroutine Collector，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Dockerd Goroutine Collector：

```bash
$ kubectl get operation dockerd-goroutine-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T06:33:56Z"
  generation: 1
  name: dockerd-goroutine-collector
  resourceVersion: "35510"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/dockerd-goroutine-collector
  uid: bbba5c0d-2bb1-49e2-925a-b6f2643a79fb
spec:
  processor:
    path: /processor/dockerdGoroutineCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Dockerd Goroutine Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/dockerdgoroutinecollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有 Dockerd 栈信息日志路径的 String 键值对。键为 `collector.runtime.dockerd.goroutine`，值为 Dockerd 栈信息日志路径。

### 举例说明

一次节点上 Dockerd 栈信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Dockerd Goroutine Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Dockerd Goroutine Collector 接收到请求后在节点上向 Dockerd 进程发送 SIGUSR1 信号以生成栈信息，栈信息日志会被收集到 KubeDiag 的数据根目录。
1. 如果 Dockerd Goroutine Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "collector.runtime.dockerd.goroutine": "/var/lib/kubediag/dockerd-goroutine/goroutine-stacks-2021-05-17T172336+0800.log"
}
```

1. 如果 Dockerd Goroutine Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。
