// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iradix

import (
	"slices"
)

// Tree implements an immutable radix tree. This can be treated as a
// Dictionary abstract data type. The main advantage over a standard
// hash map is prefix-based lookups and ordered iteration. The immutability
// means that it is safe to concurrently read from a Tree without any
// coordination.
type Tree[K keyT, T any] struct {
	options
	root *Node[K, T]
	size int
}

// New returns an empty Tree
func New[K keyT, T any](opts ...Option) *Tree[K, T] {
	o := defaultOptions
	for _, opt := range opts {
		opt(&o)
	}

	t := &Tree[K, T]{

		options: o,
		root: &Node[K, T]{
			mutateCh: make(chan struct{}),
		},
	}
	return t
}

// Len is used to return the number of elements in the tree
func (t *Tree[K, T]) Len() int {
	return t.size
}

// Txn is a transaction on the tree. This transaction is applied
// atomically and returns a new tree when committed. A transaction
// is not thread safe, and should only be used by a single goroutine.
type Txn[K keyT, T any] struct {
	options
	// root is the modified root for the transaction.
	root *Node[K, T]

	// snap is a snapshot of the root node for use if we have to run the
	// slow notify algorithm.
	snap *Node[K, T]

	// size tracks the size of the tree as it is modified during the
	// transaction.
	size int

	// writable is a cache of writable nodes that have been created during
	// the course of the transaction. This allows us to re-use the same
	// nodes for further writes and avoid unnecessary copies of nodes that
	// have never been exposed outside the transaction. This will only hold
	// up to defaultChannelLimit number of entries.
	writable Cache

	// trackChannels is used to hold channels that need to be notified to
	// signal mutation of the tree. This will only hold up to
	// defaultChannelLimit number of entries, after which we will set the
	// trackOverflow flag, which will cause us to use a more expensive
	// algorithm to perform the notifications. Mutation tracking is only
	// performed if trackMutate is true.
	trackChannels map[chan struct{}]struct{}
	trackOverflow bool
	trackMutate   bool
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *Tree[K, T]) Txn() *Txn[K, T] {
	txn := &Txn[K, T]{
		options: t.options,
		root:    t.root,
		snap:    t.root,
		size:    t.size,
	}
	return txn
}

// Clone makes an independent copy of the transaction. The new transaction
// does not track any nodes and has TrackMutate turned off. The cloned transaction will contain any uncommitted writes in the original transaction but further mutations to either will be independent and result in different radix trees on Commit. A cloned transaction may be passed to another goroutine and mutated there independently however each transaction may only be mutated in a single thread.
func (t *Txn[K, T]) Clone() *Txn[K, T] {
	// reset the writable node cache to avoid leaking future writes into the clone
	if t.writable != nil {
		t.writable.Clear()
		t.writable = nil
	}

	txn := &Txn[K, T]{
		options: t.options,
		root:    t.root,
		snap:    t.snap,
		size:    t.size,
	}
	return txn
}

// TrackMutate can be used to toggle if mutations are tracked. If this is enabled
// then notifications will be issued for affected internal nodes and leaves when
// the transaction is committed.
func (t *Txn[K, T]) TrackMutate(track bool) {
	t.trackMutate = track
}

// trackChannel safely attempts to track the given mutation channel, setting the
// overflow flag if we can no longer track any more. This limits the amount of
// state that will accumulate during a transaction and we have a slower algorithm
// to switch to if we overflow.
func (t *Txn[K, T]) trackChannel(ch chan struct{}) {
	// In overflow, make sure we don't store any more objects.
	if t.trackOverflow {
		return
	}

	// If this would overflow the state we reject it and set the flag (since
	// we aren't tracking everything that's required any longer).
	if len(t.trackChannels) >= t.channelLimit {
		// Mark that we are in the overflow state
		t.trackOverflow = true

		// Clear the map so that the channels can be garbage collected. It is
		// safe to do this since we have already overflowed and will be using
		// the slow notify algorithm.
		t.trackChannels = nil
		return
	}

	// Create the map on the fly when we need it.
	if t.trackChannels == nil {
		t.trackChannels = make(map[chan struct{}]struct{})
	}

	// Otherwise we are good to track it.
	t.trackChannels[ch] = struct{}{}
}

// writeNode returns a node to be modified, if the current node has already been
// modified during the course of the transaction, it is used in-place. Set
// forLeafUpdate to true if you are getting a write node to update the leaf,
// which will set leaf mutation tracking appropriately as well.
func (t *Txn[K, T]) writeNode(n *Node[K, T], forLeafUpdate bool) *Node[K, T] {
	// Ensure the writable set exists.
	if t.writable == nil {
		t.writable = t.cacheProvider()
	}

	// If this node has already been modified, we can continue to use it
	// during this transaction. We know that we don't need to track it for
	// a node update since the node is writable, but if this is for a leaf
	// update we track it, in case the initial write to this node didn't
	// update the leaf.
	if t.writable.Has(n) {
		if t.trackMutate && forLeafUpdate && n.leaf != nil {
			t.trackChannel(n.leaf.mutateCh)
		}
		return n
	}

	if t.trackMutate {
		t.trackChannel(n.mutateCh)
		if forLeafUpdate && n.leaf != nil {
			t.trackChannel(n.leaf.mutateCh)
		}
	}

	// Copy the existing node. If you have set forLeafUpdate it will be
	// safe to replace this leaf with another after you get your node for
	// writing. You MUST replace it, because the channel associated with
	// this leaf will be closed when this transaction is committed.
	nc := &Node[K, T]{
		mutateCh: make(chan struct{}),
		leaf:     n.leaf,
		prefix:   slices.Clone(n.prefix),
		edges:    slices.Clone(n.edges),
	}

	// Mark this node as writable.
	t.writable.Set(nc)
	return nc
}

// Visit all the nodes in the tree under n, and add their mutateChannels to the transaction
// Returns the size of the subtree visited
func (t *Txn[K, T]) trackChannelsAndCount(n *Node[K, T]) int {
	// Count only leaf nodes
	leaves := 0
	if n.leaf != nil {
		leaves = 1
	}
	// Mark this node as being mutated.
	if t.trackMutate {
		t.trackChannel(n.mutateCh)
	}

	// Mark its leaf as being mutated, if appropriate.
	if t.trackMutate && n.leaf != nil {
		t.trackChannel(n.leaf.mutateCh)
	}

	// Recurse on the children
	for _, e := range n.edges {
		leaves += t.trackChannelsAndCount(e.node)
	}
	return leaves
}

// mergeChild is called to collapse the given node with its child. This is only
// called when the given node is not a leaf and has a single edge.
func (t *Txn[K, T]) mergeChild(n *Node[K, T]) {
	// Mark the child node as being mutated since we are about to abandon
	// it. We don't need to mark the leaf since we are retaining it if it
	// is there.
	e := n.edges[0]
	child := e.node
	if t.trackMutate {
		t.trackChannel(child.mutateCh)
	}

	// Merge the nodes.
	n.prefix = append(n.prefix, child.prefix...)
	n.leaf = child.leaf
	n.edges = slices.Clone(child.edges)
}

// insert does a recursive insertion
func (t *Txn[K, T]) insert(n *Node[K, T], k, search []K, v T) (*Node[K, T], T, bool) {
	var zero T

	// Handle key exhaustion
	if len(search) == 0 {
		var oldVal T
		didUpdate := false
		if n.isLeaf() {
			oldVal = n.leaf.val
			didUpdate = true
		}

		nc := t.writeNode(n, true)
		nc.leaf = &leafNode[K, T]{
			mutateCh: make(chan struct{}),
			key:      k,
			val:      v,
		}
		return nc, oldVal, didUpdate
	}

	// Look for the edge
	idx, child := n.getEdge(search[0])

	// No edge, create one
	if child == nil {
		e := edge[K, T]{
			label: search[0],
			node: &Node[K, T]{
				mutateCh: make(chan struct{}),
				leaf: &leafNode[K, T]{
					mutateCh: make(chan struct{}),
					key:      k,
					val:      v,
				},
				prefix: search,
			},
		}
		nc := t.writeNode(n, false)
		nc.addEdge(e)
		return nc, zero, false
	}

	// Determine longest prefix of the search key on match
	commonPrefix := longestPrefix(search, child.prefix)
	if commonPrefix == len(child.prefix) {
		search = search[commonPrefix:]
		newChild, oldVal, didUpdate := t.insert(child, k, search, v)
		if newChild != nil {
			nc := t.writeNode(n, false)
			nc.edges[idx].node = newChild
			return nc, oldVal, didUpdate
		}
		return nil, oldVal, didUpdate
	}

	// Split the node
	nc := t.writeNode(n, false)
	splitNode := &Node[K, T]{
		mutateCh: make(chan struct{}),
		prefix:   search[:commonPrefix],
	}
	nc.replaceEdge(edge[K, T]{
		label: search[0],
		node:  splitNode,
	})

	// Restore the existing child node
	modChild := t.writeNode(child, false)
	splitNode.addEdge(edge[K, T]{
		label: modChild.prefix[commonPrefix],
		node:  modChild,
	})
	modChild.prefix = modChild.prefix[commonPrefix:]

	// Create a new leaf node
	leaf := &leafNode[K, T]{
		mutateCh: make(chan struct{}),
		key:      k,
		val:      v,
	}

	// If the new key is a subset, add to to this node
	search = search[commonPrefix:]
	if len(search) == 0 {
		splitNode.leaf = leaf
		return nc, zero, false
	}

	// Create a new edge for the node
	splitNode.addEdge(edge[K, T]{
		label: search[0],
		node: &Node[K, T]{
			mutateCh: make(chan struct{}),
			leaf:     leaf,
			prefix:   search,
		},
	})
	return nc, zero, false
}

// delete does a recursive deletion
func (t *Txn[K, T]) delete(n *Node[K, T], search []K) (*Node[K, T], *leafNode[K, T]) {
	// Check for key exhaustion
	if len(search) == 0 {
		if !n.isLeaf() {
			return nil, nil
		}
		// Copy the pointer in case we are in a transaction that already
		// modified this node since the node will be reused. Any changes
		// made to the node will not affect returning the original leaf
		// value.
		oldLeaf := n.leaf

		// Remove the leaf node
		nc := t.writeNode(n, true)
		nc.leaf = nil

		// Check if this node should be merged
		if n != t.root && len(nc.edges) == 1 {
			t.mergeChild(nc)
		}
		return nc, oldLeaf
	}

	// Look for an edge
	label := search[0]
	idx, child := n.getEdge(label)
	if child == nil || !keyHasPrefix(search, child.prefix) {
		return nil, nil
	}

	// Consume the search prefix
	search = search[len(child.prefix):]
	newChild, leaf := t.delete(child, search)
	if newChild == nil {
		return nil, nil
	}

	// Copy this node. WATCH OUT - it's safe to pass "false" here because we
	// will only ADD a leaf via nc.mergeChild() if there isn't one due to
	// the !nc.isLeaf() check in the logic just below. This is pretty subtle,
	// so be careful if you change any of the logic here.
	nc := t.writeNode(n, false)

	// Delete the edge if the node has no edges
	if newChild.leaf == nil && len(newChild.edges) == 0 {
		nc.delEdge(label)
		if n != t.root && len(nc.edges) == 1 && !nc.isLeaf() {
			t.mergeChild(nc)
		}
	} else {
		nc.edges[idx].node = newChild
	}
	return nc, leaf
}

// delete does a recursive deletion
func (t *Txn[K, T]) deletePrefix(n *Node[K, T], search []K) (*Node[K, T], int) {
	// Check for key exhaustion
	if len(search) == 0 {
		nc := t.writeNode(n, true)
		if n.isLeaf() {
			nc.leaf = nil
		}
		nc.edges = nil
		return nc, t.trackChannelsAndCount(n)
	}

	// Look for an edge
	label := search[0]
	idx, child := n.getEdge(label)
	// We make sure that either the child node's prefix starts with the search term, or the search term starts with the child node's prefix
	// Need to do both so that we can delete prefixes that don't correspond to any node in the tree
	if child == nil || !keyHasPrefix(child.prefix, search) && !keyHasPrefix(search, child.prefix) {
		return nil, 0
	}

	// Consume the search prefix
	if len(child.prefix) > len(search) {
		search = search[:0]
	} else {
		search = search[len(child.prefix):]
	}
	newChild, numDeletions := t.deletePrefix(child, search)
	if newChild == nil {
		return nil, 0
	}
	// Copy this node. WATCH OUT - it's safe to pass "false" here because we
	// will only ADD a leaf via nc.mergeChild() if there isn't one due to
	// the !nc.isLeaf() check in the logic just below. This is pretty subtle,
	// so be careful if you change any of the logic here.

	nc := t.writeNode(n, false)

	// Delete the edge if the node has no edges
	if newChild.leaf == nil && len(newChild.edges) == 0 {
		nc.delEdge(label)
		if n != t.root && len(nc.edges) == 1 && !nc.isLeaf() {
			t.mergeChild(nc)
		}
	} else {
		nc.edges[idx].node = newChild
	}
	return nc, numDeletions
}

// Insert is used to add or update a given key. The return provides
// the previous value and a bool indicating if any was set.
func (t *Txn[K, T]) Insert(k []K, v T) (T, bool) {
	newRoot, oldVal, didUpdate := t.insert(t.root, k, k, v)
	if newRoot != nil {
		t.root = newRoot
	}
	if !didUpdate {
		t.size++
	}
	return oldVal, didUpdate
}

// Delete is used to delete a given key. Returns the old value if any,
// and a bool indicating if the key was set.
func (t *Txn[K, T]) Delete(k []K) (T, bool) {
	var zero T
	newRoot, leaf := t.delete(t.root, k)
	if newRoot != nil {
		t.root = newRoot
	}
	if leaf != nil {
		t.size--
		return leaf.val, true
	}
	return zero, false
}

// DeletePrefix is used to delete an entire subtree that matches the prefix
// This will delete all nodes under that prefix
func (t *Txn[K, T]) DeletePrefix(prefix []K) bool {
	newRoot, numDeletions := t.deletePrefix(t.root, prefix)
	if newRoot != nil {
		t.root = newRoot
		t.size -= numDeletions
		return true
	}
	return false

}

// Root returns the current root of the radix tree within this
// transaction. The root is not safe across insert and delete operations,
// but can be used to read the current state during a transaction.
func (t *Txn[K, T]) Root() *Node[K, T] {
	return t.root
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Txn[K, T]) Get(k []K) (T, bool) {
	return t.root.Get(k)
}

// GetWatch is used to lookup a specific key, returning
// the watch channel, value and if it was found
func (t *Txn[K, T]) GetWatch(k []K) (<-chan struct{}, T, bool) {
	return t.root.GetWatch(k)
}

// Commit is used to finalize the transaction and return a new tree. If mutation
// tracking is turned on then notifications will also be issued.
func (t *Txn[K, T]) Commit() *Tree[K, T] {
	nt := t.CommitOnly()
	if t.trackMutate {
		t.Notify()
	}
	return nt
}

// CommitOnly is used to finalize the transaction and return a new tree, but
// does not issue any notifications until Notify is called.
func (t *Txn[K, T]) CommitOnly() *Tree[K, T] {
	nt := &Tree[K, T]{
		options: t.options,
		root:    t.root,
		size:    t.size,
	}
	if t.writable != nil {
		t.writable.Clear()
		t.writable = nil
	}
	return nt
}

// slowNotify does a complete comparison of the before and after trees in order
// to trigger notifications. This doesn't require any additional state but it
// is very expensive to compute.
func (t *Txn[K, T]) slowNotify() {
	snapIter := t.snap.rawIterator()
	rootIter := t.root.rawIterator()
	for snapIter.Front() != nil || rootIter.Front() != nil {
		// If we've exhausted the nodes in the old snapshot, we know
		// there's nothing remaining to notify.
		if snapIter.Front() == nil {
			return
		}
		snapElem := snapIter.Front()

		// If we've exhausted the nodes in the new root, we know we need
		// to invalidate everything that remains in the old snapshot. We
		// know from the loop condition there's something in the old
		// snapshot.
		if rootIter.Front() == nil {
			close(snapElem.mutateCh)
			if snapElem.isLeaf() {
				close(snapElem.leaf.mutateCh)
			}
			snapIter.Next()
			continue
		}

		// Do one string compare so we can check the various conditions
		// below without repeating the compare.
		cmp := keyCompare(snapIter.Path(), rootIter.Path())

		// If the snapshot is behind the root, then we must have deleted
		// this node during the transaction.
		if cmp < 0 {
			close(snapElem.mutateCh)
			if snapElem.isLeaf() {
				close(snapElem.leaf.mutateCh)
			}
			snapIter.Next()
			continue
		}

		// If the snapshot is ahead of the root, then we must have added
		// this node during the transaction.
		if cmp > 0 {
			rootIter.Next()
			continue
		}

		// If we have the same path, then we need to see if we mutated a
		// node and possibly the leaf.
		rootElem := rootIter.Front()
		if snapElem != rootElem {
			close(snapElem.mutateCh)
			if snapElem.leaf != nil && (snapElem.leaf != rootElem.leaf) {
				close(snapElem.leaf.mutateCh)
			}
		}
		snapIter.Next()
		rootIter.Next()
	}
}

// Notify is used along with TrackMutate to trigger notifications. This must
// only be done once a transaction is committed via CommitOnly, and it is called
// automatically by Commit.
func (t *Txn[K, T]) Notify() {
	if !t.trackMutate {
		return
	}

	// If we've overflowed the tracking state we can't use it in any way and
	// need to do a full tree compare.
	if t.trackOverflow {
		t.slowNotify()
	} else {
		for ch := range t.trackChannels {
			close(ch)
		}
	}

	// Clean up the tracking state so that a re-notify is safe (will trigger
	// the else clause above which will be a no-op).
	t.trackChannels = nil
	t.trackOverflow = false
}

// Insert is used to add or update a given key. The return provides
// the new tree, previous value and a bool indicating if any was set.
func (t *Tree[K, T]) Insert(k []K, v T) (*Tree[K, T], T, bool) {
	txn := t.Txn()
	old, ok := txn.Insert(k, v)
	return txn.Commit(), old, ok
}

// Delete is used to delete a given key. Returns the new tree,
// old value if any, and a bool indicating if the key was set.
func (t *Tree[K, T]) Delete(k []K) (*Tree[K, T], T, bool) {
	txn := t.Txn()
	old, ok := txn.Delete(k)
	return txn.Commit(), old, ok
}

// DeletePrefix is used to delete all nodes starting with a given prefix. Returns the new tree,
// and a bool indicating if the prefix matched any nodes
func (t *Tree[K, T]) DeletePrefix(k []K) (*Tree[K, T], bool) {
	txn := t.Txn()
	ok := txn.DeletePrefix(k)
	return txn.Commit(), ok
}

// Root returns the root node of the tree which can be used for richer
// query operations.
func (t *Tree[K, T]) Root() *Node[K, T] {
	return t.root
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Tree[K, T]) Get(k []K) (T, bool) {
	return t.root.Get(k)
}

// longestPrefix finds the length of the shared prefix
// of two strings
func longestPrefix[K keyT](k1, k2 []K) int {
	size1 := len(k1)
	if l := len(k2); l < size1 {
		size1 = l
	}
	var i int
	for i = 0; i < size1; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}
