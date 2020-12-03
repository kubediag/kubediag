# Command Executor

命令执行器用于在节点上执行命令，旨在解决故障诊断恢复流程中的扩展性和易用性问题，用户可以自定义在故障诊断恢复流程某个阶段中执行的命令。

## API 设计

Abnormal 中的 `.spec.commandExecutors` 字段用于自定义需要执行的命令，命令执行的结果会被同步到 `.status.commandExecutors` 字段。`commandExecutors` 是一个包含多个 `CommandExecutorSpec` 的列表，`CommandExecutorSpec` 的字段包括：

* `command`：需要执行的命令，该字段是一个数组。
* `type`：命令执行器的类型，该字段支持 InformationCollector、Diagnoser、Recoverer。InformationCollector、Diagnoser 和 Recoverer 类型分别在故障的 `InformationCollecting`、`Diagnosing` 和 `Recovering` 阶段被执行。信息管理器、故障分析链和故障恢复链在执行完相应类型的命令执行器后才会执行 `InformationCollector`、`Diagnoser` 和 `Recoverer`。命令执行器的执行结果不会影响故障的状态迁移，例如 Recoverer 类型的命令执行器失败后故障的状态仍然可以被标记为 `Succeeded`。
* `timeoutSeconds`：命令执行器执行超时时间。如果命令未在超时时间内执行完成，则 `error` 字段会被更新并且执行该命令的进程会被终止。

Abnormal 中的 `.status.commandExecutors` 是一个包含多个 `CommandExecutorStatus` 的列表，`CommandExecutorStatus` 的字段包括：

* `command`：需要执行的命令，与 `CommandExecutorSpec` 保持一致。
* `type`：命令执行器的类型，与 `CommandExecutorSpec` 保持一致。
* `stdout`：命令执行的标准输出，如果命令无标准输出该字段则为空。
* `stderr`：命令执行的标准错误，如果命令无标准错误该字段则为空。
* `error`：命令执行的错误，如果命令执行成功该字段则为空。

## 如何使用

用户可以创建 Abnormal 并在 `.spec.commandExecutors` 字段中包含需要执行的命令，一个典型的 Abnormal 如下所示：

```yaml
apiVersion: diagnosis.netease.com/v1
kind: Abnormal
metadata:
  name: command-executor
spec:
  source: Custom
  commandExecutors:
  - command:
    - "ps"
    - "aux"
    type: InformationCollector
    timeoutSeconds: 5
  - command:
    - "du"
    - "-sh"
    - "/"
    type: Diagnoser
    timeoutSeconds: 10
  - command:
    - "kill"
    - "-10"
    - "1000"
    type: Recoverer
    timeoutSeconds: 5
  nodeName: 10.177.16.22
```

该 Abnormal 定义了三个需要执行的命令：

* `ps aux`：该命令会在 `InformationCollecting` 阶段被执行，执行超时时间为5秒。
* `du -sh /`：该命令会在 `Diagnosing` 阶段被执行，执行超时时间为10秒。
* `kill -10 1000`：该命令会在 `Recovering` 阶段被执行，执行超时时间为5秒。

命令的执行结果结果会被同步到 `.status.commandExecutors` 字段：

```yaml
status:
  commandExecutors:
  - command:
    - "ps"
    - "aux"
    stdout: |
      USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
      root         1  0.6  0.0 226140  9840 ?        Ss   09:47   1:24 /sbin/init splash
      root         2  0.0  0.0      0     0 ?        S    09:47   0:00 [kthreadd]
      root         4  0.0  0.0      0     0 ?        I<   09:47   0:00 [kworker/0:0H]
    type: InformationCollector
  - command:
    - "du"
    - "-sh"
    - "/"
    error: command [du -sh /] timed out
    type: Diagnoser
  - command:
    - "kill"
    - "-10"
    - "1000"
    stderr: exit status 1
    stdout: |
      kill: (1000): No such process
    type: Recoverer
  conditions:
  - lastTransitionTime: "2020-08-31T05:35:14Z"
    status: "True"
    type: InformationCollected
  - lastTransitionTime: "2020-08-31T05:35:19Z"
    status: "True"
    type: Identified
  - lastTransitionTime: "2020-08-31T05:35:19Z"
    status: "True"
    type: Recovered
  identifiable: true
  phase: Succeeded
  recoverable: true
  startTime: "2020-08-31T05:35:14Z"
```

命令的执行结果如下：

* `ps aux`：该命令在 `InformationCollecting` 阶段执行成功，执行的标准输出被记录在 `stdout` 字段。
* `du -sh /`：该命令在 `Diagnosing` 阶段执行失败，命令未在超时时间内执行完成，错误信息被记录在 `error` 字段。
* `kill -10 1000`：该命令在 `Recovering` 阶段执行成功，执行的标准输出被记录在 `stdout` 字段，执行的标准错误被记录在 `stderr` 字段。
