# Command Executor

Command Executor 是一个 [Processor](../architecture/processor.md)，用户可以通过 Command Executor 自定义在故障诊断恢复流程中执行的命令。

## 背景

在诊断过程中，用户可能需要在节点上执行某些命令来采集信息。通过引入 Command Executor 可以满足以下需求：

* 在节点上执行命令以采集信息。
* 在节点上执行命令以恢复故障。
* 开发者不需要针对某个简单命令实现新的 Processor。

## 实现

Command Executor 按照 [Processor](../architecture/processor.md) 规范实现。通过 Operation 可以在 Kube Diagnoser 中注册 Command Executor，该 Operation 在 Kube Diagnoser 部署时已默认注册，执行下列命令可以查看已注册的 Command Executor：

```bash
$ kubectl get operation command-executor -o yaml
apiVersion: diagnosis.netease.com/v1
kind: Operation
metadata:
  creationTimestamp: "2021-03-15T07:09:38Z"
  generation: 1
  name: command-executor
  resourceVersion: "12000033"
  selfLink: /apis/diagnosis.netease.com/v1/operations/command-executor
  uid: 740c8bff-add4-444d-a544-9ef718221ea7
spec:
  processor:
    path: /processor/commandexecutor
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Command Executor 处理的请求必须为 POST 类型，处理的 HTTP 请求中必须包含特定 JSON 格式请求体。

#### HTTP 请求

POST /processor/commandexecutor

#### 请求体参数

JSON 请求体格式为 Object，参数如下：

| Parameter | Scheme | Description |
|-|-|-|
| command | string | 待执行命令。 |
| args | []string | 待执行命令的参数。 |
| timeoutSeconds | int | 命令执行器执行超时时间。 |

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 400 | Bad Request |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 Object，参数如下：

| Parameter | Scheme | Description |
|-|-|-|
| stdout | string | 命令执行的标准输出，如果命令无标准输出该字段则为空。 |
| stderr | string | 命令执行的标准错误，如果命令无标准错误该字段则为空。 |
| error | string | 命令执行的错误，如果命令执行成功该字段则为空。 |

### 举例说明

以执行命令 `du -sh /` 为例，一次操作执行的流程如下：

1. Kube Diagnoser Agent 向 Command Executor 发送 HTTP 请求，请求类型为 POST，请求体中包含如下 JSON 数据：

   ```json
   {
       "command": "du",
       "args": [
           "-sh",
           "/"
       ],
       "timeoutSeconds": 10
   }
   ```

1. Command Executor 接收到请求后解析请求体中的 JSON 数据，在节点上执行命令 `du -sh /`，默认超时时间为 10 秒。
1. Command Executor 命令执行超时并向 Kube Diagnoser Agent 返回 500 状态码，返回体中包含如下 JSON 数据：

   ```json
   {
       "error": "command [du -sh /] timed out"
   }
   ```
