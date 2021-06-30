# 创建运维流水线

我们可以对已注册的诊断操作进行编排以组成运维流水线，现在让我们将之前注册的诊断操作制作成流水线。

## 编写运维流水线

OperationSet 是用于声明诊断流水线的自定义资源，通过 OperationSet 您可以定义诊断流水线中需要执行的诊断操作。下列 OperationSet 中定义了一个包含之前已注册诊断操作的诊断流水线：

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: http-operation
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: http-operation
```

OperationSet 中的 `.spec.adjacencyList` 字段包含了一个有向无环图，其中 `id` 为 0 的顶点表示诊断流水线的开始，其他顶点表示一个诊断操作。

## 将运维流水线注册到 KubeDiag 中

通过创建上述 OperationSet 注册诊断流水线：

```bash
kubectl apply -f samples/http-operation/manifests/operationset.yaml
```

查看 OperationSet 的状态：

```bash
$ kubectl get operationset http-operation -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  labels:
    adjacency-list-hash: 864dcbdbfb
  name: http-operation
  resourceVersion: "2002860"
  selfLink: /apis/diagnosis.kubediag.org/v1/operationsets/http-operation
  uid: 34886fc6-4a4f-484c-b965-22e51d12dea0
spec:
  adjacencyList:
  - to:
    - 1
  - id: 1
    operation: http-operation
status:
  paths:
  - - id: 1
      operation: http-operation
  ready: true
```

您可能已经注意到 `.status.ready` 字段为 `true` 并且 `.status.paths` 字段中包含一条诊断路径，该状态表示 OperationSet 被 KubeDiag 成功接受。
