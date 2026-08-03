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

// AllowNPair atomically consumes the same amount from two independent
// windows.  Chat forwarding uses this so a request rejected by the long
// window does not consume a token from the short window.
func (l *RateLimiter) AllowNPair(action, key string, amount, firstLimit int, firstWindow time.Duration, secondLimit int, secondWindow time.Duration) (bool, time.Duration) {
	if l == nil || amount <= 0 || firstLimit <= 0 || firstWindow <= 0 || secondLimit <= 0 || secondWindow <= 0 {
		return true, 0
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	firstKey := action + "\x00short\x00" + key
	secondKey := action + "\x00long\x00" + key
	first, firstOK := l.buckets[firstKey]
	second, secondOK := l.buckets[secondKey]
	if !firstOK || now.Sub(first.started) >= firstWindow {
		firstOK = false
	}
	if !secondOK || now.Sub(second.started) >= secondWindow {
		secondOK = false
	}
	if (!firstOK && amount > firstLimit) || (!secondOK && amount > secondLimit) {
		return false, firstWindow
	}
	if firstOK && first.count+amount > firstLimit {
		return false, firstWindow - now.Sub(first.started)
	}
	if secondOK && second.count+amount > secondLimit {
		return false, secondWindow - now.Sub(second.started)
	}
	if firstOK {
		first.count += amount
		l.buckets[firstKey] = first
	} else {
		l.buckets[firstKey] = rateBucket{started: now, count: amount}
	}
	if secondOK {
		second.count += amount
		l.buckets[secondKey] = second
	} else {
		l.buckets[secondKey] = rateBucket{started: now, count: amount}
	}
	return true, 0
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: make(map[string]rateBucket), now: time.Now}
}

func (l *RateLimiter) Allow(action, key string, limit int, window time.Duration) (bool, time.Duration) {
	return l.AllowN(action, key, 1, limit, window)
}

// AllowN consumes amount tokens atomically from one rate-limit bucket.
// It is used by operations such as multi-recipient chat forwarding where one
// request creates more than one logical message.
func (l *RateLimiter) AllowN(action, key string, amount, limit int, window time.Duration) (bool, time.Duration) {
	if l == nil || amount <= 0 || limit <= 0 || window <= 0 {
		return true, 0
	}
	now := l.now()
	bucketKey := action + "\x00" + key
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 10000 {
		for bucketKey, bucket := range l.buckets {
			if now.Sub(bucket.started) >= window {
				delete(l.buckets, bucketKey)
			}
		}
	}
	bucket, ok := l.buckets[bucketKey]
	if !ok || now.Sub(bucket.started) >= window {
		if amount > limit {
			return false, window
		}
		l.buckets[bucketKey] = rateBucket{started: now, count: amount}
		return true, 0
	}
	if bucket.count+amount > limit {
		return false, window - now.Sub(bucket.started)
	}
	bucket.count += amount
	l.buckets[bucketKey] = bucket
	return true, 0
}

func RemoteIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return host
	}
	return raw
}
