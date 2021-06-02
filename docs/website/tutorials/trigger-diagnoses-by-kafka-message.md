# 通过 Kafka 消息触发诊断

本文介绍了如何通过 Kafka 消息创建 Diagnosis 来触发诊断。

## 开始之前

在教程开始前，您需要确定 Kubernetes 集群中已经正确安装 Kube Diagnoser。

## 在 Kube Diagnoser Master 参数中指定需要消费的 Kafka 和 Topic

您需要在 Kube Diagnoser Master 启动时指定下列参数以使用该功能：

| 参数 | 描述 | 默认值 | 示例 |
|-|-|-|-|
| --kafka-brokers strings | 需要连接 Kafka 集群的 Broker 地址列表。 | "" | "my-cluster-kafka-0:9092,my-cluster-kafka-1:9092" |
| --kafka-topic string | 获取消息的 Topic。 | "" | "my-topic" |

如果上述参数均未指定，则通过 Kafka 消息触发诊断的功能不开启。

## Kafka 消息格式

Kafka 消息中必须包含用于创建 Diagnosis 的元信息，一个能够触发诊断的 Kafka 消息的 Value 必须是一个 JSON 对象，且对象中的键值对必须均为 String 类型。JSON 对象中支持的键值对包括：

| 字段 | 描述 | 必须指定 |
|-|-|-|
| operationset | 用于指定被创建 Diagnosis 的 `.spec.operationSet` 字段。 | 是 |
| node | 用于指定被创建 Diagnosis 的 `.spec.nodeName` 字段。 | 是 |
| pod | 用于指定被创建 Diagnosis 的 `.spec.podReference.name` 字段。 | 否 |
| namespace | 用于指定被创建 Diagnosis 的 `.spec.podReference.namespace` 字段。 | 否 |
| container | 用于指定被创建 Diagnosis 的 `.spec.podReference.container` 字段。 | 否 |

JSON 对象中的所有键值对会被注入到生成 Diagnosis 的 `.spec.parameters` 字段。

## 举例说明

当 Kube Diagnoser 接收到包含下列 Value 的 Kafka 消息时会根据 JSON 对象创建 Diagnosis：

```json
{
    "operationset": "my-operationset",
    "node": "my-node",
    "pod": "my-pod",
    "namespace": "default",
    "container": "my-container",
    "key1": "value1",
    "key2": "value2"
}
```

通过该 Kafka 消息创建出的 Diagnosis 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Diagnosis
metadata:
  annotations:
    diagnosis.netease.com/kafka-message-headers: ""
    diagnosis.netease.com/kafka-message-key: ""
    diagnosis.netease.com/kafka-message-offset: "7"
    diagnosis.netease.com/kafka-message-partition: "0"
    diagnosis.netease.com/kafka-message-time: "20210603085224"
    diagnosis.netease.com/kafka-message-topic: my-topic
    diagnosis.netease.com/kafka-message-value: '{"operationset":"my-operationset","node":"my-node","pod":"my-pod","namespace":"default","container":"my-container","key1":"value1","key2":"value2"}'
  name: kafka-message.20210603085224
  namespace: kube-diagnoser
spec:
  nodeName: my-node
  operationSet: my-operationset
  parameters:
    container: my-container
    key1: value1
    key2: value2
    namespace: default
    node: my-node
    operationset: my-operationset
    pod: my-pod
  podReference:
    container: my-container
    name: my-pod
    namespace: default
```
