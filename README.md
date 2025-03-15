# go-iradix-generic

This is a hard fork of Hashicorp's [go-immutable-radix](https://github.com/hashicorp/go-immutable-radix) package.

The only difference from the original is a swap of `[]byte` keys to `[]constraints.Ordered` generic interface.

## Motivation

For the purpose of one of my projects, using `[]byte` for keys is unnecessary and somewhat wasteful. Hashicorp's
package is a perfect fit for my needs, but `[]byte` was getting in a way, so I decided to fork it.

## State of the Fork

This project is provided AS IS, keeping the original Hashicorp's license for obvious reasons. I will probably abandon
it soon, so don't keep your hopes high. I will accept PRs, especially the ones that keep this fork in sync with the original.

## Performance Impact

Who knows. While `bytes.Equal` and `bytes.Compare` might be faster with pure `[]byte`, using basic slice comparison
shouldn't have a significant performance impact. But I would be happy to learn something if I'm wrong.

I don't have any benchmarks because I just don't really care that much yet. I will be happy to see apples-to-apples
if you run some benchmarks with it.

## How to Use

This is a drop-in replacement. The only thing you'll need to change is how you create a new tree:

```go
tree := iradix.New[byte, int]() // equivalent to `iradix.New[int]()
```

## Documentation

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

