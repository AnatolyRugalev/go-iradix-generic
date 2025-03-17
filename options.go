package iradix

const (
	defaultMapCacheCapacity = 16
	defaultChannelLimit     = 2 << 12
)

var defaultOptions = options{
	cacheProvider: MapCache(defaultMapCacheCapacity),
	channelLimit:  defaultChannelLimit,
}

type options struct {
	cacheProvider CacheProvider
	channelLimit  int
}

type Option func(o *options)

func WithCacheProvider(cache CacheProvider) Option {
	return func(o *options) {
		o.cacheProvider = cache
	}
}

func WithChannelLimit(limit int) Option {
	return func(o *options) {
		o.channelLimit = limit
	}
}
