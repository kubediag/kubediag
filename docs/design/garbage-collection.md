# 垃圾回收

KubeDiag 自动诊断系统运行中会根据集群监测状态或者用户行为生成一些资源或文件，需要实现自主垃圾回收。垃圾回收器每隔固定时长进行一次垃圾回收，清理不需要的 Diagnosis 资源、Java 性能剖析文件、Golang 性能剖析文件。

## Diagnosis 回收

Diagnosis 可以是由用户手动创建，也可以是 KubeDiag 系统自动诊断生成。Diagnosis 的诊断结果可能是 Succeeded、Failed、Unknown。Diagnosis 的回收考虑三种因素，MaximumDiagnosissPerNode、DiagnosisTTL、MinimumDiagnosisTTLDuration。

* 如果节点上的 Diagnosis 数量超过节点限额 MaximumDiagnosissPerNode，将 Diagnosis 的数量下调至阈值以下。
* 如果 Diagnosis 的存活时间超过上限阈值 DiagnosisTTL，触发垃圾回收。
* 如果 Diagnosis 的 Phase 是 Succeeded 或者 Failed，并且 Diagnosis 的存活时间超过 MinimumDiagnosisTTLDuration，触发垃圾回收。

## Golang 性能剖析文件回收

Golang 的性能剖析文件的回收考虑因素为存活时间上限阈值 DiagnosisTTL。Golang 的性能剖析文件从远端 Source 下载到本地后被持久化，持久化目录的存活时间超过上限阈值 DiagnosisTTL 则触发垃圾回收。

## Java 性能剖析文件回收

Java 的性能剖析文件的回收考虑因素为存活时间上限阈值 DiagnosisTTL，Java 的性能剖析文件的存储目录存活时间超过 DiagnosisTTL 时，触发垃圾回收。

## 用户配置

用户可以使用以下参数来优化垃圾回收：

* `maximum-Diagnosiss-per-node`：每个节点上保留的 Diagnosis 的最大数量，默认为 20。
* `diagnosis-ttl`：Diagnosis 资源、Golang 性能剖析文件、Java 性能剖析文件被垃圾回收前的最大保留时长，默认是 240 小时。
* `minimum-Diagnosis-ttl-duration`：Diagnosis 资源被垃圾回收前的最小保留时长，默认是 30 分钟。
