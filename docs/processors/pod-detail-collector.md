# Pod Detail Collector

Pod Detail Collector 是一个 [Processor](../design/processor.md)，用户可以通过 Pod Detail Collector 采集某个指定 Pod 的信息。

## 背景

在诊断过程中，用户可能需要某个 Pod 的信息。通过引入 Pod Detail  Collector 可以满足该需求。

## 实现

Pod Detail Collector 按照 [Processor](../design/processor.md) 规范实现。通过 Operation 可以在 KubeDiag 中注册 Pod Detail Collector，该 Operation 在 KubeDiag 部署时已默认注册，执行下列命令可以查看已注册的 Pod Detail Collector：

```bash
$ kubectl get operation pod-detail-collector -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  creationTimestamp: "2021-05-17T03:30:46Z"
  generation: 1
  name: pod-detail-collector
  resourceVersion: "4891"
  selfLink: /apis/diagnosis.kubediag.org/v1/operations/pod-detail-collector
  uid: 598cf23b-a6f0-4bfe-86a7-c9ac9e648196
spec:
  processor:
    path: /processor/podDetailCollector
    scheme: http
    timeoutSeconds: 60
```

### HTTP 请求格式

Pod Detail Collector 处理的请求必须为 POST 类型，处理的 HTTP 请求中不包含请求体。

#### HTTP 请求

POST /processor/podDetailCollector

#### 状态码

| Code | Description |
|-|-|
| 200 | OK |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

#### 请求参数

该 Processor 需要指定 Pod 的 namespace 和 name  。这部分信息将会从 Diagnosis 对象的 `spec.podReference` 中获得，并以 map[string]string 的格式作为 body 传给 Processor

#### 返回体参数

JSON 返回体格式为 JSON 对象，对象中包含存有 Pod 列表的 String 键值对。键为 `collector.kubernetes.pod.detail`，值可以被解析为下列数据结构：

| Scheme | Description |
|-|-|
| [][Pod](https://github.com/kubernetes/api/blob/v0.19.11/core/v1/types.go#L3667) | Pod 的元数据信息数组。 |

### 举例说明

一次节点上 Pod 信息采集操作执行的流程如下：

1. KubeDiag Agent 向 Pod Detail Collector 发送 HTTP 请求，请求类型为 POST，请求中不包含请求体。
1. Pod Detail Collector 接收到请求后在节点上调用 Docker 客户端获取指定 Pod 的信息。
1. 如果 Pod Detail Collector 完成采集则向 KubeDiag Agent 返回 200 状态码，返回体中包含如下 JSON 数据：

```json
{
    "collector.kubernetes.pod.detail": '{"kind":"Pod","apiVersion":"v1","metadata":{"name":"kube-scheduler-my-node","namespace":"kube-system","selfLink":"/api/v1/namespaces/kube-system/pods/kube-scheduler-my-node","uid":"64fc326d-1ad6-4807-a9df-c075aea9722a","resourceVersion":"813133","creationTimestamp":"2021-05-17T02:38:42Z","labels":{"component":"kube-scheduler","tier":"control-plane"},"annotations":{"kubernetes.io/config.hash":"dc675150aa3673437a278feada9047bb","kubernetes.io/config.mirror":"dc675150aa3673437a278feada9047bb","kubernetes.io/config.seen":"2021-05-17T10:37:33.814176150+08:00","kubernetes.io/config.source":"file"}},"spec":{"volumes":[{"name":"kubeconfig","hostPath":{"path":"/etc/kubernetes/scheduler.conf","type":"FileOrCreate"}}],"containers":[{"name":"kube-scheduler","image":"k8s.gcr.io/kube-scheduler:v1.16.15","command":["kube-scheduler","--authentication-kubeconfig=/etc/kubernetes/scheduler.conf","--authorization-kubeconfig=/etc/kubernetes/scheduler.conf","--bind-address=127.0.0.1","--kubeconfig=/etc/kubernetes/scheduler.conf","--leader-elect=true","--port=0"],"resources":{"requests":{"cpu":"100m"}},"volumeMounts":[{"name":"kubeconfig","readOnly":true,"mountPath":"/etc/kubernetes/scheduler.conf"}],"livenessProbe":{"httpGet":{"path":"/healthz","port":10259,"host":"127.0.0.1","scheme":"HTTPS"},"initialDelaySeconds":15,"timeoutSeconds":15,"periodSeconds":10,"successThreshold":1,"failureThreshold":8},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"IfNotPresent"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","nodeName":"my-node","hostNetwork":true,"securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"operator":"Exists","effect":"NoExecute"}],"priorityClassName":"system-cluster-critical","priority":2000000000,"enableServiceLinks":true},"status":{"phase":"Running","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2021-06-01T01:49:33Z"},{"type":"Ready","status":"True","lastProbeTime":null,"lastTransitionTime":"2021-06-01T01:50:07Z"},{"type":"ContainersReady","status":"True","lastProbeTime":null,"lastTransitionTime":"2021-06-01T01:50:07Z"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2021-06-01T01:49:33Z"}],"hostIP":"10.0.2.15","podIP":"10.0.2.15","podIPs":[{"ip":"10.0.2.15"}],"startTime":"2021-06-01T01:49:33Z","containerStatuses":[{"name":"kube-scheduler","state":{"running":{"startedAt":"2021-06-01T01:49:36Z"}},"lastState":{"terminated":{"exitCode":2,"reason":"Error","startedAt":"2021-05-31T02:08:27Z","finishedAt":"2021-05-31T10:29:55Z","containerID":"docker://b4a302f168490ab2d81f13dadafe122c3b53cbcd9ed55512b6fc972bbda4795d"}},"ready":true,"restartCount":31,"image":"k8s.gcr.io/kube-scheduler:v1.16.15","imageID":"docker-pullable://k8s.gcr.io/kube-scheduler@sha256:d9156baf649cd356bad6be119a62cf137b73956957604275ab8e3008bee96c8f","containerID":"docker://5c1138bd4cd6600f404225fdd335009f52512161b18376cd3e528577808dd338","started":true}],"qosClass":"Burstable"}},......'
}
```

1. 如果 Pod Detail Collector 采集失败则向 KubeDiag Agent 返回 500 状态码。
