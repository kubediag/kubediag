# Changelog

- [v0.1.2](#v012)
  - [Changes by Kind](#changes-by-kind)
    - [API Change](#api-change)
    - [Bug or Regression](#bug-or-regression)
    - [Other](#other)
  - [Dependencies](#dependencies)
    - [Added](#added)
    - [Changed](#changed)
    - [Removed](#removed)
- [v0.1.1](#v011)
  - [Changes by Kind](#changes-by-kind)
    - [API Change](#api-change)
    - [Bug or Regression](#bug-or-regression)
    - [Other](#other)
  - [Dependencies](#dependencies)
    - [Added](#added)
    - [Changed](#changed)
    - [Removed](#removed)
- [v0.1.0](#v010)
  - [Changes by Kind](#changes-by-kind)
    - [API Change](#api-change)
    - [Bug or Regression](#bug-or-regression)
    - [Other](#other)
  - [Dependencies](#dependencies)
    - [Added](#added)
    - [Changed](#changed)
    - [Removed](#removed)

## v0.1.2

### Changes by Kind

#### API Change

- Remove used APIs including `Label` type and `ReadinessProbe` field. Set `NodeName` as required field in `AbnormalSpec`. ([#36](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/36))
- Implement `CommandExecutor` API. ([#37](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/37))

#### Bug or Regression

- Continue loop on process collector util function error. ([#34](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/34))
- Set timeout for `du` command in in `DiskUsage` function. ([#35](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/35))

#### Other

_Nothing has changed._

## Dependencies

### Added

_Nothing has changed._

### Changed

_Nothing has changed._

### Removed

_Nothing has changed._

## v0.1.1

### Changes by Kind

#### API Change

- Add interfaces for AbnormalProcessor and AbnormalManager. ([#27](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/27))

#### Bug or Regression

- Fix inappropriate privates fields usages in private types. ([#29](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/29))
- Fix duplicated `Created` event issue. ([#30](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/30))

#### Other

- Process collector handler on advertised port and http path `/informationcollector/processcollector`. ([#25](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/25))
- Signal recoverer handler on advertised port and http path `/recoverer/signalrecoverer`. ([#26](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/26))
- Terminating pod diagnoser handler on advertised port and http path `/diagnoser/terminatingpoddiagnoser`. ([#28](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/28))
- Implement abnormal reaper for garbage collection. ([#32](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/32))

## Dependencies

### Added

- github.com/StackExchange/wmi: [cbe66965904d](https://github.com/StackExchange/wmi/tree/cbe66965904d)
- github.com/go-ole/go-ole: [v1.2.4](https://github.com/go-ole/go-ole/tree/v1.2.4)
- github.com/shirou/gopsutil: [v2.20.7](https://github.com/shirou/gopsutil/tree/v2.20.7)

### Changed

_Nothing has changed._

### Removed

_Nothing has changed._

## v0.1.0

### Changes by Kind

#### API Change

- API definitions for Abnormal, InformationCollector, Diagnoser and Recoverer. ([#2](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/2))
- Abnormal will be synchronized by abnormal controller and sent to information manager, diagnoser chain or recoverer chain according to its phase. ([#2](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/2), [#12](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/12))

#### Bug or Regression

_Nothing has changed._

#### Other

- Information manager, diagnoser chain and recoverer chain would send http request with payload of abnormal to information collectors, diagnosers and recoverers. ([#3](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/3))
- Golang pprof handler on advertised port and http path `/debug/pprof`. ([19](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/19))
- Add event recorder for source manager, information manager, diagnoser chain and recoverer chain. ([#20](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/20))
- Implement pod collector handler, container collector handler and pod disk usage diagnoser handler on advertised port and http path `/informationcollector/podcollector`, `/informationcollector/containercollector` and `/diagnoser/poddiskusagediagnoser`. ([#22](https://g.hz.netease.com/k8s/kube-diagnoser/-/merge_requests/22))

## Dependencies

### Added

- github.com/Microsoft/go-winio: [v0.4.14](https://github.com/Microsoft/go-winio/tree/v0.4.14)
- github.com/containerd/containerd: [481103c87933](https://github.com/containerd/containerd/tree/481103c87933)
- github.com/docker/distribution: [0d3efadf0154](https://github.com/docker/distribution/tree/0d3efadf0154)
- github.com/docker/docker: [9dc6525e6118](https://github.com/docker/docker/tree/9dc6525e6118)
- github.com/docker/go-connections: [v0.4.0](https://github.com/docker/go-connections/tree/v0.4.0)
- github.com/go-logr/logr: [v0.1.0](https://github.com/go-logr/logr/tree/v0.1.0)
- github.com/gorilla/mux: [v1.7.4](https://github.com/gorilla/mux/tree/v1.7.4)
- github.com/morikuni/aec: [v1.0.0](https://github.com/morikuni/aec/tree/v1.0.0)
- github.com/onsi/ginkgo: [v1.11.0](https://github.com/onsi/ginkgo/tree/v1.11.0)
- github.com/onsi/gomega: [v1.8.1](https://github.com/onsi/gomega/tree/v1.8.1)
- github.com/opencontainers/go-digest: [v1.0.0](https://github.com/opencontainers/go-digest/tree/v1.0.0)
- github.com/opencontainers/image-spec: [v1.0.1](https://github.com/opencontainers/image-spec/tree/v1.0.1)
- github.com/spf13/cobra: [v0.0.5](https://github.com/spf13/cobra/tree/v0.0.5)
- github.com/spf13/pflag: [v1.0.5](https://github.com/spf13/pflag/tree/v1.0.5)
- github.com/stretchr/testify: [v1.4.0](https://github.com/stretchr/testify/tree/v1.4.0)
- k8s.io/api: [v0.17.2](https://github.com/kubernetes/api/tree/v0.17.2)
- k8s.io/apimachinery: [v0.17.2](https://github.com/kubernetes/apimachinery/tree/v0.17.2)
- k8s.io/client-go: [v0.17.2](https://github.com/kubernetes/client-go/tree/v0.17.2)
- sigs.k8s.io/controller-runtime: [v0.5.0](https://github.com/kubernetes-sigs/controller-runtime/tree/v0.5.0)

### Changed

_Nothing has changed._

### Removed

_Nothing has changed._
