# Container Collector

Container Collector 是一个 Kube Diagnoser 内置的信息采集器，用于收集节点上的容器信息。

## 如何部署

运行以下命令注册 Container Collector 信息采集器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/deploy/container_collector.yaml
```

## 请求类型

Container Collector 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/informationcollector/containercollector`。Container Collector 可以对 POST 和 GET 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Container Collector 请求 Docker 获取节点容器列表并将容器列表记录到 Abnormal 的 `.status.context.containerInformation` 字段。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 当接收到 GET 请求时，Container Collector 请求 Docker 获取节点容器列表。处理成功返回节点容器列表，返回码为 `200`；处理失败则返回错误信息，返回码为 `500`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.assignedInformationCollectors` 字段中包含 `container-collector` 来使用 Container Collector 采集信息。一个指定 Container Collector 采集信息的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: container-collector
spec:
  assignedInformationCollectors:
  - name: container-collector
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  skipDiagnosis: true
  skipInformationCollection: false
  skipRecovery: true
  source: Custom
```

Container Collector 成功采集信息后会将节点容器列表记录到 `.status.context.containerInformation` 字段：

```yaml
status:
  context:
    containerInformation:
    - Command: /pause
      Created: 1597630531
      HostConfig:
        NetworkMode: none
      Id: 93f241547812697998f5a3e257745741cec209fb61aa5f14ec0f9fbd84de409b
      Image: k8s.gcr.io/pause:3.1
      ImageID: sha256:da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e
      Labels:
        annotation.kubernetes.io/config.seen: "2020-08-17T10:14:33.641535585+08:00"
        annotation.kubernetes.io/config.source: api
        app: nginx
        io.kubernetes.container.name: POD
        io.kubernetes.docker.type: podsandbox
        io.kubernetes.pod.name: nginx-deployment-7fd6966748-lt65j
        io.kubernetes.pod.namespace: default
        io.kubernetes.pod.uid: 13d025ab-451d-40b4-9d18-d2b0d09cd102
        pod-template-hash: 7fd6966748
      Mounts: []
      Names:
      - /k8s_POD_nginx-deployment-7fd6966748-lt65j_default_13d025ab-451d-40b4-9d18-d2b0d09cd102_102
      NetworkSettings:
        Networks:
          none:
            Aliases: null
            DriverOpts: null
            EndpointID: 7132b2fd1deffd7564370f7af662e9e62311be14e5b35240b1b9f26f0a9b5b3b
            Gateway: ""
            GlobalIPv6Address: ""
            GlobalIPv6PrefixLen: 0
            IPAMConfig: null
            IPAddress: ""
            IPPrefixLen: 0
            IPv6Gateway: ""
            Links: null
            MacAddress: ""
            NetworkID: 2499f96edd88779350ff37ed240f7466fcbda7825532f8fd868a6c66872d1f4c
      Ports: []
      State: running
      Status: Up About a minute
```
