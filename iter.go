// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iradix

// Iterator is used to iterate over a set of nodes
// in pre-order
type Iterator[K keyT, T any] struct {
	node  *Node[K, T]
	stack []edges[K, T]
}

// SeekPrefixWatch is used to seek the iterator to a given prefix
// and returns the watch channel of the finest granularity
func (i *Iterator[K, T]) SeekPrefixWatch(prefix []K) (watch <-chan struct{}) {
	// Wipe the stack
	i.stack = nil
	n := i.node
	watch = n.mutateCh
	search := prefix
	for {
		// Check for key exhaustion
		if len(search) == 0 {
			i.node = n
			return
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			i.node = nil
			return
		}

		// Update to the finest granularity as the search makes progress
		watch = n.mutateCh

		// Consume the search prefix
		switch {
		case keyHasPrefix(search, n.prefix):
			search = search[len(n.prefix):]
		case keyHasPrefix(n.prefix, search):
			i.node = n
			return
		default:
			i.node = nil
			return
		}
	}
}

// SeekPrefix is used to seek the iterator to a given prefix
func (i *Iterator[K, T]) SeekPrefix(prefix []K) {
	i.SeekPrefixWatch(prefix)
}

func (i *Iterator[K, T]) recurseMin(n *Node[K, T]) *Node[K, T] {
	// Traverse to the minimum child
	if n.leaf != nil {
		return n
	}
	nEdges := len(n.edges)
	if nEdges > 1 {
		// Add all the other edges to the stack (the min node will be added as
		// we recurse)
		i.stack = append(i.stack, n.edges[1:])
	}
	if nEdges > 0 {
		return i.recurseMin(n.edges[0].node)
	}
	// Shouldn't be possible
	return nil
}

// SeekLowerBound is used to seek the iterator to the smallest key that is
// greater or equal to the given key. There is no watch variant as it's hard to
// predict based on the radix structure which node(s) changes might affect the
// result.
func (i *Iterator[K, T]) SeekLowerBound(key []K) {
	// Wipe the stack. Unlike Prefix iteration, we need to build the stack as we
	// go because we need only a subset of edges of many nodes in the path to the
	// leaf with the lower bound. Note that the iterator will still recurse into
	// children that we don't traverse on the way to the reverse lower bound as it
	// walks the stack.
	i.stack = []edges[K, T]{}
	// i.node starts off in the common case as pointing to the root node of the
	// tree. By the time we return we have either found a lower bound and setup
	// the stack to traverse all larger keys, or we have not and the stack and
	// node should both be nil to prevent the iterator from assuming it is just
	// iterating the whole tree from the root node. Either way this needs to end
	// up as nil so just set it here.
	n := i.node
	i.node = nil
	search := key

	found := func(n *Node[K, T]) {
		i.stack = append(
			i.stack,
			edges[K, T]{edge[K, T]{node: n}},
		)
	}

	findMin := func(n *Node[K, T]) {
		n = i.recurseMin(n)
		if n != nil {
			found(n)
			return
		}
	}

	for {
		// Compare current prefix with the search key's same-length prefix.
		var prefixCmp int
		if len(n.prefix) < len(search) {
			prefixCmp = keyCompare(n.prefix, search[0:len(n.prefix)])
		} else {
			prefixCmp = keyCompare(n.prefix, search)
		}

		if prefixCmp > 0 {
			// Prefix is larger, that means the lower bound is greater than the search
			// and from now on we need to follow the minimum path to the smallest
			// leaf under this subtree.
			findMin(n)
			return
		}

		if prefixCmp < 0 {
			// Prefix is smaller than search prefix, that means there is no lower
			// bound
			i.node = nil
			return
		}

		// Prefix is equal, we are still heading for an exact match. If this is a
		// leaf and an exact match we're done.
		if n.leaf != nil && keyEqual(n.leaf.key, key) {
			found(n)
			return
		}

		// Consume the search prefix if the current node has one. Note that this is
		// safe because if n.prefix is longer than the search slice prefixCmp would
		// have been > 0 above and the method would have already returned.
		search = search[len(n.prefix):]

		if len(search) == 0 {
			// We've exhausted the search key, but the current node is not an exact
			// match or not a leaf. That means that the leaf value if it exists, and
			// all child nodes must be strictly greater, the smallest key in this
			// subtree must be the lower bound.
			findMin(n)
			return
		}

		// Otherwise, take the lower bound next edge.
		idx, exact := n.findLowerBoundEdge(search[0])
		if !exact {
			return
		}

		// Create stack edges for the all strictly higher edges in this node.
		if idx+1 < len(n.edges) {
			i.stack = append(i.stack, n.edges[idx+1:])
		}

		// Recurse
		n = n.edges[idx].node
	}
}

// Next returns the next node in order
func (i *Iterator[K, T]) Next() ([]K, T, bool) {
	var zero T
	// Initialize our stack if needed
	if i.stack == nil && i.node != nil {
		i.stack = []edges[K, T]{{edge[K, T]{node: i.node}}}
	}

	for len(i.stack) > 0 {
		// Inspect the last element of the stack
		n := len(i.stack)
		last := i.stack[n-1]
		elem := last[0].node

		// Update the stack
		if len(last) > 1 {
			i.stack[n-1] = last[1:]
		} else {
			i.stack = i.stack[:n-1]
		}

		// Push the edges onto the frontier
		if len(elem.edges) > 0 {
			i.stack = append(i.stack, elem.edges)
		}

		// Return the leaf values if any
		if elem.leaf != nil {
			return elem.leaf.key, elem.leaf.val, true
		}
	}
	return nil, zero, false
}
