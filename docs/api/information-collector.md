# InformationCollector API 设计

InformationCollector 用于表示信息采集器实例。故障诊断恢复平台通过 InformationCollector 提供以下功能：

* 注册信息采集器到故障诊断恢复平台。
* 记录信息采集器当前的状态。
* 对信息采集器进行管理，InformationCollector 中记录了信息采集器的访问地址。

## InformationCollector

InformationCollector 用于表示信息采集器实例。

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | API 资源元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |
| spec | 信息采集器的说明。 | [InformationCollectorSpec](#informationcollectorspec) | true |

## InformationCollectorSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| externalIP | 外部信息采集器的监听 IP。 | string | false |
| externalPort | 外部信息采集器的监听端口。 | string | false |
| path | 信息采集器的 HTTP 路径。 | string | false |
| scheme | 信息采集器的 HTTP 协议。 | string | false |
| timeoutSeconds | 信息采集器执行超时时间。 | int32 | false |
