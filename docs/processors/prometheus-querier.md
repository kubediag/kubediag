# Prometheus Querier

Prometheus Querier is  a [processor](../design/processor.md), users can use it to query prometheus.

## Background

During the diagnosis process, users may need to query time series data from prometheus. This requirement can be met by introducing Prometheus Querier.

## Implementation

Prometheus Querier is implemented according to [processor](../design/processor.md). You can register Prometheus Querier in KubeDiag through Operation. This Operation is registered by default when KubeDiag is deployed. You  can execute command to view the registered Prometheus Querier:

```bash
$ kubectl get operation prometheus-querier -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: operation
metadata:
  name: prometheus-querier
spec:
  processor:
    path: /processor/prometheusQuerier
    scheme: http
    timeoutSeconds: 10
```

### HTTP Request Format

The request processed by Prometheus Querier must be POST type, and the HTTP request must contain a specific JSON format request body.

#### HTTP Request

POST /processor/prometheusQuerier

#### Request Body Parameters

```json
{
    "prometheus_querier.address": "http://localhost:30090/prometheus/local", // required
    "prometheus_querier.expression": "rate(container_cpu_usage_seconds_total[${interval}])>0.1; sum(up) by (job)", // required
    "prometheus_querier.timeFrom": "5m",
    "prometheus_querier.timeTo": "now",
    "prometheus_querier.step": "1m",
    "interval": "5m", // variable for expression
}
```

Parameters in the request body (the longer prefix is omitted here for ease of reading):

- `address` is the address of prometheus server url. Required.
- `expression` is your query to run with promql (support for `${varname}` syntax variable in expression, variable should be given in request body too. Example: rate(container_cpu_usage_seconds_total[${interval}])>0.1). Multiple keywords are separated by `;`. Required.
- `timeFrom` is the query range start time (an ISO 8601 formatted date string, or as a lookback in h,m,s e.g. 5m). Required for range queries.
- `timeTo` is the query range end time (an ISO 8601 formatted date string, or 'now') (default "now")
- `step` is the step duration of query result (h,m,s e.g 1m) (default "1m")

#### Status code

| Code | Description |
|-|-|
| 200 | OK |
| 400 | Bad Request |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### Response Body

The format of the response body is a JSON object, which is key-value pairs that contain a list of processors. The key is `prometheus_querier.result`, and the value is the query result returned by prometheus.

### Example

The execution process of Prometheus Querier operation on prometheus is as follow:

1. KubeDiag Agent sends an HTTP request to the Prometheus Querier, the request type is POST, and the request contains the request body.
2. After the Prometheus Querier receives the request, it initializes a prometheus api client, combines the query statement and sends to the prometheus server to obtain query results.
3. If the Prometheus Querier completes the querying, it returns a 200 status code to the KubeDiag Agent, and the response body contains the following JSON data:

```json
{"prometheus_querier.result":"[{\"query\":\"rate(container_cpu_usage_seconds_total[5m])\\u003e0.1\",\"result\":[{\"metric\":{\"cpu\":\"total\",\"endpoint\":\"https-metrics\",\"id\":\"/\",\"instance\":\"10.0.2.15:10250\",\"job\":\"kubelet\",\"metrics_path\":\"/metrics/cadvisor\",\"node\":\"ywh-virtualbox\",\"service\":\"kube-prometheus-stack-kubelet\"},\"values\":[[1658816659.313,\"0.12097948378984323\"]]}]},{\"query\":\"sum(up) by (job)\",\"result\":[{\"metric\":{\"job\":\"apiserver\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"coredns\"},\"values\":[[1658816059.315,\"1\"],[1658816119.315,\"1\"],[1658816179.315,\"1\"],[1658816239.315,\"1\"],[1658816599.315,\"2\"],[1658816659.315,\"2\"]]},{\"metric\":{\"job\":\"kube-controller-manager\"},\"values\":[[1658816599.315,\"0\"],[1658816659.315,\"0\"]]},{\"metric\":{\"job\":\"kube-etcd\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"kube-prometheus-stack-grafana\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"kube-prometheus-stack-operator\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"kube-prometheus-stack-prometheus\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"kube-proxy\"},\"values\":[[1658816599.315,\"0\"],[1658816659.315,\"0\"]]},{\"metric\":{\"job\":\"kube-scheduler\"},\"values\":[[1658816599.315,\"0\"],[1658816659.315,\"0\"]]},{\"metric\":{\"job\":\"kube-state-metrics\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"kubediag-agent-metrics-monitor\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]},{\"metric\":{\"job\":\"kubelet\"},\"values\":[[1658816059.315,\"1\"],[1658816599.315,\"3\"],[1658816659.315,\"3\"]]},{\"metric\":{\"job\":\"node-exporter\"},\"values\":[[1658816599.315,\"1\"],[1658816659.315,\"1\"]]}]}]"}
```

4. If the Prometheus Querier fails to query, it returns 500 status code to the KubeDiag Agent.
