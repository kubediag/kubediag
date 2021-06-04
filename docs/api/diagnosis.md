# Diagnosis API 设计

Diagnosis 是诊断过程中用于描述和管理故障的 API 对象。

## Diagnosis

| Field | Description | Scheme | Required |
|-|-|-|-|
| metadata | API 资源元数据。 | [ObjectMeta](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta) | false |
| spec | 用于定义诊断的期望状态。 | [DiagnosisSpec](#diagnosisspec) | false |
| status | 用于记录诊断的当前状态。 | [DiagnosisStatus](#diagnosisstatus) | false |

## DiagnosisSpec

| Field | Description | Scheme | Required |
|-|-|-|-|
| operationSet | 诊断过程中需要执行的 OperationSet 名称。OperationSet 是用于表示诊断流水线的 API 对象。 | string | true |
| nodeName | 诊断执行的节点。NodeName 和 PodReference 中至少需要指定一个。 | string | false |
| podReference | 目标 Pod 的详细信息。 | *[PodReference](#podreference) | false |
| parameters | 传递给诊断操作的参数集合。Parameters 和 OperationResults 会被序列化成 JSON 对象并在诊断执行过程中发送给诊断处理器。 | map[string]string | false |

## PodReference

| Field | Description | Scheme | Required |
|-|-|-|-|
| namespace | Pod 的命名空间。 | string | true |
| name | Pod 的名称。 | string | true |
| container | 容器的名称。 | string | false |

## DiagnosisStatus

| Field | Description | Scheme | Required |
|-|-|-|-|
| phase | 诊断的当前阶段。该字段支持 Pending、Running、Succeeded、Failed、Unknown。 | stirng | false |
| conditions | 描述诊断流程中关键点的状况。 | [][DiagnosisCondition](#diagnosiscondition) | false |
| startTime | 表示当前故障开始被诊断的时间。 | [Time](https://github.com/kubernetes/apimachinery/blob/release-1.17/pkg/apis/meta/v1/time.go#L33) | false |
| failedPaths | 包含诊断执行过程中所有失败的路径。 | [][][Node](./operation-set.md#node) | false |
| succeededPath | 诊断执行过程中成功的路径。 | [][Node](./operation-set.md#node) | false |
| operationResults | 包含诊断操作的结果。Parameters 和 OperationResults 会被序列化成 JSON 对象并在诊断执行过程中发送给诊断处理器。 | map[string]string | false |
| checkpoint | 用于恢复未完成诊断的检查点。 | *[Checkpoint](#checkpoint) | false |

## DiagnosisCondition

| Field | Description | Scheme | Required |
|-|-|-|-|
| type | 诊断状况的类型。 | string | true |
| status | 诊断状况的状态。该字段支持 True、False、Unknown。 | string | true |
| lastTransitionTime | 上一次状况的状态变化时间。 | [Time](https://github.com/kubernetes/apimachinery/blob/release-1.17/pkg/apis/meta/v1/time.go#L33) | false |
| reason | 表示当前状况的状态变化原因的简短信息。 | string | false |
| message | 表示当前状况的状态变化原因的可读信息。 | string | false |

## Checkpoint

| Field | Description | Scheme | Required |
|-|-|-|-|
| pathIndex | 诊断当前执行路径在 OperationSet 中的序号。 | int | true |
| nodeIndex | 诊断当前执行顶点在路径中的序号。 | int | true |
