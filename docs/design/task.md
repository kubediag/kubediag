# Task

Task is the execution entity of a running diagnosis. It contains information to execute a task on the node along with its output.

## Background

We have designed the `Diagnosis` API to represent a execution of a diagnosis so far. However, it is difficult for us to demonstrate the state machine with current model if the diagnosis need to be done on multiple nodes. A mechanism to store the state of diagnosis on one node and a combination of all states is required. We decide to introduce a `Task` API to represent the execution on one node, and extend the `Diagnosis` API to contain multiple tasks.

## API Design

### Task API

Task contains the information to execute a task on one node. It is a running entity of an Operation template.

```go
// TaskSpec defines the desired state of Task.
type TaskSpec struct {
    // One of NodeName and PodReference must be specified.
    // NodeName is a specific node which the task is on.
    // +optional
    NodeName string `json:"nodeName,omitempty"`
    // PodReference contains details of the target pod.
    // +optional
    PodReference *PodReference `json:"podReference,omitempty"`
    // Parameters is a set of the parameters to be passed to opreations.
    // Parameters and Results are encoded into a json object and sent to operation processor when
    // running a task.
    // +optional
    Parameters map[string]string `json:"parameters,omitempty"`
}

// TaskStatus defines the observed state of Task.
type TaskStatus struct {
    // Phase is a simple, high-level summary of where the task is in its lifecycle.
    // The conditions array, the reason and message fields contain more detail about the
    // pod's status.
    // There are five possible phase values:
    //
    // TaskPending: The task has been accepted by the system, but no operation has been started.
    // TaskRunning: The task has been bound to a node and the operation has been started.
    // TaskSucceeded: The task has voluntarily terminated a response code of 200.
    // TaskFailed: The task has terminated in a failure.
    // TaskUnknown: For some reason the state of the task could not be obtained, typically due to an error 
    // in communicating with the host of the task.
    // +optional
    Phase TaskPhase `json:"phase,omitempty"`
    // Conditions contains current service state of task.
    // +optional
    Conditions []TaskCondition `json:"conditions,omitempty"`
    // StartTime is RFC 3339 date and time at which the object was acknowledged by the system.
    // +optional
    StartTime metav1.Time `json:"startTime,omitempty"`
    // Results contains results of a task.
    // Parameters and Results are encoded into a json object and sent to operation processor when running task.
    // +optional
    Results map[string]string `json:"results,omitempty"`
}

// Task is the Schema for the tasks API.
type Task struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TaskSpec   `json:"spec,omitempty"`
    Status TaskStatus `json:"status,omitempty"`
}
```

### Diagnosis API

Diagnosis consists of multiple tasks. It is a running entity of an OperationSet template.

```go

// DiagnosisSpec defines the desired state of Diagnosis.
type DiagnosisSpec struct {
    // OperationSet is the name of operation set which represents diagnosis pipeline to be executed.
    OperationSet string `json:"operationSet"`
    // Parameters is a set of the parameters to be passed to operations.
    // Parameters and OperationResults are encoded into a json object and sent to operation processor when
    // running diagnosis.
    // +optional
    Parameters map[string]string `json:"parameters,omitempty"`
    // TargetSelector contains information to calculate target node to schedule tasks on.
    TargetSelector TargetSelector `json:"targetSelector,omitempty"`
}

// TargetSelector contains information to calculate target node to schedule tasks on.
type TargetSelector struct {
    // NodeSelector queries over a set of nodes. Tasks will be scheduled on the result nodes of matched nodes.
    NodeSelector metav1.LabelSelector `json:"targetNodeSelector,omitempty"`
    // NodeNames specifies nodes which tasks should be scheduled on.
    NodeNames []string `json:"targetNodeNames,omitempty"`
    // PodSelector queries over a set of pods. A tasks will be scheduled on the node which any matched pod is on.
    PodSelector metav1.LabelSelector `json:"targetPodSelector,omitempty"`
    // PodNames specifies pods which tasks should be scheduled on the same node.
    PodNames []PodReference `json:"targetPodNames"`
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
    // Checkpoint is the checkpoint for resuming unfinished diagnosis.
    // +optional
    Checkpoint *Checkpoint `json:"checkpoint,omitempty"`
}
```

## Scheduling

Tasks will be created and scheduled when running a Diagnosis. The required field `.spec.targetSelector` in Diagnosis API provides information to schedule tasks to nodes. A typical successful execution is describe as below:

* A Diagnosis is created with `.spec.targetSelector.nodeSelector` specified.
* KubeDiag Master queries over node and calculates matched nodes. Tasks are created on all matched nodes when running an Operation.
* KubeDiag Agent starts executing diagnosing progress if any Task is created on current node. The task will be set as `Succeeded` if the task is finished without any error.
* KubeDiag Master will move on to the next Operation if all Tasks created when running an Operation is succeeded.

## Garbage Collection

Multiple Tasks are created during the execution of a Diagnosis. The execution details on each node is recorded in the status of a task. The dependency between a Task and a Diagnosis should fulfill the following requirements:

* A Task cannot be deleted if the dependent Diagnosis has not been deleted.
* A Task need to be deleted if the dependent Diagnosis has been deleted.

To fulfill the above requirements, a Diagnosis should be the owner of Tasks which created by the Diagnosis. A finalizer should be added to block the deletion of a Task if its dependent Diagnosis has not been deleted.

## Information Propagation

All results of Task execution are store in the context of a Diagnosis. An example context is in the following syntax:

```json
{
    "parameters": {
        "a": "b",
        "c": "d"
    },
    "operations": {
        "operation1": {
            "task1": "result1",
            "task2": "result2"
        },
        "operation2":{
            "task3": "result3"
        }
    }
}
```

The `parameters` field are defined by `.spec.parameters` in Diagnosis API. The `operations` field is updated during the progress of Diagnosis, it records all results of task executions.
