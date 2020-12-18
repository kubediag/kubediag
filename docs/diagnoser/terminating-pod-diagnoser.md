# Terminating Pod Diagnoser

Terminating Pod Diagnoser 是一个 Kube Diagnoser 内置的故障分析器，用于分析节点上无法正常删除的 Pod。

## 如何部署

运行以下命令注册 Terminating Pod Diagnoser 故障分析器到 Kube Diagnoser 中：

```bash
kubectl apply -f config/deploy/terminating_pod_diagnoser.yaml
```

## 请求类型

Terminating Pod Diagnoser 的监听地址与 Kube Diagnoser 一致，默认监听地址为 `0.0.0.0:8090`。HTTP 访问路径为 `/diagnoser/terminatingpoddiagnoser`。Terminating Pod Diagnoser 可以对 POST 请求进行处理：

* 当接收到 POST 请求并且请求体为 Abnormal 结构体时，Terminating Pod Diagnoser 从 Abnormal 的 `.status.context.podInformation` 和 `.status.context.processInformation` 字段获取节点 Pod 列表和节点进程列表并将节点上无法正常删除的 Pod 和无法正常删除 Pod 的 Containerd Shim 进程记录到 Abnormal 的 `.status.context.terminatingPodDiagnosis` 和 `.status.context.signalRecovery` 字段。处理成功返回更新后的 Abnormal 结构体，返回码为 `200`；处理失败则返回请求体中的 Abnormal 结构体，返回码为 `500`。
* 如果 Terminating Pod Diagnoser 分析时无法正常删除的 Pod 已经被成功删除则返回请求体中的 Abnormal 结构体，返回码为 `200`。
* 当接收到其他请求时返回码为 `405`。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.assignedDiagnosers` 字段中包含 `terminating-pod-diagnoser` 来使用 Terminating Pod Diagnoser 分析故障。Terminating Pod Diagnoser 会从 Abnormal 的 `.status.context.podInformation` 和 `.status.context.processInformation` 字段获取节点 Pod 列表和节点进程列表，分析完成后会将无法正常删除 Pod 的 Containerd Shim 进程记录到 Abnormal 的 `.status.context.signalRecovery` 字段，因此 Terminating Pod Diagnoser 通常与 Pod Collector、Process Collector 和 Signal Recoverer 配合使用。一个指定 Terminating Pod Diagnoser 分析故障的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: terminating-pod-diagnosis
  namespace: default
spec:
  assignedDiagnosers:
  - name: terminating-pod-diagnoser
    namespace: kube-diagnoser
  assignedInformationCollectors:
  - name: pod-collector
    namespace: kube-diagnoser
  - name: process-collector
    namespace: kube-diagnoser
  assignedRecoverers:
  - name: signal-recoverer
    namespace: kube-diagnoser
  nodeName: 10.177.16.22
  source: Custom
```

Terminating Pod Diagnoser 成功分析故障后会将节点上无法正常删除的 Pod 和无法正常删除 Pod 的 Containerd Shim 进程记录到 Abnormal 的 `.status.context.terminatingPodDiagnosis` 和 `.status.context.signalRecovery` 字段：

```yaml
status:
  context:
    signalRecovery:
    - pid: 984
      signal: 9
    terminatingPodDiagnosis:
    - apiVersion: v1
      kind: Pod
      metadata:
        creationTimestamp: "2020-08-20T03:16:05Z"
        deletionGracePeriodSeconds: 30
        deletionTimestamp: "2020-08-20T07:31:17Z"
        generateName: nginx-deployment-7fd6966748-
        labels:
          app: nginx
          pod-template-hash: 7fd6966748
        name: nginx-deployment-7fd6966748-6hmf5
        namespace: default
        ownerReferences:
        - apiVersion: apps/v1
          blockOwnerDeletion: true
          controller: true
          kind: ReplicaSet
          name: nginx-deployment-7fd6966748
          uid: 99ec0edc-4fad-4561-bcff-902ed5912284
        resourceVersion: "2010244"
        selfLink: /api/v1/namespaces/default/pods/nginx-deployment-7fd6966748-6hmf5
        uid: d9d8b33b-8e0f-4eab-b53b-23aacc056e9d
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
          lastTransitionTime: "2020-08-20T03:19:52Z"
          status: "True"
          type: Initialized
        - lastProbeTime: null
          lastTransitionTime: "2020-08-20T07:31:10Z"
          status: "False"
          type: Ready
        - lastProbeTime: null
          lastTransitionTime: "2020-08-20T03:19:56Z"
          status: "True"
          type: ContainersReady
        - lastProbeTime: null
          lastTransitionTime: "2020-08-20T03:16:05Z"
          status: "True"
          type: PodScheduled
        containerStatuses:
        - containerID: docker://1b16fdba38d7344e52beb9453b4592bfe2ecd922ed7be4ecbe184c88993796bd
          image: nginx:1.14.2
          imageID: docker-pullable://nginx@sha256:f7988fb6c02e0ce69257d9bd9cf37ae20a60f1df7563c3a2a6abe24160306b8d
          lastState: {}
          name: nginx
          ready: true
          restartCount: 0
          state:
            running:
              startedAt: "2020-08-20T03:19:55Z"
        hostIP: 10.0.2.15
        phase: Running
        podIP: 10.244.0.205
        qosClass: BestEffort
        startTime: "2020-08-20T03:19:52Z"
```
