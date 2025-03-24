module github.com/AnatolyRugalev/go-iradix-generic/benchmark

go 1.21

replace github.com/AnatolyRugalev/go-iradix-generic => ../

require (
	github.com/AnatolyRugalev/go-iradix-generic v0.0.0-00010101000000-000000000000
	github.com/hashicorp/go-immutable-radix/v2 v2.1.0
)

require github.com/hashicorp/golang-lru/v2 v2.0.0 // indirect
