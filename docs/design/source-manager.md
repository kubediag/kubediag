# Source Manager

故障管理器是故障诊断恢复平台 Master 中用于产生和获取故障事件的组件。

## 架构

故障管理器中通过 Channel 接收待处理的 Abnormal，故障管理器接收的 Abnormal 类型包括：

* Prometheus 报警（PrometheusAlert）：基于 Prometheus 报警产生的故障，由故障诊断恢复平台 Master 中的 Alertmanager 产生。
* Kubernetes 事件（KubernetesEvent）：基于 Kubernetes 事件产生的故障，由故障诊断恢复平台 Master 中的 Eventer 产生。
* 自定义（Custom）：用于自定义故障，用户可以自定义进行扩展。

用户通过 [AbnormalSource](../api/abnormal-source.md) 资源注册故障事件源到故障诊断恢复平台。故障事件源中定义了基于 Prometheus 报警或 Kubernetes 事件产生故障的模板。故障管理器对 Channel 中的 Abnormal 按类型进行处理：

* Abnormal 未被创建：Alertmanager 或 Eventer 产生 Abnormal 后发送至故障管理器 Channel，故障管理器遍历故障事件源并对 Abnormal 进行匹配，如果产生的 Abnormal 能够成功匹配某个故障事件源中定义的模板则创建 Abnormal。
* Abnormal 已被创建：用户创建自定义故障后由故障诊断恢复平台 Master 发送至故障管理器 Channel。故障管理器更新 Abnormal 的状态为 InformationCollecting，将 Abnormal 发送至信息管理器。

```
--------------------          --------------------          -------------------
|                  |          |                  |          |                 |
| Prometheus Alert |          | Kubernetes Event |          | Custom Abnormal |
|                  |          |                  |          |                 |
--------------------          --------------------          -------------------
         |                             |                             |
         |                             |                             |
         |                             |                             |
        \|/                           \|/                           \|/
  ----------------                -----------             -----------------------                 --------------
  |              |                |         |             |                     |     Abnormal    |            |
  | Alertmanager |                | Eventer |             | Abnormal Controller |<----------------| API Server |
  |              |                |         |             |                     |                 |            |
  ----------------                -----------             -----------------------                 --------------
         |                             |                             |                                 /|\
         |                             |                             |                                  |
         |                             |                             |                                  |
         ------------------------------|------------------------------                                  |
                                       |                                                                |
                                       | Enqueue                                                        |
                                       |                                                                |
                                      \|/                                                               |
                           --------------------------                                                   |
                           |                        |                                                   |
                           | Source Manager Channel |                                                   |
                           |                        |                                                   |
                           --------------------------                                                   |
                                       |                                                                |
                                       |                                                                |
                                       |                                                                |
                                       |-----------------------------------------------------------------
                                       |                           Nonexistent
                              Existent |
                                       |
                                      \|/
                             ----------------------
                             |                    |
                             | InformationManager |
                             |                    |
                             ----------------------
```

### Alertmanager

Alertmanager 通过监听 `/api/v1/alerts` 路径接收 Prometheus 报警。Alertmanager 通过解析 Prometheus 报警产生 Abnormal 并发送至故障管理器 Channel。Abnormal 的 `.spec.prometheusAlert` 字段为 Prometheus 报警详细信息。用户通过在 Prometheus 配置项 [`.alerting.alertmanagers`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#alertmanager_config) 中增加 Kube Diagnoser 来使用该功能，部分 Prometheus 配置如下所示：

```yaml
alerting:
  alertmanagers:
  - path_prefix: /
    scheme: http
    kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
        - kube-diagnoser
    relabel_configs:
    - action: keep
      source_labels:
      - __meta_kubernetes_service_name
      regex: kube-diagnoser-master
    - action: keep
      source_labels:
      - __meta_kubernetes_endpoint_port_name
      regex: http
```

### Eventer

Eventer 通过监听 APIServer 获取 Kubernetes 事件。Eventer 通过解析 Event 产生 Abnormal 并发送至故障管理器 Channel。Abnormal 的 `.spec.kubernetesEvent` 字段为 Event 详细信息。
