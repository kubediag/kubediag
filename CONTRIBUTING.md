# 为 KubeDiag 做贡献

欢迎你加入 KubeDiag！请阅读关于如何建立一个开发环境与提交贡献的开发指南。

## 环境准备

* `go` 版本 v1.15+
* `docker` 版本 17.03+
* `kubenetes` 版本 v1.16+

## 如何在本地部署 KubeDiag

通过源代码进行安装部署可以方便我们在本地测试所做的修改。

1. 首先安装 [Cert Manager](https://github.com/jetstack/cert-manager) 用于管理 Webhook Server 的证书。

   ```bash
   # Kubernetes 1.16+
   kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.2/cert-manager.yaml

   # Kubernetes <1.16
   kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.2/cert-manager-legacy.yaml
   ```

1. 安装 KubeDiag，下面介绍两种安装方式

   * 以 Kubectl 的方式安装 KubeDiag

   ```bash
   kubectl create namespace kubediag
   kubectl apply -f config/deploy
   ```

   查看是否所有运行 KubeDiag 的 Pod 处于 Running 状态：

   ```bash
   kubectl get -n kubediag pod -o wide
   ```

   * 以 Kustomize 方式安装 KubeDiag

   在开发环境进行调试时，如果安装了 [`kustomize`](https://github.com/kubernetes-sigs/kustomize) 工具，运行以下命令可以将当前分支上的代码打包成镜像并进行部署：

   ```bash
   make docker-build
   make deploy
   ```

## 快速测试你的 KubeDiag

只有当你的修改通过代码编译与所有的单元测试后，KubeDiag 才会合入你的 PR。此段落帮助你快速开始测试你的修改。

* 编译你的代码进行本地测试

   ```bash
   # 设置你的镜像标签
   export TAG=<TAG_NAME>
   make docker-build
   make deploy
   ```

* 运行测试

   请在提交你的修改前使用以下命令运行所有测试，这样能更大程度地保证你的提交能够被合入代码库。

   ```bash
   make test
   ```

* 卸载你的 KubeDiag

   ```bash
   make uninstall
   ```

## 发现需要改善的内容

当你在此项目中遇到问题时，我们非常乐意接受你反馈的意见。我们非常感激你能够提交一个高可读性的问题报告。

在提交你的问题报告前请事先在 [Issues](https://github.com/kubediag/kubediag/issues) 中查找是否已经有类似的问题与建议。如果已有类似的问题，你可以点击 `subscribe` 来跟踪此问题。

在提交你的问题报告时，请附加上可以重现问题的所有必要步骤。这些信息能够方便我们快速复现与修复你的问题。

## 提交 Issue

GitHub Issue 是追踪 Bug 报告、改善需求，或者反馈例如测试失败等问题的主要途径。你如果发现以下问题，你可以提交 Issue。

* 你发现了一个文档中没有提及的新需求。
* 你发现一个文档中提到了但是没有实现的需求。
* 你发现一个代码漏洞或设计缺陷。
* 你发现文档有错别字、语病、表述不准确或者不完整。

对于错误类型的 Issue，请尽量清楚地描述以下几点：

* 发生了什么样的错误。
* 你的预期输出是什么。
* 如何重现这个错误。
* 当前 Kubernetes 集群环境、主机操作系统版本、主机内核版本、Kube Dignoser 版本等环境信息。

对于功能类型的 Issue，请尽量清楚地描述以下几点：

* 你要添加什么功能。
* 你为什么要添加这个功能。

## GitHub 工作流

本段介绍当你从本地提交代码到 GitHub 时遵循的工作流程。

1. 打开 `https://github.com/kubediag/kubediag`。
1. 点击 `fork` 按钮，建立你个人账户下的 KubeDiag 代码库 `https://github.com/<YOUR_ACCOUNT>/kubediag`。
1. 复制你的个人账户下的 KubeDiag 代码库的 `clone URL`
1. 在你的本地终端窗口使用 `git` 命令将代码库 `clone` 到本地。

   ```bash
   git clone https://github.com/<YOUR_ACCOUNT>/kubediag   

   # 进入你的本地 KubeDiag 项目目录，关联远程分支。
   cd kubediag
   git remote add upstream https://github.com/kubediag/kubediag
   git remote set-url --push upstream no-pushing
   ```

1. 保持你的 `master` 分支与 `upstream` 仓库内容一致，为你需要开发的功能单独创建功能分支。

   ```bash
   git fetch upstream
   git checkout master
   git rebase upstream/master

   # 创建你的功能性分支
   git checkout -b myfeature
   ```

   然后你可以开始在你的 `myfeature` 分支下进行开发。

1. 保持你的开发分支与 `upstream/master` 的同步。

   ```bash
   # 在你的功能性分支下执行
   git fetch upstream
   git rebase upstream/master
   ```

1. 提交你的修改，同时在你的 Commit 中描述你的修改内容与原因

   ```bash
   git commit
   ```

   这个过程中你可能会重复修改多次，使用 `git commit --amend` 修改这次的提交。

1. 将你的修改推送至 GitHub 云端。

   ```bash
   git push -f origin myfeature
   ```

1. 创建 Pull Request

   1. 打开你个人 GitHub 账户下的代码库：`https://github.com/<YOUR_ACCOUNT>/kubediag`。
   1. 在你的 `myfeature` 分支下点击 `Compare & Pull request`。
   1. 检查你的修改内容并添加相应的说明，最后点击 `Create pull request` 完成 PR 的提交。

## 提交 Pull Request

提交 Pull Request 请遵循以下步骤：

1. Fork 代码库至你的个人 GitHub 账户下，并创建新的分支。
1. 根据问题创建 Issue，与项目维护者讨论修改方案。
1. 修改代码，同时修改与此代码有关联的文档。
1. 提交修改前执行 `make manifest` 以更新 Manifest 文件。
1. 提交 Commit 与 Pull Request，添加必要的说明，同时关联你的 Issue。
1. 等待所有 CI 执行完毕并且确定无报错。
1. 等待项目维护者进行 Review。

## 最佳实践

* 在提交大型或者有重大影响的修改前，一定要提前与项目维护者协调沟通。这样能避免多做额外的工作影响修改的合并。
* Commit 信息的描写必须是一句完整的、简明扼要的语句，并且首字母大写。请阅读关于 Commit 的描述规则：[How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/)。
* 在你的 PR 中尽量详细的描述这次修改的内容与原因，方便项目维护者理解你的修改内容。
* 项目维护者在 Review 过程中可能会在你的 Pull Request 下添加评论，你可以考虑采纳维护者的建议或者进行进一步的讨论。在根据建议进行修改后再次提交，并且回复维护者的评论，以便维护者快速感知到你的新提交。
* KubeDiag 项目维护者在 Review 阶段使用 `LGTM` 标签，即表示你的提交将很快被 Merge。

## 代码准则

参阅 [Kubernetes Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)。
