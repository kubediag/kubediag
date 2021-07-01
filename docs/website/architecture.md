# 架构

KubeDiag 是一个用于编排诊断运维操作的框架，主要由 KubeDiag Master 和 KubeDiag Agent 两个部分组成。KubeDiag Master 主要负责下列工作：

* 基于 OperationSet 计算出所有的诊断运维路径。
* 接收 Prometheus、Kafka 等系统发送的消息并根据 Trigger 中的模板创建 Diagnosis 自动触发诊断运维流水线。

KubeDiag Agent 主要负责下列工作：

* 执行 OperationSet 中的诊断运维路径并记录执行的状况。
* 将运维操作的结果记录到 Diagnosis 中。

![Architecture](../images/kubediag-architecture.png)

## Master

KubeDiag Master 主要由图构建器、Prometheus 报警管理器、Kafka 消息管理器和事件管理器组成：

* 图构建器（GraphBuilder）基于 OperationSet 计算出所有的诊断运维路径并更新至 Status 中。
* Prometheus 报警管理器（Alertmanager）接收 Prometheus 报警并与 Trigger 中定义的模板进行匹配，如果匹配成功则根据 Trigger 创建 Diagnosis。
* Kafka 消息管理器（KafkaConsumer）接收 Kafka 消息并创建 Diagnosis。
* 事件管理器（Eventer）接收 Kubernetes Event 并与 Trigger 中定义的模板进行匹配，如果匹配成功则根据 Trigger 创建 Diagnosis。

## Agent

KubeDiag Agent 主要由执行器组成：

* 执行器（Executor）负责执行诊断运维路径中的操作并将运维操作的结果和执行的状况更新至 Diagnosis 的 Status 中。
