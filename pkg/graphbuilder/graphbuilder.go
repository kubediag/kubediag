/*
Copyright 2021 The KubeDiag Authors.

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

package graphbuilder

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/util"
)

var (
	graphbuilderSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "graphbuilder_sync_success_count",
			Help: "Counter of successful operationset syncs by graphbuilder",
		},
	)
	graphbuilderSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "graphbuilder_sync_skip_count",
			Help: "Counter of skipped operationset syncs by graphbuilder",
		},
	)
	graphbuilderSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "graphbuilder_sync_error_count",
			Help: "Counter of erroneous operationset syncs by graphbuilder",
		},
	)
)

// GraphBuilder analyzes directed acyclic graph defined in operation set.
type GraphBuilder interface {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// Run runs the GraphBuilder.
	Run(<-chan struct{})
}

// graphBuilder validates directed acyclic graph defined in the operation set and generates paths
// according to the directed acyclic graph.
type graphBuilder struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// eventRecorder knows how to record events on behalf of an EventSource.
	eventRecorder record.EventRecorder
	// scheme defines methods for serializing and deserializing API objects.
	scheme *runtime.Scheme
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// graphBuilderCh is a channel for queuing OperationSets to be processed by graph builder.
	graphBuilderCh chan diagnosisv1.OperationSet
}

// NewGraphBuilder creates a new graph builder.
func NewGraphBuilder(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	graphBuilderCh chan diagnosisv1.OperationSet,
) GraphBuilder {
	metrics.Registry.MustRegister(
		graphbuilderSyncSuccessCount,
		graphbuilderSyncSkipCount,
		graphbuilderSyncErrorCount,
	)
	return &graphBuilder{
		Context:        ctx,
		Logger:         logger,
		client:         cli,
		eventRecorder:  eventRecorder,
		scheme:         scheme,
		cache:          cache,
		graphBuilderCh: graphBuilderCh,
	}
}

// Run runs the graph builder.
// TODO: Prometheus metrics.
func (gb *graphBuilder) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !gb.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process operation sets queuing in graph builder channel.
		case operationSet := <-gb.graphBuilderCh:

			err := gb.client.Get(gb, client.ObjectKey{
				Name: operationSet.Name,
			}, &operationSet)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				gb.addDiagnosisToGraphBuilderQueue(operationSet)
				continue
			}

			// Only process unready operation set.
			if operationSet.Status.Ready {
				graphbuilderSyncSkipCount.Inc()
				continue
			}

			operationSet, err = gb.syncOperationSet(operationSet)
			if err != nil {
				gb.Error(err, "failed to sync OperationSet", "operationSet", operationSet)
				gb.addDiagnosisToGraphBuilderQueue(operationSet)
				continue
			}
			graphbuilderSyncSuccessCount.Inc()
		// Stop graph builder on stop signal.
		case <-stopCh:
			return
		}
	}
}

// syncOperationSet syncs operation sets.
// TODO: Update conditions on error.
func (gb *graphBuilder) syncOperationSet(operationSet diagnosisv1.OperationSet) (diagnosisv1.OperationSet, error) {
	// Build directed graph from adjacency list.
	graph, err := newGraphFromAdjacencyList(operationSet.Spec.AdjacencyList)
	if err != nil {
		return operationSet, err
	}

	nodes := graph.Nodes()
	for nodes.Next() {
		node := nodes.Node()
		toNodes := graph.To(node.ID())
		source := true
		for toNodes.Next() {
			source = false
		}
		if node.ID() == 0 {
			if !source {
				// Return error if start node is the destination of other node in the graph.
				return operationSet, fmt.Errorf("indegree of start node is not 0")
			}
		} else {
			if source {
				// Return error if some node is unreachable from start node in the graph.
				return operationSet, fmt.Errorf("node %d is unreachable from start node", node.ID())
			}
		}
	}

	// Validate the graph does not have any cycles.
	_, err = topo.Sort(graph)
	if err != nil {
		return operationSet, fmt.Errorf("invalid directed acyclic graph: %s", err)
	}

	// Search all paths from start node to any node with outdegree of 0.
	diagnosisPaths, err := searchDiagnosisPaths(graph, len(operationSet.Spec.AdjacencyList))
	if err != nil {
		return operationSet, fmt.Errorf("unable to search diagnosis path: %s", err)
	}

	// Set operation set status with diagnosis paths.
	paths := make([]diagnosisv1.Path, 0)
	for _, diagnosisPath := range diagnosisPaths {
		path := make(diagnosisv1.Path, 0)
		for _, id := range diagnosisPath {
			if operationSet.Spec.AdjacencyList[int(id)].ID != 0 {
				path = append(path, operationSet.Spec.AdjacencyList[int(id)])
			}
		}
		paths = append(paths, path)
	}
	operationSet.Status.Paths = paths
	operationSet.Status.Ready = true

	if err := gb.client.Status().Update(gb, &operationSet); err != nil {
		return operationSet, fmt.Errorf("unable to update OperationSet: %s", err)
	}

	return operationSet, nil
}

// addDiagnosisToGraphBuilderQueue adds OperationSets to the queue processed by graph builder.
func (gb *graphBuilder) addDiagnosisToGraphBuilderQueue(operationSet diagnosisv1.OperationSet) {
	graphbuilderSyncErrorCount.Inc()
	err := util.QueueOperationSet(gb, gb.graphBuilderCh, operationSet)
	if err != nil {
		gb.Error(err, "failed to send operation set to graph builder queue", "operationset", client.ObjectKey{
			Name: operationSet.Name,
		})
	}
}

// newGraphFromAdjacencyList builds a directed graph from a adjacency list.
// TODO: Panic recovery.
func newGraphFromAdjacencyList(adjacencyList []diagnosisv1.Node) (*simple.DirectedGraph, error) {
	graph := simple.NewDirectedGraph()
	for id, node := range adjacencyList {
		if graph.Node(int64(id)) == nil {
			graph.AddNode(simple.Node(id))
		}
		for _, to := range node.To {
			graph.SetEdge(graph.NewEdge(simple.Node(id), simple.Node(to)))
		}
	}

	return graph, nil
}

// searchDiagnosisPaths traverses all nodes in the directed acyclic graph from start node with id of 0.
// It returns all paths from start node to any node with outdegree of 0 and an error.
func searchDiagnosisPaths(graph *simple.DirectedGraph, nodeCount int) ([][]int64, error) {
	var queue NodeQueue
	visited := make([]bool, nodeCount)
	nodePathCache := make([][][]int64, nodeCount)
	sinkNodes := make([]int64, 0)

	// Validate the graph contains start node with id of 0.
	start := graph.Node(0)
	if start == nil {
		return nil, fmt.Errorf("start node not found in graph")
	}

	// Set start node as visited and enqueue all nodes that can reach directly from it.
	visited[start.ID()] = true
	fromNodes := graph.From(start.ID())
	for fromNodes.Next() {
		fromNode := fromNodes.Node()
		queue.Enqueue(fromNode)
	}

	// Initialize node path cache with start node.
	nodePaths := make([][]int64, 0)
	nodePaths = append(nodePaths, []int64{start.ID()})
	nodePathCache[start.ID()] = nodePaths

	for queue.Len() != 0 {
		// Dequeue a node from queue and retrieve all nodes that can reach directly to or from current node.
		current := queue.Dequeue()
		toNodes := graph.To(current.ID())
		fromNodes := graph.From(current.ID())

		// Skip current node if it has already been visited.
		if visited[current.ID()] {
			continue
		}

		// Set current node as visited if all nodes that can reach directly to current node are visited.
		// Otherwise, enqueue current node.
		visited[current.ID()] = true
		for toNodes.Next() {
			toNode := toNodes.Node()
			if !visited[toNode.ID()] {
				visited[current.ID()] = false
				queue.Enqueue(current)
				break
			}
		}

		if visited[current.ID()] {
			// Update node path of current node with visited node that can reach directly to current node.
			toNodes.Reset()
			for toNodes.Next() {
				toNode := toNodes.Node()
				nodePaths := nodePathCache[current.ID()]
				if nodePaths == nil {
					nodePaths = make([][]int64, 0)
				}
				toNodePaths := nodePathCache[toNode.ID()]
				for _, toNodePath := range toNodePaths {
					nodePath := make([]int64, len(toNodePath))
					copy(nodePath, toNodePath)
					nodePath = append(nodePath, current.ID())
					nodePaths = append(nodePaths, nodePath)
				}
				// Node path appended by current node is updated as node path of current node.
				nodePathCache[current.ID()] = nodePaths
			}

			// Enqueue all nodes that can reach directly from current node if current node is visited.
			sink := true
			for fromNodes.Next() {
				sink = false
				fromNode := fromNodes.Node()
				queue.Enqueue(fromNode)
			}
			// Set current node as sink if its outdegree is 0.
			if sink {
				sinkNodes = append(sinkNodes, current.ID())
			}
		}
	}

	// Set diagnosis paths with all node paths of nodes which has outdegree of 0.
	diagnosisPaths := make([][]int64, 0)
	for _, id := range sinkNodes {
		paths := nodePathCache[id]
		diagnosisPaths = append(diagnosisPaths, paths...)
	}

	return diagnosisPaths, nil
}
