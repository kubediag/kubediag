# Storage

KubeDiag supports storing the processing results of processors in storage backend.

## Background

Users can store the diagnosis result file and download file by configuring the operation storage mode.

## Design

This design represents the minimally viable changes required to provision based on storage configuration. We hope that KubeDiag has the following storage functions:

1. The execution result file of the operation can be stored centrally, and the storage path can be sensed.
1. Before the operation is executed, files can be downloaded from the storage center to the local for subsequent diagnosis.

### Controller workflow for upload

1. The stored procedure is operated by KubeDiag agent, and the access parameters of the storage backend need to be configured in the startup parameters of KubeDiag agent, such as the endpoint, username, password, etc. of the storage backend.
1. Record the storage type you want to use in the operation (currently only support MinIO).
1. Configure the key corresponding to the file that the operation needs to store. The result of the operation must return the path of the file in the form of key: value.
1. When the agent executes each operation, it will judge whether there is file need to be stored according to the configuration of the operation. If it is true and the storage is smooth, the key of the diagnosis result will be recorded as `storage.<storage_type>.operation.<operation_name>.upload.file`, which value is the stored file path.If the storage process does not go well, the result of diagnosis will record the key as `storage.<storage_type>.operation.<operation_name>.upload.error`, and the value is the reason for the storage failure.

### Controller workflow for download

During the diagnosis process, it may be necessary to download the data of the storage center to the local for file operations. We also made the following design for download.

1. The downloading process of remote data is operated by the KubeDiag agent, and the access parameters of storage driver need to be configured in the KubeDiag agent's startup parameters, such as the endpoint, username, and password.
1. Record the storage type you want to use in the operation (currently only support MinIO).
1. Configure the key corresponding to the file to be downloaded in operation.
1. Declare the address of this file in the data center in the input parameter of diagnosis.
1. In the processing flow of the agent on the operation, when it is found that the file in the storage center needs to be downloaded, the file is retrieved according to the diagnosis input parameter corresponding to the key in the operation.
1. The agent stores the downloaded file in the directory `$data_root/storage/<operation_name>/`.
1. After the file download is completed, the agent will return the local file path and assign this value to the value corresponding to the initial key in diagnosis, that is, the file path of the input parameter of diagnosis will be modified from the remote path to the local path after the download is successful.
1. If the file download fails, the agent will not modify the value corresponding to the file path key, and will only pass it intact to the operation processor. At the same time, it will record a download failure reason in the diagnosis result, the key is `storage.<storage_type>.operation.<operation_name>.download.error`, and the value is the returned error message.

### Additional details

* When the agent reports an error while uploading or downloading files, it will not affect the integrity of the operation, but leave a storage failed record in the diagnosis result.
* When the agent uploads a file successfully, it records the file storage information with a brand new key; when the agent download a file successfully, it directly modifies the file address and records it in the original key.
* The action of uploading files is carried out after the operation is completed, and the action of downloading files is carried out before the operation starts.
* If the storage upload strategy is configured, the value corresponding to the file path key of the operation is regarded as the local file path.
* If the storage download policy is configured, the value in the file path of the operation is considered to be the access path of the storage backend, rather than the local file path or other backend paths.

### API

To achieve our design purposes, we extend the `Operation` API with `Storage`.

```go
// OperationSpec defines the desired state of Operation.
type OperationSpec struct {
  // Processor describes how to register a operation processor into kubediag.
  Processor Processor `json:"processor"`
  // Dependences is the list of all depended operations required to be precedently executed.
  // +optional
  Dependences []string `json:"dependences,omitempty"`
  // Storage represents the type of storage for operation results.
  // Operation output will not be managed by any storage driver if nil.
  // +optional
  Storage *Storage `json:"storage,omitempty"`
}

// Storage represents the type of storage for operation results.
type Storage struct {
  // Type specify which storage to save object.
  // Only support MinIO now.
  Provisioner string `json:"provisioner"`
  // OperationUploadFilePathKey is the key corresponding to the file to be stored in the operation. The storage driver gets the local file path to upload according to the content of this key.
  // The value of this parameter configured in the actual diagnosis must be a local file path.
  // If configured, the upload and storage operation will be executed.
  // +optional
  OperationUploadFilePathKey string `json:"operationUploadFilePathKey"`
  // OperationDownloadFilePathKey is the key of the file to be downloaded in operation, and the storage driver downloads the file to the local according to the content of this key.
  // The value of this parameter configured in the actual diagnosis must be a file path in the backend storage.
  // If configured, it will perform the operation of downloading the file.
  // After the download is complete, the file path corresponding to this key in diagnosis will be changed from the remote path to the local path.
  // +optional
  OperationDownloadFilePathKey string `json:"operationDownloadFilePathKey"`
}
```

The storage is designed for users to define how should KubeDiag store their data. It contains the following fields:

* `spec.storage.provisioner`: Provisioner should be declared as `minio` in operation.
* `spec.storage.operationUploadFilePathKey`: OperationUploadFilePathKey is the key corresponding to the file to be stored in the operation. The storage driver gets the local file path to upload according to the content of this key.
* `spec.storage.operationDownloadFilePathKey`: OperationDownloadFilePathKey is the key of the file to be downloaded in operation, and the storage driver downloads the file to the local according to the content of this key.

## MinIO

## Implementation

We currently implement support for MinIO storage. The following are the implementation details.

### KubeDiag Agent Configuration

Enabling MinIO storage requires configuring the parameters `minio-endpoint`, `minio-access-key-id`, `minio-secret-access-key`, and `minio-ssl` in the agent.

* `minio-endpoint`: S3 compatible object storage endpoint.
* `minio-access-key-id`: Access key for the object storage.
* `minio-secret-access-key`: Secret key for the object storage.
* `minio-ssl`: If 'true' API requests will be secure (HTTPS), and insecure (HTTP) otherwise.

### Example for upload

Here is example to store the file generated by go profiler operation in MinIO.

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: go-profiler
spec:
  processor:
    path: /processor/goProfiler
    scheme: http
    timeoutSeconds: 60
  storage: 
    provisioner: minio
    operationUploadFilePathKey: diagnoser.runtime.go_profiler.result.path
```

After diagnosis, the result generated by go profiler processor contains key `diagnoser.runtime.go_profiler.result.path`. The value of key `diagnoser.runtime.go_profiler.result.path` corresponds to the generated file path, which will be obtained by agent and stored as file in MinIO in the form of a key-value pair, with `<bucket_name>/<diagnosis_namespace>.<diagnosis_name>.<node_name>.<pod_namespace>.<pod_name>.<container_name>.<timestamp>.<file_name>` as key and file as value. Undefined values in key are represented by `0`.

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: go-profiler
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: go-profiler
---
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: go-profiler
  namespace: default
spec:
  nodeName: my-node
  operationSet: go-profiler
  parameters:
    param.diagnoser.runtime.go_profiler.expiration_seconds: "7200"
    param.diagnoser.runtime.go_profiler.source: https://10.0.2.15:6443
    param.diagnoser.runtime.go_profiler.tls.secret_reference.name: apiserver-profiler-sa-token-wqnr2
    param.diagnoser.runtime.go_profiler.tls.secret_reference.namespace: kubediag
    param.diagnoser.runtime.go_profiler.type: Heap
status:
  phase: Succeeded
  operationResults:
    diagnoser.runtime.go_profiler.result.path: /var/lib/kubediag/profilers/go/pprof/20210728084437/default.go-profiler.heap.prof
    storage.minio.operation.go_profiler.upload.file: go-profiler/default.go-profiler.my-node.0.0.0.20210728084437.default.go-profiler.heap.prof
  # ......
```

After successful storage, the result of this diagnosis will record the stored file in MinIO is `go-profiler/default.go-profiler.my-node.0.0.0.20210728084437.default.go-profiler.heap.prof`. If the stored procedure fails,
Another record will be added to the result,  `storage.minio.operation.go_profiler.upload.error`: Error message of storage failure.

### Example for download

Here is example to download file from MinIO.

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: go-profiler
spec:
  processor:
    path: /processor/goProfiler
    scheme: http
    timeoutSeconds: 60
  storage: 
    provisioner: minio
    operationDownloadFilePathKey: param.diagnoser.runtime.go_profiler.file.path
```

The diagnosis corresponding to this operation needs to configure the value of the file to be downloaded, as follows:

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: go-profiler
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: go-profiler
---
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: go-profiler
  namespace: default
spec:
  nodeName: my-node
  operationSet: go-profiler
  parameters:
    param.diagnoser.runtime.go_profiler.expiration_seconds: "7200"
    param.diagnoser.runtime.go_profiler.file.path: go-profiler/default.go-profiler.my-node.0.0.0.20210728084437.default.go-profiler.heap.prof
status:
  phase: Succeeded
  operationResults:
    param.diagnoser.runtime.go_profiler.file.path: /var/lib/kubediag/storage/go-profiler/default.go-profiler.my-node.0.0.0.20210728084437.default.go-profiler.heap.prof
  # ......
```

In the process of diagnosis, the agent downloads the file from MinIO according to the input file path to the local , and stores it in`/var/lib/kubediag/storage/go-profiler/default.go-profiler.my-node.0.0.0.20210728084437.default.go-profiler.heap.prof`, Then change the value of `param.diagnoser.runtime.go_profiler.file.path` to the local path for subsequent diagnosis. If the download fails, the record key added to the result is `storage.minio.operation.go_profiler.download.error`, and the value is the error message.
