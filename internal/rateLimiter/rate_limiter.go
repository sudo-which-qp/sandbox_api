package ratelimiter

import "time"

type Limiter interface {
	Allow(ip string) (bool, time.Duration)
}

type Config struct {
	RequestPerTimeForIP int
	TimeFrame           time.Duration
	Enabled             bool
}
