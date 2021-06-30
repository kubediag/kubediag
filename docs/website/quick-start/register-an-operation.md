# 注册运维操作

现在让我们将诊断操作注册到集群中。

## 撰写一个简单的诊断操作

下列是一个通过 HTTP 服务器的形式提供运维操作的简单示例：

```go
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "time"

    "github.com/gorilla/mux"
)

var (
    cache = map[string]string{
        "a": "1",
        "b": "2",
        "c": "3",
        "d": "4",
    }
)

func main() {
    address := flag.String("address", "0.0.0.0", "The address on which to advertise.")
    port := flag.String("port", "8000", "The port to serve on.")
    flag.Parse()

    r := mux.NewRouter()
    r.HandleFunc("/", handler)
    srv := &http.Server{
        Handler:      r,
        Addr:         fmt.Sprintf("%s:%s", *address, *port),
        WriteTimeout: 30 * time.Second,
        ReadTimeout:  30 * time.Second,
    }

    log.Fatal(srv.ListenAndServe())
}

func handler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "POST":
        // Parse the request payload into a map[string]string.
        body, err := ioutil.ReadAll(r.Body)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        parameters := make(map[string]string)
        err = json.Unmarshal(body, &parameters)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        // Update cache with parameters.
        for key, value := range parameters {
            log.Printf("Update cache with %s:%s", key, value)
            cache[key] = value
        }
        data, err := json.Marshal(cache)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Response with cache.
        w.Header().Set("Content-Type", "application/json")
        w.Write(data)
    default:
        http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
    }
}
```

## 将诊断程序部署到集群中

通过下列 YAML 文件您可以将该程序部署到 Kubernetes 集群中：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: http-operation
  namespace: kubediag
  labels:
    operation: cache
    type: http
spec:
  containers:
  - name: http-operation
    image: hub.c.163.com/kubediag/http-operation:0.2.0
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: http-operation
  namespace: kubediag
spec:
  selector:
    operation: cache
    type: http
  ports:
  - name: http
    port: 80
    targetPort: 80
```

创建运行该程序的 Pod 以及用于访问的 Service：

```bash
kubectl apply -f samples/http-operation/manifests/pod.yaml
kubectl apply -f samples/http-operation/manifests/service.yaml
```

该程序启动了一个 HTTP 服务器并初始化了内容如下的缓存：

```json
{
  "a":"1",
  "b":"2",
  "c":"3",
  "d":"4"
}
```

查看该程序是否部署成功：

```bash
$ kubectl get -n kubediag pod http-operation
NAME             READY   STATUS    RESTARTS   AGE
http-operation   1/1     Running   0          4s
$ kubectl get -n kubediag service http-operation
NAME             TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
http-operation   ClusterIP   10.96.73.28   <none>        80/TCP    18s
```

通过 HTTP 请求可以增加或修改缓存中的键值对，HTTP 服务器会返回修改后缓存中的所有数据：

```bash
$ curl -X POST --data '{"a":"100","e":"5"}' http://10.96.73.28:80
{"a":"100","b":"2","c":"3","d":"4","e":"5"}
```

## 将诊断操作注册到 KubeDiag 中

Operation 中定义了诊断操作的详细信息，通过创建下列 Operation 可以将示例中的操作注册到 KubeDiag 中：

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Operation
metadata:
  name: http-operation
spec:
  processor:
    externalIP: http-operation.kubediag.svc.cluster.local
    externalPort: 80
    path: /
    scheme: http
    timeoutSeconds: 30
```

创建用于注册诊断操作的 Operation：

```bash
kubectl apply -f samples/http-operation/manifests/operation.yaml
```
