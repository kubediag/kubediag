# Operation API 设计

Operation 是用于定义诊断操作的 API 对象。

## Operation

| Field | Description | Scheme | Required |
|-|-|-|-|
| metadata | API 资源元数据。 | [ObjectMeta](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta) | false |
| spec | 用于定义诊断操作的期望状态。 | [OperationSpec](#operationspec) | false |

## OperationSpec

| Field | Description | Scheme | Required |
|-|-|-|-|
| processor | 用于描述如何在 KubeDiag 中注册一个诊断操作处理器。 | [Processor](#processor) | true |
| dependences | Dependences 是所有被依赖且必须预先执行的诊断操作名称列表。 | []string | false |
| storage | 表示操作处理结果的存储类型。如果该字段为空，那么操作处理结果不会被保存。 | *[Storage](#storage) | false |

## Processor

| Field | Description | Scheme | Required |
|-|-|-|-|
| externalAddress | 诊断操作处理器的监听地址。如果该字段为空，那么默认为 KubeDiag Agent 的地址。 | *string | false |
| externalPort | 诊断操作处理器的服务端口。如果该字段为空，那么默认为 KubeDiag Agent 的端口。 | *int32 | false |
| path | 服务的 HTTP 路径。如果该字段为空，那么默认为 `/`。 | *string | false |
| scheme | 处理请求的协议。默认为 `http`，可以是 `http` 或 `https`。 | *string | false |
| timeoutSeconds | 处理请求超时的秒数。默认为 30 秒，最小值为 1。 | *int32 | false |

## Storage

| Field | Description | Scheme | Required |
|-|-|-|-|
| hostPath | 宿主机上的目录。 | *[HostPath](#hostpath) | false |

## HostPath

| Field | Description | Scheme | Required |
|-|-|-|-|
| path | 宿主机上目录的路径。 | string | false |
