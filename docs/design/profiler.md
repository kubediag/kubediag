# Profiler

性能剖析器用于在节点上获取某个进程的性能剖析数据，用户可以自定义在故障诊断恢复流程某个阶段中执行的性能剖析器。

## API 设计

Abnormal 中的 `.spec.profilers` 字段用于自定义需要执行的性能剖析器，性能剖析执行的结果会被同步到 `.status.profilers` 字段。`profilers` 是一个包含多个 `profiler` 的列表，`profiler` 的字段包括：

* `name`：性能剖析器名称。
* `type`：性能剖析器的类型，该字段支持 InformationCollector、Diagnoser、Recoverer。InformationCollector、Diagnoser 和 Recoverer 类型分别在故障的 `InformationCollecting`、`Diagnosing` 和 `Recovering` 阶段被执行。信息管理器、故障分析链和故障恢复链在执行完相应类型的性能剖析器后才会执行 `InformationCollector`、`Diagnoser` 和 `Recoverer`。性能剖析器的执行结果不会影响故障的状态迁移，例如 Recoverer 类型的性能剖析器失败后故障的状态仍然可以被标记为 `Succeeded`。
* `go`：Go 语言性能剖析器。
* `error`：性能剖析的错误，如果性能剖析执行成功该字段则为空。
* `timeoutSeconds`：性能剖析器执行超时时间。如果性能剖析未在超时时间内执行完成，则 `error` 字段会被更新并且执行该性能剖析的进程会被终止。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.profilers` 字段中包含需要执行的性能剖析，一个典型的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: go-profiler
spec:
  source: Custom
  skipInformationCollection: true
  skipDiagnosis: true
  skipRecovery: true
  profilers:
  - name: go-profiler
    type: InformationCollector
    go:
      source: http://127.0.0.1:8090/debug/pprof/heap
    timeoutSeconds: 300
  nodeName: 10.177.16.22
```

该 Abnormal 定义了一个需要执行的 Go 语言内存性能剖析，Go 语言程序的内存性能剖析数据访问地址为 `http://127.0.0.1:8090/debug/pprof/heap`。

性能剖析的执行结果结果会被同步到 `.status.profilers` 字段：

```yaml
status:
  profilers:
  - endpoint: 0.0.0.0:41609
    go:
      source: http://127.0.0.1:8090/debug/pprof/heap
    name: go-profiler
    timeoutSeconds: 300
    type: InformationCollector
  recoverable: true
  startTime: "2020-09-10T09:30:59Z"
```

性能剖析执行后可以通过 Abnormal 所在节点的 `0.0.0.0:41609` 地址访问性能剖析数据。
