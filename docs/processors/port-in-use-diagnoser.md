# port-in-use diagnoser

port-in-use diagnoser is a [Processor](../design/processor.md), users can build OperationSet based on this processor to detect whether there is a bug as following.

## Background

Here is a question related to nodePort: having reset the range of nodePort, using ports outside default range may lead an error if the nodePort which kebelet listens is already used by a service:

```bash
error: unable to upgrade connection: error dialing backend: dial tcp 127.0.0.1:36527: connect: connection refused
```

And kube Proxy keeps printing error:

```bash
E0924 07:38:31.536854       1 proxier.go:1254] can't open "nodePort for default/kube-processor-service" (:36527/tcp), skipping this nodePort: listen tcp4 :36527: bind: address already in use
E0924 07:38:31.641180       1 proxier.go:1254] can't open "nodePort for default/kube-processor-service" (:36527/tcp), skipping this nodePort: listen tcp4 :36527: bind: address already in use
E0924 07:39:41.732229       1 proxier.go:1254] can't open "nodePort for default/kube-processor-service" (:36527/tcp), skipping this nodePort: listen tcp4 :36527: bind: address already in use
E0924 07:42:14.687111       1 proxier.go:1254] can't open "nodePort for default/kube-processor-service" (:36527/tcp), skipping this nodePort: listen tcp4 :36527: bind: address already in use
E0924 07:47:23.807420       1 proxier.go:1254] can't open "nodePort for default/kube-processor-service" (:36527/tcp), skipping this nodePort: listen tcp4 :36527: bind: address already in use
E0924 07:49:44.709079       1 proxier.go:1254] can't open "nodePort for default/kube-processor-service" (:36527/tcp), skipping this nodePort: listen tcp4 :36527: bind: address already in use
```

Ways to solve it:

1. reset the service.
2. restart kubelet to reset ports kubelet listens.

## Implementation

port-in-use diagnoser detects the bug, and will collect information about the ports kubelet listens and information from specific service if there is the bug.  

```bash
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: port-in-use-diagnoser
spec:
  processor:
    httpServer:
      path: /processor/portInUseDiagnoser
      scheme: http
    timeoutSeconds: 60

---

apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: port-in-use-op-set
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: port-in-use-diagnoser
```

## Example

The execution process of port-in-use diagnoser is as follows:

1. Reset the range of nodePort. Build a service which uses the nodePort kubelet listens。

```bash
$ kubectl get svc 
NAME                     TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
kube-processor-service   NodePort    10.97.104.166   <none>        8080:36527/TCP   42h
```

Ports kubelet listens：

```bash
$ sudo ss -tlnp |grep kubelet
LISTEN   0         4096              127.0.0.1:10248            0.0.0.0:*        users:(("kubelet",pid=12816,fd=29))                                            
LISTEN   0         4096              127.0.0.1:36527            0.0.0.0:*        users:(("kubelet",pid=12816,fd=13))                                            
LISTEN   0         4096                      *:10250                  *:*        users:(("kubelet",pid=12816,fd=27)) 
```

2. Then we get an error executing following command:

```bash
$ kubectl exec -it -n kubediag kubediag-agent-9mhhn bash
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
error: unable to upgrade connection: error dialing backend: dial tcp 127.0.0.1:36527: connect: connection refused
```

3. Create operation and operationset as mentioned in implementation. Then create a diagnosis：

```bash
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: port-in-use
spec:
  operationSet: port-in-use-op-set
  nodeName: ywh-virtualbox
  parameters:
    param.diagnoser.port_in_use.port: "36527"
```

4. Execute following command to view the diagnosis：

```bash
$ kubectl get diagnosis port-in-use -o yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Diagnosis
metadata:
  name: port-in-use
  namespace: default
spec:
  nodeName: ywh-virtualbox
  operationSet: port-in-use-op-set
  parameters:
    param.diagnoser.port_in_use.port: "36527"
status:
  checkpoint:
    nodeIndex: 1
    pathIndex: 0
  conditions:
  - lastTransitionTime: "2021-09-26T02:07:38Z"
    message: Diagnosis is accepted by agent on node ywh-virtualbox
    reason: DiagnosisAccepted
    status: "True"
    type: Accepted
  - lastTransitionTime: "2021-09-26T02:07:38Z"
    message: Diagnosis is completed
    reason: DiagnosisComplete
    status: "True"
    type: Complete
  operationResults:
    diagnoser.kubernetes.port_in_use.kubelet_listen_port: "LISTEN    0         4096
      \            127.0.0.1:10248            0.0.0.0:*        users:((\"kubelet\",pid=12816,fd=29))
      \                                           \nLISTEN    0         4096             127.0.0.1:36527
      \           0.0.0.0:*        users:((\"kubelet\",pid=12816,fd=13))                                            \nLISTEN
      \   0         4096                     *:10250                  *:*        users:((\"kubelet\",pid=12816,fd=27))
      \                                           \n"
    diagnoser.kubernetes.port_in_use.result: 'connection reset by peer has been encountered.
      listen tcp :36527: bind: address already in use'
    diagnoser.kubernetes.port_in_use.service: '{"kind":"Service","apiVersion":"v1","metadata":{"name":"kube-processor-service","namespace":"default","selfLink":"/api/v1/namespaces/default/services/kube-processor-service","uid":"5c9c99a3-7ce0-4084-b29e-a28a4570cdc3","resourceVersion":"759225","creationTimestamp":"2021-09-24T07:38:31Z","labels":{"name":"kube-processor-service"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"labels\":{\"name\":\"kube-processor-service\"},\"name\":\"kube-processor-service\",\"namespace\":\"default\"},\"spec\":{\"ports\":[{\"nodePort\":36527,\"port\":8080,\"protocol\":\"TCP\",\"targetPort\":8080}],\"selector\":{\"app\":\"web\"},\"type\":\"NodePort\"}}\n"},"managedFields":[{"manager":"kubectl-client-side-apply","operation":"Update","apiVersion":"v1","time":"2021-09-24T07:38:31Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubectl.kubernetes.io/last-applied-configuration":{}},"f:labels":{".":{},"f:name":{}}},"f:spec":{"f:externalTrafficPolicy":{},"f:ports":{".":{},"k:{\"port\":8080,\"protocol\":\"TCP\"}":{".":{},"f:nodePort":{},"f:port":{},"f:protocol":{},"f:targetPort":{}}},"f:selector":{".":{},"f:app":{}},"f:sessionAffinity":{},"f:type":{}}}}]},"spec":{"ports":[{"protocol":"TCP","port":8080,"targetPort":8080,"nodePort":36527}],"selector":{"app":"web"},"clusterIP":"10.97.104.166","type":"NodePort","sessionAffinity":"None","externalTrafficPolicy":"Cluster"},"status":{"loadBalancer":{}}}'
  phase: Succeeded
  startTime: "2021-09-26T02:07:38Z"
  succeededPath:
  - id: 1
    operation: port-in-use-diagnoser
```
