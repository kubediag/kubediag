# Statefulset Stuck Diagnoser

Statefulset Stuck Diagnoser is a [Processor](../architecture/processor.md) that can be used to diagnose whether a pod has a bug as described in [Issue-67250](https://github.com/kubernetes/kubernetes/issues/67250), and actively recover after diagnosis.

## Focus

  The bug described by [Issue-67250](https://github.com/kubernetes/kubernetes/issues/67250) is: when creating a Statefulset whose image does not exist or cannot be pulled, it will generate a pod cannot run and is stucked in the `ImagePullBackOff` state. At this time, correct the image of statefulset.spec to will not save the pod. The Statefulset Controller will wait for the state machine of the pod to return to normal, which will never be possible. Therefore, the solution at this time is to manually delete the Pod and trigger the mechanism of Statefulset Controller to rebuild the Pod.
  
  The Processor designed in this article is to quickly diagnose this problem and recover it.

## Implement

Statefulset Stuck Diagnoser will detect whether the Statefulset pod status is ImagePullBackOff and its ownerRef's Statefulset has changed. In this case, the uid of ownerRef will not change, but the resourceversion will. The Statefulset Stuck Diagnoser deletes the pod according to the owner's inconsistency, triggering the reconstruction of the pod.

```bash
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: statefulset-stuck-diagnoser
spec:
  processor:
    path: /processor/statefulsetStuckDiagnoser
    scheme: http
    timeoutSeconds: 60

---

apiVersion: diagnosis.kubediag.org/v1
kind: OperationSet
metadata:
  name: statefulset-stuck-diagnoser
spec:
  adjacencyList:
  - id: 0
    to:
    - 1
  - id: 1
    operation: statefulset-stuck-diagnoser

```

### Example

We'll create a Statefulset and triggers the above bug, then create a diagnosis to fix it.

1. Create a Statefulset with a non-existing image :

   ```yaml
   apiVersion: apps/v1
   kind: StatefulSet
   metadata:
     name: web
   spec:
     selector:
       matchLabels:
         app: nginx
     serviceName: "nginx"
     replicas: 1
     template:
       metadata:
         labels:
           app: nginx
       spec:
         terminationGracePeriodSeconds: 10
         containers:
         - name: nginx
           image: k8s.gcr.io/nginx-slim:0.8-diagnoser
           ports:
           - containerPort: 80
             name: web
   ```

2. Waiting for the pod to be created, the pod has been stuck in the wrong state of pulling the image.

   ```bash
   $ kubectl get pod
   NAME    READY   STATUS         RESTARTS   AGE
   web-0   0/1     ErrImagePull   0          3s
   ```

3. Now we change the Statefulset's image to the correct name:

   ```yaml
   image: k8s.gcr.io/nginx-slim:0.8
   ```

4. Wait for Statefulset to rebuild the pod. It's found that the Pod is still stuck in the ImagePullBackOff state.

   ```bash
   $ kubectl get pod
   NAME    READY   STATUS             RESTARTS   AGE
   web-0   0/1     ImagePullBackOff   0          1m30s
   ```

5. Create the Operation and OperationSet objects provided above, then create a Diagnosis object to start the analysis:

   ```yaml
   apiVersion: diagnosis.kubediag.org/v1
   kind: Diagnosis
   metadata:
     name: statefulset-stuck-diagnoser
   spec:
     operationSet: statefulset-stuck-diagnoser
     nodeName: my-node
     podReference:
       namespace: default
       name: web-0
   ```

6. In the progress of Diagnosis:
   1. Check if the pod is in imagepullbackoff state, if so, go to step 2. If not, go to step 3.
   1. Get the pod's OwnerRef info, so we can get the statefulset name based on OwnerRefs[0].name. Grope the statefulset controller and check its spec.Image, If it is different with the pod spec.Image,  delete the pod.
   1. If the above conditions are not met, do nothing.

7. Wait for a while and the pod returns to Running.
