// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iradix

import (
	"sort"
)

// WalkFn is used when walking the tree. Takes a
// key and value, returning if iteration should
// be terminated.
type WalkFn[K keyT, T any] func(k []K, v T) bool

// leafNode is used to represent a value
type leafNode[K keyT, T any] struct {
	mutateCh chan struct{}
	key      []K
	val      T
}

// edge is used to represent an edge node
type edge[K keyT, T any] struct {
	label K
	node  *Node[K, T]
}

// Node is an immutable node in the radix tree
type Node[K keyT, T any] struct {
	// mutateCh is closed if this node is modified
	mutateCh chan struct{}

	// leaf is used to store possible leaf
	leaf *leafNode[K, T]

	// prefix is the common prefix we ignore
	prefix []K

	// Edges should be stored in-order for iteration.
	// We avoid a fully materialized slice to save memory,
	// since in most cases we expect to be sparse
	edges edges[K, T]
}

func (n *Node[K, T]) cacheableNode() {}

func (n *Node[K, T]) isLeaf() bool {
	return n.leaf != nil
}

func (n *Node[K, T]) findEdge(label K) (idx int, ok bool) {
	size := len(n.edges)
	idx = sort.Search(size, func(i int) bool {
		return n.edges[i].label >= label
	})
	ok = idx != size && n.edges[idx].label == label
	return
}

func (n *Node[K, T]) findLowerBoundEdge(label K) (int, bool) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	return idx, idx != num
}

func (n *Node[K, T]) addEdge(e edge[K, T]) {
	idx, exact := n.findLowerBoundEdge(e.label)
	n.edges = append(n.edges, e)
	if exact {
		copy(n.edges[idx+1:], n.edges[idx:])
		n.edges[idx] = e
	}
}

func (n *Node[K, T]) replaceEdge(e edge[K, T]) {
	idx, ok := n.findEdge(e.label)
	if !ok {
		panic("replacing missing edge")
	}
	n.edges[idx].node = e.node
}

func (n *Node[K, T]) getEdge(label K) (int, *Node[K, T]) {
	idx, ok := n.findEdge(label)
	if !ok {
		return -1, nil
	}
	return idx, n.edges[idx].node
}

func (n *Node[K, T]) delEdge(label K) {
	idx, ok := n.findEdge(label)
	if !ok {
		return
	}
	copy(n.edges[idx:], n.edges[idx+1:])
	n.edges[len(n.edges)-1] = edge[K, T]{}
	n.edges = n.edges[:len(n.edges)-1]
}

func (n *Node[K, T]) GetWatch(k []K) (<-chan struct{}, T, bool) {
	search := k
	watch := n.mutateCh
	for {
		// Check for key exhaustion
		if len(search) == 0 {
			if n.isLeaf() {
				return n.leaf.mutateCh, n.leaf.val, true
			}
			break
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Update to the finest granularity as the search makes progress
		watch = n.mutateCh

		// Consume the search prefix
		if keyHasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	var zero T
	return watch, zero, false
}

func (n *Node[K, T]) Get(k []K) (T, bool) {
	_, val, ok := n.GetWatch(k)
	return val, ok
}

// LongestPrefix is like Get, but instead of an
// exact match, it will return the longest prefix match.
func (n *Node[K, T]) LongestPrefix(k []K) ([]K, T, bool) {
	var last *leafNode[K, T]
	search := k
	for {
		// Look for a leaf node
		if n.isLeaf() {
			last = n.leaf
		}

		// Check for key exhaustion
		if len(search) == 0 {
			break
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Consume the search prefix
		if keyHasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	if last != nil {
		return last.key, last.val, true
	}
	var zero T
	return nil, zero, false
}

// Minimum is used to return the minimum value in the tree
func (n *Node[K, T]) Minimum() ([]K, T, bool) {
	for {
		if n.isLeaf() {
			return n.leaf.key, n.leaf.val, true
		}
		if len(n.edges) > 0 {
			n = n.edges[0].node
		} else {
			break
		}
	}
	var zero T
	return nil, zero, false
}

// Maximum is used to return the maximum value in the tree
func (n *Node[K, T]) Maximum() ([]K, T, bool) {
	for {
		if num := len(n.edges); num > 0 {
			n = n.edges[num-1].node // bug?
			continue
		}
		if n.isLeaf() {
			return n.leaf.key, n.leaf.val, true
		}
		break
	}
	var zero T
	return nil, zero, false
}

// Iterator is used to return an iterator at
// the given node to walk the tree
func (n *Node[K, T]) Iterator() *Iterator[K, T] {
	return &Iterator[K, T]{node: n}
}

// ReverseIterator is used to return an iterator at
// the given node to walk the tree backwards
func (n *Node[K, T]) ReverseIterator() *ReverseIterator[K, T] {
	return NewReverseIterator(n)
}

// Iterator is used to return an iterator at
// the given node to walk the tree
func (n *Node[K, T]) PathIterator(path []K) *PathIterator[K, T] {
	return &PathIterator[K, T]{node: n, path: path}
}

// rawIterator is used to return a raw iterator at the given node to walk the
// tree.
func (n *Node[K, T]) rawIterator() *rawIterator[K, T] {
	iter := &rawIterator[K, T]{node: n}
	iter.Next()
	return iter
}

// Walk is used to walk the tree
func (n *Node[K, T]) Walk(fn WalkFn[K, T]) {
	recursiveWalk(n, fn)
}

// WalkBackwards is used to walk the tree in reverse order
func (n *Node[K, T]) WalkBackwards(fn WalkFn[K, T]) {
	reverseRecursiveWalk(n, fn)
}

// WalkPrefix is used to walk the tree under a prefix
func (n *Node[K, T]) WalkPrefix(prefix []K, fn WalkFn[K, T]) {
	search := prefix
loop:
	for {
		// Check for key exhaustion
		if len(search) == 0 {
			recursiveWalk(n, fn)
			return
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			break
		}

		switch {
		case keyHasPrefix(search, n.prefix):
			search = search[len(n.prefix):]
		case keyHasPrefix(n.prefix, search):
			recursiveWalk(n, fn)
			return
		default:
			break loop
		}
	}
}

// WalkPath is used to walk the tree, but only visiting nodes
// from the root down to a given leaf. Where WalkPrefix walks
// all the entries *under* the given prefix, this walks the
// entries *above* the given prefix.
func (n *Node[K, T]) WalkPath(path []K, fn WalkFn[K, T]) {
	i := n.PathIterator(path)

	for path, val, ok := i.Next(); ok; path, val, ok = i.Next() {
		if fn(path, val) {
			return
		}
	}
}

// recursiveWalk is used to do a pre-order walk of a node
// recursively. Returns true if the walk should be aborted
func recursiveWalk[K keyT, T any](n *Node[K, T], fn WalkFn[K, T]) bool {
	// Visit the leaf values if any
	if n.leaf != nil && fn(n.leaf.key, n.leaf.val) {
		return true
	}

	// Recurse on the children
	for _, e := range n.edges {
		if recursiveWalk(e.node, fn) {
			return true
		}
	}
	return false
}

// reverseRecursiveWalk is used to do a reverse pre-order
// walk of a node recursively. Returns true if the walk
// should be aborted
func reverseRecursiveWalk[K keyT, T any](n *Node[K, T], fn WalkFn[K, T]) bool {
	// Visit the leaf values if any
	if n.leaf != nil && fn(n.leaf.key, n.leaf.val) {
		return true
	}

	// Recurse on the children in reverse order
	for i := len(n.edges) - 1; i >= 0; i-- {
		e := n.edges[i]
		if reverseRecursiveWalk(e.node, fn) {
			return true
		}
	}
	return false
}
