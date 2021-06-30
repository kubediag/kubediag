# Process Collector

Process Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Process Collector 采集节点上的进程信息。

## 背景

在诊断过程中，用户可能需要采集节点上的进程信息。通过引入 Process Collector 可以满足该需求。

## 实现

Process Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Process Collector，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Process Collector：

```bash
$ kubectl get operation process-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T03:30:46Z"
  generation: 1
  name: process-collector
  resourceVersion: "4892"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/process-collector
  uid: a4d4eff3-7059-45f3-9e8d-c3a9280cd224
spec:
  processor:
    path: /processor/processCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Process Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/processCollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有进程列表的 String 键值对。键为 `collector.system.process.list`，值可以被解析为下列数据结构：

| Scheme | Description |
|-|-|
| []Process | 进程的元数据信息数组。 |

### 举例说明

一次节点上进程信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Process Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Process Collector 接收到请求后在节点上获取所有进程信息数组。
1. 如果 Process Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "collector.system.process.list": '[{"pid":1,"ppid":0,"tgid":1,"command":["/sbin/init","splash"],"status":"S","createTime":"2021-06-02T01:35:50Z","cpuPercent":1.7139181742323948,"nice":20,"memoryInfo":{"rss":10752000,"vms":165097472,"hwm":0,"data":0,"stack":0,"locked":0,"swap":0}},......]'
}
```

1. 如果 Process Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。
