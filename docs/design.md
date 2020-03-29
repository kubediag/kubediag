# Kubernetes 故障诊断恢复平台架构设计

Kubernetes 是一个生产级的容器编排引擎，但是 Kubernetes 仍然存在系统复杂、故障诊断成本高等问题。Kubernetes 故障诊断恢复平台是基于 Kubernetes 云原生基础设施能力打造的框架，旨在解决云原生体系中故障诊断、运维恢复的自动化问题。

## 目标

Kubernetes 故障诊断恢复平台的设计目标包括：

* 通用性：平台依赖通用技术实现，平台组件可以在绝大部分的 Linux 系统下运行并且能够对 Linux 下运行遇到的故障进行诊断和运维。
* 可扩展性：平台组件之间的交互为松耦合接口设计并且整个框架是可插拔式的。
* 可维护性：框架逻辑简洁明了，维护成本与功能数量为线性关系，不同故障的分析和恢复逻辑具有独立性。

## 架构

Kubernetes 故障诊断恢复平台分为 Master 和 Agent 组件：

```bash
                                                          ---------------------
                                      Watch (Event, CRD)  |                   |
                                    --------------------->|     APIServer     |
                                    |                     |                   |
                                    |                     |-------------------|
                                    |                     |                   |
--------------                 ----------                 |       Etcd        |
|            |     Monitor     |        |     Monitor     |                   |
| Prometheus |<----------------| Master |---------------->|-------------------|
|            |                 |        |                 |                   |
--------------                 ----------                 | ControllerManager |
                                   /|\                    |                   |
                                    |                     |-------------------|
                                    |                     |                   |
                                    |                     |     Scheduler     |
                                    |                     |                   |
                                    |                     ---------------------
                     -------------------------------
                     |         |         |         |
                     |         |         |         |                     ---------------------
                    \|/       \|/       \|/       \|/                    |                   |
                 --------- --------- --------- ---------                 |      Kernel       |
                 |       | |       | |       | |       |     Monitor     |      Docker       |
                 | Agent | | Agent | | Agent | | Agent |---------------->|      Kubelet      |
                 |       | |       | |       | |       |                 |      Cgroup       |
                 --------- --------- --------- ---------                 |      ......       |
                                                                         |                   |
                                                                         ---------------------
```

故障诊断恢复平台的 Master 和 Agent 均由下列部分组成：

* 故障事件源（AbnormalSource）
* 故障分析链（DiagnoseChain）
* 信息管理器（InformationManager）
* 故障恢复链（RecoverChain）

```bash
    ------------------             -----------------             ----------------
    |                |  Abnormal   |               |  Abnormal   |              |
    | AbnormalSource |------------>| DiagnoseChain |------------>| RecoverChain |
    |                |             |               |             |              |
    ------------------             -----------------             ----------------
                                           |                             |
                                           |                             |
                                           |                             |
                                          \|/                           \|/
                                    ---------------               ---------------
                                    |             |               |             |
                                    | Diagnoser 1 |               | Recoverer 1 |
                                    |             |               |             |
                                    ---------------               ---------------
                                           |                             |
                                           |                             |
                                           |                             |
                                          \|/                           \|/
  ----------------------            ---------------               ---------------
  |                    |            |             |               |             |
  | InformationManager |<-----------| Diagnoser 2 |               | Recoverer 2 |
  |                    |            |             |               |             |
  ----------------------            ---------------               ---------------
            |                              |                             |
            |                              |                             |
            |                              |                             |
           \|/                            \|/                           \|/
--------------------------              .......                       .......
|                        |
| InformationCollector 1 |
|                        |
--------------------------
```

故障诊断恢复平台的 Master 和 Agent 工作流程如下：

* 初始为无故障状态。
* 获取到故障事件，进入故障分析流程。
* 如果故障分析后被识别为已知问题，获取相关排障信息并进入恢复流程。
  * 如果故障自动恢复成功则置为无故障状态并发送报警。
  * 如果故障自动恢复失败则置为待处理状态并发送报警。
* 如果故障分析后无法被识别，进入人工干预流程，此时获取相关排障信息、置为待处理状态并发送报警。

故障诊断恢复平台的 Master 和 Agent 状态迁移图如下：

```bash
                          Recover Successfully, Send Warning
           -----------------------------------------------------------------
           |                                                               |
           |                                                   -------------------------
           |                                                   |                       |
          \|/                                                  |   Recover Abnormal    |--------------------------------------------------
------------------------                                       |                       |                                                 |
|                      |                                       -------------------------                                                 |
| No Abnormal Detected |                                                  /|\                                                            |
|                      |           Fetch Information, Recoverable Abnormal |                                                             |
------------------------                                                   |                                                             |
           |                                                   -------------------------                                                 |
           | Abnormal Received         Identifiable Abnormal   |                       |                                                 |
          \|/                       -------------------------->| Identifiable Abnormal |----                                             |
------------------------            |                          |                       |   |                                             |
|                      |-------------                          -------------------------   |                                             |
|  Abnormal Detected   |                                                                   |                Recover Failed, Send Warning |
|                      |-------------                          -------------------------   |                                             |
------------------------            |                          |                       |   |                                             |
                                    -------------------------->|   Need Intervention   |   |                                             |
                                      Unidentifiable Abnormal  |                       |   | Fetch Information, Unrecoverable Abnormal   |
                                                               -------------------------   |                                             |
                                                                           |               |                                             |
                                           Fetch Information, Send Warning |               |                                             |
                                                                          \|/              |                                             |
                                                               -------------------------   |                                             |
                                                               |                       |   |                                             |
                                                               |   Pending Abnormal    |<-------------------------------------------------
                                                               |                       |
                                                               -------------------------
```

故障诊断恢复平台的 Master 和 Agent 组件在状态转换和工作处理流程上基本一致，在功能细节上有以下区别：

* Master 监听 APIServer 获取 Event 和 CRD 作为故障事件源；Agent 不直接与 APIServer 交互而是通过 Master 获取集群层面相关信息。
* Master 直接与集群 Prometheus 通信获取报警信息，Agent 不直接与 Prometheus 交互。
* Master 负责 Kubernetes Master 组件的故障诊断和恢复；Agent 负责 Kubernetes Node 组件的故障诊断和恢复。
* Master 在接收到 Node 级别的 Event 或者 CRD 故障事件后，会将事件转发至相应 Node 上的 Agent，Agent 通过故障事件源接收故障事件。

### Master

故障诊断恢复平台 Master 组件功能如下：

* 监控 Kubernetes Master 组件，包括 APIServer、ControllerManager、Scheduler、Etcd 等。
* 获取 Event 以及 Prometheus 报警作为故障源。
* 监听故障诊断 CRD 资源并进行处理和状态同步。
* 管理 Agent 组件。
* 用户可以通过 Master 本地端口获取 Master 组件的 Abnormal 列表。

### Agent

故障诊断恢复平台 Agent 组件功能如下：

* 监听 Master 组件并处理故障事件。
* 对本节点故障进行诊断和恢复。
* Prometheus 监控指标动态注册。
* 用户可以通过 Agent 本地端口获取本节点的 Abnormal 列表。

### Abnormal CRD

Abnormal CRD 是故障诊断恢复平台中故障事件源和故障分析链通信的接口。故障事件的详情记录在 Spec 中，故障事件源、故障分析链和故障恢复链对 Abnormal 进行处理并通过变更 Status 字段进行通信。详细信息参考 [Abnormal API 设计](./abnormal.md)。

### Diagnoser CRD

Diagnoser CRD 用于注册故障分析器，故障分析器的元数据记录在 Spec 中，包括发现方式和监听地址。Diagnoser 的当前状态记录在 Status 字段。详细信息参考 [Diagnoser API 设计](./diagnoser.md)。

### InformationCollector CRD

InformationCollector CRD 用于注册信息采集器，信息采集器的元数据记录在 Spec 中，包括发现方式和监听地址。InformationCollector 的当前状态记录在 Status 字段。详细信息参考 [InformationCollector API 设计](./information-collector.md)。

### Recoverer CRD

Recoverer CRD 用于注册故障恢复器，故障恢复器的元数据记录在 Spec 中，包括发现方式和监听地址。Recoverer 的当前状态记录在 Status 字段。详细信息参考 [Recoverer API 设计](./recoverer.md)。

### 故障事件源

故障事件源是获取故障事件的接口，大致可分为以下几类，每一类都需要实现故障事件源接口：

* 日志：日志是获悉集群发生故障的重要手段，通过监听内核、Kubernetes 以及 Docker 日志中的异常字段可以在第一时间发现故障并进行处理。
* Prometheus：Prometheus 满足云原生体系中绝大部分组件的监控需求，大部分组件的内部异常可由 Prometheus 接口暴露。
* Event：Kubernetes 中的事件支持更细致的故障上报机制。
* CRD：CRD 用于自定义故障，用户可以自定义进行扩展。

故障事件源接口如下：

```go
type AbnormalSource interface {
	// 运行故障事件源。
	// 故障事件源可以读取日志、监听 Prometheus 报警、获取 Event、CRD 等，并通过消费故障事件生成 Abnormal。
	Run() error
	// 将故障发送到故障分析链。
	SendAbnormal(abnormal Abnormal) (Abnormal, error)
}
```

故障事件源在消费日志、Prometheus 报警和 Event 后会生成 Abnormal 故障事件并发往故障分析链。用户也可以直接通过 CRD 来创建故障事件。

### 故障分析链

故障分析链是一个调用链框架，本身并不包含故障分析的逻辑，用户需要实现故障分析的具体逻辑并注册到故障分析链中。故障分析链从故障事件源接收故障事件并将故障事件逐一传入被注册的故障分析器中，当故障事件能够被某个故障分析器识别则中止调用并交由该逻辑进行处理。如果故障无法被任何故障分析器识别则直接获取相关排障信息并报警。故障分析器一般是一个 HTTP 服务器。用户可以将自定义故障诊断分析的脚本放入特定路径，故障分析链会动态的获取自定义故障诊断分析文件。

故障分析链和故障分析器接口如下：

```go
type DiagnoseChain interface {
	// 运行故障分析链。
	// 故障分析链会将 Abnormal 故障事件逐一传入被注册的故障分析器中，当 Abnormal 能够被某个故障分析器识别则中止调用并交由该逻辑进行处理。
	Run() error
	// 获取所有故障分析器。
	ListDiagnosers() []Diagnoser
	// 注册就绪的故障分析器。
	// 故障分析器一般是一个 HTTP 服务器。
	RegisterDiagnoser(diagnoser Diagnoser) error
	// 解注册未就绪的故障分析器。
	DeregisterDiagnoser(diagnoser Diagnoser) error
	// 将故障发送到故障恢复链。
	SendAbnormal(abnormal Abnormal) (Abnormal, error)
}

type Diagnoser interface {
	// 获取故障分析器名称。
	Name() (string, error)
	// 执行故障分析器。
	Diagnose(abnormal Abnormal) (Abnormal, error)
	// 设置信息采集器。
	WithInformationCollector(namespace string, name string)
}
```

故障分析器在无法识别 Abnormal 故障事件时返回错误，故障分析器在成功识别 Abnormal 故障事件后变更 Status 字段。故障分析链在某个故障分析器成功识别 Abnormal 故障事件后将 Abnormal 故障事件发往故障恢复链。故障分析器在执行诊断时可以通过调用信息采集器获取更多信息。

### 信息管理器

当故障分析或恢复流程较复杂时，需要从其他接口获取更多信息用于故障的诊断和确认。此时故障分析器或故障恢复器可以调用信息采集器获取更多信息。常用的信息采集器包括 eBPF、Golang 剖析文件、Java 虚拟机工具等。信息采集器一般是一个 HTTP 服务器。故障分析器或故障恢复器需要保证能够正确处理从信息采集器获取的信息。信息管理器用于管理多个信息采集器，用户通过请求信息管理器获取额外信息，信息管理器在接收到请求后会转发到相应的信息采集器中。

信息管理器和信息采集器的接口如下：

```go
type InformationManager interface {
	// 运行信息管理器。
	// 信息管理器转发信息采集请求至被注册的信息采集器并返回信息。
	Run() error
	// 获取所有信息采集器。
	ListInformationCollectors() []InformationCollector
	// 注册就绪的信息采集器。
	// 信息采集器一般是一个 HTTP 服务器。
	RegisterInformationCollector(informationCollector InformationCollector) error
	// 解注册未就绪的信息采集器。
	DeregisterInformationCollector(informationCollector InformationCollector) error
}

type InformationCollector interface {
	// 获取信息采集器名称。
	Name() (string, error)
	// 采集信息。
	Collect() ([]byte, error)
}
```

### 故障恢复链

和故障分析链相似，故障恢复链也是一个调用链框架，本身并不包含故障恢复的逻辑，用户需要实现故障恢复的具体逻辑并注册到故障恢复链中。故障恢复链从故障分析链接收故障事件并将故障事件逐一传入被注册的故障恢复器中，当故障能够被某个故障恢复器识别则中止调用并交由该逻辑进行处理。如果故障无法被任何故障恢复器识别则报错中止。故障恢复器一般是一个 HTTP 服务器。用户可以将自定义故障恢复的脚本放入特定路径，故障恢复链会动态的获取自定义故障恢复文件。

故障恢复链和故障恢复器接口如下：

```go
type RecoverChain interface {
	// 运行故障恢复链。
	// 故障恢复链会将 Abnormal 故障事件逐一传入被注册的故障恢复器中，当 Abnormal 能够被某个故障恢复器识别则中止调用并交由该逻辑进行处理。
	Run() error
	// 获取所有故障恢复器。
	ListRecoverers() []Recoverer
	// 注册就绪的故障恢复器。
	// 故障恢复器一般是一个 HTTP 服务器。
	RegisterRecoverer(recoverer Recoverer) error
	// 解注册未就绪的故障恢复器。
	DeregisterRecoverer(recoverer Recoverer) error
}

type Recoverer interface {
	// 获取故障恢复器名称。
	Name() (string, error)
	// 执行故障恢复器。
	Recover(abnormal Abnormal) error
	// 设置信息采集器。
	WithInformationCollector(namespace string, name string)
}
```

## 典型用例

当用户的 Pod 长时间无法被正常删除时，需要对这些处于 Terminating 状态的 Pod 进行强制删除的简单恢复。通过以下步骤可以实现该故障的诊断恢复：

* 实现故障事件源：通过脚本获取当前 Terminating 状态的 Pod，当该类 Pod 存在时创建表示该故障的 CRD。
* 实现故障分析器：因为此时已经能够确认故障，直接跳过故障分析步骤到故障恢复链。
* 实现故障恢复器：通过脚本强制删除该 Pod 进行恢复。
