package dnsproxy

import (
	"time"

	logs "github.com/jursonmo/beelogs"
)

var (
	defaultCap      = 10000
	defaultTTL      = 60 * 5
	defaultInterval = 10
)

type CacheConfig struct {
	Cap      int `toml:"cap"`
	TTL      int `toml:"ttl"`
	Interval int `toml:"interval"`
}

type cacheValue struct {
	val interface{}
	exp time.Time
}

type Cache struct {
	table    *LRUCache
	done     chan struct{}
	ttl      time.Duration
	interval time.Duration
}

func (cv *cacheValue) Size() int {
	return 1
}

func NewCache(cfg *CacheConfig) *Cache {
	cap := cfg.Cap
	if cap <= 0 {
		cap = defaultCap
	}

	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}

	intval := cfg.Interval
	if intval <= 0 {
		intval = defaultInterval
	}

	cache := &Cache{
		ttl:      time.Second * time.Duration(ttl),
		interval: time.Second * time.Duration(intval),
		table:    NewLRUCache(int64(cap)),
		done:     make(chan struct{}),
	}

	go cache.gc()
	return cache
}

func (c *Cache) Close() {
	close(c.done)
}

func (c *Cache) Get(key string) interface{} {
	val, ok := c.table.Get(key)
	if ok {
		cv := val.(*cacheValue)
		if cv.exp.Before(time.Now()) {
			c.table.Delete(key)
			return nil
		}

		return cv.val
	}
	return nil
}

func (c *Cache) Set(key string, val interface{}) {
	cv := &cacheValue{
		val: val,
		exp: time.Now().Add(c.ttl),
	}

	c.table.Set(key, cv)
}

func (c *Cache) gc() {
	for {
		select {
		case <-c.done:
			return

		case <-time.After(c.interval):
			logs.Info("cache gcing")
			deleted := 0

			items := c.table.Items()
			for _, it := range items {
				if it.Value.(*cacheValue).exp.Before(time.Now()) {
					c.table.Delete(it.Key)
					deleted += 1
				}
			}
			logs.Info("cache gc finished, delete %d elements, total size: %d", deleted, c.table.Size())
		}
	}
}
