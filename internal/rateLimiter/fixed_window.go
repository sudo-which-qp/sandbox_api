package ratelimiter

import (
	"sync"
	"time"
)

type FixedWindowRateLimiter struct {
	sync.RWMutex
	client map[string]int
	limit  int
	window time.Duration
}

func NewFixedWindowLimiter(limit int, window time.Duration) *FixedWindowRateLimiter {
	return &FixedWindowRateLimiter{
		client: make(map[string]int),
		limit:  limit,
		window: window,
	}
}

func (rateLimit *FixedWindowRateLimiter) Allow(ip string) (bool, time.Duration) {
	rateLimit.RLock()
	count, exist := rateLimit.client[ip]
	rateLimit.RUnlock()

	if !exist || count < rateLimit.limit {
		rateLimit.Lock()
		if !exist {
			go rateLimit.resetCount(ip)
		}

		rateLimit.client[ip]++
		rateLimit.Unlock()
		return true, 0
	}

	return false, rateLimit.window
}

func (rateLimit *FixedWindowRateLimiter) resetCount(ip string) {
	time.Sleep(rateLimit.window)
	rateLimit.Lock()
	delete(rateLimit.client, ip)
	rateLimit.Unlock()
}
