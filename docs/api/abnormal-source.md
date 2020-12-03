# AbnormalSource API 设计

AbnormalSource 用于表示故障事件源实例。故障诊断恢复平台通过 AbnormalSource 提供以下功能：

* 注册故障事件源到故障诊断恢复平台。
* 基于故障事件源中定义的 Prometheus 报警模板和收到的 Prometheus 报警产生 Abnormal。
* 基于故障事件源中定义的 Kubernetes 事件模板和收到的 Kubernetes 事件产生 Abnormal。
* 记录故障事件源上一次产生的 Abnormal 元数据。

## AbnormalSource

AbnormalSource 用于表示故障事件源实例。

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata | API 资源元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |
| spec | 故障事件源的说明。 | [AbnormalSourceSpec](#abnormalsourcespec) | true |
| status | 故障事件源当前的状态。 | [AbnormalSourceStatus](#abnormalsourcestatus) | true |

## AbnormalSourceSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| sourceTemplate | 故障事件源生成 Abnormal 的模板。 | [SourceTemplate](#sourcetemplate) | true |
| assignedInformationCollectors | 用于指定生成 Abnormal 的信息采集器列表。 | [][NamespacedName](#namespacedname) | false |
| assignedDiagnosers | 用于指定生成 Abnormal 的故障诊断器列表。 | [][NamespacedName](#namespacedname) | false |
| assignedRecoverers | 用于指定生成 Abnormal 的故障恢复器列表。 | [][NamespacedName](#namespacedname) | false |
| commandExecutors | 用于指定生成 Abnormal 的命令执行器列表。 | [][CommandExecutor](#commandexecutor) | false |
| profilers | 用于指定生成 Abnormal 的性能剖析器目标行为列表。 | [][ProfilerSpec](#profilerspec) | false |
| context | 用于指定生成 Abnormal 的上下文信息。 | [runtime.RawExtension](https://github.com/kubernetes/apimachinery/blob/release-1.17/pkg/runtime/types.go#L94) | false |

## AbnormalSourceStatus

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| lastAbnormal | 故障事件源上一次产生的 Abnormal 元数据。 | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta) | false |

## SourceTemplate

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| type | 用于生成 Abnormal 模板的类型。 | string | true |
| prometheusAlertTemplate | Prometheus 报警模板。 | [PrometheusAlertTemplate](#prometheusalerttemplate) | false |
| kubernetesEventTemplate | Kubernetes 事件模板。 | [KubernetesEventTemplate](#kuberneteseventtemplate) | false |

## NamespacedName

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| namespace | API 资源的命名空间。 | string | false |
| name | API 资源的名称。 | string | true |

## PrometheusAlertTemplate

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| regexp | 用于匹配 Prometheus 报警的正则表达式。 | [PrometheusAlertTemplateRegexp](#prometheusalerttemplateregexp) | true |
| nodeNameReferenceLabel | 用于设置 NodeName 的 Label。 | string | true |

## PrometheusAlertTemplateRegexp

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| alertName | 用于匹配 AlertName 的正则表达式。 | string | false |
| labels | 用于匹配 Labels 的正则表达式集合。 | [model.LabelSet](https://github.com/prometheus/common/blob/v0.12.0/model/labelset.go#L28) | false |
| annotations | 用于匹配 Annotations 的正则表达式集合。 | [model.LabelSet](https://github.com/prometheus/common/blob/v0.12.0/model/labelset.go#L28) | false |
| startsAt | 用于匹配 StartsAt 的正则表达式。 | string | false |
| endsAt | 用于匹配 EndsAt 的正则表达式。 | string | false |
| generatorURL | 用于匹配 GeneratorURL 的正则表达式。 | string | false |

## KubernetesEventTemplate

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| regexp | 用于匹配 Kubernetes 事件的正则表达式。 | [KubernetesEventTemplateRegexp](#kuberneteseventtemplateregexp) | true |

## KubernetesEventTemplateRegexp

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | 用于匹配 Name 的正则表达式。 | string | false |
| namespace | 用于匹配 Namespace 的正则表达式。 | string | false |
| involvedObject | 用于匹配 InvolvedObject 的正则表达式。 | [corev1.ObjectReference](https://github.com/kubernetes/api/blob/release-1.17/core/v1/types.go#L4985) | false |
| reason | 用于匹配 Reason 的正则表达式。 | string | false |
| message | 用于匹配 Message 的正则表达式。 | string | false |
| source | 用于匹配 Source 的正则表达式。 | [corev1.EventSource](https://github.com/kubernetes/api/blob/release-1.17/core/v1/types.go#L5057) | false |
| firstTimestamp | 用于匹配 FirstTimestamp 的正则表达式。 | string | false |
| lastTimestamp | 用于匹配 LastTimestamp 的正则表达式。 | string | false |
| count | 用于匹配 Count 的正则表达式。 | string | false |
| type | 用于匹配 Type 的正则表达式。 | string | false |
| action | 用于匹配 Action 的正则表达式。 | string | false |
| reportingController | 用于匹配 ReportingController 的正则表达式。 | string | false |
| reportingInstance | 用于匹配 ReportingInstance 的正则表达式。 | string | false |

## CommandExecutor

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| command | 需要执行的命令。 | []string | true |
| type | 命令执行器的类型。该字段支持 InformationCollector、Diagnoser、Recoverer。 | string | true |
| stdout | 命令执行的标准输出。 | string | false |
| stderr | 命令执行的标准错误。 | string | false |
| error | 命令执行的错误。 | string | false |
| timeoutSeconds | 命令执行器执行超时时间。 | int32 | false |

## ProfilerSpec

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | 性能剖析器名称。 | string | true |
| type | 性能剖析器的类型。该字段支持 InformationCollector、Diagnoser、Recoverer。 | string | true |
| go | Go 语言性能剖析器。 | [GoProfiler](#goprofiler) | false |
| timeoutSeconds | 性能剖析器执行超时时间。 | int32 | false |

## GoProfiler

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| source | Go 语言性能剖析器源。通常是一个 HTTP 访问路径。 | string | true |
