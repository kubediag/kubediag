# Cluster Health Evaluator

集群健康评估器用于对集群健康状况进行评估并基于特定异常状况产生 Abnormal 触发自动诊断流程。集群健康评估器可以采集集群中 Workload、Node 和控制面组件的当前状态并计算出集群的健康分。

## 架构

集群健康评估器被集成在故障诊断恢复平台 Master 中。集群健康评估器会定期收集信息并计算集群的健康分，计算健康分的时间间隔由启动参数 `--cluster-health-evaluator-housekeeping-interval` 指定。集群的健康分详情可以通过故障诊断恢复平台 Master 暴露的 `/clusterhealth` 路径进行查看。评估的维度包括：

* Cluster：基于集群内 Workload 和 Node 的健康分进行评估。
* Workload：基于集群内 Pod、Deployment、StatefulSet 和 DaemonSet 的健康分进行评估。
* Pod：基于集群内 Pod 的 Phase 和 ContainerStatuses 进行评估。
* Deployment：基于集群内 Deployment 的 Status 进行评估。
* StatefulSet：基于集群内 StatefulSet 的 Status 进行评估。
* DaemonSet：基于集群内 DaemonSet 的 Status 进行评估。
* Node：基于集群内 Node 的 Conditions 进行评估。

### Cluster 健康

Cluster 健康反映了当前集群的状态，Cluster 健康分计算公式为：

```
Cluster Health Score = (Workload Health Score + Node Health Score) / 2
```

### Workload 健康

Workload 健康反映了当前集群内 Pod、Deployment、StatefulSet 和 DaemonSet 的状态，Workload 健康分计算公式为：

```
Workload Health Score = (Pod Health Score + Deployment Health Score + StatefulSet Health Score + DaemonSet Health Score) / 4
```

### Pod 健康

Pod 健康反映了当前集群内 Pod 的状态，Pod 健康的暴露格式如下：

```json
{
    "Score": 85,
    "Statistics": {
        "Total": 40,
        "Healthy": {
            "Ready": 33,
            "Succeeded": 1
        },
        "Unhealthy": {
            "Failed": 0,
            "Pending": 3,
            "Terminating": 0,
            "Unknown": 0,
            "Unready": 3,
            "ContainerStateReasons": {
                "Completed": 3,
                "Unknown": 3
            }
        }
    }
}
```

Pod 健康字段含义如下：

* Score：Pod 健康分。
* Statistics：集群 Pod 健康统计详情。
  * Total：集群 Pod 数量。
  * Healthy：处于健康状态的 Pod 数量。
    * Ready：Phase 为 Running 并且所有容器处于 Ready 状态的 Pod 数量。
    * Succeeded：Phase 为 Succeeded 的 Pod 数量。
  * Unhealthy：处于非健康状态的 Pod 数量。
    * Unready：Phase 为 Running 并且有容器不处于 Ready 状态的 Pod 数量。
    * Terminating：Phase 为 Running 并且 DeletionTimestamp 不为空的 Pod 数量。
    * Pending：Phase 为 Pending 的 Pod 数量。
    * Failed：Phase 为 Failed 的 Pod 数量。
    * Unknown：Phase 为 Unknown 的 Pod 数量。
    * ContainerStateReasons：Pod 处于非健康状态原因列表以及各原因的统计。
      * CrashLoopBackOff：容器不处于 Ready 状态原因为 CrashLoopBackOff 的 Pod 数量。
      * RunContainerError：容器不处于 Ready 状态原因为 RunContainerError 的 Pod 数量。
      * KillContainerError：容器不处于 Ready 状态原因为 KillContainerError 的 Pod 数量。
      * VerifyNonRootError：容器不处于 Ready 状态原因为 VerifyNonRootError 的 Pod 数量。
      * RunInitContainerError：容器不处于 Ready 状态原因为 RunInitContainerError 的 Pod 数量。
      * CreatePodSandboxError：容器不处于 Ready 状态原因为 CreatePodSandboxError 的 Pod 数量。
      * ConfigPodSandboxError：容器不处于 Ready 状态原因为 ConfigPodSandboxError 的 Pod 数量。
      * KillPodSandboxError：容器不处于 Ready 状态原因为 KillPodSandboxError 的 Pod 数量。
      * SetupNetworkError：容器不处于 Ready 状态原因为 SetupNetworkError 的 Pod 数量。
      * TeardownNetworkError：容器不处于 Ready 状态原因为 TeardownNetworkError 的 Pod 数量。
      * OOMKilled：容器不处于 Ready 状态原因为 OOMKilled 的 Pod 数量。
      * Error：容器不处于 Ready 状态原因为 Error 的 Pod 数量。
      * ContainerCannotRun：容器不处于 Ready 状态原因为 ContainerCannotRun 的 Pod 数量。
      * Unknown：容器不处于 Ready 状态原因未知的 Pod 数量。

Pod 健康分计算公式为：

```
Pod Health Score = 100 * Healthy / Total
```

### Deployment 健康

Deployment 健康反映了当前集群内 Deployment 的状态，Deployment 健康的暴露格式如下：

```json
{
    "Score": 83,
    "Statistics": {
        "Total": 12,
        "Healthy": 10,
        "Unhealthy": {
            "OneQuarterAvailable": 1,
            "TwoQuartersAvailable": 0,
            "ThreeQuartersAvailable": 1,
            "FourQuartersAvailable": 0
        }
    }
}
```

Deployment 健康字段含义如下：

* Score：Deployment 健康分。
* Statistics：集群 Deployment 健康统计详情。
  * Total：集群 Deployment 数量。
  * Healthy：处于健康状态的 Deployment 数量。
  * Unhealthy：处于非健康状态的 Deployment 数量。
    * OneQuarterAvailable：小于25%的副本处于 Available 状态的 Deployment 数量。
    * TwoQuartersAvailable：大于等于25%并且小于50%的副本处于 Available 状态的 Deployment 数量。
    * ThreeQuartersAvailable：大于等于50%并且小于75%的副本处于 Available 状态的 Deployment 数量。
    * FourQuartersAvailable：大于等于75%并且小于100%的副本处于 Available 状态的 Deployment 数量。

Deployment 健康分计算公式为：

```
Deployment Health Score = 100 * (Healthy + OneQuarterAvailable * 0 + TwoQuartersAvailable * 0.25 + ThreeQuartersAvailable * 0.5 + FourQuartersAvailable * 0.75) / Total
```

### StatefulSet 健康

StatefulSet 健康反映了当前集群内 StatefulSet 的状态，StatefulSet 健康的暴露格式如下：

```json
{
    "Score": 50,
    "Statistics": {
        "Total": 2,
        "Healthy": 0,
        "Unhealthy": {
            "OneQuarterReady": 0,
            "TwoQuartersReady": 0,
            "ThreeQuartersReady": 2,
            "FourQuartersReady": 0
        }
    }
}
```

StatefulSet 健康字段含义如下：

* Score：StatefulSet 健康分。
* Statistics：集群 StatefulSet 健康统计详情。
  * Total：集群 StatefulSet 数量。
  * Healthy：处于健康状态的 StatefulSet 数量。
  * Unhealthy：处于非健康状态的 StatefulSet 数量。
    * OneQuarterReady：小于25%的副本处于 Ready 状态的 StatefulSet 数量。
    * TwoQuartersReady：大于等于25%并且小于50%的副本处于 Ready 状态的 StatefulSet 数量。
    * ThreeQuartersReady：大于等于50%并且小于75%的副本处于 Ready 状态的 StatefulSet 数量。
    * FourQuartersReady：大于等于75%并且小于100%的副本处于 Ready 状态的 StatefulSet 数量。

StatefulSet 健康分计算公式为：

```
StatefulSet Health Score = 100 * (Healthy + OneQuarterReady * 0 + TwoQuartersReady * 0.25 + ThreeQuartersReady * 0.5 + FourQuartersReady * 0.75) / Total
```

### DaemonSet 健康

DaemonSet 健康反映了当前集群内 DaemonSet 的状态，DaemonSet 健康的暴露格式如下：

```json
{
    "Score": 100,
    "Statistics": {
        "Total": 9,
        "Healthy": 9,
        "Unhealthy": {
            "OneQuarterAvailableAndScheduled": 0,
            "TwoQuartersAvailableAndScheduled": 0,
            "ThreeQuartersAvailableAndScheduled": 0,
            "FourQuartersAvailableAndScheduled": 0
        }
    }
}
```

DaemonSet 健康字段含义如下：

* Score：DaemonSet 健康分。
* Statistics：集群 DaemonSet 健康统计详情。
  * Total：集群 DaemonSet 数量。
  * Healthy：处于健康状态的 DaemonSet 数量。
  * Unhealthy：处于非健康状态的 DaemonSet 数量。
    * OneQuarterAvailableAndScheduled：小于25%的副本处于 Available 状态并且调度正确的 DaemonSet 数量。
    * TwoQuartersAvailableAndScheduled：大于等于25%并且小于50%的副本处于 Available 状态并且调度正确的 DaemonSet 数量。
    * ThreeQuartersAvailableAndScheduled：大于等于50%并且小于75%的副本处于 Available 状态并且调度正确的 DaemonSet 数量。
    * FourQuartersAvailableAndScheduled：大于等于75%并且小于100%的副本处于 Available 状态并且调度正确的 DaemonSet 数量。

DaemonSet 健康分计算公式为：

```
DaemonSet Health Score = 100 * (Healthy + OneQuarterAvailableAndScheduled * 0 + TwoQuartersAvailableAndScheduled * 0.25 + ThreeQuartersAvailableAndScheduled * 0.5 + FourQuartersAvailableAndScheduled * 0.75) / Total
```

### Node 健康

Node 健康反映了当前集群内 Node 的状态，Node 健康的暴露格式如下：

```json
{
    "Score": 90,
    "Statistics": {
        "Total": 10,
        "Healthy": 9,
        "Unhealthy": {
            "MemoryPressure": 1
        }
    }
}
```

Node 健康字段含义如下：

* Score：Node 健康分。
* Statistics：集群 Node 健康统计详情。
  * Total：集群 Node 数量。
  * Healthy：Condition 为 Ready 并且不为 NodeNetworkUnavailable 的 Node 数量。
  * Unhealthy：处于非健康状态的 Node 数量。
    * MemoryPressure：Condition 为 MemoryPressure 的 Node 数量。
    * DiskPressure：Condition 为 DiskPressure 的 Node 数量。
    * PIDPressure：Condition 为 PIDPressure 的 Node 数量。
    * NodeNetworkUnavailable：Condition 为 NodeNetworkUnavailable 的 Node 数量。

Node 健康分计算公式为：

```
Node Health Score = 100 * Healthy / Total
```

### 基于特定异常状况产生 Abnormal

集群健康评估器可以基于特定异常状况产生 Abnormal，当前支持在集群出现下列异常时产生 Abnormal：

* 集群出现 Pod 的状态为 Terminating 时，集群健康评估器会在该 Pod 的 Namespace 下创建包含 Pod 名称和 UID 并且前缀为 `terminating-pod` 的 Abnormal，如 `terminating-pod.my-pod.fe29d47d-f772-4c95-af5a-93328949f8aa`。

## 如何使用

故障诊断恢复平台 Master 启动后，通过启动参数 `--address` 指定地址的 `/clusterhealth` 路径可以查看集群的健康分详情：

```bash
curl http://0.0.0.0:8089/clusterhealth
```

输出的集群的健康分详情如下：

```json
{
    "Score":89,
    "WorkloadHealth":{
        "Score":79,
        "PodHealth":{
            "Score":85,
            "Statistics":{
                "Total":40,
                "Healthy":{
                    "Ready":33,
                    "Succeeded":1
                },
                "Unhealthy":{
                    "Unready":3,
                    "Terminating":0,
                    "Pending":3,
                    "Failed":0,
                    "Unknown":0,
                    "ContainerStateReasons":{
                        "Completed":3,
                        "Unknown":3
                    }
                }
            }
        },
        "DeploymentHealth":{
            "Score":83,
            "Statistics":{
                "Total":12,
                "Healthy":10,
                "Unhealthy":{
                    "OneQuarterAvailable":1,
                    "TwoQuartersAvailable":0,
                    "ThreeQuartersAvailable":1,
                    "FourQuartersAvailable":0
                }
            }
        },
        "StatefulSetHealth":{
            "Score":50,
            "Statistics":{
                "Total":2,
                "Healthy":0,
                "Unhealthy":{
                    "OneQuarterReady":0,
                    "TwoQuartersReady":0,
                    "ThreeQuartersReady":2,
                    "FourQuartersReady":0
                }
            }
        },
        "DaemonSetHealth":{
            "Score":100,
            "Statistics":{
                "Total":9,
                "Healthy":9,
                "Unhealthy":{
                    "OneQuarterAvailableAndScheduled":0,
                    "TwoQuartersAvailableAndScheduled":0,
                    "ThreeQuartersAvailableAndScheduled":0,
                    "FourQuartersAvailableAndScheduled":0
                }
            }
        }
    },
    "NodeHealth":{
        "Score":100,
        "Statistics":{
            "Total":10,
            "Healthy":10
        }
    }
}
```
