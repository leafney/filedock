package service

import (
	"testing"
	"time"
)

func TestRateLimiterSeparatesActionsAndIPs(t *testing.T) {
	limiter := NewRateLimiter()
	limiter.now = func() time.Time { return time.Unix(100, 0) }
	if allowed, _ := limiter.Allow("join", "10.0.0.1", 1, time.Minute); !allowed {
		t.Fatal("first request was denied")
	}
	if allowed, retry := limiter.Allow("join", "10.0.0.1", 1, time.Minute); allowed || retry <= 0 {
		t.Fatal("second request should be limited")
	}
	if allowed, _ := limiter.Allow("join", "10.0.0.2", 1, time.Minute); !allowed {
		t.Fatal("different IP was limited")
	}
	if allowed, _ := limiter.Allow("other", "10.0.0.1", 1, time.Minute); !allowed {
		t.Fatal("different action was limited")
	}
	limiter.now = func() time.Time { return time.Unix(161, 0) }
	if allowed, _ := limiter.Allow("join", "10.0.0.1", 1, time.Minute); !allowed {
		t.Fatal("request was not allowed after window reset")
	}
}
