# Profiler

性能剖析器用于在节点上获取某个进程的性能剖析数据，用户可以自定义在故障诊断恢复流程某个阶段中执行的性能剖析器。

## API 设计

Abnormal 中的 `.spec.profilers` 字段用于自定义需要执行的性能剖析器，性能剖析执行的结果会被同步到 `.status.profilers` 字段。`.spec.profilers` 是一个包含多个 `ProfilerSpec` 的列表，`ProfilerSpec` 的字段包括：

* `name`：性能剖析器名称。
* `type`：性能剖析器的类型，该字段支持 InformationCollector、Diagnoser、Recoverer。InformationCollector、Diagnoser 和 Recoverer 类型分别在故障的 `InformationCollecting`、`Diagnosing` 和 `Recovering` 阶段被执行。信息管理器、故障分析链和故障恢复链在执行完相应类型的性能剖析器后才会执行 `InformationCollector`、`Diagnoser` 和 `Recoverer`。性能剖析器的执行结果不会影响故障的状态迁移，例如 Recoverer 类型的性能剖析器失败后故障的状态仍然可以被标记为 `Succeeded`。
* `go`：Go 语言性能剖析器的目标状态。
* `java`：Java 语言性能剖析器的目标状态。
* `timeoutSeconds`：性能剖析器执行超时时间。如果性能剖析未在超时时间内执行完成，则 `error` 字段会被更新并且执行该性能剖析的进程会被终止。
* `expirationSeconds`：性能剖析器服务过期时间。性能剖析的结果会通过一个随机服务地址暴露，超过该时间后该性能剖析的服务进程会被终止。

Abnormal 中的 `.status.profilers` 是一个包含多个 `ProfilerStatus` 的列表，`ProfilerStatus` 的字段包括：

* `name`：性能剖析器名称，与 `ProfilerSpec` 保持一致。
* `type`：性能剖析器的类型，与 `ProfilerSpec` 保持一致。
* `endpoint`：用于暴露性能剖析结果的服务地址。如果性能剖析器服务过期，则该字段被更新为 `expired`。
* `error`：性能剖析的错误，如果性能剖析执行成功该字段则为空。

### Go 语言性能剖析器

Go 语言性能剖析的目标状态被定义在 `GoProfilerSpec` 中，`GoProfilerSpec` 的字段包括：

* `source`：Go 语言性能剖析数据的获取地址，必须是一个暴露 `pprof` 数据接口。

### Java 语言性能剖析器

Java 语言性能剖析器包括以下类型：

* `Arthas`：使用 Arthas 对 Java 程序进行分析。
* `MemoryAnalyzer`：使用 Eclipse Memory Analyzer 对 Java 程序进行分析。

Java 语言性能剖析的目标状态被定义在 `JavaProfilerSpec` 中，`JavaProfilerSpec` 的字段包括：

* `type`：Java 语言性能剖析器的类型，该字段支持 Arthas 和 MemoryAnalyzer。
* `hprofFilePath`：HPROF 文件的绝对路径，MemoryAnalyzer 类型必须指定。

## 如何使用 Go 语言性能剖析器

用户可以创建 Abnormal 并在 `.spec.profilers` 字段中包含需要执行的 Go 语言性能剖析器，一个典型的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: go-profiler
spec:
  source: Custom
  profilers:
  - expirationSeconds: 7200
    go:
      source: http://127.0.0.1:8090/debug/pprof/heap
    name: go-profiler
    timeoutSeconds: 300
    type: InformationCollector
  nodeName: 10.177.16.22
```

该 Abnormal 定义了一个需要执行的 Go 语言内存性能剖析。Go 语言程序的性能剖析数据访问地址为 `http://127.0.0.1:8090/debug/pprof/heap`，性能剖析执行的超时时间为300秒。性能剖析的执行结果会被同步到 `.status.profilers` 字段：

```yaml
status:
  profilers:
  - endpoint: 10.177.16.22:41609
    name: go-profiler
    type: InformationCollector
  recoverable: true
  startTime: "2020-09-10T09:30:59Z"
```

性能剖析执行后可以通过 `10.177.16.22:41609` 地址访问性能剖析结果，该性能剖析的服务进程会在7200秒后终止。终止后 `.status.profilers` 字段会被更新：

```yaml
status:
  profilers:
  - endpoint: expired
    name: go-profiler
    type: InformationCollector
  recoverable: true
  startTime: "2020-09-10T09:30:59Z"
```

## 如何使用 Java 语言性能剖析器

用户可以创建 Abnormal 并在 `.spec.profilers` 字段中包含需要执行的 Java 语言性能剖析器，一个典型的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: java-profiler
spec:
  source: Custom
  profilers:
  - expirationSeconds: 7200
    java:
      hprofFilePath: /dump/heap.hprof
      type: MemoryAnalyzer
    name: java-profiler
    timeoutSeconds: 300
    type: InformationCollector
  podReference:
    namespace: default
    name: java-app
  nodeName: 10.177.16.22
```

该 Abnormal 定义了一个需要执行的 Java 语言内存性能剖析。该性能剖析的目标 Pod 为 `java-app`，目标 Pod 所在节点为 `10.177.16.22`，性能剖析类型为 `MemoryAnalyzer`，性能剖析执行的超时时间为300秒。需要分析的 HPROF 文件在节点上的绝对路径为 `/dump/heap.hprof`。性能剖析的执行结果会被同步到 `.status.profilers` 字段：

```yaml
status:
  phase: Succeeded
  profilers:
  - endpoint: 10.177.16.22:44935
    name: java-profiler
    type: InformationCollector
  startTime: "2020-12-14T06:08:20Z"
```

性能剖析执行后可以通过 `10.177.16.22:44935` 地址访问性能剖析结果，该性能剖析的服务进程会在7200秒后终止。终止后 `.status.profilers` 字段会被更新：

```yaml
status:
  phase: Succeeded
  profilers:
  - endpoint: expired
    name: java-profiler
    type: InformationCollector
  startTime: "2020-12-14T06:08:20Z"
```
