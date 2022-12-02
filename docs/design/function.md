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
* The function is built as OCI-compatible container image.

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

* Define a template that contains a main function, Dockerfile and a 'function' directory to put handler function and requirements.
* Mount the template directory (named by runtime) to kubediag agent.

It is necessary to define a main function and Dockerfile when implementing a function runtime. Each function will be built as OCI-compativle container images and works as a HTTP server. Context propagation depends on the details of main function. A example main function in python is described below:

```python
#!/usr/bin/env python
from flask import Flask, request, jsonify
from waitress import serve
import os

from function import handler

app = Flask(__name__)

class Event:
    def __init__(self):
        self.body = request.get_data()
        self.headers = request.headers
        self.method = request.method
        self.query = request.args
        self.path = request.path

class Context:
    def __init__(self):
        self.hostname = os.getenv('HOSTNAME', 'localhost')

def format_status_code(resp):
    if 'statusCode' in resp:
        return resp['statusCode']

    return 200

def format_body(resp):
    if 'body' not in resp:
        return ""
    elif type(resp['body']) == dict:
        return jsonify(resp['body'])
    else:
        return str(resp['body'])

def format_headers(resp):
    if 'headers' not in resp:
        return []
    elif type(resp['headers']) == dict:
        headers = []
        for key in resp['headers'].keys():
            header_tuple = (key, resp['headers'][key])
            headers.append(header_tuple)
        return headers

    return resp['headers']

def format_response(resp):
    if resp == None:
        return ('', 200)

    statusCode = format_status_code(resp)
    body = format_body(resp)
    headers = format_headers(resp)

    return (body, statusCode, headers)

@app.route('/', defaults={'path': ''}, methods=['GET', 'PUT', 'POST', 'PATCH', 'DELETE'])
@app.route('/<path:path>', methods=['GET', 'PUT', 'POST', 'PATCH', 'DELETE'])
def call_handler(path):
    event = Event()
    context = Context()
    response_data = handler.handle(event, context)

    resp = format_response(response_data)
    return resp

if __name__ == '__main__':
    serve(app, host='0.0.0.0', port=5000)
```

The main function is invoked by the kubediag agent during a diagnosis execution. The context is passed as arguments in the main function. A handler is required to complete the function:

```python
handler.py: |
    from .hello import hello
    import json
    def handle(event, context):
        data = dict()
        if len(event.body) != 0:
            data = json.loads(event.body)

        result = dict()
        for key in data:
            result[key] = data[key]
        result["a"] = "1"
        result["b"] = "2"
        json_object = json.dumps(result)
        return {
            "statusCode": 200,
            "body": json_object 
        }
```

The following explains function invocations and lifecycle:

* In a kubediag diagnosis, the kubediag agent build container image for function and deploy function as a Kubernetes Pod.
* Functions will be invoked through HTTP requests by kubediag agent.
* The main file invokes user defined handler function.
* The kubediag agent get HTTP response from the main function.
