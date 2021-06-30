# Node Cordon

Node Cordon 是一个 [Processor](../design/processor.md)，用户可以通过 Node Cordon 将节点置为不可调度。

## 背景

在对某些节点进行故障隔离的过程中，用户可能需要将故障节点置为不可调度。通过引入 Node Cordon 可以满足该需求。

## 实现

Node Cordon 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Node Cordon，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Node Cordon：

```bash
$ kubectl get operation node-cordon -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T06:34:21Z"
  generation: 1
  name: node-cordon
  resourceVersion: "35665"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/node-cordon
  uid: 24c2012e-42c6-4f09-a259-fd10bc924836
spec:
  processor:
    path: /processor/nodeCordon
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Node Cordon 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/nodeCordon

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有被置为不可调度 Node 的 String 键值对。键为 `recover.kubernetes.node_cordon.result.name`，值为节点名。

### 举例说明

一次节点上 Containerd 栈信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Node Cordon 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Node Cordon 接收到请求后调用 Kubernetes 客户端将当前 KubeDiag Agent 所在节点置为不可调度。
1. 如果 Node Cordon 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "recover.kubernetes.node_cordon.result.name": "my-node"
}
```

1. 如果 Node Cordon 采集失败则向 KubeDiag Agent 返回 500 状态码。
