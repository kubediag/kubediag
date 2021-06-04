# OperationSet API 设计

OperationSet 是用于定义诊断流水线的 API 对象。

## OperationSet

| Field | Description | Scheme | Required |
|-|-|-|-|
| metadata | API 资源元数据。 | [ObjectMeta](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta) | false |
| spec | 用于定义诊断流水线的期望状态。 | [OperationSetSpec](#operationsetspec) | false |
| status | 用于记录诊断流水线的当前状态。 | [OperationSetStatus](#operationsetstatus) | false |

## OperationSetSpec

| Field | Description | Scheme | Required |
|-|-|-|-|
| adjacencyList | 包含有向无环图中所有表示诊断操作的顶点。数组的第一个顶点表示诊断开始而不是某个特定的诊断操作。 | [][Node](#node) | true |

## Node

| Field | Description | Scheme | Required |
|-|-|-|-|
| id | 顶点的唯一标识符。必须与 AdjacencyList 中顶点的序号相同，通常由 Admission Webhook 自动设置。 | int | false |
| to | 从该顶点能够直接到达的顶点序号列表。 | []int | false |
| operation | 在该顶点执行的 Operation 名称。用于表示诊断开始的顶点该字段为空。 | string | false |
| dependences | Dependences 是所有被当前顶点依赖且必须预先执行的顶点序号列表。 | []int | false |

## OperationSetStatus

| Field | Description | Scheme | Required |
|-|-|-|-|
| paths | 表示诊断流水线的有向无环图中所有诊断路径的集合。 | [][][Node](#node) | false |
| ready | 定义中提供的顶点是否能生成合法的有向无环图。 | bool | true |
