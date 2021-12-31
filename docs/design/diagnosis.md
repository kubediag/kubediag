# Diagnosis

本文阐述了 Diagnosis 对象的设计。

## 背景

通过支持下列功能可以满足用户管理诊断的需求：

* 指定诊断的目标 Node 或 Pod。
* 查看当前诊断的阶段。
* 通过参数扩展诊断的状态。
* 诊断成功时查看诊断的结果以及排查路径。
* 诊断失败时查看失败的原因以及排查路径。
* 查看诊断过程中某个阶段的详细信息。

## 实现

Diagnosis 是针对用户管理诊断的需求所设计的 API 对象。

### API 对象

`Diagnosis` API 对象的数据结构如下：

```go
// DiagnosisSpec defines the desired state of Diagnosis.
type DiagnosisSpec struct {
    // OperationSet is the name of operation set which represents diagnosis pipeline to be executed.
    OperationSet string `json:"operationSet"`
    // One of NodeName and PodReference must be specified.
    // NodeName is a specific node which the diagnosis is on.
    // +optional
    NodeName string `json:"nodeName,omitempty"`
    // PodReference contains details of the target pod.
    // +optional
    PodReference *PodReference `json:"podReference,omitempty"`
    // Parameters is a set of the parameters to be passed to opreations.
    // Parameters and OperationResults are encoded into a json object and sent to operation processor when
    // running diagnosis.
    // +optional
    Parameters map[string]string `json:"parameters,omitempty"`
}

// PodReference contains details of the target pod.
type PodReference struct {
    NamespacedName `json:",inline"`
    // Container specifies name of the target container.
    // +optional
    Container string `json:"container,omitempty"`
}

// NamespacedName represents a kubernetes api resource.
type NamespacedName struct {
    // Namespace specifies the namespace of a kubernetes api resource.
    Namespace string `json:"namespace"`
    // Name specifies the name of a kubernetes api resource.
    Name string `json:"name"`
}

// DiagnosisStatus defines the observed state of Diagnosis.
type DiagnosisStatus struct {
    // Phase is a simple, high-level summary of where the diagnosis is in its lifecycle.
    // The conditions array, the reason and message fields contain more detail about the
    // pod's status.
    // There are five possible phase values:
    //
    // DiagnosisPending: The diagnosis has been accepted by the system, but no operation has been started.
    // DiagnosisRunning: The diagnosis has been bound to a node and one of the operations have been started.
    // At least one operation is still running.
    // DiagnosisSucceeded: All operations in some path have voluntarily terminated with a response code
    // of 200, and the system is not going to execute rest operations.
    // DiagnosisFailed: All paths in the graph have terminated, and at least one operation in each path
    // terminated in a failure.
    // DiagnosisUnknown: For some reason the state of the diagnosis could not be obtained, typically due
    // to an error in communicating with the host of the diagnosis.
    // +optional
    Phase DiagnosisPhase `json:"phase,omitempty"`
    // Conditions contains current service state of diagnosis.
    // +optional
    Conditions []DiagnosisCondition `json:"conditions,omitempty"`
    // StartTime is RFC 3339 date and time at which the object was acknowledged by the system.
    // +optional
    StartTime metav1.Time `json:"startTime,omitempty"`
    // FailedPaths contains all failed paths in diagnosis pipeline.
    // The last node in the path is the one which fails to execute operation.
    // +optional
    FailedPaths []Path `json:"failedPaths,omitempty"`
    // SucceededPath is the succeeded paths in diagnosis pipeline.
    // +optional
    SucceededPath Path `json:"succeededPath,omitempty"`
    // OperationResults contains results of operations.
    // Parameters and OperationResults are encoded into a json object and sent to operation processor when
    // running diagnosis.
    // +optional
    OperationResults map[string]string `json:"operationResults,omitempty"`
    // Checkpoint is the checkpoint for resuming unfinished diagnosis.
    // +optional
    Checkpoint *Checkpoint `json:"checkpoint,omitempty"`
}

// DiagnosisCondition contains details for the current condition of this diagnosis.
type DiagnosisCondition struct {
    // Type is the type of the condition.
    Type DiagnosisConditionType `json:"type"`
    // Status is the status of the condition.
    // Can be True, False, Unknown.
    Status corev1.ConditionStatus `json:"status"`
    // LastTransitionTime specifies last time the condition transitioned from one status
    // to another.
    // +optional
    LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
    // Reason is a unique, one-word, CamelCase reason for the condition's last transition.
    // +optional
    Reason string `json:"reason,omitempty"`
    // Message is a human readable message indicating details about last transition.
    // +optional
    Message string `json:"message,omitempty"`
}

// Checkpoint is the checkpoint for resuming unfinished diagnosis.
type Checkpoint struct {
    // PathIndex is the index of current path in operation set status.
    PathIndex int `json:"pathIndex"`
    // NodeIndex is the index of current node in path.
    NodeIndex int `json:"nodeIndex"`
}

// DiagnosisPhase is a label for the condition of a diagnosis at the current time.
type DiagnosisPhase string

// DiagnosisConditionType is a valid value for DiagnosisCondition.Type.
type DiagnosisConditionType string

// Diagnosis is the Schema for the diagnoses API.
type Diagnosis struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   DiagnosisSpec   `json:"spec,omitempty"`
    Status DiagnosisStatus `json:"status,omitempty"`
}
```

### 管理诊断状态的迁移

诊断实际上是一个有状态的任务，在诊断的生命周期中其状态可能发生多次迁移，管理诊断状态迁移的能力在很多场景中是必不可少的。

某个操作可能依赖特定格式的输入。Diagnosis 中的 `.spec.parameters` 字段用于指定诊断过程中需要传入的参数。该字段是一个键值对，键和值均必须为 String 类型。当执行的操作依赖特定格式的输入时，用户可以在该字段中定义操作执行时需要输入的参数，操作处理器在获取参数后执行诊断操作。

某个操作可能依赖某个之前操作的输出。Diagnosis 中的 `.status.operationResults` 字段用于记录诊断运行过程中操作的结果。该字段是一个键值对，键和值均必须为 String 类型。当前操作执行的结果必须以 JSON 对象的形式返回，返回结果会被更新到该字段中，如果后续操作的执行依赖当前操作的输出，那么后续操作处理器可以从 `.status.operationResults` 中获取当前操作的结果。值得注意的是，如果在排查路径中如果有两个相同的操作对同一个键进行了更新，那么后执行操作的结果会覆盖先执行操作的结果。

用户需要分析排查路径中某个操作的结果并进行优化。Diagnosis 中的 `.status.failedPath` 字段和 `.status.succeededPath` 字段分别记录了所有运行失败的路径和成功的路径。每条路径由一个数组表示，数组的元素中包含顶点的序号和操作名。通过遍历路径可以还原操作执行的顺序，每个操作结果的访问信息被记录在 [Operation](./graph-based-pipeline.md) 中的 `.spec.storage` 字段。

### Diagnosis 阶段

Diagnosis 包含 `.status.phase` 字段，该字段是 Diagnosis 在其生命周期中所处位置的简单宏观概述。该阶段并不是对 Diagnosis 状态的综合汇总，也不是为了成为完整的状态机。Diagnosis 阶段的数量和含义是严格定义的。除了本文档中列举的内容外，不应该再假定 Diagnosis 有其他的阶段值。

下面是 `.status.phase` 可能的值：

* Pending：Diagnosis 已被系统接受，但诊断执行前的准备工作还未完成。
* Running：Diagnosis 已经绑定到了某个节点，至少有一个诊断操作正处于运行状态。
* Succeeded：诊断流水线中某个路径中的所有诊断操作均执行成功。
* Failed：诊断流水线中的所有路径失败。也就是说，所有路径中最后一个执行的诊断操作返回码非 200。
* Unknown：因为某些原因无法取得 Diagnosis 的状态。这种情况通常是因为与 Diagnosis 所在主机通信失败。
