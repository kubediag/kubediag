# Docker Info Collector

Docker Info Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Docker Info Collector 获取节点上 Docker 系统信息。

## 背景

在诊断过程中，用户可能需要获取节点上 Docker 系统信息。通过引入 Docker Info Collector 可以满足该需求。

## 实现

Docker Info Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Docker Info Collector，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Docker Info Collector：

```bash
$ $ kubectl get operation docker-info-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T06:33:56Z"
  generation: 1
  name: docker-info-collector
  resourceVersion: "35507"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/docker-info-collector
  uid: 69c8afa5-f98c-4eb9-adca-f4babbd4ca52
spec:
  processor:
    path: /processor/dockerInfoCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Docker Info Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/dockerInfoCollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有 Containerd 栈信息生成时间戳的 String 键值对。键为 `collector.kubernetes.docker.info`，值可以被解析为下列数据结构：

| Scheme | Description |
|-|-|
| [Info](https://github.com/moby/moby/blob/v19.03.15/api/types/types.go#L147) | Docker 系统信息。 |

### 举例说明

一次节点上 Docker 系统信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Docker Info Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Docker Info Collector 接收到请求后在节点上调用 Docker 客户端获取节点系统信息。
1. 如果 Docker Info Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含一个 map[string]string  ,记录了 docker 服务信息，该信息将保存到 Diagnosis 对象中，如下：

```json
  collector.kubernetes.docker.info: |
    Client:
     Debug Mode: false
    
    Server:
     Containers: 40
      Running: 33
      Paused: 0
      Stopped: 7
     Images: 326
     Server Version: 19.03.15
     Storage Driver: overlay
    ...

```

1. 如果 Docker Info Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。
