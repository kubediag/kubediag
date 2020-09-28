# Pod Collector

Pod Collector 是一个 Kube Diagnoser 内置的信息采集器，用于采集节点上的 Pod 信息。

## 如何部署

运行以下命令注册 Pod Collector 信息采集器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/deploy/pod_collector.yaml
```

## 请求类型

Pod Collector 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/informationcollector/podcollector`。Pod Collector 可以对 POST 和 GET 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Pod Collector 请求本地 Kubernetes 缓存获取节点 Pod 列表并将 Pod 列表记录到 Abnormal 的 `.status.context.podInformation` 字段。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 当接收到 GET 请求时，Pod Collector 请求本地 Kubernetes 缓存获取节点 Pod 列表。处理成功返回节点 Pod 列表，返回码为 `200`；处理失败则返回错误信息，返回码为 `500`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.assignedInformationCollectors` 字段中包含 `pod-collector` 来使用 Pod Collector 采集信息。一个指定 Pod Collector 采集信息的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: pod-collector
spec:
  assignedInformationCollectors:
  - name: pod-collector
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  source: Custom
```

Pod Collector 成功采集信息后会将节点 Pod 列表记录到 `.status.context.podInformation` 字段：

```yaml
status:
  context:
    podInformation:
    - apiVersion: v1
      kind: Pod
      metadata:
        creationTimestamp: "2020-07-21T08:00:46Z"
        generateName: nginx-deployment-7fd6966748-
        labels:
          app: nginx
          pod-template-hash: 7fd6966748
        name: nginx-deployment-7fd6966748-tsbkq
        namespace: default
        ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: ReplicaSet
          name: nginx-deployment-7fd6966748
          uid: 99ec0edc-4fad-4561-bcff-902ed5912284
        resourceVersion: "1835021"
        selfLink: /api/v1/namespaces/default/pods/nginx-deployment-7fd6966748-tsbkq
        uid: 41290940-f767-48e5-b6db-7ccbe51ad482
      spec:
        containers:
        - image: nginx:1.14.2
          imagePullPolicy: IfNotPresent
          name: nginx
          ports:
          - containerPort: 80
            protocol: TCP
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          volumeMounts:
          - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
            name: default-token-4l8z9
            readOnly: true
        dnsPolicy: ClusterFirst
        enableServiceLinks: true
        nodeName: netease
        priority: 0
        restartPolicy: Always
        schedulerName: default-scheduler
        securityContext: {}
        serviceAccount: default
        serviceAccountName: default
        terminationGracePeriodSeconds: 30
        tolerations:
        - effect: NoExecute
          key: node.kubernetes.io/not-ready
          operator: Exists
          tolerationSeconds: 300
        - effect: NoExecute
          key: node.kubernetes.io/unreachable
          operator: Exists
          tolerationSeconds: 300
        volumes:
        - name: default-token-4l8z9
          secret:
            defaultMode: 420
            secretName: default-token-4l8z9
      status:
        conditions:
        - lastProbeTime: null
          lastTransitionTime: "2020-07-21T08:51:01Z"
          status: "True"
          type: Initialized
        - lastProbeTime: null
          lastTransitionTime: "2020-08-17T02:15:32Z"
          status: "True"
          type: Ready
        - lastProbeTime: null
          lastTransitionTime: "2020-08-17T02:15:32Z"
          status: "True"
          type: ContainersReady
        - lastProbeTime: null
          lastTransitionTime: "2020-07-21T08:51:01Z"
          status: "True"
          type: PodScheduled
        containerStatuses:
        - containerID: docker://5d63078782c69259a890f6ac60451b76fc61058ad743a207490ce960737b308f
          image: nginx:1.14.2
          imageID: docker-pullable://nginx@sha256:f7988fb6c02e0ce69257d9bd9cf37ae20a60f1df7563c3a2a6abe24160306b8d
          lastState:
            terminated:
              containerID: docker://dc36b92852a11a6e5d55ddc5427fbfbc89413e456f6b33fff254fb5ac1c00126
              exitCode: 0
              finishedAt: "2020-08-14T10:17:10Z"
              reason: Completed
              startedAt: "2020-08-14T05:56:21Z"
          name: nginx
          ready: true
          restartCount: 28
          state:
            running:
              startedAt: "2020-08-17T02:15:29Z"
        hostIP: 10.0.2.15
        phase: Running
        podIP: 10.244.0.145
        qosClass: BestEffort
        startTime: "2020-07-21T08:51:01Z"
```
