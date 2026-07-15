// Copyright 2025 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package scc contains an implementation of Tarjan's algorithm, which converts
// a directed graph into a DAG of strongly-connected components (subgraphs
// such that every node is reachable from every other node).
package scc

import (
	"iter"
	"slices"
	"unsafe"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// Graph is a "local" representation of a directed graph, which exposes the
// outgoing edges (i.e., dependencies) from some node.
type Graph[Node any] func(Node) iter.Seq[Node]

// DAG represents the strongly connected component DAG of some arbitrary
// directed graph.
type DAG[Node comparable] struct {
	keys       map[Node]int      // Indexes into the scc that K is part of.
	components []Component[Node] // Topologically sorted
}

// Component is a strongly connected component.
type Component[Node comparable] struct {
	dag     *DAG[Node]
	members []Node
	deps    []int
}

// Sort sorts the strongly connected components of a directed graph
// represented by deps, using Tarjan's algorithm.
func Sort[Node comparable](root Node, graph Graph[Node]) *DAG[Node] {
	out := &DAG[Node]{keys: make(map[Node]int)}
	sorter := &tarjan[Node]{
		graph: graph,
		dag:   out,

		metadata: make(map[Node]*metadata),
		depset:   make(map[int]struct{}),
	}
	sorter.rec(root)

	return out
}

// ForNode returns the component for some node, or nil if that node is not in
// the graph.
func (d *DAG[Node]) ForNode(node Node) *Component[Node] {
	idx, ok := d.keys[node]
	if !ok {
		return nil
	}
	return &d.components[idx]
}

// Topological returns an iterator over the components in topological order.
func (d *DAG[Node]) Topological() iter.Seq[*Component[Node]] {
	return func(yield func(*Component[Node]) bool) {
		for i := range d.components {
			if !yield(&d.components[i]) {
				return
			}
		}
	}
}

// Members returns the members of a component.
func (c *Component[Node]) Members() []Node {
	return c.members
}

// Deps ranges over the direct dependencies of this component.
func (c *Component[Node]) Deps() iter.Seq[*Component[Node]] {
	return func(yield func(*Component[Node]) bool) {
		for _, i := range c.deps {
			if !yield(&c.dag.components[i]) {
				return
			}
		}
	}
}

// Index returns this component's position in topological order.
func (c *Component[Node]) Index() int {
	return xunsafe.Sub(c, unsafe.SliceData(c.dag.components))
}

// tarjan is the state needed to execute Tarjan's recursive SCC algorithm.
//
// See https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm
type tarjan[Node comparable] struct {
	graph Graph[Node]
	dag   *DAG[Node]

	index    int
	stack    []Node
	metadata map[Node]*metadata

	// Used for building the dependency set of component.
	depset map[int]struct{}
}

// metadata is per-mode metadata associated with a node in [tarjan].
type metadata struct {
	index, low int
	onStack    bool
}

// rec is the recursive step of Tarjan's algorithm.
func (s *tarjan[Node]) rec(node Node) *metadata {
	meta := &metadata{
		index:   s.index,
		low:     s.index,
		onStack: true,
	}
	debug.Log(nil, "rec", "%v, index: %d", node, meta.index)

	s.metadata[node] = meta
	s.index++
	offset := len(s.stack)
	s.stack = append(s.stack, node)

	for dep := range s.graph(node) {
		m := s.metadata[dep]
		if m == nil {
			m = s.rec(dep)
			debug.Log(nil, "dep", "%v->%v, low: %d/%d", node, dep, meta.low, m.low)
			meta.low = min(meta.low, m.low)
			continue
		}

		if m.onStack {
			debug.Log(nil, "dep", "%v->%v, low: %d/%d", node, dep, meta.low, m.index)
			meta.low = min(meta.low, m.index)
		}
	}

	if meta.index == meta.low {
		scc := Component[Node]{
			dag:     s.dag,
			members: slices.Clone(s.stack[offset:]),
		}
		s.stack = s.stack[:offset]
		debug.Log(nil, "scc", "%v%v", s.stack, scc.members)

		for _, node := range scc.members {
			s.metadata[node].onStack = false

			s.dag.keys[node] = len(s.dag.components)
			for dep := range s.graph(node) {
				n, ok := s.dag.keys[dep]
				if ok && n < len(s.dag.components) {
					s.depset[n] = struct{}{}
				}
			}

			scc.deps = make([]int, 0, len(s.depset))
			for i := range s.depset {
				scc.deps = append(scc.deps, i)
			}
			slices.Sort(scc.deps)
			clear(s.depset)
		}
		debug.Log(nil, "deps", "%v", scc.deps)

		s.dag.components = append(s.dag.components, scc)
	}

	return meta
}
