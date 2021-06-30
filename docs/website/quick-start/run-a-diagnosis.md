# 运行一次诊断

运维流水线已经准备就绪，我们现在可以运行流水线中定义的诊断。

## 编写诊断

Diagnosis 是用于声明诊断的自定义资源，通过 Diagnosis 您可以定义如何运行一个诊断。下列 Diagnosis 中定义运行之前已注册运维流水线的诊断：

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: http-operation
spec:
  operationSet: http-operation
  nodeName: my-node
  parameters:
    b: "200"
    f: "6"
```

Diagnosis 的定义中包含下列信息：

* `.spec.operationSet` 字段包含了表示运维流水线 OperationSet 的名称。即我们之前创建的 `http-operation` 运维流水线。
* `.spec.nodeName` 字段表示诊断运行的 Node。因为该运维流水线的运行与节点无关，这里填入任意节点即可。
* `.spec.parameters` 字段是诊断执行时传入的参数。所有参数均以 JSON 格式发送给 HTTP 诊断程序。

## 运行诊断

在运行诊断前，让我们通过下列命令查看当前诊断程序缓存中的所有数据：

```bash
$ curl -X POST --data '{}' http://10.96.73.28:80
{"a":"100","b":"2","c":"3","d":"4","e":"5"}
```

通过创建上述 Diagnosis 运行诊断：

```bash
kubectl apply -f samples/http-operation/manifests/diagnosis.yaml
```

查看 Diagnosis 的状态：

```bash
$ kubectl get diagnosis http-operation -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  labels:
    adjacency-list-hash: 864dcbdbfb
  name: http-operation
  namespace: default
  resourceVersion: "2053976"
  selfLink: /apis/diagnosis.kubediag.org/v1/namespaces/default/diagnoses/http-operation
  uid: 59f1aee9-e00c-4680-92c3-d1cee080f8dc
spec:
  nodeName: my-node
  operationSet: http-operation
  parameters:
    b: "200"
    f: "6"
status:
  checkpoint:
    nodeIndex: 0
    pathIndex: 0
  conditions:
  - lastTransitionTime: "2021-06-22T08:08:51Z"
    message: Diagnosis is accepted by agent on node my-node
    reason: DiagnosisAccepted
    status: "True"
    type: Accepted
  - lastTransitionTime: "2021-06-22T08:08:51Z"
    message: Diagnosis is completed
    reason: DiagnosisComplete
    status: "True"
    type: Complete
  operationResults:
    a: "100"
    b: "200"
    c: "3"
    d: "4"
    diagnosis.name: http-operation
    diagnosis.namespace: default
    diagnosis.uid: 59f1aee9-e00c-4680-92c3-d1cee080f8dc
    e: "5"
    f: "6"
    node: my-node
  phase: Succeeded
  startTime: "2021-06-22T08:08:01Z"
  succeededPath:
  - id: 1
    operation: http-operation
```

您可能已经注意到 `.status.phase` 字段为 `Succeeded` 并且 `.status.succeededPath` 字段中包含一条诊断路径，该状态表示诊断运行成功。诊断运行的结果会被记录到 `.status.operationResults` 字段，即 HTTP 诊断程序返回的缓存在更改后的所有数据。`.status.operationResults` 字段中还出现了一些诊断相关的元数据，这是因为这些元数据和 `.spec.parameters` 字段中传入的参数被 KubeDiag 一起发送至了 HTTP 诊断程序。

我们通过下列命令可以确定当前诊断程序缓存中的所有数据与 `.status.operationResults` 字段一致：

```bash
$ curl -X POST --data '{}' http://10.96.73.28:80
{"a":"100","b":"200","c":"3","d":"4","diagnosis.name":"http-operation","diagnosis.namespace":"default","diagnosis.uid":"59f1aee9-e00c-4680-92c3-d1cee080f8dc","e":"5","f":"6","node":"my-node"}
```
