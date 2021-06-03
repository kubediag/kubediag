# Docker Info Collector

Docker Info Collector 是一个 [Processor](../architecture/processor.md)，用户可以通过 Docker Info Collector 获取节点上 Docker 系统信息。

## 背景

在诊断过程中，用户可能需要获取节点上 Docker 系统信息。通过引入 Docker Info Collector 可以满足该需求。

## 实现

Docker Info Collector 按照 [Processor](../architecture/processor.md) 规范实现。通过 Operation 可以在 Kube Diagnoser 中注册 Docker Info Collector，该 Operation 在 Kube Diagnoser 部署时已默认注册，执行下列命令可以查看已注册的 Docker Info Collector：

```bash
$ $ kubectl get operation docker-info-collector -o yaml
apiVersion: diagnosis.netease.com/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T06:33:56Z"
  generation: 1
  name: docker-info-collector
  resourceVersion: "35507"
  selfLink: /apis/diagnosis.netease.com/v1/operations/docker-info-collector
  uid: 69c8afa5-f98c-4eb9-adca-f4babbd4ca52
spec:
  processor:
    path: /processor/dockerinfocollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Docker Info Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/dockerinfocollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有 Containerd 栈信息生成时间戳的 String 键值对。键为 `containerd.goroutine`，值可以被解析为下列数据结构：

| Scheme | Description |
|-|-|
| [Info](https://github.com/moby/moby/blob/v19.03.15/api/types/types.go#L147) | Docker 系统信息。 |

### 举例说明

一次节点上 Docker 系统信息采集操作执行的流程如下：

1. Kube Diagnoser Agent 向 Docker Info Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Docker Info Collector 接收到请求后在节点上调用 Docker 客户端获取节点系统信息。
1. 如果 Docker Info Collector 完成采集则向 Kube Diagnoser Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "docker.info": '{"ID":"LJM3:UWWT:L6L3:J6RJ:QRB2:NPMT:FXNC:WA6A:S2AN:JNKV:XE6V:HL7C","Containers":90,"ContainersRunning":47,"ContainersPaused":0,"ContainersStopped":43,"Images":135,"Driver":"overlay2","DriverStatus":[["Backing Filesystem","\u003cunknown\u003e"],["Supports d_type","true"],["Native Overlay Diff","true"]],"SystemStatus":null,"Plugins":{"Volume":["local"],"Network":["bridge","host","ipvlan","macvlan","null","overlay"],"Authorization":null,"Log":["awslogs","fluentd","gcplogs","gelf","journald","json-file","local","logentries","splunk","syslog"]},"MemoryLimit":true,"SwapLimit":false,"KernelMemory":true,"KernelMemoryTCP":true,"CpuCfsPeriod":true,"CpuCfsQuota":true,"CPUShares":true,"CPUSet":true,"PidsLimit":true,"IPv4Forwarding":true,"BridgeNfIptables":true,"BridgeNfIp6tables":true,"Debug":false,"NFd":272,"OomKillDisable":true,"NGoroutines":227,"SystemTime":"2021-05-18T17:23:36.750559813+08:00","LoggingDriver":"json-file","CgroupDriver":"systemd","NEventsListener":0,"KernelVersion":"4.15.0-143-generic","OperatingSystem":"Ubuntu 18.04.3 LTS","OSType":"linux","Architecture":"x86_64","IndexServerAddress":"https://index.docker.io/v1/","RegistryConfig":{"AllowNondistributableArtifactsCIDRs":[],"AllowNondistributableArtifactsHostnames":[],"InsecureRegistryCIDRs":["127.0.0.0/8"],"IndexConfigs":{"docker.io":{"Name":"docker.io","Mirrors":["https://docker.mirrors.ustc.edu.cn/"],"Secure":true,"Official":true}},"Mirrors":["https://docker.mirrors.ustc.edu.cn/"]},"NCPU":4,"MemTotal":11645636608,"GenericResources":null,"DockerRootDir":"/data","HttpProxy":"","HttpsProxy":"","NoProxy":"","Name":"netease","Labels":[],"ExperimentalBuild":false,"ServerVersion":"19.03.8","ClusterStore":"","ClusterAdvertise":"","Runtimes":{"runc":{"path":"runc"}},"DefaultRuntime":"runc","Swarm":{"NodeID":"","NodeAddr":"","LocalNodeState":"inactive","ControlAvailable":false,"Error":"","RemoteManagers":null},"LiveRestoreEnabled":false,"Isolation":"","InitBinary":"docker-init","ContainerdCommit":{"ID":"7ad184331fa3e55e52b890ea95e65ba581ae3429","Expected":"7ad184331fa3e55e52b890ea95e65ba581ae3429"},"RuncCommit":{"ID":"dc9208a3303feef5b3839f4323d9beb36df0a9dd","Expected":"dc9208a3303feef5b3839f4323d9beb36df0a9dd"},"InitCommit":{"ID":"fec3683","Expected":"fec3683"},"SecurityOptions":["name=apparmor","name=seccomp,profile=default"],"Warnings":["WARNING: No swap limit support"]}'
}
```

1. 如果 Docker Info Collector 采集失败则向 Kube Diagnoser Agent 返回 500 状态码。
