# subpath remount processors

subpath remount processors 是一组 [Processor](../architecture/processor.md)，基于这组 Processor 构建的 OperationSet 可以用于诊断 pod 是否发生了如 [Issue-68211](https://github.com/kubernetes/kubernetes/issues/68211) 所描述的 bug ，并在确诊后主动 recover 。



## 问题聚焦

 [Issue-68211](https://github.com/kubernetes/kubernetes/issues/68211) 所描述的 bug 是: 当某个 pod 使用 subpath 方式 mount 了一个来自 configmap 或 secret 的 volume 时， 一旦更新了 configmap 或 secret 中存储的配置内容，再将 pod 的容器删除（例如执行 docker  rm ），等待 kubelet 主动重建容器， 此时新的容器会无法成功创建。并伴随了报错：

```
OCI runtime create failed: container_linux.go:348: starting container process caused "process_linux.go:402: container init caused "rootfs_linux.go:58: mounting \"/var/lib/kubelet/pods/b9ffd644-af98-11e8-a05e-246e96748774/volume-subpaths/extra-cfg/puppetdb/2\" to rootfs \"/var/lib/docker/overlay2/c8790b7f3f690c1ef7a582672e2d153062ff6b4ed1ee21aab1158897310fd3d1/merged\" at \"/var/lib/docker/overlay2/c8790b7f3f690c1ef7a582672e2d153062ff6b4ed1ee21aab1158897310fd3d1/merged/etc/puppetlabs/puppetdb/conf.d/extra.ini\" caused \"no such file or directory\""": unknown
```



这源于 kubernetes 代码中的一处漏洞，使用 subpath 方式挂载时，会在 node 上重建一个临时数据文件，并建立硬链接指向该文件。 硬链接就是挂载给 pod 的 volume source 。当数据源被修改了， 则会删除旧的临时文件，新建一个临时数据文件，并调整硬链接，而 pod 的 volume source 不会变化。内核的 mount 信息中认为这个 volume 的 source 是指向旧临时文件的硬链接 ，因此 source 已经找不到了，故强行将这个volume mount 到容器中，会报如上错误。解决方法有二：

1. 将 pod 删除，触发重建
2. 主动将这个 volume  umount



该 bug 在 1.19 版本该漏洞得到修复（能探测到并主动 umount）。但尚未 backport 到老版本中。



本文设计的一套 Processor 就是为了快速诊断出这个问题，并进行恢复。



## 实现

subpath remount processors 包括了四个 Processor ：

- pod-detail-collector 一个通用的 Processor , 它会基于请求中的参数，找到指定的 pod ，并记录 pod 的完整信息（包括 spec 和 status ）
- mount-info-collector 一个通用的 Processor ，它会在读取node上的 mountinfo 文件 （路径是宿主机的 /proc/1/mountinfo ）并记录文件内容
- subpath-remount-diagnoser 它将基于前两个 collector 提供的信息， 判断pod是否受相应的 bug 的影响而处于异常。并将 出现问题的 volume 的 source 和 destination 记录下来。
- subpath-remount-recover  它将基于 subpath-remount-diagnoser 提供的信息，在节点上主动umount 出问题的 volume 。这样 kubelet 下次重建容器就会成功。 



```bash
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: pod-detail-collector
spec:
  processor:
    path: /processor/podDetailCollector
    scheme: http
    timeoutSeconds: 60

---
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: mount-info-collector
spec:
  processor:
    path: /processor/mountInfoCollector
    scheme: http
    timeoutSeconds: 60

---

apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: subpath-remount-diagnoser
spec:
  processor:
    path: /processor/subpathRemountDiagnoser
    scheme: http
    timeoutSeconds: 60

---

apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: subpath-remount-recover
spec:
  processor:
    path: /processor/subpathRemountRecover
    scheme: http
    timeoutSeconds: 60

---

apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: subpath-remount-op-set
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: pod-detail-collector
    to:
    - 2
  - id: 2
    operation: mount-info-collector
    to:
    - 3
  - id: 3
    operation: subpath-remount-diagnoser
    to:
    - 4
  - id: 4
    operation: subpath-remount-recover

```





### 举例说明

我们将创建一个使用 subpath 挂载 configmap 的 pod ，并触发上述的 bug ， 然后创建一个 diagnosis 对象来修复它。

1. 创建一个 configmap ：

```
apiVersion: v1
data:
  nginx.log: |
    /var/log/nginx_ingress_controller/access.log {
        rotate 7
        daily
        maxsize 30M
        minsize 10M
        copytruncate
        missingok
        create 0644 root root
    }
    /var/log/nginx_ingress_controller/error.log {
        rotate 7
        daily
        maxsize 30M
        minsize 10M
        copytruncate
        missingok
        create 0644 root root
    }
kind: ConfigMap
metadata:
  name: nginx-logrotate
  namespace: default
```



2. 创建一个测试用的 deployment ，该 pod 会以 subpath 方式挂载 configmap ：

```
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: core2
  name: core2
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: core2
  template:
    metadata:
      labels:
        app: core2
    spec:
      containers:
      - image: hub.c.163.com/public/debian:9.1-deb163
        imagePullPolicy: IfNotPresent
        name: debian
        volumeMounts:
        - mountPath: /etc/logrotate.d/nginx.log
          name: logrotateconf
          subPath: nginx.log
      volumes:
      - configMap:
          defaultMode: 420
          items:
          - key: nginx.log
            path: nginx.log
          name: nginx-logrotate
        name: logrotateconf
```



3. 等待 pod  Running ，修改 configmap 内容，然后到 pod 所在的 node 上，主动将 pod 的容器删除：

   ```
   docker rm -f XXXX
   ```

4. 等待 kubelet 将 pod 的容器重建， 发现始终无法重建。 通过 `kubectl describe pod xxx` 可以看到上文中提到的报错。

5. 创建上文提供的 Operation 和 OperationSet 对象。 然后创建一个 Diagnosis 对象开始分析：

   ```
   apiVersion: diagnosis.kubediag.org/v1
   kind: Diagnosis
   metadata:
     name: subpath-remount-diagnosis
   spec:
     operationSet: subpath-remount-op-set
     nodeName: my-node
     podReference:
       namespace: default
       name: core2-5b89896b96-d44xl
   ```

6. 查看 diagnosis 的进展（此处为了方便阅读对输出内容做了省略）：

   ```
   apiVersion: diagnosis.kubediag.org/v1
   kind: Diagnosis
   metadata:
     name: subpath-remount-diagnosis
     namespace: default
   spec:
     nodeName: my-node
     operationSet: subpath-remount-op-set
     podReference:
       name: ingress-nginx-ttbms
       namespace: kube-system
   status:
     checkpoint:
       nodeIndex: 3
       pathIndex: 0
     conditions:
     - lastTransitionTime: "2021-06-02T07:28:55Z"
       message: Diagnosis is accepted by agent on node my-node
       reason: DiagnosisAccepted
       status: "True"
       type: Accepted
     - lastTransitionTime: "2021-06-02T07:28:55Z"
       message: Diagnosis is completed
       reason: DiagnosisComplete
       status: "True"
       type: Complete
     operationResults:
       collector.system.mountinfo: |
         17 22 0:17 / /sys rw,nosuid,nodev,noexec,relatime shared:7 - sysfs sysfs rw
         36 26 0:32 / /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime shared:20 - cgroup cgroup rw,perf_event
         37 26 0:33 / /sys/fs/cgroup/pids rw,nosuid,nodev,noexec,relatime shared:21 - cgroup cgroup rw,pids
         216 22 254:1 /var/lib/kubelet/pods/29da79e5-386c-4e0d-82c1-215f93ff955d/volumes/kubernetes.io~configmap/logrotateconf/..2021_06_02_06_01_21.244319292/nginx.log//deleted /var/lib/kubelet/pods/29da79e5-386c-4e0d-82c1-215f93ff955d/volume-subpaths/logrotateconf/nginx-ingress-controller/1 rw,relatime shared:1 - ext4 /dev/vda1 rw,errors=remount-ro,data=ordered
         201 70 0:76 / /var/lib/docker/overlay/2bfc1017017a6707aa66bb47a78f25aae5cd54ee50d0a06f33608349267158e2/merged rw,relatime shared:108 - overlay overlay rw,lowerdir=/var/lib/docker/overlay/b6636b86514105ef1d73c7b2d63a7be21e835214e7cbe1de0ccbf7d4a6b3724e/root,upperdir=/var/lib/docker/overlay/2bfc1017017a6707aa66bb47a78f25aae5cd54ee50d0a06f33608349267158e2/upper,workdir=/var/lib/docker/overlay/2bfc1017017a6707aa66bb47a78f25aae5cd54ee50d0a06f33608349267158e2/work
       collector.kubernetes.pod.detail: '{"kind":"Pod","apiVersion":"v1","metadata":{"name":"ingress-nginx-ttbms","generateName":"ingress-nginx-","namespace":"kube-system","selfLink":"/api/v1/namespaces/kube-system/pods/ingress-nginx-ttbms","uid":"29da79e5-386c-4e0d-82c1-215f93ff955d","resourceVersion":"21829399","creationTimestamp":"2021-06-01T09:50:26Z","labels":{"app":"ingress-nginx","controller-revision-hash":"77c587db8","pod-template-generation":"2"},"annotations":{"prometheus.io/port":"10254","prometheus.io/scrape":"true"},"ownerReferences":[{"apiVersion":"apps/v1","kind":"DaemonSet","name":"ingress-nginx","uid":"7b9164a0-efe4-44b5-96c8-00c181912643","controller":true,"blockOwnerDeletion":true}]},"spec":{"volumes":[{"name":"logdir","hostPath":{"path":"/data/log/nginx_ingress_controller","type":""}},{"name":"logrotateconf","configMap":{"name":"nginx-ingress-logrotate","items":[{"key":"nginx.log","path":"nginx.log"}],"defaultMode":420}},{"name":"nginx-ingress-serviceaccount-token-2fplq","secret":{"secretName":"nginx-ingress-serviceaccount-token-2fplq","defaultMode":420}}],"initContainers":[{"name":"adddirperm","image":"hub.c.163.com/kubediag/adddirperm:1.0.0","env":[{"name":"LOG_DIR","value":"/var/log/nginx_ingress_controller"},{"name":"USER_ID","value":"33"}],"resources":{},"volumeMounts":[{"name":"logdir","mountPath":"/var/log/nginx_ingress_controller"},{"name":"nginx-ingress-serviceaccount-token-2fplq","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"IfNotPresent"}],"containers":[{"name":"nginx-ingress-controller","image":"quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.22.0","args":["/nginx-ingress-controller","--default-backend-service=$(POD_NAMESPACE)/default-http-backend","--configmap=$(POD_NAMESPACE)/nginx-configuration","--publish-service=$(POD_NAMESPACE)/ingress-nginx","--annotations-prefix=nginx.ingress.kubernetes.io","--log_dir=/var/log/nginx_ingress_controller","--logtostderr=false"],"ports":[{"name":"http","hostPort":80,"containerPort":80,"protocol":"TCP"},{"name":"https","hostPort":443,"containerPort":443,"protocol":"TCP"}],"env":[{"name":"POD_NAME","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.name"}}},{"name":"POD_NAMESPACE","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.namespace"}}}],"resources":{},"volumeMounts":[{"name":"logdir","mountPath":"/var/log/nginx_ingress_controller"},{"name":"logrotateconf","mountPath":"/etc/logrotate.d/nginx.log","subPath":"nginx.log"},{"name":"nginx-ingress-serviceaccount-token-2fplq","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"readinessProbe":{"httpGet":{"path":"/healthz","port":10254,"scheme":"HTTP"},"timeoutSeconds":1,"periodSeconds":10,"successThreshold":1,"failureThreshold":3},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"IfNotPresent","securityContext":{"capabilities":{"add":["NET_BIND_SERVICE"],"drop":["ALL"]},"runAsUser":33}}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"nginx-ingress-serviceaccount","serviceAccount":"nginx-ingress-serviceaccount","nodeName":"my-node","hostNetwork":true,"securityContext":{},"affinity":{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchFields":[{"key":"metadata.name","operator":"In","values":["my-node"]}]}]}}},"schedulerName":"default-scheduler","priority":0,"enableServiceLinks":true,"preemptionPolicy":"PreemptLowerPriority"},"status":{"phase":"Running","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2021-06-01T09:50:30Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2021-06-02T07:28:01Z","reason":"ContainersNotReady","message":"containers
         with unready status: [nginx-ingress-controller]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2021-06-02T07:28:01Z","reason":"ContainersNotReady","message":"containers
         with unready status: [nginx-ingress-controller]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2021-06-01T09:50:26Z"}],"hostIP":"10.173.32.4","podIP":"10.173.32.4","podIPs":[{"ip":"10.173.32.4"}],"startTime":"2021-06-01T09:50:26Z","initContainerStatuses":[{"name":"adddirperm","state":{"terminated":{"exitCode":0,"reason":"Completed","startedAt":"2021-06-01T09:50:29Z","finishedAt":"2021-06-01T09:50:29Z","containerID":"docker://580d8ebccb7fd1832b773a55a039b5d539b8b282878350d1813e19df9c26278d"}},"lastState":{},"ready":true,"restartCount":0,"image":"hub.c.163.com/kubediag/adddirperm:1.0.0","imageID":"docker-pullable://hub.c.163.com/kubediag/adddirperm@sha256:b15a147b946e4f9602b01c4770a7a2d3622dd4a6abc18b313d6c89ae37d8d401","containerID":"docker://580d8ebccb7fd1832b773a55a039b5d539b8b282878350d1813e19df9c26278d"}],"containerStatuses":[{"name":"nginx-ingress-controller","state":{"waiting":{"reason":"CrashLoopBackOff","message":"back-off
         40s restarting failed container=nginx-ingress-controller pod=ingress-nginx-ttbms_kube-system(29da79e5-386c-4e0d-82c1-215f93ff955d)"}},"lastState":{"terminated":{"exitCode":128,"reason":"ContainerCannotRun","message":"OCI
         runtime create failed: container_linux.go:370: starting container process caused:
         process_linux.go:459: container init caused: rootfs_linux.go:59: mounting \"/var/lib/kubelet/pods/29da79e5-386c-4e0d-82c1-215f93ff955d/volume-subpaths/logrotateconf/nginx-ingress-controller/1\"
         to rootfs at \"/var/lib/docker/overlay/6819afb9daf385c691d2a8c39b7be4c1b12ed355837ebf24b1ab692aae996dac/merged/etc/logrotate.d/nginx.log\"
         caused: no such file or directory: unknown","startedAt":"2021-06-02T07:28:36Z","finishedAt":"2021-06-02T07:28:36Z","containerID":"docker://1ca1a8b5ea3d05b920cb730751f2917a7fc6322759f43779f9bcfbf3e4d08eba"}},"ready":false,"restartCount":8,"image":"quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.22.0","imageID":"docker-pullable://quay.io/kubernetes-ingress-controller/nginx-ingress-controller@sha256:47ef793dc8dfcbf73c9dee4abfb87afa3aa8554c35461635f6539c6cc5073b2c","containerID":"docker://1ca1a8b5ea3d05b920cb730751f2917a7fc6322759f43779f9bcfbf3e4d08eba","started":false}],"qosClass":"BestEffort"}}'
       diagnoser.kubernetes.subpath_remount.bug_link: https://github.com/kubernetes/kubernetes/issues/68211
       diagnoser.kubernetes.subpath_remount.original_destination_path: /var/lib/kubelet/pods/29da79e5-386c-4e0d-82c1-215f93ff955d/volume-subpaths/logrotateconf/nginx-ingress-controller/1
       diagnoser.kubernetes.subpath_remount.original_source_path: /var/lib/kubelet/pods/29da79e5-386c-4e0d-82c1-215f93ff955d/volumes/kubernetes.io~configmap/logrotateconf/..2021_06_02_06_01_21.244319292/nginx.log//deleted
       diagnoser.kubernetes.subpath_remount.result: pod met a bug of subpath-remounting
       recover.kubernetes.subpath_remount.result: Succeesfully umount /var/lib/kubelet/pods/29da79e5-386c-4e0d-82c1-215f93ff955d/volume-subpaths/logrotateconf/nginx-ingress-controller/1 on host
     phase: Succeeded
     startTime: "2021-06-02T07:28:55Z"
     succeededPath:
     - id: 1
       operation: pod-detail-collector
       to:
       - 2
     - id: 2
       operation: mount-info-collector
       to:
       - 3
     - id: 3
       operation: subpath-remount-diagnoser
       to:
       - 4
     - id: 4
       operation: subpath-remount-recover
   ```

7. 等待一会就发现 pod 的状态又恢复 Running 了





