# 命令行工具

命令行工具 `kubediag` 可以用于启动 Master 和 Agent 组件。Master 在整个框架中负责控制诊断运维流水线的生成以及诊断的触发。Agent 负责诊断的实际执行。

```bash
kubediag [flags]
```

## 参数

| 参数 | 类型 | 描述 | 默认值 |
|-|-|-|-|
| --mode | string | 指定作为 Master 或者 Agent 运行。 | agent |
| --bind-address | string | 监听的地址。 | 0.0.0.0 |
| --port | int | 监听的端口。 | 8090 |
| --node-name | string | 节点名称。 | |
| --metrics-port | int | 监控指标的暴露端口。 | 10357 |
| --enable-leader-election | bool | 为 Master 开启选主。 | false |
| --docker-endpoint | string | Docker 的监听地址。 | unix:///var/run/docker.sock |
| --webhook-port | int | Webhook 服务器的监听地址。 | 9443 |
| --host | string | Webhook 服务器的 Host。 | |
| --cert-dir | string | 包含服务端密钥和证书的文件地址。 | |
| --repeat-interval | duration | Alertmanager 发送通知成功后第二次发送相同通知的间隔时间。 | 6h |
| --kafka-brokers | strings | 需要连接 Kafka 集群的 Broker 地址列表。 | |
| --kafka-topic | string | 获取消息的 Topic。 | |
| --diagnosis-ttl | duration | 已完成 Diagnosis 的最大保留时间。 | 240h |
| --minimum-diagnosis-ttl-duration | duration | 已完成 Diagnosis 的最小保留时间。 | 30m |
| --maximum-diagnoses-per-node | int32 | 每台节点上已完成 Diagnosis 的最大保留个数。 | 20 |
| --feature-gates | mapStringBool | 表示特性关闭或打开的键值对列表。 | Alertmanager=true,ContainerCollector=true,ContainerdGoroutineCollector=true,CorefileProfiler=false,DockerInfoCollector=true,DockerdGoroutineCollector=true,Eventer=false,GoProfiler=true,KafkaConsumer=true,MountInfoCollector=true,NodeCordon=true,PodCollector=true,ProcessCollector=true,SubpathRemountDiagnoser=true,SystemdCollector=true |
| --data-root | string | 存储 KubeDiag 数据的根目录。 | /var/lib/kubediag |
