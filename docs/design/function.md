# Function

This page demonstrates the design of a mechanism to register and run a function as an operation.

## Background

The user may integrate an independent system by implementing an operation as an HTTP server. However, it is necessary to provide a more convenient way for users to deploy their own logics of integration. Cloud vendors provide serverless features to deliver code without concerning about resource management. This is why we would like to introduce function API in operation.

## Implementation

The function API should meet the following needs:

* An user could write his or her operation as a function.
* An user does not need to manage the deployment of his or her operation.
* The function could take in parameters from other operations.
* The result of a function could be used as input of other operations.

### API Object

To achieve our design purposes, we extend the `Operation` API for users to integrate functions into an operation pipeline.

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
    // Function contains the details to run a function as an operation.
    // +optional
    Function *Function `json:"function,omitempty"`
    // ...
}

// Function contains the details to run a function as an operation.
type Function struct {
    // CodeSource contains the code source files.
    CodeSource map[string]string `json:"codeSource"`
    // Runtime is the language to use for writing a function.
    Runtime FunctionRuntime `json:"runtime"`
}

// FunctionRuntime is a valid value for Function.Runtime.
type FunctionRuntime string
```

The `Function` processor is designed for users to define how should KubeDiag run their functions. It contains the following fields:

* `.spec.processor.function.codeSource`: The code source files.
* `.spec.processor.function.runtime`: The language to use for writing a function.

### Implementing a Function Runtime

To implement a function runtime, a developer should:

* Define a main function.
* Add controlling logics to manage the main file on node. It is completed by the reconciler of kubediag agent.
* Invoke the main function by kubediag agent if the operation is defined as a function.

It is necessary to define a main function when implementing a function runtime. Context propagation depends on the details of main function. A simple main function in python is described below:

```python
import sys
import json
from function import handler


def main():
    # The last argument is the context in json format.
    context_string = sys.argv[-1]
    context = json.loads(context_string)

    # Call user defined handler.
    result = handler(context)

    # Serialize result from user defined handler to a json formatted string.
    json_object = json.dumps(result)
    print("\n"+json_object)


if __name__ == "__main__":
    sys.exit(main())
```

The main function is invoked by the kubediag agent during a diagnosis execution. The context is passed as arguments in the main function. A handler is required to complete the function:

```python
def handler(context):
    result = dict()
    for key in context:
        result[key] = context[key]
    result["a"] = "1"
    result["b"] = "2"

    return result
```

The following is a typical function invoke sequence:

* The kubediag agent runs the main function of specific runtime.
* The main file invokes user defined handler function.
* The kubediag agent get the return object from the main function.
