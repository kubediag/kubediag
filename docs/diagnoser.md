# Diagnoser API 设计

Diagnoser 用于表示故障分析器实例。故障诊断恢复平台通过 Diagnoser 提供以下功能：

* 接收 Abnormal 结构体。
* 注册故障分析器到故障诊断恢复平台。
* 记录故障分析器上一次分析的状态。
* 对故障分析器进行管理，Diagnoser 中记录了故障分析器的访问地址。故障分析链通过请求故障分析器的访问地址将当前的 Abnormal 结构体传递给故障分析器，故障分析器分析后返回 Abnormal 结构体到故障分析链。

## Diagnoser

Diagnoser 用于表示故障分析器实例。

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | API 资源元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |
| spec | 故障分析器的说明。 | [DiagnoserSpec](#diagnoserspec) | true |
| status | 故障分析器当前的状态。 | [DiagnoserStatus](#diagnoserstatus) | true |

## DiagnoserSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| ip | 故障分析器的监听 IP。 | string | true |
| port | 故障分析器的监听端口。 | string | true |
| path | 故障分析器的 HTTP 路径。 | string | false |
| livenessProbe | 故障分析器的健康检查探针，探测失败时不会重建故障分析器。 | [v1core.Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#probe-v1-core) | false |
| readinessProbe | 故障分析器的就绪检查探针，探测失败时不会发送请求到故障分析器。 | [v1core.Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#probe-v1-core) | false |
| scheme | 故障分析器的 HTTP 协议。 | string | false |
| timeoutSeconds | 故障分析器执行超时时间。 | int32 | false |

## DiagnoserStatus

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| ready | 故障分析器是否就绪。 | bool | true |
| healthy | 故障分析器是否健康。 | bool | true |
| lastDiagnosis | 故障分析器上次进行分析详情。 | [Diagnosis](#diagnosis) | false |

## Diagnosis

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| startTime | 分析开始的时间。 | [metav1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta) | false |
| endTime | 分析结束的时间。 | [metav1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta) | false |
| abnormal | 分析的故障。 | [Abnormal](#abnormal) | false |
