# Systemd Collector

Systemd Collector 是一个 Kube Diagnoser 内置的信息采集器，用于采集节点上的 Systemd 相关信息。

## 如何部署

运行以下命令注册 Systemd Collector 信息采集器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/samples/systemd_collector.yaml
```

## 请求类型

Systemd Collector 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/informationcollector/systemdcollector`。Systemd Collector 可以对 POST 和 GET 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Systemd Collector 从 Abnormal 的 `.spec.context.systemdUnitNameInformationContextKey` 字段获取需要处理的 Unit 列表并将相应的 Property 信息记录到 Abnormal 的 `.status.context.systemdUnitPropertyInformationContextKey` 字段。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 当接收到 GET 请求时，Systemd Collector 从 `systemdUnitNameInformationContextKey` 请求参数获取需要处理的 Unit 列表。处理成功返回相应的 Property 信息，返回码为 `200`；处理失败则返回错误信息，返回码为 `500`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

### 通过 Abnormal 使用

用户可以创建 Abnormal 并在 `.spec.assignedInformationCollectors` 字段中包含 `systemd-collector` 来使用 Systemd Collector 采集信息。一个指定 Systemd Collector 采集信息的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: systemd-collection
spec:
  source: Custom
  assignedInformationCollectors:
  - name: systemd-collector
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  context:
    systemdUnitNameInformationContextKey:
    - "docker"
    - "kubelet"
    - ""
```

当 `.spec.context.systemdUnitNameInformationContextKey` 中某个 Unit 为空字符串时，Systemd Collector 会采集 Systemd 管理器的 Property 信息。Systemd Collector 成功采集信息后会将相应的 Property 信息记录到 `.status.context.systemdUnitPropertyInformationContextKey` 字段：

```yaml
status:
  conditions:
  - lastTransitionTime: "2020-09-23T03:06:17Z"
    status: "True"
    type: InformationCollected
  - lastTransitionTime: "2020-09-23T03:06:17Z"
    status: "True"
    type: Identified
  - lastTransitionTime: "2020-09-23T03:06:17Z"
    status: "True"
    type: Recovered
  context:
    systemdUnitPropertyInformationContextKey:
    - name: docker
      properties:
      - name: Type
        value: notify
      - name: Restart
        value: always
      ...
    - name: kubelet
      properties:
      - name: Type
        value: simple
      - name: Restart
        value: always
      ...
    - name: ""
      properties:
      - name: Version
        value: "237"
      - name: Features
        value: +PAM +AUDIT +SELINUX +IMA +APPARMOR +SMACK +SYSVINIT +UTMP +LIBCRYPTSETUP
          +GCRYPT +GNUTLS +ACL +XZ +LZ4 +SECCOMP +BLKID +ELFUTILS +KMOD -IDN2 +IDN
          -PCRE2 default-hierarchy=hybrid
      ...
```

### 通过 GET 请求使用

用户可以通过 GET 请求来使用 Systemd Collector 采集信息。一个指定 Systemd Collector 采集信息的 GET 请求 URL 如下所示：

```
/informationcollector/systemdcollector?systemdUnitNameInformationContextKey=kubelet%2Cdocker%2C
```

通过 GET 请求来使用 Systemd Collector 的请求路径为 `/informationcollector/systemdcollector`。请求参数的键值为 `systemdUnitNameInformationContextKey=kubelet,docker,`。当请求参数的值中某个 Unit 为空字符串时，Systemd Collector 会采集 Systemd 管理器的 Property 信息。该请求参数表示采集 kubelet、docker 以及 Systemd 管理器的 Property 信息。Systemd Collector 成功采集信息后会返回相应的 Property 信息：

```json
[
  {
    "name": "kubelet",
    "properties": [
      {
        "name": "Type",
        "value": "simple"
      },
      {
        "name": "Restart",
        "value": "always"
      },
      ...
    ]
  },
  {
    "name": "docker",
    "properties": [
      {
        "name": "Type",
        "value": "notify"
      },
      {
        "name": "Restart",
        "value": "always"
      },
      ...
    ]
  },
  {
    "name": "",
    "properties": [
      {
        "name": "Version",
        "value": "237"
      },
      {
        "name": "Features",
        "value": "+PAM +AUDIT +SELINUX +IMA +APPARMOR +SMACK +SYSVINIT +UTMP +LIBCRYPTSETUP +GCRYPT +GNUTLS +ACL +XZ +LZ4 +SECCOMP +BLKID +ELFUTILS +KMOD -IDN2 +IDN -PCRE2 default-hierarchy=hybrid"
      },
      ...
    ]
  }
]
```
