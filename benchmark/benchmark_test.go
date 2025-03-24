package benchmark

import (
	"testing"
	"time"

	"github.com/AnatolyRugalev/go-iradix-generic"
	hashicorp "github.com/hashicorp/go-immutable-radix/v2"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

var profiles = []Profile{
	{
		Name:        "hashicorp",
		Depth:       16,
		Cardinality: 256,
		Seed:        0,
		MakeTree:    NewHashicorpRadix,
	},
	{
		Name:        "generic-lru",
		Depth:       16,
		Cardinality: 256,
		Seed:        0,
		MakeTree:    NewGenericRadixWithLRU,
	},
	{
		Name:        "generic",
		Depth:       16,
		Cardinality: 256,
		Seed:        0,
		MakeTree:    NewGenericRadix,
	},
}

func NewHashicorpRadix(keys [][]byte) Txn {
	tree := hashicorp.New[struct{}]()
	txn := tree.Txn()
	for _, key := range keys {
		txn.Insert(key, struct{}{})
	}
	tree = txn.Commit()
	return tree.Txn()
}

func NewGenericRadix(keys [][]byte) Txn {
	tree := iradix.New[byte, struct{}](
		iradix.WithCacheProvider(iradix.MapCache(0)),
	)
	txn := tree.Txn()
	for _, key := range keys {
		txn.Insert(key, struct{}{})
	}
	tree = txn.Commit()
	return tree.Txn()
}

func NewGenericRadixWithLRU(keys [][]byte) Txn {
	tree := iradix.New[byte, struct{}](
		iradix.WithCacheProvider(NewLRU),
	)
	txn := tree.Txn()
	for _, key := range keys {
		txn.Insert(key, struct{}{})
	}
	tree = txn.Commit()
	return tree.Txn()
}

func NewLRU() iradix.Cache {
	lru, err := simplelru.NewLRU[iradix.CacheableNode, struct{}](8192, nil)
	if err != nil {
		panic(err)
	}
	return &lruCache{lru: lru}
}

type lruCache struct {
	lru *simplelru.LRU[iradix.CacheableNode, struct{}]
}

func (l *lruCache) Set(n iradix.CacheableNode) {
	l.lru.Add(n, struct{}{})
}

func (l *lruCache) Has(n iradix.CacheableNode) bool {
	_, ok := l.lru.Get(n)
	return ok
}

func (l *lruCache) Clear() {
	l.lru.Purge()
}

func BenchmarkRadix(b *testing.B) {
	seed := time.Now().UnixNano()
	b.Logf("seed: %d", seed)
	for _, profile := range profiles {
		profile.Seed = seed
		Run(b, profile)
	}
}
