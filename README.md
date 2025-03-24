# go-iradix-generic

[![Run Tests](https://github.com/AnatolyRugalev/go-iradix-generic/actions/workflows/mian.yaml/badge.svg)](https://github.com/AnatolyRugalev/go-iradix-generic/actions/workflows/mian.yaml)

This is a hard fork of Hashicorp's [go-immutable-radix](https://github.com/hashicorp/go-immutable-radix) package.

The only difference from the original is a swap of `[]byte` keys to `[]constraints.Ordered` generic interface.

## Motivation

For the purpose of one of my projects, using `[]byte` for keys is unnecessary and somewhat wasteful. Hashicorp's
package is a perfect fit for my needs, but `[]byte` was getting in a way, so I decided to fork it.

## State of the Fork

This project is provided AS IS, keeping the original Hashicorp's license for obvious reasons. I will probably abandon
it soon, so don't keep your hopes high. I will accept PRs, especially the ones that keep this fork in sync with the original.

## Performance Impact

Practically, there's zero performance impact. Hashicorp's implementation uses LRU cache to keep write cache, however, according
to [unsophisticated benchmarks](benchmark/benchmark_test.go), LRU caching allocates more than a simple hashmap. For this reason,
LRU is not a part of this package, however you can still plug it in. See [benchmark/benchmark_test.go](benchmark/benchmark_test.go) for an example.

### Benchmark Results

The Generic implementation demonstrates improved performance and efficiency over Hashicorp's original version in most use cases.
By removing LRU caching, which added overhead, it achieves faster execution and fewer allocations, particularly in `Update`,
`Insert`, and `Delete` operations. Overall, it's a lightweight and effective alternative for scenarios where `[]byte` keys are unnecessary.

> :point_up: AI slop

#### ns/op

| Benchmark           | Generic (ns/op) | Generic-LRU (ns/op) | Hashicorp (ns/op) |
|---------------------|-----------------|---------------------|-------------------|
| Get                 | 508.6           | 528.9               | 544.1             |
| Insert (Random)     | 1026            | 1848                | 1729              |
| Insert (Sequential) | 670.3           | 689.2               | 653.6             |
| Insert (Reverse)    | 716.8           | 754.4               | 665.1             |
| Update (First)      | 1623            | 2378                | 2578              |
| Update (Second)     | 947.9           | 2583                | 2325              |
| Delete (Random)     | 1228            | 1956                | 1768              |
| Delete (Sequential) | 857.6           | 832.6               | 733.9             |
| Delete (Reverse)    | 74.54           | 73.96               | 76.03             |

#### allocs/op

| Benchmark           | Generic (allocs/op) | Generic-LRU (allocs/op) | Hashicorp (allocs/op) |
|---------------------|---------------------|-------------------------|-----------------------|
| Get                 | 0                   | 0                       | 0                     |
| Insert (Random)     | 5                   | 9                       | 9                     |
| Insert (Sequential) | 5                   | 5                       | 5                     |
| Insert (Reverse)    | 5                   | 5                       | 5                     |
| Update (First)      | 5                   | 11                      | 11                    |
| Update (Second)     | 2                   | 11                      | 11                    |
| Delete (Random)     | 3                   | 8                       | 8                     |
| Delete (Sequential) | 3                   | 4                       | 4                     |
| Delete (Reverse)    | 0                   | 0                       | 0                     |

## How to Use

This is a drop-in replacement. The only thing you'll need to change is how you create a new tree:

```go
tree := iradix.New[byte, int]() // equivalent to `iradix.New[int]()
```

## Original Documentation

The full documentation is available on [Godoc](http://godoc.org/github.com/AnatolyRugalev/go-iradix-generic).

## Example

Below is a simple example of usage

```go
// Create a tree
r := iradix.New[byte, int]()
r, _, _ = r.Insert([]byte("foo"), 1)
r, _, _ = r.Insert([]byte("bar"), 2)
r, _, _ = r.Insert([]byte("foobar"), 2)

// Find the longest prefix match
m, _, _ := r.Root().LongestPrefix([]byte("foozip"))
if string(m) != "foo" {
    panic("should be foo")
}
```

Here is an example of performing a range scan of the keys.

```go
// Create a tree
r := iradix.New[byte, int]()
r, _, _ = r.Insert([]byte("001"), 1)
r, _, _ = r.Insert([]byte("002"), 2)
r, _, _ = r.Insert([]byte("005"), 5)
r, _, _ = r.Insert([]byte("010"), 10)
r, _, _ = r.Insert([]byte("100"), 10)

// Range scan over the keys that sort lexicographically between [003, 050)
it := r.Root().Iterator()
it.SeekLowerBound([]byte("003"))
for key, _, ok := it.Next(); ok; key, _, ok = it.Next() {
  if string(key) >= "050" {
      break
  }
  fmt.Println(string(key))
}
// Output:
//  005
//  010
```

