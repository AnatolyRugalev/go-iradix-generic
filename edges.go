// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package iradix

type edges[K keyT, T any] []edge[K, T]

func (e edges[K, T]) Len() int {
	return len(e)
}

func (e edges[K, T]) Less(i, j int) bool {
	return e[i].label < e[j].label
}

func (e edges[K, T]) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

type EdgeIndexer[K keyT] interface {
}
