package iradix

// CacheProvider can be used as an option to cache nodes during transaction execution.
type CacheProvider func() Cache

// Cache implements basic Set and Has methods.
type Cache interface {
	Set(ptr uintptr)
	Has(ptr uintptr) bool
	Clear()
}

// NoCache disables node caching.
func NoCache() Cache {
	return noCache{}
}

type noCache struct{}

func (noCache) Set(_ uintptr) {}
func (noCache) Has(_ uintptr) bool {
	return false
}
func (noCache) Clear() {}

// MapCache uses basic Go map to keep node cache during txn execution.
func MapCache(initCapacity int) CacheProvider {
	return func() Cache {
		return make(mapCache, initCapacity)
	}
}

type mapCache map[uintptr]struct{}

func (m mapCache) Set(ptr uintptr) {
	m[ptr] = struct{}{}
}

func (m mapCache) Has(ptr uintptr) bool {
	_, ok := m[ptr]
	return ok
}

func (m mapCache) Clear() {
	clear(m)
}
