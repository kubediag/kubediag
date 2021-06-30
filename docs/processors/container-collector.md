# Container Collector

Container Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Container Collector 采集节点上的容器信息。

## 背景

在诊断过程中，用户可能需要采集节点上的容器信息。通过引入 Container Collector 可以满足该需求。

## 实现

Container Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Container Collector，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Container Collector：

```bash
$ kubectl get operation container-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-03-15T07:09:38Z"
  generation: 1
  name: container-collector
  resourceVersion: "12000039"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/container-collector
  uid: 82237cde-59bc-4812-93e3-2741bc64476f
spec:
  processor:
    path: /processor/containerCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Container Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/containerCollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有容器列表的 String 键值对。键为 `container.list`，值可以被解析为下列数据结构：

| Scheme | Description |
|-|-|
| [][Container](https://github.com/moby/moby/blob/v19.03.15/api/types/types.go#L58) | 容器的元数据信息数组。 |

### 举例说明

一次节点上容器信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Container Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Container Collector 接收到请求后在节点上调用 Docker 客户端获取节点上所有容器信息数组。
1. 如果 Container Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "collector.kubernetes.container.list": '[{{"Id":"b29a190d773988689efe3ef7b616e4dd4159f54ac4d131595d442a60d0b0b5ed","Names":["/k8s_POD_coredns-5644d7b6d9-c5tbq_kube-system_caf1aa9e-c2ee-4782-ad6d-98736fa15dd2_13"],"Image":"k8s.gcr.io/pause:3.1","ImageID":"sha256:da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e","Command":"/pause","Created":1622512223,"Ports":[],"Labels":{"annotation.cni.projectcalico.org/podIP":"192.168.236.158/32","annotation.cni.projectcalico.org/podIPs":"192.168.236.158/32","annotation.kubernetes.io/config.seen":"2021-06-01T09:49:50.754123792+08:00","annotation.kubernetes.io/config.source":"api","io.kubernetes.container.name":"POD","io.kubernetes.docker.type":"podsandbox","io.kubernetes.pod.name":"coredns-5644d7b6d9-c5tbq","io.kubernetes.pod.namespace":"kube-system","io.kubernetes.pod.uid":"caf1aa9e-c2ee-4782-ad6d-98736fa15dd2","k8s-app":"kube-dns","pod-template-hash":"5644d7b6d9"},"State":"running","Status":"Up 5 hours","HostConfig":{"NetworkMode":"none"},"NetworkSettings":{"Networks":{"none":{"IPAMConfig":null,"Links":null,"Aliases":null,"NetworkID":"2499f96edd88779350ff37ed240f7466fcbda7825532f8fd868a6c66872d1f4c","EndpointID":"bc34c9eca08ae33e719dfd7acbff65fe83fe55f798f0aeab179865333650c880","Gateway":"","IPAddress":"","IPPrefixLen":0,"IPv6Gateway":"","GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"MacAddress":"","DriverOpts":null}}},"Mounts":[]},......}]'
}
```

1. 如果 Container Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。

