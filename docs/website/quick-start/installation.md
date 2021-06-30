# 安装

欢迎！在学习如何使用 KubeDiag 来编排诊断运维操作前，您首先需要在 Kubernetes 集群中安装 KubeDiag。

## 系统要求

KubeDiag 需要运行在满足下列条件的 Kubernetes 集群中：

* Kubernetes 1.16+
* Docker 18.09+

## 安装 Cert Manager

我们建议您使用 [Cert Manager](https://github.com/jetstack/cert-manager) 管理 Webhook Server 的证书。如果集群中未部署 Cert Manager 可参考[官方文档](https://cert-manager.io/docs/installation/kubernetes/)进行安装，运行以下命令可以快速安装 Cert Manager：

```bash
# Kubernetes 1.16+
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.2/cert-manager.yaml
```

## 安装 KubeDiag

运行下列命令可以快速安装 KubeDiag：

```bash
kubectl create namespace kubediag
kubectl apply -f https://raw.githubusercontent.com/kubediag/kubediag/master/config/deploy/manifests.yaml
```

查看是否所有运行 KubeDiag 的 Pod 处于 Running 状态：

```bash
kubectl get -n kubediag pod
```

运行 Master 和 Agent 的 Pod 状态全部变为 Ready 则表示安装成功：

```
NAME                                     READY   STATUS    RESTARTS   AGE
kubediag-agent-4m8kd               1/1     Running   0          9s
kubediag-agent-d4k5x               1/1     Running   0          10s
kubediag-agent-hs22s               1/1     Running   0          10s
kubediag-master-67cd58cfdc-djgtb   1/1     Running   0          12s
```
