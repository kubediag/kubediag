# Abnormal API 设计

Abnormal 是故障诊断恢复平台中故障事件源、故障分析链、故障恢复链之间通信的接口。故障诊断恢复平台通过 Abnormal 提供以下功能：

* 记录故障现象和来源，故障事件源会在接收到故障事件后将现象和来源写入 AbnormalSpec 中。
* 维护故障恢复的状态机，故障事件源、故障分析链、故障恢复链会在对故障恢复后将结果更新到 AbnormalStatus 中。
* 在节点上或容器内执行探测指令，如运行命令或者发送 HTTP 请求，并将结果输出到 AbnormalStatus 中。
* 故障分析链将 Abnormal 逐个发送至故障分析器，故障分析器分析后输出 Abnormal，故障分析链对输出的 Abnormal 进行验证后决定下一步流程。如果 Abnormal 被成功识别则更新 AbnormalStatus 并将 Abnormal 发往故障恢复链。如果无法识别或者发生错误则更新 AbnormalStatus 并等待人工干预。
* 故障恢复链将 Abnormal 逐个发送至故障恢复器，故障恢复器恢复后输出 Abnormal，故障恢复链对输出的 Abnormal 进行验证后决定下一步流程。如果 Abnormal 被成功恢复则更新 AbnormalStatus。如果无法恢复或者发生错误则更新 AbnormalStatus 并等待人工干预。

## Abnormal

Abnormal 是故障诊断恢复平台中故障事件源、故障分析链、故障恢复链之间通信的接口，用于描述故障。

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | API 资源元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |
| spec | 故障的来源和现象说明。支持用户自定义字段。 | [AbnormalSpec](#abnormalspec) | true |
| status | 故障当前的状态。由故障事件源、故障分析链、故障恢复链维护，用户无法自行修改。 | [AbnormalStatus](#abnormalstatus) | true |

## AbnormalSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| source | 故障的来源。该字段支持 Log、KubernetesEvent、PrometheusAlert、Probe 和 Custom。 | string | true |
| log | 表示故障的日志详细信息，对应 source 字段的 Log。 | [Log](#log) | false |
| kubernetesEvent | 表示故障的 Kubernetes Event 详细信息，对应 source 字段的 KubernetesEvent。 | [corev1.Event](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#event-v1-core) | false |
| prometheusAlert | 表示故障的 Prometheus Alert 详细信息，对应 source 字段的 PrometheusAlert。 | [PrometheusAlert](#prometheusalert) | false |
| nodeProbe | 用户自定义的节点故障探测 Probe，支持 Probe 类型故障。 | [NodeProbe](#nodeprobe) | false |
| podProbe | 用户自定义的容器故障探测 Probe，支持 Probe 类型故障。 | [PodProbe](#podprobe) | false |
| skipDiagnosis | 跳过故障分析步骤。 | bool | false |
| skipRecovery | 跳过故障恢复步骤。 | bool | false |
| nodeName | Abnormal 所在节点名。 | string | false |
| assignedDiagnosers | 指定进行诊断的故障诊断器列表。 | [][NamespacedName](#namespacedname) | false |
| assignedRecoverers | 指定进行恢复的故障恢复器列表。 | [][NamespacedName](#namespacedname) | false |
| assignedInformationCollectors | 指定进行信息采集的信息采集器列表。 | [][NamespacedName](#namespacedname) | false |
| context | 用于扩展的上下文信息，支持 Custom 类型故障。 | map[string]string | false |

## Log

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| filePath | 日志文件的绝对路径。 | string | true |
| logEntry | 日志中表示故障的条目。 | string | true |

## PrometheusAlert

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| labels | Alert 的标签。 | [labels.Labels](https://github.com/prometheus/prometheus/blob/v2.17.2/pkg/labels/labels.go#L42) | true |
| annotations | Alert 的注解。 | [labels.Labels](https://github.com/prometheus/prometheus/blob/v2.17.2/pkg/labels/labels.go#L42) | true |
| startsAt | 告警的开始时间 | [metav1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta) | false |
| endsAt | 告警的结束时间 | [metav1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta) | false |
| generatorURL | 告警生成者的 URL | string | false |

## NodeProbe

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | 执行 Probe 的 Node。 | string | true |
| timeoutSeconds | Probe 执行超时时间。 | int32 | false |
| exec | Exec 命令。 | [corev1.ExecAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#execaction-v1-core) | false |
| httpGet | HTTP 请求。 | [corev1.HTTPGetAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#httpgetaction-v1-core) | false |
| tcpSocket | TCP 探活。 | [corev1.TCPSocketAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#tcpsocketaction-v1-core) | false |

## PodProbe

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| namespace | 执行 Probe 的 Pod 命名空间。 | string | true |
| name | 执行 Probe 的 Pod。 | string | true |
| container | 执行 Probe 的容器。 | string | true |
| timeoutSeconds | Probe 执行超时时间。 | int32 | false |
| exec | Exec 命令。 | [corev1.ExecAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#execaction-v1-core) | false |
| httpGet | HTTP 请求。 | [corev1.HTTPGetAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#httpgetaction-v1-core) | false |
| tcpSocket | TCP 探活。 | [corev1.TCPSocketAction](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#tcpsocketaction-v1-core) | false |

## AbnormalStatus

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| identifiable | 表示该故障为可以被故障分析器识别的故障。 | bool | true |
| recoverable | 表示该故障为可以被故障恢复器恢复的故障。 | bool | true |
| conditions | 描述故障恢复流程中关键点的状况。 | [][AbnormalCondition](#abnormalcondition) | false |
| phase | 故障的当前阶段。该字段支持 Diagnosing、Recovering、Succeeded、Failed、Unknown。 | string | false |
| message | 表示当前故障恢复阶段的可读信息。用于输出故障原因、故障恢复建议等。 | string | false |
| reason | 表示当前故障恢复阶段的简短信息。 | string | false |
| output | Exec 命令、HTTP 请求、TCP 探活的输出。 | string | false |
| startTime | 表示当前故障开始被诊断的时间。 | [metav1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta) | false |
| diagnoser | 成功执行的故障诊断器。 | NamespacedName | false |
| recoverer | 成功执行的故障恢复器。 | NamespacedName | false |
| context | 用于扩展的上下文信息，支持 Custom 类型故障。 | map[string]string | false |

## AbnormalCondition

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| type | 故障状况的类型。 | string | true |
| status | 故障状况的状态。该字段支持 True、False、Unknown。 | string | true |
| lastTransitionTime | 上一次状况的状态变化时间。 | [metav1.Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta) | false |
| message | 表示当前状况的状态变化原因的可读信息。 | string | false |
| reason | 表示当前状况的状态变化原因的简短信息。 | string | false |

## NamespacedName

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| namespace | API 资源的命名空间。 | string | false |
| name | API 资源的名称。 | string | true |
