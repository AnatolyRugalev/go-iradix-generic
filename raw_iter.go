// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iradix

// rawIterator visits each of the nodes in the tree, even the ones that are not
// leaves. It keeps track of the effective path (what a leaf at a given node
// would be called), which is useful for comparing trees.
type rawIterator[K keyT, T any] struct {
	// node is the starting node in the tree for the iterator.
	node *Node[K, T]

	// stack keeps track of edges in the frontier.
	stack []rawStackEntry[K, T]

	// pos is the current position of the iterator.
	pos *Node[K, T]

	// path is the effective path of the current iterator position,
	// regardless of whether the current node is a leaf.
	path []K
}

// rawStackEntry is used to keep track of the cumulative common path as well as
// its associated edges in the frontier.
type rawStackEntry[K keyT, T any] struct {
	path  []K
	edges edges[K, T]
}

// Front returns the current node that has been iterated to.
func (i *rawIterator[K, T]) Front() *Node[K, T] {
	return i.pos
}

// Path returns the effective path of the current node, even if it's not actually
// a leaf.
func (i *rawIterator[K, T]) Path() []K {
	return i.path
}

// Next advances the iterator to the next node.
func (i *rawIterator[K, T]) Next() {
	// Initialize our stack if needed.
	if i.stack == nil && i.node != nil {
		i.stack = []rawStackEntry[K, T]{
			{
				edges: edges[K, T]{
					edge[K, T]{node: i.node},
				},
			},
		}
	}

	if len(i.stack) > 0 {
		// Inspect the last element of the stack.
		n := len(i.stack)
		last := i.stack[n-1]
		elem := last.edges[0].node

		// Update the stack.
		if len(last.edges) > 1 {
			i.stack[n-1].edges = last.edges[1:]
		} else {
			i.stack = i.stack[:n-1]
		}

		// Push the edges onto the frontier.
		if len(elem.edges) > 0 {
			path := append(last.path, elem.prefix...)
			i.stack = append(i.stack, rawStackEntry[K, T]{path, elem.edges})
		}

		i.pos = elem
		i.path = append(last.path, elem.prefix...)
		return
	}

	i.pos = nil
	i.path = i.path[:0]
}
