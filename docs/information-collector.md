# InformationCollector API 设计

InformationCollector 用于表示信息采集器实例。故障诊断恢复平台通过 InformationCollector 提供以下功能：

* 注册信息采集器到故障诊断恢复平台。
* 记录信息采集器当前的状态。
* 对信息采集器进行管理，InformationCollector 中记录了信息采集器的访问地址。
* 支持基于 Prometheus 的监控扩展信息。

## InformationCollector

InformationCollector 用于表示信息采集器实例。

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | API 资源元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |
| spec | 信息采集器的说明。 | [InformationCollectorSpec](#informationcollectorspec) | true |
| status | 信息采集器当前的状态。 | [InformationCollectorStatus](#informationcollectorstatus) | true |

## InformationCollectorSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| ip | 信息采集器的监听 IP。 | string | true |
| port | 信息采集器的监听端口。 | string | true |
| path | 信息采集器的 HTTP 路径。 | string | false |
| metricPath | 信息采集器的 Metric 路径。 | string | false |
| livenessProbe | 信息采集器的健康检查探针，探测失败时不会重建信息采集器。 | [v1core.Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#probe-v1-core) | false |
| readinessProbe | 信息采集器的就绪检查探针，探测失败时不会发送请求到信息采集器。 | [v1core.Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#probe-v1-core) | false |
| scheme | 信息采集器的 HTTP 协议。 | string | false |
| timeoutSeconds | 信息采集器执行超时时间。 | int32 | false |

## InformationCollectorStatus

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| ready | 信息采集器是否就绪。 | bool | true |
| healthy | 信息采集器是否健康。 | bool | true |
