//go:build tt
// +build tt

package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

// RateLimiter provides sliding-window rate limiting.
// Uses Redis when available; falls back to in-memory.
type RateLimiter struct {
	mu       sync.Mutex
	counters map[string]*window
}

type window struct {
	count   int
	resetAt time.Time
}

var globalRateLimiter = &RateLimiter{
	counters: make(map[string]*window),
}

// CheckRateLimit returns true if the request is allowed, false if rate limit exceeded.
// key identifies the entity (e.g. "team_key:<id>"), limit is requests per minute.
func CheckRateLimit(key string, limit int) (bool, int) {
	if limit <= 0 {
		return true, 0
	}

	if common.RedisEnabled {
		return checkRateLimitRedis(key, limit)
	}
	return globalRateLimiter.checkLocal(key, limit)
}

func checkRateLimitRedis(key string, limit int) (bool, int) {
	redisKey := fmt.Sprintf("rate_limit:%s", key)

	ctx := common.RDB.Context()
	current, err := common.RDB.Incr(ctx, redisKey).Result()
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("[RateLimit] Redis INCR failed: %v, falling back to allow", err))
		return true, 0
	}

	if current == 1 {
		common.RDB.Expire(ctx, redisKey, time.Minute)
	}

	remaining := limit - int(current)
	if remaining < 0 {
		remaining = 0
	}
	return int(current) <= limit, remaining
}

func (r *RateLimiter) checkLocal(key string, limit int) (bool, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	w, exists := r.counters[key]
	if !exists || now.After(w.resetAt) {
		r.counters[key] = &window{count: 1, resetAt: now.Add(time.Minute)}
		return true, limit - 1
	}

	w.count++
	remaining := limit - w.count
	if remaining < 0 {
		remaining = 0
	}
	return w.count <= limit, remaining
}
