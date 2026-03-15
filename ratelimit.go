package main

import (
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	rateInterval    = 5 * time.Second
	maxTrackedRates = 10_000
)

type VehicleRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	stop     chan struct{}
	once     sync.Once
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewVehicleRateLimiter() *VehicleRateLimiter {
	vrl := &VehicleRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		stop:     make(chan struct{}),
	}
	go vrl.cleanup()
	return vrl
}

// Stop shuts down the background cleanup goroutine.
func (vrl *VehicleRateLimiter) Stop() {
	vrl.once.Do(func() { close(vrl.stop) })
}

func (vrl *VehicleRateLimiter) Allow(key string) bool {
	vrl.mu.Lock()
	defer vrl.mu.Unlock()

	entry, ok := vrl.limiters[key]
	if !ok {
		if len(vrl.limiters) >= maxTrackedRates {
			log.Printf("rate limiter at capacity (%d entries), allowing untracked key %q", maxTrackedRates, key)
			return true
		}
		entry = &rateLimiterEntry{
			limiter: rate.NewLimiter(rate.Every(rateInterval), 1),
		}
		vrl.limiters[key] = entry
	}

	entry.lastSeen = time.Now()
	return entry.limiter.Allow()
}

func (vrl *VehicleRateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().Add(-time.Minute)
			vrl.mu.Lock()
			for id, entry := range vrl.limiters {
				if entry.lastSeen.Before(cutoff) {
					delete(vrl.limiters, id)
				}
			}
			vrl.mu.Unlock()
		case <-vrl.stop:
			return
		}
	}
}
