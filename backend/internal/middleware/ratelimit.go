package middleware

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter 简单的滑动窗口计数器（按 key 限流）
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// RateLimit 按客户端 IP 限流：window 时间内最多 max 次
func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	l := &rateLimiter{hits: map[string][]time.Time{}, max: max, window: window}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(ClientIP(r)) {
				JSON(w, http.StatusTooManyRequests, map[string]string{"error": "操作过于频繁，请稍后再试"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
