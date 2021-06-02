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
// DiagnosisSpec 定义了 Diagnosis 的目标状态。
type DiagnosisSpec struct {
    // OperationSet 是待执行诊断流水线的 OperationSet 名。
    OperationSet string `json:"operationSet"`
    // 必须指定 NodeName 或 PodReference 其中的一个字段。
    // NodeName 是诊断执行的节点。
    NodeName string `json:"nodeName,omitempty"`
    // PodReference 包含目标 Pod 的详细信息。
    PodReference *PodReference `json:"podReference,omitempty"`
    // Parameters 包含诊断过程中需要传入的参数。
    // 通常该字段的键为 OperationSet 中顶点的序号，值为执行该顶点诊断操作需要的参数。
    // Parameters 和 OperationResults 会被序列化为 JSON 对象并在运行诊断的过程中发送给故障处理器。
    Parameters map[string]string `json:"parameters,omitempty"`
}

// PodReference 包含目标 Pod 的详细信息。
type PodReference struct {
    NamespacedName `json:",inline"`
    // Container 是目标容器名。
    Container string `json:"container,omitempty"`
}

// NamespacedName 表示 Kubernetes API 对象。
type NamespacedName struct {
    // Namespace 是 Kubernetes API 对象命名空间。
    Namespace string `json:"namespace"`
    // Namespace 是 Kubernetes API 对象名。
    Name string `json:"name"`
}

// DiagnosisStatus 定义了 Diagnosis 的实际状态。
type DiagnosisStatus struct {
    // Phase 是 Diagnosis 在其生命周期中所处位置的简单宏观概述。状况列表包含更多关于 Diagnosis 状态的信息。
    // 阶段可能存在五种不同的值：
    //
    // Pending：Diagnosis 已被系统接受，但诊断执行前的准备工作还未完成。
    // Running：Diagnosis 已经绑定到了某个节点，至少有一个诊断操作正处于运行状态。
    // Succeeded：诊断流水线中某个路径中的所有诊断操作均执行成功。
    // Failed：诊断流水线中的所有路径失败。也就是说，所有路径中最后一个执行的诊断操作返回码非 200。
    // Unknown：因为某些原因无法取得 Diagnosis 的状态。这种情况通常是因为与 Diagnosis 所在主机通信失败。
    Phase DiagnosisPhase `json:"phase,omitempty"`
    // Conditions 包含 Diagnosis 当前的服务状态。
    Conditions []DiagnosisCondition `json:"conditions,omitempty"`
    // StartTime 是对象被系统接收的 RFC 3339 日期和时间。
    StartTime metav1.Time `json:"startTime,omitempty"`
    // FailedPaths 包含诊断流水线中所有运行失败的路径。路径的最后一个顶点是操作执行失败的顶点。
    FailedPaths []Path `json:"failedPath,omitempty"`
    // SucceededPath 是诊断流水线中运行成功的路径。
    SucceededPath Path `json:"succeededPath,omitempty"`
    // OperationResults 包含诊断运行过程中操作的结果。
    // Parameters 和 OperationResults 会被序列化为 JSON 对象并在运行诊断的过程中发送给故障处理器。
    OperationResults map[string]string `json:"operationResults,omitempty"`
    // Checkpoint 是恢复未完成诊断的检查点。
    Checkpoint *Checkpoint `json:"checkpoint,omitempty"`
}

// DiagnosisCondition 包含 Diagnosis 当前的服务状态。
type DiagnosisCondition struct {
    // Type 是状况的类型。
    Type DiagnosisConditionType `json:"type"`
    // Status 是状况的状态。
    // 可以是 True、False、Unknown。
    Status corev1.ConditionStatus `json:"status"`
    // LastTransitionTime 描述了上一次从某个状况迁移到另一个状况的时间。
    LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
    // Reason 是对上一次状况迁移的描述，该原因描述是唯一的、只包含单个词语的、符合驼峰命名法的。
    Reason string `json:"reason,omitempty"`
    // Message 是描述上一次状况迁移细节的信息。
    Message string `json:"message,omitempty"`
}

// Checkpoint 是恢复未完成诊断的检查点。
type Checkpoint struct {
    // PathIndex 是当前路径在 OperationSet 状态中的序号。
    PathIndex int `json:"pathIndex"`
    // NodeIndex 是当前顶点在路径中的序号。
    NodeIndex int `json:"nodeIndex"`
}

// DiagnosisPhase 是描述当前 Diagnosis 状况的标签。
type DiagnosisPhase string

// DiagnosisConditionType 是 Diagnosis 状况类型的合法值。
type DiagnosisConditionType string

// Diagnosis 的 API 对象。
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
