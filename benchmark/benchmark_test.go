package benchmark

import (
	"github.com/AnatolyRugalev/go-iradix-generic"
	hashicorp "github.com/hashicorp/go-immutable-radix/v2"
	"testing"
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
	tree := iradix.New[byte, struct{}]()
	txn := tree.Txn()
	for _, key := range keys {
		txn.Insert(key, struct{}{})
	}
	tree = txn.Commit()
	return tree.Txn()
}

func BenchmarkRadix(b *testing.B) {
	for _, profile := range profiles {
		b.Run(profile.Name, func(b *testing.B) {
			Run(b, profile)
		})
	}
}
