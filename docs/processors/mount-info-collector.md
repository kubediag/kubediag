# Mount Info Collector

Mount Info Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Process Collector 采集节点上的 mount 情况。

## 背景

在诊断过程中，用户可能需要采集节点上的 mount 信息。通过引入 Mount Info Collector 可以满足该需求。

## 实现

Mount Info Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Mount Info Collector ，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Mount Info Collector ：

```bash
$ kubectl get operation mount-info-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T03:30:46Z"
  generation: 1
  name: mount-info-collector
  resourceVersion: "4892"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/mount-info-collector
  uid: a4d4eff3-7059-45f3-9e8d-c3a9280cd224
spec:
  processor:
    path: /processor/mountInfoCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Mount Info Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/mountInfoCollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有进程列表的 String 键值对。键为 `collector.system.mountinfo` 。 值即为宿主机上 /proc/1/mountinfo 文件的内容。

### 举例说明

一次节点上进程信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Mount Info Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Mount Info Collector 接收到请求后在节点上获取 mount 信息。
1. 如果 Mount Info Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
    collector.system.mountinfo: |
      17 22 0:17 / /sys rw,nosuid,nodev,noexec,relatime shared:7 - sysfs sysfs rw
      18 22 0:4 / /proc rw,nosuid,nodev,noexec,relatime shared:12 - proc proc rw
      19 22 0:6 / /dev rw,nosuid,relatime shared:2 - devtmpfs udev rw,size=8202632k,nr_inodes=2050658,mode=755
      20 19 0:18 / /dev/pts rw,nosuid,noexec,relatime shared:3 - devpts devpts rw,gid=5,mode=620,ptmxmode=000
      21 22 0:19 / /run rw,nosuid,noexec,relatime shared:5 - tmpfs tmpfs rw,size=1642800k,mode=755
      22 0 254:1 / / rw,relatime shared:1 - ext4 /dev/vda1 rw,errors=remount-ro,data=ordered
      23 17 0:16 / /sys/kernel/security rw,nosuid,nodev,noexec,relatime shared:8 - securityfs securityfs rw
      24 19 0:20 / /dev/shm rw,nosuid,nodev shared:4 - tmpfs tmpfs rw
      ...
```

1. 如果 Mount Info Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。
