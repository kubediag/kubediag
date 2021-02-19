# Go Profiler

Go 性能剖析器用于在节点上获取某个进程的性能剖析数据，用户可以自定义在故障诊断恢复流程某个阶段中执行性能剖析器。

## Go 性能剖析器支持的类型

* CPU：CPU 分析，按照一定的频率采集所监听的应用程序的 CPU 使用情况，可确定应用程序在主动消耗 CPU 周期时花费时间的位置。
* Heap：内存分析，在应用程序堆栈分配时记录跟踪，用于监视当前和历史内存使用情况，检查内存泄漏情况。
* Goroutine：Goroutine 分析，对所有当前 Goroutine 的堆栈跟踪。

## Go 性能剖析器的实现

Go 性能剖析器的实现逻辑如下：

1. 使用 HTTP Client 获取远程服务器的 `pprof` 路径下的性能分析文件。
1. 基于 `go tool pprof` 的命令解析性能分析文件，同时生成一个本地 HTTP 地址，将性能分析结果暴露于 HTTP Server 端。
1. 更新 `diagnosis.status` 的 Endpoint 为此 HTTP Server 的访问地址，用户可以通过浏览器访问性能分析结果。
1. 在 Go Profiler 功能正常启动后，此 Server 进程并不会长期保存，默认 `expirationSeconds` 后会被终止。同时 `diagnosis.status` 中的 Endpoint 也会被更新为 `expired`。

为保证在任何异常情况下，所有的 Command 子进程都能正确回收，以上使用的命令在运行时被设置为一个全新的进程组。

## 支持 Secret

若用户需要性能剖析的 Source 是 HTTPS 类型，需要指定 Secret。Go Profiler 性能剖析器通过获取 Secret 的 `data.token` 内容，作为 `curl` 的 Token 参数访问 HTTPS Server 以下载性能剖析文件。

## 垃圾回收

详见 [Garbage Collection](./garbage-collection.md)
