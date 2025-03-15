// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iradix

// PathIterator is used to iterate over a set of nodes from the root
// down to a specified path. This will iterate over the same values that
// the Node.WalkPath method will.
type PathIterator[K keyT, T any] struct {
	node *Node[K, T]
	path []K
}

// Next returns the next node in order
func (i *PathIterator[K, T]) Next() ([]K, T, bool) {
	// This is mostly just an asynchronous implementation of the WalkPath
	// method on the node.
	var zero T
	var leaf *leafNode[K, T]

	for leaf == nil && i.node != nil {
		// visit the leaf values if any
		if i.node.leaf != nil {
			leaf = i.node.leaf
		}

		i.iterate()
	}

	if leaf != nil {
		return leaf.key, leaf.val, true
	}

	return nil, zero, false
}

func (i *PathIterator[K, T]) iterate() {
	// Check for key exhaustion
	if len(i.path) == 0 {
		i.node = nil
		return
	}

	// Look for an edge
	_, i.node = i.node.getEdge(i.path[0])
	if i.node == nil {
		return
	}

	// Consume the search prefix
	if keyHasPrefix(i.path, i.node.prefix) {
		i.path = i.path[len(i.node.prefix):]
	} else {
		// there are no more nodes to iterate through so
		// nil out the node to prevent returning results
		// for subsequent calls to Next()
		i.node = nil
	}
}
