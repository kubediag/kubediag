# Metrics

故障诊断恢复平台实现了下列 Prometheus 指标项以提升可观测性。

## AbnormalReaper

| Name | Description | Type |
| ---- | ----------- | ---- |
| abnormal_garbage_collection_success_count | 垃圾回收 Abnormal 成功的次数。 | Counter |
| abnormal_garbage_collection_error_count | 垃圾回收 Abnormal 错误的次数。 | Counter |
| node_abnormal_count | 当前节点 Abnormal 的数量。 | Gauge |

## Alertmanager

| Name | Description | Type |
| ---- | ----------- | ---- |
| prometheus_alert_received_count | Alertmanager 接收 Prometheus 报警的次数。 | Counter |
| alertmanager_abnormal_generation_success_count | Alertmanager 生成 Abnormal 成功的次数。 | Counter |
| alertmanager_abnormal_generation_error_count | Alertmanager 生成 Abnormal 错误的次数。 | Counter |

## Eventer

| Name | Description | Type |
| ---- | ----------- | ---- |
| event_received_count | Eventer 接收 Event 的次数。 | Counter |
| eventer_abnormal_generation_success_count | Eventer 生成 Abnormal 成功的次数。 | Counter |
| eventer_abnormal_generation_error_count | Eventer 生成 Abnormal 错误的次数。 | Counter |

## SourceManager

| Name | Description | Type |
| ---- | ----------- | ---- |
| source_manager_sync_success_count | SourceManager 同步 Abnormal 成功的次数。 | Counter |
| source_manager_sync_error_count | SourceManager 同步 Abnormal 错误的次数。 | Counter |
| prometheus_alert_generated_abnormal_creation_count | SourceManager 基于 Prometheus 报警创建 Abnormal 成功的次数。 | Counter |
| event_generated_abnormal_creation_count | SourceManager 基于 Event 创建 Abnormal 成功的次数。 | Counter |

## InformationManager

| Name | Description | Type |
| ---- | ----------- | ---- |
| information_manager_sync_success_count | InformationManager 同步 Abnormal 成功的次数。 | Counter |
| information_manager_sync_skip_count | InformationManager 跳过同步 Abnormal 的次数 | Counter |
| information_manager_sync_fail_count | InformationManager 同步 Abnormal 失败的次数。 | Counter |
| information_manager_sync_error_count | InformationManager 同步 Abnormal 错误的次数。 | Counter |
| information_manager_command_executor_success_count | InformationManager 运行命令执行器成功的次数。 | Counter |
| information_manager_command_executor_fail_count | InformationManager 运行命令执行器失败的次数。 | Counter |
| information_manager_profiler_success_count | InformationManager 运行性能剖析器成功的次数。 | Counter |
| information_manager_profiler_fail_count | InformationManager 运行性能剖析器失败的次数。 | Counter |

## DiagnoserChain

| Name | Description | Type |
| ---- | ----------- | ---- |
| diagnoser_chain_sync_success_count | DiagnoserChain 同步 Abnormal 成功的次数。 | Counter |
| diagnoser_chain_sync_skip_count | DiagnoserChain 跳过同步 Abnormal 的次数 | Counter |
| diagnoser_chain_sync_fail_count | DiagnoserChain 同步 Abnormal 失败的次数。 | Counter |
| diagnoser_chain_sync_error_count | DiagnoserChain 同步 Abnormal 错误的次数。 | Counter |
| diagnoser_chain_command_executor_success_count | DiagnoserChain 运行命令执行器成功的次数。 | Counter |
| diagnoser_chain_command_executor_fail_count | DiagnoserChain 运行命令执行器失败的次数。 | Counter |
| diagnoser_chain_profiler_success_count | DiagnoserChain 运行性能剖析器成功的次数。 | Counter |
| diagnoser_chain_profiler_fail_count | DiagnoserChain 运行性能剖析器失败的次数。 | Counter |

## RecovererChain

| Name | Description | Type |
| ---- | ----------- | ---- |
| recoverer_chain_sync_success_count | RecovererChain 同步 Abnormal 成功的次数。 | Counter |
| recoverer_chain_sync_skip_count | RecovererChain 跳过同步 Abnormal 的次数 | Counter |
| recoverer_chain_sync_fail_count | RecovererChain 同步 Abnormal 失败的次数。 | Counter |
| recoverer_chain_sync_error_count | RecovererChain 同步 Abnormal 错误的次数。 | Counter |
| recoverer_chain_command_executor_success_count | RecovererChain 运行命令执行器成功的次数。 | Counter |
| recoverer_chain_command_executor_fail_count | RecovererChain 运行命令执行器失败的次数。 | Counter |
| recoverer_chain_profiler_success_count | RecovererChain 运行性能剖析器成功的次数。 | Counter |
| recoverer_chain_profiler_fail_count | RecovererChain 运行性能剖析器失败的次数。 | Counter |
