# Trigger API 设计

Operation 是用于定义如何通过外部系统触发诊断并创建 Diagnosis 的 API 对象。

## Trigger

| Field | Description | Scheme | Required |
|-|-|-|-|
| metadata | API 资源元数据。 | [ObjectMeta](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta) | false |
| spec | 用于定义诊断触发器的期望状态。 | [TriggerSpec](#triggerspec) | false |

## TriggerSpec

| Field | Description | Scheme | Required |
|-|-|-|-|
| operationSet | 产生的 Diagnosis 中需要执行的 OperationSet 名称。 | string | true |
| sourceTemplate | 用于生成 Diagnosis 的模板。 | [SourceTemplate](#sourcetemplate) | true |

## SourceTemplate

| Field | Description | Scheme | Required |
|-|-|-|-|
| prometheusAlertTemplate | 根据 Prometheus 报警生成 Diagnosis 的模板。 | [PrometheusAlertTemplate](#prometheusalerttemplate) | false |
| kubernetesEventTemplate | 根据 Kubernetes 事件生成 Diagnosis 的模板。 | [KubernetesEventTemplate](#kuberneteseventtemplate) | false |

## PrometheusAlertTemplate

| Field | Description | Scheme | Required |
|-|-|-|-|
| regexp | 包含用于匹配 Prometheus 报警模板的正则表达式。 | [PrometheusAlertTemplateRegexp](#prometheusalerttemplateregexp) | true |
| nodeNameReferenceLabel | 用于设置 `.spec.nodeName` 字段的标签键。 | string | false |
| podNamespaceReferenceLabel | 用于设置 `.spec.podReference.namespace` 字段的标签键。 | string | false |
| podNameReferenceLabel | 用于设置 `.spec.podReference.name` 字段的标签键。 | string | false |
| containerReferenceLabel | 用于设置 `.spec.podReference.container` 字段的标签键。 | string | false |
| parameterInjectionLabels | 需要注入到 `.spec.parameters` 字段的标签键列表。 | []string | false |

## PrometheusAlertTemplateRegexp

| Field | Description | Scheme | Required |
|-|-|-|-|
| alertName | 用于匹配 AlertName 的正则表达式。 | string | false |
| labels | 用于匹配 Labels 的正则表达式集合。 | map[string]string | false |
| annotations | 用于匹配 Annotations 的正则表达式集合。 | map[string]string | false |
| startsAt | 用于匹配 StartsAt 的正则表达式。 | string | false |
| endsAt | 用于匹配 EndsAt 的正则表达式。 | string | false |
| generatorURL | 用于匹配 GeneratorURL 的正则表达式。 | string | false |

## KubernetesEventTemplate

| Field | Description | Scheme | Required |
|-|-|-|-|
| regexp | 包含用于匹配 Kubernetes 事件模板的正则表达式。 | [KubernetesEventTemplateRegexp](#kuberneteseventtemplateregexp) | true |

## KubernetesEventTemplateRegexp

| Field | Description | Scheme | Required |
|-|-|-|-|
| name | 用于匹配 Name 的正则表达式。 | string | false |
| namespace | 用于匹配 Namespace 的正则表达式。 | string | false |
| reason | 用于匹配 Reason 的正则表达式。 | string | false |
| message | 用于匹配 Message 的正则表达式。 | string | false |
| source | 用于匹配 Source 的正则表达式。 | [EventSource](https://github.com/kubernetes/api/blob/release-1.17/core/v1/types.go#L5057) | false |
