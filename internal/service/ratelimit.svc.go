package service

import (
	"net"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	started time.Time
	count   int
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
	now     func() time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: make(map[string]rateBucket), now: time.Now}
}

func (l *RateLimiter) Allow(action, ip string, limit int, window time.Duration) (bool, time.Duration) {
	if l == nil || limit <= 0 || window <= 0 {
		return true, 0
	}
	now := l.now()
	key := action + "\x00" + ip
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 10000 {
		for bucketKey, bucket := range l.buckets {
			if now.Sub(bucket.started) >= window {
				delete(l.buckets, bucketKey)
			}
		}
	}
	bucket, ok := l.buckets[key]
	if !ok || now.Sub(bucket.started) >= window {
		l.buckets[key] = rateBucket{started: now, count: 1}
		return true, 0
	}
	if bucket.count >= limit {
		return false, window - now.Sub(bucket.started)
	}
	bucket.count++
	l.buckets[key] = bucket
	return true, 0
}

func RemoteIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return host
	}
	return raw
}
