# Elasticsearch Collector

Elasticsearch Collector is a [processor](../design/processor.md), users can use it to collect elasticsearch log info.

## Background

During the diagnosis process, users may need to collect log information from elasticsearch. This requirement can be met by introducing Elasticsearch Collector. It supports elasticsearch 7.x.

## Implementation

Elasticsearch Collector is implemented according to [Processor](../design/processor.md). You can register Elasticsearch Collector in KubeDiag through Operation. This Operation is registered by default when KubeDiag is deployed. You can execute the following command to view the registered Elasticsearch Collector:

```bash
$ kubectl get operation elasticsearch-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: elasticsearch-collector
spec:
  processor:
    path: /processor/elasticsearchCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP Request Format

The request processed by Elasticsearch Collector must be POST type, and the HTTP request must contain a specific JSON format request body.

#### HTTP Request

POST /processor/elasticsearchCollector

#### Request Body Parameters

```json
{
  "param.collector.log.elasticsearch.address": "https://10.0.2.15:9200", // required
  "param.collector.log.elasticsearch.username": "elastic",
  "param.collector.log.elasticsearch.password": "123456",
  "param.collector.log.elasticsearch.index": "filebeat-*", 
  "param.collector.log.elasticsearch.match": "keyword1 keyword2 keyword3",  // required
  "param.collector.log.elasticsearch.timeFrom": "2021-07-09T02:01:45.147Z",
  "param.collector.log.elasticsearch.timeTo": "2021-07-09T12:51:45.147Z"
}
```

Parameters in the request body (the longer prefix is omitted here for ease of reading):

- `address` is the address of elasticserach. It is usually one or more HTTP access paths. Required.
- `username` is used to log in to elasticserach.
- `password` is used to log in to elasticserach.
- `index` is the elasticserach index to be queried.
- `match` is the keyword to be searched. Multiple keywords are separated by spaces. Required.
- `timeFrom` is the start time of the search range.
- `timeTo` is the end time of the search range.

#### Status Code

| Code | Description |
|-|-|
| 200 | OK |
| 400 | Bad Request |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### Response Body

The format of the response body is a JSON object, which is key-value pairs that contain a list of processes. The key is `collector.log.elasticsearch`, the value is the query result returned by elasticsearch.

### Example

The execution process of Elasticsearch Collector operation on elasticsearch is as follows:

1. KubeDiag Agent sends an HTTP request to the Elasticsearch Collector, the request type is POST, and the request contains the request body.
1. After the Elasticsearch Collector receives the request, it initializes an elasticsearch client, combines the query statement and sends to the elasticsearch server to obtain query results. 10 query results are displayed by default, sorted by score.
1. If the Elasticsearch Collector completes the collection, it returns a 200 status code to the KubeDiag Agent, and the response body contains the following JSON data:

   ```json
   {
     "collector.log.elasticsearch.hits": {
      "took": 4,
      "timed_out": false,
      "_shards": {
        "total": 1,
        "successful": 1,
        "skipped": 0,
        "failed": 0
      },
      "hits": {
        "total": {
          "value": 10000,
          "relation": "gte"
        },
        "max_score": 11.940922,
        "hits": [
          {
            "_index": "filebeat-8.0.0-2021.07.09-000001",
            "_id": "fbGgmHoBeIRj7XR6x4h5",
            "_score": 11.940922,
            "_source": {
              "@timestamp": "2021-07-12T02:51:45.147Z",
              "log": {
                "offset": 4686649,
                "file": {
                  "path": "/var/log/containers/kube-apiserver-my-node_kube-system_kube-apiserver-2bfcb139999c2bd7c7a53d08bbe12ba814775758e3085c9f77464e218955ea78.log"
                }
              },
              "message": """Trace[1335186706]: ---"About to write a response" 522ms (02:51:00.145)""",
              "stream": "stderr",
              "input": {
                "type": "container"
              },
              "container": {
                "image": {
                  "name": "k8s.gcr.io/kube-apiserver:v1.19.12"
                },
                "id": "2bfcb139999c2bd7c7a53d08bbe12ba814775758e3085c9f77464e218955ea78",
                "runtime": "docker"
              },
              "kubernetes": {
                "namespace_uid": "b4200416-cca9-4370-b01a-24bb74a90d25",
                "pod": {
                  "name": "kube-apiserver-my-node",
                  "uid": "5b3cce30-6837-4748-b2bb-c719ea71c5db",
                  "ip": "10.0.2.15"
                },
                "namespace": "kube-system",
                "labels": {
                  "tier": "control-plane",
                  "component": "kube-apiserver"
                },
                "container": {
                  "name": "kube-apiserver"
                },
                "node": {
                  "name": "my-node",
                  "uid": "ebdaadf7-b055-4859-a3fc-97c433b6169c",
                  "labels": {
                    "beta_kubernetes_io/os": "linux",
                    "kubernetes_io/arch": "amd64",
                    "kubernetes_io/hostname": "my-node",
                    "kubernetes_io/os": "linux",
                    "node-role_kubernetes_io/master": "",
                    "beta_kubernetes_io/arch": "amd64"
                  },
                  "hostname": "my-node"
                }
              },
              "ecs": {
                "version": "1.10.0"
              },
              "host": {
                "name": "helm-filebeat-security-filebeat-5fv4l"
              },
              "agent": {
                "ephemeral_id": "9f68933d-41b7-4fd5-8be9-cb6a09e09e4b",
                "id": "04cf4ce1-6c1b-4256-8d3a-2952e51f0c55",
                "name": "helm-filebeat-security-filebeat-5fv4l",
                "type": "filebeat",
                "version": "8.0.0"
              }
            }
          }
          // ......
          // ......
        ]
      }
    }
   ```

1. If the Elasticsearch Collector fails to collect, it returns 500 status code to the KubeDiag Agent.
