/**
 * @Author:      leafney
 * @GitHub:      https://github.com/leafney
 * @Project:     grape
 * @Date:        2024-10-20 15:07
 * @Description:
 */

package cachex

import (
	"github.com/leafney/filedock/pkg/configx"
	"github.com/leafney/filedock/pkg/xlogx"
	rcache "github.com/leafney/rose-cache"
)

type CacheSvc struct {
	*rcache.Cache
}

func NewCacheSvc(cfg configx.CacheConfig, log *xlogx.XLogSvc, stop chan struct{}) *CacheSvc {
	cfgCacheMinutes := cfg.GetCacheMinutes()

	cc, err := rcache.NewCache(cfgCacheMinutes)
	if err != nil {
		log.Fatalf("[Cache] NewCache error [%v]", err)
	}

	go func() {
		// 等待停止信号
		<-stop
		cc.Close()
		log.Infoln("[Cache] Exit successful")
	}()

	log.Infoln("[Cache] Load successful")

	return &CacheSvc{cc}
}
