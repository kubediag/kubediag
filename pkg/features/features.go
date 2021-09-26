/*
Copyright 2020 The KubeDiag Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package features

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/featuregate"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// Alertmanager can handle valid post alerts requests.
	//
	// Mode: master
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	Alertmanager featuregate.Feature = "Alertmanager"
	// Eventer generates diagnoses from kubernetes events.
	//
	// Mode: master
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	Eventer featuregate.Feature = "Eventer"
	// KafkaConsumer can processs valid kafka messages.
	//
	// Mode: master
	// Owner: @huangjiuyuan
	// Alpha: 0.2.0
	KafkaConsumer featuregate.Feature = "KafkaConsumer"

	// PodCollector manages information of pods.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	PodCollector featuregate.Feature = "PodCollector"
	// ContainerCollector manages information of all containers on the node.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	ContainerCollector featuregate.Feature = "ContainerCollector"
	// ProcessCollector manages information of all processes on the node.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	ProcessCollector featuregate.Feature = "ProcessCollector"
	// SystemdCollector manages information of systemd on the node.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	SystemdCollector featuregate.Feature = "SystemdCollector"
	// SignalRecoverer manages recovery that sending signal to processes.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.1.5
	SignalRecoverer featuregate.Feature = "SignalRecoverer"
	// CoreFileProfiler manages corefiles and supports gdb debugging.
	//
	// Mode: agent
	// Owner: @fzu-huang
	// Alpha: 0.2.0
	CoreFileProfiler featuregate.Feature = "CoreFileProfiler"
	// DockerInfoCollector fetches system-wide information on docker.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.2.0
	DockerInfoCollector featuregate.Feature = "DockerInfoCollector"
	// DockerdGoroutineCollector retrives dockerd goroutine on the node.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.2.0
	DockerdGoroutineCollector featuregate.Feature = "DockerdGoroutineCollector"
	// ContainerdGoroutineCollector retrives containerd goroutine on the node.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.2.0
	ContainerdGoroutineCollector featuregate.Feature = "ContainerdGoroutineCollector"
	// NodeCordon marks node as unschedulable.
	//
	// Mode: agent
	// Owner: @huangjiuyuan
	// Alpha: 0.2.0
	NodeCordon featuregate.Feature = "NodeCordon"
	// GoProfiler manages go profiler.
	//
	// Mode: agent
	// Owner: @April-Q
	// Alpha: 0.2.0
	GoProfiler featuregate.Feature = "GoProfiler"
	// MountInfoCollector manages mount info on node
	//
	// Mode: agent
	// Owner: @fzu-huang
	// Alpha: 0.2.0
	MountInfoCollector featuregate.Feature = "MountInfoCollector"
	// SubpathRemountDiagnoser diagnosis whether abnormal came from subpath-remount-bug
	//
	// BugLink: https://github.com/kubernetes/kubernetes/issues/68211
	// Mode: agent
	// Owner: @fzu-huang
	// Alpha: 0.2.0
	SubpathRemountDiagnoser featuregate.Feature = "SubpathRemountDiagnoser"
	// ElasticsearchCollector retrieves log info from elasticsearch.
	//
	// Mode: agent
	// Owner: @April-Q
	// Alpha: 0.2.0
	ElasticsearchCollector featuregate.Feature = "ElasticsearchCollector"
	// PortInUseDiagnoser diagnosis whether abnormal came from port-in-use-bug.
	//
	// Mode: agent
	// Owner: @Harmol
	// Alpha: 0.2.0
	PortInUseDiagnoser featuregate.Feature = "PortInUseDiagnoser"
)

var (
	featureGateInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "feature_gate",
			Help: "Information about feature gate",
		},
		[]string{"name", "enabled"},
	)
)

var defaultKubeDiagFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	Alertmanager:                 {Default: true, PreRelease: featuregate.Alpha},
	Eventer:                      {Default: false, PreRelease: featuregate.Alpha},
	KafkaConsumer:                {Default: true, PreRelease: featuregate.Alpha},
	PodCollector:                 {Default: true, PreRelease: featuregate.Alpha},
	ContainerCollector:           {Default: true, PreRelease: featuregate.Alpha},
	ProcessCollector:             {Default: true, PreRelease: featuregate.Alpha},
	SystemdCollector:             {Default: true, PreRelease: featuregate.Alpha},
	SignalRecoverer:              {Default: true, PreRelease: featuregate.Alpha},
	CoreFileProfiler:             {Default: false, PreRelease: featuregate.Alpha},
	DockerInfoCollector:          {Default: true, PreRelease: featuregate.Alpha},
	DockerdGoroutineCollector:    {Default: true, PreRelease: featuregate.Alpha},
	ContainerdGoroutineCollector: {Default: true, PreRelease: featuregate.Alpha},
	NodeCordon:                   {Default: true, PreRelease: featuregate.Alpha},
	GoProfiler:                   {Default: true, PreRelease: featuregate.Alpha},
	MountInfoCollector:           {Default: true, PreRelease: featuregate.Alpha},
	SubpathRemountDiagnoser:      {Default: true, PreRelease: featuregate.Alpha},
	ElasticsearchCollector:       {Default: true, PreRelease: featuregate.Alpha},
	PortInUseDiagnoser:           {Default: true, PreRelease: featuregate.Alpha},
}

// KubeDiagFeatureGate indicates whether a given feature is enabled or not and stores flag gates for known features.
type KubeDiagFeatureGate interface {
	// Enabled returns true if the key is enabled.
	Enabled(featuregate.Feature) bool
	// KnownFeatures returns a slice of strings describing the known features.
	KnownFeatures() []string
	// SetFromMap stores flag gates for known features from a map[string]bool or returns an error.
	SetFromMap(map[string]bool) error
}

// kubediagFeatureGate manages features of kubediag.
type kubediagFeatureGate struct {
	// lock guards writes to known and enabled.
	lock sync.Mutex
	// known holds a map[featuregate.Feature]featuregate.FeatureSpec.
	known *atomic.Value
	// enabled holds a map[featuregate.Feature]bool.
	enabled *atomic.Value
}

var metricRegistry sync.Once

// NewFeatureGate creates a new KubeDiagFeatureGate.
func NewFeatureGate() KubeDiagFeatureGate {
	metricRegistry.Do(func() { metrics.Registry.MustRegister(featureGateInfo) })
	// Set default known features.
	knownMap := make(map[featuregate.Feature]featuregate.FeatureSpec)
	for key, value := range defaultKubeDiagFeatureGates {
		knownMap[key] = value
	}
	known := new(atomic.Value)
	known.Store(knownMap)

	// Set default enabled features.
	enabledMap := make(map[featuregate.Feature]bool)
	for key, value := range defaultKubeDiagFeatureGates {
		enabledMap[key] = value.Default
	}
	enabled := new(atomic.Value)
	enabled.Store(enabledMap)

	return &kubediagFeatureGate{
		known:   known,
		enabled: enabled,
	}
}

// Enabled returns true if the key is enabled.
func (kf *kubediagFeatureGate) Enabled(key featuregate.Feature) bool {
	if value, ok := kf.enabled.Load().(map[featuregate.Feature]bool)[key]; ok {
		return value
	}
	if value, ok := kf.known.Load().(map[featuregate.Feature]featuregate.FeatureSpec)[key]; ok {
		return value.Default
	}

	return false
}

// KnownFeatures returns a slice of strings describing the known features.
// Deprecated and GA features are hidden from the list.
func (kf *kubediagFeatureGate) KnownFeatures() []string {
	var known []string
	for key, value := range kf.known.Load().(map[featuregate.Feature]featuregate.FeatureSpec) {
		if value.PreRelease == featuregate.GA || value.PreRelease == featuregate.Deprecated {
			continue
		}
		known = append(known, fmt.Sprintf("%s=true|false (%s - default=%t)", key, value.PreRelease, value.Default))
	}
	sort.Strings(known)

	return known
}

// SetFromMap stores flag gates for known features from a map[string]bool or returns an error.
func (kf *kubediagFeatureGate) SetFromMap(featureMap map[string]bool) error {
	kf.lock.Lock()
	defer kf.lock.Unlock()

	// Copy existing state.
	knownMap := make(map[featuregate.Feature]featuregate.FeatureSpec)
	for key, value := range kf.known.Load().(map[featuregate.Feature]featuregate.FeatureSpec) {
		knownMap[key] = value
	}
	enabledMap := make(map[featuregate.Feature]bool)
	for key, value := range kf.enabled.Load().(map[featuregate.Feature]bool) {
		enabledMap[key] = value
	}

	// Set flag gates for known features from a map[string]bool.
	for key, value := range featureMap {
		key := featuregate.Feature(key)
		featureSpec, ok := knownMap[key]
		if !ok {
			return fmt.Errorf("unrecognized feature gate: %s", key)
		}
		if featureSpec.LockToDefault && featureSpec.Default != value {
			return fmt.Errorf("cannot set feature gate %v to %v, feature is locked to %v", key, value, featureSpec.Default)
		}
		enabledMap[key] = value
	}

	// Persist changes.
	kf.known.Store(knownMap)
	kf.enabled.Store(enabledMap)

	return nil
}

// Collect feature gate metrics.
func Collect(features KubeDiagFeatureGate) {
	featureGateInfo.Reset()
	for key := range defaultKubeDiagFeatureGates {
		if features.Enabled(key) {
			featureGateInfo.WithLabelValues(string(key), "true").Set(1)
		} else {
			featureGateInfo.WithLabelValues(string(key), "false").Set(1)
		}
	}
}
