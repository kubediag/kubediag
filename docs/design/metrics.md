# Metrics

故障诊断恢复平台实现了下列 Prometheus 指标项以提升可观测性。

## DiagnosisReaper

| Name | Description | Type |
| ---- | ----------- | ---- |
| diagnosis_garbage_collection_cycle_count | 垃圾回收 Diagnosis 的周期次数。 | Counter |
| diagnosis_garbage_collection_success_count | 垃圾回收 Diagnosis 成功的个数。 | Counter |
| diagnosis_garbage_collection_error_count | 垃圾回收 Diagnosis 错误的个数。 | Counter |

## AlertManager

| Name | Description | Type |
| ---- | ----------- | ---- |
| prometheus_alert_received_count | Alertmanager 接收 Prometheus 报警的次数。 | Counter |
| alertmanager_diagnosis_generation_success_count | Alertmanager 生成 Diagnosis 成功的次数。 | Counter |
| alertmanager_diagnosis_generation_error_count | Alertmanager 生成 Diagnosis 错误的次数。 | Counter |

## Eventer

| Name | Description | Type |
| ---- | ----------- | ---- |
| event_received_count | Eventer 接收 Event 的次数。 | Counter |
| eventer_diagnosis_generation_success_count | Eventer 生成 Diagnosis 成功的次数。 | Counter |
| eventer_diagnosis_generation_error_count | Eventer 生成 Diagnosis 错误的次数。 | Counter |

## Kafka

| Name | Description | Type |
| ---- | ----------- | ---- |
| kafka_received_count | Kafka 接收 Message 的次数。 | Counter |
| kafka_diagnosis_generation_success_count | Kafka 生成 Diagnosis 成功的次数。 | Counter |
| kafka_diagnosis_generation_error_count | Kafka 生成 Diagnosis 错误的次数。 | Counter |

## Executor

| Name | Description | Type |
| ---- | ----------- | ---- |
| executor_sync_success_count | Executor 同步 Diagnosis 成功的次数。 | Counter |
| executor_sync_skip_count | Executor 跳过同步 Diagnosis 的次数。 | Counter |
| executor_sync_fail_count | Executor 同步 Diagnosis 失败的次数。 | Counter |
| executor_sync_error_count | Executor 同步 Diagnosis 错误的次数。 | Counter |
| executor_operation_error_counter | Operation 处理 Diagnosis 错误的次数 | Counter |
| executor_operation_success_counter | Operation 处理 Diagnosis 成功的次数 | Counter |
| executor_operation_fail_counter | Operation 处理 Diagnosis 失败的次数 | Counter |

## GraphBuilder

| Name | Description | Type |
| ---- | ----------- | ---- |
| graphbuilder_sync_success_count | Graphbuilder 同步 Operationset 成功的次数。 | Counter |
| graphbuilder_sync_skip_count | Graphbuilder 跳过同步 Operationset 的次数 | Counter |
| graphbuilder_sync_error_count | Graphbuilder 同步 Operationset 错误的次数。 | Counter |

## KubeDiag Master

| Name | Description | Type |
| ---- | ----------- | ---- |
| diagnosis_master_skip_count | KubeDiag Master 跳过同步 Diagnosis 的次数 | Counter |
| diagnosis_master_assign_node_count | KubeDiag Master 指定 Diagnosis 到节点的次数 | Counter |
| diagnosis_total_count | 集群中创建过的 Diagnosis 的数量。 | Counter |
| diagnosis_total_success_count | 集群中创建过的成功的 Diagnosis 的数量。 | Counter |
| diagnosis_total_fail_count | 集群中创建过的失败的 Diagnosis 的数量。 | Counter |
| diagnosis_info | Diagnosis 的信息。 | Gauge |

## KubeDiag Agent

| Name | Description | Type |
| ---- | ----------- | ---- |
| diagnosis_agent_skip_count | KubeDiag Agent 跳过同步 Diagnosis 的次数 | Counter |
| diagnosis_agent_queued_count | KubeDiag Agent 指派 Diagnosis 进入同步队列的次数 | Counter |

## Feature Gate

| Name | Description | Type |
| ---- | ----------- | ---- |
| feature_gate | Feature 的启用状态 | Gauge |

## Operation Controller

| Name | Description | Type |
| ---- | ----------- | ---- |
| operation_info | Operation 的信息 | Gauge |

## Operationset Controller

| Name | Description | Type |
| ---- | ----------- | ---- |
| operationset_info | Operationset 的信息 | Gauge |

## Trigger Controller

| Name | Description | Type |
| ---- | ----------- | ---- |
| trigger_info | Trigger 的信息 | Gauge |
