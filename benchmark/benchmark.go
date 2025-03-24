package benchmark

import (
	"bytes"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

type Profile struct {
	// Name is a name of the profile.
	Name string
	// Depth is a size of a generated key.
	// The longer the key, the deeper the tree.
	Depth int
	// Cardinality is the maximum number of keys per edge.
	// In original implementation, only `byte` keys are supported, limiting max cardinality to 256
	Cardinality int
	Tests       []string
	Seed        int64
	// MakeTree creates a tree an initializes a new transaction for benchmarking.
	MakeTree func(keys [][]byte) Txn
}

type Txn interface {
	Get(key []byte) (struct{}, bool)
	Insert(key []byte, v struct{}) (struct{}, bool)
	Delete(key []byte) (struct{}, bool)
}

func randomBytes(rng *rand.Rand, cardinality, n int) []byte {
	gen := make([]byte, n)
	for i := 0; i < n; i++ {
		gen[i] = byte(rng.Intn(cardinality))
	}
	return gen
}

func makeKeys(rng *rand.Rand, cardinality, size, n int) [][]byte {
	keys := make([][]byte, n)
	for i := 0; i < n; i++ {
		keys[i] = randomBytes(rng, cardinality, size)
	}
	return keys
}

func runTest(b *testing.B, profile Profile, name string, fn func(b *testing.B, keys [][]byte)) {
	fullName := b.Name() + "/" + name
	shouldRun := len(profile.Tests) == 0 || slices.ContainsFunc(profile.Tests, func(suffix string) bool {
		return strings.HasSuffix(fullName, suffix)
	})
	if !shouldRun {
		return
	}
	rng := rand.New(rand.NewSource(profile.Seed))
	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		keys := makeKeys(rng, profile.Cardinality, profile.Depth, b.N)
		b.ResetTimer()
		fn(b, keys)
	})
}

func Run(b *testing.B, profile Profile) {
	b.Run(profile.Name, func(b *testing.B) {
		runTest(b, profile, "Get", func(b *testing.B, keys [][]byte) {
			tx := profile.MakeTree(keys)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Get(keys[i])
			}
		})
		runTest(b, profile, "Insert/Random", func(b *testing.B, keys [][]byte) {
			tx := profile.MakeTree(nil)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Insert(keys[i], struct{}{})
			}
		})
		runTest(b, profile, "Insert/Sequential", func(b *testing.B, keys [][]byte) {
			slices.SortFunc(keys, bytes.Compare)
			tx := profile.MakeTree(nil)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Insert(keys[i], struct{}{})
			}
		})
		runTest(b, profile, "Insert/Reverse", func(b *testing.B, keys [][]byte) {
			slices.SortFunc(keys, bytes.Compare)
			slices.Reverse(keys)
			tx := profile.MakeTree(nil)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Insert(keys[i], struct{}{})
			}
		})
		runTest(b, profile, "Update/First", func(b *testing.B, keys [][]byte) {
			tx := profile.MakeTree(keys)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Insert(keys[i], struct{}{})
			}
		})
		runTest(b, profile, "Update/Second", func(b *testing.B, keys [][]byte) {
			tx := profile.MakeTree(keys)
			for i := 0; i < b.N; i++ {
				tx.Insert(keys[i], struct{}{})
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Insert(keys[i], struct{}{})
			}
		})
		runTest(b, profile, "Delete/Random", func(b *testing.B, keys [][]byte) {
			tx := profile.MakeTree(keys)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Delete(keys[i])
			}
		})
		runTest(b, profile, "Delete/Sequential", func(b *testing.B, keys [][]byte) {
			slices.SortFunc(keys, bytes.Compare)
			tx := profile.MakeTree(keys)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Delete(keys[i])
			}
		})
		runTest(b, profile, "Delete/Reverse", func(b *testing.B, keys [][]byte) {
			slices.SortFunc(keys, bytes.Compare)
			slices.Reverse(keys)
			tx := profile.MakeTree(keys)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tx.Delete(keys[0])
			}
		})
	})
}
