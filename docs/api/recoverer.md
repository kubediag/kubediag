# Recoverer API 设计

Recoverer 用于表示故障恢复器实例。故障诊断恢复平台通过 Recoverer 提供以下功能：

* 接收 Abnormal 结构体。
* 注册故障恢复器到故障诊断恢复平台。
* 记录故障恢复器上一次恢复的状态。
* 对故障恢复器进行管理，Recoverer 中记录了故障恢复器的访问地址。故障恢复链通过请求故障恢复器的访问地址将当前的 Abnormal 结构体传递给故障恢复器，故障恢复器恢复后返回 Abnormal 结构体到故障恢复链。

## Recoverer

Recoverer 用于表示故障恢复器实例。

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | API 资源元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |
| spec | 故障恢复器的说明。 | [RecovererSpec](#recovererspec) | true |

## RecovererSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| externalIP | 外部故障恢复器的监听 IP。 | string | false |
| externalPort | 外部故障恢复器的监听端口。 | string | false |
| path | 故障恢复器的 HTTP 路径。 | string | false |
| scheme | 故障恢复器的 HTTP 协议。 | string | false |
| timeoutSeconds | 故障恢复器执行超时时间。 | int32 | false |
