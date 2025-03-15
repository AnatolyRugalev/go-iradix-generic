package iradix

import (
	"slices"
)

// keyT is identical to `constraints.Ordered` from `golang.org/x/exp/constraints`.
type keyT interface {
	~string |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

func keyHasPrefix[K keyT](s, prefix []K) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}

func keyEqual[K keyT](a, b []K) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func keyCompare[K keyT](a, b []K) int {
	return slices.Compare(a, b)
}
