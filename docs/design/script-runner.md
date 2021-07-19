# Script Runner

This page demonstrates the design of a mechanism to register and run a shell script as an operation.

## Background

Many users operates their environments by running shell scripts. It is an essential user case that KubeDiag supports integration and management of user defined scripts in a convenient way.

## Implementation

Since most shell scripts are maintained by system or application operators, we have made the following assumption to narrow down our design purpose:

* A script sometimes takes arguments.
* A script outputs its execution status in stderr or stdout.
* Error sometimes happens during script execution.
* The node name which a script runs on must be specified.
* An user need to check the details of a script and the result of one execution.
* A script could be executed before or after other operations.
* A script could depend on the result of other operations.
* Other operations could depend on the result of a script.

### API Object

To achieve our design purposes, we extend the `Operation` API for users to integrate scripts into an operation pipeline.

```go
// OperationSpec defines the desired state of Operation.
type OperationSpec struct {
    // Processor describes how to register a operation processor into kubediag.
    Processor Processor `json:"processor"`
    // ......
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
    // ......
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
```

The `ScriptRunner` processor is designed for users to define how should KubeDiag run their scripts. It contains the following fields:

* `.spec.processor.scriptRunner.script`: The content of shell script.
* `.spec.processor.scriptRunner.argKeys`: ArgKeys contains a slice of keys in parameters or operationResults. The script arguments are generated from specified key value pairs. No argument will be passed to the script if not specified.
* `.spec.processor.scriptRunner.operationResultKey`: OperationResultKey is the prefix of keys to store script stdout, stderr or error message in operationResults. Execution results will not be updated if not specified.

### Managing Scripts on Node

KubeDiag stores all scripts under the agent data root directory. The operation controller will create, update or delete an executable file if an `Operation` of script runner type is reconciled. The following steps will happen during reconciliation:

* Operation controller in agent reconciles the created operation and check if `.spec.processor.scriptRunner` is nil.
* If the operation is not a script runner, skip synchronizing the object.
* If the operation is a script runner and event type is create, the agent will create a file under data root directory, which defaults to `/var/lib/kubediag/scripts`.
  * Change file mode of the script to executable.
* If the operation is a script runner and event type is update, the agent will remove the older file with operation name and create a new file under data root directory.
  * Change file mode of the script to executable.
* If the operation is a script runner and event type is delete, the agent will remove the file with operation name under data root directory.

### Passing Arguments to a Script

An `Operation` is defined before any execution of script. The `Diagnosis` resource stores details of a pipeline run, which includes how can we pass arguments to a script defined in `Operation`.

To pass any arguments to a script when running a diagnosis, `.spec.parameters` of `Diagnosis` should be specified on creation. A scripts can also take results from precedent operations as arguments if any component in `.spec.processor.scriptRunner.argKeys` of `Operation` matches any keys in `.status.operationResults` of `Diagnosis`. Arguments will be constructed from `.spec.parameters` and `.status.operationResults` just in time before script run. For example, a `Diagnosis` might contain the following JSON object in `.spec.parameters` and `.status.operationResults`:

```json
{
    "key1": "value1",
    "key2": "value2",
    "key3": "value3",
    "key4": "value4"
}
```

If `.spec.processor.scriptRunner.argKeys` of `Operation` is `["key1", "key4"]`, then `value1` and `value4` are passed as arguments to the script.

### Storing the Result of a Script

The result of a script run usually matters if it mutates the environment. By defining `.spec.processor.scriptRunner.operationResultKey`, the result will be updated in `.status.operationResults` of `Diagnosis` with user defined scheme. For example, if operationResultKey is set as `sysctl_config_dump`, the following keys would be set in `.status.operationResults`:

* `operation.sysctl_config_dump.output`: The stdout or stderr of script.
* `operation.sysctl_config_dump.error`: The error if not empty.
