# Changelog

- [v0.1.0](#first-release)
  - [Changes by Kind](#changes-by-kind)
    - [API Change](#api-change)
    - [Bug or Regression](#bug-or-regression)
    - [Other](#other)
  - [Dependencies](#dependencies)
    - [Added](#added)
    - [Changed](#changed)
    - [Removed](#removed)

## First Release

### Changes by Kind

#### API Change

- API definitions for Abnormal, InformationCollector, Diagnoser and Recoverer.
- Abnormal will be synchronized by abnormal controller and sent to information manager, diagnoser chain or recoverer chain according to its phase.

#### Bug or Regression

_Nothing has changed._

#### Other

- Pod collector handler on advertised port and http path `/informationcollector/podcollector`.
- Container collector handler on advertised port and http path `/informationcollector/containercollector`.
- Pod disk usage diagnoser handler on advertised port and http path `/diagnoser/poddiskusagediagnoser`.
- Promentheus metrics handler on metrics port and http path `/metrics`.
- Golang pprof handler on advertised port and http path `/debug/pprof`.

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
