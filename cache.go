package iradix

// CacheProvider can be used as an option to cache nodes during transaction execution.
type CacheProvider func() Cache

type CacheableNode interface {
	cacheableNode()
}

type Cache interface {
	Set(ptr CacheableNode)
	Has(ptr CacheableNode) bool
	Clear()
}

// NoCache disables node caching.
func NoCache() Cache {
	return &noCache{}
}

type noCache struct{}

func (*noCache) Set(_ CacheableNode) {}
func (*noCache) Has(_ CacheableNode) bool {
	return false
}
func (*noCache) Clear() {}

func MapCache(initCapacity int) CacheProvider {
	return func() Cache {
		return make(mapCache, initCapacity)
	}
}

type mapCache map[CacheableNode]struct{}

func (m mapCache) Set(ptr CacheableNode) {
	m[ptr] = struct{}{}
}

func (m mapCache) Has(ptr CacheableNode) bool {
	_, ok := m[ptr]
	return ok
}

func (m mapCache) Clear() {
	clear(m)
}
