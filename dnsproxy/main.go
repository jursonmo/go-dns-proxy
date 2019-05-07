package dnsproxy

import (
	"fmt"

	logs "github.com/jursonmo/beelogs"
)

func Main(path string) {
	conf, err := ParseConfig(path)
	if err != nil {
		fmt.Printf("parse config fail: %v\n", err)
		return
	}

	logs.Init(conf.Log.Path, conf.Log.Level, conf.Log.MaxDay)
	logs.Info("load config: %s", conf.String())

	var cache *Cache = nil
	if conf.Cache != nil && conf.Cache.Enable{
		cache = NewCache(conf.Cache)
	}

	var policy *Policy = nil
	if conf.Policy != nil {
		policy = NewPolicy(conf.Policy)
		policy.Load()
	}

	proxy := NewProxy(conf.Proxy, cache, policy)
	logs.Error("run proxy error: %v", proxy.Run())
}
