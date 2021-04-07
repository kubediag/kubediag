# Container Collector

Container Collector 是一个 [Processor](../architecture/processor.md)，用户可以通过 Container Collector 采集节点上的容器信息。

## 背景

在诊断过程中，用户可能需要采集节点上的容器信息。通过引入 Container Collector 可以满足该需求。

## 实现

Container Collector 按照 [Processor](../architecture/processor.md) 规范实现。通过 Operation 可以在 Kube Diagnoser 中注册 Container Collector，该 Operation 在 Kube Diagnoser 部署时已默认注册，执行下列命令可以查看已注册的 Container Collector：

```bash
$ kubectl get operation container-collector -o yaml
apiVersion: diagnosis.netease.com/v1
kind: Operation
metadata:
  creationTimestamp: "2021-03-15T07:09:38Z"
  generation: 1
  name: container-collector
  resourceVersion: "12000039"
  selfLink: /apis/diagnosis.netease.com/v1/operations/container-collector
  uid: 82237cde-59bc-4812-93e3-2741bc64476f
spec:
  processor:
    path: /processor/containercollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Container Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/containercollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 Array，数组中单个元素如下：

| Scheme | Description |
|-|-|
| [Container](https://github.com/moby/moby/blob/v19.03.15/api/types/types.go#L58) | 容器的元数据信息。 |

### 举例说明

一次节点上容器信息采集操作执行的流程如下：

1. Kube Diagnoser Agent 向 Container Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Container Collector 接收到请求后在节点上调用 Docker 客户端获取节点上所有容器信息。
1. 如果 Container Collector 完成采集则向 Kube Diagnoser Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

   ```json
   [
       {
           "Command": "/pause",
           "Created": 1597630531,
           "HostConfig": {
               "NetworkMode": "none"
           },
           "Id": "93f241547812697998f5a3e257745741cec209fb61aa5f14ec0f9fbd84de409b",
           "Image": "k8s.gcr.io/pause:3.1",
           "ImageID": "sha256:da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e",
           "Labels": {
               "annotation.kubernetes.io/config.seen": "2020-08-17T10:14:33.641535585+08:00",
               "annotation.kubernetes.io/config.source": "api",
               "app": "nginx",
               "io.kubernetes.container.name": "POD",
               "io.kubernetes.docker.type": "podsandbox",
               "io.kubernetes.pod.name": "nginx-deployment-7fd6966748-lt65j",
               "io.kubernetes.pod.namespace": "default",
               "io.kubernetes.pod.uid": "13d025ab-451d-40b4-9d18-d2b0d09cd102",
               "pod-template-hash": "7fd6966748"
           },
           "Mounts": [],
           "Names": [
               "/k8s_POD_nginx-deployment-7fd6966748-lt65j_default_13d025ab-451d-40b4-9d18-d2b0d09cd102_102"
           ],
           "NetworkSettings": {
               "Networks": {
                   "none": {
                       "Aliases": null,
                       "DriverOpts": null,
                       "EndpointID": "7132b2fd1deffd7564370f7af662e9e62311be14e5b35240b1b9f26f0a9b5b3b",
                       "Gateway": "",
                       "GlobalIPv6Address": "",
                       "GlobalIPv6PrefixLen": 0,
                       "IPAMConfig": null,
                       "IPAddress": "",
                       "IPPrefixLen": 0,
                       "IPv6Gateway": "",
                       "Links": null,
                       "MacAddress": "",
                       "NetworkID": "2499f96edd88779350ff37ed240f7466fcbda7825532f8fd868a6c66872d1f4c"
                   }
               }
           },
           "Ports": [],
           "State": "running",
           "Status": "Up About a minute"
       },
       // ......
   ]
   ```

1. 如果 Container Collector 采集失败则向 Kube Diagnoser Agent 返回 500 状态码，返回体中包含描述错误的字符串：

   ```string
   Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
   ```
