# 基于图的诊断流水线

本文阐述了 KubeDiag 的诊断流水线设计。

## 背景

KubeDiag 早期设计上为了规范和简化流水线的定义，将诊断流程基于链表的设计分成了三个阶段：信息采集、故障诊断、故障恢复。用户可以在每个阶段定义需要处理的操作并对问题进行诊断。在很多情况下，不同问题导致的现象可能是相同的，或着某个现象是多个故障连锁反应导致的。例如导致节点状态由 `Ready` 变为 `NotReady` 的因素非常多，分析时需要需要在多个排查路径上逐个分析来寻找根本原因。基于链表的设计虽然降低了管理复杂性，但是无法适应更加多样化的诊断场景。图数据结构对该场景明显具备更准确的抽象能力。

## 设计假设

基于图的诊断流水线在设计上需要考虑以下假设条件：

* 有限可终止：整个流水线不是无限执行的，在一定时间和空间复杂度内能够终止运行。
* 过程可追溯：诊断结束后可以查看运行过程中某个顶点产生的结果。
* 状态机可扩展：支持增加新的处理顶点到流水线中。

## 实现

通过引入下列 API 对象可以实现基于图的诊断流水线：

* `Operation`：描述如何在诊断流水线中加入处理顶点以及如何存储该处理顶点产生的结果。
* `OperationSet`：表示诊断过程状态机的有向无环图。
* `Trigger`：描述如何通过 Prometheus 报警或 Event 触发一次诊断。

### Operation

`Operation` API 对象的数据结构如下：

```go
// OperationSpec defines the desired state of Operation.
type OperationSpec struct {
    // Processor describes how to register a operation processor into kubediag.
    Processor Processor `json:"processor"`
    // Dependences is the list of all depended operations required to be precedently executed.
    // +optional
    Dependences []string `json:"dependences,omitempty"`
    // Storage represents the type of storage for operation results.
    // Operation results will not be stored if nil.
    // +optional
    Storage *Storage `json:"storage,omitempty"`
}

// Processor describes how to register a operation processor into kubediag.
type Processor struct {
    // One and only one of the following processor should be specified.
    // HTTPServer specifies the http server to do operations.
    // +optional
    HTTPServer *HTTPServer `json:"httpServer,omitempty"`
    // ScriptRunner contains the information to run a script.
    // +optional
    ScriptRunner *ScriptRunner `json:"scriptRunner,omitempty"`
    // Number of seconds after which the processor times out.
    // Defaults to 30 seconds. Minimum value is 1.
    // +optional
    TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// Storage represents the type of storage for operation results.
type Storage struct {
    // HostPath represents a directory on the host.
    // +optional
    HostPath *HostPath `json:"hostPath,omitempty"`
}

// HostPath represents a directory on the host.
type HostPath struct {
    // Path of the directory on the host.
    // Defaults to kubediag agent data root if not specified.
    Path string `json:"path"`
}

// HTTPServer specifies the http server to do operations.
type HTTPServer struct {
    // Address is the serving address of the processor. It must be either an ip or a dns address.
    // Defaults to kubediag agent advertised address if not specified.
    // +optional
    Address *string `json:"address,omitempty"`
    // Port is the serving port of the processor.
    // Defaults to kubediag agent serving port if not specified.
    // +optional
    Port *int32 `json:"port,omitempty"`
    // Path is the serving http path of processor.
    // +optional
    Path *string `json:"path,omitempty"`
    // Scheme is the serving scheme of processor. It must be either http or https.
    // +optional
    Scheme *string `json:"scheme,omitempty"`
}

// ScriptRunner contains the information to run a script.
type ScriptRunner struct {
    // Script is the content of shell script.
    Script string `json:"script"`
    // ArgKeys contains a slice of keys in parameters or operationResults. The script arguments are generated
    // from specified key value pairs.
    // No argument will be passed to the script if not specified.
    // +optional
    ArgKeys []string `json:"argKeys,omitempty"`
    // OperationResultKey is the prefix of keys to store script stdout, stderr or error message in operationResults.
    // Execution results will not be updated if not specified.
    // +optional
    OperationResultKey *string `json:"operationResultKey,omitempty"`
}

// Operation is the Schema for the operations API.
type Operation struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec OperationSpec `json:"spec,omitempty"`
}
```

### OperationSet

`OperationSet` API 对象的数据结构如下：

```go
// OperationSetSpec defines the desired state of OperationSet.
type OperationSetSpec struct {
    // AdjacencyList contains all nodes in the directed acyclic graph. The first node in the list represents the
    // start of a diagnosis.
    AdjacencyList []Node `json:"adjacencyList"`
}

// Node is a node in the directed acyclic graph. It contains details of the operation.
type Node struct {
    // ID is the unique identifier of the node.
    // It is identical to node index in adjacency list and set by admission webhook server.
    // +optional
    ID int `json:"id,omitempty"`
    // To is the list of node ids this node links to.
    // +optional
    To NodeSet `json:"to,omitempty"`
    // Operation is the name of operation running on the node.
    // It is empty if the node is the first in the list.
    // +optional
    Operation string `json:"operation,omitempty"`
    // Dependences is the list of depended node ids.
    // +optional
    Dependences NodeSet `json:"dependences,omitempty"`
}

// NodeSet is the set of node ids.
type NodeSet []int

// OperationSetStatus defines the observed state of OperationSet.
type OperationSetStatus struct {
    // Paths is the collection of all directed paths of the directed acyclic graph.
    // +optional
    Paths []Path `json:"paths,omitempty"`
    // Specifies whether a valid directed acyclic graph can be generated via provided nodes.
    Ready bool `json:"ready"`
}

// Path represents a linear ordering of nodes along the direction of every directed edge.
type Path []Node

// OperationSet is the Schema for the operationsets API.
type OperationSet struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   OperationSetSpec   `json:"spec,omitempty"`
    Status OperationSetStatus `json:"status,omitempty"`
}
```

### Trigger

`Trigger` API 对象的数据结构如下：

```go
// TriggerSpec defines the desired state of Trigger.
type TriggerSpec struct {
    // OperationSet is the name of referenced operation set in the generated diagnosis.
    OperationSet string `json:"operationSet"`
    // NodeName is the default node which the diagnosis is on.
    // +optional
    NodeName string `json:"nodeName,omitempty"`
    // SourceTemplate is the template of trigger.
    SourceTemplate SourceTemplate `json:"sourceTemplate"`
}

// SourceTemplate describes the information to generate a diagnosis.
type SourceTemplate struct {
    // One and only one of the following source should be specified.
    // PrometheusAlertTemplate specifies the template to create a diagnosis from a prometheus alert.
    // +optional
    PrometheusAlertTemplate *PrometheusAlertTemplate `json:"prometheusAlertTemplate,omitempty"`
    // KubernetesEventTemplate specifies the template to create a diagnosis from a kubernetes event.
    // +optional
    KubernetesEventTemplate *KubernetesEventTemplate `json:"kubernetesEventTemplate,omitempty"`
    // CronTemplate specifies the template to create a diagnosis periodically at fixed times.
    // +optional
    CronTemplate *CronTemplate `json:"cronTemplate,omitempty"`
}

// PrometheusAlertTemplate specifies the template to create a diagnosis from a prometheus alert.
type PrometheusAlertTemplate struct {
    // Regexp is the regular expression for matching prometheus alert template.
    Regexp PrometheusAlertTemplateRegexp `json:"regexp"`
    // NodeNameReferenceLabel specifies the label for setting ".spec.nodeName" of generated diagnosis.
    // The label value will be set as ".spec.nodeName" field.
    // +optional
    NodeNameReferenceLabel model.LabelName `json:"nodeNameReferenceLabel,omitempty"`
    // PodNamespaceReferenceLabel specifies the label for setting ".spec.podReference.namespace" of generated diagnosis.
    // The label value will be set as ".spec.podReference.namespace" field.
    // +optional
    PodNamespaceReferenceLabel model.LabelName `json:"podNamespaceReferenceLabel,omitempty"`
    // PodNameReferenceLabel specifies the label for setting ".spec.podReference.name" of generated diagnosis.
    // The label value will be set as ".spec.podReference.name" field.
    // +optional
    PodNameReferenceLabel model.LabelName `json:"podNameReferenceLabel,omitempty"`
    // ContainerReferenceLabel specifies the label for setting ".spec.podReference.container" of generated diagnosis.
    // The label value will be set as ".spec.podReference.container" field.
    // +optional
    ContainerReferenceLabel model.LabelName `json:"containerReferenceLabel,omitempty"`
    // ParameterInjectionLabels specifies the labels for setting ".spec.parameters" of generated diagnosis.
    // All label names and values will be set as key value pairs in ".spec.parameters" field.
    // +optional
    ParameterInjectionLabels []model.LabelName `json:"parameterInjectionLabels,omitempty"`
}

// PrometheusAlertTemplateRegexp is the regular expression for matching prometheus alert template.
// All regular expressions must be in the syntax accepted by RE2 and described at https://golang.org/s/re2syntax.
type PrometheusAlertTemplateRegexp struct {
    // AlertName is the regular expression for matching "AlertName" of prometheus alert.
    // +optional
    AlertName string `json:"alertName,omitempty"`
    // Labels is the regular expression for matching "Labels" of prometheus alert.
    // Only label values are regular expressions while all label names must be identical to the
    // prometheus alert label names.
    // +optional
    Labels model.LabelSet `json:"labels,omitempty"`
    // Annotations is the regular expression for matching "Annotations" of prometheus alert.
    // Only annotation values are regular expressions while all annotation names must be identical to the
    // prometheus alert annotation names.
    // +optional
    Annotations model.LabelSet `json:"annotations,omitempty"`
    // StartsAt is the regular expression for matching "StartsAt" of prometheus alert.
    // +optional
    StartsAt string `json:"startsAt,omitempty"`
    // EndsAt is the regular expression for matching "EndsAt" of prometheus alert.
    // +optional
    EndsAt string `json:"endsAt,omitempty"`
    // GeneratorURL is the regular expression for matching "GeneratorURL" of prometheus alert.
    // +optional
    GeneratorURL string `json:"generatorURL,omitempty"`
}

// KubernetesEventTemplate specifies the template to create a diagnosis from a kubernetes event.
type KubernetesEventTemplate struct {
    // Regexp is the regular expression for matching kubernetes event template.
    Regexp KubernetesEventTemplateRegexp `json:"regexp"`
}

// KubernetesEventTemplateRegexp is the regular expression for matching kubernetes event template.
// All regular expressions must be in the syntax accepted by RE2 and described at https://golang.org/s/re2syntax.
type KubernetesEventTemplateRegexp struct {
    // Name is the regular expression for matching "Name" of kubernetes event.
    // +optional
    Name string `json:"name,omitempty"`
    // Namespace is the regular expression for matching "Namespace" of kubernetes event.
    // +optional
    Namespace string `json:"namespace,omitempty"`
    // Reason is the regular expression for matching "Reason" of kubernetes event.
    // +optional
    Reason string `json:"reason,omitempty"`
    // Message is the regular expression for matching "Message" of kubernetes event.
    // +optional
    Message string `json:"message,omitempty"`
    // Source is the regular expression for matching "Source" of kubernetes event.
    // All fields of "Source" are regular expressions.
    // +optional
    Source corev1.EventSource `json:"source,omitempty"`
}

// CronTemplate specifies the template to create a diagnosis periodically at fixed times.
type CronTemplate struct {
    // Schedule is the schedule in cron format.
    // See https://en.wikipedia.org/wiki/Cron for more details.
    Schedule string `json:"schedule"`
}

// TriggerStatus defines the observed state of Trigger.
type TriggerStatus struct {
    // LastScheduleTime is the last time the cron was successfully scheduled.
    // +optional
    LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
}

// Trigger is the Schema for the triggers API.
type Trigger struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TriggerSpec   `json:"spec,omitempty"`
    Status TriggerStatus `json:"status,omitempty"`
}
```

### 注册诊断操作

诊断操作表示在诊断流水线中运行的某个逻辑，是对诊断流水线管理的最小单元，例如获取节点信息、对日志中的关键字进行匹配、对进程进行性能剖析等。通过创建 Operation 对象可以注册诊断操作。诊断操作的后端是一个 HTTP 服务器。注册诊断操作时需要指定 HTTP 服务器监听的地址、路径、诊断结果的存储类型等。

### 注册诊断流水线

诊断流水线是多个诊断操作的集合，通常一次诊断可能有多个排查路径，所以一次诊断的状态机通过有向无环图进行抽象。通过创建 OperationSet 对象可以定义表示诊断状态机的有向无环图。诊断开始的状态为有向无环图的起点，有向无环图中的路径均为诊断过程中的排查路径，当某条路径可以成功运行到终点时则表示诊断运行成功。诊断流水线的生成逻辑如下：

1. 用户创建 OperationSet 资源并定义有向无环图中所有的边。
1. 根据 OperationSet 的定义构建有向无环图。
1. 如果无法构建合法的有向无环图，则将注册失败的状态和失败原因更新到 OperationSet 中。
1. 枚举 OperationSet 中所有的诊断路径并更新到 OperationSet 中。

表示诊断流水线的有向无环图必须只包含一个源顶点（Source Node），该顶点用于表示诊断的开始状态且不包含任何诊断操作。诊断路径是任何从源顶点到任意阱顶点（Sink Node）的路径。诊断路径不包括表示诊断的开始状态的源顶点。图构建器通过搜索出有向无环图中所有的诊断路径并更新至 OperationSet 的 `.status.paths` 字段。

### 触发诊断

Diagnosis 对象的元数据中包含了需要执行的 OperationSet。触发诊断包括手动和自动两种方式。通过手动创建 Diagnosis 对象可以直接触发诊断。通过创建 Trigger 对象并配置 Prometheus 报警模板或 Event 模板可以基于 Prometheus 或 Event 自动生成 Diagnosis 以触发诊断流水线。用户还可以通过定义 Cron 来定时触发预定义的诊断。

### 运行诊断流水线

Diagnosis 对象的元数据中记录了诊断流水线的运行状态。诊断流水线的运行逻辑如下：

1. 获取被 Diagnosis 引用的 OperationSet 中所有的诊断路径。
1. 按照诊断执行路径中 Operation 定义的诊断操作，将 Operation 运行的结果更新到 Diagnosis 中并持久化到 Operation 中相应的存储类型。
1. 如果路径中定义的某个诊断操作执行失败，则执行下一条诊断路径。
1. 如果路径中定义的所有诊断操作均执行成功，则该次诊断成功。
1. 如果所有路径均无法成功，则该次诊断失败。

![Graph](../images/graph.png)

上列为一个表示诊断流水线的有向无环图，该图的诊断路径表示多个可执行的排查路径：

* 数据收集 1、数据分析 1、恢复 1
* 数据收集 1、数据分析 1、恢复 2
* 数据收集 2、数据分析 2、恢复 2
* 数据收集 3、数据收集 4
